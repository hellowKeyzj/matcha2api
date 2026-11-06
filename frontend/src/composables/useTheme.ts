import { readonly, ref } from 'vue'

const isDark = ref(false)
const theme = { isDark: readonly(isDark), toggleTheme }

function applyTheme(dark: boolean) {
  isDark.value = dark
  document.documentElement.classList.toggle('dark', dark)
}

// Bootstrap resolves the preference before mount; components only consume it.
export function initializeTheme() {
  const saved = localStorage.getItem('theme')
  applyTheme(saved === 'dark' || (!saved && window.matchMedia('(prefers-color-scheme: dark)').matches))
}

function toggleTheme() {
  applyTheme(!isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

export function useTheme() {
  return theme
}
