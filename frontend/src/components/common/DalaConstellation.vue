<template>
  <canvas ref="canvas" class="dala-constellation" aria-hidden="true" />
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import geometryUrl from '@/assets/constellation/constellation.bin?url'
import { constellationPose, createConstellationRenderer, decodeConstellation } from './constellationScene'
import type { ConstellationRenderer } from './constellationScene'

const props = defineProps<{ progress?: number; light?: boolean; align?: 'left' | 'center' }>()
const canvas = ref<HTMLCanvasElement>()
let dispose: (() => void) | undefined

onMounted(async () => {
  const element = canvas.value!
  const controller = new AbortController()
  const motion = matchMedia('(prefers-reduced-motion: reduce)')
  let renderer: ConstellationRenderer | null = null
  let geometry: ReturnType<typeof decodeConstellation> | undefined
  let frame: number | undefined
  let visible = true
  let contextAvailable = true
  let elapsed = 0
  let last: number | undefined
  let mobile = false
  let currentProgress = props.progress
  let progressVelocity = 0
  const pointer = { x: 0, y: 0 }

  function draw() {
    if (motion.matches || currentProgress === undefined || props.progress === undefined) {
      currentProgress = props.progress
      progressVelocity = 0
    } else {
      // Match the reference's spring-and-friction settling rather than snapping on scroll.
      progressVelocity = (progressVelocity + (props.progress - currentProgress) * .024) * .8
      currentProgress += progressVelocity
    }
    if (contextAvailable) renderer?.draw(elapsed, constellationPose(currentProgress, elapsed, mobile, motion.matches), props.light === true, pointer)
  }
  function stop() {
    if (frame !== undefined) cancelAnimationFrame(frame)
    frame = undefined
    last = undefined
  }
  function tick(timestamp: number) {
    if (last === undefined || timestamp - last >= 1000 / 30) {
      if (last !== undefined) elapsed += Math.min(timestamp - last, 100) / 1000
      last = timestamp
      draw()
    }
    frame = requestAnimationFrame(tick)
  }
  function sync() {
    stop()
    draw()
    if (renderer && visible && contextAvailable && !document.hidden && !motion.matches) frame = requestAnimationFrame(tick)
  }
  function resize() {
    const rect = element.getBoundingClientRect()
    mobile = rect.width < 600
    const centerX = props.align === 'left' && rect.width > 900 ? .34 : .5
    renderer?.resize(rect.width, rect.height, Math.min(devicePixelRatio || 1, 1.5), centerX)
    draw()
  }
  function onPointer(event: PointerEvent) {
    const rect = element.getBoundingClientRect()
    pointer.x = (event.clientX - rect.left) / rect.width * 2 - 1
    pointer.y = 1 - (event.clientY - rect.top) / rect.height * 2
  }
  function resetPointer() { pointer.x = 0; pointer.y = 0 }
  function contextLost(event: Event) { event.preventDefault(); contextAvailable = false; stop() }
  function contextRestored() {
    contextAvailable = true
    renderer?.dispose()
    renderer = geometry ? createConstellationRenderer(element, geometry) : null
    resize()
    sync()
  }
  const resizeObserver = new ResizeObserver(resize)
  const intersection = new IntersectionObserver(([entry]) => { visible = entry.isIntersecting; sync() })
  resizeObserver.observe(element)
  intersection.observe(element)
  motion.addEventListener('change', sync)
  document.addEventListener('visibilitychange', sync)
  window.addEventListener('pointermove', onPointer, { passive: true })
  window.addEventListener('blur', resetPointer)
  element.addEventListener('webglcontextlost', contextLost)
  element.addEventListener('webglcontextrestored', contextRestored)
  const stopWatch = watch(() => [props.light, props.progress], () => { if (frame === undefined) draw() })
  const stopAlignWatch = watch(() => props.align, resize)
  dispose = () => {
    controller.abort()
    stop()
    renderer?.dispose()
    resizeObserver.disconnect()
    intersection.disconnect()
    motion.removeEventListener('change', sync)
    document.removeEventListener('visibilitychange', sync)
    window.removeEventListener('pointermove', onPointer)
    window.removeEventListener('blur', resetPointer)
    element.removeEventListener('webglcontextlost', contextLost)
    element.removeEventListener('webglcontextrestored', contextRestored)
    stopWatch()
    stopAlignWatch()
  }
  try {
    const response = await fetch(geometryUrl, { signal: controller.signal })
    if (!response.ok) throw new Error(`Constellation geometry: ${response.status}`)
    geometry = decodeConstellation(await response.arrayBuffer())
    if (controller.signal.aborted) return
    renderer = createConstellationRenderer(element, geometry)
    resize()
    sync()
  } catch (error) {
    if (!controller.signal.aborted) console.warn('Constellation could not start', error)
  }
})
onUnmounted(() => dispose?.())
</script>

<style scoped>
.dala-constellation { display: block; width: 100%; height: 100%; pointer-events: none; contain: strict; }
</style>
