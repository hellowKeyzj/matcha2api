import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import postcss, { type Root } from 'postcss'
import tailwindcss from 'tailwindcss'
import authLayout from '../../components/layout/AuthLayout.vue?raw'
import homeView from '../../views/HomeView.vue?raw'

const themeCss = readFileSync(resolve('src/styles/component-theme.css'), 'utf8')
const workspaceCss = readFileSync(resolve('src/styles/workspace.css'), 'utf8')

function declaration(root: Root, selector: string, property: string) {
  let value: string | undefined
  root.walkRules(selector, rule => {
    rule.walkDecls(property, decl => { value = decl.value })
  })
  return value
}

describe('warm light theme', () => {
  it('owns the canvas, chrome, surface and border colors centrally', () => {
    const root = postcss.parse(themeCss)
    expect(declaration(root, '.theme-dala', '--ui-canvas')).toBe('#f5f2ec')
    expect(declaration(root, '.theme-dala', '--ui-chrome')).toBe('#f8f5ef')
    expect(declaration(root, '.theme-dala', '--ui-surface')).toBe('rgb(var(--palette-surface))')
    expect(declaration(root, '.theme-dala', '--palette-surface')).toBe('252 250 247')
    expect(declaration(root, '.theme-dala', '--ui-line')).toBe('#e5e0d8')
    expect(declaration(root, '.theme-dala', '--ui-action')).toBe('#8052ff')
    expect(declaration(root, '.theme-dala', '--ui-logo')).toBe('#241f2b')
  })

  it('preserves the dark theme and limits warm gray surfaces to light mode', () => {
    const root = postcss.parse(themeCss)
    expect(declaration(root, '.dark .theme-dala', '--ui-canvas')).toBe('#000000')
    expect(declaration(root, '.dark .theme-dala', '--ui-chrome')).toBe('var(--ui-canvas)')
    expect(declaration(root, '.dark .theme-dala', '--palette-surface')).toBe('0 0 0')
    expect(declaration(root, '.dark .theme-dala', '--ui-logo')).toBe('#f5f2ec')
    expect(declaration(root, '.theme-dala', '--palette-gray-50')).toBe('250 250 250')
    expect(declaration(root, ':root:not(.dark) .theme-dala', '--palette-gray-50')).toBe('248 245 239')
    expect(declaration(root, ':root:not(.dark) .theme-dala', '--palette-gray-200')).toBe('229 224 216')
  })

  it('uses the shared colors in the home, login and workspace shell', () => {
    expect(authLayout).toContain('--auth-background: var(--ui-canvas)')
    expect(authLayout).toContain('--auth-panel: rgb(var(--palette-surface) / 0.85)')
    expect(authLayout).toContain('--auth-input: var(--ui-chrome)')
    expect(homeView).toContain('--home-bg: var(--ui-canvas)')
    const root = postcss.parse(workspaceCss)
    expect(declaration(root, '.app-layout .sidebar', 'background')).toBe('var(--ui-chrome)')
    expect(declaration(root, '.app-layout header.glass', 'background')).toBe('var(--ui-chrome)')
  })

  it('compiles explicit surfaces and opacity without redefining literal white', async () => {
    const result = await postcss([tailwindcss({
      content: [{ raw: 'bg-surface bg-surface/50 bg-white bg-white/50 text-white', extension: 'html' }],
    })]).process(readFileSync(resolve('src/style.css'), 'utf8'), { from: resolve('src/style.css') })
    expect(declaration(result.root, '.bg-surface', 'background-color')).toContain('var(--palette-surface, 255 255 255)')
    expect(declaration(result.root, '.bg-surface\\/50', 'background-color')).toBe('rgb(var(--palette-surface, 255 255 255) / 0.5)')
    expect(declaration(result.root, '.bg-white', 'background-color')).toContain('255 255 255')
    expect(declaration(result.root, '.bg-white', 'background-color')).not.toContain('--palette-surface')
    expect(declaration(result.root, '.bg-white\\/50', 'background-color')).toBe('rgb(255 255 255 / 0.5)')
    expect(declaration(result.root, '.text-white', 'color')).toContain('255 255 255')
    expect(declaration(result.root, '.text-white', 'color')).not.toContain('--palette-surface')
  })
})
