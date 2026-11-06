import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { isNavigationFailure, NavigationFailureType, RouterView, type Router } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  getCurrentUser: vi.fn(), exchangeBrowserLoginTicket: vi.fn(),
}))

vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return { ...actual, createWebHistory: () => actual.createMemoryHistory() }
})
vi.mock('@/api', () => ({ authAPI: api, isTotp2FARequired: () => false, passkeyAPI: {} }))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    siteName: 'Test', backendModeEnabled: false, publicSettingsLoaded: true,
    cachedPublicSettings: {},
  }),
}))
vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}))
vi.mock('@/stores/adminCompliance', () => ({
  useAdminComplianceStore: () => ({ initialized: true }),
}))
vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({ startNavigation: vi.fn(), endNavigation: vi.fn() }),
}))
vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({ triggerPrefetch: vi.fn() }),
}))
vi.mock('@/router/title', () => ({ resolveRouteDocumentTitle: () => 'Test' }))
vi.mock('@/api/setup', () => ({ getSetupStatus: vi.fn().mockResolvedValue({ needs_setup: false }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/components/layout', () => ({ AuthLayout: { template: '<div><slot /></div>' } }))

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: Error) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

function response(id: number) {
  return {
    access_token: `access-${id}`, refresh_token: `refresh-${id}`, expires_in: 3600,
    token_type: 'Bearer',
    user: { id, role: 'user', email: `${id}@example.test`, status: 'active' },
  }
}

const ticket = 'A'.repeat(43)
let router: Router
let wrapper: VueWrapper | undefined
let auth: ReturnType<typeof import('@/stores/auth')['useAuthStore']>
let tickets: typeof import('@/router/desktopLogin')
let desktopView: typeof import('@/views/auth/DesktopLoginView.vue')['default']
let profileTimer: ReturnType<typeof setInterval>
let refreshTimer: ReturnType<typeof setTimeout>

async function settle() {
  for (let i = 0; i < 3; i++) {
    await flushPromises()
    await nextTick()
  }
}

beforeEach(async () => {
  vi.resetModules()
  vi.resetAllMocks()
  vi.useFakeTimers()
  vi.spyOn(globalThis, 'setInterval')
  vi.spyOn(globalThis, 'clearInterval')
  vi.spyOn(globalThis, 'setTimeout')
  vi.spyOn(globalThis, 'clearTimeout')
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  localStorage.clear()
  window.history.replaceState({}, '', '/')
  const pinia = createPinia()
  setActivePinia(pinia)
  api.getCurrentUser.mockResolvedValue({ data: response(1).user })
  auth = (await import('@/stores/auth')).useAuthStore()
  tickets = await import('@/router/desktopLogin')
  desktopView = (await import('@/views/auth/DesktopLoginView.vue')).default
  router = (await import('@/router')).default
  // Keep the production guards and callback; unrelated pages need only an outlet.
  for (const record of router.getRoutes()) {
    if (record.path !== '/auth/desktop' && record.components) {
      record.components.default = { template: '<div><router-view /></div>' }
    }
  }
  localStorage.setItem('auth_token', 'access-1')
  localStorage.setItem('auth_user', JSON.stringify(response(1).user))
  localStorage.setItem('refresh_token', 'refresh-1')
  localStorage.setItem('token_expires_at', String(Date.now() + 3600000))
  await router.push('/profile')
  profileTimer = vi.mocked(setInterval).mock.results[0].value
  const refreshCall = vi.mocked(setTimeout).mock.calls.findIndex((call) => call[1] === 3480000)
  refreshTimer = vi.mocked(setTimeout).mock.results[refreshCall].value
  wrapper = mount(RouterView, { global: { plugins: [pinia, router] } })
  await settle()
  vi.mocked(clearInterval).mockClear()
  vi.mocked(clearTimeout).mockClear()
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  auth?.suspendSession()
  vi.clearAllTimers()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

function deferDesktopLoad() {
  const load = deferred<{ default: typeof desktopView }>()
  router.getRoutes().find((record) => record.path === '/auth/desktop')!.components!.default =
    () => load.promise
  return load
}

async function coldStart() {
  wrapper!.unmount()
  wrapper = undefined
  auth.suspendSession()
  vi.resetModules()
  const pinia = createPinia()
  setActivePinia(pinia)
  auth = (await import('@/stores/auth')).useAuthStore()
  tickets = await import('@/router/desktopLogin')
  desktopView = (await import('@/views/auth/DesktopLoginView.vue')).default
  router = (await import('@/router')).default
  for (const record of router.getRoutes()) {
    if (record.path !== '/auth/desktop' && record.components) {
      record.components.default = { template: '<div><router-view /></div>' }
    }
  }
  return pinia
}

describe('desktop callback navigation ownership', () => {
  it('discards a cancelled lazy callback without pausing the old session or redeeming on hashless re-entry', async () => {
    const load = deferDesktopLoad()
    const navigation = router.push(`/auth/desktop#ticket=${ticket}`)
    await settle()
    const authenticatedWhileLoading = auth.isAuthenticated
    const pendingWhileLoading = tickets.desktopLoginTicket.value
    await router.push('/home')
    load.resolve({ default: desktopView })
    const failure = await navigation
    await settle()

    expect(isNavigationFailure(failure, NavigationFailureType.cancelled)).toBe(true)
    expect(authenticatedWhileLoading).toBe(true)
    expect(pendingWhileLoading).toBeNull()
    expect(auth.isAuthenticated).toBe(true)
    expect(clearInterval).not.toHaveBeenCalledWith(profileTimer)
    expect(clearTimeout).not.toHaveBeenCalledWith(refreshTimer)
    expect(tickets.desktopLoginTicket.value).toBeNull()
    await router.push('/auth/desktop')
    await settle()
    expect(api.exchangeBrowserLoginTicket).not.toHaveBeenCalled()
    expect(wrapper!.get('[role="alert"]').text()).toBe('auth.desktopLogin.invalid')
  })

  it('keeps the old session and timers when the callback chunk fails to load', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const load = deferDesktopLoad()
    const navigation = router.push(`/auth/desktop#ticket=${ticket}`)
    const rejected = expect(navigation).rejects.toThrow('callback load failed')
    await settle()
    load.reject(new Error('callback load failed'))
    await rejected

    expect(router.currentRoute.value.path).toBe('/profile')
    expect(auth.token).toBe('access-1')
    expect(auth.isAuthenticated).toBe(true)
    expect(clearInterval).not.toHaveBeenCalledWith(profileTimer)
    expect(clearTimeout).not.toHaveBeenCalledWith(refreshTimer)
    expect(localStorage.getItem('auth_token')).toBe('access-1')
    expect(tickets.desktopLoginTicket.value).toBeNull()
    expect(api.exchangeBrowserLoginTicket).not.toHaveBeenCalled()
    await router.push('/profile?after=failed')
    expect(router.currentRoute.value.path).toBe('/profile')
  })

  it('still restores persisted auth on the next route when the initial callback fails', async () => {
    await coldStart()
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const load = deferDesktopLoad()
    const navigation = router.push(`/auth/desktop#ticket=${ticket}`)
    const rejected = expect(navigation).rejects.toThrow('initial callback failed')
    await settle()
    load.reject(new Error('initial callback failed'))
    await rejected
    expect(tickets.desktopLoginTicket.value).toBeNull()
    await router.push('/profile')
    expect(router.currentRoute.value.path).toBe('/profile')
    expect(auth.token).toBe('access-1')
    expect(api.exchangeBrowserLoginTicket).not.toHaveBeenCalled()
  })

  it.each(['', '#ticket=short', `#ticket=${ticket}&ticket=${ticket}`])(
    'restores persisted auth after a cold-start invalid callback then /profile (%s)',
    async (fragment) => {
      const pinia = await coldStart()
      expect(auth.isAuthenticated).toBe(false)
      await router.push(`/auth/desktop${fragment}`)
      wrapper = mount(RouterView, { global: { plugins: [pinia, router] } })
      await settle()
      expect(wrapper.get('[role="alert"]').text()).toBe('auth.desktopLogin.invalid')
      expect(api.exchangeBrowserLoginTicket).not.toHaveBeenCalled()
      expect(localStorage.getItem('auth_token')).toBe('access-1')
      await router.push('/profile')
      await settle()
      expect(router.currentRoute.value.path).toBe('/profile')
      expect(auth.token).toBe('access-1')
      expect(auth.isAuthenticated).toBe(true)
    },
  )

  it('does not restore the old persisted session while a cold-start valid exchange is pending', async () => {
    const pinia = await coldStart()
    const exchange = deferred<ReturnType<typeof response>>()
    api.exchangeBrowserLoginTicket.mockReturnValue(exchange.promise)
    await router.push(`/auth/desktop#ticket=${ticket}`)
    wrapper = mount(RouterView, { global: { plugins: [pinia, router] } })
    await settle()
    expect(api.exchangeBrowserLoginTicket).toHaveBeenCalledTimes(1)
    expect(auth.isAuthenticated).toBe(false)
    expect(localStorage.getItem('auth_token')).toBe('access-1')
    await router.push('/profile')
    await settle()
    expect(router.currentRoute.value.path).toBe('/login')
    expect(auth.isAuthenticated).toBe(false)
    exchange.resolve(response(2))
    await settle()
    expect(auth.isAuthenticated).toBe(false)
    expect(localStorage.getItem('auth_token')).toBe('access-1')
    expect(router.currentRoute.value.path).toBe('/login')
  })

  it('does not pause or retain a ticket when another guard rejects the clean callback', async () => {
    router.beforeEach((to) => to.path === '/auth/desktop' ? false : undefined)
    const failure = await router.push(`/auth/desktop#ticket=${ticket}`)
    expect(isNavigationFailure(failure, NavigationFailureType.aborted)).toBe(true)
    expect(router.currentRoute.value.path).toBe('/profile')
    expect(auth.token).toBe('access-1')
    expect(clearInterval).not.toHaveBeenCalledWith(profileTimer)
    expect(clearTimeout).not.toHaveBeenCalledWith(refreshTimer)
    expect(tickets.desktopLoginTicket.value).toBeNull()
    expect(api.exchangeBrowserLoginTicket).not.toHaveBeenCalled()
  })

  it('does not let an older load failure discard a newer callback ticket', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const oldLoad = deferred<{ default: typeof desktopView }>()
    const newLoad = deferred<{ default: typeof desktopView }>()
    router.getRoutes().find((record) => record.path === '/auth/desktop')!.components!.default =
      vi.fn().mockReturnValueOnce(oldLoad.promise).mockReturnValueOnce(newLoad.promise)
    const first = router.push(`/auth/desktop#ticket=${ticket}`)
    const rejected = expect(first).rejects.toThrow('older callback failed')
    await settle()
    const second = router.push(`/auth/desktop#ticket=${'B'.repeat(43)}`)
    await settle()
    oldLoad.reject(new Error('older callback failed'))
    await rejected
    expect(auth.isAuthenticated).toBe(true)
    api.exchangeBrowserLoginTicket.mockResolvedValue(response(3))
    newLoad.resolve({ default: desktopView })
    await second
    await settle()
    expect(api.exchangeBrowserLoginTicket).toHaveBeenCalledTimes(1)
    expect(api.exchangeBrowserLoginTicket).toHaveBeenCalledWith('B'.repeat(43), expect.any(AbortSignal))
    expect(auth.user?.id).toBe(3)
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it.each(['', '#ticket=short', `#ticket=${ticket}&ticket=${ticket}`])(
    'rejects a missing or malformed fragment without pausing the old session (%s)',
    async (fragment) => {
      await router.push(`/auth/desktop${fragment}`)
      await settle()
      expect(wrapper!.get('[role="alert"]').text()).toBe('auth.desktopLogin.invalid')
      expect(api.exchangeBrowserLoginTicket).not.toHaveBeenCalled()
      expect(auth.token).toBe('access-1')
      expect(tickets.desktopLoginTicket.value).toBeNull()
    },
  )

  it('scrubs the fragment and exchanges once only after the callback route and View succeed', async () => {
    const load = deferDesktopLoad()
    const exchange = deferred<ReturnType<typeof response>>()
    api.exchangeBrowserLoginTicket.mockReturnValue(exchange.promise)
    window.history.replaceState({}, '', `/auth/desktop#ticket=${ticket}`)
    const navigation = router.push(`/auth/desktop#ticket=${ticket}`)
    await settle()
    const authenticatedWhileLoading = auth.isAuthenticated
    const exchangesWhileLoading = api.exchangeBrowserLoginTicket.mock.calls.length
    const hashWhileLoading = window.location.hash
    load.resolve({ default: desktopView })
    await navigation
    await settle()

    expect(authenticatedWhileLoading).toBe(true)
    expect(exchangesWhileLoading).toBe(0)
    expect(hashWhileLoading).toBe('')
    expect(router.currentRoute.value.fullPath).toBe('/auth/desktop')
    expect(api.exchangeBrowserLoginTicket).toHaveBeenCalledTimes(1)
    expect(api.exchangeBrowserLoginTicket).toHaveBeenCalledWith(ticket, expect.any(AbortSignal))
    expect(auth.isAuthenticated).toBe(false)
    expect(clearInterval).toHaveBeenCalledWith(profileTimer)
    expect(clearTimeout).toHaveBeenCalledWith(refreshTimer)
    expect(localStorage.getItem('auth_token')).toBe('access-1')
    expect(tickets.desktopLoginTicket.value).toBeNull()
    exchange.resolve(response(2))
    await settle()
    expect(auth.user?.id).toBe(2)
    expect(localStorage.getItem('auth_token')).toBe('access-2')
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('accepts a second ticket on the mounted View and ignores a late first response', async () => {
    const first = deferred<ReturnType<typeof response>>()
    const second = deferred<ReturnType<typeof response>>()
    api.exchangeBrowserLoginTicket.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    await router.push(`/auth/desktop#ticket=${ticket}`)
    await settle()
    const instance = wrapper!.getComponent(desktopView).vm.$.uid
    const firstSignal = api.exchangeBrowserLoginTicket.mock.calls[0][1] as AbortSignal
    await router.push(`/auth/desktop#ticket=${'B'.repeat(43)}`)
    await settle()

    expect(wrapper!.getComponent(desktopView).vm.$.uid).toBe(instance)
    expect(api.exchangeBrowserLoginTicket).toHaveBeenCalledTimes(2)
    expect(firstSignal.aborted).toBe(true)
    first.resolve(response(2))
    await settle()
    expect(auth.isAuthenticated).toBe(false)
    expect(localStorage.getItem('auth_token')).toBe('access-1')
    expect(router.currentRoute.value.path).toBe('/auth/desktop')
    second.resolve(response(3))
    await settle()
    expect(auth.user?.id).toBe(3)
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('does not write credentials or redirect when the View unmounts before exchange finishes', async () => {
    const exchange = deferred<ReturnType<typeof response>>()
    api.exchangeBrowserLoginTicket.mockReturnValue(exchange.promise)
    await router.push(`/auth/desktop#ticket=${ticket}`)
    await settle()
    const signal = api.exchangeBrowserLoginTicket.mock.calls[0][1] as AbortSignal
    await router.push('/home')
    await settle()
    expect(signal.aborted).toBe(true)
    exchange.resolve(response(2))
    await settle()
    expect(auth.isAuthenticated).toBe(false)
    expect(localStorage.getItem('auth_token')).toBe('access-1')
    expect(router.currentRoute.value.path).toBe('/home')
    expect(tickets.desktopLoginTicket.value).toBeNull()
  })
})
