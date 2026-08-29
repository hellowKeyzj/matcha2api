import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'

import type { ApiKey, Group } from '@/types'
import { keysAPI } from '@/api'
import KeysView from '../KeysView.vue'

enableAutoUnmount(afterEach)

const {
  listKeys,
  updateKey,
  getPublicSettings,
  getDashboardApiKeysUsage,
  getAvailableGroups,
  getUserGroupRates,
  rotateKey,
  showError,
  showSuccess,
  copyToClipboard,
  isCurrentStep,
  nextStep,
} = vi.hoisted(() => ({
  listKeys: vi.fn(),
  updateKey: vi.fn(),
  getPublicSettings: vi.fn(),
  getDashboardApiKeysUsage: vi.fn(),
  getAvailableGroups: vi.fn(),
  getUserGroupRates: vi.fn(),
  rotateKey: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  copyToClipboard: vi.fn(),
  isCurrentStep: vi.fn(),
  nextStep: vi.fn(),
}))

const messages: Record<string, string> = {
  'common.actions': 'Actions',
  'common.edit': 'Edit',
  'common.name': 'Name',
  'common.refresh': 'Refresh',
  'common.status': 'Status',
  'keys.apiKey': 'API Key',
  'keys.allGroups': 'All Groups',
  'keys.allStatus': 'All Status',
  'keys.columnSettings': 'Column Settings',
  'keys.createKey': 'Create API Key',
  'keys.deleteKey': 'Delete API Key',
  'common.delete': 'Delete API Key',
  'keys.created': 'Created',
  'keys.expiresAt': 'Expires',
  'keys.group': 'Group',
  'keys.id': 'ID',
  'keys.currentConcurrency': 'Current Concurrency',
  'keys.lastUsedAt': 'Last Used',
  'keys.lastUsedIP': 'Last Used IP',
  'keys.rateLimitColumn': 'Rate Limit',
  'keys.rotate': 'Rotate',
  'keys.rotateKey': 'Rotate API Key',
  'keys.rotateConfirmMessage': 'Rotate this key?',
  'keys.keyRotatedSuccess': 'Rotated',
  'keys.failedToRotate': 'Failed to rotate',
  'keys.useKey': 'Use Key',
  'keys.importToCcSwitch': 'Import to CCS',
  'keys.enable': 'Enable',
  'keys.disable': 'Disable',
  'keys.defaultKeyProvisioningHint': 'Default keys are issued automatically.',
  'keys.searchPlaceholder': 'Search name or key...',
  'keys.status.active': 'Active',
  'keys.status.expired': 'Expired',
  'keys.status.inactive': 'Inactive',
  'keys.status.quota_exhausted': 'Quota exhausted',
  'keys.usage': 'Usage',
}

vi.mock('@/api', () => ({
  keysAPI: {
    list: listKeys,
    create: vi.fn(),
    update: updateKey,
    bulkUpdate: vi.fn(),
    delete: vi.fn(),
    toggleStatus: vi.fn(),
    rotate: rotateKey,
  },
  authAPI: {
    getPublicSettings,
  },
  usageAPI: {
    getDashboardApiKeysUsage,
  },
  userGroupsAPI: {
    getAvailable: getAvailableGroups,
    getUserGroupRates,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    isCurrentStep,
    nextStep,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const createApiKey = (): ApiKey => ({
  id: 1,
  user_id: 1,
  key: 'sk-test-key',
  name: 'test-key',
  group_id: null,
  status: 'active',
  ip_whitelist: [],
  ip_blacklist: [],
  last_used_at: null,
  last_used_ip: null,
  quota: 0,
  quota_used: 0,
  expires_at: null,
  created_at: '2026-06-27T00:00:00Z',
  updated_at: '2026-06-27T00:00:00Z',
  current_concurrency: 3,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
  window_5h_start: null,
  window_1d_start: null,
  window_7d_start: null,
  reset_5h_at: null,
  reset_1d_at: null,
  reset_7d_at: null,
})

const AppLayoutStub = {
  template: '<div><slot /></div>',
}

const TablePageLayoutStub = {
  template: `
    <div>
      <slot name="filters" />
      <slot name="actions" />
      <slot name="table" />
      <slot name="pagination" />
    </div>
  `,
}

const DataTableStub = {
  name: 'DataTable',
  props: { columns: Array, data: Array, selectedKeys: Array, selectable: Boolean },
  emits: ['sort', 'update:selectedKeys'],
  template: `
    <div>
      <div data-test="columns">{{ columns.map((col) => col.key).join(',') }}</div>
      <div data-test="columns-meta">{{ JSON.stringify(columns.map((col) => ({ key: col.key, sortable: !!col.sortable }))) }}</div>
      <button data-test="sort-current-concurrency" @click="$emit('sort', 'current_concurrency', 'asc')">
        Sort Current Concurrency
      </button>
      <div v-for="row in data" :key="row.id">
        <div
          v-if="columns.some((col) => col.key === 'id')"
          data-test="key-id"
        >
          <slot name="cell-id" :value="row.id" :row="row" />
        </div>
        <slot name="cell-name" :value="row.name" :row="row" />
        <div data-test="current-concurrency">
          <slot name="cell-current_concurrency" :value="row.current_concurrency" :row="row" />
        </div>
        <div data-test="group-cell">
          <slot name="cell-group" :value="row.group" :row="row" />
        </div>
        <div data-test="actions-cell">
          <slot name="cell-actions" :row="row" />
        </div>
        <div
          v-if="columns.some((col) => col.key === 'last_used_ip')"
          data-test="last-used-ip"
        >
          <slot name="cell-last_used_ip" :value="row.last_used_ip" :row="row" />
        </div>
      </div>
      <slot name="empty" />
    </div>
  `,
}

const ConfirmDialogStub = {
  name: 'ConfirmDialog',
  props: ['show', 'title', 'message', 'confirmText'],
  emits: ['confirm', 'cancel'],
  template: `
    <div v-if="show" data-test="confirm-dialog">
      <div data-test="confirm-title">{{ title }}</div>
      <div data-test="confirm-message">{{ message }}</div>
      <button data-test="confirm-action" @click="$emit('confirm')">{{ confirmText }}</button>
    </div>
  `,
}

const SelectStub = {
  name: 'Select',
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"></select>',
}

const SearchInputStub = {
  name: 'SearchInput',
  props: ['modelValue'],
  emits: ['update:modelValue', 'search'],
  template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
}

const PaginationStub = {
  name: 'Pagination',
  props: ['page', 'total', 'pageSize'],
  emits: ['update:page', 'update:pageSize'],
  template: `
    <div>
      <button data-test="page-size-50" @click="$emit('update:pageSize', 50)">50</button>
      <button data-test="page-2" @click="$emit('update:page', 2)">Page 2</button>
    </div>
  `,
}

const IconStub = {
  props: ['name'],
  template: '<span data-test="icon">{{ name }}</span>',
}

const mountView = async ({ realBulkEdit = false, realUseKey = false } = {}) => {
  const wrapper = mount(KeysView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: PaginationStub,
        BaseDialog: {
          props: ['show', 'title'],
          emits: ['close'],
          template: '<div v-if="show" role="dialog"><button data-test="close-dialog" @click="$emit(\'close\')">Close</button><slot /><slot name="footer" /></div>',
        },
        ConfirmDialog: ConfirmDialogStub,
        EmptyState: true,
        Select: SelectStub,
        SearchInput: SearchInputStub,
        Icon: IconStub,
        UseKeyModal: !realUseKey,
        BulkEditKeysModal: !realBulkEdit,
        EndpointPopover: true,
        GroupBadge: true,
        GroupOptionItem: true,
        Teleport: true,
      },
    },
  })
  await flushPromises()
  await nextTick()
  return wrapper
}

const visibleColumnKeys = (wrapper: VueWrapper) =>
  wrapper.get('[data-test="columns"]').text().split(',').filter(Boolean)

const visibleColumnMeta = (wrapper: VueWrapper): Array<{ key: string; sortable: boolean }> =>
  JSON.parse(wrapper.get('[data-test="columns-meta"]').text())

const getButtonByText = (wrapper: VueWrapper, text: string) => {
  const button = wrapper.findAll('button').find((item) => item.text().includes(text))
  if (!button) {
    throw new Error(`Button not found: ${text}`)
  }
  return button
}

describe('user KeysView column settings', () => {
  beforeEach(() => {
    localStorage.clear()

    listKeys.mockReset()
    updateKey.mockReset()
    vi.mocked(keysAPI.create).mockReset()
    vi.mocked(keysAPI.delete).mockReset()
    vi.mocked(keysAPI.bulkUpdate).mockReset()
    vi.mocked(keysAPI.bulkUpdate).mockResolvedValue({ succeededIds: [1], failures: [] })
    getPublicSettings.mockReset()
    getDashboardApiKeysUsage.mockReset()
    getAvailableGroups.mockReset()
    getUserGroupRates.mockReset()
    rotateKey.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    copyToClipboard.mockReset()
    isCurrentStep.mockReset()
    nextStep.mockReset()

    listKeys.mockResolvedValue({
      items: [createApiKey()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getPublicSettings.mockResolvedValue({})
    getDashboardApiKeysUsage.mockResolvedValue({ stats: {} })
    getAvailableGroups.mockResolvedValue([])
    getUserGroupRates.mockResolvedValue({})
    isCurrentStep.mockReturnValue(false)
  })

  it.each([
    { initialStatus: 'quota_exhausted', status: 'active', formStatus: 'active' },
    { initialStatus: 'inactive', status: 'inactive', formStatus: 'inactive' },
    { initialStatus: 'active', status: 'active', formStatus: 'inactive' },
  ] as const)('syncs quota reset from $initialStatus to $status with form status $formStatus', async ({ initialStatus, status, formStatus }) => {
    const key: ApiKey = {
      ...createApiKey(), group_id: 1, quota: 10, quota_used: 10,
      status: initialStatus,
    }
    listKeys.mockResolvedValueOnce({ items: [key], total: 1, page: 1, page_size: 20, pages: 1 })
    updateKey.mockResolvedValue({ ...key, status, quota_used: 0 })
    const wrapper = await mountView()
    await getButtonByText(wrapper, 'Edit').trigger('click')
    await wrapper.get('[data-tour="key-form-name"]').setValue('Unsaved name')
    const statusSelect = wrapper.findAllComponents({ name: 'Select' })
      .find((select) => select.props('options').length === 2 &&
        select.props('options')[0].value === 'active')!
    statusSelect.vm.$emit('update:modelValue', 'inactive')
    await wrapper.get('button[title="keys.resetQuotaUsed"]').trigger('click')
    const confirmation = wrapper.findAllComponents({ name: 'ConfirmDialog' })
      .find((dialog) => dialog.props('title') === 'keys.resetQuotaTitle')!
    confirmation.vm.$emit('confirm')
    await flushPromises()

    expect(updateKey).toHaveBeenNthCalledWith(1, key.id, { reset_quota: true })
    expect(wrapper.findComponent({ name: 'DataTable' }).props('data')[0])
      .toMatchObject({ status, quota_used: 0 })
    expect(statusSelect.props('modelValue')).toBe(formStatus)
    expect((wrapper.get('[data-tour="key-form-name"]').element as HTMLInputElement).value)
      .toBe('Unsaved name')

    await wrapper.get('#key-form').trigger('submit')
    await flushPromises()
    expect(updateKey).toHaveBeenNthCalledWith(2, key.id, expect.objectContaining({ name: 'Unsaved name', status: formStatus }))
    wrapper.unmount()
  })

  it('uses the default API key columns with low-frequency columns hidden', async () => {
    const wrapper = await mountView()

    expect(visibleColumnKeys(wrapper)).toEqual([
      'name',
      'key',
      'group',
      'current_concurrency',
      'usage',
      'expires_at',
      'status',
      'created_at',
      'actions',
    ])
    expect(visibleColumnKeys(wrapper)).not.toContain('rate_limit')
    expect(visibleColumnKeys(wrapper)).not.toContain('last_used_at')
    expect(visibleColumnKeys(wrapper)).not.toContain('last_used_ip')
    expect(visibleColumnKeys(wrapper)).not.toContain('id')
  })

  it('opens bulk editing with only selected visible keys', async () => {
    const wrapper = await mountView()
    const table = wrapper.findComponent({ name: 'DataTable' })
    expect(table.props('selectable')).toBe(true)
    table.vm.$emit('update:selectedKeys', [1, 99])
    await nextTick()
    await wrapper.get('[data-test="bulk-edit-keys"]').trigger('click')
    const modal = wrapper.findComponent({ name: 'BulkEditKeysModal' })
    expect(modal.props('show')).toBe(true)
    expect(modal.props('selectedKeys').map((key: ApiKey) => key.id)).toEqual([1])
    wrapper.unmount()
  })

  it.each(['filter', 'page size', 'sort'])('clears selection on %s changes', async (change) => {
    const wrapper = await mountView()
    const table = wrapper.findComponent({ name: 'DataTable' })
    table.vm.$emit('update:selectedKeys', [1])
    await nextTick()
    if (change === 'filter') {
      wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('search')
    } else if (change === 'page size') {
      await wrapper.get('[data-test="page-size-50"]').trigger('click')
    } else {
      table.vm.$emit('sort', 'created_at', 'asc')
    }
    await flushPromises()
    expect(table.props('selectedKeys')).toEqual([])
    expect(wrapper.find('[data-test="bulk-edit-keys"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('removes successful keys from the selection and refreshes the table', async () => {
    listKeys.mockResolvedValue({
      items: [createApiKey(), { ...createApiKey(), id: 2, name: 'Second' }],
      total: 2, pages: 1
    })
    const wrapper = await mountView()
    const table = wrapper.findComponent({ name: 'DataTable' })
    table.vm.$emit('update:selectedKeys', [1, 2])
    await nextTick()
    await wrapper.get('[data-test="bulk-edit-keys"]').trigger('click')
    wrapper.findComponent({ name: 'BulkEditKeysModal' }).vm.$emit('updated', [1])
    await flushPromises()
    expect(listKeys).toHaveBeenCalledTimes(2)
    expect(table.props('selectedKeys')).toEqual([2])
    wrapper.unmount()
  })

  it('drops keys that are no longer visible after a refresh', async () => {
    const wrapper = await mountView()
    const table = wrapper.findComponent({ name: 'DataTable' })
    table.vm.$emit('update:selectedKeys', [1])
    await nextTick()
    listKeys.mockResolvedValue({ items: [], total: 0, pages: 0 })
    await wrapper.get('button[title="Refresh"]').trigger('click')
    await flushPromises()
    expect(table.props('selectedKeys')).toEqual([])
    wrapper.unmount()
  })

  it('shows a hidden column when toggled and persists the preference', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'Rate Limit').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('rate_limit')
    expect(localStorage.getItem('api-key-hidden-columns')).toBe(
      JSON.stringify(['id', 'last_used_at', 'last_used_ip'])
    )
    expect(localStorage.getItem('api-key-column-settings-version')).toBe('3')
  })

  it('shows the API key ID column when toggled', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'ID').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('id')
    expect(wrapper.get('[data-test="key-id"]').text()).toBe('#1')
    expect(visibleColumnMeta(wrapper).find((column) => column.key === 'id')?.sortable).toBe(true)
  })

  it('shows the last used IP column when toggled', async () => {
    listKeys.mockResolvedValueOnce({
      items: [{ ...createApiKey(), last_used_ip: '203.0.113.10' }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'Last Used IP').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('last_used_ip')
    expect(wrapper.get('[data-test="last-used-ip"]').text()).toBe('203.0.113.10')
  })

  it('restores column preferences from localStorage on mount', async () => {
    localStorage.setItem('api-key-hidden-columns', JSON.stringify(['group', 'created_at']))
    localStorage.setItem('api-key-column-settings-version', '1')

    const wrapper = await mountView()

    expect(visibleColumnKeys(wrapper)).toEqual([
      'name',
      'key',
      'current_concurrency',
      'usage',
      'rate_limit',
      'expires_at',
      'status',
      'last_used_at',
      'actions',
    ])
    expect(localStorage.getItem('api-key-hidden-columns')).toBe(
      JSON.stringify(['group', 'created_at', 'last_used_ip', 'id'])
    )
    expect(localStorage.getItem('api-key-column-settings-version')).toBe('3')
  })

  it('does not include always-visible columns in the toggleable menu', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await nextTick()

    const columnMenuText = wrapper.text()
    expect(columnMenuText).toContain('API Key')
    expect(columnMenuText).toContain('ID')
    expect(columnMenuText).toContain('Current Concurrency')
    expect(columnMenuText).toContain('Rate Limit')
    expect(columnMenuText).toContain('Last Used IP')
    expect(columnMenuText).not.toContain('Name')
    expect(columnMenuText).not.toContain('Actions')
  })

  it('renders the current concurrency value', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-test="current-concurrency"]').text()).toBe('3')
  })

  it('marks current concurrency as sortable', async () => {
    const wrapper = await mountView()

    const currentConcurrencyColumn = visibleColumnMeta(wrapper).find(
      (column) => column.key === 'current_concurrency'
    )
    expect(currentConcurrencyColumn?.sortable).toBe(true)
  })

  it('shows managed key actions without create, delete or rebind entry points', async () => {
    const wrapper = await mountView()

    expect(wrapper.find('[data-tour="keys-create-btn"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Create API Key')
    expect(wrapper.text()).not.toContain('Delete API Key')
    expect(wrapper.get('[data-test="group-cell"]').find('button').exists()).toBe(false)
    expect(wrapper.text()).toContain('Rotate')
    expect(keysAPI.create).not.toHaveBeenCalled()
    expect(keysAPI.delete).not.toHaveBeenCalled()
  })

  it('rotates a managed key only after confirmation and refreshes the credential', async () => {
    const rotated = { ...createApiKey(), key: 'sk-rotated' }
    rotateKey.mockResolvedValueOnce(rotated)
    const wrapper = await mountView()
    listKeys.mockResolvedValueOnce({ items: [rotated], total: 1, pages: 1 })

    await getButtonByText(wrapper, 'Rotate').trigger('click')
    expect(rotateKey).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="confirm-title"]').text()).toBe('Rotate API Key')

    await wrapper.get('[data-test="confirm-action"]').trigger('click')
    await flushPromises()

    expect(rotateKey).toHaveBeenCalledOnce()
    expect(rotateKey).toHaveBeenCalledWith(1)
    expect(showSuccess).toHaveBeenCalledWith('Rotated')
    expect(wrapper.find('[data-test="confirm-dialog"]').exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'DataTable' }).props('data')[0].key).toBe('sk-rotated')
  })

  it('keeps the old credential and confirmation available when rotation fails', async () => {
    rotateKey.mockRejectedValueOnce(new Error('rotation failed'))
    const wrapper = await mountView()
    await getButtonByText(wrapper, 'Rotate').trigger('click')
    await wrapper.get('[data-test="confirm-action"]').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('Failed to rotate')
    expect(showSuccess).not.toHaveBeenCalled()
    expect(listKeys).toHaveBeenCalledOnce()
    expect(wrapper.find('[data-test="confirm-dialog"]').exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'DataTable' }).props('data')[0].key).toBe('sk-test-key')
  })

  it.each([
    { key: 'current_concurrency', order: 'asc' },
    { key: 'group', order: 'asc' },
    { key: 'group', order: 'desc' },
  ] as const)('keeps filters and resets pagination and selection when sorting $key $order', async ({ key, order }) => {
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'OpenAI' }])
    const wrapper = await mountView()

    await wrapper.get('[data-test="page-size-50"]').trigger('click')
    await flushPromises()

    await wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('update:modelValue', 'target')
    await wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('search')
    await flushPromises()

    const selects = wrapper.findAllComponents({ name: 'Select' })
    await selects[0].vm.$emit('update:modelValue', 42)
    await flushPromises()
    await selects[1].vm.$emit('update:modelValue', 'active')
    await flushPromises()

    await wrapper.get('[data-test="page-2"]').trigger('click')
    await flushPromises()
    const table = wrapper.findComponent({ name: 'DataTable' })
    table.vm.$emit('update:selectedKeys', [1])
    await nextTick()
    expect(table.props('selectedKeys')).toEqual([1])
    expect(visibleColumnMeta(wrapper).find((column) => column.key === key)?.sortable).toBe(true)
    listKeys.mockClear()

    table.vm.$emit('sort', key, order)
    await flushPromises()

    expect(table.props('selectedKeys')).toEqual([])
    expect(listKeys).toHaveBeenLastCalledWith(
      1,
      50,
      {
        search: 'target',
        status: 'active',
        group_id: 42,
        sort_by: key,
        sort_order: order,
      },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
  })

  describe('managed provider binding', () => {
    const platforms = ['anthropic', 'openai', 'kimi', 'zhipu', 'deepseek', 'minimax', 'gemini', 'grok', 'antigravity', 'composite', 'opencode_go', 'typesafe'] as const
    const availableGroups = platforms.map((platform, index) => ({
      id: index + 1,
      // Ambiguous names ensure the Use Key flow follows the assigned platform.
      name: `Shared group ${index + 1}`,
      platform,
      rate_multiplier: 1,
      subscription_type: 'standard',
    } as Group))

    beforeEach(() => {
      getAvailableGroups.mockResolvedValue(availableGroups)
    })

    it('retains every configured platform in the table filter without offering provider creation', async () => {
      const wrapper = await mountView()
      const options = wrapper.findAllComponents({ name: 'Select' })[0].props('options')
      expect(options.map((option: { value: string | number }) => option.value)).toEqual([
        '', 0, ...availableGroups.map((group) => group.id),
      ])
      expect(wrapper.find('[data-tour="keys-create-btn"]').exists()).toBe(false)
      expect(wrapper.find('input[name="key-provider"]').exists()).toBe(false)
    })

    it.each(platforms)('keeps the %s binding read-only and passes its platform to Use Key', async (platform) => {
      const group = availableGroups.find((item) => item.platform === platform)!
      const key = { ...createApiKey(), group_id: group.id, group }
      listKeys.mockResolvedValue({ items: [key], total: 1, pages: 1 })
      const wrapper = await mountView()
      await getButtonByText(wrapper, 'Use Key').trigger('click')
      expect(wrapper.findComponent({ name: 'UseKeyModal' }).props()).toMatchObject({
        show: true, apiKey: key.key, platform,
      })
      wrapper.findComponent({ name: 'UseKeyModal' }).vm.$emit('close')
      await nextTick()

      await getButtonByText(wrapper, 'Edit').trigger('click')
      expect(wrapper.find('[data-tour="key-form-group"]').exists()).toBe(false)
      expect(wrapper.find('[data-tour="key-form-provider"]').exists()).toBe(false)
      expect(wrapper.findAllComponents({ name: 'GroupBadge' })[1].props()).toMatchObject({
        name: group.name, platform,
      })
      await wrapper.get('[data-tour="key-form-name"]').setValue('Renamed managed key')
      await wrapper.get('#key-form').trigger('submit')
      await flushPromises()
      expect(updateKey).toHaveBeenCalledOnce()
      expect(updateKey).toHaveBeenCalledWith(key.id, expect.objectContaining({ name: 'Renamed managed key' }))
      expect(updateKey.mock.calls[0][1]).not.toHaveProperty('group_id')
      expect(keysAPI.create).not.toHaveBeenCalled()
      expect(keysAPI.delete).not.toHaveBeenCalled()
    })

    it('shows SystemOne instructions for a managed typesafe key', async () => {
      const group = availableGroups.find((item) => item.platform === 'typesafe')!
      listKeys.mockResolvedValue({ items: [{ ...createApiKey(), group_id: group.id, group }], total: 1, pages: 1 })
      getPublicSettings.mockResolvedValue({ api_base_url: 'https://gateway.example' })
      const wrapper = await mountView({ realUseKey: true })
      await getButtonByText(wrapper, 'Use Key').trigger('click')
      const modal = wrapper.findComponent({ name: 'UseKeyModal' })
      expect(modal.get('nav[aria-label="Client"]').text()).toContain('keys.useKeyModal.cliTabs.systemOne')
      expect(modal.get('pre code').text()).toContain('https://gateway.example/v1/systemone')
      expect(modal.get('pre code').text()).toContain('Bearer sk-test-key')
    })

    it('updates late-arriving group filters without changing the managed binding', async () => {
      let resolveGroups!: (value: Group[]) => void
      getAvailableGroups.mockReturnValue(new Promise((resolve) => { resolveGroups = resolve }))
      const group = availableGroups[11]
      listKeys.mockResolvedValue({ items: [{ ...createApiKey(), group_id: group.id, group }], total: 1, pages: 1 })
      const wrapper = await mountView()
      await getButtonByText(wrapper, 'Edit').trigger('click')
      const filter = wrapper.findAllComponents({ name: 'Select' })[0]
      expect(filter.props('options')).toHaveLength(2)
      resolveGroups(availableGroups)
      await flushPromises()
      expect(filter.props('options')).toHaveLength(14)
      expect(wrapper.findAllComponents({ name: 'GroupBadge' })[1].props('platform')).toBe('typesafe')
      expect(wrapper.find('[data-tour="key-form-group"]').exists()).toBe(false)
    })

    it('allows editing an existing managed key without any available groups', async () => {
      getAvailableGroups.mockResolvedValue([])
      const wrapper = await mountView()
      await getButtonByText(wrapper, 'Edit').trigger('click')
      await wrapper.get('#key-form').trigger('submit')
      await flushPromises()
      expect(updateKey).toHaveBeenCalledOnce()
      expect(updateKey.mock.calls[0][1]).not.toHaveProperty('group_id')
      expect(showError).not.toHaveBeenCalled()
    })
  })

  describe('managed bulk limits', () => {
    const openBulkEdit = async () => {
      const wrapper = await mountView({ realBulkEdit: true })
      wrapper.findComponent({ name: 'DataTable' }).vm.$emit('update:selectedKeys', [1])
      await nextTick()
      await wrapper.get('[data-test="bulk-edit-keys"]').trigger('click')
      return wrapper
    }

    it.each(['quota', 'rate_limit_5h', 'rate_limit_1d', 'rate_limit_7d'])('validates %s before updating a managed key', async (field) => {
      const wrapper = await openBulkEdit()
      await wrapper.get(`[data-test="enable-${field}"]`).setValue(true)
      for (const value of ['', '-1', 'not-a-number']) {
        await wrapper.get(`[data-test="${field}-input"]`).setValue(value)
        await wrapper.get('#bulk-edit-keys-form').trigger('submit')
        expect(keysAPI.bulkUpdate).not.toHaveBeenCalled()
        expect(wrapper.get('[data-test="submit"]').attributes('disabled')).toBeDefined()
        expect(wrapper.get('[role="alert"]').text()).toContain('keys.bulkEdit.invalidLimit')
      }
      await wrapper.get(`[data-test="${field}-input"]`).setValue('0')
      await wrapper.get('#bulk-edit-keys-form').trigger('submit')
      await flushPromises()
      expect(keysAPI.bulkUpdate).toHaveBeenCalledOnce()
      expect(keysAPI.bulkUpdate).toHaveBeenCalledWith([1], { [field]: 0 })
      expect(keysAPI.create).not.toHaveBeenCalled()
      expect(keysAPI.delete).not.toHaveBeenCalled()
    })

    it('prevents duplicate bulk updates and closing while a limit update is in flight', async () => {
      let finish!: (result: { succeededIds: number[]; failures: [] }) => void
      vi.mocked(keysAPI.bulkUpdate).mockReturnValueOnce(new Promise((resolve) => { finish = resolve }))
      const wrapper = await openBulkEdit()
      await wrapper.get('[data-test="enable-rate_limit_1d"]').setValue(true)
      await wrapper.get('[data-test="rate_limit_1d-input"]').setValue('25.5')
      await wrapper.get('#bulk-edit-keys-form').trigger('submit')
      await wrapper.get('#bulk-edit-keys-form').trigger('submit')
      await wrapper.get('[data-test="close-dialog"]').trigger('click')
      const modal = wrapper.findComponent({ name: 'BulkEditKeysModal' })
      expect(modal.props('show')).toBe(true)
      expect(wrapper.get('fieldset').attributes('disabled')).toBeDefined()
      expect(wrapper.get('[data-test="submit"]').attributes('disabled')).toBeDefined()
      expect(keysAPI.bulkUpdate).toHaveBeenCalledOnce()
      expect(keysAPI.bulkUpdate).toHaveBeenCalledWith([1], { rate_limit_1d: 25.5 })
      finish({ succeededIds: [1], failures: [] })
      await flushPromises()
      expect(modal.props('show')).toBe(false)
      expect(wrapper.findComponent({ name: 'DataTable' }).props('selectedKeys')).toEqual([])
      expect(listKeys).toHaveBeenCalledTimes(2)
    })
  })
})
