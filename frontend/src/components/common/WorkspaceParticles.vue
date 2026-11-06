<template>
  <div class="workspace-particles" :class="{ 'is-paused': paused }" aria-hidden="true">
    <svg v-for="layer in 3" :key="layer" viewBox="0 0 1200 900" preserveAspectRatio="xMidYMid slice">
      <circle
        v-for="point in 24"
        :key="point"
        :cx="(point * 173 + layer * 97) % 1200"
        :cy="(point * 257 + layer * 131) % 900"
        :r="point % 3 === 0 ? 1.8 : 1.1"
      />
    </svg>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'

const paused = ref(false)
const syncVisibility = () => { paused.value = document.hidden }
onMounted(() => {
  syncVisibility()
  document.addEventListener('visibilitychange', syncVisibility)
})
onUnmounted(() => document.removeEventListener('visibilitychange', syncVisibility))
</script>

<style>
.workspace-particles {
  position: fixed;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
  contain: strict;
  background: radial-gradient(ellipse at 20% 20%, #bda2ff0a, transparent 65%);
}
.workspace-particles svg {
  position: absolute;
  inset: -24px;
  width: calc(100% + 48px);
  height: calc(100% + 48px);
  fill: #827991;
  opacity: 0.22;
  animation: dust-drift 32s ease-in-out infinite alternate;
}
.workspace-particles svg:nth-child(2) {
  fill: #a98bd2;
  opacity: 0.3;
  animation-duration: 43s;
  animation-direction: alternate-reverse;
}
.workspace-particles svg:nth-child(3) {
  fill: #9a91aa;
  animation-duration: 51s;
}
.dark .workspace-particles svg { fill: #b8a1e0; opacity: 0.32; }
.dark .workspace-particles svg:nth-child(2) { fill: #739eaa; opacity: 0.28; }
.dark .workspace-particles svg:nth-child(3) { fill: #b9a580; opacity: 0.24; }
.workspace-particles.is-paused svg { animation-play-state: paused; }
@keyframes dust-drift {
  from { transform: translate3d(-12px, 8px, 0); }
  to { transform: translate3d(18px, -18px, 0); }
}
@media (prefers-reduced-motion: reduce) {
  .workspace-particles svg { animation: none; }
}
@media (max-width: 640px) {
  .workspace-particles svg:nth-child(3) { display: none; }
}
</style>
