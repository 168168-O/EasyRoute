import assert from 'node:assert/strict'
import test from 'node:test'

import {
  DOUYIN_PROCESSES,
  SAFETY_DOMAIN_SUFFIXES,
  WECHAT_PROCESSES,
  applyAppRouting,
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

test('proxy process dns uses fakeip when a fakeip server exists', () => {
  const base = sampleBaseConfig()
  base.dns.servers.push({ type: 'fakeip', tag: 'fakeip-dns', inet4_range: '198.18.0.0/15' })
  const routed = applyAppRouting(base, {
    programs,
    proxyOutbound: 'proxy',
    directOutbound: 'direct',
  })
  const dnsRules = routed.dns.rules as Record<string, unknown>[]
  const chrome = dnsRules.find((rule) => names(rule.process_name).includes('chrome.exe'))
  assert.equal(chrome?.server, 'fakeip-dns')
  assert.equal(routed.dns.final, 'Remote-DNS')
  const cn = dnsRules.find((rule) => ruleSets(rule).includes('geosite-cn'))
  assert.equal(cn?.server, 'Local-DNS')
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
