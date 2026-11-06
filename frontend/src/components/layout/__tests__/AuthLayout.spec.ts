import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import AuthLayout from '../AuthLayout.vue'

vi.mock('@/stores', () => ({
  useAppStore: () => ({ fetchPublicSettings: vi.fn(), publicSettingsLoaded: true })
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

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
    wrapper.unmount()
  })

  it('switches the shared theme and persists the preference', async () => {
    const wrapper = render(true)
    const toggle = wrapper.get('.auth-theme-toggle')
    expect(toggle.text()).toBe('nav.darkMode')
    await toggle.trigger('click')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')
    expect(toggle.text()).toBe('nav.lightMode')
    await toggle.trigger('click')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(localStorage.getItem('theme')).toBe('light')
    wrapper.unmount()
  })

  it('starts with the existing dark preference', () => {
    document.documentElement.classList.add('dark')
    const wrapper = render(true)
    expect(wrapper.get('.auth-theme-toggle').text()).toBe('nav.lightMode')
    wrapper.unmount()
  })

  it('keeps other auth screens free of the animation', () => {
    const wrapper = render()
    expect(wrapper.classes()).not.toContain('dark')
    expect(wrapper.find('.auth-particles').exists()).toBe(false)
    expect(wrapper.find('.auth-decoration').exists()).toBe(true)
    wrapper.unmount()
  })
})
