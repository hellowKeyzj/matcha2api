package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGetByKeyForAuthCarriesShardProjection(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "shard-auth@test.local")
	group, err := client.Group.Create().
		SetName("shard-auth").SetPlatform(service.PlatformOpenAI).
		SetShardEnabled(true).SetShardMaxUsersPerShard(8).SetShardAutoCreate(true).
		Save(ctx)
	require.NoError(t, err)
	key := &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-shard-auth", Name: "shard-auth", Status: service.StatusActive}
	require.NoError(t, repo.Create(ctx, key))

	got, err := repo.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.NotNil(t, got.Group)
	require.True(t, got.Group.ShardEnabled)
	require.Equal(t, 8, got.Group.ShardMaxUsersPerShard)
	require.True(t, got.Group.ShardAutoCreate)
}
