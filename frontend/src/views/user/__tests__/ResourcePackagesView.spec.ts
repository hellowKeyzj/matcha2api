import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import packagesAPI, { publishPackageVersion, type ResourcePackageVersion } from '@/api/packages'
import DataTable from '@/components/common/DataTable.vue'
import enDashboard from '@/i18n/locales/en/dashboard'
import enCommon from '@/i18n/locales/en/common'
import zhDashboard from '@/i18n/locales/zh/dashboard'
import zhCommon from '@/i18n/locales/zh/common'
import ResourcePackagesView from '../ResourcePackagesView.vue'

const { get, post, showError, showSuccess } = vi.hoisted(() => ({
  get: vi.fn(), post: vi.fn(), showError: vi.fn(), showSuccess: vi.fn(),
}))
vi.mock('@/api/client', () => ({ apiClient: { get, post } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))

enableAutoUnmount(afterEach)
const messages = {
  en: { ...enDashboard, ...enCommon },
  zh: { ...zhDashboard, ...zhCommon },
}
const draft: ResourcePackageVersion = {
  packageId: '1', packageVersionId: '11', name: 'draft-skill', packageType: 'skill',
  version: '1.0', status: 'draft', downloadable: false,
}
const otherDraft = { ...draft, packageVersionId: '12', name: 'other-draft' }
const rows = [draft, otherDraft, ...['published', 'disabled', 'yanked'].map((status, index) => ({
  ...draft, packageVersionId: String(13 + index), name: `${status}-skill`, status,
}))]
function page(items: ResourcePackageVersion[]) {
  return { data: { items, total: items.length, page: 1, page_size: 20, pages: 1 } }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}
function mountPage(locale: 'en' | 'zh' = 'en') {
  return mount(ResourcePackagesView, { global: {
    plugins: [createI18n({ legacy: false, locale, messages,
      // Vitest aliases the runtime-only build; these assertions use plain-text messages.
      messageCompiler: (message) => () => String(message),
    })],
    stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true },
  } })
}
type Wrapper = ReturnType<typeof mountPage>
function button(wrapper: Wrapper, label: string) {
  return wrapper.findAll('button').find((item) => item.text() === label)!
}
function row(wrapper: Wrapper, name: string) {
  return wrapper.findAll('tbody tr').find((item) => item.text().includes(name))!
}
async function mine(wrapper: Wrapper, locale: 'en' | 'zh' = 'en') {
  await flushPromises()
  await button(wrapper, messages[locale].resourcePackages.tabs.mine).trigger('click')
  await flushPromises()
}

describe('resource package publishing', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    get.mockResolvedValue(page(rows))
    post.mockResolvedValue({ data: { ...draft, status: 'published', downloadable: true } })
  })

  it('uses the existing POST endpoint and returns the ResourcePackageVersion DTO unchanged', async () => {
    const published = { ...draft, status: 'published', downloadable: false }
    post.mockResolvedValueOnce({ data: published })
    expect(packagesAPI.publishPackageVersion).toBe(publishPackageVersion)
    expect(await publishPackageVersion('11')).toBe(published)
    expect(post).toHaveBeenCalledTimes(1)
    expect(post).toHaveBeenCalledWith('/packages/11/publish')
  })

  it('shows publish only for mine drafts, never for market or non-draft rows', async () => {
    const wrapper = mountPage()
    await flushPromises()
    expect(button(wrapper, 'Publish')).toBeUndefined()
    await mine(wrapper)
    expect(wrapper.findAll('button').filter((item) => item.text() === 'Publish')).toHaveLength(2)
    for (const status of ['published', 'disabled', 'yanked']) {
      expect(row(wrapper, `${status}-skill`).findAll('button').map((item) => item.text())).toEqual(['Download'])
    }
    expect(post).not.toHaveBeenCalled()
  })

  it.each([true, false])('serializes publishing through refresh and preserves server downloadable=%s', async (downloadable) => {
    const wrapper = mountPage()
    await mine(wrapper)
    const published = { ...draft, status: 'published', downloadable }
    const publishing = deferred<{ data: ResourcePackageVersion }>()
    const refreshing = deferred<ReturnType<typeof page>>()
    post.mockReturnValueOnce(publishing.promise)
    get.mockReturnValueOnce(refreshing.promise)
    const publishButton = row(wrapper, draft.name).find('button')
    // Two events in the same tick also exercise the handler guard before DOM disabling.
    publishButton.element.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    publishButton.element.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()
    expect(post).toHaveBeenCalledWith('/packages/11/publish')
    expect(publishButton.attributes('disabled')).toBeDefined()
    const otherButton = row(wrapper, otherDraft.name).find('button')
    expect(otherButton.attributes('disabled')).toBeDefined()
    otherButton.element.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()
    expect(post).toHaveBeenCalledTimes(1)

    publishing.resolve({ data: published })
    await flushPromises()
    expect(showSuccess).toHaveBeenCalledWith(messages.en.resourcePackages.messages.published)
    expect(get).toHaveBeenLastCalledWith('/packages/mine', expect.objectContaining({
      params: expect.objectContaining({ page: 1 }), signal: expect.any(AbortSignal),
    }))
    // The publish response updates status and entitlement-derived downloadable even before refresh finishes.
    const table = wrapper.findComponent(DataTable)
    expect(table.props('data')[0]).toEqual(published)
    expect(table.props('loading')).toBe(true)
    otherButton.element.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()
    expect(post).toHaveBeenCalledTimes(1)

    refreshing.resolve(page([published, { ...otherDraft, description: 'refreshed list' }]))
    await flushPromises()
    expect(row(wrapper, draft.name).text()).toContain('Published')
    const downloadButton = row(wrapper, draft.name).find('button')
    expect(downloadButton.text()).toBe('Download')
    expect(downloadButton.attributes('disabled') !== undefined).toBe(!downloadable)
    expect(row(wrapper, otherDraft.name).text()).toContain('refreshed list')
    expect(row(wrapper, otherDraft.name).find('button').attributes('disabled')).toBeUndefined()
    expect(showError).not.toHaveBeenCalled()
  })

  const errorCodes = [
    'MATCHA_PACKAGE_CLOUD_ENVELOPE_REQUIRED',
    'MATCHA_PACKAGE_CLOUD_KEY_NOT_CONFIGURED',
    'MATCHA_PACKAGE_CLOUD_KEY_UNAVAILABLE',
    'MATCHA_PACKAGE_CLOUD_KEY_ID_CONFLICT',
    'MATCHA_PACKAGE_VERSION_NOT_PUBLISHABLE',
  ] as const
  for (const locale of ['en', 'zh'] as const) {
    it.each(errorCodes)(`${locale}: localizes %s and allows retry without refreshing on failure`, async (reason) => {
      const wrapper = mountPage(locale)
      await mine(wrapper, locale)
      post.mockRejectedValueOnce({ code: 503, reason, message: 'raw backend message' })
      await button(wrapper, messages[locale].resourcePackages.actions.publish).trigger('click')
      await flushPromises()
      expect(showError).toHaveBeenCalledWith(messages[locale].resourcePackages.errors[reason])
      expect(showSuccess).not.toHaveBeenCalled()
      expect(get).toHaveBeenCalledTimes(2)
      expect(button(wrapper, messages[locale].resourcePackages.actions.publish).attributes('disabled')).toBeUndefined()
      expect(wrapper.findComponent(DataTable).props('data')[0]).toEqual(draft)
    })

    it(`${locale}: uses a localized fallback for unknown errors`, async () => {
      const wrapper = mountPage(locale)
      await mine(wrapper, locale)
      post.mockRejectedValueOnce({ reason: 'UNKNOWN', message: 'raw backend message' })
      await button(wrapper, messages[locale].resourcePackages.actions.publish).trigger('click')
      await flushPromises()
      expect(showError).toHaveBeenCalledWith(messages[locale].resourcePackages.errors.failedToPublish)
    })
  }

  it('keeps a successful publish when list refresh fails and reports a load error, not a publish error', async () => {
    const wrapper = mountPage()
    await mine(wrapper)
    get.mockRejectedValueOnce(new Error('raw refresh failure'))
    await button(wrapper, 'Publish').trigger('click')
    await flushPromises()
    expect(showSuccess).toHaveBeenCalledWith(messages.en.resourcePackages.messages.published)
    expect(showError).toHaveBeenCalledWith(messages.en.resourcePackages.errors.failedToLoad)
    expect(row(wrapper, draft.name).text()).toContain('Published')
    expect(row(wrapper, draft.name).find('button').attributes('disabled')).toBeUndefined()
  })

  it('uploads as a draft and refreshes mine without automatically publishing', async () => {
    const wrapper = mountPage()
    await flushPromises()
    post.mockResolvedValueOnce({ data: draft })
    const file = new File(['sealed package'], 'skill.matcha-skillpkg')
    const input = wrapper.find('input[type="file"]')
    Object.defineProperty(input.element, 'files', { configurable: true, value: [file] })
    await input.trigger('change')
    await flushPromises()
    expect(post).toHaveBeenCalledTimes(1)
    expect(post).toHaveBeenCalledWith('/packages/upload', expect.any(FormData), expect.any(Object))
    expect((post.mock.calls[0][1] as FormData).get('package')).toBe(file)
    expect(get).toHaveBeenLastCalledWith('/packages/mine', expect.any(Object))
    expect(button(wrapper, 'Publish')).toBeDefined()
    expect(showSuccess).toHaveBeenCalledWith(messages.en.resourcePackages.messages.uploaded)
  })
})
