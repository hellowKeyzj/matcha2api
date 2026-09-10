//go:build integration

package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
				account := newBillingAccountFixture(t, client, &service.Account{
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

func packageBatchFixtureCommands(t *testing.T, f batchBillingFixture, prefix string, steps []batchBillingStep) []*service.UsageBillingCommand {
	t.Helper()
	ctx := context.Background()
	packages := NewMatchaPackageRepository(integrationDB)
	pkg, err := packages.CreatePackage(ctx, &model.MatchaPackage{
		PackageKey: "billing-batch-" + uuid.NewString(), Name: "batch metering", Status: model.MatchaPackageStatusActive,
	})
	require.NoError(t, err)
	version, err := packages.CreateVersion(ctx, &model.MatchaPackageVersion{
		PackageID: pkg.ID, Version: "1.0.0", Kind: model.MatchaPackageKindSkill, Runtime: "openclaw",
		SHA256: strings.Repeat("a", 64), SizeBytes: 1, ObjectKey: "billing-batch/" + prefix,
		Status: model.MatchaPackageVersionStatusPublished, OwnerUserID: &f.userID,
		PriceAmount: decimal.RequireFromString("0.25"), PriceCurrency: "USD",
	})
	require.NoError(t, err)
	_, err = packages.GrantEntitlement(ctx, &model.MatchaPackageEntitlement{
		PackageID: pkg.ID, VersionID: &version.ID, UserID: f.userID,
	})
	require.NoError(t, err)
	binding, ok := service.BuildPackageMeteringBinding(service.PackageMeteringBindingPayload{
		V: 1, Kind: version.Kind, Runtime: version.Runtime, PackageSHA256: version.SHA256, Key: pkg.PackageKey, Use: "turn",
	})
	require.True(t, ok)
	cmds := f.commands(prefix, steps)
	for _, cmd := range cmds {
		cmd.BillingType = service.BillingTypeBalance
		if cmd.SubscriptionID != nil {
			cmd.BillingType = service.BillingTypeSubscription
		}
		cmd.ApplyAPIKeyQuota, cmd.ApplyAPIKeyRateLimit, cmd.ApplyAccountQuota = true, true, true
		cmd.PackageMetering = service.PackageMeteringUsage{Bindings: []string{binding}}
		cmd.Normalize()
	}
	return cmds
}

func assertPackageBatchLedger(t *testing.T, f batchBillingFixture, count int, amount float64) {
	t.Helper()
	for _, table := range []string{"matcha_package_usage_records", "matcha_package_revenue_ledger"} {
		var records int
		var total float64
		err := integrationDB.QueryRow("SELECT COUNT(*), COALESCE(SUM(amount), 0) FROM "+table+" WHERE user_id=$1", f.userID).Scan(&records, &total)
		require.NoError(t, err)
		require.Equal(t, count, records, table)
		require.InDelta(t, amount, total, 1e-9, table)
	}
}

func TestUsageBillingPackageBatch_MatchesSequentialPackageEffects(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	steps := []batchBillingStep{
		{key: 'A', account: 'P'},
		{key: 'A', account: 'P', balance: 0.05, keyQuota: 0.05, keyRate: 0.05, accountQuota: 0.10},
		{key: 'B', account: 'Q', subscription: 0.10, keyQuota: 0.10, keyRate: 0.10, accountQuota: 0.20},
	}
	sequential, batched := newBatchBillingFixture(t, client, 10), newBatchBillingFixture(t, client, 10)
	seqCmds := packageBatchFixtureCommands(t, sequential, "seq-package-"+uuid.NewString(), steps)
	batchCmds := packageBatchFixtureCommands(t, batched, "batch-package-"+uuid.NewString(), steps)
	originals := make([]service.UsageBillingCommand, len(batchCmds))
	single := make([]*service.UsageBillingApplyResult, len(seqCmds))
	for i, cmd := range seqCmds {
		original := *cmd
		var err error
		single[i], err = repo.Apply(ctx, cmd)
		require.NoError(t, err)
		require.Equal(t, original, *cmd)
		originals[i] = *batchCmds[i]
	}
	outcomes, err := repo.ApplyBatch(ctx, batchCmds)
	require.NoError(t, err)
	require.Len(t, outcomes, len(single))
	var balance, subscription, rate, account float64
	for i, out := range outcomes {
		require.NoError(t, out.Err)
		require.Equal(t, single[i], out.Result)
		require.Equal(t, originals[i], *batchCmds[i])
		require.Equal(t, 0.25, out.Result.PackageMeteringCost)
		balance += out.Result.BalanceCostApplied
		subscription += out.Result.SubscriptionCostApplied
		rate += out.Result.APIKeyRateLimitCostApplied
		account += out.Result.AccountQuotaCostApplied
	}
	require.InDelta(t, 0.55, balance, 1e-9)
	require.InDelta(t, 0.35, subscription, 1e-9)
	require.InDelta(t, 0.90, rate, 1e-9)
	require.InDelta(t, 1.05, account, 1e-9)
	seqState, batchState := readBatchBillingState(t, ctx, sequential), readBatchBillingState(t, ctx, batched)
	// JSON numeric values retain SQL parameter scale: 0.60 and 0.6 are the same amount.
	for _, state := range []*batchBillingState{&seqState, &batchState} {
		for _, amount := range []*string{&state.pUsed, &state.pDailyUsed, &state.pWeeklyUsed, &state.qUsed, &state.qDailyUsed, &state.qWeeklyUsed} {
			*amount = decimal.RequireFromString(*amount).String()
		}
	}
	require.Equal(t, seqState, batchState)
	assertPackageBatchLedger(t, batched, 3, 0.75)
	before := readBatchBillingState(t, ctx, batched)
	retry, err := repo.ApplyBatch(ctx, batchCmds)
	require.NoError(t, err)
	for _, out := range retry {
		require.NoError(t, out.Err)
		require.False(t, out.Result.Applied)
	}
	require.Equal(t, before, readBatchBillingState(t, ctx, batched))
	assertPackageBatchLedger(t, batched, 3, 0.75)
}

func TestUsageBillingPackageBatch_RollbackThenSingleFallback(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	f := newBatchBillingFixture(t, client, 10)
	cmds := packageBatchFixtureCommands(t, f, "rollback-package-"+uuid.NewString(), []batchBillingStep{
		{key: 'A', account: 'P'},
		{key: 'A', account: 'P', balance: 0.05, keyQuota: 0.05, keyRate: 0.05, accountQuota: 0.05},
	})
	originals := []service.UsageBillingCommand{*cmds[0], *cmds[1]}
	before := readBatchBillingState(t, ctx, f)
	trigger := fmt.Sprintf("billing_fail_key_%d", f.keyA)
	_, err := integrationDB.ExecContext(ctx, fmt.Sprintf(`
		CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.id=%d AND NEW.quota_used > OLD.quota_used THEN
		RAISE EXCEPTION 'injected key counter failure'; END IF; RETURN NEW; END $$;
		CREATE TRIGGER %s BEFORE UPDATE ON api_keys FOR EACH ROW EXECUTE FUNCTION %s();
	`, trigger, f.keyA, trigger, trigger))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON api_keys; DROP FUNCTION IF EXISTS %s();", trigger, trigger))
	})
	outcomes, err := repo.ApplyBatch(ctx, cmds)
	require.ErrorContains(t, err, "injected key counter failure")
	require.Nil(t, outcomes)
	require.Equal(t, before, readBatchBillingState(t, ctx, f), "balance and all counters must roll back")
	assertPackageBatchLedger(t, f, 0, 0)
	for i, cmd := range cmds {
		require.Equal(t, originals[i], *cmd)
	}
	_, err = integrationDB.ExecContext(ctx, fmt.Sprintf("DROP TRIGGER %s ON api_keys; DROP FUNCTION %s();", trigger, trigger))
	require.NoError(t, err)
	for i, cmd := range cmds {
		res, err := repo.Apply(ctx, cmd)
		require.NoError(t, err)
		require.Equal(t, 0.25+cmd.BalanceCost, res.BalanceCostApplied)
		require.Equal(t, originals[i], *cmd)
	}
	assertPackageBatchLedger(t, f, 2, 0.5)
	var balance, used float64
	require.NoError(t, integrationDB.QueryRow("SELECT balance FROM users WHERE id=$1", f.userID).Scan(&balance))
	require.NoError(t, integrationDB.QueryRow("SELECT quota_used FROM api_keys WHERE id=$1", f.keyA).Scan(&used))
	require.InDelta(t, 9.45, balance, 1e-9)
	require.InDelta(t, 0.55, used, 1e-9)
}

func TestUsageBillingPackageLane_ConcurrentPackageOnlyRequests(t *testing.T) {
	for _, failBatch := range []bool{false, true} {
		t.Run(fmt.Sprintf("batch_failure=%v", failBatch), func(t *testing.T) {
			testUsageBillingPackageLane(t, failBatch)
		})
	}
}

func testUsageBillingPackageLane(t *testing.T, failBatch bool) {
	t.Helper()
	ctx := context.Background()
	client := testEntClient(t)
	f := newBatchBillingFixture(t, client, 100)
	const n = 40
	cmds := packageBatchFixtureCommands(t, f, "lane-package-"+uuid.NewString(), make([]batchBillingStep, n))
	if failBatch {
		trigger := fmt.Sprintf("billing_fail_batch_%d", f.keyA)
		_, err := integrationDB.ExecContext(ctx, fmt.Sprintf(`
			CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN IF NEW.id=%d AND NEW.quota_used - OLD.quota_used > 0.30 THEN
			RAISE EXCEPTION 'injected batch failure'; END IF; RETURN NEW; END $$;
			CREATE TRIGGER %s BEFORE UPDATE ON api_keys FOR EACH ROW EXECUTE FUNCTION %s();
		`, trigger, f.keyA, trigger, trigger))
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = integrationDB.Exec(fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON api_keys; DROP FUNCTION IF EXISTS %s();", trigger, trigger))
		})
	}
	lanes := service.NewUsageBillingLaneCombiner(NewUsageBillingRepository(client, integrationDB))
	defer lanes.Stop()
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, n)
	results := make([]*service.UsageBillingApplyResult, n)
	for i, cmd := range cmds {
		wg.Add(1)
		go func(i int, cmd *service.UsageBillingCommand) {
			defer wg.Done()
			<-start
			results[i], errs[i] = lanes.Apply(ctx, cmd)
		}(i, cmd)
	}
	close(start)
	wg.Wait()
	for i := range cmds {
		require.NoError(t, errs[i])
		require.True(t, results[i].Applied)
		require.Equal(t, 0.25, results[i].BalanceCostApplied)
		require.Zero(t, cmds[i].BalanceCost)
	}
	stats := lanes.Stats()
	require.Positive(t, stats.Batches, "package requests must use the official batch path")
	if failBatch {
		require.Positive(t, stats.BatchFallbacks, "real rolled-back batches must retry original commands through the lane")
	} else {
		require.Zero(t, stats.BatchFallbacks)
	}
	require.Equal(t, uint64(n), stats.SingleApplies+stats.BatchedCommands)
	assertPackageBatchLedger(t, f, n, n*0.25)
	var balance float64
	require.NoError(t, integrationDB.QueryRow("SELECT balance FROM users WHERE id=$1", f.userID).Scan(&balance))
	require.Equal(t, 90.0, balance)
}
