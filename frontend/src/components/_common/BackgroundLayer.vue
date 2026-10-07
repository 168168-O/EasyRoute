<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'

import { BackgroundVideoURL, WindowIsMinimised } from '@/bridge'
import { useAppSettingsStore } from '@/stores'
import { previewVideoURL } from '@/utils/backgroundVideo'

const appSettings = useAppSettingsStore()
const canvas = ref<HTMLCanvasElement>()
const video = ref<HTMLVideoElement>()
const paused = ref(false)
const videoBroken = ref(false)
const remoteURL = ref('')

const enabled = computed(() => appSettings.app.dynamicBackground !== false)
const showVideo = computed(
  () => enabled.value && !videoBroken.value && !!(previewVideoURL.value || appSettings.app.backgroundVideo),
)
const videoSrc = computed(() => {
  if (previewVideoURL.value) return previewVideoURL.value
  if (!appSettings.app.backgroundVideo || !remoteURL.value) return ''
  return remoteURL.value + '?v=' + encodeURIComponent(appSettings.app.backgroundVideo)
})

let timer = 0

const drawWaves = () => {
  const cv = canvas.value
  if (!cv) return
  const dpr = Math.min(window.devicePixelRatio || 1, 1.25)
  const W = window.innerWidth
  const H = window.innerHeight
  cv.width = Math.max(1, Math.floor(W * dpr))
  cv.height = Math.max(1, Math.floor(H * dpr))
  const context = cv.getContext('2d')
  if (!context) return
  const c = context
  c.setTransform(dpr, 0, 0, dpr, 0, 0)
  const css = getComputedStyle(document.documentElement)
  const C1 = css.getPropertyValue('--wave-rgb').trim() || '24, 140, 255'
  const C2 = css.getPropertyValue('--wave-2-rgb').trim() || '80, 215, 255'
  let seed = 20261008
  const rnd = () => (seed = (seed * 1664525 + 1013904223) % 4294967296) / 4294967296

  const g = c.createLinearGradient(0, 0, W * 0.25, H)
  g.addColorStop(0, '#010718')
  g.addColorStop(0.4, '#041d52')
  g.addColorStop(0.75, '#07307a')
  g.addColorStop(1, '#0a44a0')
  c.fillStyle = g
  c.fillRect(0, 0, W, H)

  const glow = (x: number, y: number, r: number, col: string, a: number) => {
    const rg = c.createRadialGradient(x, y, 0, x, y, r)
    rg.addColorStop(0, `rgba(${col},${a})`)
    rg.addColorStop(0.5, `rgba(${col},${a * 0.35})`)
    rg.addColorStop(1, `rgba(${col},0)`)
    c.fillStyle = rg
    c.fillRect(0, 0, W, H)
  }
  glow(W * 0.55, H * 1.05, W * 0.65, C2, 0.55)
  glow(W * 0.98, H * 0.45, W * 0.42, C1, 0.45)
  glow(W * 0.02, H * 0.55, W * 0.38, C1, 0.35)
  c.globalCompositeOperation = 'lighter'

  const yAt = (
    b: { y: number; tilt: number; amp: number; freq: number; ph: number; twist: number; sp: number },
    x: number,
    i: number,
  ) =>
    b.y * H -
    b.tilt * H * (x / W) +
    b.amp * H * Math.sin((x / W) * b.freq * 6.283 + b.ph + i * b.twist) +
    i * b.sp * (1 + 0.6 * Math.sin((x / W) * 3.1 + i * 0.07))

  const bundles = [
    { y: 0.58, tilt: 0.32, amp: 0.08, freq: 0.95, ph: 0, twist: 0.02, n: 36, sp: 1.2, col: C2, a: 0.16 },
    { y: 0.78, tilt: 0.22, amp: 0.07, freq: 0.7, ph: 2.2, twist: 0.025, n: 28, sp: 1.5, col: C1, a: 0.18 },
    { y: 1, tilt: 0.4, amp: 0.06, freq: 1.15, ph: 0.6, twist: 0.02, n: 32, sp: 1.2, col: C2, a: 0.17 },
  ]
  const stroke = (
    b: (typeof bundles)[number],
    i: number,
    w: number,
    col: string,
  ) => {
    c.beginPath()
    for (let x = -60; x <= W + 60; x += 10) {
      const y = yAt(b, x, i)
      if (x < -50) c.moveTo(x, y)
      else c.lineTo(x, y)
    }
    c.strokeStyle = col
    c.lineWidth = w
    c.stroke()
  }
  bundles.forEach((b) => {
    for (let i = 0; i < b.n; i++) {
      const k = 1 - Math.abs(i - b.n / 2) / (b.n / 2)
      stroke(b, i, 0.8 + k * 0.7, `rgba(${i % 3 ? b.col : '200,240,255'},${(b.a * (0.3 + k)).toFixed(3)})`)
    }
  })
  bundles.forEach((b) => {
    c.shadowColor = `rgba(${C2},1)`
    c.shadowBlur = 16
    stroke(b, b.n * 0.5, 2.2, 'rgba(255,255,255,.95)')
    c.shadowBlur = 0
  })
  for (let p = 0; p < 420; p++) {
    const b = bundles[p % bundles.length]!
    const x = W * (0.12 + 0.86 * rnd())
    const y = yAt(b, x, b.n * rnd()) + (rnd() - 0.5) * 40
    const r = 0.5 + rnd() * 1.4
    c.fillStyle = `rgba(${rnd() < 0.45 ? '160,230,255' : '255,255,255'},${(0.45 + rnd() * 0.5).toFixed(2)})`
    c.beginPath()
    c.arc(x, y, r, 0, 6.283)
    c.fill()
  }
  c.globalCompositeOperation = 'source-over'
}

const syncPlayback = async () => {
  let minimised = false
  try {
    minimised = await WindowIsMinimised()
  } catch {
    minimised = false
  }
  const hidden = document.hidden || minimised
  paused.value = hidden || !enabled.value
  const el = video.value
  if (!el) return
  if (paused.value) el.pause()
  else if (showVideo.value) el.play().catch(() => undefined)
}

const onResize = () => drawWaves()

onMounted(async () => {
  drawWaves()
  window.addEventListener('resize', onResize)
  document.addEventListener('visibilitychange', syncPlayback)
  timer = window.setInterval(syncPlayback, 1500)
  try {
    remoteURL.value = await BackgroundVideoURL()
  } catch {
    remoteURL.value = ''
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', onResize)
  document.removeEventListener('visibilitychange', syncPlayback)
  window.clearInterval(timer)
})

watch(
  () => appSettings.app.themePalette,
  () => drawWaves(),
)

watch(showVideo, () => syncPlayback())
watch(previewVideoURL, () => {
  videoBroken.value = false
})
</script>

<template>
  <div class="bg-layer" :class="{ 'is-paused': paused }" aria-hidden="true">
    <canvas ref="canvas" class="bg-canvas" />
    <video
      v-if="showVideo && videoSrc"
      ref="video"
      class="bg-video"
      autoplay
      muted
      loop
      playsinline
      :src="videoSrc"
      style="pointer-events: none"
      @error="videoBroken = true"
    />
    <div class="bg-dim" />
  </div>
</template>
