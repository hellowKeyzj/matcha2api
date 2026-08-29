package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/groupshard"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type groupShardRepository struct {
	client *dbent.Client
	db     *sql.DB
}

func NewGroupShardRepository(client *dbent.Client, sqlDB *sql.DB) service.GroupShardRepository {
	return &groupShardRepository{client: client, db: sqlDB}
}

func (r *groupShardRepository) ListByGroup(ctx context.Context, groupID int64) ([]service.GroupShard, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT gs.id, gs.group_id, gs.name, gs.description, gs.status, gs.created_at, gs.updated_at,
       COALESCE(u.user_count, 0), COALESCE(a.account_count, 0)
FROM group_shards gs
LEFT JOIN (
    SELECT group_shard_id, COUNT(*) AS user_count
    FROM group_shard_users
    GROUP BY group_shard_id
) u ON u.group_shard_id = gs.id
LEFT JOIN (
    SELECT group_shard_id, COUNT(*) AS account_count
    FROM group_shard_accounts
    GROUP BY group_shard_id
) a ON a.group_shard_id = gs.id
WHERE gs.group_id = $1
ORDER BY gs.id`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list group shards: %w", err)
	}
	defer rows.Close()

	shards := make([]service.GroupShard, 0)
	for rows.Next() {
		shard, err := scanGroupShard(rows)
		if err != nil {
			return nil, err
		}
		shards = append(shards, shard)
	}
	return shards, rows.Err()
}

func (r *groupShardRepository) GetByID(ctx context.Context, groupID, shardID int64) (*service.GroupShard, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT gs.id, gs.group_id, gs.name, gs.description, gs.status, gs.created_at, gs.updated_at,
       COALESCE(u.user_count, 0), COALESCE(a.account_count, 0)
FROM group_shards gs
LEFT JOIN (
    SELECT group_shard_id, COUNT(*) AS user_count
    FROM group_shard_users
    GROUP BY group_shard_id
) u ON u.group_shard_id = gs.id
LEFT JOIN (
    SELECT group_shard_id, COUNT(*) AS account_count
    FROM group_shard_accounts
    GROUP BY group_shard_id
) a ON a.group_shard_id = gs.id
WHERE gs.group_id = $1 AND gs.id = $2`, groupID, shardID)
	shard, err := scanGroupShard(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGroupShardNotFound
	}
	if err != nil {
		return nil, err
	}
	return &shard, nil
}

func (r *groupShardRepository) Create(ctx context.Context, shard *service.GroupShard) error {
	if shard == nil {
		return service.ErrGroupShardUnavailable
	}
	name := strings.TrimSpace(shard.Name)
	if name == "" {
		return service.ErrGroupShardUnavailable
	}
	status := strings.TrimSpace(shard.Status)
	if status == "" {
		status = service.StatusActive
	}
	created, err := r.client.GroupShard.Create().
		SetGroupID(shard.GroupID).
		SetName(name).
		SetDescription(shard.Description).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, service.ErrGroupShardExists)
	}
	shard.ID = created.ID
	shard.Name = created.Name
	shard.Description = derefString(created.Description)
	shard.Status = created.Status
	shard.CreatedAt = created.CreatedAt
	shard.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *groupShardRepository) Update(ctx context.Context, shard *service.GroupShard) error {
	if shard == nil {
		return service.ErrGroupShardUnavailable
	}
	name := strings.TrimSpace(shard.Name)
	if name == "" {
		return service.ErrGroupShardUnavailable
	}
	status := strings.TrimSpace(shard.Status)
	if status == "" {
		status = service.StatusActive
	}
	updated, err := r.client.GroupShard.UpdateOneID(shard.ID).
		Where(groupshard.GroupIDEQ(shard.GroupID)).
		SetName(name).
		SetDescription(shard.Description).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrGroupShardNotFound, service.ErrGroupShardExists)
	}
	shard.Name = updated.Name
	shard.Description = derefString(updated.Description)
	shard.Status = updated.Status
	shard.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *groupShardRepository) Delete(ctx context.Context, groupID, shardID int64) error {
	deleted, err := r.client.GroupShard.Delete().
		Where(groupshard.IDEQ(shardID), groupshard.GroupIDEQ(groupID)).
		Exec(ctx)
	if err != nil {
		return err
	}
	if deleted == 0 {
		return service.ErrGroupShardNotFound
	}
	return nil
}

func (r *groupShardRepository) ListUsers(ctx context.Context, groupID, shardID int64) ([]service.GroupShardUser, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT gsu.group_shard_id, gsu.group_id, gsu.user_id, u.email, COALESCE(u.username, ''), gsu.created_at
FROM group_shard_users gsu
JOIN users u ON u.id = gsu.user_id AND u.deleted_at IS NULL
WHERE gsu.group_id = $1 AND gsu.group_shard_id = $2
ORDER BY gsu.created_at, gsu.user_id`, groupID, shardID)
	if err != nil {
		return nil, fmt.Errorf("list group shard users: %w", err)
	}
	defer rows.Close()

	users := make([]service.GroupShardUser, 0)
	for rows.Next() {
		var u service.GroupShardUser
		if err := rows.Scan(&u.GroupShardID, &u.GroupID, &u.UserID, &u.Email, &u.Username, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *groupShardRepository) AssignUser(ctx context.Context, groupID, shardID, userID int64, maxUsers int) error {
	if maxUsers < 1 {
		return service.ErrGroupShardUnavailable
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM group_shards WHERE group_id = $1 AND id = $2 FOR UPDATE`, groupID, shardID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrGroupShardNotFound
		}
		return err
	}
	if status != service.StatusActive {
		return service.ErrGroupShardUnavailable
	}

	var existingShardID int64
	err = tx.QueryRowContext(ctx, `SELECT group_shard_id FROM group_shard_users WHERE group_id = $1 AND user_id = $2`, groupID, userID).Scan(&existingShardID)
	if err == nil {
		if existingShardID == shardID {
			return tx.Commit()
		}
		return service.ErrGroupShardUserConflict
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	var userCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_shard_users WHERE group_shard_id = $1`, shardID).Scan(&userCount); err != nil {
		return err
	}
	if userCount >= maxUsers {
		return service.ErrGroupShardFull
	}

	result, err := tx.ExecContext(ctx, `
INSERT INTO group_shard_users (group_shard_id, group_id, user_id)
VALUES ($1, $2, $3)
ON CONFLICT (group_id, user_id) DO NOTHING`, shardID, groupID, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		if err := tx.QueryRowContext(ctx, `SELECT group_shard_id FROM group_shard_users WHERE group_id = $1 AND user_id = $2`, groupID, userID).Scan(&existingShardID); err != nil {
			return err
		}
		if existingShardID != shardID {
			return service.ErrGroupShardUserConflict
		}
	}
	return tx.Commit()
}

func (r *groupShardRepository) RemoveUser(ctx context.Context, groupID, shardID, userID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM group_shard_users WHERE group_id = $1 AND group_shard_id = $2 AND user_id = $3`, groupID, shardID, userID)
	return err
}

func (r *groupShardRepository) ListAccounts(ctx context.Context, groupID, shardID int64) ([]service.GroupShardAccount, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT gsa.group_shard_id, gsa.group_id, gsa.account_id, a.name, gsa.platform, a.type, a.status, gsa.priority, gsa.created_at
FROM group_shard_accounts gsa
JOIN accounts a ON a.id = gsa.account_id AND a.deleted_at IS NULL
WHERE gsa.group_id = $1 AND gsa.group_shard_id = $2
ORDER BY gsa.platform, gsa.priority, gsa.account_id`, groupID, shardID)
	if err != nil {
		return nil, fmt.Errorf("list group shard accounts: %w", err)
	}
	defer rows.Close()

	accounts := make([]service.GroupShardAccount, 0)
	for rows.Next() {
		var a service.GroupShardAccount
		if err := rows.Scan(&a.GroupShardID, &a.GroupID, &a.AccountID, &a.Name, &a.Platform, &a.Type, &a.Status, &a.Priority, &a.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func (r *groupShardRepository) SetAccounts(ctx context.Context, groupID, shardID int64, accounts []service.GroupShardAccount) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM group_shards WHERE group_id = $1 AND id = $2 FOR UPDATE`, groupID, shardID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrGroupShardNotFound
		}
		return err
	}
	if status != service.StatusActive {
		return service.ErrGroupShardUnavailable
	}

	accountIDs, inputPriorities := uniqueShardAccountInputs(accounts)
	if len(accountIDs) > 0 {
		conflicts, err := r.listConflictingShardAccountIDs(ctx, tx, groupID, shardID, accountIDs)
		if err != nil {
			return err
		}
		if len(conflicts) > 0 {
			return service.ErrGroupShardAccountConflict
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM group_shard_accounts WHERE group_id = $1 AND group_shard_id = $2`, groupID, shardID); err != nil {
		return err
	}
	if len(accountIDs) == 0 {
		return tx.Commit()
	}

	rows, err := tx.QueryContext(ctx, `
SELECT a.id, a.platform, COALESCE(ag.priority, a.priority)
FROM accounts a
JOIN account_groups ag ON ag.account_id = a.id AND ag.group_id = $1
WHERE a.deleted_at IS NULL AND a.id = ANY($2)`, groupID, pq.Array(accountIDs))
	if err != nil {
		return err
	}
	defer rows.Close()

	accountMeta := make(map[int64]service.GroupShardAccount, len(accountIDs))
	for rows.Next() {
		var item service.GroupShardAccount
		if err := rows.Scan(&item.AccountID, &item.Platform, &item.Priority); err != nil {
			return err
		}
		accountMeta[item.AccountID] = item
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(accountMeta) != len(accountIDs) {
		return service.ErrAccountNotFound
	}

	for _, accountID := range accountIDs {
		meta := accountMeta[accountID]
		priority := meta.Priority
		if inputPriorities[accountID] > 0 {
			priority = inputPriorities[accountID]
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO group_shard_accounts (group_shard_id, group_id, account_id, platform, priority)
VALUES ($1, $2, $3, $4, $5)`, shardID, groupID, accountID, meta.Platform, priority); err != nil {
			if isUniqueViolation(err) {
				return service.ErrGroupShardAccountConflict
			}
			return err
		}
	}
	return tx.Commit()
}

func (r *groupShardRepository) EnsureUserShard(ctx context.Context, group *service.Group, userID int64) (*service.GroupShard, error) {
	if group == nil || !group.ShardEnabled {
		return nil, nil
	}
	if userID <= 0 {
		return nil, service.ErrGroupShardUnavailable
	}
	if shard, err := r.findUserShard(ctx, group.ID, userID); err == nil || !errors.Is(err, service.ErrGroupShardNotFound) {
		return shard, err
	}
	return r.assignUserToAvailableShard(ctx, group.ID, userID, group.ShardMaxUsersPerShard)
}

func (r *groupShardRepository) ListAccountIDsForUserShard(ctx context.Context, group *service.Group, userID int64, platform string) (*service.GroupShardAccountScope, error) {
	if group == nil || !group.ShardEnabled {
		return &service.GroupShardAccountScope{Enabled: false}, nil
	}
	shard, err := r.EnsureUserShard(ctx, group, userID)
	if err != nil {
		return nil, err
	}
	if shard == nil {
		return &service.GroupShardAccountScope{Enabled: false}, nil
	}

	platform = strings.TrimSpace(platform)
	args := []any{group.ID, shard.ID}
	query := `SELECT account_id FROM group_shard_accounts WHERE group_id = $1 AND group_shard_id = $2`
	if platform != "" {
		query += ` AND platform = $3`
		args = append(args, platform)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scope := &service.GroupShardAccountScope{Enabled: true, ShardID: shard.ID, AccountIDs: make(map[int64]struct{})}
	for rows.Next() {
		var accountID int64
		if err := rows.Scan(&accountID); err != nil {
			return nil, err
		}
		scope.AccountIDs[accountID] = struct{}{}
	}
	return scope, rows.Err()
}

func (r *groupShardRepository) AccountBelongsToUserShard(ctx context.Context, groupID, userID, accountID int64) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM group_shard_users gsu
    JOIN group_shard_accounts gsa ON gsa.group_shard_id = gsu.group_shard_id AND gsa.group_id = gsu.group_id
    JOIN group_shards gs ON gs.id = gsu.group_shard_id AND gs.group_id = gsu.group_id AND gs.status = $4
    WHERE gsu.group_id = $1 AND gsu.user_id = $2 AND gsa.account_id = $3
)`, groupID, userID, accountID, service.StatusActive).Scan(&ok)
	return ok, err
}

func (r *groupShardRepository) findUserShard(ctx context.Context, groupID, userID int64) (*service.GroupShard, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT gs.id, gs.group_id, gs.name, gs.description, gs.status, gs.created_at, gs.updated_at,
       COALESCE(u.user_count, 0), COALESCE(a.account_count, 0)
FROM group_shard_users gsu
JOIN group_shards gs ON gs.id = gsu.group_shard_id AND gs.group_id = gsu.group_id
LEFT JOIN (
    SELECT group_shard_id, COUNT(*) AS user_count
    FROM group_shard_users
    GROUP BY group_shard_id
) u ON u.group_shard_id = gs.id
LEFT JOIN (
    SELECT group_shard_id, COUNT(*) AS account_count
    FROM group_shard_accounts
    GROUP BY group_shard_id
) a ON a.group_shard_id = gs.id
WHERE gsu.group_id = $1 AND gsu.user_id = $2 AND gs.status = $3`, groupID, userID, service.StatusActive)
	shard, err := scanGroupShard(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGroupShardNotFound
	}
	if err != nil {
		return nil, err
	}
	return &shard, nil
}

func (r *groupShardRepository) assignUserToAvailableShard(ctx context.Context, groupID, userID int64, maxUsers int) (*service.GroupShard, error) {
	if maxUsers < 1 {
		return nil, service.ErrGroupShardUnavailable
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var existingShardID int64
	err = tx.QueryRowContext(ctx, `SELECT group_shard_id FROM group_shard_users WHERE group_id = $1 AND user_id = $2`, groupID, userID).Scan(&existingShardID)
	if err == nil {
		shard, err := getShardTx(ctx, tx, groupID, existingShardID)
		if err != nil {
			return nil, err
		}
		return shard, tx.Commit()
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var shardID int64
	err = tx.QueryRowContext(ctx, `
SELECT gs.id
FROM group_shards gs
WHERE gs.group_id = $1
  AND gs.status = $2
  AND (SELECT COUNT(*) FROM group_shard_users gsu WHERE gsu.group_shard_id = gs.id) < $3
ORDER BY gs.id
LIMIT 1
FOR UPDATE SKIP LOCKED`, groupID, service.StatusActive, maxUsers).Scan(&shardID)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.QueryRowContext(ctx, `
WITH next_id AS (
    SELECT nextval(pg_get_serial_sequence('group_shards', 'id')) AS id
)
INSERT INTO group_shards (id, group_id, name, status)
SELECT id, $1, 'Shard ' || id::text, $2
FROM next_id
RETURNING id`, groupID, service.StatusActive).Scan(&shardID); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	var userCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_shard_users WHERE group_shard_id = $1`, shardID).Scan(&userCount); err != nil {
		return nil, err
	}
	if userCount >= maxUsers {
		return nil, service.ErrGroupShardFull
	}
	result, err := tx.ExecContext(ctx, `
INSERT INTO group_shard_users (group_shard_id, group_id, user_id)
VALUES ($1, $2, $3)
ON CONFLICT (group_id, user_id) DO NOTHING`, shardID, groupID, userID)
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		if err := tx.QueryRowContext(ctx, `SELECT group_shard_id FROM group_shard_users WHERE group_id = $1 AND user_id = $2`, groupID, userID).Scan(&shardID); err != nil {
			return nil, err
		}
	}
	shard, err := getShardTx(ctx, tx, groupID, shardID)
	if err != nil {
		return nil, err
	}
	return shard, tx.Commit()
}

func (r *groupShardRepository) listConflictingShardAccountIDs(ctx context.Context, tx *sql.Tx, groupID, shardID int64, accountIDs []int64) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT account_id
FROM group_shard_accounts
WHERE group_id = $1 AND group_shard_id <> $2 AND account_id = ANY($3)`, groupID, shardID, pq.Array(accountIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	conflicts := make([]int64, 0)
	for rows.Next() {
		var accountID int64
		if err := rows.Scan(&accountID); err != nil {
			return nil, err
		}
		conflicts = append(conflicts, accountID)
	}
	return conflicts, rows.Err()
}

func uniqueShardAccountInputs(accounts []service.GroupShardAccount) ([]int64, map[int64]int) {
	seen := make(map[int64]struct{}, len(accounts))
	priorities := make(map[int64]int, len(accounts))
	ids := make([]int64, 0, len(accounts))
	for _, item := range accounts {
		if item.AccountID <= 0 {
			continue
		}
		if _, ok := seen[item.AccountID]; ok {
			continue
		}
		seen[item.AccountID] = struct{}{}
		priorities[item.AccountID] = item.Priority
		ids = append(ids, item.AccountID)
	}
	return ids, priorities
}

func scanGroupShard(scanner interface{ Scan(dest ...any) error }) (service.GroupShard, error) {
	var shard service.GroupShard
	var description sql.NullString
	if err := scanner.Scan(
		&shard.ID,
		&shard.GroupID,
		&shard.Name,
		&description,
		&shard.Status,
		&shard.CreatedAt,
		&shard.UpdatedAt,
		&shard.UserCount,
		&shard.AccountCount,
	); err != nil {
		return service.GroupShard{}, err
	}
	if description.Valid {
		shard.Description = description.String
	}
	return shard, nil
}

func getShardTx(ctx context.Context, tx *sql.Tx, groupID, shardID int64) (*service.GroupShard, error) {
	row := tx.QueryRowContext(ctx, `
SELECT gs.id, gs.group_id, gs.name, gs.description, gs.status, gs.created_at, gs.updated_at,
       COALESCE(u.user_count, 0), COALESCE(a.account_count, 0)
FROM group_shards gs
LEFT JOIN (
    SELECT group_shard_id, COUNT(*) AS user_count
    FROM group_shard_users
    GROUP BY group_shard_id
) u ON u.group_shard_id = gs.id
LEFT JOIN (
    SELECT group_shard_id, COUNT(*) AS account_count
    FROM group_shard_accounts
    GROUP BY group_shard_id
) a ON a.group_shard_id = gs.id
WHERE gs.group_id = $1 AND gs.id = $2`, groupID, shardID)
	shard, err := scanGroupShard(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGroupShardNotFound
	}
	if err != nil {
		return nil, err
	}
	return &shard, nil
}
