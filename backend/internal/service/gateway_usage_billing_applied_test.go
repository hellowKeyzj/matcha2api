package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFinalizePostUsageBilling_HonorsAppliedCosts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		quoted  float64
		applied float64
		legacy  bool
	}{
		{name: "rounded_to_zero", quoted: 0.0000000049},
		{name: "rounded_up", quoted: 0.000078125, applied: 0.00007813},
		{name: "package_charge", quoted: 0.05, applied: 0.55},
		{name: "without_transaction_result", quoted: 0.05, applied: 0.05, legacy: true},
	} {
		for _, source := range []BillingSource{BillingSourceBalance, BillingSourceSubscription} {
			t.Run(tc.name+"/"+string(source), func(t *testing.T) {
				groupID := int64(3)
				p := &postUsageBillingParams{
					Cost:               &CostBreakdown{ActualCost: tc.quoted},
					User:               &User{ID: 1},
					APIKey:             &APIKey{ID: 2, GroupID: &groupID, RateLimit1d: 10},
					Account:            &Account{ID: 4},
					IsSubscriptionBill: source.IsSubscription(),
				}
				var result *UsageBillingApplyResult
				kind := cacheWriteDeductBalance
				if !tc.legacy {
					result = &UsageBillingApplyResult{Applied: true, APIKeyRateLimitCostApplied: tc.applied}
					if source.IsSubscription() {
						result.SubscriptionCostApplied = tc.applied
					} else {
						result.BalanceCostApplied = tc.applied
					}
				}
				if source.IsSubscription() {
					kind = cacheWriteUpdateSubscriptionUsage
				}
				writes := make(chan cacheWriteTask, 2)
				finalizePostUsageBilling(context.Background(), p, &billingDeps{
					billingCacheService: &BillingCacheService{cache: &billingCacheWorkerStub{}, cacheWriteChan: writes},
					deferredService:     &DeferredService{},
				}, result)
				if tc.applied == 0 {
					require.Empty(t, writes, "a zero committed charge must not increase cached usage")
					return
				}
				require.Len(t, writes, 2)
				charge := <-writes
				require.Equal(t, kind, charge.kind)
				require.Equal(t, tc.applied, charge.amount)
				rateLimit := <-writes
				require.Equal(t, cacheWriteUpdateRateLimitUsage, rateLimit.kind)
				require.Equal(t, tc.applied, rateLimit.amount)
			})
		}
	}
}

func TestResolvedAccountQuotaCost_HonorsAppliedCosts(t *testing.T) {
	p := &postUsageBillingParams{
		Cost:                  &CostBreakdown{TotalCost: 0.0000000049},
		AccountRateMultiplier: 1,
	}
	for _, applied := range []float64{0, 0.25} {
		result := &UsageBillingApplyResult{Applied: true, AccountQuotaCostApplied: applied}
		require.Equal(t, applied, resolvedAccountQuotaCost(p, result),
			"the transaction result, including zero, owns the notification amount")
	}
	require.Equal(t, p.Cost.TotalCost, resolvedAccountQuotaCost(p, nil),
		"legacy billing without a transaction result still uses the model cost")
}
