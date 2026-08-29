package service

import (
	"context"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const groupShardStatusInactive = "inactive"

func (s *adminServiceImpl) ListGroupShards(ctx context.Context, groupID int64) ([]GroupShard, error) {
	if err := s.ensureGroupShardRepo(); err != nil {
		return nil, err
	}
	if _, err := s.groupRepo.GetByIDLite(ctx, groupID); err != nil {
		return nil, err
	}
	return s.groupShardRepo.ListByGroup(ctx, groupID)
}

func (s *adminServiceImpl) GetGroupShard(ctx context.Context, groupID, shardID int64) (*GroupShard, error) {
	if err := s.ensureGroupShardRepo(); err != nil {
		return nil, err
	}
	return s.groupShardRepo.GetByID(ctx, groupID, shardID)
}

func (s *adminServiceImpl) CreateGroupShard(ctx context.Context, groupID int64, input GroupShardInput) (*GroupShard, error) {
	if err := s.ensureGroupShardRepo(); err != nil {
		return nil, err
	}
	if _, err := s.groupRepo.GetByIDLite(ctx, groupID); err != nil {
		return nil, err
	}
	shard, err := groupShardFromInput(groupID, input)
	if err != nil {
		return nil, err
	}
	if err := s.groupShardRepo.Create(ctx, shard); err != nil {
		return nil, err
	}
	s.invalidateGroupShardAuthCaches(ctx, groupID)
	return shard, nil
}

func (s *adminServiceImpl) UpdateGroupShard(ctx context.Context, groupID, shardID int64, input UpdateGroupShardInput) (*GroupShard, error) {
	if err := s.ensureGroupShardRepo(); err != nil {
		return nil, err
	}
	shard, err := s.groupShardRepo.GetByID(ctx, groupID, shardID)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		shard.Name = *input.Name
	}
	if input.Description != nil {
		shard.Description = *input.Description
	}
	if input.Status != nil {
		shard.Status = *input.Status
	}
	if err := normalizeGroupShard(shard); err != nil {
		return nil, err
	}
	if err := s.groupShardRepo.Update(ctx, shard); err != nil {
		return nil, err
	}
	s.invalidateGroupShardAuthCaches(ctx, groupID)
	return shard, nil
}

func (s *adminServiceImpl) DeleteGroupShard(ctx context.Context, groupID, shardID int64) error {
	if err := s.ensureGroupShardRepo(); err != nil {
		return err
	}
	if err := s.groupShardRepo.Delete(ctx, groupID, shardID); err != nil {
		return err
	}
	s.invalidateGroupShardAuthCaches(ctx, groupID)
	return nil
}

func (s *adminServiceImpl) ListGroupShardUsers(ctx context.Context, groupID, shardID int64) ([]GroupShardUser, error) {
	if err := s.ensureGroupShardRepo(); err != nil {
		return nil, err
	}
	if _, err := s.groupShardRepo.GetByID(ctx, groupID, shardID); err != nil {
		return nil, err
	}
	return s.groupShardRepo.ListUsers(ctx, groupID, shardID)
}

func (s *adminServiceImpl) AssignGroupShardUser(ctx context.Context, groupID, shardID, userID int64) error {
	if err := s.ensureGroupShardRepo(); err != nil {
		return err
	}
	group, err := s.groupRepo.GetByIDLite(ctx, groupID)
	if err != nil {
		return err
	}
	if _, err := s.groupShardRepo.GetByID(ctx, groupID, shardID); err != nil {
		return err
	}
	if _, err := s.userRepo.GetByID(ctx, userID); err != nil {
		return err
	}
	maxUsers := group.ShardMaxUsersPerShard
	if maxUsers < 1 {
		maxUsers = 2
	}
	if err := s.groupShardRepo.AssignUser(ctx, groupID, shardID, userID, maxUsers); err != nil {
		return err
	}
	s.invalidateGroupShardAuthCaches(ctx, groupID, userID)
	return nil
}

func (s *adminServiceImpl) RemoveGroupShardUser(ctx context.Context, groupID, shardID, userID int64) error {
	if err := s.ensureGroupShardRepo(); err != nil {
		return err
	}
	if _, err := s.groupShardRepo.GetByID(ctx, groupID, shardID); err != nil {
		return err
	}
	if err := s.groupShardRepo.RemoveUser(ctx, groupID, shardID, userID); err != nil {
		return err
	}
	s.invalidateGroupShardAuthCaches(ctx, groupID, userID)
	return nil
}

func (s *adminServiceImpl) ListGroupShardAccounts(ctx context.Context, groupID, shardID int64) ([]GroupShardAccount, error) {
	if err := s.ensureGroupShardRepo(); err != nil {
		return nil, err
	}
	if _, err := s.groupShardRepo.GetByID(ctx, groupID, shardID); err != nil {
		return nil, err
	}
	return s.groupShardRepo.ListAccounts(ctx, groupID, shardID)
}

func (s *adminServiceImpl) SetGroupShardAccounts(ctx context.Context, groupID, shardID int64, input GroupShardAccountsInput) ([]GroupShardAccount, error) {
	if err := s.ensureGroupShardRepo(); err != nil {
		return nil, err
	}
	group, err := s.groupRepo.GetByIDLite(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if _, err := s.groupShardRepo.GetByID(ctx, groupID, shardID); err != nil {
		return nil, err
	}
	accounts, err := s.normalizeGroupShardAccounts(ctx, group, input.Accounts)
	if err != nil {
		return nil, err
	}
	if err := s.groupShardRepo.SetAccounts(ctx, groupID, shardID, accounts); err != nil {
		return nil, err
	}
	s.invalidateGroupShardAuthCaches(ctx, groupID)
	return s.groupShardRepo.ListAccounts(ctx, groupID, shardID)
}

func (s *adminServiceImpl) ensureGroupShardRepo() error {
	if s.groupShardRepo == nil {
		return fmt.Errorf("group shard repository is not configured")
	}
	return nil
}

func groupShardFromInput(groupID int64, input GroupShardInput) (*GroupShard, error) {
	shard := &GroupShard{
		GroupID:     groupID,
		Name:        input.Name,
		Description: input.Description,
		Status:      input.Status,
	}
	if err := normalizeGroupShard(shard); err != nil {
		return nil, err
	}
	return shard, nil
}

func normalizeGroupShard(shard *GroupShard) error {
	shard.Name = strings.TrimSpace(shard.Name)
	shard.Status = strings.TrimSpace(shard.Status)
	if shard.Name == "" {
		return infraerrors.BadRequest("GROUP_SHARD_NAME_REQUIRED", "group shard name is required")
	}
	if shard.Status == "" {
		shard.Status = StatusActive
	}
	switch shard.Status {
	case StatusActive, groupShardStatusInactive:
		return nil
	default:
		return infraerrors.BadRequest("GROUP_SHARD_STATUS_INVALID", "group shard status must be active or inactive")
	}
}

func (s *adminServiceImpl) normalizeGroupShardAccounts(ctx context.Context, group *Group, input []GroupShardAccountInput) ([]GroupShardAccount, error) {
	accountIDs, priorities := uniqueGroupShardAccountInputs(input)
	if len(accountIDs) == 0 {
		return nil, nil
	}

	parentAccountIDs, err := s.groupRepo.GetAccountIDsByGroupIDs(ctx, []int64{group.ID})
	if err != nil {
		return nil, err
	}
	parentSet := make(map[int64]struct{}, len(parentAccountIDs))
	for _, accountID := range parentAccountIDs {
		parentSet[accountID] = struct{}{}
	}
	for _, accountID := range accountIDs {
		if _, ok := parentSet[accountID]; !ok {
			return nil, ErrGroupShardAccountNotInGroup
		}
	}

	items, err := s.accountRepo.GetByIDs(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	if len(items) != len(accountIDs) {
		return nil, ErrAccountNotFound
	}
	byID := make(map[int64]*Account, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}

	accounts := make([]GroupShardAccount, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		item := byID[accountID]
		if item == nil {
			return nil, ErrAccountNotFound
		}
		if group.Platform == PlatformComposite {
			if !isConcreteRequestPlatform(item.Platform) {
				return nil, ErrGroupShardAccountPlatformMismatch
			}
		} else if item.Platform != group.Platform {
			return nil, ErrGroupShardAccountPlatformMismatch
		}
		accounts = append(accounts, GroupShardAccount{
			GroupShardID: 0,
			GroupID:      group.ID,
			AccountID:    accountID,
			Platform:     item.Platform,
			Priority:     priorities[accountID],
		})
	}
	return accounts, nil
}

func uniqueGroupShardAccountInputs(input []GroupShardAccountInput) ([]int64, map[int64]int) {
	seen := make(map[int64]struct{}, len(input))
	priorities := make(map[int64]int, len(input))
	accountIDs := make([]int64, 0, len(input))
	for _, item := range input {
		if item.AccountID <= 0 {
			continue
		}
		if _, ok := seen[item.AccountID]; ok {
			continue
		}
		seen[item.AccountID] = struct{}{}
		priorities[item.AccountID] = item.Priority
		accountIDs = append(accountIDs, item.AccountID)
	}
	return accountIDs, priorities
}

func (s *adminServiceImpl) invalidateGroupShardAuthCaches(ctx context.Context, groupID int64, userIDs ...int64) {
	if s.authCacheInvalidator == nil {
		return
	}
	s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, groupID)
	seen := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
}
