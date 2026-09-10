//go:build integration

package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestUsageBillingRepositoryApply_PackageCostsWithQuotaHints(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	packages := NewMatchaPackageRepository(integrationDB)
	for _, costs := range []struct {
		name    string
		balance float64
		account float64
	}{{"package_only", 0, 0}, {"mixed", 0.05, 0.10}} {
		for _, quota := range []struct {
			name       string
			daily      bool
			weekly     bool
			staleHints bool
		}{
			{name: "total_only"},
			{name: "daily", daily: true},
			{name: "weekly", weekly: true},
			{name: "daily_weekly", daily: true, weekly: true},
			{name: "stale_hints", daily: true, weekly: true, staleHints: true},
		} {
			t.Run(costs.name+"/"+quota.name, func(t *testing.T) {
				id := uuid.NewString()
				user := mustCreateUser(t, client, &service.User{
					Email: fmt.Sprintf("package-billing-%s@example.com", id), PasswordHash: "hash", Balance: 10,
				})
				key := mustCreateApiKey(t, client, &service.APIKey{
					UserID: user.ID, Key: "sk-package-billing-" + id, Name: "package billing", Quota: 10, RateLimit5h: 10,
				})
				extra := map[string]any{"quota_limit": 100.0}
				now := time.Now().UTC().Format(time.RFC3339Nano)
				if quota.daily {
					extra["quota_daily_limit"], extra["quota_daily_used"], extra["quota_daily_start"] = 1.0, 0.90, now
				}
				if quota.weekly {
					extra["quota_weekly_limit"], extra["quota_weekly_used"], extra["quota_weekly_start"] = 1.0, 0.90, now
				}
				account := mustCreateAccount(t, client, &service.Account{
					Name: "package-billing-" + id, Type: service.AccountTypeAPIKey, Extra: extra,
				})
				pkg, err := packages.CreatePackage(ctx, &model.MatchaPackage{
					PackageKey: "package-billing-" + id, Name: "Package billing", Status: model.MatchaPackageStatusActive,
				})
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, packages.DeletePackage(ctx, pkg.ID)) })
				version, err := packages.CreateVersion(ctx, &model.MatchaPackageVersion{
					PackageID: pkg.ID, Version: "1.0.0", Kind: model.MatchaPackageKindSkill, Runtime: "openclaw",
					SHA256: strings.Repeat("a", 64), SizeBytes: 1, ObjectKey: "package-billing/" + id,
					Status: model.MatchaPackageVersionStatusPublished, OwnerUserID: &user.ID,
					PriceAmount: decimal.RequireFromString("0.25"), PriceCurrency: "USD",
				})
				require.NoError(t, err)
				_, err = packages.GrantEntitlement(ctx, &model.MatchaPackageEntitlement{
					PackageID: pkg.ID, VersionID: &version.ID, UserID: user.ID,
				})
				require.NoError(t, err)
				binding, ok := service.BuildPackageMeteringBinding(service.PackageMeteringBindingPayload{
					V: 1, Kind: version.Kind, Runtime: version.Runtime, PackageSHA256: version.SHA256, Key: pkg.PackageKey, Use: "turn",
				})
				require.True(t, ok)
				command := func() *service.UsageBillingCommand {
					return &service.UsageBillingCommand{
						RequestID: id, UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID,
						AccountType: account.Type, BillingType: service.BillingTypeBalance,
						BalanceCost: costs.balance, AccountQuotaCost: costs.account,
						APIKeyQuotaCost: costs.balance, APIKeyRateLimitCost: costs.balance,
						ApplyAPIKeyQuota: true, ApplyAPIKeyRateLimit: true, ApplyAccountQuota: true,
						AccountQuotaHasDailyLimit:  quota.daily && !quota.staleHints,
						AccountQuotaHasWeeklyLimit: quota.weekly && !quota.staleHints,
						PackageMetering:            service.PackageMeteringUsage{Bindings: []string{binding}},
					}
				}
				result, err := repo.Apply(ctx, command())
				require.NoError(t, err)
				require.True(t, result.Applied)
				balanceCost, accountCost := costs.balance+0.25, costs.account+0.25
				require.InDelta(t, 0.25, result.PackageMeteringCost, 0.000001)
				require.InDelta(t, balanceCost, result.BalanceCostApplied, 0.000001)
				require.InDelta(t, accountCost, result.AccountQuotaCostApplied, 0.000001)
				require.NotNil(t, result.NewBalance)
				require.InDelta(t, 10-balanceCost, *result.NewBalance, 0.000001)
				require.NotNil(t, result.QuotaState)
				require.InDelta(t, accountCost, result.QuotaState.TotalUsed, 0.000001)
				if quota.daily {
					require.InDelta(t, 0.90+accountCost, result.QuotaState.DailyUsed, 0.000001)
				}
				if quota.weekly {
					require.InDelta(t, 0.90+accountCost, result.QuotaState.WeeklyUsed, 0.000001)
				}

				// A fresh retry must neither charge again nor duplicate package revenue/outbox.
				retry, err := repo.Apply(ctx, command())
				require.NoError(t, err)
				require.False(t, retry.Applied)
				var balance, totalUsed, dailyUsed, weeklyUsed, keyUsed, key5h float64
				var hasDaily, hasWeekly bool
				require.NoError(t, integrationDB.QueryRowContext(ctx,
					"SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
				require.InDelta(t, 10-balanceCost, balance, 0.000001)
				require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT
					COALESCE((extra->>'quota_used')::numeric, 0),
					COALESCE((extra->>'quota_daily_used')::numeric, 0),
					COALESCE((extra->>'quota_weekly_used')::numeric, 0),
					extra ? 'quota_daily_used', extra ? 'quota_weekly_used'
					FROM accounts WHERE id = $1`, account.ID).Scan(&totalUsed, &dailyUsed, &weeklyUsed, &hasDaily, &hasWeekly))
				require.InDelta(t, accountCost, totalUsed, 0.000001)
				require.Equal(t, quota.daily, hasDaily)
				require.Equal(t, quota.weekly, hasWeekly)
				if quota.daily {
					require.InDelta(t, 0.90+accountCost, dailyUsed, 0.000001)
				}
				if quota.weekly {
					require.InDelta(t, 0.90+accountCost, weeklyUsed, 0.000001)
				}
				require.NoError(t, integrationDB.QueryRowContext(ctx,
					"SELECT quota_used, usage_5h FROM api_keys WHERE id = $1", key.ID).Scan(&keyUsed, &key5h))
				require.InDelta(t, balanceCost, keyUsed, 0.000001)
				require.InDelta(t, balanceCost, key5h, 0.000001)
				var usageCount, ledgerCount, outboxCount int
				var packageAmount, revenueAmount float64
				require.NoError(t, integrationDB.QueryRowContext(ctx,
					"SELECT COUNT(*), COALESCE(SUM(amount), 0) FROM matcha_package_usage_records WHERE version_id = $1 AND user_id = $2", version.ID, user.ID).Scan(&usageCount, &packageAmount))
				require.Equal(t, 1, usageCount)
				require.InDelta(t, 0.25, packageAmount, 0.000001)
				require.NoError(t, integrationDB.QueryRowContext(ctx,
					"SELECT COUNT(*), COALESCE(SUM(amount), 0) FROM matcha_package_revenue_ledger WHERE version_id = $1 AND user_id = $2", version.ID, user.ID).Scan(&ledgerCount, &revenueAmount))
				require.Equal(t, 1, ledgerCount)
				require.InDelta(t, 0.25, revenueAmount, 0.000001)
				require.NoError(t, integrationDB.QueryRowContext(ctx,
					"SELECT COUNT(*) FROM scheduler_outbox WHERE event_type = $1 AND account_id = $2", service.SchedulerOutboxEventAccountChanged, account.ID).Scan(&outboxCount))
				wantOutbox := 0
				if quota.daily || quota.weekly {
					wantOutbox = 1
				}
				require.Equal(t, wantOutbox, outboxCount)
			})
		}
	}
}
