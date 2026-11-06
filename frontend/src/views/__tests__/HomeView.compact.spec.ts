import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

import HomeView from '../HomeView.vue'
import { initializeTheme } from '@/composables/useTheme'
import enLanding from '@/i18n/locales/en/landing'
import zhLanding from '@/i18n/locales/zh/landing'

const { appStore, authStore } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: {} as Record<string, unknown>,
    siteName: 'Fallback site',
    siteLogo: '',
    docUrl: '',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn(),
  },
  authStore: {
    isAuthenticated: false,
    isAdmin: false,
    user: null as { email?: string } | null,
    checkAuth: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

function mountHome(settings: Record<string, unknown> = {}) {
  appStore.cachedPublicSettings = {
    site_name: 'Test site',
    site_subtitle: 'Test subtitle',
    ...settings,
  }

  return mount(HomeView, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        LocaleSwitcher: { template: '<div data-testid="locale-switcher" />' },
        Icon: { template: '<span data-testid="icon" />' },
        DalaConstellation: { template: '<div data-testid="constellation" />' },
      },
    },
  })
}

function compactDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper.findAllComponents(RouterLinkStub).find(link => link.classes().includes('home-cta'))?.props('to')
}

function modelPlazaDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper
    .findAllComponents(RouterLinkStub)
    .find((link) => link.props('to') === '/model-plaza')
    ?.props('to')
}

describe('HomeView compact mode', () => {
  beforeEach(() => {
    authStore.isAuthenticated = false
    authStore.isAdmin = false
    authStore.user = null
    authStore.checkAuth.mockClear()
    appStore.fetchPublicSettings.mockClear()
    localStorage.clear()
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
    initializeTheme()
  })

  it('renders custom HTML ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: '<section id="custom-home">Custom home</section>',
    })

    expect(wrapper.get('#custom-home').text()).toBe('Custom home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('renders custom URL content ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: ' https://example.com/home ',
    })

    expect(wrapper.get('iframe').attributes('src')).toBe('https://example.com/home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('treats whitespace-only custom content as empty and selects compact mode', () => {
    const wrapper = mountHome({ compact_home_enabled: true, home_content: ' \n\t ' })

    expect(wrapper.get('[data-testid="compact-home"]').text()).toContain('Test site')
  })

  it.each([undefined, false])('selects the default home when compact mode is %s', (enabled) => {
    const settings = enabled === undefined ? {} : { compact_home_enabled: enabled }
    const wrapper = mountHome(settings)

    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="constellation"]').exists()).toBe(true)
    expect(wrapper.find('#gateway-details').exists()).toBe(true)
  })

  it('links unauthenticated visitors to login', () => {
    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toBe('/login')
  })

  it('shares the same hero in compact mode without rendering the extended section', () => {
    const wrapper = mountHome({ compact_home_enabled: true })
    expect(wrapper.find('[data-testid="constellation"]').exists()).toBe(true)
    expect(wrapper.find('#gateway-details').exists()).toBe(false)
    expect(wrapper.find('.terminal-container').exists()).toBe(false)
  })

  it.each([false, true])('keeps the homepage copy minimal (compact: %s)', compact => {
    const wrapper = mountHome({ compact_home_enabled: compact })

    expect(wrapper.find('.home-kicker, .home-description, .home-protocols').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Test subtitle')
    expect(wrapper.get('#home-title').text()).toContain('home.orbit.headline')
    expect(wrapper.get('.home-actions .home-cta').text()).toBe('home.getStarted')
    expect(wrapper.findAll('.home-section-copy > p').map(paragraph => paragraph.text())).toEqual(compact ? [] : [
      'home.features.unifiedGatewayDesc',
      'home.features.multiAccountDesc',
      'home.features.balanceQuotaDesc',
    ])
    expect(wrapper.findAll('.home-footer > span').map(span => span.text())).toEqual([`© ${new Date().getFullYear()} Test site`])
    expect(wrapper.get('.home-footer a').text()).toContain('GitHub')
    wrapper.unmount()
  })

  it.each([
    { language: 'en', home: enLanding.home },
    { language: 'zh', home: zhLanding.home },
  ])('omits full stops from homepage headings and descriptions ($language)', ({ home }) => {
    const copy = [
      home.orbit.headline,
      home.orbit.headlineAccent,
      ...Object.values(home.constellation),
      home.features.unifiedGatewayDesc,
      home.features.multiAccountDesc,
      home.features.balanceQuotaDesc,
    ]

    for (const text of copy) expect(text).not.toMatch(/[。．.]/)
  })

  it('uses the existing theme preference and toggles without remounting the scene', async () => {
    document.documentElement.classList.remove('dark')
    const wrapper = mountHome({ compact_home_enabled: true })
    expect(wrapper.classes()).toContain('is-light')
    const globe = wrapper.get('[data-testid="constellation"]').element
    await wrapper.get('.home-theme').trigger('click')
    expect(wrapper.classes()).not.toContain('is-light')
    expect(localStorage.getItem('theme')).toBe('dark')
    expect(wrapper.get('[data-testid="constellation"]').element).toBe(globe)
    document.documentElement.classList.remove('dark')
  })

  it('links authenticated users to their dashboard', () => {
    authStore.isAuthenticated = true

    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toBe('/dashboard')
  })

  it('links administrators to the admin dashboard', () => {
    authStore.isAuthenticated = true
    authStore.isAdmin = true

    const wrapper = mountHome({ compact_home_enabled: true })
    expect(compactDestination(wrapper)).toBe('/admin/dashboard')
    expect(authStore.checkAuth).toHaveBeenCalledOnce()
    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
  })

  it('shows the model plaza link to anonymous visitors when public access is enabled', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('hides the model plaza link from anonymous visitors when sign-in is required', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: true,
    })

    expect(modelPlazaDestination(wrapper)).toBeUndefined()
  })

  it('shows the model plaza link to authenticated visitors when sign-in is required', () => {
    authStore.isAuthenticated = true

    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: true,
      model_plaza_require_auth: true,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('shows the model plaza link in the default home header', () => {
    const wrapper = mountHome({
      model_plaza_enabled: true,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBe('/model-plaza')
  })

  it('hides the model plaza link when the feature is disabled', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      model_plaza_enabled: false,
      model_plaza_require_auth: false,
    })

    expect(modelPlazaDestination(wrapper)).toBeUndefined()
  })

  it.each([false, true])('keeps configured documentation outside the hidden mobile navigation (compact: %s)', compact => {
    const wrapper = mountHome({ compact_home_enabled: compact, doc_url: 'https://example.com/docs' })

    const docs = wrapper.get('.home-controls a[href="https://example.com/docs"]')
    expect(docs.attributes('aria-label')).toBe('home.viewDocs')
    expect(docs.attributes('rel')).toBe('noopener noreferrer')
    expect(wrapper.find('.home-nav a[href="https://example.com/docs"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
