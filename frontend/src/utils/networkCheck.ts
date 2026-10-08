/**
 * One-shot 网络检测. Nothing here runs until runNetworkCheck is called.
 *
 * 本地出口 dials with no proxy. 代理出口 and foreign sites go through the
 * core mixed/http inbound, which sends this app's own process to the selected
 * node. 百度 stays direct. 微信 and 抖音 follow the saved pinned choice.
 * A connection captured by the core can override that plan.
 */

import { DOUYIN_DOMAIN_SUFFIXES, WECHAT_DOMAIN_SUFFIXES, type PinnedRoutes } from './appRouting.ts'

export const CHECK_TIMEOUT_SEC = 4
export const SLOW_MS = 800
export const DIRECT_PROXY = 'direct'

const LOCAL_EXIT_URLS = ['https://myip.ipip.net', 'http://myip.ipip.net']
const PROXY_EXIT_URLS = ['https://ipinfo.io/json', 'http://ip-api.com/json?lang=zh-CN']
const DNS_HOSTS = ['www.baidu.com', 'www.qq.com']

export const SITE_PROBES = [
  { id: 'baidu', host: 'www.baidu.com', url: 'https://www.baidu.com/robots.txt' },
  { id: 'wechat', host: 'weixin.qq.com', url: 'https://weixin.qq.com/' },
  { id: 'douyin', host: 'www.douyin.com', url: 'https://www.douyin.com/robots.txt' },
  { id: 'google', host: 'www.google.com', url: 'https://www.google.com/generate_204' },
  { id: 'youtube', host: 'www.youtube.com', url: 'https://www.youtube.com/generate_204' },
  { id: 'github', host: 'github.com', url: 'https://github.com/robots.txt' },
  { id: 'telegram', host: 'web.telegram.org', url: 'https://web.telegram.org/' },
] as const

export type CheckStatus = 'green' | 'amber' | 'red'
export type RouteKind = 'direct' | 'proxy'

export interface ProbeResponse {
  ok: boolean
  status: number
  body: string
  elapsedMs: number
  error?: string
}

export interface ConnectionSnap {
  chains: string[]
  host: string
}

export interface ExitCheck {
  status: CheckStatus
  ip: string
  place: string
}

export interface SiteCheck {
  id: string
  status: CheckStatus
  route: RouteKind
  elapsedMs: number | null
}

export interface NetworkCheckReport {
  local: ExitCheck
  proxy: ExitCheck
  sites: SiteCheck[]
  dns: { status: 'green' | 'red'; address: string }
  core: { running: boolean; tun: boolean; mode: string; node: string }
}

export interface NetworkCheckDeps {
  directGet(url: string): Promise<ProbeResponse>
  proxyGet(url: string): Promise<ProbeResponse>
  connections(): Promise<ConnectionSnap[]>
  lookup(host: string): Promise<{ ok: boolean; address: string }>
  core: { running: boolean; tun: boolean; mode: string; node: string; proxy: string }
  pinned: PinnedRoutes
}

const IP = /(?:\d{1,3}\.){3}\d{1,3}/

const hostMatches = (host: string, suffixes: readonly string[]) => {
  const value = host.toLowerCase().replace(/\.$/, '')
  return suffixes.some((suffix) => {
    const item = suffix.toLowerCase()
    return value === item || value.endsWith('.' + item)
  })
}

/** Which transport this check uses, before a live connection can correct it. */
export const routeForHost = (host: string, pinned: PinnedRoutes): RouteKind => {
  if (hostMatches(host, WECHAT_DOMAIN_SUFFIXES)) return pinned.wechat === 'proxy' ? 'proxy' : 'direct'
  if (hostMatches(host, DOUYIN_DOMAIN_SUFFIXES)) return pinned.douyin === 'proxy' ? 'proxy' : 'direct'
  if (hostMatches(host, ['baidu.com'])) return 'direct'
  return 'proxy'
}

const NEUTRAL_TAGS = new Set(['direct', 'block', 'reject', 'dns-out'])

/** null when the core did not record a chain. direct only when every tag is a local outbound. */
export const classifyChains = (chains: string[]): RouteKind | null => {
  const tags = chains.map((item) => item.trim().toLowerCase()).filter(Boolean)
  if (!tags.length) return null
  return tags.every((tag) => NEUTRAL_TAGS.has(tag)) ? 'direct' : 'proxy'
}

export const matchConnection = (connections: ConnectionSnap[], host: string) => {
  const target = host.toLowerCase()
  const hits = connections.filter((item) => {
    const name = item.host.toLowerCase()
    return name === target || name.endsWith('.' + target)
  })
  return hits.length ? hits[hits.length - 1] : undefined
}

const placeFrom = (data: Record<string, unknown>) => {
  const parts = [data.country_name, data.country, data.regionName, data.region, data.city]
    .map((item) => (item == null ? '' : String(item).trim()))
    .filter((item, index, all) => item && all.indexOf(item) === index)
  return parts.join(' ')
}

export const parseExit = (body: unknown): { ip: string; place: string } | null => {
  if (body && typeof body === 'object') {
    const data = body as Record<string, unknown>
    const raw = String(data.ip || data.query || data.origin || '')
    const found = raw.match(IP)
    if (!found) return null
    return { ip: found[0], place: placeFrom(data) }
  }
  const text = String(body ?? '').trim()
  if (!text || text.startsWith('<')) return null
  try {
    if (text.startsWith('{')) return parseExit(JSON.parse(text) as unknown)
  } catch {
    // Plain text from myip.ipip.net.
  }
  const labeled = text.match(/IP[:：]\s*((?:\d{1,3}\.){3}\d{1,3})/)
  const loose = text.match(IP)
  const ip = labeled?.[1] || loose?.[0]
  if (!ip) return null
  const from = text.match(/来自于[:：]\s*(.+)/)
  return { ip, place: (from?.[1] || '').trim() }
}

export const siteStatus = (ok: boolean, elapsedMs: number | null): CheckStatus => {
  if (!ok) return 'red'
  if (elapsedMs != null && elapsedMs >= SLOW_MS) return 'amber'
  return 'green'
}

export const formatCoreProxy = (
  endpoint: { schema: string; host: string; port: number; username?: string; password?: string } | null | undefined,
) => {
  if (!endpoint?.port) return ''
  let host = (endpoint.host || '').trim()
  if (!host || host === '0.0.0.0' || host === '::' || host === '[::]') host = '127.0.0.1'
  const user = endpoint.username ? encodeURIComponent(endpoint.username) : ''
  const pass = endpoint.password ? `:${encodeURIComponent(endpoint.password)}` : ''
  const auth = user || endpoint.password ? `${user}${pass}@` : ''
  return `${endpoint.schema}://${auth}${host}:${endpoint.port}`
}

export const selectedNode = (
  proxies: Record<string, { name?: string; type?: string; now?: string }> | null | undefined,
) => {
  if (!proxies) return ''
  const preferred = proxies.proxy || proxies.Proxy || proxies.GLOBAL
  if (preferred?.now) return preferred.now
  const selector = Object.values(proxies).find((item) => {
    const type = (item?.type || '').toLowerCase()
    return (type === 'selector' || type === 'urltest') && !!item.now
  })
  return selector?.now || ''
}

const failedExit = (): ExitCheck => ({ status: 'red', ip: '', place: '' })

const probeExit = async (get: (url: string) => Promise<ProbeResponse>, urls: string[]): Promise<ExitCheck> => {
  for (const url of urls) {
    let response: ProbeResponse
    try {
      response = await get(url)
    } catch (error) {
      response = { ok: false, status: 0, body: '', elapsedMs: 0, error: String(error) }
    }
    if (!response.ok && !response.body) continue
    const parsed = parseExit(response.body)
    if (!parsed) continue
    return { status: parsed.place ? 'green' : 'amber', ip: parsed.ip, place: parsed.place }
  }
  return failedExit()
}

const checkDns = async (lookup: NetworkCheckDeps['lookup']) => {
  for (const host of DNS_HOSTS) {
    try {
      const result = await lookup(host)
      if (result.ok && result.address) return { status: 'green' as const, address: result.address }
    } catch {
      // The next public name is the fallback.
    }
  }
  return { status: 'red' as const, address: '' }
}

export const runNetworkCheck = async (deps: NetworkCheckDeps): Promise<NetworkCheckReport> => {
  const pinned = deps.pinned
  const proxyReady = !!deps.core.proxy
  const siteJobs = SITE_PROBES.map(async (site) => {
    const planned = routeForHost(site.host, pinned)
    // Without a core inbound the request cannot enter the proxy, so the label stays 直连.
    const used: RouteKind = planned === 'proxy' && proxyReady ? 'proxy' : 'direct'
    const get = used === 'proxy' ? deps.proxyGet : deps.directGet
    let response: ProbeResponse
    try {
      response = await get(site.url)
    } catch (error) {
      response = { ok: false, status: 0, body: '', elapsedMs: 0, error: String(error) }
    }
    return { site, used, response }
  })

  const [local, proxy, dns, sites] = await Promise.all([
    probeExit(deps.directGet, LOCAL_EXIT_URLS),
    proxyReady ? probeExit(deps.proxyGet, PROXY_EXIT_URLS) : Promise.resolve(failedExit()),
    checkDns(deps.lookup),
    Promise.all(siteJobs),
  ])

  let connections: ConnectionSnap[] = []
  try {
    connections = await deps.connections()
  } catch {
    connections = []
  }

  return {
    local,
    proxy,
    dns,
    core: {
      running: deps.core.running,
      tun: deps.core.tun,
      mode: deps.core.mode,
      node: deps.core.node,
    },
    sites: sites.map(({ site, used, response }) => {
      const seen = matchConnection(connections, site.host)
      const route = (seen && classifyChains(seen.chains)) || used
      return {
        id: site.id,
        status: siteStatus(response.ok, response.elapsedMs),
        route,
        elapsedMs: response.ok ? response.elapsedMs : null,
      }
    }),
  }
}
