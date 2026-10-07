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
})
