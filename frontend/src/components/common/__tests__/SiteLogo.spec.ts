import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import SiteLogo from '../SiteLogo.vue'
import siteLogoSource from '../SiteLogo.vue?raw'

const dir = dirname(fileURLToPath(import.meta.url))
enableAutoUnmount(afterEach)

describe('SiteLogo', () => {
  it('references a valid reusable mark in the standalone favicon asset', () => {
    const source = readFileSync(resolve(dir, '../../../../public/logo.svg'), 'utf8')
    const svg = new DOMParser().parseFromString(source, 'image/svg+xml')

    expect(svg.querySelector('parsererror')).toBeNull()
    expect(svg.documentElement.getAttribute('viewBox')).toBe('0 0 100 100')
    expect(svg.getElementById('matcha-mark')?.querySelectorAll('path')).toHaveLength(2)
  })

  it('renders the shared Matcha mark by default', () => {
    const wrapper = mount(SiteLogo)

    expect(wrapper.get('svg').attributes()).toMatchObject({
      viewBox: '0 0 100 100',
      role: 'img',
      'aria-label': 'Logo',
    })
    expect(wrapper.get('use').attributes('href')).toBe('/logo.svg#matcha-mark')
    expect(wrapper.find('img').exists()).toBe(false)
  })

  it('takes its color from the application theme rather than owning a second dark-mode policy', () => {
    expect(siteLogoSource).toContain('color: var(--ui-logo)')
    expect(siteLogoSource).not.toContain('.dark .site-logo')
  })

  it.each([
    'https://example.com/custom-logo.png',
    '/uploads/custom-logo.svg',
    'data:image/png;base64,abc',
  ])('preserves the configured logo: %s', (src) => {
    const wrapper = mount(SiteLogo, { props: { src, alt: 'Custom site' } })

    expect(wrapper.get('img').attributes()).toMatchObject({ src, alt: 'Custom site' })
    expect(wrapper.find('svg').exists()).toBe(false)
  })

  it.each(['javascript:alert(1)', 'data:text/html,<script>alert(1)</script>', ''])(
    'falls back to the default mark for an empty or unsafe URL: %s',
    (src) => {
      const wrapper = mount(SiteLogo, { props: { src } })

      expect(wrapper.find('svg').exists()).toBe(true)
      expect(wrapper.find('img').exists()).toBe(false)
    },
  )

  it('returns to the default mark when the configured logo is cleared', async () => {
    const wrapper = mount(SiteLogo, { props: { src: '/uploads/custom-logo.svg' } })

    await wrapper.setProps({ src: '' })

    expect(wrapper.get('use').attributes('href')).toBe('/logo.svg#matcha-mark')
    expect(wrapper.find('img').exists()).toBe(false)
  })

  it('can render a decorative logo next to an accessible site name', () => {
    const wrapper = mount(SiteLogo, { props: { alt: '' } })

    expect(wrapper.get('svg').attributes('aria-hidden')).toBe('true')
    expect(wrapper.get('svg').attributes('aria-label')).toBeUndefined()
  })

  it.each(['', '/uploads/custom-logo.svg'])('preserves layout classes on either root (src: %s)', src => {
    const wrapper = mount(SiteLogo, { props: { src }, attrs: { class: 'brand-logo' } })

    expect(wrapper.classes()).toContain('brand-logo')
  })
})
