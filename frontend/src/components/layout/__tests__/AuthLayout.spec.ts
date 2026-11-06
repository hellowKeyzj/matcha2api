import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AuthLayout from '../AuthLayout.vue'
import DalaConstellation from '@/components/common/DalaConstellation.vue'
import Icon from '@/components/icons/Icon.vue'
import { initializeTheme } from '@/composables/useTheme'

vi.mock('@/stores', () => ({
  useAppStore: () => ({ fetchPublicSettings: vi.fn(), publicSettingsLoaded: true })
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

beforeEach(() => {
  vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
  initializeTheme()
})

afterEach(() => {
  document.documentElement.classList.remove('dark')
  localStorage.removeItem('theme')
})

describe('AuthLayout background', () => {
  function render(animatedBackground = false) {
    return mount(AuthLayout, {
      props: { animatedBackground },
      slots: { default: '<button>Login</button>' },
      global: { stubs: { DalaConstellation: true } }
    })
  }

  it('uses the full-screen animation only when requested, preserving the form', () => {
    const wrapper = render(true)
    expect(wrapper.classes()).not.toContain('dark')
    expect(wrapper.find('.auth-particles dala-constellation-stub').exists()).toBe(true)
    expect(wrapper.find('.auth-decoration').exists()).toBe(false)
    expect(wrapper.get('.auth-card button').text()).toBe('Login')
    expect(wrapper.getComponent(DalaConstellation).props('light')).toBe(true)
    expect(wrapper.getComponent(DalaConstellation).props('align')).toBe('left')
    wrapper.unmount()
  })

  it('switches the shared theme and persists the preference', async () => {
    const wrapper = render(true)
    const toggle = wrapper.get('.auth-theme-toggle')
    expect(toggle.text()).toBe('')
    expect(toggle.attributes('aria-label')).toBe('nav.darkMode')
    expect(toggle.get('svg').attributes('aria-hidden')).toBe('true')
    expect(wrapper.getComponent(Icon).props('name')).toBe('moon')
    await toggle.trigger('click')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')
    expect(toggle.text()).toBe('')
    expect(toggle.attributes('aria-label')).toBe('nav.lightMode')
    expect(wrapper.getComponent(Icon).props('name')).toBe('sun')
    expect(wrapper.getComponent(DalaConstellation).props('light')).toBe(false)
    await toggle.trigger('click')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(localStorage.getItem('theme')).toBe('light')
    expect(toggle.attributes('aria-label')).toBe('nav.darkMode')
    expect(wrapper.getComponent(Icon).props('name')).toBe('moon')
    expect(wrapper.getComponent(DalaConstellation).props('light')).toBe(true)
    wrapper.unmount()
  })

  it('starts with the existing dark preference', () => {
    localStorage.setItem('theme', 'dark')
    initializeTheme()
    const wrapper = render(true)
    expect(wrapper.get('.auth-theme-toggle').text()).toBe('')
    expect(wrapper.get('.auth-theme-toggle').attributes('aria-label')).toBe('nav.lightMode')
    expect(wrapper.getComponent(Icon).props('name')).toBe('sun')
    wrapper.unmount()
  })

  it('keeps mounted theme controls and particle scenes in sync', async () => {
    const first = render(true)
    const second = render(true)

    await first.get('.auth-theme-toggle').trigger('click')

    expect(second.get('.auth-theme-toggle').attributes('aria-label')).toBe('nav.lightMode')
    expect(second.getComponent(Icon).props('name')).toBe('sun')
    expect(second.getComponent(DalaConstellation).props('light')).toBe(false)

    await second.get('.auth-theme-toggle').trigger('click')
    expect(first.getComponent(DalaConstellation).props('light')).toBe(true)
    expect(document.documentElement.classList.contains('dark')).toBe(false)

    first.unmount()
    second.unmount()
  })

  it('keeps other auth screens free of the animation', () => {
    const wrapper = render()
    expect(wrapper.classes()).not.toContain('dark')
    expect(wrapper.find('.auth-particles').exists()).toBe(false)
    expect(wrapper.find('.auth-decoration').exists()).toBe(true)
    expect(wrapper.find('.auth-theme-toggle').exists()).toBe(false)
    wrapper.unmount()
  })
})
