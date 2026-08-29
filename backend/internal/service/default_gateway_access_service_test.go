//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type defaultAccessGroupRepoStub struct {
	groupRepoStub
	group    *Group
	getCalls int
}

func (s *defaultAccessGroupRepoStub) GetDefaultShardGroup(context.Context) (*Group, error) {
	s.getCalls++
	if s.group == nil {
		return nil, ErrDefaultShardGroupNotConfigured
	}
	clone := *s.group
	return &clone, nil
}

func (s *defaultAccessGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	if s.group == nil || s.group.ID != id {
		return nil, ErrGroupNotFound
	}
	clone := *s.group
	return &clone, nil
}

type defaultAccessUserRepoStub struct {
	userRepoStub
	addGroupCalls []struct {
		userID  int64
		groupID int64
	}
	addGroupErr error
}

func (s *defaultAccessUserRepoStub) AddGroupToAllowedGroups(_ context.Context, userID int64, groupID int64) error {
	s.addGroupCalls = append(s.addGroupCalls, struct {
		userID  int64
		groupID int64
	}{userID: userID, groupID: groupID})
	return s.addGroupErr
}

type defaultAccessGroupShardRepoStub struct {
	ensureCalls []struct {
		groupID  int64
		userID   int64
		maxUsers int
	}
	shard     *GroupShard
	ensureErr error
}

func (s *defaultAccessGroupShardRepoStub) ListByGroup(context.Context, int64) ([]GroupShard, error) {
	panic("unexpected ListByGroup call")
}
func (s *defaultAccessGroupShardRepoStub) GetByID(context.Context, int64, int64) (*GroupShard, error) {
	panic("unexpected GetByID call")
}
func (s *defaultAccessGroupShardRepoStub) Create(context.Context, *GroupShard) error {
	panic("unexpected Create call")
}
func (s *defaultAccessGroupShardRepoStub) Update(context.Context, *GroupShard) error {
	panic("unexpected Update call")
}
func (s *defaultAccessGroupShardRepoStub) Delete(context.Context, int64, int64) error {
	panic("unexpected Delete call")
}
func (s *defaultAccessGroupShardRepoStub) ListUsers(context.Context, int64, int64) ([]GroupShardUser, error) {
	panic("unexpected ListUsers call")
}
func (s *defaultAccessGroupShardRepoStub) AssignUser(context.Context, int64, int64, int64, int) error {
	panic("unexpected AssignUser call")
}
func (s *defaultAccessGroupShardRepoStub) RemoveUser(context.Context, int64, int64, int64) error {
	panic("unexpected RemoveUser call")
}
func (s *defaultAccessGroupShardRepoStub) ListAccounts(context.Context, int64, int64) ([]GroupShardAccount, error) {
	panic("unexpected ListAccounts call")
}
func (s *defaultAccessGroupShardRepoStub) SetAccounts(context.Context, int64, int64, []GroupShardAccount) error {
	panic("unexpected SetAccounts call")
}
func (s *defaultAccessGroupShardRepoStub) EnsureUserShard(_ context.Context, group *Group, userID int64) (*GroupShard, error) {
	s.ensureCalls = append(s.ensureCalls, struct {
		groupID  int64
		userID   int64
		maxUsers int
	}{groupID: group.ID, userID: userID, maxUsers: group.ShardMaxUsersPerShard})
	if s.ensureErr != nil {
		return nil, s.ensureErr
	}
	if s.shard == nil {
		return nil, nil
	}
	clone := *s.shard
	return &clone, nil
}
func (s *defaultAccessGroupShardRepoStub) ListAccountIDsForUserShard(context.Context, *Group, int64, string) (*GroupShardAccountScope, error) {
	panic("unexpected ListAccountIDsForUserShard call")
}
func (s *defaultAccessGroupShardRepoStub) AccountBelongsToUserShard(context.Context, int64, int64, int64) (bool, error) {
	panic("unexpected AccountBelongsToUserShard call")
}

type defaultAccessAPIKeyRepoStub struct {
	apiKeyRepoStub
	created []APIKey
	nextID  int64
}

func (s *defaultAccessAPIKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	if s.nextID != 0 && key.ID == 0 {
		key.ID = s.nextID
	}
	clone := *key
	s.created = append(s.created, clone)
	return nil
}

func newDefaultAccessServiceForTest(groupID int64, existing []APIKey) (*DefaultGatewayAccessService, *defaultAccessUserRepoStub, *defaultAccessGroupShardRepoStub, *defaultAccessAPIKeyRepoStub) {
	groupRepo := &defaultAccessGroupRepoStub{group: &Group{
		ID:                    groupID,
		Name:                  "default-shard",
		Status:                StatusActive,
		ShardEnabled:          true,
		ShardMaxUsersPerShard: 2,
	}}
	userRepo := &defaultAccessUserRepoStub{userRepoStub: userRepoStub{user: &User{ID: 7, Status: StatusActive}}}
	shardRepo := &defaultAccessGroupShardRepoStub{shard: &GroupShard{ID: 12, GroupID: groupID, Status: StatusActive}}
	apiKeyRepo := &defaultAccessAPIKeyRepoStub{
		apiKeyRepoStub: apiKeyRepoStub{
			allowListByUserID: true,
			listByUserIDKeys:  existing,
		},
		nextID: 99,
	}
	apiKeySvc := &APIKeyService{apiKeyRepo: apiKeyRepo, userRepo: userRepo, groupRepo: groupRepo}
	return NewDefaultGatewayAccessService(groupRepo, shardRepo, userRepo, apiKeySvc), userRepo, shardRepo, apiKeyRepo
}

func TestDefaultGatewayAccessService_ProvisionCreatesDefaultKeyAndShard(t *testing.T) {
	groupID := int64(5)
	svc, userRepo, shardRepo, apiKeyRepo := newDefaultAccessServiceForTest(groupID, nil)

	access, err := svc.ProvisionDefaultGatewayAccess(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, &DefaultGatewayAccess{GroupID: groupID, ShardID: 12, APIKeyID: 99}, access)
	require.Equal(t, []struct {
		userID  int64
		groupID int64
	}{{userID: 7, groupID: groupID}}, userRepo.addGroupCalls)
	require.Equal(t, []struct {
		groupID  int64
		userID   int64
		maxUsers int
	}{{groupID: groupID, userID: 7, maxUsers: 2}}, shardRepo.ensureCalls)
	require.Len(t, apiKeyRepo.created, 1)
	require.Equal(t, defaultGatewayAPIKeyName, apiKeyRepo.created[0].Name)
	require.Equal(t, groupID, *apiKeyRepo.created[0].GroupID)
}

func TestDefaultGatewayAccessService_ProvisionReusesExistingDefaultGroupKey(t *testing.T) {
	groupID := int64(5)
	existing := []APIKey{{ID: 66, UserID: 7, Key: "sk-existing", Name: "existing", GroupID: &groupID}}
	svc, _, _, apiKeyRepo := newDefaultAccessServiceForTest(groupID, existing)

	access, err := svc.ProvisionDefaultGatewayAccess(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, int64(66), access.APIKeyID)
	require.Empty(t, apiKeyRepo.created)
}

func TestDefaultGatewayAccessService_RequireDefaultShardGroupRejectsMissingGroup(t *testing.T) {
	svc := NewDefaultGatewayAccessService(&defaultAccessGroupRepoStub{}, nil, nil, nil)

	_, err := svc.RequireDefaultShardGroup(context.Background())
	require.ErrorIs(t, err, ErrDefaultShardGroupNotConfigured)
}
