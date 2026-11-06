import { describe, expect, it } from 'vitest'

const sources = import.meta.glob('../../**/*.vue', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('application theme ownership', () => {
  it('keeps preference persistence and root theme mutations out of components', () => {
    const violations = Object.entries(sources).flatMap(([file, source]) => {
      const writesTheme = /classList\.(?:toggle|add|remove)\(['"]dark['"]/.test(source)
        || /localStorage\.(?:getItem|setItem)\(['"]theme['"]/.test(source)
      return writesTheme ? [file] : []
    })
    expect(violations).toEqual([])
  })

  it('keeps switch thumb pseudo-elements independent of themed surfaces', () => {
    const violations = Object.entries(sources).flatMap(([file, source]) => {
      return /\b(?:before|after):bg-surface\b/.test(source) ? [file] : []
    })
    expect(violations).toEqual([])
  })

  it('uses the primary palette for blue-family interaction accents across pages and components', () => {
    const violations = Object.entries(sources).flatMap(([file, source]) => {
      // Status badges, chart series and provider identities may retain their own colors.
      const matches = source.match(/\b(?:hover|focus|focus-visible|focus-within):(bg|text|border|ring)-(blue|cyan|teal|indigo)-\d+/g)
      return matches ? [`${file}: ${matches.join(', ')}`] : []
    })
    expect(violations).toEqual([])
  })

  it('uses the shared gray palette instead of a separate blue-gray surface palette', () => {
    const violations = Object.entries(sources).flatMap(([file, source]) => {
      const matches = source.match(/\b(bg|text|border|ring|divide|from|to|via|shadow)-slate-\d+/g)
      return matches ? [`${file}: ${matches.join(', ')}`] : []
    })
    expect(violations).toEqual([])
  })
})
