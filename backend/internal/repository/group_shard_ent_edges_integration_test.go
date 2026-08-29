//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupShardEntEdgesUseMigratedGroupID(t *testing.T) {
	ctx := context.Background()
	// The harness applies the real SQL migrations; do not create an Ent schema.
	client := testEntTx(t).Client()
	parent, err := client.Group.Create().SetName(t.Name()).SetPlatform(service.PlatformOpenAI).Save(ctx)
	require.NoError(t, err)
	shard, err := client.GroupShard.Create().SetGroupID(parent.ID).SetName("owner").Save(ctx)
	require.NoError(t, err)

	t.Run("parent eager loads shards through group_id", func(t *testing.T) {
		loaded, err := client.Group.Query().Where(group.IDEQ(parent.ID)).WithShards().Only(ctx)
		require.NoError(t, err)
		require.Len(t, loaded.Edges.Shards, 1)
		require.Equal(t, shard.ID, loaded.Edges.Shards[0].ID)
		require.Equal(t, parent.ID, loaded.Edges.Shards[0].GroupID)
	})
	t.Run("shard queries the same parent", func(t *testing.T) {
		loaded, err := shard.QueryGroup().Only(ctx)
		require.NoError(t, err)
		require.Equal(t, parent.ID, loaded.ID)
	})
}
