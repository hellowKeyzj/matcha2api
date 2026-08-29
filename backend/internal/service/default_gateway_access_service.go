package service

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const defaultGatewayAPIKeyName = "Default API Key"

type DefaultGatewayAccess struct {
	GroupID  int64
	ShardID  int64
	APIKeyID int64
}

type DefaultGatewayAccessProvisioner interface {
	RequireDefaultShardGroup(ctx context.Context) (*Group, error)
	ProvisionDefaultGatewayAccess(ctx context.Context, userID int64) (*DefaultGatewayAccess, error)
}

type DefaultGatewayAccessService struct {
	groupRepo      GroupRepository
	groupShardRepo GroupShardRepository
	userRepo       UserRepository
	apiKeyService  *APIKeyService
}

func NewDefaultGatewayAccessService(
	groupRepo GroupRepository,
	groupShardRepo GroupShardRepository,
	userRepo UserRepository,
	apiKeyService *APIKeyService,
) *DefaultGatewayAccessService {
	return &DefaultGatewayAccessService{
		groupRepo:      groupRepo,
		groupShardRepo: groupShardRepo,
		userRepo:       userRepo,
		apiKeyService:  apiKeyService,
	}
}

func (s *DefaultGatewayAccessService) RequireDefaultShardGroup(ctx context.Context) (*Group, error) {
	if s == nil || s.groupRepo == nil {
		return nil, ErrDefaultShardGroupNotConfigured
	}
	group, err := s.groupRepo.GetDefaultShardGroup(ctx)
	if err != nil {
		return nil, err
	}
	if group == nil || !group.ShardEnabled || !group.IsActive() {
		return nil, ErrDefaultShardGroupNotConfigured
	}
	if group.ShardMaxUsersPerShard < 1 {
		return nil, ErrGroupShardUnavailable
	}
	group.ShardAutoCreate = true
	return group, nil
}

func (s *DefaultGatewayAccessService) ProvisionDefaultGatewayAccess(ctx context.Context, userID int64) (*DefaultGatewayAccess, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	group, err := s.RequireDefaultShardGroup(ctx)
	if err != nil {
		return nil, err
	}
	if s.userRepo == nil || s.groupShardRepo == nil || s.apiKeyService == nil {
		return nil, ErrServiceUnavailable
	}

	if err := s.userRepo.AddGroupToAllowedGroups(ctx, userID, group.ID); err != nil {
		return nil, fmt.Errorf("grant default shard group: %w", err)
	}
	shard, err := s.groupShardRepo.EnsureUserShard(ctx, group, userID)
	if err != nil {
		return nil, fmt.Errorf("ensure default shard: %w", err)
	}

	access := &DefaultGatewayAccess{GroupID: group.ID}
	if shard != nil {
		access.ShardID = shard.ID
	}
	existing, err := s.findDefaultGroupAPIKey(ctx, userID, group.ID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		access.APIKeyID = existing.ID
		return access, nil
	}

	apiKey, err := s.apiKeyService.CreateSystem(ctx, userID, CreateAPIKeyRequest{
		Name:    defaultGatewayAPIKeyName,
		GroupID: &group.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("create default api key: %w", err)
	}
	access.APIKeyID = apiKey.ID
	return access, nil
}

func (s *DefaultGatewayAccessService) findDefaultGroupAPIKey(ctx context.Context, userID, groupID int64) (*APIKey, error) {
	keys, _, err := s.apiKeyService.List(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 1}, APIKeyListFilters{GroupID: &groupID})
	if err != nil {
		return nil, fmt.Errorf("list default api key: %w", err)
	}
	if len(keys) == 0 {
		return nil, nil
	}
	return &keys[0], nil
}
