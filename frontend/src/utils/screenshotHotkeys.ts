import { ref } from 'vue'

import { ApplyScreenshotHotkeys } from '@/bridge'
import { useAppSettingsStore, useKernelApiStore } from '@/stores'

import { localHotkeyWarning } from './hotkeyConflict'

export const screenshotHotkeyWarning = ref('')

export const coreProxyForScreenshot = () => {
  const kernel = useKernelApiStore()
  if (!kernel.running) return ''
  const endpoint = kernel.getProxyEndpoint()
  if (!endpoint || endpoint.schema !== 'http' || !endpoint.port) return ''
  const host =
    endpoint.host && endpoint.host !== '0.0.0.0' && endpoint.host !== '::'
      ? endpoint.host
      : '127.0.0.1'
  const auth = endpoint.username
    ? `${encodeURIComponent(endpoint.username)}:${encodeURIComponent(endpoint.password || '')}@`
    : ''
  return `http://${auth}${host}:${endpoint.port}`
}

let timer: ReturnType<typeof setTimeout> | undefined
let seq = 0

export const scheduleScreenshotHotkeys = () => {
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => {
    void pushScreenshotHotkeys()
  }, 200)
}

export const pushScreenshotHotkeys = async () => {
  const app = useAppSettingsStore().app
  const shot = app.screenshotHotkey || ''
  const translate = app.screenshotTranslateHotkey || ''
  const local = localHotkeyWarning(shot, translate)
  const id = ++seq
  try {
    const result = await ApplyScreenshotHotkeys(
      shot,
      translate,
      app.deepseekApiKey || '',
      coreProxyForScreenshot(),
    )
    if (id !== seq) return
    const remote = String(result?.data || '')
    const notes = [local, remote].filter((item, index, all) => item && all.indexOf(item) === index)
    screenshotHotkeyWarning.value = notes.join('\n')
  } catch (error) {
    if (id !== seq) return
    screenshotHotkeyWarning.value = local || String(error || '')
  }
}
