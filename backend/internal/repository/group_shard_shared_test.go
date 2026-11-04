package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupShardSharedScopeQuery(t *testing.T) {
	for _, platform := range []string{"", service.PlatformAnthropic} {
		t.Run("platform="+platform, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := &groupShardRepository{db: db}
			now := time.Now()
			mock.ExpectQuery(`(?s)FROM group_shard_users gsu\s+JOIN users live_user.*live_user.deleted_at IS NULL.*WHERE gsu.group_id = \$1 AND gsu.user_id = \$2 AND gs.status = \$3`).
				WithArgs(int64(1), int64(2), service.StatusActive).
				WillReturnRows(sqlmock.NewRows([]string{"id", "group_id", "name", "description", "status", "created_at", "updated_at", "user_count", "account_count"}).
					AddRow(3, 1, "own", "", service.StatusActive, now, now, 1, 1))
			query := `(?s)SELECT account_id FROM \(.*FROM group_shard_accounts gsa\s+JOIN account_groups ag ON ag.group_id = gsa.group_id AND ag.account_id = gsa.account_id.*gsa.group_shard_id = \$2\s+UNION ALL.*FROM group_shared_accounts gsa\s+JOIN account_groups ag ON ag.group_id = gsa.group_id AND ag.account_id = gsa.account_id.*WHERE gsa.group_id = \$1\s+\) eligible WHERE TRUE`
			if platform != "" {
				query += ` AND platform = \$3`
			}
			expected := mock.ExpectQuery(query)
			if platform == "" {
				expected.WithArgs(int64(1), int64(3))
			} else {
				expected.WithArgs(int64(1), int64(3), platform)
			}
			expected.WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(10).AddRow(20).AddRow(20))
			scope, err := repo.ListAccountIDsForUserShard(context.Background(), &service.Group{ID: 1, ShardEnabled: true}, 2, platform)
			require.NoError(t, err)
			require.True(t, scope.Enabled)
			require.EqualValues(t, 3, scope.ShardID)
			require.Len(t, scope.AccountIDs, 2)
			require.True(t, scope.Allows(10))
			require.True(t, scope.Allows(20))
			require.False(t, scope.Allows(30))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGroupShardRemoveUserCleanupTransaction(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "commit"
		if fail {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := &groupShardRepository{db: db}
			mock.ExpectBegin()
			mock.ExpectQuery(`SELECT id FROM group_shards WHERE group_id = \$1 AND id = \$2 FOR UPDATE`).
				WithArgs(int64(1), int64(3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
			mock.ExpectExec(`DELETE FROM group_shard_users WHERE group_id = \$1 AND group_shard_id = \$2 AND user_id = \$3`).
				WithArgs(int64(1), int64(3), int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
			cleanup := mock.ExpectExec(`(?s)DELETE FROM group_shards gs WHERE gs.group_id = \$1 AND gs.id = \$2\s+AND NOT EXISTS.*JOIN users u ON u.id = gsu.user_id AND u.deleted_at IS NULL.*WHERE gsu.group_shard_id = gs.id`).
				WithArgs(int64(1), int64(3))
			cleanupErr := errors.New("cleanup failed")
			if fail {
				cleanup.WillReturnError(cleanupErr)
				mock.ExpectRollback()
			} else {
				cleanup.WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			err = repo.RemoveUser(context.Background(), 1, 3, 2)
			if fail {
				require.ErrorIs(t, err, cleanupErr)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
