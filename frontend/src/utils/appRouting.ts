/**
 * Per-app routing for 软件分流.
 *
 * Unlisted programs go direct. Only programs explicitly set to 走代理 use the
 * proxy outbound. WeChat and Douyin are pinned direct and their rules are
 * prepended so they win over global mode, user profiles, mixins and scripts.
 */

export type AppRouteMode = 'proxy' | 'direct'

export interface RoutedProgram {
  id: string
  name: string
  exe: string
  mode: AppRouteMode
}

export interface AppRoutingOptions {
  programs?: RoutedProgram[]
  /** Extra process names (for example Douyin exes discovered on disk). */
  extraDirectExes?: string[]
  proxyOutbound?: string
  directOutbound?: string
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

export const SAFETY_DOMAIN_SUFFIXES = [
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
  'weixin.qq.com',
  'wechat.com',
  'qq.com',
  'qpic.cn',
  'qlogo.cn',
]

export const LOCAL_DNS_TAG = 'dhagn-local-dns'

const DIRECT_PROCESS_CATCHALL = '.+'

const lower = (value: string) => value.trim().toLowerCase()

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
  clashMode?: 'global' | 'rule' | 'direct'
}

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

    if (rule.process_name && query.processName) {
      if (nameHit(rule, query.processName)) return rule.outbound as string | undefined
      continue
    }
    if (rule.process_path_regex) {
      const target = query.processPath || query.processName || ''
      if (!target) continue
      let matched = false
      try {
        matched = new RegExp(String(rule.process_path_regex), 'i').test(target)
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
      (rule.process_name || rule.process_path),
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

const buildPrefix = (
  programs: RoutedProgram[],
  extraDirectExes: string[],
  proxyOutbound: string,
  directOutbound: string,
) => {
  const pinned = unique([...WECHAT_PROCESSES, ...DOUYIN_PROCESSES, ...extraDirectExes])
  const pinnedSet = new Set(pinned.map(lower))
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

  const wechat = unique(WECHAT_PROCESSES)
  const extraDouyin = unique(extraDirectExes).filter(
    (exe) =>
      !DOUYIN_PROCESSES.some((item) => lower(item) === lower(exe)) &&
      !WECHAT_PROCESSES.some((item) => lower(item) === lower(exe)),
  )
  const douyinNames = unique([...DOUYIN_PROCESSES, ...extraDouyin])

  const rules: Record<string, unknown>[] = [
    { action: 'route', process_name: wechat, outbound: directOutbound },
    { action: 'route', process_name: douyinNames, outbound: directOutbound },
    { action: 'route', process_path_regex: DOUYIN_PATH_REGEX, outbound: directOutbound },
    { action: 'sniff' },
    { action: 'route', domain_suffix: [...SAFETY_DOMAIN_SUFFIXES], outbound: directOutbound },
    { action: 'route', ip_is_private: true, outbound: directOutbound },
  ]

  if (proxyExes.length) {
    rules.push({
      action: 'hijack-dns',
      protocol: 'dns',
      process_name: proxyExes,
    })
    rules.push({ action: 'route', process_name: proxyExes, outbound: proxyOutbound })
  }

  // Any remaining program (unlisted, or explicitly 走本地) stays on the local network.
  rules.push({
    action: 'route',
    process_path_regex: DIRECT_PROCESS_CATCHALL,
    outbound: directOutbound,
  })

  return { rules, proxyExes, directExes, douyinNames, wechat }
}

const stripInjected = (existing: Record<string, unknown>[], injected: Record<string, unknown>[]) => {
  const keys = new Set(injected.map((rule) => ruleKey(rule)))
  return existing.filter((rule) => !keys.has(ruleKey(rule)))
}

/**
 * Prepend safety and per-app rules, enable TUN + find_process, and split DNS.
 * Unidentified connections use the direct outbound. Safe to call more than once.
 */
export const applyAppRouting = (config: Record<string, any>, options: AppRoutingOptions = {}) => {
  const programs = options.programs || []
  const proxyOutbound = findProxyOutbound(config, options.proxyOutbound)
  const directOutbound = findDirectOutbound(config, options.directOutbound)
  ensureDirectOutbound(config, directOutbound)
  ensureTun(config)

  const { rules: prefix, proxyExes, directExes } = buildPrefix(
    programs,
    options.extraDirectExes || [],
    proxyOutbound,
    directOutbound,
  )

  const route = (config.route = config.route || {})
  const existing = (Array.isArray(route.rules) ? route.rules : []) as Record<string, unknown>[]
  const merged = [...prefix, ...stripInjected(existing, prefix)]
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

  const dnsPrefix: Record<string, unknown>[] = [
    { action: 'route', domain_suffix: [...SAFETY_DOMAIN_SUFFIXES], server: localDns },
    {
      action: 'route',
      process_name: unique([...WECHAT_PROCESSES, ...DOUYIN_PROCESSES, ...(options.extraDirectExes || [])]),
      server: localDns,
    },
  ]
  if (directExes.length) {
    dnsPrefix.push({ action: 'route', process_name: directExes, server: localDns })
  }
  for (const tag of cnTags) {
    dnsPrefix.push({ action: 'route', rule_set: tag, server: localServerFor(tag) })
  }
  if (proxyExes.length) {
    const server = fakeIpDns || remoteDns
    if (server) dnsPrefix.push({ action: 'route', process_name: proxyExes, server })
  }

  const dns = (config.dns = config.dns || {})
  const dnsRules = (Array.isArray(dns.rules) ? dns.rules : []) as Record<string, unknown>[]
  dns.rules = [...dnsPrefix, ...stripInjected(dnsRules, dnsPrefix)]
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
