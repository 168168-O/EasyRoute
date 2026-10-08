/**
 * Per-app routing for 软件分流.
 *
 * Unlisted programs go direct. Only programs explicitly set to 走代理 use the
 * proxy outbound. WeChat and Douyin stay at the front of the rule list so they
 * win over global mode, user profiles, mixins and scripts. Each one is either
 * 走本地 or 走代理, default 走本地. The choice comes from user.yaml and is not
 * inferred from subscriptions or the current clash mode.
 */

export type AppRouteMode = 'proxy' | 'direct'

export interface RoutedProgram {
  id: string
  name: string
  exe: string
  mode: AppRouteMode
}

export interface PinnedRoutes {
  wechat: AppRouteMode
  douyin: AppRouteMode
}

export interface AppRoutingOptions {
  programs?: RoutedProgram[]
  /** Extra process names (for example Douyin exes discovered on disk). */
  extraDirectExes?: string[]
  /**
   * Saved WeChat / Douyin choice. Missing or unknown values stay 走本地.
   * A saved 走代理 is never rewritten to 走本地 here.
   */
  pinnedRoutes?: Partial<PinnedRoutes> | null
  proxyOutbound?: string
  directOutbound?: string
  /** Full path of this app, from envStore.env.appPath. */
  appPath?: string
}

export const WECHAT_PROCESSES = [
  'Weixin.exe',
  'WeixinExt.exe',
  'WeixinUpdate.exe',
  'WeChat.exe',
  'WeChatAppEx.exe',
  'WeChatPlayer.exe',
  'WeChatOCR.exe',
  'WeChatUtility.exe',
  'WxgameDaemon.exe',
]

export const DOUYIN_PROCESSES = [
  'douyin.exe',
  'douyin_guard.exe',
  'douyin_tray.exe',
  'douyin_widget.exe',
  'douyin_launcher.exe',
  'douyin_doctor.exe',
]

/** Matches any exe under the Douyin install directory, including ones not listed above. */
export const DOUYIN_PATH_REGEX = String.raw`(?i)ByteDance[/\\]douyin[/\\].*\.exe$`

export const DOUYIN_DOMAIN_SUFFIXES = [
  'douyin.com',
  'douyinpic.com',
  'douyinvod.com',
  'douyincdn.com',
  'douyinstatic.com',
  'amemv.com',
  'snssdk.com',
  'zijieapi.com',
  'bytedance.com',
  'byteimg.com',
]

export const WECHAT_DOMAIN_SUFFIXES = [
  'weixin.qq.com',
  'wechat.com',
  'qq.com',
  'qpic.cn',
  'qlogo.cn',
]

export const SAFETY_DOMAIN_SUFFIXES = [...DOUYIN_DOMAIN_SUFFIXES, ...WECHAT_DOMAIN_SUFFIXES]

const routeMode = (value: unknown): AppRouteMode => (value === 'proxy' ? 'proxy' : 'direct')

/** Keep a saved 走代理. Anything missing or invalid stays 走本地. */
export const normalizePinnedRoutes = (value: unknown): PinnedRoutes => {
  const record = value && typeof value === 'object' ? (value as Record<string, unknown>) : {}
  return {
    wechat: routeMode(record.wechat),
    douyin: routeMode(record.douyin),
  }
}

export const LOCAL_DNS_TAG = 'dhagn-local-dns'

const DIRECT_PROCESS_CATCHALL = '.+'

const lower = (value: string) => value.trim().toLowerCase()

const normalizePath = (value: string) => lower(value).replaceAll('/', '\\')

const appExeName = (appPath: string) => appPath.trim().split(/[/\\]/).pop() || ''

export const isPinnedExe = (exe: string) => {
  const name = lower(exe)
  return (
    WECHAT_PROCESSES.some((item) => lower(item) === name) ||
    DOUYIN_PROCESSES.some((item) => lower(item) === name)
  )
}

const asList = (value: unknown): string[] => {
  if (Array.isArray(value)) return value.map((item) => String(item))
  if (typeof value === 'string' && value) return [value]
  return []
}

const unique = (names: string[]) => {
  const seen = new Set<string>()
  const result: string[] = []
  for (const name of names) {
    const key = lower(name)
    if (!key || seen.has(key)) continue
    seen.add(key)
    result.push(name.trim())
  }
  return result
}

const ruleKey = (rule: Record<string, unknown>) => JSON.stringify(rule)

export interface ConnectionQuery {
  processName?: string
  processPath?: string
  domain?: string
  ipIsPrivate?: boolean
  inbound?: string
  clashMode?: 'global' | 'rule' | 'direct'
}

export interface DnsQuery {
  processName?: string
  domain?: string
  queryType?: string
  clashMode?: 'global' | 'rule' | 'direct'
}

/** Same exclusions as the app's own Fake-IP DNS rule, plus the bare localhost name. */
export const FAKEIP_EXCLUDED_SUFFIXES = [
  '.lan',
  '.localdomain',
  '.example',
  '.invalid',
  '.localhost',
  'localhost',
  '.test',
  '.local',
  '.home.arpa',
  '.msftconnecttest.com',
  '.msftncsi.com',
]

const nameHit = (rule: Record<string, unknown>, processName: string) => {
  const names = asList(rule.process_name).map(lower)
  if (!names.length || !processName) return false
  const hit = names.includes(lower(processName))
  return rule.invert ? !hit : hit
}

/**
 * First matching outbound for a connection. Used by tests and the sample config.
 * Sniff / hijack actions do not select an outbound.
 */
export const matchOutbound = (rules: Record<string, unknown>[], query: ConnectionQuery) => {
  const clashMode = query.clashMode || 'rule'
  for (const rule of rules) {
    if (!rule || typeof rule !== 'object') continue
    if (rule.clash_mode && String(rule.clash_mode) !== clashMode) continue
    const action = rule.action
    if (action === 'sniff' || action === 'hijack-dns') continue

    if (rule.inbound) {
      const tags = asList(rule.inbound).map(String)
      if (!query.inbound || !tags.includes(query.inbound)) continue
      const hasMore =
        rule.process_name ||
        rule.process_path ||
        rule.process_path_regex ||
        rule.domain_suffix ||
        rule.ip_is_private === true ||
        rule.clash_mode ||
        rule.rule_set
      if (!hasMore && rule.outbound) return rule.outbound as string
    }

    if (rule.process_name && query.processName) {
      if (nameHit(rule, query.processName)) return rule.outbound as string | undefined
      continue
    }
    if (rule.process_path) {
      const target = query.processPath || ''
      if (!target) continue
      const wanted = asList(rule.process_path).map((item) => normalizePath(String(item)))
      if (wanted.includes(normalizePath(target))) return rule.outbound as string | undefined
      continue
    }
    if (rule.process_path_regex) {
      const target = query.processPath || query.processName || ''
      if (!target) continue
      let matched = false
      try {
        // sing-box uses Go regex, where (?i) is an inline flag. JavaScript rejects it.
        const source = String(rule.process_path_regex).replace(/^\(\?[imsU]+\)/, '')
        matched = new RegExp(source, 'i').test(target)
      } catch {
        matched = false
      }
      if (matched) return rule.outbound as string | undefined
      continue
    }
    if (rule.domain_suffix && query.domain) {
      const domain = lower(query.domain)
      const hit = asList(rule.domain_suffix).some(
        (suffix) => domain === lower(suffix) || domain.endsWith('.' + lower(suffix)),
      )
      if (hit) return rule.outbound as string | undefined
      continue
    }
    if (rule.ip_is_private === true && query.ipIsPrivate) {
      return rule.outbound as string | undefined
    }
    if (rule.clash_mode && String(rule.clash_mode) === clashMode && rule.outbound) {
      return rule.outbound as string
    }
  }
  return undefined
}

const suffixHit = (suffixes: string[], domain: string) => {
  const value = lower(domain)
  return suffixes.some((suffix) => {
    const item = lower(suffix)
    const bare = item.startsWith('.') ? item.slice(1) : item
    const withDot = item.startsWith('.') ? item : '.' + item
    return value === item || value === bare || value.endsWith(withDot)
  })
}

const matchesDnsRule = (rule: Record<string, any>, query: DnsQuery, queryType: string): boolean => {
  if (!rule || typeof rule !== 'object') return false
  if (rule.type === 'logical') {
    const parts = Array.isArray(rule.rules) ? (rule.rules as Record<string, any>[]) : []
    const results = parts.map((part) => matchesDnsRule(part, query, queryType))
    return String(rule.mode || 'and') === 'or' ? results.some(Boolean) : results.every(Boolean)
  }
  if (rule.clash_mode && String(rule.clash_mode) !== (query.clashMode || 'rule')) return false
  if (rule.rule_set) return false
  if (rule.process_name && (!query.processName || !nameHit(rule, query.processName))) return false
  if (rule.query_type) {
    const types = asList(rule.query_type).map((item) => String(item).toUpperCase())
    if (!types.includes(queryType)) return false
  }
  if (rule.domain_suffix) {
    if (!query.domain) return false
    const hit = suffixHit(asList(rule.domain_suffix), query.domain)
    if (rule.invert ? hit : !hit) return false
  }
  return Boolean(rule.process_name || rule.query_type || rule.domain_suffix || rule.clash_mode)
}

/** First matching DNS server tag. rule_set rules are left to sing-box. */
export const matchDnsServer = (rules: Record<string, unknown>[], query: DnsQuery) => {
  const queryType = (query.queryType || 'A').toUpperCase()
  for (const rule of rules) {
    if (!rule || typeof rule !== 'object') continue
    if (rule.action === 'sniff' || rule.action === 'hijack-dns') continue
    if (matchesDnsRule(rule as Record<string, any>, query, queryType)) {
      return rule.server as string | undefined
    }
  }
  return undefined
}

const findProxyOutbound = (config: Record<string, any>, explicit?: string) => {
  if (explicit) return explicit
  const rules = (config.route?.rules || []) as Record<string, any>[]
  const globalRule = rules.find((rule) => rule.clash_mode === 'global' && rule.outbound)
  if (globalRule?.outbound && globalRule.outbound !== 'direct') return String(globalRule.outbound)
  // After the first apply, clash global is retargeted to direct. The proxy
  // process rule still names the outbound, so a second apply stays stable.
  const processRule = rules.find(
    (rule) =>
      rule.action === 'route' &&
      rule.outbound &&
      rule.outbound !== 'direct' &&
      (rule.process_name || rule.process_path || rule.inbound),
  )
  if (processRule?.outbound) return String(processRule.outbound)
  const selector = ((config.outbounds || []) as Record<string, any>[]).find(
    (outbound) => outbound.type === 'selector' && outbound.tag && outbound.tag !== 'GLOBAL',
  )
  return selector?.tag ? String(selector.tag) : 'proxy'
}

const findDirectOutbound = (config: Record<string, any>, explicit?: string) => {
  if (explicit) return explicit
  const direct = ((config.outbounds || []) as Record<string, any>[]).find(
    (outbound) => outbound.type === 'direct' && outbound.tag,
  )
  return direct?.tag ? String(direct.tag) : 'direct'
}

const ensureDirectOutbound = (config: Record<string, any>, tag: string) => {
  const outbounds = (config.outbounds || (config.outbounds = [])) as Record<string, any>[]
  if (!outbounds.some((outbound) => outbound.tag === tag || outbound.type === 'direct')) {
    outbounds.push({ type: 'direct', tag })
  }
}

const ensureTun = (config: Record<string, any>) => {
  const inbounds = (config.inbounds || (config.inbounds = [])) as Record<string, any>[]
  let tun = inbounds.find((inbound) => inbound.type === 'tun')
  if (!tun) {
    tun = {
      type: 'tun',
      tag: 'tun-in',
      address: ['172.18.0.1/30', 'fdfe:dcba:9876::1/126'],
      endpoint_independent_nat: true,
    }
    inbounds.push(tun)
  }
  // Per-process rules only see traffic that the TUN actually captures.
  tun.auto_route = true
  tun.strict_route = true
  delete tun.disabled
  config.route = config.route || {}
  config.route.find_process = true
  if (config.route.auto_detect_interface === undefined) {
    config.route.auto_detect_interface = true
  }
}

const ensureLocalDns = (config: Record<string, any>) => {
  config.dns = config.dns || {}
  const servers = (config.dns.servers || (config.dns.servers = [])) as Record<string, any>[]
  if (!servers.some((server) => server.tag === LOCAL_DNS_TAG)) {
    servers.unshift({ type: 'local', tag: LOCAL_DNS_TAG })
  }
  return LOCAL_DNS_TAG
}

const findRemoteDns = (config: Record<string, any>) => {
  const rules = (config.dns?.rules || []) as Record<string, any>[]
  const globalRule = rules.find((rule) => rule.clash_mode === 'global' && rule.server)
  if (globalRule?.server && globalRule.server !== LOCAL_DNS_TAG) return String(globalRule.server)
  const remote = ((config.dns?.servers || []) as Record<string, any>[]).find(
    (server) =>
      server.tag &&
      server.tag !== LOCAL_DNS_TAG &&
      server.type !== 'fakeip' &&
      (server.detour || /remote/i.test(String(server.tag))),
  )
  return remote?.tag ? String(remote.tag) : undefined
}

const findFakeIpDns = (config: Record<string, any>) => {
  const fake = ((config.dns?.servers || []) as Record<string, any>[]).find(
    (server) => server.type === 'fakeip' && server.tag,
  )
  return fake?.tag ? String(fake.tag) : undefined
}

/** CN geosite tags such as geosite-cn / GeoSite-CN. Skips geolocation-!cn. */
const isCnGeosite = (tag: string) => {
  const value = tag.toLowerCase()
  if (!value || value.includes('!')) return false
  if (!/geo[-_]?site/.test(value)) return false
  return /(^|[^a-z])cn([^a-z]|$)/.test(value)
}

const collectCnGeositeTags = (config: Record<string, any>) => {
  const tags: string[] = []
  const push = (value: unknown) => {
    for (const tag of asList(value)) {
      if (!isCnGeosite(tag)) continue
      if (tags.some((item) => lower(item) === lower(tag))) continue
      tags.push(tag)
    }
  }
  const rules = [
    ...((config.dns?.rules || []) as Record<string, any>[]),
    ...((config.route?.rules || []) as Record<string, any>[]),
  ]
  for (const rule of rules) {
    if (rule && typeof rule === 'object') push(rule.rule_set)
  }
  const sets = config.route?.rule_set
  if (Array.isArray(sets)) {
    for (const set of sets) {
      if (typeof set === 'string') push(set)
      else if (set && typeof set === 'object') push(set.tag)
    }
  }
  return tags
}

const findProxyInboundTags = (config: Record<string, any>) => {
  const tags: string[] = []
  for (const inbound of (config.inbounds || []) as Record<string, any>[]) {
    if (!inbound || inbound.disabled || !inbound.tag) continue
    if (inbound.type !== 'mixed' && inbound.type !== 'http') continue
    const tag = String(inbound.tag)
    if (!tags.includes(tag)) tags.push(tag)
  }
  return tags
}

const sameSet = (left: string[], right: string[]) => {
  if (left.length !== right.length) return false
  const keys = new Set(left.map(lower))
  return right.every((item) => keys.has(lower(item)))
}

const ownedProcessList = (names: string[], lists: string[][]) =>
  lists.some((list) => list.length > 0 && sameSet(names, list))

const buildPrefix = (
  programs: RoutedProgram[],
  extraDirectExes: string[],
  pinnedRoutes: PinnedRoutes,
  proxyOutbound: string,
  directOutbound: string,
  proxyInboundTags: string[],
  appPath: string,
) => {
  const wechat = unique(WECHAT_PROCESSES)
  const extraDouyin = unique(extraDirectExes).filter(
    (exe) =>
      !DOUYIN_PROCESSES.some((item) => lower(item) === lower(exe)) &&
      !WECHAT_PROCESSES.some((item) => lower(item) === lower(exe)),
  )
  const douyinNames = unique([...DOUYIN_PROCESSES, ...extraDouyin])
  const pinnedSet = new Set([...wechat, ...douyinNames].map(lower))
  const wechatOutbound = pinnedRoutes.wechat === 'proxy' ? proxyOutbound : directOutbound
  const douyinOutbound = pinnedRoutes.douyin === 'proxy' ? proxyOutbound : directOutbound
  const proxyExes = unique(
    programs
      .filter((program) => program.mode === 'proxy' && program.exe && !isPinnedExe(program.exe))
      .map((program) => program.exe)
      .filter((exe) => !pinnedSet.has(lower(exe))),
  )
  const directExes = unique(
    programs
      .filter((program) => program.mode === 'direct' && program.exe && !isPinnedExe(program.exe))
      .map((program) => program.exe)
      .filter((exe) => !pinnedSet.has(lower(exe))),
  )
  const pinnedProxyExes = unique([
    ...(pinnedRoutes.wechat === 'proxy' ? wechat : []),
    ...(pinnedRoutes.douyin === 'proxy' ? douyinNames : []),
  ])
  const pinnedDirectExes = unique([
    ...(pinnedRoutes.wechat === 'direct' ? wechat : []),
    ...(pinnedRoutes.douyin === 'direct' ? douyinNames : []),
  ])
  const dnsProxyExes = unique([...proxyExes, ...pinnedProxyExes])
  const directDomains = SAFETY_DOMAIN_SUFFIXES.filter((suffix) =>
    (WECHAT_DOMAIN_SUFFIXES.includes(suffix) ? pinnedRoutes.wechat : pinnedRoutes.douyin) === 'direct',
  )
  const proxyDomains = SAFETY_DOMAIN_SUFFIXES.filter((suffix) =>
    (WECHAT_DOMAIN_SUFFIXES.includes(suffix) ? pinnedRoutes.wechat : pinnedRoutes.douyin) === 'proxy',
  )

  const rules: Record<string, unknown>[] = [
    { action: 'route', process_name: wechat, outbound: wechatOutbound },
    { action: 'route', process_name: douyinNames, outbound: douyinOutbound },
    { action: 'route', process_path_regex: DOUYIN_PATH_REGEX, outbound: douyinOutbound },
    { action: 'sniff' },
  ]
  if (proxyDomains.length) {
    rules.push({ action: 'route', domain_suffix: proxyDomains, outbound: proxyOutbound })
  }
  if (directDomains.length) {
    rules.push({ action: 'route', domain_suffix: directDomains, outbound: directOutbound })
  }
  rules.push({ action: 'route', ip_is_private: true, outbound: directOutbound })

  if (dnsProxyExes.length) {
    rules.push({
      action: 'hijack-dns',
      protocol: 'dns',
      process_name: dnsProxyExes,
    })
  }
  if (proxyExes.length) {
    rules.push({ action: 'route', process_name: proxyExes, outbound: proxyOutbound })
  }

  // Only this app's own requests on the mixed/http inbound go to the proxy
  // (the Google fallback). Every other program on that inbound follows the
  // same per-app rules as TUN. process_name covers a missing process path.
  const exe = appExeName(appPath)
  if (proxyInboundTags.length && appPath.trim()) {
    rules.push({
      action: 'route',
      inbound: [...proxyInboundTags],
      process_path: [appPath],
      outbound: proxyOutbound,
    })
    if (exe) {
      rules.push({
        action: 'route',
        inbound: [...proxyInboundTags],
        process_name: [exe],
        outbound: proxyOutbound,
      })
    }
  }

  // Any remaining program (unlisted, or explicitly 走本地) stays on the local network.
  rules.push({
    action: 'route',
    process_path_regex: DIRECT_PROCESS_CATCHALL,
    outbound: directOutbound,
  })

  return {
    rules,
    proxyExes,
    directExes,
    douyinNames,
    wechat,
    pinnedProxyExes,
    pinnedDirectExes,
    dnsProxyExes,
    directDomains,
    proxyDomains,
  }
}

const stripInjected = (existing: Record<string, unknown>[], injected: Record<string, unknown>[]) => {
  const keys = new Set(injected.map((rule) => ruleKey(rule)))
  return existing.filter((rule) => !keys.has(ruleKey(rule)))
}

/** Drop an earlier WeChat/Douyin rule even if its outbound changed, so a new choice cannot be shadowed. */
const isPinnedRouteRule = (rule: Record<string, unknown>, douyinNames: string[]) => {
  if (!rule || rule.inbound || rule.clash_mode || rule.rule_set) return false
  if (rule.action && rule.action !== 'route') return false
  if (
    rule.process_path_regex === DOUYIN_PATH_REGEX &&
    !rule.process_name &&
    !rule.domain_suffix &&
    !rule.process_path
  ) {
    return true
  }
  const proc = asList(rule.process_name)
  if (proc.length && !rule.domain_suffix && !rule.process_path && !rule.process_path_regex) {
    return ownedProcessList(proc, [WECHAT_PROCESSES, DOUYIN_PROCESSES, douyinNames])
  }
  const suffixes = asList(rule.domain_suffix)
  if (suffixes.length && !rule.process_name && !rule.process_path) {
    return [WECHAT_DOMAIN_SUFFIXES, DOUYIN_DOMAIN_SUFFIXES, SAFETY_DOMAIN_SUFFIXES].some(
      (list) => sameSet(suffixes, list),
    )
  }
  return false
}

const isPinnedDnsRule = (rule: Record<string, unknown>, douyinNames: string[], extraDirectExes: string[]) => {
  if (!rule || rule.inbound || rule.clash_mode || rule.rule_set || rule.type === 'logical') return false
  if (rule.action && rule.action !== 'route') return false
  const proc = asList(rule.process_name)
  if (proc.length && !rule.domain_suffix) {
    return ownedProcessList(proc, [
      WECHAT_PROCESSES,
      DOUYIN_PROCESSES,
      douyinNames,
      unique([...WECHAT_PROCESSES, ...DOUYIN_PROCESSES, ...extraDirectExes]),
    ])
  }
  const suffixes = asList(rule.domain_suffix)
  if (suffixes.length && !rule.process_name) {
    return [WECHAT_DOMAIN_SUFFIXES, DOUYIN_DOMAIN_SUFFIXES, SAFETY_DOMAIN_SUFFIXES].some((list) =>
      sameSet(suffixes, list),
    )
  }
  return false
}

/**
 * Prepend safety and per-app rules, enable TUN + find_process, and split DNS.
 * Unidentified connections use the direct outbound. Safe to call more than once.
 */
export const applyAppRouting = (config: Record<string, any>, options: AppRoutingOptions = {}) => {
  const programs = options.programs || []
  const extraDirectExes = options.extraDirectExes || []
  const pinnedRoutes = normalizePinnedRoutes(options.pinnedRoutes)
  const proxyOutbound = findProxyOutbound(config, options.proxyOutbound)
  const directOutbound = findDirectOutbound(config, options.directOutbound)
  ensureDirectOutbound(config, directOutbound)
  ensureTun(config)

  const proxyInboundTags = findProxyInboundTags(config)
  const built = buildPrefix(
    programs,
    extraDirectExes,
    pinnedRoutes,
    proxyOutbound,
    directOutbound,
    proxyInboundTags,
    options.appPath || '',
  )
  const { rules: prefix, proxyExes, directExes, douyinNames, pinnedDirectExes, dnsProxyExes, directDomains, proxyDomains } =
    built

  const route = (config.route = config.route || {})
  const existing = (Array.isArray(route.rules) ? route.rules : []) as Record<string, unknown>[]
  const withoutPinned = existing.filter((rule) => !isPinnedRouteRule(rule, douyinNames))
  const merged = [...prefix, ...stripInjected(withoutPinned, prefix)]
  // Connections with no process path skip every process rule and would otherwise
  // hit clash_mode global → proxy. Send those to direct. Named 走代理 processes
  // still match their earlier process rule.
  route.rules = merged.map((rule) => {
    if (rule?.clash_mode === 'global' && rule.outbound && rule.outbound !== directOutbound) {
      return { ...rule, outbound: directOutbound }
    }
    return rule
  })
  route.final = directOutbound
  route.find_process = true
  route.default_domain_resolver = { server: ensureLocalDns(config) }

  const localDns = LOCAL_DNS_TAG
  const remoteDns = findRemoteDns(config)
  const fakeIpDns = findFakeIpDns(config)
  const cnTags = collectCnGeositeTags(config)
  const dnsRulesBefore = (Array.isArray(config.dns?.rules) ? config.dns.rules : []) as Record<string, any>[]
  const localServerFor = (tag: string) => {
    const current = dnsRulesBefore.find((rule) => asList(rule.rule_set).some((item) => lower(item) === lower(tag)))
    const server = current?.server ? String(current.server) : ''
    if (server && server !== remoteDns && server !== fakeIpDns) return server
    return localDns
  }

  const dnsPrefix: Record<string, unknown>[] = []
  if (proxyDomains.length && remoteDns) {
    dnsPrefix.push({ action: 'route', domain_suffix: proxyDomains, server: remoteDns })
  }
  if (directDomains.length) {
    dnsPrefix.push({ action: 'route', domain_suffix: directDomains, server: localDns })
  }
  if (pinnedDirectExes.length) {
    dnsPrefix.push({ action: 'route', process_name: pinnedDirectExes, server: localDns })
  }
  if (directExes.length) {
    dnsPrefix.push({ action: 'route', process_name: directExes, server: localDns })
  }
  for (const tag of cnTags) {
    dnsPrefix.push({ action: 'route', rule_set: tag, server: localServerFor(tag) })
  }
  if (dnsProxyExes.length && fakeIpDns) {
    dnsPrefix.push({
      action: 'route',
      server: fakeIpDns,
      type: 'logical',
      mode: 'and',
      rules: [
        { process_name: dnsProxyExes },
        { query_type: ['A', 'AAAA'] },
        { domain_suffix: [...FAKEIP_EXCLUDED_SUFFIXES], invert: true },
      ],
    })
  }
  if (dnsProxyExes.length && remoteDns) {
    dnsPrefix.push({ action: 'route', process_name: dnsProxyExes, server: remoteDns })
  }

  const dns = (config.dns = config.dns || {})
  const dnsRules = (Array.isArray(dns.rules) ? dns.rules : []) as Record<string, unknown>[]
  const withoutPinnedDns = dnsRules.filter((rule) => !isPinnedDnsRule(rule, douyinNames, extraDirectExes))
  dns.rules = [...dnsPrefix, ...stripInjected(withoutPinnedDns, dnsPrefix)]
  // Leftover queries (Windows system DNS, non-CN) use the remote resolver.
  // CN, safety domains, WeChat/Douyin and explicit 走本地 processes already matched above.
  if (remoteDns) dns.final = remoteDns

  return config
}

/** A small fake profile used by tests and the sample config in the pull request. */
export const sampleBaseConfig = () => ({
  log: { level: 'info' },
  inbounds: [
    {
      type: 'mixed',
      tag: 'mixed-in',
      listen: '127.0.0.1',
      listen_port: 20122,
    },
  ],
  outbounds: [
    { type: 'selector', tag: 'proxy', outbounds: ['anytls-hk'] },
    { type: 'anytls', tag: 'anytls-hk', server: 'example.invalid', server_port: 443, password: 'fake-password' },
    { type: 'direct', tag: 'direct' },
    { type: 'block', tag: 'block' },
  ],
  route: {
    rules: [
      { action: 'sniff', inbound: 'tun-in' },
      { action: 'hijack-dns', protocol: 'dns' },
      { action: 'route', clash_mode: 'direct', outbound: 'direct' },
      { action: 'route', clash_mode: 'global', outbound: 'proxy' },
      { action: 'route', rule_set: 'geosite-cn', outbound: 'direct' },
      { action: 'route', rule_set: 'geoip-private', outbound: 'direct' },
    ],
    final: 'proxy',
    auto_detect_interface: true,
    find_process: false,
  },
  dns: {
    servers: [
      { type: 'https', tag: 'Remote-DNS', server: '8.8.8.8', server_port: 443, detour: 'proxy' },
      { type: 'https', tag: 'Local-DNS', server: '223.5.5.5', server_port: 443 },
    ],
    rules: [
      { action: 'route', clash_mode: 'direct', server: 'Local-DNS' },
      { action: 'route', clash_mode: 'global', server: 'Remote-DNS' },
      { action: 'route', rule_set: 'geosite-cn', server: 'Local-DNS' },
    ],
    final: 'Remote-DNS',
  },
})
