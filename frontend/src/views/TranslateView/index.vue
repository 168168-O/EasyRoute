<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { ClipboardGetText, ClipboardSetText, TestTranslateAI, Translate, type TranslateHit } from '@/bridge'
import { useAppSettingsStore, useKernelApiStore } from '@/stores'
import { message, sampleID } from '@/utils'

const { t } = useI18n()
const appSettings = useAppSettingsStore()
const kernelApi = useKernelApiStore()

const coreProxyURL = () => {
  if (!kernelApi.running) return ''
  const endpoint = kernelApi.getProxyEndpoint()
  if (!endpoint || endpoint.schema !== 'http' || !endpoint.port) return ''
  const host =
    endpoint.host && endpoint.host !== '0.0.0.0' && endpoint.host !== '::' ? endpoint.host : '127.0.0.1'
  const auth = endpoint.username
    ? `${encodeURIComponent(endpoint.username)}:${encodeURIComponent(endpoint.password || '')}@`
    : ''
  return `http://${auth}${host}:${endpoint.port}`
}

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
const results = ref<TranslateHit[]>([])
const note = ref('')
const target = ref('auto')
const busy = ref(false)
const testing = ref(false)
const aiOpen = ref(false)
const errorText = ref('')

const presets = [
  { id: 'custom', label: '自定义', baseUrl: '', model: '' },
  { id: 'deepseek', label: 'DeepSeek', baseUrl: 'https://api.deepseek.com', model: 'deepseek-chat' },
  { id: 'qwen', label: '通义千问', baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1', model: 'qwen-plus' },
  { id: 'zhipu', label: '智谱', baseUrl: 'https://open.bigmodel.cn/api/paas/v4', model: 'glm-4-flash' },
  { id: 'kimi', label: 'Kimi', baseUrl: 'https://api.moonshot.cn/v1', model: 'moonshot-v1-8k' },
  { id: 'openai', label: 'OpenAI', baseUrl: 'https://api.openai.com/v1', model: 'gpt-4o-mini' },
]

const ai = computed(() => appSettings.app.translateAI)
const primary = computed(() => results.value[0])
const rest = computed(() => results.value.slice(1))

const history = computed(() => (appSettings.app.translateHistory || []).slice(0, 20))

const langLabel = (code: string) => targets.find((item) => item.value === code)?.label || code || t('translate.auto')

const hitLabel = (hit?: TranslateHit) => {
  if (!hit) return ''
  if (hit.provider === 'google') return t('translate.engineGoogle')
  if (hit.provider === 'bing') return t('translate.engineBing')
  if (hit.provider === 'mymemory') return t('translate.engineMyMemory')
  if (hit.provider === 'deepl') return 'DeepL'
  if (hit.provider === 'openai') return hit.label || 'AI'
  return hit.label || hit.provider
}

const aiReady = computed(() => {
  if (ai.value.provider === 'openai') return !!ai.value.apiKey && !!ai.value.baseUrl && !!ai.value.model
  if (ai.value.provider === 'deepl') return !!ai.value.deeplKey
  return false
})

const applyPreset = (id: string) => {
  ai.value.preset = id
  const preset = presets.find((item) => item.id === id)
  if (!preset || preset.id === 'custom') return
  ai.value.baseUrl = preset.baseUrl
  ai.value.model = preset.model
  ai.value.provider = 'openai'
}

const remember = (entry: { input: string; output: string; from: string; to: string; provider: string }) => {
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
    const bundle = await Translate(text, target.value, coreProxyURL(), JSON.stringify(ai.value))
    results.value = bundle.results || []
    note.value = bundle.note || ''
    const top = results.value[0]
    if (!top?.text) throw new Error('翻译失败：翻译服务没有返回可用的结果。请稍后再试。')
    remember({
      input: text,
      output: top.text,
      from: top.sourceLang || '',
      to: top.targetLang || target.value,
      provider: top.provider || '',
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
  const text = primary.value?.text || ''
  if (!text.trim()) {
    message.warn(t('translate.copyEmpty'))
    return
  }
  try {
    await ClipboardSetText(text)
    message.success(t('translate.copied'))
  } catch (error) {
    message.error(error)
  }
}

const swap = () => {
  const previous = input.value
  const top = primary.value
  input.value = top?.text || ''
  results.value = top?.text ? [{ ...top, text: previous, provider: '' }] : []
  note.value = ''
  if (top?.sourceLang && targets.some((item) => item.value === top.sourceLang)) {
    target.value = top.sourceLang
  } else if (top?.targetLang === 'en' || top?.targetLang === 'zh-Hans') {
    target.value = top.targetLang === 'en' ? 'zh-Hans' : 'en'
  }
  errorText.value = ''
}

const restore = (item: { input: string; output: string; from: string; to: string; provider?: string }) => {
  input.value = item.input
  results.value = [
    {
      text: item.output,
      sourceLang: item.from,
      targetLang: item.to,
      provider: item.provider || '',
    },
  ]
  note.value = ''
  if (item.to) target.value = item.to
  errorText.value = ''
}

const testAI = async () => {
  testing.value = true
  errorText.value = ''
  try {
    message.success(await TestTranslateAI(JSON.stringify(ai.value), coreProxyURL()))
  } catch (error) {
    const detail = String(error || '')
    errorText.value = detail
    message.error(detail)
  } finally {
    testing.value = false
  }
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
      <button type="button" class="tr-swap glass-ring" :title="t('translate.swap')" @click="swap">⇄</button>
      <label class="tr-pane">
        <span>
          {{ t('translate.result') }}
          <em v-if="primary && (primary.sourceLang || primary.targetLang)">{{ langLabel(primary.sourceLang) }} → {{ langLabel(primary.targetLang) }}</em>
        </span>
        <textarea class="tr-box" readonly :value="primary?.text || ''" :placeholder="t('translate.resultPh')" />
        <p v-if="primary" class="tr-engine">
          {{ hitLabel(primary) }}
          <em v-if="primary.sourceLang || primary.targetLang">{{ langLabel(primary.sourceLang) }} → {{ langLabel(primary.targetLang) }}</em>
        </p>
      </label>
    </div>
    <div v-if="rest.length" class="tr-compare">
      <div v-for="(row, index) in rest" :key="`${row.provider}-${index}`" class="tr-compare-row">
        <b>{{ hitLabel(row) }}</b>
        <span>{{ row.text }}</span>
      </div>
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
    <div v-if="note" class="tr-note">{{ note }}</div>
    <div class="tr-note">{{ t('translate.direct') }}</div>

    <button type="button" class="tr-ai-toggle" @click="aiOpen = !aiOpen">
      <b>{{ t('translate.aiTitle') }}</b>
      <span>{{ aiReady ? t('translate.aiReady') : t('translate.aiEmpty') }}</span>
    </button>
    <div v-if="aiOpen" class="tr-ai">
      <label class="tr-field">
        <span>{{ t('translate.aiMode') }}</span>
        <select class="tr-select" :value="ai.provider" @change="ai.provider = ($event.target as HTMLSelectElement).value as 'off' | 'openai' | 'deepl'">
          <option value="off">{{ t('translate.aiOff') }}</option>
          <option value="openai">{{ t('translate.aiOpenAI') }}</option>
          <option value="deepl">{{ t('translate.aiDeepL') }}</option>
        </select>
      </label>
      <template v-if="ai.provider === 'openai'">
        <label class="tr-field">
          <span>{{ t('translate.aiPreset') }}</span>
          <select class="tr-select" :value="ai.preset" @change="applyPreset(($event.target as HTMLSelectElement).value)">
            <option v-for="item in presets" :key="item.id" :value="item.id">{{ item.label }}</option>
          </select>
        </label>
        <label class="tr-field tr-field-grow">
          <span>{{ t('translate.aiBase') }}</span>
          <input v-model="ai.baseUrl" class="tr-select" autocomplete="off" placeholder="https://api.deepseek.com" />
        </label>
        <label class="tr-field">
          <span>{{ t('translate.aiModel') }}</span>
          <input v-model="ai.model" class="tr-select" autocomplete="off" placeholder="deepseek-chat" />
        </label>
        <label class="tr-field tr-field-grow">
          <span>{{ t('translate.aiKey') }}</span>
          <input v-model="ai.apiKey" class="tr-select" type="password" autocomplete="new-password" />
        </label>
      </template>
      <label v-else-if="ai.provider === 'deepl'" class="tr-field tr-field-grow">
        <span>{{ t('translate.aiKey') }}</span>
        <input v-model="ai.deeplKey" class="tr-select" type="password" autocomplete="new-password" />
      </label>
      <button v-if="ai.provider !== 'off'" type="button" class="ghost-btn" :disabled="testing" @click="testAI">
        {{ testing ? t('translate.aiTesting') : t('translate.aiTest') }}
      </button>
      <p class="tr-note">{{ ai.provider === 'deepl' ? t('translate.aiDeepLHint') : t('translate.aiKeyHint') }}</p>
    </div>

    <div class="group">
      <b>{{ t('translate.history') }}</b>
      <span>{{ history.length }}</span>
    </div>
    <div class="tr-history-list">
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
  </div>
</template>
