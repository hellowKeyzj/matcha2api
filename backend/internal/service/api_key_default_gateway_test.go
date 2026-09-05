//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type defaultGatewayGroupRepoStub struct {
	GroupRepository
	group       *Group
	err         error
	calls       int
	createCalls int
}

func (r *defaultGatewayGroupRepoStub) GetDefaultShardGroup(context.Context) (*Group, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	if r.group == nil {
		return nil, nil
	}
	clone := *r.group
	return &clone, nil
}

func (r *defaultGatewayGroupRepoStub) Create(context.Context, *Group) error {
	r.createCalls++
	return nil
}

type defaultGatewayAPIKeyRepoStub struct {
	APIKeyRepository
	keys        []APIKey
	listErr     error
	listCalls   int
	createCalls int
	userID      int64
	params      pagination.PaginationParams
	filters     APIKeyListFilters
}

func (r *defaultGatewayAPIKeyRepoStub) ListByUserID(_ context.Context, userID int64, params pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	r.listCalls++
	r.userID = userID
	r.params = params
	r.filters = filters
	if r.listErr != nil {
		return nil, nil, r.listErr
	}
	out := make([]APIKey, len(r.keys))
	copy(out, r.keys)
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func (r *defaultGatewayAPIKeyRepoStub) Create(context.Context, *APIKey) error {
	r.createCalls++
	return nil
}

func TestAPIKeyServiceGetDefaultGatewayAPIKeyReturnsUsableKeyBoundToDefaultShardGroup(t *testing.T) {
	groupID := int64(42)
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)
	groupRepo := &defaultGatewayGroupRepoStub{group: &Group{
		ID:               groupID,
		Name:             "default-supply",
		Platform:         PlatformComposite,
		Status:           StatusActive,
		ShardEnabled:     true,
		SubscriptionType: SubscriptionTypeSubscriptionBalance,
	}}
	apiKeyRepo := &defaultGatewayAPIKeyRepoStub{keys: []APIKey{
		{ID: 1, UserID: 7, Key: "sk-expired", Status: StatusActive, GroupID: &groupID, ExpiresAt: &past},
		{ID: 2, UserID: 7, Key: "sk-usable", Name: "Default API Key", Status: StatusActive, GroupID: &groupID, ExpiresAt: &future, IPWhitelist: []string{"192.0.2.0/24"}},
	}}
	svc := NewAPIKeyService(apiKeyRepo, nil, groupRepo, nil, nil, nil, nil)

	got, err := svc.GetDefaultGatewayAPIKey(context.Background(), 7)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, int64(2), got.ID)
	require.Equal(t, "sk-usable", got.Key)
	require.NotNil(t, got.Group)
	require.Equal(t, groupID, got.Group.ID)
	require.NotNil(t, got.CompiledIPWhitelist)
	require.Equal(t, 1, groupRepo.calls)
	require.Equal(t, 1, apiKeyRepo.listCalls)
	require.Equal(t, int64(7), apiKeyRepo.userID)
	require.Equal(t, pagination.PaginationParams{Page: 1, PageSize: 10, SortBy: "created_at", SortOrder: pagination.SortOrderAsc}, apiKeyRepo.params)
	require.Equal(t, StatusActive, apiKeyRepo.filters.Status)
	require.NotNil(t, apiKeyRepo.filters.GroupID)
	require.Equal(t, groupID, *apiKeyRepo.filters.GroupID)
	require.Zero(t, groupRepo.createCalls)
	require.Zero(t, apiKeyRepo.createCalls)
}

func TestAPIKeyServiceGetDefaultGatewayAPIKeyUnavailableDoesNotMutate(t *testing.T) {
	groupID := int64(42)
	otherGroupID := int64(43)
	past := time.Now().Add(-time.Hour)
	activeDefaultGroup := &Group{ID: groupID, Status: StatusActive, ShardEnabled: true}

	tests := []struct {
		name          string
		group         *Group
		groupErr      error
		keys          []APIKey
		wantListCalls int
	}{
		{
			name:     "missing default shard group",
			groupErr: ErrDefaultShardGroupNotConfigured,
		},
		{
			name: "nil default shard group",
		},
		{
			name:  "default group disabled",
			group: &Group{ID: groupID, Status: StatusDisabled, ShardEnabled: true},
		},
		{
			name:  "default group not shard enabled",
			group: &Group{ID: groupID, Status: StatusActive},
		},
		{
			name:  "only unusable keys",
			group: activeDefaultGroup,
			keys: []APIKey{
				{ID: 1, UserID: 7, Key: "sk-disabled", Status: StatusDisabled, GroupID: &groupID},
				{ID: 2, UserID: 7, Key: "sk-expired", Status: StatusActive, GroupID: &groupID, ExpiresAt: &past},
				{ID: 3, UserID: 7, Key: "sk-exhausted", Status: StatusActive, GroupID: &groupID, Quota: 1, QuotaUsed: 1},
				{ID: 4, UserID: 7, Key: "sk-other-group", Status: StatusActive, GroupID: &otherGroupID},
			},
			wantListCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groupRepo := &defaultGatewayGroupRepoStub{group: tt.group, err: tt.groupErr}
			apiKeyRepo := &defaultGatewayAPIKeyRepoStub{keys: tt.keys}
			svc := NewAPIKeyService(apiKeyRepo, nil, groupRepo, nil, nil, nil, nil)

			got, err := svc.GetDefaultGatewayAPIKey(context.Background(), 7)

			require.Nil(t, got)
			require.ErrorIs(t, err, ErrGatewayAPIKeyUnavailable)
			require.Equal(t, tt.wantListCalls, apiKeyRepo.listCalls)
			require.Zero(t, groupRepo.createCalls)
			require.Zero(t, apiKeyRepo.createCalls)
		})
	}
}

func TestAPIKeyServiceGetDefaultGatewayAPIKeyRejectsInvalidUserID(t *testing.T) {
	svc := NewAPIKeyService(&defaultGatewayAPIKeyRepoStub{}, nil, &defaultGatewayGroupRepoStub{}, nil, nil, nil, nil)

	got, err := svc.GetDefaultGatewayAPIKey(context.Background(), 0)

	require.Nil(t, got)
	require.ErrorIs(t, err, ErrUserNotFound)
}
