package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newTestGinContext builds a bare gin.Context backed by an httptest recorder.
func newTestGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c
}

type cyberPolicyBillingRepo struct {
	service.UsageBillingRepository
	commands chan *service.UsageBillingCommand
	release  chan struct{}
}

func (r *cyberPolicyBillingRepo) Apply(_ context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	r.commands <- cmd
	<-r.release
	return &service.UsageBillingApplyResult{Applied: false}, nil
}

func TestRecordCyberPolicyIfMarked_FailedPackageUsageRetainsWSReservation(t *testing.T) {
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60}
	cache := newHandlerInflightCache(1)
	billingCache := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCache.Stop)
	groupID := int64(3)
	apiKey := &service.APIKey{ID: 4, UserID: 5, User: &service.User{ID: 5}, GroupID: &groupID,
		Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, RateMultiplier: 1, SubscriptionType: service.SubscriptionTypeSubscriptionBalance}}
	account := &service.Account{ID: 6, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey}
	subscription := &service.UserSubscription{ID: 7}
	// Like WS, only the explicit local parent carries the reservation, not c.Request.
	parent, done, err := reserveInflightBalanceCtx(context.Background(), billingCache, &countingEstimator{cost: 0.9, priced: true}, apiKey, subscription, service.BillingSourceBalance, tokenInflightEstimate("gpt-5", nil))
	require.NoError(t, err)
	defer done()
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{WorkerCount: 1, QueueSize: 1, TaskTimeout: time.Second, OverflowPolicy: "drop"})
	t.Cleanup(pool.Stop)
	started, unblock := make(chan struct{}), make(chan struct{})
	defer close(unblock)
	pool.Submit(func(context.Context) { close(started); <-unblock })
	<-started
	repo := &cyberPolicyBillingRepo{commands: make(chan *service.UsageBillingCommand, 2), release: make(chan struct{})}
	defer close(repo.release)
	billing := service.NewBillingService(cfg, nil)
	gw := service.NewOpenAIGatewayService(nil, nil, nil, nil, repo, nil, nil, nil, nil, cfg, nil, nil, billing, nil, nil, nil, nil, nil, nil, service.NewModelPricingResolver(nil, billing), nil, nil, nil, nil)
	h := &OpenAIGatewayHandler{gatewayService: gw, usageRecordWorkerPool: pool}
	c := newTestGinContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Writer.Header().Set("X-Request-Id", "cyber-package")
	binding, ok := service.BuildPackageMeteringBinding(service.PackageMeteringBindingPayload{V: 1, Kind: "skill", Runtime: "matcha", PackageSHA256: strings.Repeat("a", 64), Key: "test"})
	require.True(t, ok)
	metering := service.PackageMeteringUsage{Bindings: []string{binding}}
	mark := service.CyberPolicyMark{Message: "blocked", UpstreamStatus: http.StatusBadRequest}
	service.MarkOpsCyberPolicy(c, mark)
	h.recordCyberPolicyIfMarked(c, parent, apiKey, account, subscription, service.BillingSourceBalance, metering, "gpt-5", false, nil, service.ChannelUsageFields{}, "")
	require.Empty(t, repo.commands, "successful forwards use the normal usage path, not cyber billing")
	clearCyberPolicyTurnState(c)
	service.MarkOpsCyberPolicy(c, mark)
	h.recordCyberPolicyIfMarked(c, parent, apiKey, account, subscription, service.BillingSourceBalance, metering, "gpt-5", true, nil, service.ChannelUsageFields{}, "")
	h.recordCyberPolicyIfMarked(c, parent, apiKey, account, subscription, service.BillingSourceBalance, metering, "gpt-5", true, nil, service.ChannelUsageFields{}, "")
	done()
	require.Equal(t, 1, cache.count(), "queued failed-forward billing must acquire before WS handler retires")
	unblock <- struct{}{}
	select {
	case cmd := <-repo.commands:
		require.Equal(t, service.BillingTypeBalance, cmd.BillingType)
		require.Nil(t, cmd.SubscriptionID, "explicit balance source overrides the retained subscription")
		require.Equal(t, []string{binding}, cmd.PackageMetering.Bindings, "zero-token failure still meters packages")
		require.Equal(t, 1, cache.count(), "reservation remains until atomic billing completes")
	case <-time.After(3 * time.Second):
		t.Fatal("failed-forward package billing was not submitted")
	}
	repo.release <- struct{}{}
	require.Eventually(t, func() bool { return cache.count() == 0 }, time.Second, time.Millisecond)
	require.Empty(t, repo.commands, "recorded guard prevents double billing")
}

// TestRecordCyberPolicyIfMarked_NoMark verifies that when no cyber mark is set,
// the function returns immediately and does NOT set the recorded flag.
func TestRecordCyberPolicyIfMarked_NoMark(t *testing.T) {
	c := newTestGinContext()
	h := &OpenAIGatewayHandler{}

	h.recordCyberPolicyIfMarked(c, context.Background(), nil, nil, nil, service.BillingSourceBalance, service.PackageMeteringUsage{}, "gpt-5", true, nil, service.ChannelUsageFields{}, "")

	// Flag must NOT be set when there was no mark.
	require.False(t, c.GetBool(cyberPolicyRecordedKey),
		"cyberPolicyRecordedKey must remain false when no cyber mark is present")
}

// TestRecordCyberPolicyIfMarked_WithMark verifies that:
//  1. When a cyber mark is present, the recorded flag is set (guard activated).
//  2. A second call is a no-op (idempotent guard).
//  3. Nil services do not panic.
func TestRecordCyberPolicyIfMarked_WithMark(t *testing.T) {
	c := newTestGinContext()
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{
		Message:        "flagged",
		Body:           `{"error":{"code":"cyber_policy"}}`,
		UpstreamStatus: 400,
	})

	h := &OpenAIGatewayHandler{} // nil services — must not panic

	// First call: should set the flag.
	require.NotPanics(t, func() {
		h.recordCyberPolicyIfMarked(c, context.Background(), nil, nil, nil, service.BillingSourceBalance, service.PackageMeteringUsage{}, "gpt-5", true, nil, service.ChannelUsageFields{}, "")
	})
	require.True(t, c.GetBool(cyberPolicyRecordedKey),
		"cyberPolicyRecordedKey must be true after first call with a mark")

	// Second call: flag already set — must be a no-op (idempotent).
	require.NotPanics(t, func() {
		h.recordCyberPolicyIfMarked(c, context.Background(), nil, nil, nil, service.BillingSourceBalance, service.PackageMeteringUsage{}, "gpt-5", false, nil, service.ChannelUsageFields{}, "")
	})
	// Flag should still be true (not toggled or cleared).
	require.True(t, c.GetBool(cyberPolicyRecordedKey),
		"cyberPolicyRecordedKey must remain true after second call (guard)")
}

// TestRecordCyberPolicyIfMarked_ForwardSuccessSkipsUsageLog verifies the semantic:
// when forwardErrored=false the function still sets the guard flag (mark present),
// but the cyber usage row is NOT requested (only RecordCyberPolicyEvent fires).
// Since services are nil here we only verify the guard flag and no panic.
func TestRecordCyberPolicyIfMarked_ForwardSuccessSkipsUsageLog(t *testing.T) {
	c := newTestGinContext()
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{
		Message:        "flagged",
		UpstreamStatus: 200,
	})

	h := &OpenAIGatewayHandler{}

	require.NotPanics(t, func() {
		h.recordCyberPolicyIfMarked(c, context.Background(), nil, nil, nil, service.BillingSourceBalance, service.PackageMeteringUsage{}, "gpt-5", false /* forwardErrored=false */, nil, service.ChannelUsageFields{}, "")
	})
	require.True(t, c.GetBool(cyberPolicyRecordedKey))
}

// TestClearCyberPolicyTurnState verifies F1 at the handler level: after a turn
// is finalized, both the mark and the recorded guard are reset so the next WS
// turn detects/records independently.
func TestClearCyberPolicyTurnState(t *testing.T) {
	c := newTestGinContext()
	h := &OpenAIGatewayHandler{}

	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{Message: "turn1", UpstreamStatus: 200})
	h.recordCyberPolicyIfMarked(c, context.Background(), nil, nil, nil, service.BillingSourceBalance, service.PackageMeteringUsage{}, "gpt-5", false, nil, service.ChannelUsageFields{}, "")
	require.True(t, c.GetBool(cyberPolicyRecordedKey))

	clearCyberPolicyTurnState(c)
	require.Nil(t, service.GetOpsCyberPolicy(c))
	require.False(t, c.GetBool(cyberPolicyRecordedKey))

	// turn2: a fresh cyber hit must be recordable again.
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{Message: "turn2", UpstreamStatus: 200})
	h.recordCyberPolicyIfMarked(c, context.Background(), nil, nil, nil, service.BillingSourceBalance, service.PackageMeteringUsage{}, "gpt-5", false, nil, service.ChannelUsageFields{}, "")
	require.True(t, c.GetBool(cyberPolicyRecordedKey))
	require.Equal(t, "turn2", service.GetOpsCyberPolicy(c).Message)
}

func TestAdvanceOpenAIWSCyberBlockStateDefersBlockAcrossFailover(t *testing.T) {
	failoverErr := &service.UpstreamFailoverError{StatusCode: http.StatusTooManyRequests}

	blocked, pending := advanceOpenAIWSCyberBlockState(false, false, true, failoverErr)
	require.False(t, blocked, "the replacement account must receive the current turn")
	require.True(t, pending, "the cyber hit must still block later turns")

	blocked, pending = advanceOpenAIWSCyberBlockState(blocked, pending, false, failoverErr)
	require.False(t, blocked, "additional failover attempts must remain eligible")
	require.True(t, pending)

	blocked, pending = advanceOpenAIWSCyberBlockState(blocked, pending, false, nil)
	require.True(t, blocked, "the next client turn must be blocked after failover finishes")
	require.False(t, pending)
}

func TestClearCyberPolicyAttemptStatePreservesRecordedGuardDuringFailover(t *testing.T) {
	c := newTestGinContext()
	h := &OpenAIGatewayHandler{}

	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{Message: "account-a", UpstreamStatus: http.StatusOK})
	h.recordCyberPolicyIfMarked(c, context.Background(), nil, nil, nil, service.BillingSourceBalance, service.PackageMeteringUsage{}, "gpt-5", true, nil, service.ChannelUsageFields{}, "")
	require.True(t, c.GetBool(cyberPolicyRecordedKey))

	clearCyberPolicyAttemptState(c, false)
	require.Nil(t, service.GetOpsCyberPolicy(c))
	require.True(t, c.GetBool(cyberPolicyRecordedKey), "the same logical turn must not record again after failover")

	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{Message: "account-b", UpstreamStatus: http.StatusOK})
	h.recordCyberPolicyIfMarked(c, context.Background(), nil, nil, nil, service.BillingSourceBalance, service.PackageMeteringUsage{}, "gpt-5", true, nil, service.ChannelUsageFields{}, "")
	require.Equal(t, "account-b", service.GetOpsCyberPolicy(c).Message)
	require.True(t, c.GetBool(cyberPolicyRecordedKey))

	clearCyberPolicyAttemptState(c, true)
	require.Nil(t, service.GetOpsCyberPolicy(c))
	require.False(t, c.GetBool(cyberPolicyRecordedKey), "a completed logical turn must reset the guard")
}

// TestBuildCyberSessionBlockedOpsEntry verifies the locally-rejected request is
// auditable: 403 / phase=request / type=cyber_policy_session_blocked — distinct
// from upstream cyber_policy hits, and it must NOT touch moderation/violation.
func TestBuildCyberSessionBlockedOpsEntry(t *testing.T) {
	entry := buildCyberSessionBlockedOpsEntry(cyberPolicyOpsErrorMeta{
		RequestID: "req-9", Model: "gpt-5", RequestPath: "/openai/v1/responses",
	})
	require.Equal(t, 403, entry.StatusCode)
	require.Equal(t, "cyber_policy_session_blocked", entry.ErrorType)
	require.Equal(t, "request", entry.ErrorPhase)
	require.True(t, entry.IsBusinessLimited)
	require.Equal(t, "gateway_local", entry.ErrorSource)
	require.Equal(t, "platform", entry.ErrorOwner)
	require.Empty(t, entry.ErrorBody, "no session block key → ErrorBody must be empty")

	entryWithKey := buildCyberSessionBlockedOpsEntry(cyberPolicyOpsErrorMeta{
		RequestID: "req-9", Model: "gpt-5", RequestPath: "/openai/v1/responses",
		SessionBlockKey: "abc123",
	})
	require.Equal(t, "session_block_key=abc123", entryWithKey.ErrorBody)
}

// TestRejectIfCyberSessionBlocked_FailOpen verifies fail-open paths: nil handler
// services, no explicit session signal, and (implicitly) disabled switch all
// pass the request through.
func TestRejectIfCyberSessionBlocked_FailOpen(t *testing.T) {
	c := newTestGinContext()
	c.Request = httptest.NewRequest("POST", "/openai/v1/responses", strings.NewReader(`{}`))

	h := &OpenAIGatewayHandler{}
	require.False(t, h.rejectIfCyberSessionBlocked(c, nil, []byte(`{}`), "gpt-5", cyberBlockFormatResponses), "nil apiKey → pass")

	h2 := &OpenAIGatewayHandler{gatewayService: nil}
	key := &service.APIKey{ID: 1}
	require.False(t, h2.rejectIfCyberSessionBlocked(c, key, []byte(`{}`), "gpt-5", cyberBlockFormatResponses), "nil gateway service → pass")
}

func TestBuildCyberSessionBlockWritePlanCombinesExplicitAndTranscriptKeys(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"trigger"}]}`)
	c := newTestGinContext()
	c.Request = httptest.NewRequest("POST", "/openai/v1/responses", strings.NewReader(string(body)))
	c.Request.RemoteAddr = "203.0.113.44:12345"
	c.Request.Header.Set("User-Agent", "client/1.2.3")

	plan := buildCyberSessionBlockWritePlan(7, c, body)
	require.Len(t, plan.keys, 2)
	require.NotEmpty(t, plan.scopeKey)

	c.Request.Header.Set("session_id", "sess-explicit")
	plan = buildCyberSessionBlockWritePlan(7, c, body)
	require.Len(t, plan.keys, 3)
	require.NotEmpty(t, plan.scopeKey)
}

// TestRecordCyberPolicyIfMarked_BlockKeyPlumbed verifies the 6th param is
// accepted and a non-empty key with nil gateway service does not panic
// (write-side guards live in the service layer).
func TestRecordCyberPolicyIfMarked_BlockKeyPlumbed(t *testing.T) {
	c := newTestGinContext()
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{Message: "x", UpstreamStatus: 400})
	h := &OpenAIGatewayHandler{}
	require.NotPanics(t, func() {
		h.recordCyberPolicyIfMarked(c, context.Background(), nil, nil, nil, service.BillingSourceBalance, service.PackageMeteringUsage{}, "gpt-5", true, []byte(`{"input":"deadbeef"}`), service.ChannelUsageFields{}, "")
	})
}

// TestBuildCyberPolicyOpsErrorEntry_StatusCode verifies F6: the ops error log
// records the status the codex client actually received (400 non-stream / 200 stream),
// not a hardcoded 403.
func TestBuildCyberPolicyOpsErrorEntry_StatusCode(t *testing.T) {
	for _, tc := range []struct {
		name           string
		upstreamStatus int
	}{
		{"non_stream_400", 400},
		{"stream_200", 200},
		{"zero_value", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mark := &service.CyberPolicyMark{
				Code:           "cyber_policy",
				Message:        "blocked",
				UpstreamStatus: tc.upstreamStatus,
			}
			entry := buildCyberPolicyOpsErrorEntry(cyberPolicyOpsErrorMeta{
				RequestID: "req-1", Model: "gpt-5", RequestPath: "/openai/v1/responses",
			}, mark)
			require.Equal(t, tc.upstreamStatus, entry.StatusCode)
			require.Equal(t, "cyber_policy", entry.ErrorType)
			require.Equal(t, "request", entry.ErrorPhase)
		})
	}
}
