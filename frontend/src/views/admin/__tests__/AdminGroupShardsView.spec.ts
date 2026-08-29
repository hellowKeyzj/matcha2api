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
      createShard: vi.fn(),
      updateShard: vi.fn(),
      deleteShard: vi.fn(),
      assignShardUser: vi.fn(),
      removeShardUser: vi.fn(),
      setShardAccounts: vi.fn(),
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

const AppLayoutStub = {
  template: '<div><slot /></div>',
}

const TablePageLayoutStub = {
  template: `
    <div>
      <slot name="filters" />
      <slot name="table" />
    </div>
  `,
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
        TablePageLayout: TablePageLayoutStub,
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
})
