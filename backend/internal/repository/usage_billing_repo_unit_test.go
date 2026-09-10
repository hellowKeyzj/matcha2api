//go:build unit

package repository

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	conditionalBalanceDeductSQL = `(?s)UPDATE users\s+SET balance = balance - \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL AND balance >= \$1\s+RETURNING balance`
	overdraftBalanceDeductSQL   = `(?s)UPDATE users\s+SET balance = balance - \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL\s+RETURNING balance`
	reserveBatchImageHoldSQL    = `(?s)UPDATE users\s+SET balance = balance - \$1,\s+frozen_balance = COALESCE\(frozen_balance, 0\) \+ \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL AND balance >= \$1\s+RETURNING balance, frozen_balance`
	captureBatchImageHoldSQL    = `(?s)UPDATE users\s+SET balance = balance\s+\+ CASE WHEN \$1 > \$2 THEN \$1 - \$2 ELSE 0 END\s+- CASE WHEN \$2 > \$1 THEN \$2 - \$1 ELSE 0 END,\s+frozen_balance = COALESCE\(frozen_balance, 0\) - \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$3 AND deleted_at IS NULL AND COALESCE\(frozen_balance, 0\) >= \$1\s+RETURNING balance, frozen_balance`
	releaseBatchImageHoldSQL    = `(?s)UPDATE users\s+SET balance = balance \+ \$1,\s+frozen_balance = COALESCE\(frozen_balance, 0\) - \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL AND COALESCE\(frozen_balance, 0\) >= \$1\s+RETURNING balance, frozen_balance`
	userExistsForBillingSQL     = `(?s)SELECT 1\s+FROM users\s+WHERE id = \$1 AND deleted_at IS NULL`
	apiKeyQuotaIncrementSQL     = `(?s)UPDATE api_keys\s+SET quota_used = quota_used \+ \$1,.*WHERE id = \$2 AND deleted_at IS NULL\s+RETURNING`
	apiKeyRateLimitIncrementSQL = `(?s)UPDATE api_keys SET\s+usage_5h = .*WHERE id = \$2 AND deleted_at IS NULL`
)

func TestDeductUsageBillingBalance_UsesSufficientBalanceGuard(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(7.5))
	mock.ExpectCommit()

	newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, 42, 2.5)
	require.NoError(t, err)
	require.True(t, sufficient)
	require.InDelta(t, 7.5, newBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_RecordsOverdraftWhenGuardMisses(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(overdraftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-5.0))
	mock.ExpectCommit()

	newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, 42, 10)
	require.NoError(t, err)
	require.False(t, sufficient)
	require.InDelta(t, -5.0, newBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffects_FlagsBalanceOverdraft(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(overdraftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-5.0))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:      42,
		BalanceCost: 10,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, -5.0, *result.NewBalance, 0.000001)
	require.True(t, result.BalanceOverdrafted)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffects_DeletedAPIKeyStillBillsBalance(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(90.0))
	mock.ExpectQuery(apiKeyQuotaIncrementSQL).
		WithArgs(10.0, int64(7), service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(apiKeyRateLimitIncrementSQL).
		WithArgs(10.0, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:              42,
		APIKeyID:            7,
		BalanceCost:         10,
		APIKeyQuotaCost:     10,
		APIKeyRateLimitCost: 10,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, 90.0, *result.NewBalance, 0.000001)
	require.False(t, result.APIKeyQuotaExhausted)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffects_APIKeyCounterErrorStillFails(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectExec(apiKeyRateLimitIncrementSQL).
		WithArgs(10.0, int64(7)).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:              42,
		APIKeyID:            7,
		APIKeyRateLimitCost: 10,
	}, &service.UsageBillingApplyResult{Applied: true})
	require.ErrorIs(t, err, sql.ErrConnDone)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_ReturnsUserNotFoundWhenNoUserUpdated(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(overdraftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, _, err = deductUsageBillingBalance(ctx, tx, 42, 10)
	require.ErrorIs(t, err, service.ErrUserNotFound)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveUsageBillingBatchImageBalance_MovesAvailableToFrozen(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(reserveBatchImageHoldSQL).
		WithArgs(2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(7.5, 2.5))
	mock.ExpectCommit()

	result, err := reserveUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 2.5})
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.NotNil(t, result.FrozenBalance)
	require.InDelta(t, 7.5, *result.NewBalance, 0.000001)
	require.InDelta(t, 2.5, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveUsageBillingBatchImageBalance_InsufficientBalance(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(reserveBatchImageHoldSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(userExistsForBillingSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	mock.ExpectRollback()

	_, err = reserveUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 10})
	require.ErrorIs(t, err, service.ErrBatchImageInsufficientBalance)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_ReleasesRemainder(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(captureBatchImageHoldSQL).
		WithArgs(1.0, 0.25, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(9.75, 0.0))
	mock.ExpectCommit()

	result, err := captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 1, ActualAmount: 0.25})
	require.NoError(t, err)
	require.InDelta(t, 9.75, *result.NewBalance, 0.000001)
	require.InDelta(t, 0.0, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_RejectsActualCostOverHold(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectRollback()

	_, err = captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 0.5, ActualAmount: 1})
	require.ErrorIs(t, err, service.ErrBatchImageSettlementCostExceedsHold)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUsageBillingBatchImageBalance_ReturnsFrozenToAvailable(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_release"), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	mock.ExpectQuery(releaseBatchImageHoldSQL).
		WithArgs(1.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(10.0, 0.0))
	mock.ExpectCommit()

	result, err := releaseUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_release", HoldAmount: 1})
	require.NoError(t, err)
	require.InDelta(t, 10.0, *result.NewBalance, 0.000001)
	require.InDelta(t, 0.0, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUsageBillingBatchImageBalance_SkipsWhenHoldNeverReserved(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	// dedup 与归档表均无 hold claim：说明该 job 从未成功冻结，
	// 释放必须跳过，不得从他人冻结资金池中凭空生成余额。
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_phantom"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup_archive\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_phantom"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectCommit()

	result, err := releaseUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_phantom", HoldAmount: 1})
	require.NoError(t, err)
	require.Nil(t, result.NewBalance)
	require.Nil(t, result.FrozenBalance)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyRecordsPackageMeteringAndChargesBalance(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	sha := strings.Repeat("a", 64)
	binding := testPackageMeteringBinding(t, service.PackageMeteringBindingPayload{
		V:             1,
		Kind:          model.MatchaPackageKindSkill,
		Runtime:       "openclaw",
		PackageSHA256: sha,
		Key:           "skill:openclaw:test",
		Use:           "turn",
	})
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup`).
		WithArgs("req-package", int64(7), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectQuery(`(?s)SELECT request_fingerprint\s+FROM usage_billing_dedup_archive`).
		WithArgs("req-package", int64(7)).
		WillReturnError(sql.ErrNoRows)
	expectPackageMeteringLookup(mock, sha, model.MatchaPackageKindSkill, "openclaw", int64(42), "skill:openclaw:test").
		WillReturnRows(sqlmock.NewRows([]string{"id", "package_id", "owner_user_id", "price_amount", "price_currency", "entitlement_id"}).
			AddRow(int64(7), int64(5), int64(9), "0.25000000", "USD", int64(13)))
	mock.ExpectQuery(`(?s)INSERT INTO matcha_package_usage_records`).
		WithArgs(int64(5), int64(7), int64(42), int64(13), "req-package", binding, "turn", "0.25000000", "USD").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(11)))
	mock.ExpectExec(`(?s)INSERT INTO matcha_package_revenue_ledger`).
		WithArgs(int64(5), int64(7), int64(11), sqlmock.AnyArg(), int64(42), "usage", "0.25000000", "USD", "pending").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(0.25, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(9.75))
	mock.ExpectCommit()

	result, err := (&usageBillingRepository{db: db}).Apply(ctx, &service.UsageBillingCommand{
		RequestID:       "req-package",
		APIKeyID:        7,
		UserID:          42,
		AccountID:       3,
		BillingType:     service.BillingTypeBalance,
		PackageMetering: service.PackageMeteringUsage{Bindings: []string{binding}},
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.InDelta(t, 0.25, result.PackageMeteringCost, 0.000001)
	require.InDelta(t, 0.25, result.BalanceCostApplied, 0.000001)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, 9.75, *result.NewBalance, 0.000001)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplySkipsPackageMeteringWithoutActiveEntitlement(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	sha := strings.Repeat("c", 64)
	binding := testPackageMeteringBinding(t, service.PackageMeteringBindingPayload{
		V:             1,
		Kind:          model.MatchaPackageKindSkill,
		Runtime:       "openclaw",
		PackageSHA256: sha,
		Key:           "skill:openclaw:test",
		Use:           "turn",
	})
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup`).
		WithArgs("req-package-unentitled", int64(7), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectQuery(`(?s)SELECT request_fingerprint\s+FROM usage_billing_dedup_archive`).
		WithArgs("req-package-unentitled", int64(7)).
		WillReturnError(sql.ErrNoRows)
	expectPackageMeteringLookup(mock, sha, model.MatchaPackageKindSkill, "openclaw", int64(42), "skill:openclaw:test").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectCommit()

	result, err := (&usageBillingRepository{db: db}).Apply(ctx, &service.UsageBillingCommand{
		RequestID:       "req-package-unentitled",
		APIKeyID:        7,
		UserID:          42,
		AccountID:       3,
		BillingType:     service.BillingTypeBalance,
		PackageMetering: service.PackageMeteringUsage{Bindings: []string{binding}},
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Zero(t, result.PackageMeteringCost)
	require.Nil(t, result.NewBalance)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyDedupesExistingPackageUsageRecord(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	sha := strings.Repeat("b", 64)
	binding := testPackageMeteringBinding(t, service.PackageMeteringBindingPayload{
		V:             1,
		Kind:          model.MatchaPackageKindAgent,
		Runtime:       "openclaw",
		PackageSHA256: sha,
		Key:           "agent:openclaw:test",
	})
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup`).
		WithArgs("req-package", int64(7), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectQuery(`(?s)SELECT request_fingerprint\s+FROM usage_billing_dedup_archive`).
		WithArgs("req-package", int64(7)).
		WillReturnError(sql.ErrNoRows)
	expectPackageMeteringLookup(mock, sha, model.MatchaPackageKindAgent, "openclaw", int64(42), "agent:openclaw:test").
		WillReturnRows(sqlmock.NewRows([]string{"id", "package_id", "owner_user_id", "price_amount", "price_currency", "entitlement_id"}).
			AddRow(int64(8), int64(6), nil, "1.00000000", "USD", int64(14)))
	mock.ExpectQuery(`(?s)INSERT INTO matcha_package_usage_records`).
		WithArgs(int64(6), int64(8), int64(42), int64(14), "req-package", binding, "session", "1.00000000", "USD").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectCommit()

	result, err := (&usageBillingRepository{db: db}).Apply(ctx, &service.UsageBillingCommand{
		RequestID:       "req-package",
		APIKeyID:        7,
		UserID:          42,
		AccountID:       3,
		BillingType:     service.BillingTypeBalance,
		PackageMetering: service.PackageMeteringUsage{Bindings: []string{binding}},
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Zero(t, result.PackageMeteringCost)
	require.Nil(t, result.NewBalance)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyPackageMeteringOnlyUpdatesUsageCounters(t *testing.T) {
	for _, keyDeleted := range []bool{false, true} {
		name := "active_key"
		if keyDeleted {
			name = "deleted_key"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			sha := strings.Repeat("d", 64)
			binding := testPackageMeteringBinding(t, service.PackageMeteringBindingPayload{
				V:             1,
				Kind:          model.MatchaPackageKindSkill,
				Runtime:       "openclaw",
				PackageSHA256: sha,
				Key:           "skill:openclaw:quota",
				Use:           "turn",
			})
			mock.ExpectBegin()
			mock.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup`).
				WithArgs("req-package-quota", int64(7), sqlmock.AnyArg()).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
			mock.ExpectQuery(`(?s)SELECT request_fingerprint\s+FROM usage_billing_dedup_archive`).
				WithArgs("req-package-quota", int64(7)).
				WillReturnError(sql.ErrNoRows)
			expectPackageMeteringLookup(mock, sha, model.MatchaPackageKindSkill, "openclaw", int64(42), "skill:openclaw:quota").
				WillReturnRows(sqlmock.NewRows([]string{"id", "package_id", "owner_user_id", "price_amount", "price_currency", "entitlement_id"}).
					AddRow(int64(9), int64(6), nil, "0.25000000", "USD", int64(15)))
			mock.ExpectQuery(`(?s)INSERT INTO matcha_package_usage_records`).
				WithArgs(int64(6), int64(9), int64(42), int64(15), "req-package-quota", binding, "turn", "0.25000000", "USD").
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(12)))
			mock.ExpectExec(`(?s)INSERT INTO matcha_package_revenue_ledger`).
				WithArgs(int64(6), int64(9), int64(12), sqlmock.AnyArg(), int64(42), "usage", "0.25000000", "USD", "pending").
				WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectQuery(conditionalBalanceDeductSQL).
				WithArgs(0.25, int64(42)).
				WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(9.75))
			quotaUpdate := mock.ExpectQuery(apiKeyQuotaIncrementSQL).
				WithArgs(0.25, int64(7), service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted)
			if keyDeleted {
				quotaUpdate.WillReturnError(sql.ErrNoRows)
			} else {
				quotaUpdate.WillReturnRows(sqlmock.NewRows([]string{"exhausted"}).AddRow(false))
			}
			rateUpdate := mock.ExpectExec(apiKeyRateLimitIncrementSQL).WithArgs(0.25, int64(7))
			if keyDeleted {
				rateUpdate.WillReturnResult(sqlmock.NewResult(0, 0))
			} else {
				rateUpdate.WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectQuery(`(?s)UPDATE accounts SET extra =`).
				WithArgs(0.25, int64(3)).
				WillReturnRows(sqlmock.NewRows([]string{"total_used", "total_limit", "daily_used", "daily_limit", "weekly_used", "weekly_limit"}).
					AddRow(0.25, 100.0, 0.0, 0.0, 0.0, 0.0))
			mock.ExpectCommit()

			result, err := (&usageBillingRepository{db: db}).Apply(ctx, &service.UsageBillingCommand{
				RequestID:            "req-package-quota",
				APIKeyID:             7,
				UserID:               42,
				AccountID:            3,
				AccountType:          service.AccountTypeAPIKey,
				BillingType:          service.BillingTypeBalance,
				ApplyAPIKeyQuota:     true,
				ApplyAPIKeyRateLimit: true,
				ApplyAccountQuota:    true,
				PackageMetering:      service.PackageMeteringUsage{Bindings: []string{binding}},
			})
			require.NoError(t, err)
			require.True(t, result.Applied)
			require.InDelta(t, 0.25, result.PackageMeteringCost, 0.000001)
			require.InDelta(t, 0.25, result.APIKeyRateLimitCostApplied, 0.000001)
			require.InDelta(t, 0.25, result.AccountQuotaCostApplied, 0.000001)
			require.NotNil(t, result.QuotaState)
			require.NotNil(t, result.NewBalance)
			require.InDelta(t, 9.75, *result.NewBalance, 0.000001)
			require.InDelta(t, 0.25, result.BalanceCostApplied, 0.000001)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func expectPackageMeteringLookup(mock sqlmock.Sqlmock, sha, kind, runtime string, userID int64, key string) *sqlmock.ExpectedQuery {
	return mock.ExpectQuery(`(?s)SELECT v\.id,\s+v\.package_id,\s+v\.owner_user_id,\s+v\.price_amount::text,\s+COALESCE\(NULLIF\(v\.price_currency, ''\), 'USD'\),\s+e\.id\s+FROM matcha_package_versions v\s+JOIN matcha_packages p ON p\.id = v\.package_id\s+JOIN matcha_package_entitlements e`).
		WithArgs(model.MatchaPackageStatusActive, model.MatchaPackageVersionStatusPublished, sha, kind, runtime, userID, key)
}

func testPackageMeteringBinding(t *testing.T, payload service.PackageMeteringBindingPayload) string {
	t.Helper()
	binding, ok := service.BuildPackageMeteringBinding(payload)
	require.True(t, ok)
	return binding
}
