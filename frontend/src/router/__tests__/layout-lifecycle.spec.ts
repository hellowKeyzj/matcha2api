import { reactive, ref } from 'vue'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { readdirSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import AppHeader from '@/components/layout/AppHeader.vue'
import SiteLogo from '@/components/common/SiteLogo.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ProfileView from '@/views/user/ProfileView.vue'
import AvailableChannelsView from '@/views/user/AvailableChannelsView.vue'
import ModelPlazaView from '@/views/ModelPlazaView.vue'
import StripePaymentView from '@/views/user/StripePaymentView.vue'
import { initializeTheme } from '@/composables/useTheme'

const appStore = reactive({
  siteName: 'Matcha', siteLogo: '', siteVersion: '', publicSettingsLoaded: true,
  sidebarCollapsed: false, mobileOpen: false, sidebarScrollTop: 0,
  contactInfo: '', docUrl: '',
  backendModeEnabled: false, cachedPublicSettings: {},
  fetchPublicSettings: vi.fn().mockResolvedValue({}), showError: vi.fn(),
})
const authStore = reactive({
  isAuthenticated: true, isAdmin: false, isSimpleMode: false,
  user: { id: 1, role: 'user' }, checkAuth: vi.fn(), refreshUser: vi.fn().mockResolvedValue(undefined),
})
const adminSettingsStore = { customMenuItems: [], fetch: vi.fn() }
const onboardingStore = { isCurrentStep: () => false, setReplayCallback: vi.fn() }
const startOnboarding = vi.fn(() => ({ replayTour: vi.fn() }))
const listAffiliateRecords = vi.fn().mockResolvedValue({ items: [], total: 0 })

vi.mock('@/stores', () => ({
  useAppStore: () => appStore, useAuthStore: () => authStore,
  useAdminSettingsStore: () => adminSettingsStore, useOnboardingStore: () => onboardingStore,
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => adminSettingsStore }))
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => onboardingStore }))
vi.mock('@/composables/useOnboardingTour', () => ({
  useOnboardingTour: () => startOnboarding(),
}))
vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({ canUseBatchImage: ref(false), refreshBatchImageAccess: vi.fn() }),
}))
vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({ startNavigation: vi.fn(), endNavigation: vi.fn() }),
}))
vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({ triggerPrefetch: vi.fn() }),
}))
vi.mock('@/api/channels', () => ({ default: { getAvailable: vi.fn().mockResolvedValue([]) } }))
vi.mock('@/api/groups', () => ({ default: { getUserGroupRates: vi.fn().mockResolvedValue({}) } }))
vi.mock('@/api/modelPlaza', () => ({ getModelPlaza: vi.fn().mockResolvedValue({}) }))
vi.mock('@/api/admin/affiliates', () => {
  const api = {
    listInviteRecords: () => listAffiliateRecords(),
    listRebateRecords: () => listAffiliateRecords(),
    listTransferRecords: () => listAffiliateRecords(),
  }
  return { affiliatesAPI: api, default: api }
})
vi.mock('@/stores/payment', () => ({ usePaymentStore: () => ({}) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('@/i18n', () => ({
  getLocale: () => 'en', i18n: { global: { t: (key: string) => key } },
}))

enableAutoUnmount(afterEach)

beforeEach(() => {
  appStore.siteLogo = ''
  appStore.sidebarCollapsed = false
  authStore.isAuthenticated = true
  authStore.isAdmin = false
  authStore.user.role = 'user'
  startOnboarding.mockClear()
  onboardingStore.setReplayCallback.mockClear()
})

afterEach(() => {
  localStorage.removeItem('theme')
  initializeTheme()
})

const stubs = {
  transition: true,
  AnnouncementBell: true, SubscriptionProgressMini: true,
  LocaleSwitcher: true, VersionBadge: true,
  ProfileInfoCard: true, ProfilePasswordForm: true, ProfileTotpCard: true,
  ProfilePasskeyCard: true, AvailableChannelsTable: true,
  ModelPlazaContent: true, PlazaNavBar: true,
  DataTable: true, AffiliateOfflineWithdrawDialog: true,
}

async function renderRoute(path: string) {
  const { default: configuredRouter } = await import('@/router')
  const router = createRouter({ history: createMemoryHistory(), routes: configuredRouter.options.routes })
  await router.push(path)
  const wrapper = mount(RouterView, { global: { plugins: [router], stubs } })
  await flushPromises()
  return { wrapper, router }
}

describe('workspace route lifecycle', () => {
  it.each(['', '/uploads/custom-logo.png'])(
    'keeps the sidebar and logo mounted while replacing page content (logo: %s)',
    async (logo) => {
      appStore.siteLogo = logo
      const { wrapper, router } = await renderRoute('/profile')

      const sidebar = wrapper.getComponent(AppSidebar).vm.$.uid
      const logoComponent = wrapper.getComponent(SiteLogo).vm.$.uid
      const logoElement = wrapper.getComponent(SiteLogo).element
      const header = wrapper.get('header').element
      const headerInstance = wrapper.getComponent(AppHeader).vm.$.uid
      const particles = wrapper.get('.workspace-particles').element
      const firstProfile = wrapper.getComponent(ProfileView).vm.$.uid
      const nav = wrapper.get('.sidebar-nav').element
      nav.scrollTop = 120
      appStore.sidebarCollapsed = true
      localStorage.setItem('theme', 'light')
      initializeTheme()
      await flushPromises()

      for (let i = 0; i < 3; i++) {
        await router.push('/available-channels')
        await flushPromises()
        expect(wrapper.findComponent(ProfileView).exists()).toBe(false)
        expect(wrapper.findComponent(AvailableChannelsView).exists()).toBe(true)
        expect(wrapper.getComponent(AppSidebar).vm.$.uid).toBe(sidebar)
        expect(wrapper.getComponent(SiteLogo).vm.$.uid).toBe(logoComponent)
        expect(wrapper.getComponent(SiteLogo).element).toBe(logoElement)
        expect(wrapper.get('header').element).toBe(header)
        expect(wrapper.getComponent(AppHeader).vm.$.uid).toBe(headerInstance)
        expect(wrapper.get('.workspace-particles').element).toBe(particles)
        expect(wrapper.get('.sidebar-nav').element.scrollTop).toBe(120)
        expect(wrapper.get('.app-layout-content').classes()).toContain('lg:ml-[72px]')

        await wrapper.get('input').setValue('page-local search')
        await router.push('/profile')
        await flushPromises()
        expect(wrapper.getComponent(ProfileView).vm.$.uid).not.toBe(firstProfile)
        expect(wrapper.getComponent(AppSidebar).vm.$.uid).toBe(sidebar)
      }

      await router.push('/available-channels')
      await flushPromises()
      expect((wrapper.get('input').element as HTMLInputElement).value).toBe('')
      expect(wrapper.findAll('aside.sidebar')).toHaveLength(1)
      expect(startOnboarding).toHaveBeenCalledTimes(1)
      await wrapper.get('.sidebar > div:last-of-type button').trigger('click')
      expect(document.documentElement.classList.contains('dark')).toBe(true)
      expect(wrapper.getComponent(SiteLogo).element).toBe(logoElement)
    },
  )

  it('keeps the admin shell while switching affiliate record pages and the personal workspace', async () => {
    authStore.isAdmin = true
    authStore.user.role = 'admin'
    const { wrapper, router } = await renderRoute('/admin/affiliates/invites')
    const logo = wrapper.getComponent(SiteLogo).element
    const sidebar = wrapper.getComponent(AppSidebar).vm.$.uid
    for (const path of ['/admin/affiliates/rebates', '/admin/affiliates/transfers', '/profile', '/admin/affiliates/invites']) {
      await router.push(path)
      await flushPromises()
      expect(wrapper.getComponent(SiteLogo).element).toBe(logo)
      expect(wrapper.getComponent(AppSidebar).vm.$.uid).toBe(sidebar)
      expect(wrapper.get('.app-header-title h1').text()).toBe(router.currentRoute.value.meta.titleKey)
    }
    expect(listAffiliateRecords).toHaveBeenCalledTimes(4)
  })

  it('closes the user menu after history navigation without remounting the header', async () => {
    const { wrapper, router } = await renderRoute('/profile')
    await router.push('/available-channels')
    await flushPromises()
    const header = wrapper.getComponent(AppHeader).vm.$.uid
    await wrapper.get('header button[aria-label="common.userMenu"]').trigger('click')
    expect(wrapper.find('header .dropdown').exists()).toBe(true)

    await router.back()
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/profile')
    expect(wrapper.getComponent(AppHeader).vm.$.uid).toBe(header)
    expect(wrapper.find('header .dropdown').exists()).toBe(false)
  })

  it('keeps the user menu open when navigation is cancelled', async () => {
    const { wrapper, router } = await renderRoute('/profile')
    router.beforeEach(() => false)
    await wrapper.get('header button[aria-label="common.userMenu"]').trigger('click')

    await router.push('/available-channels')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/profile')
    expect(wrapper.find('header .dropdown').exists()).toBe(true)
  })

  it('does not rebuild the Stripe page when method switches between full-page and popup presentation', async () => {
    const { wrapper, router } = await renderRoute('/payment/stripe')
    const payment = wrapper.getComponent(StripePaymentView).vm.$.uid
    const layout = wrapper.getComponent(AppLayout).vm.$.uid
    expect(wrapper.findComponent(AppSidebar).exists()).toBe(true)

    await router.replace('/payment/stripe?method=alipay')
    await flushPromises()
    expect(wrapper.findComponent(AppSidebar).exists()).toBe(false)
    expect(wrapper.find('header').exists()).toBe(false)
    expect(wrapper.find('[role="main"]').exists()).toBe(false)
    expect(wrapper.getComponent(StripePaymentView).vm.$.uid).toBe(payment)
    expect(wrapper.getComponent(AppLayout).vm.$.uid).toBe(layout)
    expect(onboardingStore.setReplayCallback).toHaveBeenLastCalledWith(null)

    await router.replace('/payment/stripe')
    await flushPromises()
    expect(wrapper.getComponent(StripePaymentView).vm.$.uid).toBe(payment)
    expect(wrapper.findComponent(AppSidebar).exists()).toBe(true)
    expect(startOnboarding).toHaveBeenCalledTimes(2)
  })

  it('shares the shell with authenticated embedded Model Plaza, but hides chrome for public presentation', async () => {
    const { wrapper, router } = await renderRoute('/profile')
    const sidebar = wrapper.getComponent(AppSidebar).vm.$.uid
    await router.push('/model-plaza?embedded=1')
    await flushPromises()
    const plaza = wrapper.getComponent(ModelPlazaView).vm.$.uid
    expect(wrapper.getComponent(AppSidebar).vm.$.uid).toBe(sidebar)
    expect(wrapper.find('plaza-nav-bar-stub').exists()).toBe(false)

    await router.replace('/model-plaza')
    await flushPromises()
    expect(wrapper.findComponent(AppSidebar).exists()).toBe(false)
    expect(wrapper.find('plaza-nav-bar-stub').exists()).toBe(true)
    expect(wrapper.getComponent(ModelPlazaView).vm.$.uid).toBe(plaza)

    authStore.isAuthenticated = false
    await router.replace('/model-plaza?embedded=1')
    await flushPromises()
    expect(wrapper.findComponent(AppSidebar).exists()).toBe(false)
    expect(wrapper.find('plaza-nav-bar-stub').exists()).toBe(true)

    authStore.isAuthenticated = true
    await flushPromises()
    expect(wrapper.findComponent(AppSidebar).exists()).toBe(true)
    expect(wrapper.getComponent(ModelPlazaView).vm.$.uid).toBe(plaza)
  })

  it('releases the workspace on standalone navigation and remounts it on return', async () => {
    const { wrapper, router } = await renderRoute('/profile')
    const sidebar = wrapper.getComponent(AppSidebar).vm.$.uid
    await router.push('/not-a-real-page')
    await flushPromises()
    expect(wrapper.findComponent(AppLayout).exists()).toBe(false)
    expect(wrapper.findComponent(AppSidebar).exists()).toBe(false)
    await router.back()
    await flushPromises()
    expect(wrapper.getComponent(AppSidebar).vm.$.uid).not.toBe(sidebar)
    expect(wrapper.findAll('aside.sidebar')).toHaveLength(1)
  })

  it('declares the shell on workspace routes, independently of auth requirements', async () => {
    const { default: router } = await import('@/router')
    for (const path of ['/admin/dashboard', '/admin/accounts', '/admin/groups/2/shards', '/admin/packages',
      '/admin/orders/plans', '/admin/affiliates/invites', '/monitor', '/dashboard', '/custom/test',
      '/docs/batch-image', '/payment/airwallex', '/payment/stripe', '/model-plaza']) {
      expect(router.resolve(path).matched.length, path).toBe(2)
    }
    for (const path of ['/home', '/login', '/auth/desktop', '/oauth/mcp/authorize', '/setup',
      '/payment/stripe-popup', '/payment/result', '/legal/privacy', '/not-a-real-page']) {
      expect(router.resolve(path).matched.length, path).toBe(1)
    }
    expect(router.resolve('/admin/accounts').meta.appLayoutClass).toBe('accounts-workspace')
    expect(router.resolve('/admin/groups').meta.appLayoutClass).toBeUndefined()
  })

  it('keeps every page content-only, including lazily selected channel monitor variants', () => {
    const files = ['views', 'pages', 'features'].flatMap(directory =>
      readdirSync(resolve('src', directory), { recursive: true })
        .filter(path => path.endsWith('.vue') && !path.includes('__tests__'))
        .map(path => `${directory}/${path.replaceAll('\\', '/')}`),
    )
    expect(files.length).toBeGreaterThan(40)
    for (const path of files) {
      const source = readFileSync(resolve('src', path), 'utf8')
      expect(source, path).not.toMatch(/<AppLayout\b|import AppLayout\b|: AppLayout\b/)
    }
  })

  it.each([
    ['/', '/home'], ['/admin', '/admin/dashboard'],
    ['/admin/channels', '/admin/channels/pricing'], ['/admin/affiliates', '/admin/affiliates/invites'],
  ])('preserves the redirect %s -> %s', async (path, expected) => {
    const { default: configuredRouter } = await import('@/router')
    const router = createRouter({ history: createMemoryHistory(), routes: configuredRouter.options.routes })
    await router.push(path)
    expect(router.currentRoute.value.path).toBe(expected)
  })

  it.each(['1', 'true'])('fullscreen Ops hides chrome without replacing its route outlet (%s)', async (fullscreen) => {
    const { default: configuredRouter } = await import('@/router')
    // Keep the real route metadata and layout, isolating the dashboard's data polling.
    const workspace = configuredRouter.options.routes.find(route => route.children)
    if (!workspace?.children) throw new Error('Workspace parent is missing')
    const ops = workspace.children.find(route => route.name === 'AdminOps')
    if (!ops) throw new Error('Ops route is missing')
    const router = createRouter({
      history: createMemoryHistory(),
      routes: configuredRouter.options.routes.map(route => route === workspace ? {
        ...route,
        children: route.children!.map(child => child === ops ? {
          ...child, component: { template: '<input aria-label="Ops state" />' },
        } : child),
      } : route),
    })
    await router.push('/admin/ops')
    const wrapper = mount(RouterView, { global: { plugins: [router], stubs } })
    await flushPromises()
    const content = wrapper.get('input').element
    await wrapper.get('input').setValue('preserved')
    await router.replace(`/admin/ops?fullscreen=${fullscreen}`)
    await flushPromises()
    expect(wrapper.findComponent(AppSidebar).exists()).toBe(false)
    expect(wrapper.get('input').element).toBe(content)
    expect((content as HTMLInputElement).value).toBe('preserved')
    await router.replace('/admin/ops')
    await flushPromises()
    expect(wrapper.findComponent(AppSidebar).exists()).toBe(true)
    expect(wrapper.get('input').element).toBe(content)
  })
})
