package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSelectGrokMediaVideoRequestAccountPreservesOwner(t *testing.T) {
	for _, state := range []string{"available", "full", "unavailable", "wrong group", "missing", "invalid id"} {
		t.Run(state, func(t *testing.T) {
			groupID := int64(24)
			ownerID := int64(1)
			owner := Account{ID: ownerID, Platform: PlatformGrok, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true, Concurrency: 50, GroupIDs: []int64{groupID}}
			other := owner
			other.ID = 2
			if state == "unavailable" {
				until := time.Now().Add(time.Minute)
				owner.TempUnschedulableUntil = &until
			}
			if state == "wrong group" {
				owner.GroupIDs = []int64{25}
			}
			accounts := []Account{owner, other}
			if state == "missing" {
				accounts = []Account{other}
			}
			if state == "invalid id" {
				ownerID = 0
			}
			var acquired, released []int64
			cache := &schedulerTestGatewayCache{}
			cfg := &config.Config{}
			cfg.Gateway.OpenAIScheduler.StickyEscapeEnabled = true
			cfg.Gateway.Scheduling.StickySessionWaitTimeout = time.Second
			cfg.Gateway.Scheduling.StickySessionMaxWaiting = 3
			svc := &OpenAIGatewayService{
				accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cache: cache, cfg: cfg,
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
					acquireResults: map[int64]bool{1: state != "full", 2: true},
					acquiredIDs:    &acquired, releasedIDs: &released,
				}),
			}
			ctx := context.Background()
			require.NoError(t, svc.BindGrokMediaVideoRequestAccount(ctx, &groupID, "task", 10, 20, 1))
			sessionHash := GrokMediaVideoRequestSessionHash("task", 10, 20)
			for range 20 {
				selection, decision, err := svc.SelectGrokMediaVideoRequestAccount(ctx, &groupID, sessionHash, ownerID, "")
				switch state {
				case "available":
					require.NoError(t, err)
					require.Equal(t, int64(1), selection.Account.ID)
					require.True(t, selection.Acquired)
					require.True(t, decision.StickySessionHit)
					selection.ReleaseFunc()
				case "full":
					require.NoError(t, err)
					require.Equal(t, int64(1), selection.Account.ID)
					require.False(t, selection.Acquired)
					require.Nil(t, selection.ReleaseFunc)
					require.Equal(t, int64(1), selection.WaitPlan.AccountID)
				default:
					require.ErrorIs(t, err, ErrNoAvailableAccounts)
					require.Nil(t, selection)
				}
				bound, err := svc.ResolveGrokMediaVideoRequestAccount(ctx, &groupID, "task", 10, 20)
				require.NoError(t, err)
				require.Equal(t, int64(1), bound)
			}
			require.NotContains(t, acquired, int64(2))
			require.Empty(t, cache.deletedSessions)
			if state == "available" {
				require.Len(t, released, 20)
			} else {
				require.Empty(t, released)
			}
		})
	}
}

func TestGrokVideoStickySelectionIgnoresHealthEscape(t *testing.T) {
	groupID := int64(24)
	account := Account{ID: 1, Platform: PlatformGrok, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 50, GroupIDs: []int64{groupID}}
	svc := &OpenAIGatewayService{
		accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:       &schedulerTestGatewayCache{},
	}
	stats := newOpenAIAccountRuntimeStats()
	for range 20 {
		stats.report(1, false, nil)
	}
	scheduler := &defaultOpenAIAccountScheduler{service: svc, stats: stats}
	req := OpenAIAccountScheduleRequest{GroupID: &groupID, Platform: PlatformGrok,
		SessionHash: "task", StickyAccountID: 1, PreserveStickyBinding: true}
	selection, escaped, err := scheduler.selectBySessionHash(context.Background(), req)
	require.NoError(t, err)
	require.Nil(t, selection)
	require.True(t, escaped)
	req.DisableStickyEscape = true
	selection, escaped, err = scheduler.selectBySessionHash(context.Background(), req)
	require.NoError(t, err)
	require.False(t, escaped)
	require.True(t, selection.Acquired)
	selection.ReleaseFunc()
}

func TestSelectMediaVideoRequestAccountShardScope(t *testing.T) {
	groupID, userID := int64(24), int64(10)
	lookupErr := errors.New("shard scope lookup failed")
	for _, platform := range []string{PlatformGrok, PlatformOpenAI} {
		for _, tc := range []struct {
			name       string
			accountIDs map[int64]struct{}
			scopeErr   error
			full       bool
			allowed    bool
		}{
			{name: "owner allowed", accountIDs: map[int64]struct{}{1: {}}, allowed: true},
			{name: "owner full", accountIDs: map[int64]struct{}{1: {}}, full: true, allowed: true},
			{name: "owner outside scope", accountIDs: map[int64]struct{}{2: {}}},
			{name: "empty scope", accountIDs: map[int64]struct{}{}},
			{name: "scope error", scopeErr: lookupErr},
		} {
			t.Run(platform+"/"+tc.name, func(t *testing.T) {
				owner := Account{ID: 1, Platform: platform, Type: AccountTypeAPIKey,
					Status: StatusActive, Schedulable: true, Concurrency: 50, GroupIDs: []int64{groupID}}
				other := owner
				other.ID = 2
				shardRepo := &openAISchedulerShardRepoStub{
					scope: &GroupShardAccountScope{Enabled: true, ShardID: 7, AccountIDs: tc.accountIDs},
					err:   tc.scopeErr,
				}
				group := &Group{ID: groupID, Platform: platform, Status: StatusActive, Hydrated: true, ShardEnabled: true}
				ctx := openAIShardSchedulingTestContext(group, userID)
				var acquired, released []int64
				cache := &schedulerTestGatewayCache{}
				cfg := &config.Config{}
				cfg.Gateway.OpenAIScheduler.StickyEscapeEnabled = true
				svc := &OpenAIGatewayService{
					accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{owner, other}},
					cache:       cache, cfg: cfg, groupShardRepo: shardRepo,
					concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
						acquireResults: map[int64]bool{1: !tc.full, 2: true},
						acquiredIDs:    &acquired, releasedIDs: &released,
					}),
				}
				require.NoError(t, svc.BindGrokMediaVideoRequestAccount(ctx, &groupID, "task", userID, 20, owner.ID))
				sessionHash := GrokMediaVideoRequestSessionHash("task", userID, 20)

				selection, decision, err := svc.SelectMediaVideoRequestAccount(ctx, &groupID, sessionHash, owner.ID, "", platform)
				switch {
				case tc.scopeErr != nil:
					require.ErrorIs(t, err, tc.scopeErr)
					require.Nil(t, selection)
				case !tc.allowed:
					require.ErrorIs(t, err, ErrNoAvailableAccounts)
					require.Nil(t, selection)
				default:
					require.NoError(t, err)
					require.NotNil(t, selection)
					require.Equal(t, owner.ID, selection.Account.ID)
					require.True(t, decision.StickySessionHit)
					require.Equal(t, !tc.full, selection.Acquired)
					require.Equal(t, []int64{owner.ID}, acquired)
					if tc.full {
						require.Nil(t, selection.ReleaseFunc)
						require.NotNil(t, selection.WaitPlan)
						require.Equal(t, owner.ID, selection.WaitPlan.AccountID)
					} else {
						require.NotNil(t, selection.ReleaseFunc)
						selection.ReleaseFunc()
						require.Equal(t, []int64{owner.ID}, released)
					}
				}
				if !tc.allowed {
					require.Empty(t, acquired)
					require.False(t, decision.StickySessionHit)
				}
				if !tc.allowed || tc.full {
					require.Empty(t, released)
				}
				require.Equal(t, 1, shardRepo.listScopeCalls)
				require.Equal(t, groupID, shardRepo.lastGroupID)
				require.Equal(t, userID, shardRepo.lastUserID)
				require.Equal(t, platform, shardRepo.lastPlatform)
				require.NotContains(t, acquired, other.ID)
				bound, err := svc.ResolveGrokMediaVideoRequestAccount(ctx, &groupID, "task", userID, 20)
				require.NoError(t, err)
				require.Equal(t, owner.ID, bound)
				require.Empty(t, cache.deletedSessions)
			})
		}
	}
}
