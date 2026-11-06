import { defineComponent, ref } from 'vue'
import { createPinia } from 'pinia'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import LocaleSwitcher from '../LocaleSwitcher.vue'
import SubscriptionProgressMini from '../SubscriptionProgressMini.vue'
import VersionBadge from '../VersionBadge.vue'
import AnnouncementBell from '../AnnouncementBell.vue'
import { useAnnouncementStore } from '@/stores/announcements'

const { appStore, subscriptionStore, performUpdate } = vi.hoisted(() => ({
  appStore: {
    currentVersion: '1.0.0', latestVersion: '1.1.0', hasUpdate: true,
    versionLoading: false, buildType: 'release', releaseInfo: null,
    fetchVersion: vi.fn().mockResolvedValue(undefined), clearVersionCache: vi.fn(),
    showError: vi.fn(), showSuccess: vi.fn(),
  },
  subscriptionStore: {
    activeSubscriptions: [{ id: 1, group_id: 1, group: { name: 'Workspace Plan' } }],
    hasActiveSubscriptions: true, fetchActiveSubscriptions: vi.fn().mockResolvedValue(undefined),
  },
  performUpdate: vi.fn(),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => ({ isAdmin: true }),
  useSubscriptionStore: () => subscriptionStore,
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/api', () => ({ announcementsAPI: { markRead: vi.fn() } }))
vi.mock('@/api/admin/system', () => ({
  performUpdate, restartService: vi.fn(), getRollbackVersions: vi.fn(), rollback: vi.fn(),
}))
vi.mock('@/utils/featureFlags', () => ({
  FeatureFlags: { subscription: 'subscription' }, isFeatureFlagEnabled: () => true,
}))
vi.mock('@/i18n', () => ({
  setLocale: vi.fn(),
  availableLocales: [{ code: 'en', name: 'English', flag: 'EN' }, { code: 'zh', name: 'Chinese', flag: 'ZH' }],
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: ref('en') }) }))
vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: ref(false), copyToClipboard: vi.fn() }),
}))
vi.mock('@/utils/format', () => ({ formatRelativeTime: () => 'now', formatRelativeWithDateTime: () => 'now' }))

enableAutoUnmount(afterEach)
beforeEach(() => vi.clearAllMocks())
afterEach(() => {
  document.body.innerHTML = ''
  document.body.style.overflow = ''
})

const overlays = [
  { name: 'locale dropdown', component: LocaleSwitcher, selector: '.absolute.right-0', detail: false },
  { name: 'subscription tooltip', component: SubscriptionProgressMini, selector: '.absolute.right-0', detail: false },
  { name: 'version dropdown', component: VersionBadge, selector: '.absolute.left-0', detail: false },
  { name: 'announcement list', component: AnnouncementBell, selector: '.fixed.inset-0', detail: false },
  { name: 'announcement detail', component: AnnouncementBell, selector: '.markdown-body', detail: true },
]

async function openOverlay(overlay: typeof overlays[number]) {
  const page = defineComponent({ template: '<div>Page</div>' })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/one', component: page }, { path: '/two', component: page },
      { path: '/subscriptions', component: page },
    ],
  })
  await router.push('/one')
  await router.push('/two')
  const pinia = createPinia()
  useAnnouncementStore(pinia).announcements = [{
    id: 1, title: 'Workspace Notice', content: 'Notice details', notify_mode: 'silent',
    created_at: '2026-11-07', updated_at: '2026-11-07', read_at: '2026-11-07',
  }]
  const host = document.createElement('div')
  document.body.appendChild(host)
  const wrapper = mount(overlay.component, {
    attachTo: host,
    global: { plugins: [pinia, router], stubs: { Icon: true, Transition: true } },
  })
  const trigger = wrapper.get('button')
  await trigger.trigger('click')
  if (overlay.detail) {
    (document.body.querySelector('.group.relative') as HTMLElement).click()
    await flushPromises()
  }
  expect(document.body.querySelector(overlay.selector)).not.toBeNull()
  return { router, wrapper, trigger }
}

describe.each(overlays)('$name across persistent workspace navigation', (overlay) => {
  it.each(['programmatic', 'history', 'query and hash'])('closes on committed %s navigation without remounting', async (navigation) => {
    const { router, wrapper, trigger } = await openOverlay(overlay)
    if (navigation === 'history') {
      await new Promise<void>((resolve) => {
        const remove = router.afterEach(() => { remove(); resolve() })
        router.back()
      })
    } else {
      await router.push(navigation === 'programmatic' ? '/one' : '/two?tab=details#section')
    }
    await flushPromises()
    expect(document.body.querySelector(overlay.selector)).toBeNull()
    expect(wrapper.get('button').element).toBe(trigger.element)
    if (overlay.component === AnnouncementBell) {
      expect(document.body.querySelectorAll('.fixed.inset-0')).toHaveLength(0)
      expect(document.body.style.overflow).toBe('')
    }
    await trigger.trigger('click')
    if (overlay.component === AnnouncementBell) {
      expect(document.body.textContent).toContain('Workspace Notice')
      expect(document.body.querySelector('.markdown-body')).toBeNull()
    } else {
      expect(document.body.querySelector(overlay.selector)).not.toBeNull()
    }
    if (overlay.component === SubscriptionProgressMini) {
      expect(wrapper.text()).toContain('Workspace Plan')
      expect(subscriptionStore.fetchActiveSubscriptions).toHaveBeenCalledTimes(1)
    }
    if (overlay.component === VersionBadge) expect(appStore.fetchVersion).toHaveBeenCalledTimes(1)
  })

  it('stays open when navigation is cancelled', async () => {
    const { router } = await openOverlay(overlay)
    router.beforeEach(() => false)
    await router.push('/one')
    await flushPromises()
    expect(document.body.querySelector(overlay.selector)).not.toBeNull()
    if (overlay.component === AnnouncementBell) expect(document.body.style.overflow).toBe('hidden')
  })
})

it.each([false, true])('explicit subscription link clicks close even if navigation is cancelled (cancelled: %s)', async (cancelled) => {
  const { router, wrapper } = await openOverlay(overlays[1]!)
  if (cancelled) router.beforeEach(() => false)
  await wrapper.get('a[href="/subscriptions"]').trigger('click')
  await flushPromises()
  expect(document.body.querySelector('.absolute.right-0')).toBeNull()
  expect(router.currentRoute.value.fullPath).toBe(cancelled ? '/two' : '/subscriptions')
})

it('closes the subscription tooltip on an explicit link click to the current route', async () => {
  const { router, wrapper, trigger } = await openOverlay(overlays[1]!)
  await router.push('/subscriptions')
  await flushPromises()
  await trigger.trigger('click')
  expect(document.body.querySelector('.absolute.right-0')).not.toBeNull()
  await wrapper.get('a[href="/subscriptions"]').trigger('click')
  await flushPromises()
  expect(document.body.querySelector('.absolute.right-0')).toBeNull()
  expect(router.currentRoute.value.fullPath).toBe('/subscriptions')
})

it('closes the version dropdown without cancelling an in-flight update', async () => {
  let finishUpdate!: (value: { need_restart: boolean }) => void
  performUpdate.mockReturnValue(new Promise((resolve) => { finishUpdate = resolve }))
  const { router, wrapper, trigger } = await openOverlay(overlays[2]!)
  await wrapper.findAll('button').find(button => button.text() === 'version.updateNow')!.trigger('click')
  expect(wrapper.text()).toContain('version.updating')
  await router.push('/one')
  await flushPromises()
  expect(document.body.querySelector('.absolute.left-0')).toBeNull()
  finishUpdate({ need_restart: true })
  await flushPromises()
  await trigger.trigger('click')
  expect(wrapper.text()).toContain('version.restartNow')
  expect(performUpdate).toHaveBeenCalledTimes(1)
  expect(appStore.clearVersionCache).toHaveBeenCalledTimes(1)
})
