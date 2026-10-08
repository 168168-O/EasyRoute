<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { getConnections, getProxies } from '@/api/kernel'
import { BrowserOpenURL, HttpGet, LookupHost, SecurityCheck, type SecurityReport } from '@/bridge'
import { useAppSettingsStore, useKernelApiStore } from '@/stores'
import { explainDomainRoute, normalizePinnedRoutes, type RouteExplanation } from '@/utils/appRouting'
import { message } from '@/utils'
import {
  CHECK_TIMEOUT_SEC,
  DIRECT_PROXY,
  formatCoreProxy,
  runNetworkCheck,
  selectedNode,
  type NetworkCheckReport,
  type ProbeResponse,
} from '@/utils/networkCheck'

const report = ref<NetworkCheckReport | null>(null)
const running = ref(false)
const security = ref<SecurityReport | null>(null)
const securityRunning = ref(false)
const domain = ref('')
const routeRows = ref<RouteExplanation[]>([])
const routeNote = ref('')

const links = [
  { label: 'home.networkCheck.openSpeed', url: 'https://speed.cloudflare.com' },
  { label: 'home.networkCheck.openDns', url: 'https://browserleaks.com/dns' },
  { label: 'home.networkCheck.openIp', url: 'https://browserleaks.com/ip' },
]

const { t } = useI18n()
const kernel = useKernelApiStore()
const settings = useAppSettingsStore()

const timedGet = async (url: string, proxy: string): Promise<ProbeResponse> => {
  const started = Date.now()
  try {
    const res = await HttpGet(url, {}, { Proxy: proxy, Timeout: CHECK_TIMEOUT_SEC, Redirect: true })
    return {
      ok: res.status > 0 && res.status < 500,
      status: res.status,
      body: typeof res.body === 'string' ? res.body : JSON.stringify(res.body ?? ''),
      elapsedMs: Date.now() - started,
    }
  } catch (error) {
    return {
      ok: false,
      status: 0,
      body: '',
      elapsedMs: Date.now() - started,
      error: error instanceof Error ? error.message : String(error),
    }
  }
}

const start = async () => {
  if (running.value) return
  running.value = true
  try {
    let node = ''
    if (kernel.running) {
      try {
        const data = await getProxies()
        node = selectedNode(data?.proxies)
      } catch {
        node = ''
      }
    }
    const endpoint = kernel.running ? kernel.getProxyEndpoint() : undefined
    const proxy = formatCoreProxy(endpoint)
    report.value = await runNetworkCheck({
      directGet: (url) => timedGet(url, DIRECT_PROXY),
      proxyGet: (url) => timedGet(url, proxy),
      connections: async () => {
        if (!kernel.running) return []
        try {
          const data = await getConnections()
          return (data?.connections || []).map((item) => {
            const row = item as { chains?: string[]; metadata?: { host?: string } }
            return { chains: row.chains || [], host: row.metadata?.host || '' }
          })
        } catch {
          return []
        }
      },
      lookup: async (host) => {
        const result = await LookupHost(host)
        return {
          ok: !!result?.flag && !!result.data,
          address: result?.flag ? String(result.data || '') : '',
        }
      },
      core: {
        running: kernel.running,
        tun: kernel.running && !!kernel.config.tun.enable,
        mode: kernel.config.mode || '',
        node,
        proxy,
      },
      pinned: normalizePinnedRoutes(settings.app.pinnedRoutes),
    })
  } catch (error) {
    message.error(error)
  } finally {
    running.value = false
  }
}

const exitText = (row: NetworkCheckReport['local']) => {
  if (row.status === 'red' || !row.ip) return t('home.networkCheck.failed')
  return row.place ? `${row.ip}  ${row.place}` : row.ip
}

const siteText = (site: NetworkCheckReport['sites'][number]) => {
  const route = t(site.route === 'proxy' ? 'home.networkCheck.viaProxy' : 'home.networkCheck.viaDirect')
  if (site.status === 'red' || site.elapsedMs == null) return `${route}  ${t('home.networkCheck.failed')}`
  return `${route}  ${site.elapsedMs}ms`
}

const proxyText = computed(() => {
  if (!report.value) return ''
  const place = exitText(report.value.proxy)
  return report.value.core.node ? `${place}  ·  ${report.value.core.node}` : place
})

const dnsText = computed(() => {
  if (!report.value) return ''
  if (report.value.dns.status === 'red') return t('home.networkCheck.dnsFail')
  return report.value.dns.address
    ? `${t('home.networkCheck.dnsOk')}  ${report.value.dns.address}`
    : t('home.networkCheck.dnsOk')
})

const coreDot = computed(() => {
  if (!report.value?.core.running) return 'red'
  return report.value.core.tun ? 'green' : 'amber'
})

const runSecurity = async () => {
  if (securityRunning.value) return
  securityRunning.value = true
  try {
    const port = Number(kernel.config['mixed-port'] || 0)
    const tun = kernel.config.tun?.device || 'tun0'
    security.value = await SecurityCheck(port, tun)
  } catch (error) {
    message.error(error)
  } finally {
    securityRunning.value = false
  }
}

const lookupRoute = () => {
  const rows = explainDomainRoute(domain.value, {
    pinned: settings.app.pinnedRoutes,
    domesticDirect: settings.app.domesticDirect !== false,
  })
  routeRows.value = rows
  routeNote.value = rows.length
    ? ''
    : domain.value.trim()
      ? t('home.networkCheck.lookupEmpty')
      : t('home.networkCheck.domainPh')
}

const coreText = computed(() => {
  if (!report.value) return ''
  const state = report.value.core.running ? t('home.networkCheck.coreOn') : t('home.networkCheck.coreOff')
  const tun = report.value.core.tun ? t('home.networkCheck.tunOn') : t('home.networkCheck.tunOff')
  const modeKey = report.value.core.mode
  const mode =
    modeKey === 'rule' || modeKey === 'global' || modeKey === 'direct' ? t(`home.${modeKey}`) : modeKey
  return [state, mode, tun].filter(Boolean).join('  ·  ')
})
</script>

<template>
  <Card title="home.networkCheck.title" class="network-check">
    <template #extra>
      <Button type="primary" size="small" :loading="running" @click="start">
        {{ running ? t('home.networkCheck.running') : t('home.networkCheck.start') }}
      </Button>
    </template>
    <div class="nc-tools">
      <button type="button" class="ghost-btn nc-link" :disabled="securityRunning" @click="runSecurity">
        {{ securityRunning ? t('home.networkCheck.securityRunning') : t('home.networkCheck.securityStart') }}
      </button>
      <button
        v-for="link in links"
        :key="link.url"
        type="button"
        class="ghost-btn nc-link"
        @click="BrowserOpenURL(link.url)"
      >
        {{ t(link.label) }}
      </button>
    </div>
    <p v-if="!security" class="nc-idle">{{ t('home.networkCheck.securityIdle') }}</p>
    <div v-else class="nc-list nc-security">
      <div v-for="item in security.items" :key="item.id" class="nc-row">
        <i class="nc-dot" :class="item.level" />
        <span class="nc-name">{{ item.name }}</span>
        <span class="nc-value">{{ item.text }}</span>
      </div>
    </div>
    <div class="nc-lookup">
      <input
        v-model="domain"
        class="nc-input"
        :placeholder="t('home.networkCheck.domainPh')"
        @keydown.enter="lookupRoute"
      />
      <button type="button" class="ghost-btn nc-link" @click="lookupRoute">
        {{ t('home.networkCheck.lookup') }}
      </button>
    </div>
    <div v-if="routeRows.length" class="nc-routes">
      <div v-for="(row, index) in routeRows" :key="index" class="nc-row">
        <span class="nc-name">{{ row.scope }}</span>
        <span class="nc-value">
          {{ t(row.route === 'proxy' ? 'home.networkCheck.viaProxy' : 'home.networkCheck.viaDirect') }}
          · {{ row.rule }}
        </span>
      </div>
    </div>
    <p v-else-if="routeNote" class="nc-idle">{{ routeNote }}</p>
    <p v-if="!report" class="nc-idle">{{ t('home.networkCheck.idle') }}</p>
    <div v-else class="nc-list">
      <div class="nc-row">
        <i class="nc-dot" :class="report.local.status" />
        <span class="nc-name">{{ t('home.networkCheck.local') }}</span>
        <span class="nc-value">{{ exitText(report.local) }}</span>
      </div>
      <div class="nc-row">
        <i class="nc-dot" :class="report.proxy.status" />
        <span class="nc-name">{{ t('home.networkCheck.proxy') }}</span>
        <span class="nc-value">{{ proxyText }}</span>
      </div>
      <div class="nc-row">
        <i class="nc-dot" :class="report.dns.status" />
        <span class="nc-name">{{ t('home.networkCheck.dns') }}</span>
        <span class="nc-value">{{ dnsText }}</span>
      </div>
      <div class="nc-row">
        <i class="nc-dot" :class="coreDot" />
        <span class="nc-name">{{ t('home.networkCheck.core') }}</span>
        <span class="nc-value">{{ coreText }}</span>
      </div>
      <div class="nc-sites">
        <div v-for="site in report.sites" :key="site.id" class="nc-row">
          <i class="nc-dot" :class="site.status" />
          <span class="nc-name">{{ t(`home.networkCheck.sites.${site.id}`) }}</span>
          <span class="nc-value">{{ siteText(site) }}</span>
        </div>
      </div>
    </div>
  </Card>
</template>
