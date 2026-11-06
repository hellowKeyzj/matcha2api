import { reactive, ref } from 'vue'
import { enableAutoUnmount, flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AppSidebar from '../AppSidebar.vue'
import AuthLayout from '../AuthLayout.vue'
import PlazaNavBar from '../../modelPlaza/PlazaNavBar.vue'
import HomeView from '@/views/HomeView.vue'
import KeyUsageView from '@/views/KeyUsageView.vue'
import LegalDocumentView from '@/views/public/LegalDocumentView.vue'
import SiteLogo from '@/components/common/SiteLogo.vue'
import { initializeTheme } from '@/composables/useTheme'

const appStore = reactive({
  siteName: 'Matcha',
  siteLogo: '',
  siteVersion: '',
  publicSettingsLoaded: true,
  sidebarCollapsed: false,
  mobileOpen: false,
  sidebarScrollTop: 0,
  cachedPublicSettings: {
    site_name: 'Matcha',
    site_logo: '',
    home_content: '',
    compact_home_enabled: true,
    custom_menu_items: [],
    login_agreement_documents: [],
  },
  fetchPublicSettings: vi.fn(),
})
const authStore = { isAuthenticated: false, isAdmin: false, isSimpleMode: false, checkAuth: vi.fn() }

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
  useOnboardingStore: () => ({ isCurrentStep: () => false }),
  useAdminSettingsStore: () => ({ customMenuItems: [], fetch: vi.fn() }),
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({ canUseBatchImage: ref(false), refreshBatchImageAccess: vi.fn() }),
}))
vi.mock('vue-router', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-router')>(),
  useRoute: () => ({ path: '/home', params: { documentId: 'privacy' } }),
  useRouter: () => ({ push: vi.fn() }),
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('@/i18n', () => ({
  getLocale: () => 'en',
  i18n: { global: { t: (key: string) => key } },
}))

enableAutoUnmount(afterEach)

beforeEach(() => {
  appStore.siteLogo = ''
  appStore.cachedPublicSettings.site_logo = ''
  appStore.fetchPublicSettings.mockResolvedValue(appStore.cachedPublicSettings)
  localStorage.removeItem('theme')
  vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
  initializeTheme()
})

afterEach(() => {
  vi.restoreAllMocks()
  document.documentElement.classList.remove('dark')
})

function setLogo(src: string) {
  appStore.siteLogo = src
  appStore.cachedPublicSettings.site_logo = src
}

describe.each([
  { name: 'sidebar', component: AppSidebar },
  { name: 'authentication', component: AuthLayout },
  { name: 'model plaza', component: PlazaNavBar },
  { name: 'homepage', component: HomeView },
  { name: 'key usage', component: KeyUsageView },
  { name: 'legal document', component: LegalDocumentView },
])('shared site_logo rendering: $name', ({ component }) => {
  function render() {
    return mount(component, {
      global: {
        stubs: { RouterLink: RouterLinkStub, LocaleSwitcher: true, DalaConstellation: true, VersionBadge: true },
      },
    })
  }

  it('uses the shared default mark when no logo is configured', async () => {
    const wrapper = render()
    await flushPromises()

    const logo = wrapper.getComponent(SiteLogo)
    expect(logo.get('use').attributes('href')).toBe('/logo.svg#matcha-mark')
    expect(logo.find('img').exists()).toBe(false)
  })

  if (component === HomeView) {
    it.each(['', '/uploads/custom-logo.svg'])('keeps sizing on the logo and truncation on the name (src: %s)', async src => {
      setLogo(src)
      const wrapper = render()
      await flushPromises()

      expect(wrapper.get('.home-brand-logo').element).toBe(wrapper.getComponent(SiteLogo).element)
      expect(wrapper.get('.home-brand-name').text()).toBe('Matcha')
    })
  }

  it('renders a configured image and returns to the default when cleared', async () => {
    setLogo('https://example.com/custom-logo.png')
    const wrapper = render()
    await flushPromises()

    const logo = wrapper.getComponent(SiteLogo)
    expect(logo.get('img').attributes('src')).toBe('https://example.com/custom-logo.png')

    setLogo('')
    await flushPromises()

    expect(logo.find('img').exists()).toBe(false)
    expect(logo.get('use').attributes('href')).toBe('/logo.svg#matcha-mark')
  })

  it.each(['javascript:alert(1)', 'data:text/html,<script>alert(1)</script>', '//example.com/logo.svg'])(
    'never passes an unsafe configured URL to an image: %s',
    async (src) => {
      setLogo(src)
      const wrapper = render()
      await flushPromises()

      const logo = wrapper.getComponent(SiteLogo)
      expect(logo.find('img').exists()).toBe(false)
      expect(logo.get('use').attributes('href')).toBe('/logo.svg#matcha-mark')
    },
  )
})
