<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { ClipboardGetText, ClipboardSetText, Translate } from '@/bridge'
import { useAppSettingsStore } from '@/stores'
import { message, sampleID } from '@/utils'

const { t } = useI18n()
const appSettings = useAppSettingsStore()

const targets = [
  { value: 'auto', label: '自动' },
  { value: 'zh-Hans', label: '中文' },
  { value: 'en', label: 'English' },
  { value: 'ja', label: '日本語' },
  { value: 'ko', label: '한국어' },
  { value: 'fr', label: 'Français' },
  { value: 'de', label: 'Deutsch' },
  { value: 'es', label: 'Español' },
  { value: 'ru', label: 'Русский' },
  { value: 'pt', label: 'Português' },
  { value: 'vi', label: 'Tiếng Việt' },
  { value: 'th', label: 'ไทย' },
]

const input = ref('')
const output = ref('')
const target = ref('auto')
const lastFrom = ref('')
const lastTo = ref('')
const busy = ref(false)
const errorText = ref('')

const history = computed(() => (appSettings.app.translateHistory || []).slice(0, 20))

const langLabel = (code: string) => targets.find((item) => item.value === code)?.label || code || t('translate.auto')

const remember = (entry: { input: string; output: string; from: string; to: string }) => {
  const next = [
    { id: sampleID(), at: Date.now(), ...entry },
    ...history.value.filter((item) => !(item.input === entry.input && item.output === entry.output)),
  ]
  appSettings.app.translateHistory = next.slice(0, 20)
}

const runTranslate = async () => {
  const text = input.value.trim()
  if (!text) {
    errorText.value = '请先输入要翻译的内容'
    message.warn(errorText.value)
    return
  }
  busy.value = true
  errorText.value = ''
  try {
    const result = await Translate(text, target.value)
    output.value = result.text
    lastFrom.value = result.sourceLang || ''
    lastTo.value = result.targetLang || ''
    remember({
      input: text,
      output: result.text,
      from: result.sourceLang || '',
      to: result.targetLang || target.value,
    })
  } catch (error) {
    const detail = String(error || '')
    errorText.value = detail.includes('翻译') || detail.includes('请先') || detail.includes('内容太长')
      ? detail
      : `翻译失败：${detail || '请稍后再试'}`
    message.error(errorText.value)
  } finally {
    busy.value = false
  }
}

const pasteAndTranslate = async () => {
  let text = ''
  try {
    text = (await ClipboardGetText()) || ''
  } catch (error) {
    errorText.value = t('translate.emptyClipboard')
    message.error(error)
    return
  }
  if (!text.trim()) {
    errorText.value = t('translate.emptyClipboard')
    message.warn(errorText.value)
    return
  }
  input.value = text
  await runTranslate()
}

const copyResult = async () => {
  if (!output.value.trim()) {
    message.warn(t('translate.copyEmpty'))
    return
  }
  try {
    await ClipboardSetText(output.value)
    message.success(t('translate.copied'))
  } catch (error) {
    message.error(error)
  }
}

const swap = () => {
  const previous = input.value
  input.value = output.value
  output.value = previous
  if (lastFrom.value && targets.some((item) => item.value === lastFrom.value)) {
    target.value = lastFrom.value
  } else if (lastTo.value === 'en' || lastTo.value === 'zh-Hans') {
    target.value = lastTo.value === 'en' ? 'zh-Hans' : 'en'
  }
  const from = lastFrom.value
  lastFrom.value = lastTo.value
  lastTo.value = from
  errorText.value = ''
}

const restore = (item: { input: string; output: string; from: string; to: string }) => {
  input.value = item.input
  output.value = item.output
  lastFrom.value = item.from
  lastTo.value = item.to
  if (item.to) target.value = item.to
  errorText.value = ''
}

const preview = (text: string) => (text.length > 42 ? text.slice(0, 42) + '…' : text)
</script>

<template>
  <div class="tr-page" @keydown.ctrl.enter.prevent="runTranslate" @keydown.meta.enter.prevent="runTranslate">
    <div class="rt-head">
      <div>
        <div class="rt-title">{{ t('router.translate') }}</div>
        <div class="rt-sub">{{ t('translate.hint') }}</div>
      </div>
      <label class="tr-target">
        <span>{{ t('translate.target') }}</span>
        <select v-model="target" class="tr-select">
          <option v-for="item in targets" :key="item.value" :value="item.value">{{ item.label }}</option>
        </select>
      </label>
    </div>

    <div class="tr-panes">
      <label class="tr-pane">
        <span>{{ t('translate.source') }}</span>
        <textarea v-model="input" class="tr-box" :placeholder="t('translate.sourcePh')" />
      </label>
      <button type="button" class="tr-swap" :title="t('translate.swap')" @click="swap">⇄</button>
      <label class="tr-pane">
        <span>
          {{ t('translate.result') }}
          <em v-if="lastFrom || lastTo">{{ langLabel(lastFrom) }} → {{ langLabel(lastTo) }}</em>
        </span>
        <textarea v-model="output" class="tr-box" readonly :placeholder="t('translate.resultPh')" />
      </label>
    </div>

    <div class="tr-actions">
      <button type="button" class="btn add tr-go" :disabled="busy" @click="runTranslate">
        {{ busy ? t('translate.working') : t('translate.go') }}
      </button>
      <button type="button" class="ghost-btn" :disabled="busy" @click="pasteAndTranslate">
        {{ t('translate.paste') }}
      </button>
      <button type="button" class="ghost-btn" @click="copyResult">{{ t('translate.copy') }}</button>
      <span class="tr-keys">Ctrl+Enter</span>
    </div>
    <div v-if="errorText" class="tr-error">{{ errorText }}</div>
    <div class="tr-note">{{ t('translate.direct') }}</div>

    <div class="group">
      <b>{{ t('translate.history') }}</b>
      <span>{{ history.length }}</span>
    </div>
    <div v-if="!history.length" class="app-exe">{{ t('translate.historyEmpty') }}</div>
    <button
      v-for="item in history"
      :key="item.id"
      type="button"
      class="tr-history"
      @click="restore(item)"
    >
      <b>{{ preview(item.input) }}</b>
      <span>{{ preview(item.output) }}</span>
    </button>
  </div>
</template>
