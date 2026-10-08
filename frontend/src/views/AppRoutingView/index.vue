<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { ListProcesses, PickFile } from '@/bridge'
import { useAppSettingsStore, useKernelApiStore } from '@/stores'
import { confirm, message, sampleID } from '@/utils'
import {
  DOUYIN_PROCESSES,
  WECHAT_PROCESSES,
  isPinnedExe,
  normalizePinnedRoutes,
  type AppRouteMode,
  type RoutedProgram,
} from '@/utils/appRouting'

const { t } = useI18n()
const appSettings = useAppSettingsStore()
const kernelApi = useKernelApiStore()

const keyword = ref('')
const pickerOpen = ref(false)
const processes = ref<{ name: string; exe: string }[]>([])
const loadingProcesses = ref(false)

const modeOptions = [
  { label: 'routing.proxy', value: 'proxy' },
  { label: 'routing.direct', value: 'direct' },
]

const pinned: { id: 'wechat' | 'douyin'; name: string; color: string; exes: string[] }[] = [
  {
    id: 'wechat',
    name: '微信',
    color: '#2aae67',
    exes: WECHAT_PROCESSES,
  },
  {
    id: 'douyin',
    name: '抖音',
    color: '#111',
    exes: DOUYIN_PROCESSES,
  },
]

const programs = computed(() => appSettings.app.appPrograms || [])

const visiblePrograms = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  if (!q) return programs.value
  return programs.value.filter(
    (item) => item.name.toLowerCase().includes(q) || item.exe.toLowerCase().includes(q),
  )
})

const visiblePinned = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  if (!q) return pinned
  return pinned.filter(
    (item) => item.name.includes(q) || item.exes.some((exe) => exe.toLowerCase().includes(q)),
  )
})

const knownNames: Record<string, string> = {
  'chrome.exe': 'Google Chrome',
  'msedge.exe': 'Microsoft Edge',
  'firefox.exe': 'Firefox',
  'telegram.exe': 'Telegram',
  'discord.exe': 'Discord',
}

const iconColor = (exe: string) => {
  const key = exe.toLowerCase()
  if (key.includes('chrome')) return '#e8a317'
  if (key.includes('edge')) return '#2f7de1'
  if (key.includes('telegram')) return '#2aa5e0'
  if (key.includes('discord')) return '#5865f2'
  return '#3d6dff'
}

const summary = (exes: string[]) => {
  const head = exes.slice(0, 3).join(' · ')
  return `${head} ${t('routing.processes', [exes.length])}`
}

const addProgram = (name: string, exe: string) => {
  const file = exe.trim()
  if (!file) return
  if (isPinnedExe(file)) {
    message.warn('routing.pinnedExists')
    return
  }
  if (programs.value.some((item) => item.exe.toLowerCase() === file.toLowerCase())) {
    message.info('routing.duplicate')
    return
  }
  const next: RoutedProgram = {
    id: sampleID(),
    name: knownNames[file.toLowerCase()] || name || file.replace(/\.exe$/i, ''),
    exe: file,
    mode: 'direct',
  }
  appSettings.app.appPrograms = [...programs.value, next]
  pickerOpen.value = false
}

const pinnedMode = (id: 'wechat' | 'douyin'): AppRouteMode =>
  normalizePinnedRoutes(appSettings.app.pinnedRoutes)[id]

const setPinnedMode = async (id: 'wechat' | 'douyin', mode: string | number | boolean | undefined) => {
  const next: AppRouteMode = mode === 'proxy' ? 'proxy' : 'direct'
  const current = normalizePinnedRoutes(appSettings.app.pinnedRoutes)
  if (current[id] === next) return
  if (next === 'proxy') {
    try {
      await confirm('routing.proxyConfirmTitle', 'routing.proxyConfirmBody')
    } catch {
      return
    }
  }
  appSettings.app.pinnedRoutes = { ...current, [id]: next }
}

const setMode = (id: string, mode: string | number | boolean | undefined) => {
  appSettings.app.appPrograms = programs.value.map((item) =>
    item.id === id ? { ...item, mode: mode === 'proxy' ? 'proxy' : 'direct' } : item,
  )
}

const removeProgram = (id: string) => {
  appSettings.app.appPrograms = programs.value.filter((item) => item.id !== id)
}

const loadProcesses = async () => {
  loadingProcesses.value = true
  pickerOpen.value = true
  try {
    processes.value = await ListProcesses()
  } catch (error) {
    message.error(error)
    processes.value = []
  } finally {
    loadingProcesses.value = false
  }
}

const browseExe = async () => {
  if ((window as Window & { __DHAGN_PREVIEW__?: boolean }).__DHAGN_PREVIEW__) {
    const exe = window.prompt('输入 exe 名称，例如 chrome.exe', 'chrome.exe')
    if (exe) addProgram(exe, exe)
    return
  }
  try {
    const path = await PickFile('选择程序', '*.exe')
    const exe = path.split(/[/\\]/).pop() || path
    addProgram(exe, exe)
  } catch (error) {
    if (String(error) !== 'cancelled') message.error(error)
  }
}

onMounted(() => {
  if (
    (window as Window & { __DHAGN_PREVIEW__?: boolean }).__DHAGN_PREVIEW__ &&
    programs.value.length === 0
  ) {
    appSettings.app.appPrograms = [
      { id: 'chrome', name: 'Google Chrome', exe: 'chrome.exe', mode: 'proxy' },
      { id: 'telegram', name: 'Telegram', exe: 'Telegram.exe', mode: 'proxy' },
      { id: 'discord', name: 'Discord', exe: 'Discord.exe', mode: 'proxy' },
      { id: 'edge', name: 'Microsoft Edge', exe: 'msedge.exe', mode: 'direct' },
    ]
  }
})
</script>

<template>
  <div>
    <div class="rt-head">
      <div>
        <div class="rt-title">
          {{ t('router.appRouting') }}
          <span class="status" :class="{ off: !kernelApi.running }">
            {{ kernelApi.running ? t('routing.connected') : t('routing.disconnected') }}
          </span>
        </div>
        <div class="rt-sub">
          <span class="step"><i>1</i>{{ t('routing.stepConnect') }}</span>
          <span class="step"><i>2</i>{{ t('routing.stepChoose') }}</span>
        </div>
      </div>
      <div class="rt-tools">
        <input v-model="keyword" class="search" :placeholder="t('routing.search')" />
        <button type="button" class="btn add" @click="loadProcesses">+ {{ t('routing.add') }}</button>
      </div>
    </div>

    <div v-if="pickerOpen" class="picker">
      <div class="flex gap-8 mb-8">
        <button type="button" class="ghost-btn" @click="browseExe">{{ t('routing.browse') }}</button>
        <button type="button" class="ghost-btn" @click="pickerOpen = false">{{ t('common.close') }}</button>
      </div>
      <div v-if="loadingProcesses">正在读取正在运行的程序…</div>
      <button
        v-for="item in processes"
        :key="item.exe"
        type="button"
        class="row-btn"
        @click="addProgram(item.name, item.exe)"
      >
        <b>{{ item.name }}</b>
        <span>{{ item.exe }}</span>
      </button>
    </div>

    <div class="group">
      <b>{{ t('routing.pinned') }}</b>
      <span class="pin-badge">{{ t('routing.pinnedBadge') }}</span>
      <span>{{ t('routing.pinnedHint') }}</span>
    </div>
    <div class="pinned">
      <div v-for="item in visiblePinned" :key="item.id" class="app-row">
        <div class="app-ico" :style="{ background: item.color }">{{ item.name.slice(0, 1) }}</div>
        <div class="app-meta">
          <div class="app-name">{{ item.name }}</div>
          <div class="app-exe">{{ summary(item.exes) }}</div>
        </div>
        <span class="lock-note">{{ t('routing.locked') }}</span>
        <Radio
          :model-value="pinnedMode(item.id)"
          tone="blue"
          :options="modeOptions"
          @update:model-value="setPinnedMode(item.id, $event)"
        />
      </div>
    </div>

    <div class="group">
      <b>{{ t('routing.domesticTitle') }}</b>
      <span>{{ t('routing.domesticHint') }}</span>
    </div>
    <div class="app-row">
      <div class="app-meta">
        <div class="app-name">{{ t('routing.domesticDirect') }}</div>
        <div class="app-exe">{{ t('routing.domesticDirectHint') }}</div>
      </div>
      <Switch v-model="appSettings.app.domesticDirect" />
    </div>

    <div class="group">
      <b>{{ t('routing.mine') }}</b>
      <span>{{ programs.length }} · {{ t('routing.mineHint') }}</span>
    </div>
    <div v-if="!visiblePrograms.length" class="app-exe px-4">{{ t('routing.empty') }}</div>
    <div v-for="item in visiblePrograms" :key="item.id" class="app-row">
      <div class="app-ico" :style="{ background: iconColor(item.exe) }">{{ item.name.slice(0, 1) }}</div>
      <div class="app-meta">
        <div class="app-name">{{ item.name }}</div>
        <div class="app-exe">{{ item.exe }}</div>
      </div>
      <Radio
        :model-value="item.mode"
        tone="blue"
        :options="modeOptions"
        @update:model-value="setMode(item.id, $event)"
      />
      <button type="button" class="del" :aria-label="t('common.delete')" @click="removeProgram(item.id)">
        ⌫
      </button>
    </div>

    <div class="note">
      <div>
        {{ t('routing.note') }}
        <b>{{ t('routing.noteEm') }}</b>
      </div>
      <button type="button" class="btn add" @click="loadProcesses">+ {{ t('routing.add') }}</button>
    </div>
  </div>
</template>
