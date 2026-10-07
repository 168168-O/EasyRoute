<script lang="ts" setup>
import { computed, ref } from 'vue'

import { BrowserOpenURL, CopyFile, MakeDir, OpenDir, PickFile, RemoveFile } from '@/bridge'
import { ColorOptions, DefaultFontFamily, LocalesFilePath, ThemeOptions } from '@/constant/app'
import { Color } from '@/enums/app'
import routes from '@/router/routes'
import { useAppSettingsStore, useAppStore } from '@/stores'
import { APP_LOCALES_URL, message } from '@/utils'
import { previewVideoURL } from '@/utils/backgroundVideo'

const palettes = [
  { id: 'gold-sea', name: '鎏金深海', rgb: '212,178,106' },
  { id: 'glacier-aurora', name: '冰川极光', rgb: '127,227,212' },
  { id: 'jade-celadon', name: '墨玉青瓷', rgb: '140,199,176' },
  { id: 'night-amethyst', name: '暗夜紫晶', rgb: '182,156,255' },
  { id: 'rose-champagne', name: '玫瑰香槟', rgb: '232,180,160' },
  { id: 'obsidian-gold', name: '曜石黑金', rgb: '227,194,122' },
  { id: 'rime-silver', name: '雾凇银灰', rgb: '201,211,222' },
  { id: 'amber-dusk', name: '琥珀暮光', rgb: '242,166,90' },
  { id: 'emerald-forest', name: '翡翠森林', rgb: '95,211,154' },
  { id: 'galaxy-blues', name: '星河蓝调', rgb: '122,168,255' },
]

const fontSizes = [
  { label: 'settings.fontScale.small', value: 'small' },
  { label: 'settings.fontScale.standard', value: 'standard' },
  { label: 'settings.fontScale.large', value: 'large' },
  { label: 'settings.fontScale.xlarge', value: 'xlarge' },
]

const pages = routes.flatMap((route) => {
  if (route.meta?.hidden !== undefined) return []
  return {
    label: route.meta!.name,
    value: route.name as string,
  }
})

const appStore = useAppStore()
const appSettings = useAppSettingsStore()

const resetFontFamily = () => {
  appSettings.app.fontFamily = DefaultFontFamily
}

const onThemeClick = (e: MouseEvent) => {
  document.documentElement.style.setProperty('--x', e.clientX + 'px')
  document.documentElement.style.setProperty('--y', e.clientY + 'px')
}

const handleOpenLocalesFolder = async () => {
  await MakeDir(LocalesFilePath)
  await OpenDir(LocalesFilePath)
}

const palette = computed({
  get: () => appSettings.app.themePalette || 'gold-sea',
  set: (value: string) => {
    appSettings.app.themePalette = value
  },
})

const videoInput = ref<HTMLInputElement>()

const resetBackground = () => {
  appSettings.app.backgroundVideo = ''
  if (previewVideoURL.value) URL.revokeObjectURL(previewVideoURL.value)
  previewVideoURL.value = ''
}

const storePickedVideo = async (path: string) => {
  const lower = path.toLowerCase()
  const dest = lower.endsWith('.webm') ? 'data/user-bg-video.webm' : 'data/user-bg-video.mp4'
  const other = dest.endsWith('.webm') ? 'data/user-bg-video.mp4' : 'data/user-bg-video.webm'
  if (path !== dest) await CopyFile(path, dest)
  await RemoveFile(other).catch(() => undefined)
  if (previewVideoURL.value) URL.revokeObjectURL(previewVideoURL.value)
  previewVideoURL.value = ''
  appSettings.app.backgroundVideo = dest
}

const chooseVideo = async () => {
  if ((window as Window & { __DHAGN_PREVIEW__?: boolean }).__DHAGN_PREVIEW__) {
    videoInput.value?.click()
    return
  }
  try {
    const path = await PickFile('选择背景视频', '*.mp4;*.webm')
    await storePickedVideo(path)
  } catch (error) {
    if (String(error) !== 'cancelled') message.error(error)
  }
}

const onVideoFile = (event: Event) => {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (previewVideoURL.value) URL.revokeObjectURL(previewVideoURL.value)
  previewVideoURL.value = URL.createObjectURL(file)
  appSettings.app.backgroundVideo = file.name
}
</script>
<template>
  <div class="px-8 py-12 text-18 font-bold">{{ $t('settings.personalization') }}</div>

  <Card>
    <div class="px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold">{{ $t('settings.theme.name') }}</div>
      <Radio v-model="appSettings.app.theme" tone="blue" :options="ThemeOptions" @click="onThemeClick" />
    </div>
    <div class="px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold">{{ $t('settings.color.name') }}</div>
      <div class="flex items-center">
        <div v-if="appSettings.app.color === Color.Custom" class="flex items-center mr-4">
          <ColorPicker v-model="appSettings.app.primaryColor">
            <template #suffix>
              <div class="text-12">{{ $t('settings.color.primary') }}</div>
            </template>
          </ColorPicker>
          <ColorPicker v-model="appSettings.app.secondaryColor">
            <template #suffix>
              <div class="text-12">{{ $t('settings.color.secondary') }}</div>
            </template>
          </ColorPicker>
        </div>
        <Radio v-model="appSettings.app.color" tone="green" :options="ColorOptions" />
      </div>
    </div>
    <div class="palette-row px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold flex items-center gap-8">
        {{ $t('settings.palette.name') }}
        <span class="tag-new">{{ $t('settings.palette.fresh') }}</span>
      </div>
      <div class="themes" role="radiogroup" :aria-label="$t('settings.palette.name')">
        <button
          v-for="item in palettes"
          :key="item.id"
          type="button"
          class="t"
          :class="{ on: palette === item.id }"
          :style="{ '--c': item.rgb }"
          @click="palette = item.id"
        >
          <i />{{ item.name }}
        </button>
      </div>
    </div>
    <div class="px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold">{{ $t('settings.motion.name') }}</div>
      <div class="flex items-center gap-12">
        <button type="button" class="ghost-btn" @click="chooseVideo">{{ $t('settings.motion.change') }}</button>
        <button type="button" class="ghost-btn" @click="resetBackground">{{ $t('settings.motion.reset') }}</button>
        <Switch v-model="appSettings.app.dynamicBackground" />
        <input ref="videoInput" hidden type="file" accept="video/mp4,video/webm" @change="onVideoFile" />
      </div>
    </div>
    <div class="px-8 py-12 flex items-center justify-between">
      <div class="flex items-center text-16 font-bold">
        <div class="mr-4">{{ $t('settings.lang.name') }}</div>
        <Button type="text" icon="link" @click="BrowserOpenURL(APP_LOCALES_URL)" />
        <Button type="text" icon="folder" @click="handleOpenLocalesFolder" />
        <Button
          v-tips="'settings.lang.load'"
          :loading="appStore.localesLoading"
          type="text"
          icon="refresh"
          @click="appStore.loadLocales()"
        />
      </div>
      <Radio v-model="appSettings.app.lang" tone="purple" :options="appStore.locales" />
    </div>
    <div class="px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold">{{ $t('settings.fontFamily') }}</div>
      <Input v-model="appSettings.app.fontFamily" editable class="text-14">
        <template #suffix>
          <Button
            v-tips="'settings.resetFont'"
            type="text"
            size="small"
            icon="reset"
            @click="resetFontFamily"
          />
        </template>
      </Input>
    </div>
    <div class="px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold">{{ $t('settings.fontScale.name') }}</div>
      <Radio v-model="appSettings.app.fontScale" tone="gold" :options="fontSizes" />
    </div>
    <div class="px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold">{{ $t('settings.pages.name') }}</div>
      <CheckBox v-model="appSettings.app.pages" :options="pages" />
    </div>
  </Card>
</template>
