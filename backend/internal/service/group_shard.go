package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrGroupShardNotFound                = infraerrors.NotFound("GROUP_SHARD_NOT_FOUND", "group shard not found")
	ErrGroupShardExists                  = infraerrors.Conflict("GROUP_SHARD_EXISTS", "group shard already exists")
	ErrGroupShardFull                    = infraerrors.Conflict("GROUP_SHARD_FULL", "group shard is full")
	ErrGroupShardUserConflict            = infraerrors.Conflict("GROUP_SHARD_USER_CONFLICT", "user already belongs to another shard in this group")
	ErrGroupShardAccountConflict         = infraerrors.Conflict("GROUP_SHARD_ACCOUNT_CONFLICT", "account already belongs to another shard in this group")
	ErrGroupShardAccountNotInGroup       = infraerrors.BadRequest("GROUP_SHARD_ACCOUNT_NOT_IN_GROUP", "account does not belong to this group")
	ErrGroupShardAccountPlatformMismatch = infraerrors.BadRequest("GROUP_SHARD_ACCOUNT_PLATFORM_MISMATCH", "account platform does not match group platform")
	ErrGroupShardUnavailable             = infraerrors.BadRequest("GROUP_SHARD_UNAVAILABLE", "group shard is unavailable")
)

type GroupShardInput struct {
	Name        string
	Description string
	Status      string
}

type UpdateGroupShardInput struct {
	Name        *string
	Description *string
	Status      *string
}

type GroupShardAccountInput struct {
	AccountID int64
	Priority  int
}

type GroupShardAccountsInput struct {
	Accounts []GroupShardAccountInput
}

type GroupShard struct {
	ID           int64
	GroupID      int64
	Name         string
	Description  string
	Status       string
	UserCount    int64
	AccountCount int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type GroupShardUser struct {
	GroupShardID int64
	GroupID      int64
	UserID       int64
	Email        string
	Username     string
	CreatedAt    time.Time
}

type GroupShardAccount struct {
	GroupShardID int64
	GroupID      int64
	AccountID    int64
	Name         string
	Platform     string
	Type         string
	Status       string
	Priority     int
	CreatedAt    time.Time
}

type GroupShardAccountScope struct {
	Enabled    bool
	ShardID    int64
	AccountIDs map[int64]struct{}
}

func (s *GroupShardAccountScope) Allows(accountID int64) bool {
	if s == nil || !s.Enabled {
		return true
	}
	_, ok := s.AccountIDs[accountID]
	return ok
}

func (s *GroupShardAccountScope) IsEmpty() bool {
	return s != nil && s.Enabled && len(s.AccountIDs) == 0
}

type GroupShardRepository interface {
	ListByGroup(ctx context.Context, groupID int64) ([]GroupShard, error)
	GetByID(ctx context.Context, groupID, shardID int64) (*GroupShard, error)
	Create(ctx context.Context, shard *GroupShard) error
	Update(ctx context.Context, shard *GroupShard) error
	Delete(ctx context.Context, groupID, shardID int64) error
	ListUsers(ctx context.Context, groupID, shardID int64) ([]GroupShardUser, error)
	AssignUser(ctx context.Context, groupID, shardID, userID int64, maxUsers int) error
	RemoveUser(ctx context.Context, groupID, shardID, userID int64) error
	ListAccounts(ctx context.Context, groupID, shardID int64) ([]GroupShardAccount, error)
	SetAccounts(ctx context.Context, groupID, shardID int64, accounts []GroupShardAccount) error
	EnsureUserShard(ctx context.Context, group *Group, userID int64) (*GroupShard, error)
	ListAccountIDsForUserShard(ctx context.Context, group *Group, userID int64, platform string) (*GroupShardAccountScope, error)
	AccountBelongsToUserShard(ctx context.Context, groupID, userID, accountID int64) (bool, error)
}
