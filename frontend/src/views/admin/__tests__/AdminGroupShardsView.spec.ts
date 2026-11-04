import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminGroup, GroupShard, GroupShardAccount, GroupShardUser } from '@/types'
import AdminGroupShardsView from '../AdminGroupShardsView.vue'

const {
  routeMock,
  routerPush,
  getGroupById,
  listShards,
  listShardUsers,
  listShardAccounts,
  listSharedAccounts,
  setSharedAccounts,
  setShardAccounts,
  removeShardUser,
  listUsers,
  listAccounts,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  routeMock: { params: { id: '10' } },
  routerPush: vi.fn(),
  getGroupById: vi.fn(),
  listShards: vi.fn(),
  listShardUsers: vi.fn(),
  listShardAccounts: vi.fn(),
  listSharedAccounts: vi.fn(),
  setSharedAccounts: vi.fn(),
  setShardAccounts: vi.fn(),
  removeShardUser: vi.fn(),
  listUsers: vi.fn(),
  listAccounts: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      getById: getGroupById,
      listShards,
      listShardUsers,
      listShardAccounts,
      listSharedAccounts,
      setSharedAccounts,
      createShard: vi.fn(),
      updateShard: vi.fn(),
      deleteShard: vi.fn(),
      assignShardUser: vi.fn(),
      removeShardUser,
      setShardAccounts,
    },
    users: {
      list: listUsers,
    },
    accounts: {
      list: listAccounts,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeMock,
  useRouter: () => ({
    push: routerPush,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'admin.groups.shards.groupSubtitle') return `Group ${params?.name ?? ''}`
        return key
      },
    }),
  }
})

// The view tests exercise the remote-picker contract; real keyboard/portal behavior
// is covered by Select tests and browser acceptance.
const SelectStub = {
  props: ['options', 'disabled'],
  emits: ['search', 'toggle'],
  data: () => ({ open: false }),
  template: `<div>
    <input type="text" :disabled="disabled" @focus="open = true; $emit('search', '')" @keydown.esc="open = false" @focusout="open = false" />
    <template v-if="open"><button v-for="option in options" :key="option.value" @click="$emit('toggle', option)">{{ option.label }}</button><slot name="footer" :close="() => open = false" /></template>
  </div>`,
}

const AppLayoutStub = {
  template: '<div><slot /></div>',
}

const BaseDialogStub = {
  props: ['show'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
}

const IconStub = {
  props: ['name'],
  template: '<span data-test="icon">{{ name }}</span>',
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

const createGroup = (overrides: Partial<AdminGroup> = {}): AdminGroup => ({
  id: 10,
  name: 'Composite Group',
  description: null,
  platform: 'composite',
  rate_multiplier: 1,
  rpm_limit: 0,
  shard_enabled: true,
  shard_max_users_per_shard: 2,
  shard_auto_create: true,
  is_exclusive: false,
  status: 'active',
  subscription_type: 'standard',
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  long_context_pricing_enabled: false,
  allow_image_generation: false,
  allow_batch_image_generation: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  batch_image_discount_multiplier: 1,
  batch_image_hold_multiplier: 1,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  video_rate_independent: false,
  video_rate_multiplier: 1,
  video_price_480p: null,
  video_price_720p: null,
  video_price_1080p: null,
  web_search_price_per_call: null,
  search_price_per_1k: null,
  audio_realtime_price_per_min: null,
  audio_tts_price_per_million_chars: null,
  audio_stt_price_per_hour: null,
  peak_rate_enabled: false,
  peak_start: '00:00',
  peak_end: '00:00',
  peak_rate_multiplier: 1,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_messages_dispatch: false,
  allow_live: false,
  default_mapped_model: '',
  messages_dispatch_model_config: undefined,
  require_oauth_only: false,
  require_privacy_set: false,
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-01T00:00:00Z',
  model_pricing: [],
  profit_control_enabled: false,
  profit_min_margin: 0,
  profit_safety_buffer: 0,
  model_routing: null,
  model_routing_enabled: false,
  mcp_xml_inject: true,
  supported_model_scopes: [],
  account_count: 0,
  active_account_count: 0,
  rate_limited_account_count: 0,
  models_list_config: undefined,
  sort_order: 0,
  ...overrides,
})

const createShard = (id: number, name: string): GroupShard => ({
  id,
  group_id: 10,
  name,
  description: '',
  status: 'active',
  user_count: 1,
  account_count: 1,
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-01T00:00:00Z',
})

const createShardUser = (shardID: number, userID: number, email: string): GroupShardUser => ({
  group_shard_id: shardID,
  group_id: 10,
  user_id: userID,
  email,
  username: email.split('@')[0],
  created_at: '2026-07-01T00:00:00Z',
})

const createShardAccount = (shardID: number, accountID: number, name: string): GroupShardAccount => ({
  group_shard_id: shardID,
  group_id: 10,
  account_id: accountID,
  name,
  platform: 'openai',
  type: 'apikey',
  status: 'active',
  priority: 1,
  created_at: '2026-07-01T00:00:00Z',
})

const mountView = async () => {
  const wrapper = mount(AdminGroupShardsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        Select: SelectStub,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        EmptyState: true,
        PlatformIcon: true,
        Icon: IconStub,
      },
    },
  })
  await flushPromises()
  return wrapper
}

const clickShard = async (wrapper: ReturnType<typeof mount>, name: string) => {
  const button = wrapper.findAll('button').find((item) => item.text().includes(name))
  expect(button, `shard button ${name}`).toBeTruthy()
  await button!.trigger('click')
}

describe('admin AdminGroupShardsView', () => {
  beforeEach(() => {
    routeMock.params.id = '10'
    routerPush.mockReset()
    getGroupById.mockReset()
    listShards.mockReset()
    listShardUsers.mockReset()
    listShardAccounts.mockReset()
    listSharedAccounts.mockReset().mockResolvedValue([])
    setSharedAccounts.mockReset()
    setShardAccounts.mockReset()
    removeShardUser.mockReset().mockResolvedValue({ message: 'removed' })
    listUsers.mockReset()
    listAccounts.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    getGroupById.mockResolvedValue(createGroup())
    listShards.mockResolvedValue([
      createShard(1, 'Shard One'),
      createShard(2, 'Shard Two'),
    ])
    listUsers.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    listAccounts.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 30, pages: 0 })
  })

  it('uses natural page height rather than the clipped table layout', async () => {
    const wrapper = await mountView()
    expect(wrapper.find('[data-testid="group-shards-page"]').exists()).toBe(true)
    expect(wrapper.find('.table-page-layout').exists()).toBe(false)
    expect(wrapper.find('.table-scroll-container').exists()).toBe(false)
    expect(wrapper.findAllComponents({ name: 'GroupShardAccountPoolEditor' })).toHaveLength(2)
  })

  it('switches the workspace between the shared pool and a shard without discarding drafts', async () => {
    listSharedAccounts.mockResolvedValue([createShardAccount(0, 201, 'Shared Account')])
    const wrapper = await mountView()
    const shared = wrapper.findAllComponents({ name: 'GroupShardAccountPoolEditor' })[0]!
    expect(wrapper.find('[data-testid="shared-pool-panel"]').attributes('style')).toContain('display: none')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.groups.shards.sharedNav'))!.trigger('click')
    expect(wrapper.find('[data-testid="shared-pool-panel"]').attributes('style') || '').not.toContain('display: none')
    await shared.find('input[type="number"]').setValue('7')
    await clickShard(wrapper, 'Shard One')
    expect(wrapper.find('[data-testid="shared-pool-panel"]').attributes('style')).toContain('display: none')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.groups.shards.sharedNav'))!.trigger('click')
    expect((shared.find('input[type="number"]').element as HTMLInputElement).value).toBe('7')
  })

  it('bounds shared and exclusive account lists with internal scrolling and sticky headers', async () => {
    const wrapper = await mountView()
    const editors = wrapper.findAllComponents({ name: 'GroupShardAccountPoolEditor' })
    expect(editors).toHaveLength(2)
    for (const editor of editors) {
      const list = editor.find('[data-testid="account-pool-scroll"]')
      expect(list.classes()).toContain('max-h-[min(24rem,50vh)]')
      expect(list.classes()).toContain('overflow-auto')
      const header = list.find('[data-testid="account-pool-header"]')
      expect(header.classes()).toContain('sticky')
      expect(header.classes()).toContain('top-0')
    }
  })

  it('does not let stale shard detail responses overwrite the latest selected shard', async () => {
    const staleUsers = deferred<GroupShardUser[]>()
    const staleAccounts = deferred<GroupShardAccount[]>()

    listShardUsers
      .mockResolvedValueOnce([createShardUser(1, 101, 'initial-one@example.com')])
      .mockReturnValueOnce(staleUsers.promise)
      .mockResolvedValueOnce([createShardUser(1, 103, 'latest-one@example.com')])
    listShardAccounts
      .mockResolvedValueOnce([createShardAccount(1, 201, 'Initial Account')])
      .mockReturnValueOnce(staleAccounts.promise)
      .mockResolvedValueOnce([createShardAccount(1, 203, 'Latest Account')])

    const wrapper = await mountView()

    expect(wrapper.text()).toContain('initial-one@example.com')
    expect(wrapper.text()).toContain('Initial Account')

    await clickShard(wrapper, 'Shard Two')
    await flushPromises()

    const staleUsersSignal = listShardUsers.mock.calls[1]?.[2]?.signal as AbortSignal | undefined
    const staleAccountsSignal = listShardAccounts.mock.calls[1]?.[2]?.signal as AbortSignal | undefined

    await clickShard(wrapper, 'Shard One')
    await flushPromises()

    expect(staleUsersSignal?.aborted).toBe(true)
    expect(staleAccountsSignal?.aborted).toBe(true)
    expect(wrapper.text()).toContain('latest-one@example.com')
    expect(wrapper.text()).toContain('Latest Account')

    staleUsers.resolve([createShardUser(2, 102, 'stale-two@example.com')])
    staleAccounts.resolve([createShardAccount(2, 202, 'Stale Account')])
    await flushPromises()

    expect(wrapper.text()).toContain('latest-one@example.com')
    expect(wrapper.text()).toContain('Latest Account')
    expect(wrapper.text()).not.toContain('stale-two@example.com')
    expect(wrapper.text()).not.toContain('Stale Account')
    expect(showError).not.toHaveBeenCalled()
  })

  it('edits and saves the parent shared pool with no shards using parent provider candidates', async () => {
    listShards.mockResolvedValue([])
    getGroupById.mockResolvedValue(createGroup({ platform: 'openai' }))
    listSharedAccounts.mockResolvedValue([createShardAccount(0, 201, 'Shared Account')])
    setSharedAccounts.mockResolvedValue([createShardAccount(0, 201, 'Saved Shared Account')])
    listAccounts.mockResolvedValue({ items: [{ id: 202, name: 'Candidate', platform: 'openai', type: 'apikey', status: 'active' }] })
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('admin.groups.shards.sharedAccountSectionTitle')
    expect(wrapper.text()).toContain('Shared Account')
    expect(listSharedAccounts).toHaveBeenCalledWith(10)
    expect(listShardAccounts).not.toHaveBeenCalled()
    const search = wrapper.find('input[type="text"]')
    await search.trigger('focus')
    await flushPromises()
    expect(listAccounts).toHaveBeenCalledWith(1, 100, expect.objectContaining({ group: '10', platform: 'openai', lite: 'true' }), expect.anything())
    await wrapper.find('input[type="number"]').setValue('7')
    await wrapper.findAll('button').find((button) => button.text().includes('admin.groups.shards.saveAccounts'))!.trigger('click')
    await flushPromises()
    expect(setSharedAccounts).toHaveBeenCalledWith(10, [{ account_id: 201, priority: 7 }])
    expect(wrapper.text()).toContain('Saved Shared Account')
  })

  it('does not let an old candidate search failure clear a newer result', async () => {
    listShards.mockResolvedValue([])
    getGroupById.mockResolvedValue(createGroup({ platform: 'openai' }))
    const stale = deferred<{ items: [] }>()
    const current = deferred<{ items: { id: number; name: string; platform: string; type: string; status: string }[] }>()
    listAccounts.mockReturnValueOnce(stale.promise).mockReturnValueOnce(current.promise)
    const wrapper = await mountView()
    const search = wrapper.find('input[type="text"]')
    await search.trigger('focus')
    await search.trigger('focus')
    current.resolve({ items: [{ id: 203, name: 'Current Candidate', platform: 'openai', type: 'apikey', status: 'active' }] })
    await flushPromises()
    expect(wrapper.text()).toContain('Current Candidate')
    stale.reject(new Error('old search failed'))
    await flushPromises()
    expect(wrapper.text()).toContain('Current Candidate')
  })

  it('excludes saved and draft accounts across shared and exclusive editors', async () => {
    getGroupById.mockResolvedValue(createGroup({ platform: 'openai' }))
    listSharedAccounts.mockResolvedValue([createShardAccount(0, 201, 'Shared Account')])
    listShardUsers.mockResolvedValue([])
    listShardAccounts.mockResolvedValue([createShardAccount(1, 202, 'Exclusive Account')])
    listAccounts.mockResolvedValue({ items: [201, 202, 203].map((id) => ({ id, name: `Candidate ${id}`, platform: 'openai', type: 'apikey', status: 'active' })) })
    const wrapper = await mountView()
    const editors = wrapper.findAllComponents({ name: 'GroupShardAccountPoolEditor' })
    expect(editors).toHaveLength(2)
    for (const editor of editors) {
      await editor.find('input[type="text"]').trigger('focus')
      await flushPromises()
      expect(editor.text()).not.toContain('Candidate 201')
      expect(editor.text()).not.toContain('Candidate 202')
      expect(editor.text()).toContain('Candidate 203')
    }
    await editors[0]!.findAll('button').find((button) => button.text().includes('Candidate 203'))!.trigger('click')
    await editors[0]!.findAll('button').find((button) => button.text().includes('admin.groups.shards.addSelectedAccounts'))!.trigger('click')
    expect(editors[1]!.text()).not.toContain('Candidate 203')
    // A pending removal must be saved before the binding can change pool.
    await editors[0]!.find('button[title="common.remove"]').trigger('click')
    await editors[1]!.find('input[type="text"]').trigger('focus')
    await flushPromises()
    expect(editors[1]!.text()).not.toContain('Candidate 201')
  })

  it('surfaces backend shared binding conflicts without discarding the draft', async () => {
    listShards.mockResolvedValue([])
    listSharedAccounts.mockResolvedValue([createShardAccount(0, 201, 'Shared Account')])
    setSharedAccounts.mockRejectedValue({ response: { data: { detail: 'Account is bound to another shard' } } })
    const wrapper = await mountView()
    await wrapper.findAll('button').find((button) => button.text().includes('admin.groups.shards.saveAccounts'))!.trigger('click')
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('Account is bound to another shard')
    expect(wrapper.text()).toContain('Shared Account')
  })

  it('adds loaded eligible accounts in one batch only after confirmation', async () => {
    listShards.mockResolvedValue([])
    getGroupById.mockResolvedValue(createGroup({ platform: 'openai' }))
    listSharedAccounts.mockResolvedValue([createShardAccount(0, 201, 'Existing')])
    listAccounts.mockResolvedValue({ items: [201, 202, 203].map(id => ({ id, name: `Candidate ${id}`, platform: 'openai', type: 'apikey', status: 'active' })) })
    const wrapper = await mountView()
    const editor = wrapper.findComponent({ name: 'GroupShardAccountPoolEditor' })
    await editor.find('input[type="text"]').trigger('focus')
    await flushPromises()
    await editor.findAll('button').find(button => button.text().includes('selectLoadedAccounts'))!.trigger('click')
    expect(editor.findAll('input[type="number"]')).toHaveLength(1)
    const addButton = () => editor.findAll('button').find(button => button.text().includes('addSelectedAccounts'))!
    expect(addButton().attributes('disabled')).toBeUndefined()
    await editor.findAll('button').find(button => button.text().includes('clearSelectedAccounts'))!.trigger('click')
    expect(addButton().attributes('disabled')).toBeDefined()
    expect(editor.findAll('input[type="number"]')).toHaveLength(1)
    await editor.findAll('button').find(button => button.text().includes('selectLoadedAccounts'))!.trigger('click')
    await editor.findAll('button').find(button => button.text().includes('addSelectedAccounts'))!.trigger('click')
    expect(editor.findAll('input[type="number"]')).toHaveLength(3)
    expect(setSharedAccounts).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('loads all candidate pages before allowing a batch selection', async () => {
    listShards.mockResolvedValue([])
    getGroupById.mockResolvedValue(createGroup({ platform: 'openai' }))
    listAccounts.mockImplementation((page) => Promise.resolve({
      items: [{ id: page === 1 ? 202 : 250, name: `Page ${page} account`, platform: 'openai', type: 'apikey', status: 'active' }],
      pages: 2,
    }))
    const wrapper = await mountView()
    const editor = wrapper.findComponent({ name: 'GroupShardAccountPoolEditor' })
    await editor.find('input[type="text"]').trigger('focus')
    await flushPromises()
    expect(listAccounts).toHaveBeenCalledWith(2, 100, expect.anything(), expect.anything())
    expect(editor.text()).toContain('Page 2 account')
    await editor.findAll('button').find(button => button.text().includes('selectLoadedAccounts'))!.trigger('click')
    await editor.findAll('button').find(button => button.text().includes('addSelectedAccounts'))!.trigger('click')
    expect(editor.findAll('input[type="number"]')).toHaveLength(2)
    wrapper.unmount()
  })

  it('preserves per-shard exclusive saves and shared pool state', async () => {
    listSharedAccounts.mockResolvedValue([createShardAccount(0, 201, 'Shared Account')])
    listShardUsers.mockResolvedValue([])
    listShardAccounts.mockResolvedValue([createShardAccount(1, 202, 'Exclusive Account')])
    setShardAccounts.mockResolvedValue([createShardAccount(1, 202, 'Saved Exclusive Account')])
    const wrapper = await mountView()
    const editor = wrapper.findAllComponents({ name: 'GroupShardAccountPoolEditor' })[1]!
    await editor.find('input[type="number"]').setValue('9')
    await editor.findAll('button').find((button) => button.text().includes('admin.groups.shards.saveAccounts'))!.trigger('click')
    await flushPromises()
    expect(setShardAccounts).toHaveBeenCalledWith(10, 1, [{ account_id: 202, priority: 9 }])
    expect(setSharedAccounts).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Shared Account')
    expect(wrapper.text()).toContain('Saved Exclusive Account')
  })

  it('does not overwrite the selected shard with a previous shard save response', async () => {
    listShardUsers.mockResolvedValue([])
    listShardAccounts.mockImplementation((_group, shard) => Promise.resolve([
      createShardAccount(shard, 200 + shard, `Account for shard ${shard}`),
    ]))
    const pending = deferred<ReturnType<typeof createShardAccount>[]>()
    setShardAccounts.mockReturnValueOnce(pending.promise)
    const wrapper = await mountView()
    const editor = wrapper.findAllComponents({ name: 'GroupShardAccountPoolEditor' })[1]!
    await editor.findAll('button').find((button) => button.text().includes('admin.groups.shards.saveAccounts'))!.trigger('click')
    await clickShard(wrapper, 'Shard Two')
    await flushPromises()
    pending.resolve([createShardAccount(1, 201, 'Saved previous shard')])
    await flushPromises()
    expect(wrapper.text()).toContain('Account for shard 2')
    expect(wrapper.text()).not.toContain('Saved previous shard')
  })

  it.each([true, false])('reloads valid selection after last-user shard deletion (remaining shard: %s)', async (hasRemainingShard) => {
    listShardUsers.mockImplementation((_group, shard) => Promise.resolve([createShardUser(shard, 101, 'last@example.com')]))
    listShardAccounts.mockResolvedValue([])
    const wrapper = await mountView()
    listShards.mockResolvedValue(hasRemainingShard ? [createShard(2, 'Shard Two')] : [])
    listShardUsers.mockClear()
    listShardAccounts.mockClear()
    await wrapper.findAll('button').find((button) => button.text().includes('common.remove'))!.trigger('click')
    await flushPromises()
    expect(removeShardUser).toHaveBeenCalledWith(10, 1, 101)
    expect(listShardUsers.mock.calls.every((call) => call[1] === 2)).toBe(true)
    expect(listShardAccounts.mock.calls.every((call) => call[1] === 2)).toBe(true)
    expect(listShardUsers).toHaveBeenCalledTimes(hasRemainingShard ? 1 : 0)
    expect(wrapper.text()).not.toContain('Shard One')
    expect(showError).not.toHaveBeenCalled()
  })

})
