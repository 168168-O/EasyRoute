import assert from 'node:assert/strict'
import test from 'node:test'

import {
  DOUYIN_PROCESSES,
  FAKEIP_EXCLUDED_SUFFIXES,
  SAFETY_DOMAIN_SUFFIXES,
  WECHAT_PROCESSES,
  applyAppRouting,
  matchDnsServer,
  matchOutbound,
  sampleBaseConfig,
} from '../src/utils/appRouting.ts'

const programs = [
  { id: 'chrome', name: 'Google Chrome', exe: 'chrome.exe', mode: 'proxy' as const },
  { id: 'edge', name: 'Microsoft Edge', exe: 'msedge.exe', mode: 'direct' as const },
  { id: 'bad-wx', name: '微信', exe: 'Weixin.exe', mode: 'proxy' as const },
  { id: 'bad-dy', name: '抖音', exe: 'douyin.exe', mode: 'proxy' as const },
]

const config = () =>
  applyAppRouting(sampleBaseConfig(), {
    programs,
    extraDirectExes: ['douyin_extra.exe'],
    proxyOutbound: 'proxy',
    directOutbound: 'direct',
  })

const names = (value: unknown) => (Array.isArray(value) ? value.map(String) : [])

test('unlisted process goes direct', () => {
  const routed = config()
  assert.equal(matchOutbound(routed.route.rules, { processName: 'notepad.exe' }), 'direct')
  assert.equal(
    matchOutbound(routed.route.rules, { processName: 'msedge.exe', clashMode: 'global' }),
    'direct',
  )
})

test('process set to 走代理 uses the proxy outbound', () => {
  const routed = config()
  assert.equal(matchOutbound(routed.route.rules, { processName: 'chrome.exe' }), 'proxy')
  assert.equal(
    matchOutbound(routed.route.rules, { processName: 'chrome.exe', clashMode: 'global' }),
    'proxy',
  )
})

test('Weixin.exe and douyin.exe stay direct even if marked 走代理 and in global mode', () => {
  const routed = config()
  for (const exe of ['Weixin.exe', 'WeChat.exe', 'douyin.exe', 'douyin_tray.exe']) {
    assert.equal(
      matchOutbound(routed.route.rules, { processName: exe, clashMode: 'global' }),
      'direct',
      exe,
    )
  }
  const proxyRule = routed.route.rules.find(
    (rule: Record<string, unknown>) =>
      names(rule.process_name).includes('chrome.exe') && rule.outbound === 'proxy',
  )
  assert.ok(proxyRule)
  assert.equal(names(proxyRule.process_name).includes('Weixin.exe'), false)
  assert.equal(names(proxyRule.process_name).includes('douyin.exe'), false)
})

test('WeChat and Douyin rules come before global mode and other rules', () => {
  const routed = config()
  const rules = routed.route.rules as Record<string, unknown>[]
  const indexOf = (pred: (rule: Record<string, unknown>) => boolean) => rules.findIndex(pred)

  const wechat = indexOf((rule) => names(rule.process_name).includes('Weixin.exe'))
  const douyin = indexOf((rule) => names(rule.process_name).includes('douyin.exe'))
  const domains = indexOf((rule) => names(rule.domain_suffix).includes('douyin.com'))
  const globalMode = indexOf((rule) => rule.clash_mode === 'global')
  const userGeo = indexOf((rule) => rule.rule_set === 'geosite-cn')
  const proxy = indexOf((rule) => names(rule.process_name).includes('chrome.exe'))

  assert.equal(wechat, 0)
  assert.ok(douyin > wechat && douyin < globalMode)
  assert.ok(domains < globalMode)
  assert.ok(domains < userGeo)
  assert.ok(wechat < proxy)
  assert.ok(domains < proxy)

  const domainRule = rules[domains]!
  for (const suffix of SAFETY_DOMAIN_SUFFIXES) {
    assert.ok(names(domainRule.domain_suffix).includes(suffix), suffix)
  }
  assert.equal(domainRule.outbound, 'direct')

  const wechatRule = rules[wechat]!
  for (const exe of WECHAT_PROCESSES) assert.ok(names(wechatRule.process_name).includes(exe))
  const douyinRule = rules[douyin]!
  for (const exe of DOUYIN_PROCESSES) assert.ok(names(douyinRule.process_name).includes(exe))
  assert.ok(names(douyinRule.process_name).includes('douyin_extra.exe'))
})

test('unknown Douyin install path and private IPs go direct, and TUN is enabled', () => {
  const routed = config()
  assert.equal(
    matchOutbound(routed.route.rules, {
      processPath: String.raw`C:\Program Files (x86)\ByteDance\douyin\douyin_unknown.exe`,
      clashMode: 'global',
    }),
    'direct',
  )
  assert.equal(matchOutbound(routed.route.rules, { ipIsPrivate: true, clashMode: 'global' }), 'direct')
  assert.equal(routed.route.find_process, true)
  assert.ok(routed.inbounds.some((inbound: { type: string }) => inbound.type === 'tun'))

  const privateIndex = routed.route.rules.findIndex(
    (rule: Record<string, unknown>) => rule.ip_is_private === true,
  )
  const proxyIndex = routed.route.rules.findIndex((rule: Record<string, unknown>) =>
    names(rule.process_name).includes('chrome.exe'),
  )
  assert.ok(privateIndex >= 0 && privateIndex < proxyIndex)

  const dnsRules = routed.dns.rules as Record<string, unknown>[]
  assert.ok(names(dnsRules[0]?.domain_suffix).includes('weixin.qq.com'))
  assert.equal(dnsRules[0]?.server, 'dhagn-local-dns')
  const proxyDns = dnsRules.find((rule) => names(rule.process_name).includes('chrome.exe'))
  assert.equal(proxyDns?.server, 'Remote-DNS')
})

test('applying routing twice does not duplicate the safety rules', () => {
  const once = config()
  const twice = applyAppRouting(once, {
    programs,
    proxyOutbound: 'proxy',
    directOutbound: 'direct',
  })
  const wechatRules = twice.route.rules.filter((rule: Record<string, unknown>) =>
    names(rule.process_name).includes('Weixin.exe'),
  )
  assert.equal(wechatRules.length, 1)
  assert.equal(twice.route.rules[0].process_name.includes('Weixin.exe'), true)
  assert.equal(twice.route.final, 'direct')
  const chrome = twice.route.rules.find(
    (rule: Record<string, unknown>) =>
      names(rule.process_name).includes('chrome.exe') && rule.outbound === 'proxy',
  )
  assert.ok(chrome)
})

const ruleSets = (rule: Record<string, unknown>) =>
  Array.isArray(rule.rule_set) ? rule.rule_set.map(String) : rule.rule_set ? [String(rule.rule_set)] : []

test('unidentified connections and route.final go direct, including global mode', () => {
  const routed = config()
  assert.equal(routed.route.final, 'direct')
  const globalRule = routed.route.rules.find(
    (rule: Record<string, unknown>) => rule.clash_mode === 'global',
  )
  assert.equal(globalRule?.outbound, 'direct')
  assert.equal(matchOutbound(routed.route.rules, { clashMode: 'global' }), 'direct')
  assert.equal(
    matchOutbound(routed.route.rules, { domain: 'example.com', clashMode: 'global' }),
    'direct',
  )
  assert.equal(
    matchOutbound(routed.route.rules, { domain: 'weixin.qq.com', clashMode: 'global' }),
    'direct',
  )
  assert.equal(
    matchOutbound(routed.route.rules, { processName: 'chrome.exe', clashMode: 'global' }),
    'proxy',
  )
  assert.equal(
    matchOutbound(routed.route.rules, { processName: 'Weixin.exe', clashMode: 'global' }),
    'direct',
  )
  assert.equal(
    matchOutbound(routed.route.rules, { processName: 'douyin.exe', clashMode: 'global' }),
    'direct',
  )
})

test('dns sends non-cn and system lookups remote, and keeps cn, direct and safety local', () => {
  const routed = config()
  const dnsRules = routed.dns.rules as Record<string, unknown>[]
  assert.equal(
    dnsRules.some((rule) => rule.process_path_regex === '.+'),
    false,
  )
  assert.equal(routed.dns.final, 'Remote-DNS')

  const indexOf = (pred: (rule: Record<string, unknown>) => boolean) => dnsRules.findIndex(pred)
  const safety = indexOf((rule) => names(rule.domain_suffix).includes('weixin.qq.com'))
  const wechat = indexOf((rule) => names(rule.process_name).includes('Weixin.exe'))
  const edge = indexOf((rule) => names(rule.process_name).includes('msedge.exe'))
  const cn = indexOf((rule) => ruleSets(rule).includes('geosite-cn'))
  const chrome = indexOf((rule) => names(rule.process_name).includes('chrome.exe'))
  const globalDns = indexOf((rule) => rule.clash_mode === 'global')

  assert.equal(safety, 0)
  assert.equal(dnsRules[safety]?.server, 'dhagn-local-dns')
  assert.ok(wechat > safety)
  assert.equal(dnsRules[wechat]?.server, 'dhagn-local-dns')
  assert.ok(edge > wechat && edge < cn)
  assert.equal(dnsRules[edge]?.server, 'dhagn-local-dns')
  assert.ok(cn < chrome)
  assert.equal(dnsRules[cn]?.server, 'Local-DNS')
  assert.equal(dnsRules[chrome]?.server, 'Remote-DNS')
  assert.ok(chrome < globalDns)

  const twice = applyAppRouting(routed, {
    programs,
    extraDirectExes: ['douyin_extra.exe'],
    proxyOutbound: 'proxy',
    directOutbound: 'direct',
  })
  const cnRules = (twice.dns.rules as Record<string, unknown>[]).filter((rule) =>
    ruleSets(rule).includes('geosite-cn'),
  )
  assert.equal(cnRules.length, 1)
  assert.equal(
    (twice.dns.rules as Record<string, unknown>[]).some((rule) => rule.process_path_regex === '.+'),
    false,
  )
})

test('proxy process dns uses fakeip only for public A/AAAA queries', () => {
  const base = sampleBaseConfig()
  base.dns.servers.push({ type: 'fakeip', tag: 'fakeip-dns', inet4_range: '198.18.0.0/15' })
  const routed = applyAppRouting(base, {
    programs,
    proxyOutbound: 'proxy',
    directOutbound: 'direct',
  })
  const dnsRules = routed.dns.rules as Record<string, any>[]
  const logical = dnsRules.find((rule) => rule.type === 'logical' && rule.server === 'fakeip-dns')
  assert.ok(logical)
  assert.equal(logical.mode, 'and')
  assert.ok(names(logical.rules[0].process_name).includes('chrome.exe'))
  assert.equal(names(logical.rules[0].process_name).includes('Weixin.exe'), false)
  assert.deepEqual(logical.rules[1].query_type, ['A', 'AAAA'])
  assert.equal(logical.rules[2].invert, true)
  for (const suffix of ['.lan', '.local', '.localhost', 'localhost', '.home.arpa']) {
    assert.ok(names(logical.rules[2].domain_suffix).includes(suffix), suffix)
  }
  assert.deepEqual(names(logical.rules[2].domain_suffix), [...FAKEIP_EXCLUDED_SUFFIXES])

  const fallback = dnsRules.find(
    (rule) => names(rule.process_name).includes('chrome.exe') && rule.server === 'Remote-DNS',
  )
  assert.ok(fallback)
  const cn = dnsRules.find((rule) => ruleSets(rule).includes('geosite-cn'))
  assert.equal(cn?.server, 'Local-DNS')
  assert.ok(dnsRules.indexOf(cn) < dnsRules.indexOf(logical))
  assert.ok(dnsRules.indexOf(logical) < dnsRules.indexOf(fallback))
  assert.equal(routed.dns.final, 'Remote-DNS')

  const dns = (query: { processName?: string; domain?: string; queryType?: string }) =>
    matchDnsServer(dnsRules, query)
  assert.equal(dns({ processName: 'chrome.exe', domain: 'www.google.com', queryType: 'A' }), 'fakeip-dns')
  assert.equal(dns({ processName: 'chrome.exe', domain: 'www.google.com', queryType: 'AAAA' }), 'fakeip-dns')
  assert.equal(dns({ processName: 'chrome.exe', domain: 'www.google.com', queryType: 'TXT' }), 'Remote-DNS')
  assert.equal(dns({ processName: 'chrome.exe', domain: 'www.google.com', queryType: 'HTTPS' }), 'Remote-DNS')
  assert.equal(dns({ processName: 'chrome.exe', domain: 'printer.local', queryType: 'A' }), 'Remote-DNS')
  assert.equal(dns({ processName: 'chrome.exe', domain: 'localhost', queryType: 'A' }), 'Remote-DNS')
  assert.equal(dns({ processName: 'chrome.exe', domain: 'host.lan', queryType: 'AAAA' }), 'Remote-DNS')
  assert.equal(dns({ processName: 'msedge.exe', domain: 'www.google.com', queryType: 'A' }), 'dhagn-local-dns')
  assert.equal(dns({ processName: 'Weixin.exe', domain: 'www.google.com', queryType: 'A' }), 'dhagn-local-dns')
  assert.equal(dns({ processName: 'douyin.exe', domain: 'www.google.com', queryType: 'AAAA' }), 'dhagn-local-dns')
})

test('only this app on the mixed inbound goes proxy; other programs follow TUN rules', () => {
  const appPath = String.raw`C:\达货爱vpn姑娘\达货爱vpn姑娘.exe`
  const base = sampleBaseConfig()
  base.inbounds.push({ type: 'http', tag: 'http-in', listen: '127.0.0.1', listen_port: 20123 })
  const routed = applyAppRouting(base, {
    programs,
    proxyOutbound: 'proxy',
    directOutbound: 'direct',
    appPath,
  })
  const rules = routed.route.rules as Record<string, unknown>[]
  const indexOf = (pred: (rule: Record<string, unknown>) => boolean) => rules.findIndex(pred)
  const wechat = indexOf((rule) => names(rule.process_name).includes('Weixin.exe') && !rule.inbound)
  const douyin = indexOf((rule) => names(rule.process_name).includes('douyin.exe') && !rule.inbound)
  const byPath = indexOf((rule) => names(rule.process_path).includes(appPath))
  const byName = indexOf(
    (rule) => names(rule.process_name).includes('达货爱vpn姑娘.exe') && !!rule.inbound,
  )
  const catchAll = indexOf((rule) => rule.process_path_regex === '.+' && rule.outbound === 'direct')
  assert.ok(wechat >= 0 && douyin > wechat && byPath > douyin && byName > byPath && catchAll > byName)
  assert.deepEqual(names(rules[byPath]?.inbound), ['mixed-in', 'http-in'])
  assert.equal(rules[byPath]?.outbound, 'proxy')
  assert.equal(rules[byName]?.outbound, 'proxy')
  assert.equal(
    rules.some((rule) => rule.inbound && rule.outbound === 'proxy' && !rule.process_path && !rule.process_name),
    false,
  )

  const via = (processName: string, processPath: string, inbound = 'mixed-in') =>
    matchOutbound(rules, { processName, processPath, inbound, clashMode: 'global' })

  assert.equal(via('notepad.exe', String.raw`C:\Windows\notepad.exe`), 'direct')
  assert.equal(
    via('msedge.exe', String.raw`C:\Program Files\Microsoft\Edge\Application\msedge.exe`),
    'direct',
  )
  assert.equal(
    via('chrome.exe', String.raw`C:\Program Files\Google\Chrome\Application\chrome.exe`),
    'proxy',
  )
  assert.equal(via('Weixin.exe', String.raw`C:\Program Files\Tencent\Weixin\Weixin.exe`), 'direct')
  assert.equal(via('douyin.exe', String.raw`C:\Program Files\ByteDance\douyin\douyin.exe`, 'http-in'), 'direct')
  assert.equal(via('达货爱vpn姑娘.exe', appPath), 'proxy')
  assert.equal(via('达货爱vpn姑娘.exe', appPath, 'http-in'), 'proxy')
  assert.equal(via('达货爱vpn姑娘.exe', appPath, 'tun-in'), 'direct')
  assert.equal(
    matchOutbound(rules, { processName: '达货爱vpn姑娘.exe', inbound: 'mixed-in', clashMode: 'rule' }),
    'proxy',
  )
  assert.equal(
    matchOutbound(rules, { processName: 'notepad.exe', inbound: 'http-in', clashMode: 'rule' }),
    'direct',
  )
  assert.equal(
    matchOutbound(rules, { domain: 'weixin.qq.com', inbound: 'mixed-in', clashMode: 'global' }),
    'direct',
  )

  const twice = applyAppRouting(routed, {
    programs,
    proxyOutbound: 'proxy',
    directOutbound: 'direct',
    appPath,
  })
  const inboundRules = (twice.route.rules as Record<string, unknown>[]).filter(
    (rule) => names(rule.inbound).includes('mixed-in') && rule.outbound === 'proxy',
  )
  assert.equal(inboundRules.length, 2)
})

test('GeoSite-CN is kept on local dns and geolocation-!cn is not rewritten', () => {
  const base = sampleBaseConfig()
  base.dns.rules = [
    { action: 'route', clash_mode: 'global', server: 'Remote-DNS' },
    { action: 'route', rule_set: 'GeoSite-CN', server: 'Local-DNS' },
    { action: 'route', rule_set: 'geolocation-!cn', server: 'Remote-DNS' },
  ]
  base.route.rules = base.route.rules.filter(
    (rule: { rule_set?: string }) => rule.rule_set !== 'geosite-cn',
  )
  const routed = applyAppRouting(base, {
    programs,
    proxyOutbound: 'proxy',
    directOutbound: 'direct',
  })
  const dnsRules = routed.dns.rules as Record<string, unknown>[]
  const cn = dnsRules.filter((rule) => ruleSets(rule).some((tag) => tag.toLowerCase() === 'geosite-cn'))
  assert.equal(cn.length, 1)
  assert.equal(cn[0]?.server, 'Local-DNS')
  const notCn = dnsRules.find((rule) => ruleSets(rule).includes('geolocation-!cn'))
  assert.equal(notCn?.server, 'Remote-DNS')
  const cnIndex = dnsRules.indexOf(cn[0]!)
  const globalIndex = dnsRules.findIndex((rule) => rule.clash_mode === 'global')
  assert.ok(cnIndex >= 0 && cnIndex < globalIndex)
})
