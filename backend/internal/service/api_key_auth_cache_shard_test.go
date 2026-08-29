package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func shardAuthRoundtrip(t *testing.T, key *APIKey) *APIKey {
	t.Helper()
	svc := &APIKeyService{}
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), key)})
	require.NoError(t, err)
	var restored APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &restored))
	materialized, used, err := svc.applyAuthCacheEntry(key.Key, &restored)
	require.NoError(t, err)
	require.True(t, used)
	return materialized
}

func TestAPIKeyAuthSnapshotShardRoundtrip(t *testing.T) {
	key := profitAuthTestAPIKey()
	key.Group.ShardEnabled = true
	key.Group.ShardMaxUsersPerShard = 8
	key.Group.ShardAutoCreate = true
	materialized := shardAuthRoundtrip(t, key)
	require.True(t, materialized.Group.ShardEnabled)
	require.Equal(t, 8, materialized.Group.ShardMaxUsersPerShard)
	require.True(t, materialized.Group.ShardAutoCreate)
}

func TestAPIKeyAuthSnapshotShardScheduling(t *testing.T) {
	for _, advanced := range []string{"false", "true"} {
		for _, empty := range []bool{false, true} {
			name := "allowed"
			if empty {
				name = "empty"
			}
			t.Run(advanced+"/"+name, func(t *testing.T) {
				key := profitAuthTestAPIKey()
				key.Group.ProfitControlEnabled = false
				key.Group.ShardEnabled = true
				key = shardAuthRoundtrip(t, key)
				outside := openAIShardSchedulingAccount(39211, *key.GroupID, 0)
				allowed := openAIShardSchedulingAccount(39212, *key.GroupID, 9)
				ids := map[int64]struct{}{}
				if !empty {
					ids[allowed.ID] = struct{}{}
				}
				shards := &openAISchedulerShardRepoStub{scope: &GroupShardAccountScope{Enabled: true, ShardID: 8, AccountIDs: ids}}
				groups := &countingPrivacyGroupRepo{group: key.Group}
				svc := &OpenAIGatewayService{
					accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{outside, allowed}}},
					cache:              &schedulerTestGatewayCache{},
					cfg:                &config.Config{},
					groupRepo:          groups,
					groupShardRepo:     shards,
					rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService(advanced),
					concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{outside.ID: true, allowed.ID: true}}),
				}
				ctx := openAIShardSchedulingTestContext(key.Group, key.UserID)
				selection, _, err := svc.SelectAccountWithScheduler(ctx, key.GroupID, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
				if empty {
					require.ErrorIs(t, err, ErrNoAvailableAccounts)
					require.Nil(t, selection)
				} else {
					require.NoError(t, err)
					require.NotNil(t, selection)
					require.Equal(t, allowed.ID, selection.Account.ID)
					if selection.ReleaseFunc != nil {
						selection.ReleaseFunc()
					}
				}
				require.Positive(t, shards.listScopeCalls)
				require.Equal(t, key.UserID, shards.lastUserID)
				require.Zero(t, groups.calls.Load(), "authenticated group must not be reloaded per selection")
			})
		}
	}
}

func TestAPIKeyAuthSnapshotWithoutShardFieldsEvicted(t *testing.T) {
	svc := &APIKeyService{}
	snapshot := svc.snapshotFromAPIKey(context.Background(), profitAuthTestAPIKey())
	snapshot.Version = 27
	key, used, err := svc.applyAuthCacheEntry("sk-old-shard", &APIKeyAuthCacheEntry{Snapshot: snapshot})
	require.NoError(t, err)
	require.False(t, used)
	require.Nil(t, key)
}
