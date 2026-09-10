//go:build unit

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func packageBatchCommand(t *testing.T, requestID string, cost float64, subscription bool) *service.UsageBillingCommand {
	t.Helper()
	binding := testPackageMeteringBinding(t, service.PackageMeteringBindingPayload{
		V: 1, Kind: model.MatchaPackageKindSkill, Runtime: "openclaw",
		PackageSHA256: strings.Repeat("a", 64), Key: "skill:batch", Use: "turn",
	})
	cmd := &service.UsageBillingCommand{
		RequestID: requestID, UserID: 42, APIKeyID: 7, AccountID: 3,
		AccountType: service.AccountTypeAPIKey, BillingType: service.BillingTypeBalance,
		BalanceCost: cost, APIKeyQuotaCost: cost, APIKeyRateLimitCost: cost, AccountQuotaCost: cost,
		ApplyAPIKeyQuota: true, ApplyAPIKeyRateLimit: true, ApplyAccountQuota: true,
		PackageMetering: service.PackageMeteringUsage{Bindings: []string{binding}},
	}
	if subscription {
		id := int64(9)
		cmd.SubscriptionID, cmd.BillingType = &id, service.BillingTypeSubscription
		cmd.SubscriptionCost, cmd.BalanceCost = cost, 0
	}
	cmd.Normalize()
	return cmd
}

func expectSinglePackageClaim(mock sqlmock.Sqlmock, cmd *service.UsageBillingCommand) {
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup`).
		WithArgs(cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT request_fingerprint\s+FROM usage_billing_dedup_archive`).
		WithArgs(cmd.RequestID, cmd.APIKeyID).WillReturnError(sql.ErrNoRows)
}

func expectBatchPackageClaim(mock sqlmock.Sqlmock, cmds []*service.UsageBillingCommand) {
	ordered := append([]*service.UsageBillingCommand(nil), cmds...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].APIKeyID != ordered[j].APIKeyID {
			return ordered[i].APIKeyID < ordered[j].APIKeyID
		}
		return ordered[i].RequestID < ordered[j].RequestID
	})
	ids, keys, fingerprints := []string{}, []int64{}, []string{}
	rows := sqlmock.NewRows([]string{"id", "request_id", "api_key_id"})
	for i, cmd := range ordered {
		ids, keys, fingerprints = append(ids, cmd.RequestID), append(keys, cmd.APIKeyID), append(fingerprints, cmd.RequestFingerprint)
		rows.AddRow(i+1, cmd.RequestID, cmd.APIKeyID)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup`).
		WithArgs(pq.Array(ids), pq.Array(keys), pq.Array(fingerprints)).WillReturnRows(rows)
	mock.ExpectQuery(`(?s)SELECT a.request_id, a.api_key_id, a.request_fingerprint\s+FROM usage_billing_dedup_archive`).
		WillReturnRows(sqlmock.NewRows([]string{"request_id", "api_key_id", "request_fingerprint"}))
}

func expectBatchPackageUsage(mock sqlmock.Sqlmock, cmd *service.UsageBillingCommand, inserted bool) {
	expectPackageMeteringLookup(mock, strings.Repeat("a", 64), model.MatchaPackageKindSkill, "openclaw", cmd.UserID, "skill:batch").
		WillReturnRows(sqlmock.NewRows([]string{"id", "package_id", "owner_user_id", "price_amount", "price_currency", "entitlement_id"}).
			AddRow(11, 5, 8, "0.25000000", "USD", 13))
	query := mock.ExpectQuery(`(?s)INSERT INTO matcha_package_usage_records`).
		WithArgs(int64(5), int64(11), cmd.UserID, int64(13), cmd.RequestID, cmd.PackageMetering.Bindings[0], "turn", "0.25000000", "USD")
	if !inserted {
		query.WillReturnError(sql.ErrNoRows)
		return
	}
	query.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(17))
	mock.ExpectExec(`(?s)INSERT INTO matcha_package_revenue_ledger`).
		WithArgs(int64(5), int64(11), int64(17), int64(8), cmd.UserID, "usage", "0.25000000", "USD", "pending").
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func expectSinglePackageEffects(mock sqlmock.Sqlmock, cmd *service.UsageBillingCommand, amount, balance, used float64) {
	if cmd.SubscriptionID != nil {
		mock.ExpectExec(`(?s)UPDATE user_subscriptions us`).WithArgs(amount, *cmd.SubscriptionID).
			WillReturnResult(sqlmock.NewResult(0, 1))
	} else {
		mock.ExpectQuery(conditionalBalanceDeductSQL).WithArgs(amount, cmd.UserID).
			WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(balance))
	}
	mock.ExpectQuery(apiKeyQuotaIncrementSQL).
		WithArgs(amount, cmd.APIKeyID, service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).
		WillReturnRows(sqlmock.NewRows([]string{"exhausted"}).AddRow(false))
	mock.ExpectExec(apiKeyRateLimitIncrementSQL).WithArgs(amount, cmd.APIKeyID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(accountQuotaIncrementSQL).WithArgs(amount, cmd.AccountID).
		WillReturnRows(sqlmock.NewRows(accountQuotaStateColumns).AddRow(used, 100, 0, 0, 0, 0))
}

func expectBatchedPackageEffects(mock sqlmock.Sqlmock, total, balance float64, subscription bool) {
	if subscription {
		mock.ExpectExec(`(?s)UPDATE user_subscriptions us`).WithArgs(total, int64(9)).
			WillReturnResult(sqlmock.NewResult(0, 1))
	} else {
		mock.ExpectQuery(overdraftBalanceDeductSQL).WithArgs(decimal.NewFromFloat(total).String(), int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(balance))
	}
	mock.ExpectQuery(apiKeyQuotaIncrementSQL).
		WithArgs(decimal.NewFromFloat(total).String(), int64(7), service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).
		WillReturnRows(sqlmock.NewRows([]string{"quota", "quota_used"}).AddRow(100, total))
	mock.ExpectExec(apiKeyRateLimitIncrementSQL).WithArgs(total, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(accountQuotaIncrementSQL).WithArgs(decimal.NewFromFloat(total).String(), int64(3)).
		WillReturnRows(sqlmock.NewRows(accountQuotaStateColumns).AddRow(total, 100, 0, 0, 0, 0))
}

func TestUsageBillingPackageBatchMatchesSingleApply(t *testing.T) {
	for _, subscription := range []bool{false, true} {
		for _, modelCost := range []float64{0, 0.05} {
			t.Run(fmt.Sprintf("subscription=%v/model=%v", subscription, modelCost), func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				defer db.Close()
				repo := &usageBillingRepository{db: db}
				cmds := []*service.UsageBillingCommand{
					packageBatchCommand(t, "package-0", modelCost, subscription),
					packageBatchCommand(t, "package-1", modelCost, subscription),
				}
				originals := []service.UsageBillingCommand{*cmds[0], *cmds[1]}
				amount := service.QuantizeUsageBillingAmount(modelCost + 0.25)
				var single []*service.UsageBillingApplyResult
				for i, cmd := range cmds {
					expectSinglePackageClaim(mock, cmd)
					expectBatchPackageUsage(mock, cmd, true)
					expectSinglePackageEffects(mock, cmd, amount, 10-float64(i+1)*amount, float64(i+1)*amount)
					mock.ExpectCommit()
					res, err := repo.Apply(context.Background(), cmd)
					require.NoError(t, err)
					require.Equal(t, originals[i], *cmd, "transaction must not add package costs to the original command")
					single = append(single, res)
				}
				expectBatchPackageClaim(mock, cmds)
				for _, cmd := range cmds {
					expectBatchPackageUsage(mock, cmd, true)
				}
				expectBatchedPackageEffects(mock, amount*2, 10-amount*2, subscription)
				mock.ExpectCommit()
				outcomes, err := repo.ApplyBatch(context.Background(), cmds)
				require.NoError(t, err)
				require.Len(t, outcomes, len(cmds))
				var actualBalance, actualSubscription, actualRate, actualAccount float64
				for i, out := range outcomes {
					require.NoError(t, out.Err)
					require.Equal(t, single[i], out.Result)
					require.Equal(t, originals[i], *cmds[i])
					require.Equal(t, 0.25, out.Result.PackageMeteringCost)
					actualBalance += out.Result.BalanceCostApplied
					actualSubscription += out.Result.SubscriptionCostApplied
					actualRate += out.Result.APIKeyRateLimitCostApplied
					actualAccount += out.Result.AccountQuotaCostApplied
				}
				require.InDelta(t, amount*2, actualBalance+actualSubscription, 1e-9)
				require.InDelta(t, amount*2, actualRate, 1e-9)
				require.InDelta(t, amount*2, actualAccount, 1e-9)
				require.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

func TestUsageBillingPackageBatchRetryDoesNotRecordPackages(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	cmd := packageBatchCommand(t, "package-retry", 0, false)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "request_id", "api_key_id"}))
	mock.ExpectQuery(`(?s)SELECT d.request_id, d.api_key_id, d.request_fingerprint`).
		WillReturnRows(sqlmock.NewRows([]string{"request_id", "api_key_id", "request_fingerprint"}).
			AddRow(cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint))
	mock.ExpectCommit()
	outcomes, err := (&usageBillingRepository{db: db}).ApplyBatch(context.Background(), []*service.UsageBillingCommand{cmd})
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.NoError(t, outcomes[0].Err)
	require.Equal(t, &service.UsageBillingApplyResult{Applied: false}, outcomes[0].Result)
	require.NoError(t, mock.ExpectationsWereMet(), "no package queries or side effects are expected for a duplicate")
}

func TestUsageBillingPackageBatchExistingPackageUsageDoesNotChargeTwice(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	cmd := packageBatchCommand(t, "package-already-recorded", 0, false)
	expectBatchPackageClaim(mock, []*service.UsageBillingCommand{cmd})
	expectBatchPackageUsage(mock, cmd, false)
	mock.ExpectCommit()
	outcomes, err := (&usageBillingRepository{db: db}).ApplyBatch(context.Background(), []*service.UsageBillingCommand{cmd})
	require.NoError(t, err)
	require.True(t, outcomes[0].Result.Applied)
	require.Zero(t, outcomes[0].Result.PackageMeteringCost)
	require.Zero(t, outcomes[0].Result.BalanceCostApplied)
	require.Nil(t, outcomes[0].Result.NewBalance)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingPackageBatchRollbackCanRetryOriginalCommands(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &usageBillingRepository{db: db}
	cmds := []*service.UsageBillingCommand{
		packageBatchCommand(t, "rollback-0", 0, false),
		packageBatchCommand(t, "rollback-1", 0.05, false),
	}
	originals := []service.UsageBillingCommand{*cmds[0], *cmds[1]}
	expectBatchPackageClaim(mock, cmds)
	for _, cmd := range cmds {
		expectBatchPackageUsage(mock, cmd, true)
	}
	mock.ExpectQuery(overdraftBalanceDeductSQL).WithArgs("0.55", int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(9.45))
	errCounter := errors.New("counter write failed after package and balance writes")
	mock.ExpectQuery(apiKeyQuotaIncrementSQL).
		WithArgs("0.55", int64(7), service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).
		WillReturnError(errCounter)
	mock.ExpectRollback()
	outcomes, err := repo.ApplyBatch(context.Background(), cmds)
	require.ErrorIs(t, err, errCounter)
	require.Nil(t, outcomes)
	for i, cmd := range cmds {
		require.Equal(t, originals[i], *cmd)
		amount := service.QuantizeUsageBillingAmount(cmd.BalanceCost + 0.25)
		expectSinglePackageClaim(mock, cmd)
		expectBatchPackageUsage(mock, cmd, true)
		expectSinglePackageEffects(mock, cmd, amount, 10-float64(i)*0.25-amount, float64(i)*0.25+amount)
		mock.ExpectCommit()
		res, err := repo.Apply(context.Background(), cmd)
		require.NoError(t, err)
		require.Equal(t, amount, res.BalanceCostApplied, "rollback fallback adds the package cost only once")
		require.Equal(t, originals[i], *cmd)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}
