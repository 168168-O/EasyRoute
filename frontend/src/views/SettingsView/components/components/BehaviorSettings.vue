<script lang="ts" setup>
import { computed, ref } from 'vue'

import { ExitApp } from '@/bridge'
import { localHotkeyWarning } from '@/utils/hotkeyConflict'
import { screenshotHotkeyWarning } from '@/utils/screenshotHotkeys'
import { WebviewGpuPolicyOptions, WindowStateOptions } from '@/constant/app'
import { OS } from '@/enums/app'
import { useAppSettingsStore, useEnvStore } from '@/stores'
import {
  confirm,
  message,
  CheckPermissions,
  SwitchPermissions,
  RunWithPowerShell,
  IsAutoStartEnabled,
  EnableAutoStart,
  DisableAutoStart,
} from '@/utils'

const appSettings = useAppSettingsStore()
const envStore = useEnvStore()

const hotkeyWarn = computed(() => {
  const local = localHotkeyWarning(
    String(appSettings.app.screenshotHotkey || ''),
    String(appSettings.app.screenshotTranslateHotkey || ''),
  )
  const remote = screenshotHotkeyWarning.value
  return [local, remote].filter((item, index, all) => item && all.indexOf(item) === index).join('\n')
})

const isAdmin = ref(false)
const isAutoStart = ref(false)

const restartApp = async (admin = false) => {
  if (admin) {
    await RunWithPowerShell(envStore.env.appPath, [], { admin, wait: false })
  } else {
    await RunWithPowerShell('explorer', [envStore.env.appPath], { wait: false })
  }
  await ExitApp()
}

const onPermChange = async (v: boolean) => {
  try {
    await SwitchPermissions(v)
    appSettings.app.preferRunAsAdmin = v
    if (v !== envStore.env.isPrivileged) {
      const ok = await confirm('Notice', 'Restart the application now?').catch(() => 0)
      ok && (await restartApp(v))
    }
  } catch (error: any) {
    message.error(error)
    console.log(error)
  }
}

const onTaskSchChange = async (v: boolean) => {
  isAutoStart.value = !v

  try {
    await (v ? EnableAutoStart(appSettings.app.startupDelay) : DisableAutoStart())
    isAutoStart.value = v
  } catch (error: any) {
    console.error(error)
    message.error(error)
  }
}

const onStartupDelayChange = async (delay: number) => {
  if (appSettings.app.startupDelay !== delay) {
    try {
      await EnableAutoStart(delay)
      appSettings.app.startupDelay = delay
    } catch (error: any) {
      console.error(error)
      message.error(error)
    }
  }
}

IsAutoStartEnabled().then((res) => {
  isAutoStart.value = res
})

if (envStore.env.os === OS.Windows) {
  CheckPermissions().then((admin) => {
    isAdmin.value = admin
  })
}
</script>

<template>
  <div class="px-8 py-12 text-18 font-bold">{{ $t('settings.behavior') }}</div>

  <Card>
    <div v-platform="[OS.Windows]" class="px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold">
        {{ $t('settings.admin') }}
        <span class="font-normal text-12">({{ $t('settings.needRestart') }})</span>
      </div>
      <div class="flex items-center gap-4">
        <Button
          v-if="envStore.env.isPrivileged !== isAdmin"
          v-tips="'titlebar.restart'"
          type="primary"
          icon="refresh"
          size="small"
          @click="() => restartApp(isAdmin)"
        />
        <Switch v-model="isAdmin" @change="onPermChange" />
      </div>
    </div>
    <div class="px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold">
        {{ $t('settings.startup.name') }}
      </div>
      <div class="flex items-center">
        <Radio
          v-if="isAutoStart"
          v-model="appSettings.app.windowStartState"
          :options="WindowStateOptions"
          type="number"
        />
        <Switch v-model="isAutoStart" class="ml-16" @change="onTaskSchChange" />
      </div>
    </div>
    <div
      v-if="isAutoStart"
      v-platform="[OS.Windows]"
      class="px-8 py-12 flex items-center justify-between"
    >
      <div class="text-16 font-bold">
        {{ $t('settings.startup.startupDelay') }}
      </div>
      <Input
        :model-value="appSettings.app.startupDelay"
        :min="10"
        :max="180"
        editable
        type="number"
        @submit="onStartupDelayChange"
      >
        <template #suffix="{ showInput }">
          <span class="ml-4" @click="showInput">{{ $t('settings.startup.delay') }}</span>
        </template>
      </Input>
    </div>
    <div class="px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold">{{ $t('settings.exitOnClose') }}</div>
      <Switch v-model="appSettings.app.exitOnClose" />
    </div>
    <div class="px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold">{{ $t('settings.autoStartKernel') }}</div>
      <Switch v-model="appSettings.app.autoStartKernel" />
    </div>
    <div class="px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold">{{ $t('settings.closeKernelOnExit') }}</div>
      <Switch v-model="appSettings.app.closeKernelOnExit" />
    </div>
    <div v-platform="[OS.Linux]" class="px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold">
        {{ $t('settings.webviewGpuPolicy.name') }}
        <span class="font-normal text-12">({{ $t('settings.needRestart') }})</span>
      </div>
      <Radio v-model="appSettings.app.webviewGpuPolicy" :options="WebviewGpuPolicyOptions" />
    </div>
    <div class="px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold">{{ $t('settings.addPluginToMenu') }}</div>
      <Switch v-model="appSettings.app.addPluginToMenu" />
    </div>
    <div class="px-8 py-12 flex items-center justify-between">
      <div class="text-16 font-bold">{{ $t('settings.addGroupToMenu') }}</div>
      <Switch v-model="appSettings.app.addGroupToMenu" />
    </div>
    <div v-platform="[OS.Windows]" class="px-8 py-12 flex items-center justify-between gap-16">
      <div class="text-16 font-bold">
        {{ $t('settings.screenshot.hotkey') }}
        <div class="shot-tip">{{ $t('settings.screenshot.hotkeyTips') }}</div>
      </div>
      <input
        v-model="appSettings.app.screenshotHotkey"
        class="shot-field"
        spellcheck="false"
        autocomplete="off"
        placeholder="Ctrl+Alt+A"
      />
    </div>
    <div v-platform="[OS.Windows]" class="px-8 py-12 flex items-center justify-between gap-16">
      <div class="text-16 font-bold">
        {{ $t('settings.screenshot.translateHotkey') }}
        <div class="shot-tip">{{ $t('settings.screenshot.translateHotkeyTips') }}</div>
      </div>
      <input
        v-model="appSettings.app.screenshotTranslateHotkey"
        class="shot-field"
        spellcheck="false"
        autocomplete="off"
        placeholder="Ctrl+Alt+T"
      />
    </div>
    <p v-if="hotkeyWarn" v-platform="[OS.Windows]" class="shot-warn">{{ hotkeyWarn }}</p>
    <div v-platform="[OS.Windows]" class="px-8 py-12 flex items-center justify-between gap-16">
      <div class="text-16 font-bold">
        {{ $t('settings.screenshot.deepseek') }}
        <div class="shot-tip">{{ $t('settings.screenshot.deepseekTips') }}</div>
      </div>
      <input
        v-model="appSettings.app.deepseekApiKey"
        class="shot-field"
        type="password"
        spellcheck="false"
        autocomplete="off"
        placeholder="sk-"
      />
    </div>
  </Card>
</template>

<style scoped>
.shot-tip {
  margin-top: 4px;
  max-width: 460px;
  font-size: 12px;
  font-weight: 400;
  line-height: 1.45;
  color: rgba(159, 214, 255, 0.82);
}
.shot-field {
  width: 210px;
  height: 32px;
  flex: none;
  padding: 0 10px;
  border-radius: 8px;
  border: 1px solid rgba(232, 213, 163, 0.55);
  background: rgba(6, 20, 40, 0.55);
  color: #f6f1e6;
  font-size: 13px;
  outline: none;
}
.shot-field:focus {
  border-color: rgba(159, 214, 255, 0.95);
  box-shadow: 0 0 0 1px rgba(159, 214, 255, 0.35);
}
.shot-warn {
  margin: 0 8px 12px;
  color: #e7c98a;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-line;
}
</style>
