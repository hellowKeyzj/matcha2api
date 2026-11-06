import { inject, provide, type InjectionKey } from 'vue'

const componentThemeKey: InjectionKey<string> = Symbol('component-theme')

// Vue context survives Teleport, unlike inherited DOM styles.
export function provideComponentTheme(theme: string) {
  provide(componentThemeKey, theme)
}

export function useComponentTheme() {
  return inject(componentThemeKey, '')
}
