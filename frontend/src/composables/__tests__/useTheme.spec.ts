import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { isReadonly } from 'vue'
import { initializeTheme, useTheme } from '../useTheme'

beforeEach(() => {
  localStorage.removeItem('theme')
  vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
})

afterEach(() => {
  localStorage.removeItem('theme')
  initializeTheme()
  vi.restoreAllMocks()
})

describe('application theme', () => {
  it.each(['light', 'dark'])('honors saved %s instead of system preference', saved => {
    localStorage.setItem('theme', saved)
    vi.mocked(window.matchMedia).mockReturnValue({ matches: saved === 'light' } as MediaQueryList)

    initializeTheme()

    expect(useTheme().isDark.value).toBe(saved === 'dark')
    expect(document.documentElement.classList.contains('dark')).toBe(saved === 'dark')
    expect(window.matchMedia).not.toHaveBeenCalled()
  })

  it.each([false, true])('uses system preference (%s) only when no preference is saved', dark => {
    vi.mocked(window.matchMedia).mockReturnValue({ matches: dark } as MediaQueryList)

    initializeTheme()

    expect(useTheme().isDark.value).toBe(dark)
    expect(document.documentElement.classList.contains('dark')).toBe(dark)
    expect(localStorage.getItem('theme')).toBeNull()
  })

  it('shares read-only state and applies explicit selections to storage and DOM', () => {
    initializeTheme()
    const home = useTheme()
    const sidebar = useTheme()
    expect(home.isDark).toBe(sidebar.isDark)
    expect(isReadonly(home.isDark)).toBe(true)

    home.toggleTheme()
    expect(sidebar.isDark.value).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)

    sidebar.toggleTheme()
    expect(home.isDark.value).toBe(false)
    expect(localStorage.getItem('theme')).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('does not reinitialize when a consumer mounts and preserves unrelated root classes', () => {
    document.documentElement.classList.add('unrelated')
    initializeTheme()
    useTheme().toggleTheme()
    const readPreference = vi.spyOn(localStorage, 'getItem')

    expect(useTheme().isDark.value).toBe(true)
    expect(readPreference).not.toHaveBeenCalled()
    expect(document.documentElement.classList.contains('unrelated')).toBe(true)
    document.documentElement.classList.remove('unrelated')
  })
})
