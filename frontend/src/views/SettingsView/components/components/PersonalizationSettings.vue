<script lang="ts" setup>
import { computed, ref } from 'vue'

import { BrowserOpenURL, CopyFile, MakeDir, OpenDir, PickFile, RemoveFile } from '@/bridge'
import { ColorOptions, DefaultFontFamily, LocalesFilePath, ThemeOptions } from '@/constant/app'
import { Color } from '@/enums/app'
import routes from '@/router/routes'
import { useAppSettingsStore, useAppStore } from '@/stores'
import { APP_LOCALES_URL, message } from '@/utils'
import { bumpBackgroundVideo, previewVideoURL } from '@/utils/backgroundVideo'

const palettes = [
  { id: 'gold-sea', name: '鎏金深海', en: 'Gold Abyss', desc: '香槟金、冰蓝与深海蓝', c1: '#D4B26A', c2: '#8FD3FF', c3: '#B69CFF' },
  { id: 'glacier-aurora', name: '冰川极光', en: 'Glacier Aurora', desc: '冰川青、雾蓝与暖金', c1: '#7FE3D4', c2: '#9DB8FF', c3: '#E7C98A' },
  { id: 'jade-celadon', name: '墨玉青瓷', en: 'Jade Celadon', desc: '青瓷、米金与冰蓝', c1: '#8CC7B0', c2: '#E6D3A3', c3: '#8FD3FF' },
  { id: 'night-amethyst', name: '暗夜紫晶', en: 'Night Amethyst', desc: '紫晶、玫粉与香槟金', c1: '#B69CFF', c2: '#F0B8E8', c3: '#E3C27A' },
  { id: 'rose-champagne', name: '玫瑰香槟', en: 'Rose Champagne', desc: '玫瑰、香槟与浅紫', c1: '#E8B4A0', c2: '#F5D7A1', c3: '#C9B6FF' },
  { id: 'obsidian-gold', name: '曜石黑金', en: 'Obsidian Gold', desc: '黑金、石墨与冰蓝', c1: '#E3C27A', c2: '#B8B8B8', c3: '#8FD3FF' },
  { id: 'rime-silver', name: '雾凇银灰', en: 'Rime Silver', desc: '银灰、雾蓝与暖金', c1: '#C9D3DE', c2: '#8FB3C9', c3: '#E3C27A' },
  { id: 'amber-dusk', name: '琥珀暮光', en: 'Amber Dusk', desc: '琥珀、珊瑚与浅紫', c1: '#F2A65A', c2: '#FF8A7A', c3: '#C9B0FF' },
  { id: 'emerald-forest', name: '翡翠森林', en: 'Emerald Forest', desc: '翡翠、柠绿与冰蓝', c1: '#5FD39A', c2: '#C9E58A', c3: '#8FD3FF' },
  { id: 'galaxy-blues', name: '星河蓝调', en: 'Galaxy Blues', desc: '星蓝、紫罗兰与暖金', c1: '#7AA8FF', c2: '#C3A6FF', c3: '#E3C27A' },
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
  bumpBackgroundVideo()
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
  bumpBackgroundVideo()
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
  bumpBackgroundVideo()
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
      <div class="theme-list" role="radiogroup" :aria-label="$t('settings.palette.name')">
        <button
          v-for="item in palettes"
          :key="item.id"
          type="button"
          class="theme-card"
          :class="{ on: palette === item.id }"
          role="radio"
          :aria-checked="palette === item.id"
          @click="palette = item.id"
        >
          <span class="swatch" aria-hidden="true">
            <i :style="{ background: item.c1 }" />
            <i :style="{ background: item.c2 }" />
            <i :style="{ background: item.c3 }" />
          </span>
          <span class="theme-copy">
            <b>{{ item.name }} <em>/ {{ item.en }}</em></b>
            <span>{{ item.desc }}</span>
          </span>
          <span class="tick" aria-hidden="true">
            <svg v-if="palette === item.id" viewBox="0 0 16 16" width="12" height="12">
              <path d="M3.2 8.2 6.3 11.4 12.8 4.6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </span>
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
