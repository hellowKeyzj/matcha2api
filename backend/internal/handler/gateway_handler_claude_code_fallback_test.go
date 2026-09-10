//go:build unit

package handler

import (
	"bytes"
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// groupScopedSchedulerCache 按桶的分组只返回该分组的成员账号，并记录被查询过的分组。
type groupScopedSchedulerCache struct {
	*fakeSchedulerCache
	mu       sync.Mutex
	groupIDs []int64
}

func (c *groupScopedSchedulerCache) GetSnapshot(_ context.Context, bucket service.SchedulerBucket) ([]*service.Account, bool, error) {
	c.mu.Lock()
	c.groupIDs = append(c.groupIDs, bucket.GroupID)
	c.mu.Unlock()
	var members []*service.Account
	for _, account := range c.accounts {
		for _, ag := range account.AccountGroups {
			if ag.GroupID == bucket.GroupID {
				members = append(members, account)
				break
			}
		}
	}
	return members, true, nil
}

func (c *groupScopedSchedulerCache) queriedGroupIDs() []int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]int64(nil), c.groupIDs...)
}

type groupMapRepo struct {
	*fakeGroupRepo
	groups map[int64]*service.Group
}

func (r *groupMapRepo) GetByID(_ context.Context, id int64) (*service.Group, error) {
	if group, ok := r.groups[id]; ok {
		return group, nil
	}
	return nil, service.ErrGroupNotFound
}

func (r *groupMapRepo) GetByIDLite(ctx context.Context, id int64) (*service.Group, error) {
	return r.GetByID(ctx, id)
}

type fallbackReservationUpstream struct {
	service.HTTPUpstream
	t       *testing.T
	calls   []int64
	amounts []float64
	oldTask service.UsageRecordTask
}

func (s *fallbackReservationUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	s.calls = append(s.calls, accountID)
	s.amounts = append(s.amounts, service.InflightReservationFromContext(req.Context()).Amount())
	if accountID == 9310 {
		// A task captured before the group switch retains only the original reservation.
		s.oldTask, _ = wrapUsageRecordTaskContext(req.Context(), func(context.Context) {})
		return &http.Response{StatusCode: http.StatusBadRequest, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"Prompt is too long"}}`))}, nil
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}, "X-Request-Id": {"fallback-billing"}}, Body: io.NopCloser(strings.NewReader(`{"id":"msg_fallback","type":"message","role":"assistant","model":"claude-opus-4-6","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":12,"output_tokens":3}}`))}, nil
}
func (s *fallbackReservationUpstream) DoWithTLS(req *http.Request, proxy string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxy, accountID, concurrency)
}

func TestMessagesInvalidRequestFallbackReplacesReservation(t *testing.T) {
	for _, source := range []service.BillingSource{service.BillingSourceSubscription, service.BillingSourceBalance} {
		t.Run(string(source), func(t *testing.T) {
			cfg := &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}}
			cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60, FailClosedOnUnpriced: true}
			cache := &systemOneBillingCache{newHandlerInflightCache(100)}
			billingCache := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billingCache.Stop)
			fallbackID := int64(9301)
			primary := &service.Group{ID: 9300, Hydrated: true, Platform: service.PlatformAntigravity, Status: service.StatusActive, RateMultiplier: 0.1, SubscriptionType: service.SubscriptionTypeSubscriptionBalance, FallbackGroupIDOnInvalidRequest: &fallbackID}
			fallback := &service.Group{ID: fallbackID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, RateMultiplier: 2}
			accounts := []*service.Account{
				{ID: 9310, Platform: service.PlatformAntigravity, Type: service.AccountTypeOAuth, Status: service.StatusActive, Schedulable: true, Concurrency: 1, AccountGroups: []service.AccountGroup{{GroupID: primary.ID}}, Credentials: map[string]any{"access_token": "token", "project_id": "proj", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)}},
				{ID: 9311, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1, AccountGroups: []service.AccountGroup{{GroupID: fallbackID}}, Credentials: map[string]any{"api_key": "token"}},
			}
			snapshot := service.NewSchedulerSnapshotService(&groupScopedSchedulerCache{fakeSchedulerCache: &fakeSchedulerCache{accounts: accounts}}, nil, nil, nil, nil)
			upstream := &fallbackReservationUpstream{t: t}
			commands := &systemOneBillingRepo{}
			billing := service.NewBillingService(cfg, nil)
			gw := service.NewGatewayService(nil, &groupMapRepo{groups: map[int64]*service.Group{primary.ID: primary, fallbackID: fallback}}, nil, nil, commands, nil, nil, nil, nil, cfg, snapshot, nil, billing, nil, billingCache, nil, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil, service.NewModelPricingResolver(nil, billing), nil, nil, nil)
			h := &GatewayHandler{cfg: cfg, gatewayService: gw, billingCacheService: billingCache, antigravityGatewayService: service.NewAntigravityGatewayService(nil, nil, snapshot, service.NewAntigravityTokenProvider(nil, nil, nil), nil, upstream, nil, nil), concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0)}
			body := []byte(`{"model":"claude-opus-4-6","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`)
			apiKey := &service.APIKey{ID: 4, UserID: 5, User: &service.User{ID: 5, Balance: 100}, GroupID: &primary.ID, Group: primary}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), ctxkey.Group, primary))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set(string(middleware.ContextKeyAPIKey), apiKey)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 5, Concurrency: 1})
			c.Set(string(middleware.ContextKeySubscription), &service.UserSubscription{ID: 7})
			c.Set(string(middleware.ContextKeyBillingSource), source)

			h.Messages(c)

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Equal(t, []int64{9310, 9311}, upstream.calls)
			if source.IsSubscription() {
				require.Zero(t, upstream.amounts[0])
			} else {
				require.Positive(t, upstream.amounts[0])
			}
			expected, priced := gw.EstimateInflightReservation(c.Request.Context(), cloneAPIKeyWithGroup(apiKey, fallback), tokenInflightEstimate("claude-opus-4-6", body))
			require.True(t, priced)
			require.InDelta(t, expected, upstream.amounts[1], 1e-12, "new reservation must use fallback group multiplier")
			require.Len(t, commands.commands, 1)
			require.Equal(t, service.BillingTypeBalance, commands.commands[0].BillingType)
			require.Nil(t, commands.commands[0].SubscriptionID)
			if source.IsSubscription() {
				require.Zero(t, cache.count())
			} else {
				require.Equal(t, 1, cache.count(), "only original task reference may survive")
			}
			upstream.oldTask(context.Background())
			require.Zero(t, cache.count(), "both groups' handler/task references must be reclaimed")
		})
	}
}

func TestGatewayOpenAICompatibleHandlersClaudeCodeOnlyFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const (
		primaryGroupID    = int64(9300)
		fallbackGroupID   = int64(9301)
		primaryAccountID  = int64(9310)
		fallbackAccountID = int64(9311)
	)

	// 账号不带 api_key：转发在取令牌时失败，请求不会触达上游，断言只看选号结果。
	newAccount := func(id, groupID int64) *service.Account {
		return &service.Account{
			ID: id, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: true, Concurrency: 1,
			AccountGroups: []service.AccountGroup{{AccountID: id, GroupID: groupID}},
		}
	}

	endpoints := []struct {
		name string
		path string
		body string
		call func(*GatewayHandler, *gin.Context)
	}{
		{
			name: "responses", path: "/v1/responses",
			body: `{"model":"claude-sonnet-4-5","input":"hello","stream":false}`,
			call: (*GatewayHandler).Responses,
		},
		{
			name: "chat completions", path: "/v1/chat/completions",
			body: `{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hello"}],"stream":false}`,
			call: (*GatewayHandler).ChatCompletions,
		},
	}

	for _, ep := range endpoints {
		for _, tc := range []struct {
			name        string
			hasFallback bool
		}{
			{name: "with fallback group", hasFallback: true},
			{name: "without fallback group", hasFallback: false},
		} {
			t.Run(ep.name+"/"+tc.name, func(t *testing.T) {
				primary := &service.Group{
					ID: primaryGroupID, Hydrated: true, Platform: service.PlatformAnthropic,
					Status: service.StatusActive, ClaudeCodeOnly: true,
				}
				if tc.hasFallback {
					fallbackID := fallbackGroupID
					primary.FallbackGroupID = &fallbackID
				}
				fallback := &service.Group{
					ID: fallbackGroupID, Hydrated: true, Platform: service.PlatformAnthropic,
					Status: service.StatusActive,
				}

				schedulerCache := &groupScopedSchedulerCache{fakeSchedulerCache: &fakeSchedulerCache{accounts: []*service.Account{
					newAccount(primaryAccountID, primaryGroupID),
					newAccount(fallbackAccountID, fallbackGroupID),
				}}}
				gatewayService := service.NewGatewayService(
					nil, &groupMapRepo{fakeGroupRepo: &fakeGroupRepo{}, groups: map[int64]*service.Group{
						primaryGroupID:  primary,
						fallbackGroupID: fallback,
					}}, nil, nil, nil, nil, nil, nil, nil, nil,
					service.NewSchedulerSnapshotService(schedulerCache, nil, nil, nil, nil),
					nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
				)
				cfg := &config.Config{RunMode: config.RunModeSimple}
				billingCacheService := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
				t.Cleanup(billingCacheService.Stop)
				h := &GatewayHandler{
					gatewayService:      gatewayService,
					billingCacheService: billingCacheService,
					concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0),
					maxAccountSwitches:  1,
					cfg:                 cfg,
				}

				primaryGroupIDRef := primaryGroupID
				apiKey := &service.APIKey{
					ID: 9320, UserID: 9330, GroupID: &primaryGroupIDRef, Group: primary, Status: service.StatusActive,
					User: &service.User{ID: 9330, Concurrency: 10, Balance: 100},
				}
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				ctx := context.WithValue(context.Background(), ctxkey.Group, primary)
				req := httptest.NewRequest(http.MethodPost, ep.path, bytes.NewBufferString(ep.body)).WithContext(ctx)
				req.Header.Set("Content-Type", "application/json")
				c.Request = req
				c.Set(string(middleware.ContextKeyAPIKey), apiKey)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})

				ep.call(h, c)

				selected, reachedSelection := c.Get(opsAccountIDKey)
				if !tc.hasFallback {
					require.Equal(t, http.StatusForbidden, recorder.Code)
					require.Contains(t, recorder.Body.String(), "This group is restricted to Claude Code clients")
					require.False(t, reachedSelection, "a Claude Code only group without fallback must be rejected before account selection")
					require.Empty(t, schedulerCache.queriedGroupIDs())
					return
				}
				require.NotContains(t, recorder.Body.String(), "restricted to Claude Code clients")
				require.True(t, reachedSelection, "a Claude Code only group with fallback must reach account selection")
				require.Equal(t, fallbackAccountID, selected)
				queried := schedulerCache.queriedGroupIDs()
				require.Contains(t, queried, fallbackGroupID)
				require.NotContains(t, queried, primaryGroupID)
			})
		}
	}
}
