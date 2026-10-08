import { parse } from 'yaml'

import {
  decodeBase64Text,
  normalizeSubscriptionProxies,
  parseShareText,
  type SingboxOutbound,
} from './subscriptionConvert.ts'

export const FREE_NODE_SUB_ID = 'dhagn-free-backup'
export const FREE_NODE_NAME = '免费节点（备用）'
export const FREE_NODE_GROUP = '🆓 免费备用'
export const FREE_NODE_WARNING = '免费节点不安全，勿用于支付/登录'
export const FREE_PARSE_CAP = 100
export const FREE_KEEP = 5
export const FREE_PROBE_CONCURRENCY = 2
export const FREE_REFRESH_MS = 12 * 60 * 60 * 1000
/** A node slower than this is discarded, same as a failed probe. */
export const FREE_SLOW_MS = 5000
export const FREE_TAG_PREFIX = 'free-'

/** Public GitHub aggregators. The list on the subscription can be changed. */
export const FREE_NODE_SOURCES = [
  'https://raw.githubusercontent.com/aiboboxx/v2rayfree/main/v2',
  'https://raw.githubusercontent.com/Pawdroid/Free-servers/main/sub',
]

const GROUP_TYPES = new Set(['selector', 'urltest', 'direct', 'block', 'dns', 'reject', 'shadowsocksr'])

/** Source dialer, DNS, and config keys. They must not override 软件分流. */
const FREE_ROUTING_KEYS = [
  'detour',
  'domain_resolver',
  'domain_strategy',
  'bind_interface',
  'inet4_bind_address',
  'inet6_bind_address',
  'routing_mark',
  'routing-mark',
  'reuse_addr',
  'netns',
  'route',
  'dns',
  'rules',
  'rule_set',
  'inbounds',
  'endpoints',
  'experimental',
  'outbounds',
  'log',
  'dialer-proxy',
  'dialer_proxy',
  'interface-name',
] as const

export const freeNodeSources = (sub: { urls?: string[]; url?: string }) => {
  const listed = (sub.urls || []).map((item) => item.trim()).filter(Boolean)
  if (listed.length) return listed
  return String(sub.url || '')
    .split(/[\s,]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}

export const dueForFreeRefresh = (updateTime: number, now = Date.now()) =>
  !updateTime || now - updateTime >= FREE_REFRESH_MS

export const isProxyOutbound = (item: { type?: string; server?: string; server_port?: number }) => {
  const type = String(item?.type || '').toLowerCase()
  if (!type || GROUP_TYPES.has(type)) return false
  return !!(item.server || item.server_port)
}

const uniqueTag = (tag: string, used: Set<string>) => {
  const cleaned = String(tag || 'node').replace(/\s+/g, ' ').trim() || 'node'
  const base = cleaned.startsWith(FREE_TAG_PREFIX) ? cleaned : FREE_TAG_PREFIX + cleaned
  let name = base
  let index = 2
  while (used.has(name)) {
    name = `${base}-${index}`
    index += 1
  }
  used.add(name)
  return name
}

/** Keep the proxy fields and drop anything that would import the source's routing or DNS. */
export const sanitizeFreeOutbound = (item: {
  type?: string
  tag?: string
  server?: string
  server_port?: number
  [key: string]: unknown
}): SingboxOutbound | undefined => {
  if (!item || typeof item !== 'object' || !isProxyOutbound(item)) return undefined
  const next = { ...item, type: String(item.type), tag: String(item.tag || '') } as SingboxOutbound
  for (const key of FREE_ROUTING_KEYS) delete next[key]
  return next
}

const onlyFreeNodes = (list: SingboxOutbound[]) =>
  list.map((item) => sanitizeFreeOutbound(item)).filter((item): item is SingboxOutbound => !!item)

/** Outbound definitions only. DNS, route rules, and group entries from the source are dropped. */
export const parseFreeSource = (body: string): SingboxOutbound[] => {
  const text = String(body || '').trim()
  if (!text) return []
  if (text.startsWith('{') || text.startsWith('[')) {
    try {
      const parsed = JSON.parse(text) as { outbounds?: unknown; proxies?: unknown }
      const list = Array.isArray(parsed) ? parsed : parsed.outbounds || parsed.proxies
      if (Array.isArray(list)) return onlyFreeNodes(normalizeSubscriptionProxies(list))
    } catch {
      // try the other shapes below
    }
  }
  if (text.includes('proxies:') || text.includes('outbounds:')) {
    try {
      const parsed = parse(text) as { proxies?: unknown; outbounds?: unknown }
      const list = Array.isArray(parsed?.proxies)
        ? parsed.proxies
        : Array.isArray(parsed?.outbounds)
          ? parsed.outbounds
          : null
      if (list) return onlyFreeNodes(normalizeSubscriptionProxies(list as Record<string, any>[]))
    } catch {
      // fall through to share links
    }
  }
  const compact = text.replace(/\s+/g, '')
  if (compact.length > 16 && !text.includes('://')) {
    try {
      return onlyFreeNodes(parseShareText(decodeBase64Text(text)))
    } catch {
      return []
    }
  }
  return onlyFreeNodes(parseShareText(text))
}

export const collectFreeCandidates = (batches: SingboxOutbound[][], cap = FREE_PARSE_CAP) => {
  const seen = new Set<string>()
  const usedTags = new Set<string>()
  const result: SingboxOutbound[] = []
  for (const batch of batches) {
    for (const item of batch) {
      const clean = sanitizeFreeOutbound(item)
      if (!clean) continue
      const key = `${String(clean.type).toLowerCase()}|${clean.server}|${clean.server_port}`
      if (seen.has(key)) continue
      seen.add(key)
      result.push({ ...clean, tag: uniqueTag(String(clean.tag || ''), usedTags) })
      if (result.length >= cap) return result
    }
  }
  return result
}

export interface ProbeResult {
  node: SingboxOutbound
  ok: boolean
  delayMs: number
  /** The core could not run the probe. This is not a dead node. */
  unavailable?: boolean
}

export const isWorkingFreeProbe = (item: ProbeResult) =>
  !item.unavailable && item.ok && item.delayMs > 0 && item.delayMs <= FREE_SLOW_MS

export const rankFreeNodes = (results: ProbeResult[], keep = FREE_KEEP) =>
  results
    .filter(isWorkingFreeProbe)
    .sort((a, b) => a.delayMs - b.delayMs || a.node.tag.localeCompare(b.node.tag))
    .slice(0, keep)
    .map((item) => sanitizeFreeOutbound(item.node))
    .filter((item): item is SingboxOutbound => !!item)

/**
 * A real retest stores only the fastest working nodes.
 * A kept node that failed or is too slow is dropped, and the next working
 * candidate fills that slot. If the probe never ran, the previous nodes stay.
 */
export const decideFreeList = (previous: SingboxOutbound[], results: ProbeResult[], keep = FREE_KEEP) => {
  const keptBefore = previous
    .map((item) => sanitizeFreeOutbound(item))
    .filter((item): item is SingboxOutbound => !!item)
    .slice(0, keep)
  const tested = results.some((item) => !item.unavailable)
  if (!tested) return { replace: false as const, proxies: keptBefore }
  return { replace: true as const, proxies: rankFreeNodes(results, keep) }
}

/** Free node tags are visible only inside their own group, and only when kept. */
export const freeNodeVisibleInGroup = (group: string, proxy: string, kept: ReadonlySet<string>) => {
  if (proxy === FREE_NODE_GROUP) return group !== FREE_NODE_GROUP && kept.size > 0
  const owned = proxy.startsWith(FREE_TAG_PREFIX) || kept.has(proxy)
  if (!owned) return true
  return group === FREE_NODE_GROUP && kept.has(proxy)
}

interface GeneratedOutbound {
  type?: string
  tag?: string
  outbounds?: string[]
  default?: string
  [key: string]: unknown
}

const AUTOMATIC_GROUP = /自动选择|漏网之鱼|urltest|fallback|loadbalance/i

/** URLTest, fallback, and load-balance pick a member by themselves. */
export const isAutomaticGroup = (item: { type?: string; tag?: string }) => {
  const type = String(item.type || '').toLowerCase()
  if (type === 'urltest' || type === 'fallback' || type === 'loadbalance') return true
  return AUTOMATIC_GROUP.test(String(item.tag || ''))
}

/** Only the manual 节点选择 selector may offer the free group, and never as its default. */
const isManualNodeSelect = (item: { type?: string; tag?: string }) =>
  String(item.type || '').toLowerCase() === 'selector' &&
  String(item.tag || '').includes('节点选择') &&
  !isAutomaticGroup(item)

/**
 * The five nodes live only in their own selector.
 * That selector is one manual entry on 节点选择. It is never a member of a
 * URLTest, fallback, or load-balance group, and it does not test or fail over by itself.
 */
export const attachFreeGroup = <T extends GeneratedOutbound>(outbounds: T[], nodes: Array<{ tag?: string }>): T[] => {
  const tags = nodes.map((node) => String(node.tag || '')).filter(Boolean).slice(0, FREE_KEEP)
  const banned = new Set([...tags, FREE_NODE_GROUP])
  const next = outbounds
    .filter((item) => item.tag !== FREE_NODE_GROUP && !tags.includes(String(item.tag || '')))
    .map((item) => {
      if (!Array.isArray(item.outbounds)) return { ...item }
      const kept = item.outbounds.filter((tag) => !banned.has(tag))
      const manualOption = isManualNodeSelect(item) && tags.length > 0
      const listed = manualOption ? [...kept, FREE_NODE_GROUP] : kept
      const currentDefault = String(item.default || '')
      const safeDefault = currentDefault && !banned.has(currentDefault) ? currentDefault : listed[0] || ''
      const copy = { ...item, outbounds: listed }
      if (currentDefault && banned.has(currentDefault)) {
        copy.default = safeDefault || undefined
      }
      return copy
    })
  if (!tags.length) return next
  return [
    ...next,
    ...(nodes.slice(0, FREE_KEEP) as T[]),
    {
      type: 'selector',
      tag: FREE_NODE_GROUP,
      outbounds: tags,
      interrupt_exist_connections: true,
    } as unknown as T,
  ]
}

export const expandsFreeSubscription = (subId: string) => subId === FREE_NODE_SUB_ID

export const freeNodeSubscription = (): App.Subscription => ({
  id: FREE_NODE_SUB_ID,
  name: FREE_NODE_NAME,
  upload: 0,
  download: 0,
  total: 0,
  expire: 0,
  updateTime: 0,
  type: 'Http',
  url: FREE_NODE_SOURCES.join('\n'),
  urls: [...FREE_NODE_SOURCES],
  website: '',
  path: `data/subscribes/${FREE_NODE_SUB_ID}.json`,
  include: '',
  exclude: '',
  includeProtocol: '',
  excludeProtocol: 'direct|reject|selector|urltest|block|dns|shadowsocksr',
  proxyPrefix: '',
  requestProxyMode: 'none' as App.Subscription['requestProxyMode'],
  customProxy: '',
  disabled: true,
  inSecure: false,
  proxies: [],
  requestMethod: 'GET' as App.Subscription['requestMethod'],
  requestTimeout: 20,
  header: {
    request: { 'User-Agent': 'clash.meta/mihomo' },
    response: {},
  },
  script: 'const onSubscribe = async (proxies, subscription) => {\n  return { proxies, subscription }\n}',
})

export const ensureFreeSubscription = (subscribes: App.Subscription[]) => {
  if (subscribes.some((item) => item.id === FREE_NODE_SUB_ID)) return false
  subscribes.push(freeNodeSubscription())
  return true
}
