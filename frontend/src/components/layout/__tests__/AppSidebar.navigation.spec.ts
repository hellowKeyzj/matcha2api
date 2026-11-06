import { defineComponent, h, reactive } from 'vue'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const list = vi.hoisted(() => vi.fn())
const appStore = reactive({
  siteName: 'Matcha', siteLogo: '', siteVersion: '', publicSettingsLoaded: true,
  sidebarCollapsed: false, mobileOpen: false, sidebarScrollTop: 0,
  backendModeEnabled: false, cachedPublicSettings: {},
})
const authStore = { isAuthenticated: true, isAdmin: false, isSimpleMode: false }

vi.mock('@/api/keys', () => ({ keysAPI: { list } }))
vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
  useAdminSettingsStore: () => ({ customMenuItems: [], fetch: vi.fn() }),
  useOnboardingStore: () => ({ isCurrentStep: () => false }),
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

enableAutoUnmount(afterEach)

beforeEach(() => {
  vi.resetModules()
  list.mockReset()
})

async function renderSidebar() {
  const { default: AppSidebar } = await import('../AppSidebar.vue')
  const page = defineComponent({ render: () => h('div', 'page content') })
  const shell = defineComponent({ render: () => [h(AppSidebar), h(RouterView)] })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{
      path: '/', component: shell,
      children: ['/profile', '/keys', '/usage', '/batch-image', '/:pathMatch(.*)*'].map(path => ({ path, component: page })),
    }],
  })
  await router.push('/profile')
  const wrapper = mount(RouterView, {
    global: { plugins: [router], stubs: { VersionBadge: true, transition: true } },
  })
  await flushPromises()
  return { wrapper, router, AppSidebar }
}

describe('persistent sidebar access lookup', () => {
  it('retries failed access on committed navigation without remounting or bypassing the cache', async () => {
    list.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({
      items: [{ status: 'active', group: { platform: 'gemini', allow_batch_image_generation: true } }],
      pages: 1,
    })
    const { wrapper, router, AppSidebar } = await renderSidebar()
    const sidebar = wrapper.getComponent(AppSidebar).vm.$.uid
    expect(list).toHaveBeenCalledTimes(1)
    expect(wrapper.find('a[href="/batch-image"]').exists()).toBe(false)

    const removeGuard = router.beforeEach(() => false)
    await router.push('/keys')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/profile')
    expect(list).toHaveBeenCalledTimes(1)
    removeGuard()

    await router.push('/keys')
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(2)
    expect(wrapper.find('a[href="/batch-image"]').exists()).toBe(true)
    expect(wrapper.getComponent(AppSidebar).vm.$.uid).toBe(sidebar)

    for (const path of ['/usage', '/profile']) {
      await router.push(path)
      await flushPromises()
      expect(list).toHaveBeenCalledTimes(2)
      expect(wrapper.getComponent(AppSidebar).vm.$.uid).toBe(sidebar)
    }
  })

  it('keeps a successful access denial cached across navigation', async () => {
    list.mockResolvedValue({ items: [], pages: 1 })
    const { wrapper, router } = await renderSidebar()
    await router.push('/keys')
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(1)
    expect(wrapper.find('a[href="/batch-image"]').exists()).toBe(false)
  })

  it('shares an in-flight lookup across consecutive navigation', async () => {
    const response = {
      items: [{ status: 'active', group: { platform: 'gemini', allow_batch_image_generation: true } }],
      pages: 1,
    }
    let finishLookup!: (value: typeof response) => void
    list.mockReturnValueOnce(new Promise<typeof response>(resolve => { finishLookup = resolve }))
    const { wrapper, router, AppSidebar } = await renderSidebar()
    const sidebar = wrapper.getComponent(AppSidebar).vm.$.uid

    for (const path of ['/keys', '/usage']) {
      await router.push(path)
      await flushPromises()
      expect(list).toHaveBeenCalledTimes(1)
      expect(wrapper.find('a[href="/batch-image"]').exists()).toBe(false)
      expect(wrapper.getComponent(AppSidebar).vm.$.uid).toBe(sidebar)
    }

    finishLookup(response)
    await flushPromises()
    expect(wrapper.find('a[href="/batch-image"]').exists()).toBe(true)
    await router.push('/profile')
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(1)
  })
})
