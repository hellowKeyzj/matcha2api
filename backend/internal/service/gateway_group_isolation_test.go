//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Part 1: isAccountInGroup 单元测试
// ============================================================================

func TestIsAccountInGroup(t *testing.T) {
	svc := &GatewayService{}
	groupID100 := int64(100)
	groupID200 := int64(200)

	tests := []struct {
		name     string
		account  *Account
		groupID  *int64
		expected bool
	}{
		// groupID == nil（无分组 API Key）
		{
			"nil_groupID_ungrouped_account_nil_groups",
			&Account{ID: 1, AccountGroups: nil},
			nil, true,
		},
		{
			"nil_groupID_ungrouped_account_empty_slice",
			&Account{ID: 2, AccountGroups: []AccountGroup{}},
			nil, true,
		},
		{
			"nil_groupID_grouped_account_single",
			&Account{ID: 3, AccountGroups: []AccountGroup{{GroupID: 100}}},
			nil, false,
		},
		{
			"nil_groupID_grouped_account_multiple",
			&Account{ID: 4, AccountGroups: []AccountGroup{{GroupID: 100}, {GroupID: 200}}},
			nil, false,
		},
		// groupID != nil（有分组 API Key）
		{
			"with_groupID_account_in_group",
			&Account{ID: 5, AccountGroups: []AccountGroup{{GroupID: 100}}},
			&groupID100, true,
		},
		{
			"with_groupID_account_not_in_group",
			&Account{ID: 6, AccountGroups: []AccountGroup{{GroupID: 200}}},
			&groupID100, false,
		},
		{
			"with_groupID_ungrouped_account",
			&Account{ID: 7, AccountGroups: nil},
			&groupID100, false,
		},
		{
			"with_groupID_multi_group_account_match_one",
			&Account{ID: 8, AccountGroups: []AccountGroup{{GroupID: 100}, {GroupID: 200}}},
			&groupID200, true,
		},
		{
			"with_groupID_multi_group_account_no_match",
			&Account{ID: 9, AccountGroups: []AccountGroup{{GroupID: 300}, {GroupID: 400}}},
			&groupID100, false,
		},
		// 防御性边界
		{
			"nil_account_nil_groupID",
			nil,
			nil, false,
		},
		{
			"nil_account_with_groupID",
			nil,
			&groupID100, false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.isAccountInGroup(tt.account, tt.groupID)
			require.Equal(t, tt.expected, got, "isAccountInGroup 结果不符预期")
		})
	}
}

// ============================================================================
// Part 2: 分组隔离端到端调度测试
// ============================================================================

// groupAwareMockAccountRepo 嵌入 mockAccountRepoForPlatform，覆写分组隔离相关方法。
// allAccounts 存储所有账号，分组查询方法按 AccountGroups 字段进行真实过滤。
type groupAwareMockAccountRepo struct {
	*mockAccountRepoForPlatform
	allAccounts []Account
}

// ListSchedulableUngroupedByPlatform 仅返回未分组账号（AccountGroups 为空）
func (m *groupAwareMockAccountRepo) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range m.allAccounts {
		if acc.Platform == platform && acc.IsSchedulable() && len(acc.AccountGroups) == 0 {
			result = append(result, acc)
		}
	}
	return result, nil
}

// ListSchedulableUngroupedByPlatforms 仅返回未分组账号（多平台版本）
func (m *groupAwareMockAccountRepo) ListSchedulableUngroupedByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	platformSet := make(map[string]bool, len(platforms))
	for _, p := range platforms {
		platformSet[p] = true
	}
	var result []Account
	for _, acc := range m.allAccounts {
		if platformSet[acc.Platform] && acc.IsSchedulable() && len(acc.AccountGroups) == 0 {
			result = append(result, acc)
		}
	}
	return result, nil
}

// ListSchedulableByGroupIDAndPlatform 返回属于指定分组的账号
func (m *groupAwareMockAccountRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range m.allAccounts {
		if acc.Platform == platform && acc.IsSchedulable() && accountBelongsToGroup(acc, groupID) {
			result = append(result, acc)
		}
	}
	return result, nil
}

// ListSchedulableByGroupIDAndPlatforms 返回属于指定分组的账号（多平台版本）
func (m *groupAwareMockAccountRepo) ListSchedulableByGroupIDAndPlatforms(ctx context.Context, groupID int64, platforms []string) ([]Account, error) {
	platformSet := make(map[string]bool, len(platforms))
	for _, p := range platforms {
		platformSet[p] = true
	}
	var result []Account
	for _, acc := range m.allAccounts {
		if platformSet[acc.Platform] && acc.IsSchedulable() && accountBelongsToGroup(acc, groupID) {
			result = append(result, acc)
		}
	}
	return result, nil
}

// accountBelongsToGroup 检查账号是否属于指定分组
func accountBelongsToGroup(acc Account, groupID int64) bool {
	for _, ag := range acc.AccountGroups {
		if ag.GroupID == groupID {
			return true
		}
	}
	return false
}

// Verify interface implementation
var _ AccountRepository = (*groupAwareMockAccountRepo)(nil)

// newGroupAwareMockRepo 创建分组感知的 mock repo
func newGroupAwareMockRepo(accounts []Account) *groupAwareMockAccountRepo {
	byID := make(map[int64]*Account, len(accounts))
	for i := range accounts {
		byID[accounts[i].ID] = &accounts[i]
	}
	return &groupAwareMockAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accounts:     accounts,
			accountsByID: byID,
		},
		allAccounts: accounts,
	}
}

func TestGroupIsolation_UngroupedKey_ShouldNotScheduleGroupedAccounts(t *testing.T) {
	// 场景：无分组 API Key（groupID=nil），池中只有已分组账号 → 应返回错误
	ctx := context.Background()

	accounts := []Account{
		{ID: 1, Platform: PlatformOpenAI, Priority: 1, Status: StatusActive, Schedulable: true,
			AccountGroups: []AccountGroup{{GroupID: 100}}},
		{ID: 2, Platform: PlatformOpenAI, Priority: 2, Status: StatusActive, Schedulable: true,
			AccountGroups: []AccountGroup{{GroupID: 200}}},
	}
	repo := newGroupAwareMockRepo(accounts)
	cache := &mockGatewayCacheForPlatform{}

	svc := &GatewayService{
		accountRepo: repo,
		cache:       cache,
		cfg:         testConfig(),
	}

	acc, err := svc.selectAccountForModelWithPlatform(ctx, nil, "", "", nil, PlatformOpenAI)
	require.Error(t, err, "无分组 Key 不应调度到已分组账号")
	require.Nil(t, acc)
}

func TestGroupIsolation_GroupedKey_ShouldNotScheduleUngroupedAccounts(t *testing.T) {
	// 场景：有分组 API Key（groupID=100），池中只有未分组账号 → 应返回错误
	ctx := context.Background()
	groupID := int64(100)

	accounts := []Account{
		{ID: 1, Platform: PlatformOpenAI, Priority: 1, Status: StatusActive, Schedulable: true,
			AccountGroups: nil},
		{ID: 2, Platform: PlatformOpenAI, Priority: 2, Status: StatusActive, Schedulable: true,
			AccountGroups: []AccountGroup{}},
	}
	repo := newGroupAwareMockRepo(accounts)
	cache := &mockGatewayCacheForPlatform{}

	svc := &GatewayService{
		accountRepo: repo,
		cache:       cache,
		cfg:         testConfig(),
	}

	acc, err := svc.selectAccountForModelWithPlatform(ctx, &groupID, "", "", nil, PlatformOpenAI)
	require.Error(t, err, "有分组 Key 不应调度到未分组账号")
	require.Nil(t, acc)
}

func TestGroupIsolation_UngroupedKey_ShouldOnlyScheduleUngroupedAccounts(t *testing.T) {
	// 场景：无分组 API Key（groupID=nil），池中有未分组和已分组账号 → 应只选中未分组的
	ctx := context.Background()

	accounts := []Account{
		{ID: 1, Platform: PlatformOpenAI, Priority: 1, Status: StatusActive, Schedulable: true,
			AccountGroups: []AccountGroup{{GroupID: 100}}}, // 已分组，不应被选中
		{ID: 2, Platform: PlatformOpenAI, Priority: 2, Status: StatusActive, Schedulable: true,
			AccountGroups: nil}, // 未分组，应被选中
		{ID: 3, Platform: PlatformOpenAI, Priority: 3, Status: StatusActive, Schedulable: true,
			AccountGroups: []AccountGroup{{GroupID: 200}}}, // 已分组，不应被选中
	}
	repo := newGroupAwareMockRepo(accounts)
	cache := &mockGatewayCacheForPlatform{}

	svc := &GatewayService{
		accountRepo: repo,
		cache:       cache,
		cfg:         testConfig(),
	}

	acc, err := svc.selectAccountForModelWithPlatform(ctx, nil, "", "", nil, PlatformOpenAI)
	require.NoError(t, err, "应成功调度未分组账号")
	require.NotNil(t, acc)
	require.Equal(t, int64(2), acc.ID, "应选中未分组的账号 ID=2")
}

func TestGroupIsolation_GroupedKey_ShouldOnlyScheduleMatchingGroupAccounts(t *testing.T) {
	// 场景：有分组 API Key（groupID=100），池中有未分组和多个分组账号 → 应只选中分组 100 内的
	ctx := context.Background()
	groupID := int64(100)

	accounts := []Account{
		{ID: 1, Platform: PlatformOpenAI, Priority: 1, Status: StatusActive, Schedulable: true,
			AccountGroups: nil}, // 未分组，不应被选中
		{ID: 2, Platform: PlatformOpenAI, Priority: 2, Status: StatusActive, Schedulable: true,
			AccountGroups: []AccountGroup{{GroupID: 200}}}, // 属于分组 200，不应被选中
		{ID: 3, Platform: PlatformOpenAI, Priority: 3, Status: StatusActive, Schedulable: true,
			AccountGroups: []AccountGroup{{GroupID: 100}}}, // 属于分组 100，应被选中
	}
	repo := newGroupAwareMockRepo(accounts)
	cache := &mockGatewayCacheForPlatform{}

	svc := &GatewayService{
		accountRepo: repo,
		cache:       cache,
		cfg:         testConfig(),
	}

	acc, err := svc.selectAccountForModelWithPlatform(ctx, &groupID, "", "", nil, PlatformOpenAI)
	require.NoError(t, err, "应成功调度分组内账号")
	require.NotNil(t, acc)
	require.Equal(t, int64(3), acc.ID, "应选中分组 100 内的账号 ID=3")
}

// ============================================================================
// Part 3: SimpleMode 旁路测试
// ============================================================================

func TestGroupIsolation_SimpleMode_SkipsGroupIsolation(t *testing.T) {
	// SimpleMode 应跳过分组隔离，使用 ListSchedulableByPlatform 返回所有账号。
	// 测试非 useMixed 路径（platform=openai，不会触发 mixed 调度逻辑）。
	ctx := context.Background()

	// 混合未分组和已分组账号，SimpleMode 下应全部可调度
	accounts := []Account{
		{ID: 1, Platform: PlatformOpenAI, Priority: 2, Status: StatusActive, Schedulable: true,
			AccountGroups: []AccountGroup{{GroupID: 100}}}, // 已分组
		{ID: 2, Platform: PlatformOpenAI, Priority: 1, Status: StatusActive, Schedulable: true,
			AccountGroups: nil}, // 未分组
	}

	// 使用基础 mock（ListSchedulableByPlatform 返回所有匹配平台的账号，不做分组过滤）
	byID := make(map[int64]*Account, len(accounts))
	for i := range accounts {
		byID[accounts[i].ID] = &accounts[i]
	}
	repo := &mockAccountRepoForPlatform{
		accounts:     accounts,
		accountsByID: byID,
	}
	cache := &mockGatewayCacheForPlatform{}

	svc := &GatewayService{
		accountRepo: repo,
		cache:       cache,
		cfg:         &config.Config{RunMode: config.RunModeSimple},
	}

	// groupID=nil 时，SimpleMode 应使用 ListSchedulableByPlatform（不过滤分组）
	acc, err := svc.selectAccountForModelWithPlatform(ctx, nil, "", "", nil, PlatformOpenAI)
	require.NoError(t, err, "SimpleMode 应跳过分组隔离直接返回账号")
	require.NotNil(t, acc)
	// 应选择优先级最高的账号（Priority=1, ID=2），即使它未分组
	require.Equal(t, int64(2), acc.ID, "SimpleMode 应按优先级选择，不考虑分组")
}

func TestGroupIsolation_SimpleMode_GroupedAccountAlsoSchedulable(t *testing.T) {
	// SimpleMode + groupID=nil 时，已分组账号也应该可被调度
	ctx := context.Background()

	// 只有已分组账号，在 standard 模式下 groupID=nil 会报错，但 simple 模式应正常
	accounts := []Account{
		{ID: 1, Platform: PlatformOpenAI, Priority: 1, Status: StatusActive, Schedulable: true,
			AccountGroups: []AccountGroup{{GroupID: 100}}},
	}

	byID := make(map[int64]*Account, len(accounts))
	for i := range accounts {
		byID[accounts[i].ID] = &accounts[i]
	}
	repo := &mockAccountRepoForPlatform{
		accounts:     accounts,
		accountsByID: byID,
	}
	cache := &mockGatewayCacheForPlatform{}

	svc := &GatewayService{
		accountRepo: repo,
		cache:       cache,
		cfg:         &config.Config{RunMode: config.RunModeSimple},
	}

	acc, err := svc.selectAccountForModelWithPlatform(ctx, nil, "", "", nil, PlatformOpenAI)
	require.NoError(t, err, "SimpleMode 下已分组账号也应可调度")
	require.NotNil(t, acc)
	require.Equal(t, int64(1), acc.ID, "SimpleMode 应能调度已分组账号")
}

type testGroupShardRepo struct {
	scope            *GroupShardAccountScope
	scopesByPlatform map[string]*GroupShardAccountScope
	lastGroupID      int64
	lastUserID       int64
	lastPlatform     string
	listScopeCalls   int
	err              error
}

func (r *testGroupShardRepo) ListByGroup(ctx context.Context, groupID int64) ([]GroupShard, error) {
	return nil, nil
}
func (r *testGroupShardRepo) GetByID(ctx context.Context, groupID, shardID int64) (*GroupShard, error) {
	return nil, nil
}
func (r *testGroupShardRepo) Create(ctx context.Context, shard *GroupShard) error      { return nil }
func (r *testGroupShardRepo) Update(ctx context.Context, shard *GroupShard) error      { return nil }
func (r *testGroupShardRepo) Delete(ctx context.Context, groupID, shardID int64) error { return nil }
func (r *testGroupShardRepo) ListUsers(ctx context.Context, groupID, shardID int64) ([]GroupShardUser, error) {
	return nil, nil
}
func (r *testGroupShardRepo) AssignUser(ctx context.Context, groupID, shardID, userID int64, maxUsers int) error {
	return nil
}
func (r *testGroupShardRepo) RemoveUser(ctx context.Context, groupID, shardID, userID int64) error {
	return nil
}
func (r *testGroupShardRepo) ListAccounts(ctx context.Context, groupID, shardID int64) ([]GroupShardAccount, error) {
	return nil, nil
}
func (r *testGroupShardRepo) SetAccounts(ctx context.Context, groupID, shardID int64, accounts []GroupShardAccount) error {
	return nil
}
func (r *testGroupShardRepo) EnsureUserShard(ctx context.Context, group *Group, userID int64) (*GroupShard, error) {
	return nil, nil
}
func (r *testGroupShardRepo) ListAccountIDsForUserShard(ctx context.Context, group *Group, userID int64, platform string) (*GroupShardAccountScope, error) {
	r.listScopeCalls++
	if group != nil {
		r.lastGroupID = group.ID
	}
	r.lastUserID = userID
	r.lastPlatform = platform
	if r.err != nil {
		return nil, r.err
	}
	if r.scopesByPlatform != nil {
		if scope, ok := r.scopesByPlatform[platform]; ok {
			return cloneTestShardScope(scope), nil
		}
	}
	return cloneTestShardScope(r.scope), nil
}
func (r *testGroupShardRepo) AccountBelongsToUserShard(ctx context.Context, groupID, userID, accountID int64) (bool, error) {
	return r.scope.Allows(accountID), nil
}

func cloneTestShardScope(scope *GroupShardAccountScope) *GroupShardAccountScope {
	if scope == nil {
		return &GroupShardAccountScope{Enabled: false}
	}
	cloned := &GroupShardAccountScope{Enabled: scope.Enabled, ShardID: scope.ShardID}
	if scope.AccountIDs != nil {
		cloned.AccountIDs = make(map[int64]struct{}, len(scope.AccountIDs))
		for id := range scope.AccountIDs {
			cloned.AccountIDs[id] = struct{}{}
		}
	}
	return cloned
}

var _ GroupShardRepository = (*testGroupShardRepo)(nil)

func TestGroupShardAccountScope_AllowsAndEmpty(t *testing.T) {
	require.True(t, (*GroupShardAccountScope)(nil).Allows(1))
	require.False(t, (*GroupShardAccountScope)(nil).IsEmpty())

	disabled := &GroupShardAccountScope{Enabled: false, AccountIDs: map[int64]struct{}{}}
	require.True(t, disabled.Allows(1))
	require.False(t, disabled.IsEmpty())

	empty := &GroupShardAccountScope{Enabled: true, ShardID: 9, AccountIDs: map[int64]struct{}{}}
	require.False(t, empty.Allows(1))
	require.True(t, empty.IsEmpty())

	scoped := &GroupShardAccountScope{Enabled: true, ShardID: 9, AccountIDs: map[int64]struct{}{7: {}}}
	require.True(t, scoped.Allows(7))
	require.False(t, scoped.Allows(8))
	require.False(t, scoped.IsEmpty())
}

func TestGroupShardScheduling_LoadAwareFiltersAccountsAndClearsInvalidSticky(t *testing.T) {
	ctx := context.Background()
	groupID := int64(4100)
	userID := int64(5100)
	accounts := []Account{
		{ID: 11, Platform: PlatformAnthropic, Priority: 1, Status: StatusActive, Schedulable: true, Concurrency: 5, AccountGroups: []AccountGroup{{GroupID: groupID}}},
		{ID: 22, Platform: PlatformAnthropic, Priority: 2, Status: StatusActive, Schedulable: true, Concurrency: 5, AccountGroups: []AccountGroup{{GroupID: groupID}}},
	}
	cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"sticky": 11}}
	concurrencyCache := &mockConcurrencyCache{}
	shardRepo := &testGroupShardRepo{scope: &GroupShardAccountScope{Enabled: true, ShardID: 3, AccountIDs: map[int64]struct{}{22: {}}}}
	group := &Group{ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true, ShardEnabled: true}
	svc := &GatewayService{
		accountRepo:        newGroupAwareMockRepo(accounts),
		groupRepo:          &mockGroupRepoForGateway{groups: map[int64]*Group{groupID: group}},
		groupShardRepo:     shardRepo,
		cache:              cache,
		cfg:                testConfig(),
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	result, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, "sticky", "claude-3-5-sonnet-20241022", nil, "", userID)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Account)
	require.Equal(t, int64(22), result.Account.ID)
	require.Equal(t, 1, cache.deletedSessions["sticky"])
	require.Equal(t, int64(22), cache.sessionBindings["sticky"])
	require.Equal(t, userID, shardRepo.lastUserID)
	require.Equal(t, "", shardRepo.lastPlatform)
}

func TestGroupShardScheduling_LoadAwareEmptyShardDoesNotFallbackToParentGroup(t *testing.T) {
	ctx := context.Background()
	groupID := int64(4101)
	userID := int64(5101)
	accounts := []Account{
		{ID: 11, Platform: PlatformAnthropic, Priority: 1, Status: StatusActive, Schedulable: true, Concurrency: 5, AccountGroups: []AccountGroup{{GroupID: groupID}}},
	}
	concurrencyCache := &mockConcurrencyCache{}
	group := &Group{ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true, ShardEnabled: true}
	svc := &GatewayService{
		accountRepo:        newGroupAwareMockRepo(accounts),
		groupRepo:          &mockGroupRepoForGateway{groups: map[int64]*Group{groupID: group}},
		groupShardRepo:     &testGroupShardRepo{scope: &GroupShardAccountScope{Enabled: true, ShardID: 4, AccountIDs: map[int64]struct{}{}}},
		cache:              &mockGatewayCacheForPlatform{},
		cfg:                testConfig(),
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	result, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, "", "claude-3-5-sonnet-20241022", nil, "", userID)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Nil(t, result)
	require.Equal(t, 0, concurrencyCache.loadBatchCalls)
}

func TestGroupShardScheduling_CompositeUsesResolvedProviderForScope(t *testing.T) {
	ctx := context.Background()
	groupID := int64(4102)
	userID := int64(5102)
	accounts := []Account{
		{ID: 31, Platform: PlatformAntigravity, Priority: 1, Status: StatusActive, Schedulable: true, Concurrency: 5, Extra: map[string]any{"mixed_scheduling": true}, AccountGroups: []AccountGroup{{GroupID: groupID}}},
		{ID: 32, Platform: PlatformAnthropic, Priority: 2, Status: StatusActive, Schedulable: true, Concurrency: 5, AccountGroups: []AccountGroup{{GroupID: groupID}}},
	}
	shardRepo := &testGroupShardRepo{scopesByPlatform: map[string]*GroupShardAccountScope{
		PlatformAnthropic: {Enabled: true, ShardID: 5, AccountIDs: map[int64]struct{}{32: {}}},
		"":                {Enabled: true, ShardID: 5, AccountIDs: map[int64]struct{}{31: {}, 32: {}}},
	}}
	group := &Group{ID: groupID, Platform: PlatformComposite, Status: StatusActive, Hydrated: true, ShardEnabled: true}
	svc := &GatewayService{
		accountRepo:        newGroupAwareMockRepo(accounts),
		groupRepo:          &mockGroupRepoForGateway{groups: map[int64]*Group{groupID: group}},
		groupShardRepo:     shardRepo,
		cache:              &mockGatewayCacheForPlatform{},
		cfg:                testConfig(),
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{}),
	}
	ctx = WithCompositeRouteDecision(ctx, CompositeRouteDecision{
		Matched:        true,
		GroupID:        groupID,
		PublicModel:    "claude-alias",
		TargetPlatform: PlatformAnthropic,
		UpstreamModel:  "claude-3-5-sonnet-20241022",
		Endpoint:       CompositeRouteEndpointResponses,
	})

	result, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, "", "claude-alias", nil, "", userID)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Account)
	require.Equal(t, int64(32), result.Account.ID)
	require.Equal(t, PlatformAnthropic, shardRepo.lastPlatform)
}
