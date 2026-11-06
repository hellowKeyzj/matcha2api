<template>
  <div
    :class="showChrome ? ['app-layout min-h-screen bg-gray-50 dark:bg-dark-950', route.meta.appLayoutClass] : undefined"
  >
    <WorkspaceParticles v-if="showChrome" />

    <!-- Sidebar -->
    <AppSidebar v-if="showChrome" />

    <!-- Main Content Area -->
    <div
      :class="showChrome ? [
        'app-layout-content relative min-h-screen transition-all duration-300',
        sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64'
      ] : undefined"
    >
      <!-- Header -->
      <AppHeader v-if="showChrome" />

      <!-- Keep the outlet's ancestry stable when a query toggles standalone mode. -->
      <div
        :role="showChrome ? 'main' : undefined"
        :class="showChrome ? 'app-layout-main p-4 md:p-6 lg:p-8' : undefined"
      >
        <RouterView />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import { useAppStore } from '@/stores'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import WorkspaceParticles from '@/components/common/WorkspaceParticles.vue'

const appStore = useAppStore()
const route = useRoute()
const showChrome = computed(() => !route.meta.hideAppChrome?.(route))
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
</script>
