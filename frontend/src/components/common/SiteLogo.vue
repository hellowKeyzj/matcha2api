<template>
  <img v-if="safeSrc" :src="safeSrc" :alt="alt" class="h-full w-full object-contain" />
  <svg
    v-else
    class="site-logo h-full w-full"
    viewBox="0 0 100 100"
    role="img"
    :aria-label="alt || undefined"
    :aria-hidden="alt ? undefined : true"
  >
    <use href="/logo.svg#matcha-mark" />
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { sanitizeUrl } from '@/utils/url'

const props = withDefaults(defineProps<{ src?: string; alt?: string }>(), {
  src: '',
  alt: 'Logo',
})

const safeSrc = computed(() => sanitizeUrl(props.src, { allowRelative: true, allowDataUrl: true }))
</script>

<style scoped>
.site-logo {
  color: var(--ui-logo);
}
</style>
