<template>
  <canvas ref="canvas" class="dala-constellation" aria-hidden="true" />
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'

const canvas = ref<HTMLCanvasElement>()
let dispose: (() => void) | undefined

onMounted(() => {
  const element = canvas.value!
  const context = element.getContext('2d')
  if (!context) return

  const motion = matchMedia('(prefers-reduced-motion: reduce)')
  const colors = ['#8052ff', '#ffb829', '#15846e', '#cf78e8', '#7299ff', '#e8c5ed']
  // Stable sampling keeps the constellation identical across mounts and resizes.
  let seed = 37
  const random = () => {
    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0
    return seed / 4294967296
  }
  const particles = Array.from({ length: 3600 }, (_, index) => {
    const y = 1 - 2 * (index + 0.5) / 3600
    const angle = index * 2.3999632297
    const radius = Math.sqrt(1 - y * y)
    const x = Math.cos(angle) * radius
    const z = Math.sin(angle) * radius
    // Two folded hemispheres; the centre cleft and surface folds remain 3D.
    const fold = 1 + 0.065 * Math.sin(angle * 7 + y * 19) * Math.cos(y * 23)
    const brain = [x * (0.94 + 0.12 * Math.abs(x)) * fold, y * 0.8 * fold, z * 0.73 * fold]
    brain[1] += 0.11 * Math.exp(-x * x * 60)
    const bulbRadius = y < 0.35 ? Math.sqrt(Math.max(0, 1 - ((y + 0.2) / 0.85) ** 2)) * 0.82 : 0.24 + 0.025 * Math.sin(y * 65)
    const bulb = [Math.cos(angle) * bulbRadius, y, Math.sin(angle) * bulbRadius]
    const color = y < -0.45 ? 2 : y < 0.1 ? 0 : y < 0.55 ? 3 : 1
    return { brain, bulb, phase: random() * Math.PI * 2, size: 0.6 + random() * 0.95, color }
  })
  const ambient = Array.from({ length: 42 }, () => ({ x: random(), y: random(), phase: random() * Math.PI * 2, size: 2 + random() * 6, color: Math.floor(random() * colors.length) }))
  let width = 0
  let height = 0
  let frame: number | undefined
  let visible = true
  let elapsed = 0
  let last = 0

  function triangle(x: number, y: number, size: number, angle: number) {
    context!.beginPath()
    for (let corner = 0; corner < 3; corner++) {
      const a = angle + corner * Math.PI * 2 / 3
      const px = x + Math.cos(a) * size
      const py = y + Math.sin(a) * size
      if (corner === 0) context!.moveTo(px, py)
      else context!.lineTo(px, py)
    }
    context!.closePath()
    context!.stroke()
  }

  function draw(time: number) {
    context!.clearRect(0, 0, width, height)
    const scale = Math.min(width * 0.43, height * 0.43)
    const rotation = time * 0.13 + 0.3
    const cos = Math.cos(rotation)
    const sin = Math.sin(rotation)
    // Hold each silhouette, then smoothly morph rather than swapping point clouds.
    const cycle = time % 24
    const blend = (start: number) => {
      const p = Math.max(0, Math.min(1, (cycle - start) / 3))
      return p * p * (3 - 2 * p)
    }
    const morph = blend(8) - blend(20)
    context!.lineWidth = 0.65
    for (let i = 0; i < particles.length; i++) {
      if (width < 500 && i % 2) continue
      const particle = particles[i]
      const x = particle.brain[0] * (1 - morph) + particle.bulb[0] * morph
      const y = particle.brain[1] * (1 - morph) + particle.bulb[1] * morph
      const z = particle.brain[2] * (1 - morph) + particle.bulb[2] * morph
      const rx = x * cos + z * sin
      const depth = z * cos - x * sin
      const perspective = 2.8 / (2.8 - depth)
      const flutter = Math.sin(time * 0.7 + particle.phase) * 0.012
      context!.globalAlpha = 0.25 + (depth + 1) * 0.3
      context!.strokeStyle = colors[particle.color]
      triangle(width * 0.5 + (rx + flutter) * scale * perspective, height * 0.5 + y * scale * perspective, particle.size * perspective, particle.phase + time * 0.1)
    }
    for (const particle of ambient) {
      context!.globalAlpha = 0.15 + 0.16 * (1 + Math.sin(time * 0.4 + particle.phase)) / 2
      context!.strokeStyle = colors[particle.color]
      triangle(particle.x * width + Math.sin(time * 0.2 + particle.phase) * 12, particle.y * height + Math.cos(time * 0.15 + particle.phase) * 10, particle.size, particle.phase + time * 0.04)
    }
    context!.globalAlpha = 1
  }

  function stop() {
    if (frame !== undefined) cancelAnimationFrame(frame)
    frame = undefined
    last = 0
  }
  function tick(timestamp: number) {
    frame = undefined
    if (!last || timestamp - last >= 1000 / 30) {
      if (last) elapsed += Math.min(timestamp - last, 100) / 1000
      last = timestamp
      draw(elapsed)
    }
    frame = requestAnimationFrame(tick)
  }
  function sync() {
    stop()
    draw(elapsed)
    if (visible && !document.hidden && !motion.matches) frame = requestAnimationFrame(tick)
  }
  function resize() {
    const rect = element.getBoundingClientRect()
    width = rect.width
    height = rect.height
    const ratio = Math.min(devicePixelRatio || 1, 2)
    element.width = Math.round(width * ratio)
    element.height = Math.round(height * ratio)
    context!.setTransform(ratio, 0, 0, ratio, 0, 0)
    draw(elapsed)
  }
  const resizeObserver = new ResizeObserver(resize)
  const intersection = new IntersectionObserver(([entry]) => {
    visible = entry.isIntersecting
    sync()
  })
  resizeObserver.observe(element)
  intersection.observe(element)
  motion.addEventListener('change', sync)
  document.addEventListener('visibilitychange', sync)
  resize()
  sync()
  dispose = () => {
    stop()
    resizeObserver.disconnect()
    intersection.disconnect()
    motion.removeEventListener('change', sync)
    document.removeEventListener('visibilitychange', sync)
  }
})
onUnmounted(() => dispose?.())
</script>

<style scoped>
.dala-constellation {
  display: block;
  width: 100%;
  height: 100%;
  pointer-events: none;
  contain: strict;
}
</style>
