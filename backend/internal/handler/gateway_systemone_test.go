package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const validSystemOneHandlerBody = `{"model":"jev-latest","state":"sample","questions":{"q":{"type":"noul","instructions":"Evaluate"}}}`

func newSystemOneHandlerContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, recorder
}

type systemOneBillingCache struct{ *handlerInflightCache }

func (s *systemOneBillingCache) GetSubscriptionCache(context.Context, int64, int64) (*service.SubscriptionCacheData, error) {
	return &service.SubscriptionCacheData{Status: service.SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

// Capture the actual atomic billing command, not just the handler's input.
type systemOneBillingRepo struct {
	service.UsageBillingRepository
	commands []*service.UsageBillingCommand
}

func (s *systemOneBillingRepo) Apply(_ context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	s.commands = append(s.commands, cmd)
	return &service.UsageBillingApplyResult{Applied: false}, nil
}

type systemOneAccountRepo struct {
	service.AccountRepository
	account service.Account
}

func (s *systemOneAccountRepo) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]service.Account, error) {
	return []service.Account{s.account}, nil
}
func (s *systemOneAccountRepo) GetByID(context.Context, int64) (*service.Account, error) {
	return &s.account, nil
}

type systemOneUpstream struct {
	service.HTTPUpstream
	calls int
}

func (s *systemOneUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	s.calls++
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}, "X-Request-Id": {"systemone-billing"}}, Body: io.NopCloser(strings.NewReader(`{"answers":{},"usage":{"input_tokens":12,"output_tokens":3}}`))}, nil
}

func TestSystemOneHybridSubscriptionZeroBalancePreservesMetering(t *testing.T) {
	cache := &systemOneBillingCache{newHandlerInflightCache(0)}
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60, FailClosedOnUnpriced: true}
	billingCache := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCache.Stop)
	groupID := int64(3)
	group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformTypeSafe, Status: service.StatusActive, RateMultiplier: 1, SubscriptionType: service.SubscriptionTypeSubscriptionBalance}
	apiKey := &service.APIKey{ID: 4, UserID: 5, User: &service.User{ID: 5}, GroupID: &groupID, Group: group}
	account := service.Account{ID: 6, Platform: service.PlatformTypeSafe, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "test-key"}}
	commands := &systemOneBillingRepo{}
	upstream := &systemOneUpstream{}
	billing := service.NewBillingService(cfg, nil)
	gw := service.NewGatewayService(&systemOneAccountRepo{account: account}, nil, nil, nil, commands, nil, nil, nil, nil, cfg, nil, nil, billing, nil, billingCache, nil, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil, service.NewModelPricingResolver(nil, billing), nil, nil, nil)
	h := &GatewayHandler{cfg: cfg, gatewayService: gw, billingCacheService: billingCache, concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(&helperConcurrencyCacheStub{userSeq: []bool{true}}), SSEPingFormatClaude, 0)}
	c, recorder := newSystemOneHandlerContext(validSystemOneHandlerBody)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Group, group))
	c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 5, Concurrency: 1})
	c.Set(string(middleware2.ContextKeySubscription), &service.UserSubscription{ID: 7})
	c.Set(string(middleware2.ContextKeyBillingSource), service.BillingSourceSubscription)
	binding, ok := service.BuildPackageMeteringBinding(service.PackageMeteringBindingPayload{V: 1, Kind: "skill", Runtime: "matcha", PackageSHA256: strings.Repeat("a", 64), Key: "test"})
	require.True(t, ok)
	c.Request.Header.Set(service.MatchaPackageMeteringHeader, binding)
	c.Request.Header.Set("X-Matcha-Private", "must-not-forward")

	h.SystemOne(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, upstream.calls)
	require.Zero(t, cache.count(), "subscription source must not reserve the zero balance")
	require.Len(t, commands.commands, 1, "mandatory billing must execute")
	cmd := commands.commands[0]
	require.Equal(t, service.BillingTypeSubscription, cmd.BillingType)
	require.Equal(t, int64(7), *cmd.SubscriptionID)
	require.Zero(t, cmd.BalanceCost)
	require.Equal(t, []string{binding}, cmd.PackageMetering.Bindings)
	for name := range c.Request.Header {
		require.False(t, strings.HasPrefix(strings.ToLower(name), "x-matcha-"), name)
	}
}

func TestSystemOneRequiresAuthentication(t *testing.T) {
	c, recorder := newSystemOneHandlerContext(validSystemOneHandlerBody)
	(&GatewayHandler{}).SystemOne(c)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "authentication_error")
}

func TestSystemOneRejectsNonTypeSafeGroupBeforeScheduling(t *testing.T) {
	c, recorder := newSystemOneHandlerContext(validSystemOneHandlerBody)
	groupID := int64(3)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 4, UserID: 5, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI}})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 5, Concurrency: 1})

	(&GatewayHandler{cfg: &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}}}).SystemOne(c)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "only available for TypeSafe")
}

func newTypeSafeGroupContext(t *testing.T, path, body, groupPlatform string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, recorder := newSystemOneHandlerContext(body)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	groupID := int64(9)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 4, UserID: 5, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: groupPlatform}})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 5, Concurrency: 1})
	return c, recorder
}

func TestRejectSystemOneOnlyPlatform(t *testing.T) {
	write := func(c *gin.Context, status int, errType, message string) {
		c.JSON(status, gin.H{"type": errType, "message": message})
	}

	c, recorder := newTypeSafeGroupContext(t, "/v1/messages", `{}`, service.PlatformTypeSafe)
	require.True(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), systemOneOnlyPlatformMessage)

	c, _ = newTypeSafeGroupContext(t, "/v1/messages", `{}`, service.PlatformComposite)
	require.False(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))
	c.Request = c.Request.WithContext(service.WithResolvedTargetPlatform(c.Request.Context(), service.PlatformTypeSafe))
	require.True(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))

	c, _ = newTypeSafeGroupContext(t, "/v1/messages", `{}`, service.PlatformAnthropic)
	require.False(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))

	c, _ = newTypeSafeGroupContext(t, "/antigravity/v1/messages", `{}`, service.PlatformTypeSafe)
	c.Set(string(middleware2.ContextKeyForcePlatform), service.PlatformAntigravity)
	require.False(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))
}

func mustAPIKey(t *testing.T, c *gin.Context) *service.APIKey {
	t.Helper()
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	require.True(t, ok)
	return apiKey
}

func TestTypeSafeGroupsRejectNonSystemOneProtocolsBeforeScheduling(t *testing.T) {
	h := &GatewayHandler{
		cfg:            &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}},
		gatewayService: &service.GatewayService{},
	}
	for _, tc := range []struct {
		name    string
		path    string
		body    string
		handler func(*gin.Context)
	}{
		{"messages", "/v1/messages", `{"model":"jev-latest","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`, h.Messages},
		{"count_tokens", "/v1/messages/count_tokens", `{"model":"jev-latest","messages":[{"role":"user","content":"hi"}]}`, h.CountTokens},
		{"chat_completions", "/v1/chat/completions", `{"model":"jev-latest","messages":[{"role":"user","content":"hi"}]}`, h.ChatCompletions},
		{"responses", "/v1/responses", `{"model":"jev-latest","input":"hi"}`, h.Responses},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := newTypeSafeGroupContext(t, tc.path, tc.body, service.PlatformTypeSafe)
			tc.handler(c)
			require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), systemOneOnlyPlatformMessage)
		})
	}
}
