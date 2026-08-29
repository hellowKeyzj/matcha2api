//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupCreateUsesShardCapacityDefault(t *testing.T) {
	for _, capacity := range []int{0, 7, -1} {
		t.Run(fmt.Sprintf("capacity_%d", capacity), func(t *testing.T) {
			ctx := context.Background()
			tx := testEntTx(t)
			client := tx.Client()
			repo := newGroupRepositoryWithSQL(client, tx)
			value := &service.Group{Name: t.Name(), Platform: service.PlatformOpenAI, Status: service.StatusActive,
				SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 1, ShardMaxUsersPerShard: capacity}
			err := repo.Create(ctx, value)
			if capacity < 0 {
				require.Error(t, err)
				require.Zero(t, value.ID)
				return
			}
			require.NoError(t, err)
			want := capacity
			if want == 0 {
				want = 2
			}
			require.Equal(t, want, value.ShardMaxUsersPerShard)
			loaded, err := repo.GetByID(ctx, value.ID)
			require.NoError(t, err)
			require.Equal(t, want, loaded.ShardMaxUsersPerShard)
			value.Description = "updated"
			require.NoError(t, repo.Update(ctx, value), "the same caller object retains the persisted default")
		})
	}
}
