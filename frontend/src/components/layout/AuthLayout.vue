<template>
  <div
    class="auth-layout relative flex min-h-screen items-center justify-center overflow-hidden p-4"
    :class="{ 'auth-layout-animated': animatedBackground }"
  >
    <!-- Background -->
    <div
      class="auth-background absolute inset-0"
    ></div>
    <div v-if="animatedBackground" class="auth-particles" aria-hidden="true">
      <DalaConstellation :light="!isDark" align="left" />
    </div>
    <button
      v-if="animatedBackground"
      type="button"
      class="auth-theme-toggle"
      :aria-label="isDark ? t('nav.lightMode') : t('nav.darkMode')"
      @click="toggleTheme"
    >
      <Icon :name="isDark ? 'sun' : 'moon'" size="md" aria-hidden="true" />
    </button>

    <!-- Decorative Elements -->
    <div v-if="!animatedBackground" class="auth-decoration pointer-events-none absolute inset-0 overflow-hidden">
      <!-- Gradient Orbs -->
      <div
        class="absolute -right-40 -top-40 h-80 w-80 rounded-full bg-primary-400/20 blur-3xl"
      ></div>
      <div
        class="absolute -bottom-40 -left-40 h-80 w-80 rounded-full bg-primary-500/15 blur-3xl"
      ></div>
      <div
        class="absolute left-1/2 top-1/2 h-96 w-96 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary-300/10 blur-3xl"
      ></div>

      <!-- Grid Pattern -->
      <div
        class="absolute inset-0 bg-[linear-gradient(rgba(128,82,255,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(128,82,255,0.03)_1px,transparent_1px)] bg-[size:64px_64px]"
      ></div>
    </div>

    <!-- Content Container -->
    <div class="auth-content relative z-10 w-full max-w-md">
      <!-- Logo/Brand -->
      <div class="auth-brand mb-8 text-center">
        <!-- Custom Logo or Default Logo -->
        <template v-if="settingsLoaded">
          <div
            class="auth-logo mb-4 inline-flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl"
          >
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <h1 class="text-gradient mb-2 text-3xl font-bold">
            {{ siteName }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ siteSubtitle }}
          </p>
        </template>
      </div>

      <!-- Card Container -->
      <div class="auth-card card-glass rounded-2xl p-8 shadow-glass">
        <slot />
      </div>

      <!-- Footer Links -->
      <div class="auth-footer mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <!-- Copyright -->
      <div class="mt-8 text-center text-xs text-gray-400 dark:text-dark-500">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import DalaConstellation from '@/components/common/DalaConstellation.vue'
import Icon from '@/components/icons/Icon.vue'
import { useTheme } from '@/composables/useTheme'

defineProps<{ animatedBackground?: boolean }>()

const appStore = useAppStore()
const { t } = useI18n()
const { isDark, toggleTheme } = useTheme()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.text-gradient {
  color: var(--ui-ink);
  font-weight: 400;
}
.auth-background {
  background: var(--ui-canvas);
}
.auth-decoration {
  opacity: 0.25;
}
.auth-particles {
  position: fixed;
  inset: 0;
  pointer-events: none;
  background: var(--auth-background);
}
.auth-layout-animated {
  --auth-background: var(--ui-canvas);
  --auth-panel: rgb(var(--palette-surface) / 0.85);
  --auth-input: var(--ui-chrome);
  --auth-shadow: #3024470a;
  --auth-particle-opacity: 1;
  color: var(--ui-ink);
  color-scheme: light;
  padding: 48px clamp(32px, 7vw, 120px);
  justify-content: flex-end;
}
.auth-theme-toggle {
  position: absolute;
  z-index: 20;
  top: 20px;
  right: 24px;
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: transparent;
  color: var(--ui-muted);
}
.auth-theme-toggle:hover {
  color: var(--ui-ink);
}
.auth-theme-toggle:focus-visible {
  outline: 2px solid var(--ui-accent);
  outline-offset: 3px;
}

.auth-layout-animated .auth-particles :deep(canvas) {
  opacity: var(--auth-particle-opacity);
}
.auth-layout-animated .auth-card {
  background: var(--auth-panel);
  border: 1px solid var(--ui-line);
  border-radius: 20px;
  box-shadow: 0 24px 80px var(--auth-shadow);
  backdrop-filter: blur(16px);
}
.auth-layout-animated .auth-content {
  max-width: 400px;
}
.auth-layout-animated .auth-brand {
  display: grid;
  grid-template-columns: 40px 1fr;
  column-gap: 12px;
  text-align: left;
  align-items: center;
  margin-bottom: 24px;
}
.auth-layout-animated .auth-logo {
  grid-row: span 2;
  width: 40px;
  height: 40px;
  margin: 0;
  border-radius: 12px;
}
.auth-layout-animated .text-gradient {
  color: var(--ui-ink);
  font-size: 22px;
  line-height: 1.2;
  margin: 0;
}
.auth-layout-animated .auth-brand p,
.auth-layout-animated .auth-content > .text-xs {
  color: var(--ui-muted);
}
.auth-layout-animated .auth-card :deep(.input) {
  min-height: 46px;
  border-radius: 10px;
  background: var(--auth-input);
}
.auth-layout-animated .auth-card :deep(.btn) {
  min-height: 44px;
  border-radius: 10px;
}
.auth-layout-animated .auth-footer :deep(p) {
  color: var(--ui-muted);
}
.auth-layout-animated .auth-footer :deep(a) {
  color: var(--ui-accent);
}
.auth-layout-animated .auth-footer :deep(a:hover) {
  text-decoration: underline;
}
@media (max-width: 900px) {
  .auth-layout-animated {
    justify-content: center;
    padding: 80px 20px 32px;
  }
  .auth-layout-animated .auth-particles :deep(canvas) {
    opacity: calc(var(--auth-particle-opacity) * 0.8);
  }
}
</style>

<style>
.dark .auth-layout-animated {
  --ui-surface: #141419;
  --ui-line: #34333e;
  --ui-ink: #f5f3fa;
  --ui-muted: #a3a0b2;
  --ui-selected: #252034;
  --ui-accent: #bda2ff;
  --auth-background: #000;
  --auth-panel: #141419d6;
  --auth-input: #0d0d12;
  --auth-shadow: #0006;
  --auth-particle-opacity: 1;
  color-scheme: dark;
}
</style>
