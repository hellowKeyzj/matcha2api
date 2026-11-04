//go:build integration

package repository

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/groupsharduser"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupShard_DeletedUserDoesNotConsumeCapacity(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	group := mustCreateGroup(t, client, &service.Group{Name: "deleted-shard-user"})
	deleted := mustCreateUser(t, client, &service.User{})
	replacement := mustCreateUser(t, client, &service.User{})
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM group_shards WHERE group_id = $1`, group.ID)
		_, _ = integrationDB.Exec(`DELETE FROM groups WHERE id = $1`, group.ID)
		_, _ = integrationDB.Exec(`DELETE FROM users WHERE id IN ($1, $2)`, deleted.ID, replacement.ID)
	})
	repo := &groupShardRepository{client: client, db: integrationDB}
	shard, err := repo.assignUserToAvailableShard(ctx, group.ID, deleted.ID, 1)
	require.NoError(t, err)
	// Simulate memberships left by older versions after soft deletion.
	_, err = integrationDB.Exec(`UPDATE users SET deleted_at = NOW() WHERE id = $1`, deleted.ID)
	require.NoError(t, err)
	listed, err := repo.ListByGroup(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Zero(t, listed[0].UserCount)
	got, err := repo.GetByID(ctx, group.ID, shard.ID)
	require.NoError(t, err)
	require.Zero(t, got.UserCount)
	assigned, err := repo.assignUserToAvailableShard(ctx, group.ID, replacement.ID, 1)
	require.NoError(t, err)
	require.Equal(t, shard.ID, assigned.ID)
	require.EqualValues(t, 1, assigned.UserCount)
	got, err = repo.findUserShard(ctx, group.ID, replacement.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, got.UserCount)

	userRepo := NewUserRepository(client, integrationDB)
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()
	require.NoError(t, userRepo.Delete(dbent.NewTxContext(ctx, tx), replacement.ID))
	require.NoError(t, tx.Rollback())
	count, err := client.GroupShardUser.Query().Where(groupsharduser.UserIDEQ(replacement.ID)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count, "membership cleanup must roll back with user deletion")
	require.NoError(t, userRepo.Delete(ctx, replacement.ID))
	count, err = client.GroupShardUser.Query().Where(groupsharduser.UserIDEQ(replacement.ID)).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	_, err = repo.GetByID(ctx, group.ID, shard.ID)
	require.ErrorIs(t, err, service.ErrGroupShardNotFound)
}

func TestGroupShard_DeleteUserKeepsOtherMembers(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	group := mustCreateGroup(t, client, &service.Group{Name: "keep-occupied-shard"})
	first := mustCreateUser(t, client, &service.User{})
	second := mustCreateUser(t, client, &service.User{})
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM group_shards WHERE group_id = $1`, group.ID)
		_, _ = integrationDB.Exec(`DELETE FROM groups WHERE id = $1`, group.ID)
		_, _ = integrationDB.Exec(`DELETE FROM users WHERE id IN ($1, $2)`, first.ID, second.ID)
	})
	repo := &groupShardRepository{client: client, db: integrationDB}
	shard, err := repo.assignUserToAvailableShard(ctx, group.ID, first.ID, 2)
	require.NoError(t, err)
	require.NoError(t, repo.AssignUser(ctx, group.ID, shard.ID, second.ID, 2))
	userRepo := NewUserRepository(client, integrationDB)
	require.NoError(t, userRepo.Delete(ctx, first.ID))
	got, err := repo.GetByID(ctx, group.ID, shard.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, got.UserCount)
	require.NoError(t, userRepo.Delete(ctx, second.ID))
	_, err = repo.GetByID(ctx, group.ID, shard.ID)
	require.ErrorIs(t, err, service.ErrGroupShardNotFound)
}
