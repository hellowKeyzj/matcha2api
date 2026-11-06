<template>
  <div v-if="hasHomeContent" class="min-h-screen">
    <iframe v-if="isHomeContentUrl" :src="homeContent.trim()" class="h-screen w-full border-0" allowfullscreen />
    <!-- Custom HTML is an existing administrator-controlled site setting. -->
    <div v-else v-html="homeContent" />
  </div>
  <div v-else class="constellation-home" :class="{ 'is-light': lightStage, 'is-compact': compactHomeEnabled }" :data-testid="compactHomeEnabled ? 'compact-home' : 'full-home'">
    <header class="home-header">
      <router-link to="/home" class="home-brand" :aria-label="siteName">
        <img :src="siteLogo || '/logo.svg'" alt="" width="32" height="32" />
        <span>{{ siteName }}</span>
      </router-link>
      <nav class="home-nav" :aria-label="t('home.orbit.navigation')">
        <button v-if="!compactHomeEnabled" type="button" @click="scrollToDetails">{{ t('home.orbit.explore') }}</button>
        <router-link v-if="showModelPlazaEntry" to="/model-plaza">{{ t('nav.modelPlaza') }}</router-link>
      </nav>
      <div class="home-controls">
        <LocaleSwitcher class="home-locale" />
        <a v-if="docUrl" :href="docUrl" class="home-icon-control" :aria-label="t('home.viewDocs')" target="_blank" rel="noopener noreferrer"><Icon name="book" size="md" aria-hidden="true" /></a>
        <button type="button" class="home-icon-control home-theme" :aria-label="lightStage ? t('home.switchToDark') : t('home.switchToLight')" @click="toggleTheme"><Icon :name="lightStage ? 'moon' : 'sun'" size="md" /></button>
        <router-link :to="entryPath" class="home-login">{{ isAuthenticated ? t('home.dashboard') : t('home.login') }}<Icon name="arrowRight" size="sm" /></router-link>
      </div>
    </header>
    <main ref="story" class="home-story">
      <div class="home-scene" aria-hidden="true">
        <DalaConstellation :progress="compactHomeEnabled ? undefined : progress" :light="lightStage" />
      </div>
      <section class="home-hero home-panel" aria-labelledby="home-title">
        <div class="home-copy">
          <h1 id="home-title">{{ t('home.orbit.headline') }}<br />{{ t('home.orbit.headlineAccent') }}</h1>
          <div class="home-actions">
            <router-link :to="entryPath" class="home-cta">{{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}<Icon name="arrowRight" size="sm" /></router-link>
            <router-link v-if="showModelPlazaEntry" to="/model-plaza" class="home-secondary">{{ t('nav.modelPlaza') }}<Icon name="arrowRight" size="sm" /></router-link>
          </div>
        </div>
        <button v-if="!compactHomeEnabled" class="home-scroll" type="button" @click="scrollToDetails">{{ t('home.orbit.explore') }} <span aria-hidden="true">&#8595;</span></button>
      </section>
      <template v-if="!compactHomeEnabled">
        <section id="gateway-details" ref="details" class="home-panel home-panel-right" aria-labelledby="home-details-title">
          <div class="home-section-copy">
            <h2 id="home-details-title">{{ t('home.constellation.connectTitle') }}<br />{{ t('home.constellation.connectAccent') }}</h2>
            <p>{{ t('home.features.unifiedGatewayDesc') }}</p>
            <router-link v-if="showModelPlazaEntry" to="/model-plaza" class="home-secondary">{{ t('nav.modelPlaza') }}<Icon name="arrowRight" size="sm" /></router-link>
          </div>
        </section>
        <section class="home-panel home-panel-center" aria-labelledby="home-field-title">
          <div class="home-section-copy">
            <h2 id="home-field-title">{{ t('home.constellation.organizeTitle') }}<br />{{ t('home.constellation.organizeAccent') }}</h2>
            <p>{{ t('home.features.multiAccountDesc') }}</p>
          </div>
        </section>
        <section class="home-panel home-panel-left" aria-labelledby="home-bulb-title">
          <div class="home-section-copy">
            <h2 id="home-bulb-title">{{ t('home.constellation.buildTitle') }}<br />{{ t('home.constellation.buildAccent') }}</h2>
            <p>{{ t('home.features.balanceQuotaDesc') }}</p>
            <router-link :to="entryPath" class="home-cta">{{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}<Icon name="arrowRight" size="sm" /></router-link>
          </div>
        </section>
      </template>
    </main>
    <footer class="home-footer"><span>&copy; {{ currentYear }} {{ siteName }}</span><a :href="githubUrl" target="_blank" rel="noopener noreferrer">GitHub &#8599;</a></footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import DalaConstellation from '@/components/common/DalaConstellation.vue'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const isHomeContentUrl = computed(() => /^https?:\/\//.test(homeContent.value.trim()))
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const isAuthenticated = computed(() => authStore.isAuthenticated)
const entryPath = computed(() => isAuthenticated.value ? (authStore.isAdmin ? '/admin/dashboard' : '/dashboard') : '/login')
const showModelPlazaEntry = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza)
  && (isAuthenticated.value || appStore.cachedPublicSettings?.model_plaza_require_auth !== true))
const { isDark, toggleTheme } = useTheme()
const lightStage = computed(() => !isDark.value)
const details = ref<HTMLElement>()
const story = ref<HTMLElement>()
const progress = ref(0)
const currentYear = new Date().getFullYear()
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

function scrollToDetails() {
  details.value?.scrollIntoView({ behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth' })
}
function updateProgress() {
  if (!story.value) return
  progress.value = Math.max(0, -story.value.getBoundingClientRect().top / window.innerHeight)
}
onMounted(() => {
  window.addEventListener('scroll', updateProgress, { passive: true })
  window.addEventListener('resize', updateProgress, { passive: true })
  updateProgress()
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings()
})
onUnmounted(() => {
  window.removeEventListener('scroll', updateProgress)
  window.removeEventListener('resize', updateProgress)
})
</script>

<style scoped>
.constellation-home {
  --home-bg: #000; --home-ink: #fff; --home-muted: #9a9a9a; --home-accent: #8052ff;
  background: var(--home-bg); color: var(--home-ink); min-height: 100svh; overflow-x: clip;
  font-family: 'PP Neue Montreal', 'Helvetica Neue', 'PingFang SC', 'Microsoft YaHei', sans-serif;
}
.constellation-home.is-light { --home-bg: var(--ui-canvas); --home-ink: #1d1828; --home-muted: #625a70; }
.home-header { position: absolute; top: 0; left: 0; right: 0; z-index: 5; display: flex; align-items: center; gap: 36px; padding: 28px 4vw; }
.home-brand { display: inline-flex; min-width: 0; align-items: center; gap: 10px; font-size: 20px; font-weight: 500; letter-spacing: -0.04em; margin-right: auto; }
.home-brand img { border-radius: 8px; flex-shrink: 0; }
.home-brand span { max-width: 220px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.home-nav { display: flex; align-items: center; gap: 32px; color: var(--home-muted); font-size: 12px; }
.home-nav a:hover, .home-nav button:hover { color: var(--home-ink); }
.home-controls { display: flex; align-items: center; gap: 14px; }
.home-icon-control { width: 34px; height: 34px; flex-shrink: 0; display: grid; place-items: center; color: var(--home-muted); border-radius: 50%; }
.home-icon-control:hover { color: var(--home-ink); }
.home-login { display: inline-flex; gap: 20px; align-items: center; font-size: 12px; padding: 12px 20px; border-radius: 24px; background: var(--home-accent); color: #fff; }
.home-locale :deep(button) { color: var(--home-muted); }
.home-locale :deep(button:hover) { background: #8052ff18; }
.home-locale :deep(.absolute) { background: var(--home-bg); border-color: #8052ff50; }
.home-story { position: relative; }
.home-scene { position: sticky; top: 0; width: 100%; height: 100svh; margin-bottom: -100svh; pointer-events: none; }
.home-panel { position: relative; min-height: 100svh; display: flex; align-items: center; padding: 100px max(7vw, calc((100vw - 1280px) / 2)); pointer-events: none; }
.home-panel a, .home-panel button { pointer-events: auto; }
.home-copy { width: 53%; max-width: 680px; position: relative; animation: home-arrive 1s both; }
.home-copy h1 { font-size: clamp(48px, 6.6vw, 104px); line-height: 1.14; letter-spacing: -.055em; font-weight: 400; text-wrap: balance; }
.home-actions { display: flex; align-items: center; gap: 30px; margin-top: 34px; }
.home-cta { display: inline-flex; align-items: center; gap: 24px; min-height: 47px; padding: 0 22px; border-radius: 24px; font-size: 13px; font-weight: 400; color: #fff; background: var(--home-accent); transition: background 180ms; }
.home-cta:hover, .home-login:hover { background: #916aff; }
.home-secondary { display: inline-flex; align-items: center; gap: 14px; color: var(--home-ink); font-size: 12px; }
.home-scroll { position: absolute; bottom: 36px; left: 7vw; display: flex; align-items: center; gap: 24px; color: var(--home-muted); font-size: 10px; }
.home-section-copy { width: 43%; max-width: 510px; }
.home-section-copy h2 { font-size: clamp(36px, 4.6vw, 72px); line-height: 1.18; letter-spacing: -.045em; font-weight: 400; text-wrap: balance; }
.home-section-copy > p { font-size: 17px; font-weight: 300; line-height: 1.8; color: var(--home-muted); margin-top: 28px; max-width: 390px; }
.home-section-copy > a { margin-top: 30px; }
.home-panel-right { justify-content: flex-end; }
.home-panel-center { justify-content: center; text-align: center; }
.home-panel-center .home-section-copy { width: 60%; max-width: 700px; }
.home-panel-center .home-section-copy > p { margin-inline: auto; }
.home-panel-center .home-section-copy h2 { text-shadow: 0 1px 25px var(--home-bg); }
.home-panel-left .home-section-copy { align-self: flex-start; padding-top: 13vh; }
.home-footer { position: relative; display: flex; gap: 20px; align-items: center; justify-content: space-between; padding: 27px 4vw; background: var(--home-bg); color: var(--home-muted); font-size: 10px; }
.home-footer > span:first-child { overflow-wrap: anywhere; }
.is-compact .home-scene { left: 30%; width: 70%; }
.constellation-home a:focus-visible, .constellation-home button:focus-visible { outline: 2px solid var(--home-accent); outline-offset: 6px; }
@keyframes home-arrive { from { opacity: 0; transform: translateY(15px); } to { opacity: 1; transform: translateY(0); } }
@media (max-width: 900px) { .home-nav { gap: 18px; } .home-header { gap: 20px; } .home-copy h1 { font-size: 64px; } }
@media (max-width: 700px) {
  .home-header { padding: 20px; gap: 10px; } .home-brand { font-size: 17px; } .home-brand span { max-width: 125px; }
  .home-nav { display: none; } .home-controls { gap: 6px; } .home-login { padding: 10px 14px; gap: 10px; } .home-brand img { width: 27px; height: 27px; }
  .home-panel { min-height: 100svh; padding: 100px 26px 55px; align-items: flex-end; }
  .home-copy { width: 100%; padding-bottom: 20px; } .home-copy h1 { font-size: clamp(40px, 10vw, 60px); }
  .home-actions { margin-top: 26px; gap: 24px; }
  .home-scroll { bottom: 24px; left: 26px; }
  .home-section-copy, .home-panel-center .home-section-copy { width: 100%; max-width: none; padding-bottom: 30px; }
  .home-section-copy h2 { font-size: 40px; } .home-section-copy > p { font-size: 14px; }
  .home-panel-center { align-items: center; } .home-panel-left .home-section-copy { align-self: flex-end; padding-top: 0; }
  .home-scene, .is-compact .home-scene { left: 0; width: 100%; height: 65svh; top: 0; margin-bottom: -65svh; }
  .home-footer { padding: 22px 20px; font-size: 9px; }
}
@media (max-width: 360px) { .home-brand span { max-width: 85px; } .home-header { padding-inline: 14px; } }
@media (prefers-reduced-motion: reduce) { .home-copy { animation: none; } }
</style>
