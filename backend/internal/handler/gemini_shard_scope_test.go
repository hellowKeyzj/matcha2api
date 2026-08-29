//go:build unit

package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type geminiShardScopeRepo struct {
	service.GroupShardRepository
	scope   *service.GroupShardAccountScope
	err     error
	calls   int
	userID  int64
	groupID int64
}

func (r *geminiShardScopeRepo) ListAccountIDsForUserShard(_ context.Context, group *service.Group, userID int64, _ string) (*service.GroupShardAccountScope, error) {
	r.calls++
	r.userID, r.groupID = userID, group.ID
	return r.scope, r.err
}

func newGeminiScopeGateway(group *service.Group, shards service.GroupShardRepository) *service.GatewayService {
	return service.NewGatewayService(nil, &fakeGroupRepo{group: group}, shards, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

type geminiScopeUpstream struct {
	service.HTTPUpstream
	accountIDs []int64
}

func (u *geminiScopeUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.accountIDs = append(u.accountIDs, accountID)
	body := `{"name":"models/gemini-owner"}`
	if req.URL.Path == "/v1beta/models" {
		body = `{"models":[{"name":"models/gemini-owner"}]}`
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func TestGeminiModelsEndpointsRespectShardScope(t *testing.T) {
	for _, declared := range []bool{false, true} {
		for _, endpoint := range []string{"list", "get"} {
			for _, scopeCase := range []string{"allowed", "empty", "error"} {
				t.Run(endpoint+"/"+scopeCase+map[bool]string{true: "/declared", false: "/upstream"}[declared], func(t *testing.T) {
					group := &service.Group{ID: 42, Hydrated: true, Platform: service.PlatformGemini, Status: service.StatusActive, ShardEnabled: true}
					apiKey := geminiCachedContentTestAPIKey(service.PlatformGemini)
					apiKey.Group = group
					shards := &geminiShardScopeRepo{scope: &service.GroupShardAccountScope{Enabled: true, AccountIDs: map[int64]struct{}{901: {}}}}
					if scopeCase == "empty" {
						shards.scope.AccountIDs = nil
					}
					if scopeCase == "error" {
						shards.err = errors.New("scope unavailable")
					}
					accounts := []service.Account{
						{ID: 900, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Priority: 0, Credentials: map[string]any{"api_key": "other"}},
						{ID: 901, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Priority: 9, Credentials: map[string]any{"api_key": "owner"}},
						{ID: 902, Platform: service.PlatformAntigravity, Extra: map[string]any{"mixed_scheduling": true}, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-other-mixed": "gemini-other-mixed"}}},
					}
					if declared {
						accounts[0].Credentials["model_mapping"] = map[string]any{"gemini-other": "gemini-other"}
						accounts[1].Credentials["model_mapping"] = map[string]any{"gemini-owner": "gemini-owner"}
					}
					repo := &geminiAllowlistAccountRepoStub{gatewayModelsAccountRepoStub: gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{group.ID: accounts}}}
					upstream := &geminiScopeUpstream{}
					h := &GatewayHandler{gatewayService: newGeminiScopeGateway(group, shards), geminiCompatService: service.NewGeminiMessagesCompatService(repo, nil, nil, nil, nil, nil, upstream, nil, &config.Config{})}
					c, rec := newGeminiCachedContentTestContext(http.MethodGet, "/v1beta/models", apiKey)
					c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Group, group))
					if endpoint == "get" {
						c.Params = gin.Params{{Key: "model", Value: "gemini-owner"}}
						h.GeminiV1BetaGetModel(c)
					} else {
						h.GeminiV1BetaListModels(c)
					}
					if scopeCase != "allowed" {
						require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
						require.Empty(t, upstream.accountIDs)
					} else {
						require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
						require.NotContains(t, rec.Body.String(), "gemini-other")
						if declared {
							require.Empty(t, upstream.accountIDs)
						} else {
							require.Equal(t, []int64{901}, upstream.accountIDs)
						}
					}
					require.Positive(t, shards.calls)
					require.Equal(t, apiKey.UserID, shards.userID, "Google auth supplies AuthSubject, not ctxkey.UserID")
					require.Equal(t, group.ID, shards.groupID)
				})
			}
		}
	}
}

func TestGeminiModelsForcedAntigravityRejectsEmptyShardScope(t *testing.T) {
	for _, endpoint := range []string{"list", "get"} {
		t.Run(endpoint, func(t *testing.T) {
			group := &service.Group{ID: 42, Hydrated: true, Platform: service.PlatformGemini, Status: service.StatusActive, ShardEnabled: true}
			apiKey := geminiCachedContentTestAPIKey(service.PlatformGemini)
			apiKey.Group = group
			shards := &geminiShardScopeRepo{scope: &service.GroupShardAccountScope{Enabled: true}}
			h := &GatewayHandler{gatewayService: newGeminiScopeGateway(group, shards)}
			c, rec := newGeminiCachedContentTestContext(http.MethodGet, "/antigravity/v1beta/models", apiKey)
			c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Group, group))
			c.Set(string(middleware.ContextKeyForcePlatform), service.PlatformAntigravity)
			if endpoint == "get" {
				c.Params = gin.Params{{Key: "model", Value: "gemini-owner"}}
				h.GeminiV1BetaGetModel(c)
			} else {
				h.GeminiV1BetaListModels(c)
			}
			require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
			require.Equal(t, apiKey.UserID, shards.userID)
		})
	}
}

type geminiScopeMutationRepo struct {
	*cachedBillingRepo
	beginCalls int
}

type geminiScopeBillingUserRepo struct {
	service.UserRepository
	reads int
}

func (r *geminiScopeBillingUserRepo) GetByID(_ context.Context, userID int64) (*service.User, error) {
	r.reads++
	return &service.User{ID: userID, Balance: 1000}, nil
}

func (r *geminiScopeMutationRepo) BeginMutation(ctx context.Context, id int64, baseline time.Time, token string, lease time.Duration) (bool, error) {
	r.beginCalls++
	return r.cachedBillingRepo.BeginMutation(ctx, id, baseline, token, lease)
}

func TestGeminiCachedContentsPatchRejectsWithdrawnOwnerBeforeSideEffects(t *testing.T) {
	for _, name := range []string{"owner withdrawn", "empty scope", "scope error"} {
		t.Run(name, func(t *testing.T) {
			shards := &geminiShardScopeRepo{scope: &service.GroupShardAccountScope{Enabled: true, AccountIDs: map[int64]struct{}{901: {}}}}
			if name == "empty scope" {
				shards.scope.AccountIDs = nil
			}
			if name == "scope error" {
				shards.err = errors.New("scope unavailable")
			}
			env := newCachedBillingEnvWithShard(t, 1000, shards, 900)
			env.group.ShardEnabled = true
			baseline := time.Now().Add(time.Hour)
			env.seed(baseline)
			billingUsers := &geminiScopeBillingUserRepo{}
			env.h.billingCacheService = service.NewBillingCacheService(nil, billingUsers, nil, nil, nil, nil, &config.Config{}, nil)
			t.Cleanup(env.h.billingCacheService.Stop)
			repo := &geminiScopeMutationRepo{cachedBillingRepo: env.repo}
			compat := service.NewGeminiMessagesCompatService(&cachedBillingAccountRepo{accounts: env.accounts}, nil, nil, nil, nil, nil, env.upstream, nil, &config.Config{})
			env.h.geminiCachedContentService = service.NewGeminiCachedContentService(repo, compat, &config.Config{})
			rec := env.patch()
			require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
			require.Zero(t, repo.beginCalls)
			require.Zero(t, billingUsers.reads, "reject before billing eligibility and fixed-charge reservation")
			require.Empty(t, env.upstream.calls)
			require.Empty(t, env.usage.totals())
			require.Empty(t, env.repo.deleted)
			require.Equal(t, baseline, env.repo.records["abc"].ExpireTime)
			require.Equal(t, env.apiKey.UserID, shards.userID)
			// Cleanup of an existing resource must remain possible outside the scope.
			before := shards.calls
			rec = env.serve(http.MethodDelete, "/v1beta/cachedContents/abc", "", gin.Params{{Key: "cacheID", Value: "abc"}}, env.h.GeminiCachedContentsDelete)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, before, shards.calls)
			require.Equal(t, 1, env.upstream.methodCalls(http.MethodDelete))
		})
	}
}

func TestGeminiCachedContentsPatchAllowedScopeKeepsBillingAndRecovery(t *testing.T) {
	shards := &geminiShardScopeRepo{scope: &service.GroupShardAccountScope{Enabled: true, AccountIDs: map[int64]struct{}{900: {}}}}
	env := newCachedBillingEnvWithShard(t, 1000, shards, 900)
	env.group.ShardEnabled = true
	env.seed(time.Now().Add(time.Hour))
	rec := env.patch()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, env.upstream.methodCalls(http.MethodPatch))
	require.Len(t, env.waitForUsage(t, 1), 1)
	require.Equal(t, env.apiKey.UserID, shards.userID)

	// A committed upstream mutation that failed to persist still needs rollback
	// after the owner is withdrawn; this is cleanup, not a new user operation.
	env.repo.failCommit = true
	rec = env.patch()
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	env.repo.failCommit = false
	env.repo.expireLease(100)
	shards.scope.AccountIDs = nil
	before := shards.calls
	recovered, err := env.h.geminiCachedContentService.RecoverStaleMutations(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.Equal(t, before, shards.calls)
	require.False(t, env.repo.pendingMutation(100))
	require.Equal(t, 3, env.upstream.methodCalls(http.MethodPatch))
}
