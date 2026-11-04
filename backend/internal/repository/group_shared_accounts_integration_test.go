//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type sharedPoolFixture struct {
	repo     *groupShardRepository
	group    *service.Group
	shards   []service.GroupShard
	users    []int64
	accounts []int64
}

func newSharedPoolFixture(t *testing.T) *sharedPoolFixture {
	t.Helper()
	ctx := context.Background()
	client := testEntClient(t)
	f := &sharedPoolFixture{repo: &groupShardRepository{client: client, db: integrationDB}}
	f.group = mustCreateGroup(t, client, &service.Group{Name: fmt.Sprintf("shared-pool-%d", time.Now().UnixNano())})
	f.group.ShardEnabled = true
	f.group.ShardMaxUsersPerShard = 2
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM groups WHERE id = $1`, f.group.ID)
		for _, id := range f.users {
			_, _ = integrationDB.Exec(`DELETE FROM users WHERE id = $1`, id)
		}
		for _, id := range f.accounts {
			_, _ = integrationDB.Exec(`DELETE FROM accounts WHERE id = $1`, id)
		}
	})
	for i := 0; i < 2; i++ {
		u := mustCreateUser(t, client, &service.User{})
		f.users = append(f.users, u.ID)
		s := service.GroupShard{GroupID: f.group.ID, Name: fmt.Sprintf("pool-shard-%d", i)}
		require.NoError(t, f.repo.Create(ctx, &s))
		f.shards = append(f.shards, s)
		require.NoError(t, f.repo.AssignUser(ctx, f.group.ID, s.ID, u.ID, 2))
	}
	for i := 0; i < 4; i++ {
		platform := service.PlatformAnthropic
		if i == 3 {
			platform = service.PlatformOpenAI
		}
		a := mustCreateAccount(t, client, &service.Account{Name: fmt.Sprintf("pool-account-%d", i), Platform: platform, Priority: 17})
		f.accounts = append(f.accounts, a.ID)
		_, err := integrationDB.Exec(`INSERT INTO account_groups (account_id, group_id, priority) VALUES ($1, $2, 23)`, a.ID, f.group.ID)
		require.NoError(t, err)
	}
	return f
}

func TestGroupSharedAccounts_VisibilityAndSticky(t *testing.T) {
	f := newSharedPoolFixture(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		require.NoError(t, f.repo.SetAccounts(ctx, f.group.ID, f.shards[i].ID, []service.GroupShardAccount{{AccountID: f.accounts[i]}}))
	}
	require.NoError(t, f.repo.SetSharedAccounts(ctx, f.group.ID, []service.GroupShardAccount{
		{AccountID: f.accounts[2], Priority: 7}, {AccountID: f.accounts[3]},
	}))
	listed, err := f.repo.ListSharedAccounts(ctx, f.group.ID)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	for _, a := range listed {
		require.Zero(t, a.GroupShardID)
		require.Equal(t, f.group.ID, a.GroupID)
		if a.AccountID == f.accounts[2] {
			require.Equal(t, 7, a.Priority)
		} else {
			require.Equal(t, 23, a.Priority)
		}
	}
	for i, userID := range f.users {
		scope, err := f.repo.ListAccountIDsForUserShard(ctx, f.group, userID, service.PlatformAnthropic)
		require.NoError(t, err)
		require.Len(t, scope.AccountIDs, 2)
		require.True(t, scope.Allows(f.accounts[i]))
		require.True(t, scope.Allows(f.accounts[2]))
		require.False(t, scope.Allows(f.accounts[1-i]))
		require.False(t, scope.Allows(f.accounts[3]))
		for j, id := range f.accounts {
			ok, err := f.repo.AccountBelongsToUserShard(ctx, f.group.ID, userID, id)
			require.NoError(t, err)
			require.Equal(t, j == i || j >= 2, ok)
		}
		all, err := f.repo.ListAccountIDsForUserShard(ctx, f.group, userID, "")
		require.NoError(t, err)
		require.Len(t, all.AccountIDs, 3)
	}
	// Stale bindings must not bypass parent-group membership, shared or exclusive.
	_, err = integrationDB.Exec(`DELETE FROM account_groups WHERE group_id = $1 AND account_id IN ($2, $3)`, f.group.ID, f.accounts[0], f.accounts[2])
	require.NoError(t, err)
	scope, err := f.repo.ListAccountIDsForUserShard(ctx, f.group, f.users[0], "")
	require.NoError(t, err)
	require.Len(t, scope.AccountIDs, 1)
	for _, id := range []int64{f.accounts[0], f.accounts[2]} {
		ok, err := f.repo.AccountBelongsToUserShard(ctx, f.group.ID, f.users[0], id)
		require.NoError(t, err)
		require.False(t, ok)
	}
	_, err = integrationDB.Exec(`UPDATE group_shards SET status = 'inactive' WHERE id = $1`, f.shards[0].ID)
	require.NoError(t, err)
	ok, err := f.repo.AccountBelongsToUserShard(ctx, f.group.ID, f.users[0], f.accounts[3])
	require.NoError(t, err)
	require.False(t, ok)
	_, err = integrationDB.Exec(`UPDATE users SET deleted_at = NOW() WHERE id = $1`, f.users[1])
	require.NoError(t, err)
	ok, err = f.repo.AccountBelongsToUserShard(ctx, f.group.ID, f.users[1], f.accounts[3])
	require.NoError(t, err)
	require.False(t, ok)
	require.ErrorIs(t, f.repo.AssignUser(ctx, f.group.ID, f.shards[1].ID, f.users[1], 2), service.ErrUserNotFound)
	_, err = f.repo.EnsureUserShard(ctx, f.group, f.users[1])
	require.ErrorIs(t, err, service.ErrUserNotFound)
}

func TestGroupSharedAccounts_ExclusiveAndAtomicReplacement(t *testing.T) {
	f := newSharedPoolFixture(t)
	ctx := context.Background()
	shared := []service.GroupShardAccount{{AccountID: f.accounts[0]}}
	exclusive := []service.GroupShardAccount{{AccountID: f.accounts[1]}}
	require.NoError(t, f.repo.SetSharedAccounts(ctx, f.group.ID, shared))
	require.NoError(t, f.repo.SetAccounts(ctx, f.group.ID, f.shards[0].ID, exclusive))
	require.ErrorIs(t, f.repo.SetAccounts(ctx, f.group.ID, f.shards[1].ID, shared), service.ErrGroupShardAccountConflict)
	require.ErrorIs(t, f.repo.SetSharedAccounts(ctx, f.group.ID, exclusive), service.ErrGroupShardAccountConflict)
	require.ErrorIs(t, f.repo.SetAccounts(ctx, f.group.ID, f.shards[1].ID, exclusive), service.ErrGroupShardAccountConflict)
	_, err := integrationDB.Exec(`DELETE FROM account_groups WHERE group_id = $1 AND account_id = $2`, f.group.ID, f.accounts[2])
	require.NoError(t, err)
	invalid := []service.GroupShardAccount{{AccountID: f.accounts[2]}}
	require.ErrorIs(t, f.repo.SetSharedAccounts(ctx, f.group.ID, invalid), service.ErrAccountNotFound)
	require.ErrorIs(t, f.repo.SetAccounts(ctx, f.group.ID, f.shards[0].ID, invalid), service.ErrAccountNotFound)
	got, err := f.repo.ListSharedAccounts(ctx, f.group.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, f.accounts[0], got[0].AccountID)
	got, err = f.repo.ListAccounts(ctx, f.group.ID, f.shards[0].ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, f.accounts[1], got[0].AccountID)
	// Clearing shared bindings makes the account eligible for an exclusive binding.
	require.NoError(t, f.repo.SetSharedAccounts(ctx, f.group.ID, nil))
	require.NoError(t, f.repo.SetAccounts(ctx, f.group.ID, f.shards[1].ID, shared))
	got, err = f.repo.ListSharedAccounts(ctx, f.group.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)
}

func TestGroupSharedAccounts_ConcurrentWriters(t *testing.T) {
	f := newSharedPoolFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Queue both writers behind the parent lock before releasing them together.
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `SELECT id FROM groups WHERE id = $1 FOR UPDATE`, f.group.ID)
	require.NoError(t, err)
	inputs := []service.GroupShardAccount{{AccountID: f.accounts[0]}}
	started := make(chan struct{}, 2)
	results := make(chan error, 2)
	go func() {
		started <- struct{}{}
		results <- f.repo.SetSharedAccounts(ctx, f.group.ID, inputs)
	}()
	go func() {
		started <- struct{}{}
		results <- f.repo.SetAccounts(ctx, f.group.ID, f.shards[0].ID, inputs)
	}()
	<-started
	<-started
	require.NoError(t, tx.Commit())
	first, second := <-results, <-results
	if first == nil {
		require.ErrorIs(t, second, service.ErrGroupShardAccountConflict)
	} else {
		require.ErrorIs(t, first, service.ErrGroupShardAccountConflict)
		require.NoError(t, second)
	}
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT
    (SELECT COUNT(*) FROM group_shared_accounts WHERE group_id = $1) +
    (SELECT COUNT(*) FROM group_shard_accounts WHERE group_id = $1)`, f.group.ID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestGroupSharedAccounts_RemoveLastLiveUser(t *testing.T) {
	f := newSharedPoolFixture(t)
	ctx := context.Background()
	// A stale deleted-user membership must not keep an otherwise empty shard alive.
	_, err := integrationDB.Exec(`UPDATE users SET deleted_at = NOW() WHERE id = $1`, f.users[1])
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO group_shard_users (group_shard_id, group_id, user_id) VALUES ($1, $2, $3)
ON CONFLICT (group_id, user_id) DO UPDATE SET group_shard_id = EXCLUDED.group_shard_id`, f.shards[0].ID, f.group.ID, f.users[1])
	require.NoError(t, err)
	require.NoError(t, f.repo.SetAccounts(ctx, f.group.ID, f.shards[0].ID, []service.GroupShardAccount{{AccountID: f.accounts[0]}}))
	require.NoError(t, f.repo.SetSharedAccounts(ctx, f.group.ID, []service.GroupShardAccount{{AccountID: f.accounts[1]}}))
	require.NoError(t, f.repo.RemoveUser(ctx, f.group.ID, f.shards[0].ID, 0))
	_, err = f.repo.GetByID(ctx, f.group.ID, f.shards[0].ID)
	require.NoError(t, err, "a live member still keeps the shard")
	require.NoError(t, f.repo.RemoveUser(ctx, f.group.ID, f.shards[0].ID, f.users[0]))
	_, err = f.repo.GetByID(ctx, f.group.ID, f.shards[0].ID)
	require.ErrorIs(t, err, service.ErrGroupShardNotFound)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM group_shard_accounts WHERE group_shard_id = $1`, f.shards[0].ID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM accounts WHERE id = $1 AND deleted_at IS NULL`, f.accounts[0]).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM account_groups WHERE group_id = $1`, f.group.ID).Scan(&count))
	require.Equal(t, 4, count)
	shared, err := f.repo.ListSharedAccounts(ctx, f.group.ID)
	require.NoError(t, err)
	require.Len(t, shared, 1)
}
