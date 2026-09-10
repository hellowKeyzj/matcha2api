package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func packageNotificationTestUsage(t *testing.T) PackageMeteringUsage {
	t.Helper()
	binding, ok := BuildPackageMeteringBinding(PackageMeteringBindingPayload{
		V: 1, Kind: "skill", Runtime: "matcha", PackageSHA256: strings.Repeat("a", 64), Key: "notification-boundary", Use: "turn",
	})
	require.True(t, ok)
	return PackageMeteringUsage{Bindings: []string{binding}}
}

func TestPackageUsageNotificationsCrossAppliedCostThresholds(t *testing.T) {
	for _, kind := range []string{"balance", "account_quota"} {
		for _, model := range []struct {
			name string
			cost float64
		}{{"package_only", 0}, {"mixed", 0.05}} {
			t.Run(kind+"/"+model.name, func(t *testing.T) {
				smtp := startNotificationEmailTestSMTPServer(t)
				settings := newNotificationEmailMemorySettingRepo()
				values := smtp.settings()
				values[SettingKeyBalanceLowNotifyEnabled] = "true"
				values[SettingKeyBalanceLowNotifyThreshold] = "1"
				values[SettingKeyAccountQuotaNotifyEnabled] = "true"
				values[SettingKeyAccountQuotaNotifyEmails] = `[{"email":"admin@example.com","verified":true}]`
				require.NoError(t, settings.SetMultiple(context.Background(), values))
				notifier := NewBalanceNotifyService(NewEmailService(settings, nil), settings, nil)
				t.Cleanup(notifier.Stop)
				p := &postUsageBillingParams{
					Cost: &CostBreakdown{ActualCost: model.cost, TotalCost: model.cost},
					User: &User{
						ID: 1, Username: "package-user", Email: "user@example.com", Balance: 20,
						BalanceNotifyEnabled:     kind == "balance",
						BalanceNotifyExtraEmails: []NotifyEmailEntry{{Email: "user@example.com", Verified: true}},
					},
					Account:               &Account{ID: 2, Name: "package-account", Type: AccountTypeAPIKey, Platform: PlatformOpenAI},
					AccountRateMultiplier: 1,
					PackageMetering:       packageNotificationTestUsage(t),
				}
				newBalance := 0.90
				result := &UsageBillingApplyResult{BalanceCostApplied: 0.25, NewBalance: &newBalance}
				if kind == "account_quota" {
					p.Account.Extra = map[string]any{
						"quota_limit": 10.0, "quota_notify_total_enabled": true, "quota_notify_total_threshold": 1.0,
					}
					result.AccountQuotaCostApplied = 0.25
					result.QuotaState = &AccountQuotaState{TotalUsed: 9.10, TotalLimit: 10}
				}

				// Only the applied charge crosses the threshold; raw model cost misses it.
				notifier.submitUsageNotifications(p, &billingDeps{balanceNotifyService: notifier}, result, result.BalanceCostApplied)
				require.Eventually(t, func() bool { return smtp.messageCount() == 1 }, 3*time.Second, 5*time.Millisecond,
					"the real bounded notification worker must deliver a threshold email")
				notifier.Stop()
				require.EqualValues(t, 1, smtp.messageCount())
				body := smtp.lastMessageBody(t)
				require.Contains(t, body, "$0.90")
				require.Contains(t, body, "$1.00")
				if kind == "balance" {
					require.Contains(t, body, "your balance is running low")
				} else {
					require.Contains(t, body, "Account Quota Alert")
					require.Contains(t, body, "$9.10")
					require.Contains(t, body, "$10.00")
				}
			})
		}
	}
}

type packagePlatformQuotaCall struct {
	userID   int64
	platform string
	cost     float64
}

type packagePlatformQuotaCache struct {
	BillingCache
	calls chan packagePlatformQuotaCall
}

func (c *packagePlatformQuotaCache) GetUserPlatformQuotaCache(context.Context, int64, string) (*UserPlatformQuotaCacheEntry, bool, error) {
	limit := 10.0
	return &UserPlatformQuotaCacheEntry{DailyLimitUSD: &limit}, true, nil
}

func (c *packagePlatformQuotaCache) IncrUserPlatformQuotaUsageCache(_ context.Context, userID int64, platform string, cost float64, _ time.Duration, _ bool) error {
	c.calls <- packagePlatformQuotaCall{userID: userID, platform: platform, cost: cost}
	return nil
}

func (*packagePlatformQuotaCache) DeductUserBalance(context.Context, int64, float64) error {
	return nil
}

type packagePlatformQuotaRepo struct {
	UserPlatformQuotaRepository
	calls   chan packagePlatformQuotaCall
	release chan struct{}
}

func (r *packagePlatformQuotaRepo) IncrementUsageWithReset(ctx context.Context, userID int64, platform string, cost float64, _ time.Time) error {
	r.calls <- packagePlatformQuotaCall{userID: userID, platform: platform, cost: cost}
	select {
	case <-r.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestPackagePlatformQuotaPersistsAppliedBalanceCostInline(t *testing.T) {
	for _, model := range []struct {
		name string
		cost float64
	}{{"package_only", 0}, {"mixed", 0.05}} {
		t.Run(model.name, func(t *testing.T) {
			cache := &packagePlatformQuotaCache{calls: make(chan packagePlatformQuotaCall, 1)}
			repo := &packagePlatformQuotaRepo{calls: make(chan packagePlatformQuotaCall, 1), release: make(chan struct{})}
			pool := NewUsageRecordWorkerPoolWithOptions(UsageRecordWorkerPoolOptions{
				WorkerCount: 1, QueueSize: 8, TaskTimeout: time.Second, OverflowPolicy: "sync",
			})
			t.Cleanup(pool.Stop)
			t.Cleanup(func() { close(repo.release) })
			cfg := &config.Config{}
			finished := make(chan struct{})
			packageUsage := packageNotificationTestUsage(t)
			pool.Submit(func(ctx context.Context) {
				defer close(finished)
				finalizePostUsageBilling(ctx, &postUsageBillingParams{
					Cost: &CostBreakdown{ActualCost: model.cost, TotalCost: model.cost},
					User: &User{ID: 7}, Account: &Account{ID: 8}, Platform: PlatformOpenAI,
					PackageMetering: packageUsage,
				}, &billingDeps{
					cfg: cfg, userPlatformQuotaRepo: repo,
					billingCacheService: &BillingCacheService{cache: cache, cfg: cfg},
					deferredService:     &DeferredService{},
				}, &UsageBillingApplyResult{BalanceCostApplied: 0.25})
			})
			want := packagePlatformQuotaCall{userID: 7, platform: PlatformOpenAI, cost: 0.25}
			select {
			case got := <-cache.calls:
				require.Equal(t, want, got, "cache must count the committed package + model amount")
			case <-time.After(2 * time.Second):
				t.Fatal("platform quota cache was not updated")
			}
			select {
			case got := <-repo.calls:
				require.Equal(t, want, got, "database must persist the same applied amount")
			case <-time.After(2 * time.Second):
				t.Fatal("platform quota database persistence did not start")
			}
			next := make(chan struct{})
			pool.Submit(func(context.Context) { close(next) })
			select {
			case <-finished:
				t.Fatal("finalize returned before its quota persistence completed")
			case <-next:
				t.Fatal("quota persistence released its usage worker slot early")
			case <-time.After(20 * time.Millisecond):
			}
		})
	}
}

type packageLegacyQuotaRepo struct {
	AccountRepository
	calls []float64
}

func (r *packageLegacyQuotaRepo) IncrementQuotaUsed(_ context.Context, _ int64, cost float64) error {
	r.calls = append(r.calls, cost)
	return nil
}

func TestPackageLegacyZeroModelCostDoesNotWriteAccountQuota(t *testing.T) {
	repo := &packageLegacyQuotaRepo{}
	p := &postUsageBillingParams{
		Cost: &CostBreakdown{}, User: &User{ID: 1}, APIKey: &APIKey{ID: 2},
		Account:               &Account{ID: 3, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 10.0}},
		AccountRateMultiplier: 1,
		PackageMetering:       packageNotificationTestUsage(t),
	}
	require.True(t, p.shouldUpdateAccountQuota(), "the unified path must allow package-only metering")
	postUsageBilling(context.Background(), p, &billingDeps{accountRepo: repo})
	require.Empty(t, repo.calls, "legacy billing does not resolve package prices and must not write a zero quota increment")
}
