import assert from 'node:assert/strict'
import test from 'node:test'

import { applyAppRouting, sampleBaseConfig } from '../src/utils/appRouting.ts'
import { buildCountryUrltests } from '../src/utils/countryGroups.ts'
import {
  FREE_KEEP,
  FREE_NODE_GROUP,
  FREE_NODE_SOURCES,
  FREE_NODE_SUB_ID,
  FREE_NODE_WARNING,
  FREE_PARSE_CAP,
  FREE_PROBE_CONCURRENCY,
  FREE_REFRESH_MS,
  FREE_SLOW_MS,
  attachFreeGroup,
  collectFreeCandidates,
  isAutomaticGroup,
  decideFreeList,
  dueForFreeRefresh,
  ensureFreeSubscription,
  expandsFreeSubscription,
  freeNodeVisibleInGroup,
  freeNodeSubscription,
  parseFreeSource,
  rankFreeNodes,
} from '../src/utils/freeNodes.ts'
import { normalizeSubscriptionProxies } from '../src/utils/subscriptionConvert.ts'

const node = (tag: string, server: string, delay: number, ok = true) => ({
  node: { type: 'trojan', tag, server, server_port: 443, password: 'x' },
  ok,
  delayMs: delay,
})

test('the free subscription is separate, off, and capped', () => {
  assert.equal(FREE_NODE_WARNING, '免费节点不安全，勿用于支付/登录')
  assert.equal(FREE_PARSE_CAP, 100)
  assert.equal(FREE_KEEP, 5)
  assert.equal(FREE_PROBE_CONCURRENCY, 2)
  assert.equal(FREE_REFRESH_MS, 12 * 60 * 60 * 1000)
  assert.equal(FREE_SLOW_MS, 5000)
  assert.equal(FREE_NODE_SOURCES.length >= 1 && FREE_NODE_SOURCES.length <= 2, true)
  const sub = freeNodeSubscription()
  assert.equal(sub.id, FREE_NODE_SUB_ID)
  assert.equal(sub.name, '免费节点（备用）')
  assert.equal(sub.disabled, true)
  assert.deepEqual(sub.urls, FREE_NODE_SOURCES)
  const list: App.Subscription[] = []
  assert.equal(ensureFreeSubscription(list), true)
  assert.equal(ensureFreeSubscription(list), false)
  assert.equal(expandsFreeSubscription(FREE_NODE_SUB_ID), true)
  assert.equal(expandsFreeSubscription('paid'), false)
  assert.equal(dueForFreeRefresh(0, 1_000), true)
  assert.equal(dueForFreeRefresh(1_000, 1_000 + FREE_REFRESH_MS - 1), false)
  assert.equal(dueForFreeRefresh(1_000, 1_000 + FREE_REFRESH_MS), true)
})

test('source dns and rules are dropped and parsing stops around 100 nodes', () => {
  const proxies = Array.from({ length: 120 }, (_, index) => ({
    name: `香港 ${index}`,
    type: 'trojan',
    server: `n${index}.example`,
    port: 443,
    password: 'p',
  }))
  const body = JSON.stringify({
    dns: { servers: [{ type: 'udp', server: '8.8.8.8' }] },
    route: { rules: [{ domain: ['weixin.com'], outbound: 'proxy' }], final: 'proxy' },
    outbounds: [
      { type: 'direct', tag: 'direct' },
      { type: 'selector', tag: '节点选择', outbounds: ['direct'] },
      ...proxies.map((item) => ({
        type: 'trojan',
        tag: item.name,
        server: item.server,
        server_port: item.port,
        password: item.password,
      })),
    ],
  })
  const parsed = parseFreeSource(body)
  assert.equal(parsed.some((item) => item.type === 'direct' || item.type === 'selector'), false)
  assert.equal(parsed.some((item) => 'route' in item || 'dns' in item), false)
  const capped = collectFreeCandidates([parsed])
  assert.equal(capped.length, 100)
  assert.equal(capped.every((item) => item.tag.startsWith('free-')), true)
  const again = collectFreeCandidates([capped, parsed])
  assert.equal(again.length, 100)
})

test('a free source cannot import its own dns, rules, or dialer', () => {
  const body = JSON.stringify({
    log: { level: 'debug' },
    experimental: { cache_file: { enabled: true } },
    inbounds: [{ type: 'tun', tag: 'tun-in' }],
    dns: {
      servers: [{ type: 'udp', tag: 'source-dns', server: '1.1.1.1' }],
      rules: [{ domain_suffix: ['weixin.com'], server: 'source-dns' }],
      final: 'source-dns',
    },
    route: {
      rules: [
        { domain_suffix: ['weixin.com', 'douyin.com'], outbound: 'source-final' },
        { domain_suffix: ['baidu.com'], outbound: 'source-proxy' },
      ],
      final: 'source-final',
    },
    outbounds: [
      { type: 'direct', tag: 'direct' },
      { type: 'dns', tag: 'dns-out' },
      { type: 'selector', tag: 'source-proxy', outbounds: ['hijack'] },
      {
        type: 'trojan',
        tag: 'hijack',
        server: 'free.example',
        server_port: 443,
        password: 'secret',
        detour: 'direct',
        domain_resolver: { server: 'source-dns', strategy: 'ipv4_only' },
        domain_strategy: 'ipv4_only',
        routing_mark: 4321,
        bind_interface: 'eth0',
        tls: { enabled: true, server_name: 'free.example' },
      },
    ],
  })
  const yaml = [
    'dns:',
    '  enable: true',
    '  nameserver:',
    '    - 8.8.8.8',
    'rules:',
    '  - DOMAIN-SUFFIX,weixin.com,PROXY',
    '  - MATCH,PROXY',
    'proxy-groups:',
    '  - name: PROXY',
    '    type: select',
    '    proxies: [节点]',
    'proxies:',
    '  - name: 节点',
    '    type: trojan',
    '    server: clash.example',
    '    port: 443',
    '    password: clash-secret',
    '    dialer-proxy: other',
    '',
  ].join('\n')

  const nodes = collectFreeCandidates([parseFreeSource(body), parseFreeSource(yaml)])
  assert.equal(nodes.length, 2)
  for (const node of nodes) {
    assert.equal(node.detour, undefined)
    assert.equal(node.domain_resolver, undefined)
    assert.equal(node.domain_strategy, undefined)
    assert.equal(node.routing_mark, undefined)
    assert.equal(node.bind_interface, undefined)
    assert.equal('dialer-proxy' in node, false)
    assert.equal('route' in node, false)
    assert.equal('dns' in node, false)
    assert.equal('rules' in node, false)
  }
  assert.equal(nodes[0]?.server, 'free.example')
  assert.equal(nodes[0]?.password, 'secret')
  assert.equal((nodes[0]?.tls as { server_name?: string }).server_name, 'free.example')
  assert.equal(nodes[1]?.server, 'clash.example')
  assert.equal(nodes[1]?.password, 'clash-secret')

  const base = sampleBaseConfig()
  base.route.rules = [
    { action: 'route', clash_mode: 'global', outbound: 'direct' },
    { action: 'route', rule_set: 'geosite-cn', outbound: 'direct' },
  ]
  base.route.final = 'direct'
  base.outbounds = attachFreeGroup(
    [
      { type: 'selector', tag: '🚀 节点选择', outbounds: ['🎈 自动选择', 'direct'] },
      { type: 'urltest', tag: '🎈 自动选择', outbounds: ['paid'] },
      { type: 'trojan', tag: 'paid', server: 'paid.example', server_port: 443, password: 'p' },
      { type: 'direct', tag: 'direct' },
    ],
    nodes,
  ) as typeof base.outbounds
  const routed = applyAppRouting(base, {
    programs: [{ id: 'chrome', name: 'Chrome', exe: 'chrome.exe', mode: 'proxy' }],
    pinnedRoutes: { wechat: 'direct', douyin: 'direct' },
    domesticDirect: true,
  })
  const rules = routed.route.rules as Record<string, unknown>[]
  const outboundFor = (name: string) =>
    rules.find((rule) => {
      const value = rule.process_name
      const list = Array.isArray(value) ? value.map(String) : value ? [String(value)] : []
      return rule.action === 'route' && !rule.rule_set && list.includes(name)
    })?.outbound
  assert.equal(outboundFor('Weixin.exe'), 'direct')
  assert.equal(outboundFor('douyin.exe'), 'direct')
  assert.equal(outboundFor('chrome.exe'), '🚀 节点选择')
  assert.equal(routed.route.final, 'direct')
  const cn = rules.find(
    (rule) => rule.rule_set === 'geosite-cn' || (Array.isArray(rule.rule_set) && rule.rule_set.includes('geosite-cn')),
  )
  assert.equal(cn?.outbound, 'direct')
  const dumped = JSON.stringify(routed)
  assert.equal(dumped.includes('source-dns'), false)
  assert.equal(dumped.includes('source-final'), false)
  assert.equal(dumped.includes('1.1.1.1'), false)
  assert.equal(dumped.includes('weixin.com'), false)
  assert.equal(dumped.includes('8.8.8.8'), true)
  const imported = (routed.outbounds as Record<string, unknown>[]).filter((item) =>
    String(item.tag || '').startsWith('free-'),
  )
  assert.equal(imported.length, 2)
  for (const item of imported) {
    assert.equal(item.detour, undefined)
    assert.equal(item.domain_resolver, undefined)
    assert.equal(item.routing_mark, undefined)
  }
  assert.equal((routed.outbounds as Record<string, unknown>[]).some((item) => item.type === 'dns'), false)
})

test('only the five fastest working nodes are kept', () => {
  const ranked = rankFreeNodes([
    node('free-a', 'a.example', 80),
    node('free-b', 'b.example', 10),
    node('free-dead', 'c.example', 1, false),
    node('free-d', 'd.example', 40),
    node('free-e', 'e.example', 20),
    node('free-f', 'f.example', 30),
    node('free-g', 'g.example', 50),
  ])
  assert.deepEqual(
    ranked.map((item) => item.tag),
    ['free-b', 'free-e', 'free-f', 'free-d', 'free-g'],
  )
  const previous = [node('free-old', 'old.example', 15).node]
  assert.deepEqual(decideFreeList(previous, []).proxies, previous)
  assert.equal(decideFreeList(previous, []).replace, false)
  const slow = node('free-slow', 'slow.example', 8000)
  const rankedFromProbes = rankFreeNodes([
    ...[node('free-a', 'a.example', 80), node('free-b', 'b.example', 10)],
    slow,
  ])
  assert.equal(rankedFromProbes.some((item) => item.tag === 'free-slow'), false)
})

test('a refresh drops dead and slow kept nodes and fills the slot', () => {
  const alive = node('free-old', 'old.example', 15).node
  const dead = node('free-dead', 'dead.example', 1, false).node
  const replacement = node('free-new', 'new.example', 40).node
  const slow = node('free-slow', 'slow.example', 8000).node
  const decision = decideFreeList([alive, dead], [
    { node: alive, ok: true, delayMs: 15 },
    { node: dead, ok: false, delayMs: 0 },
    { node: replacement, ok: true, delayMs: 40 },
    { node: slow, ok: true, delayMs: 8000 },
  ])
  assert.equal(decision.replace, true)
  assert.deepEqual(
    decision.proxies.map((item) => item.tag),
    ['free-old', 'free-new'],
  )
  const wiped = decideFreeList([alive, dead], [
    { node: alive, ok: false, delayMs: 0 },
    { node: dead, ok: false, delayMs: 0 },
  ])
  assert.equal(wiped.replace, true)
  assert.deepEqual(wiped.proxies, [])
  const skipped = decideFreeList([alive], [{ node: alive, ok: false, delayMs: 0, unavailable: true }])
  assert.equal(skipped.replace, false)
  assert.deepEqual(skipped.proxies, [alive])
})

test('discarded free nodes stay out of every group list', () => {
  const kept = new Set(['free-a', 'free-b'])
  assert.equal(freeNodeVisibleInGroup('🆓 免费备用', 'free-a', kept), true)
  assert.equal(freeNodeVisibleInGroup('🆓 免费备用', 'free-dead', kept), false)
  assert.equal(freeNodeVisibleInGroup('🚀 节点选择', 'free-a', kept), false)
  assert.equal(freeNodeVisibleInGroup('🚀 节点选择', 'HK-01', kept), true)
  assert.equal(freeNodeVisibleInGroup('🚀 节点选择', FREE_NODE_GROUP, kept), true)
  assert.equal(freeNodeVisibleInGroup('🚀 节点选择', FREE_NODE_GROUP, new Set()), false)
  assert.equal(freeNodeVisibleInGroup('🆓 免费备用', FREE_NODE_GROUP, kept), false)
})

test('free nodes stay out of auto, country, and fallback groups', () => {
  const nodes = rankFreeNodes([
    node('香港 1', 'hk.example', 20),
    node('日本 1', 'jp.example', 30),
  ])
  const outbounds = attachFreeGroup(
    [
      {
        type: 'selector',
        tag: '🚀 节点选择',
        outbounds: ['🎈 自动选择', '🇭🇰 香港', 'direct'],
      },
      { type: 'urltest', tag: '🎈 自动选择', outbounds: ['HK-01', '日本-01'] },
      { type: 'urltest', tag: '🇭🇰 香港', outbounds: ['HK-01'] },
      { type: 'selector', tag: '🐟 漏网之鱼', outbounds: ['🚀 节点选择', 'direct'] },
      { type: 'selector', tag: 'GLOBAL', outbounds: ['🚀 节点选择', '🎈 自动选择'] },
      { type: 'trojan', tag: 'HK-01', server: 'paid.example', server_port: 443 },
    ],
    nodes,
  )
  const select = outbounds.find((item) => item.tag === '🚀 节点选择')
  const auto = outbounds.find((item) => item.tag === '🎈 自动选择')
  const country = outbounds.find((item) => item.tag === '🇭🇰 香港')
  const fallback = outbounds.find((item) => item.tag === '🐟 漏网之鱼')
  const global = outbounds.find((item) => item.tag === 'GLOBAL')
  const free = outbounds.find((item) => item.tag === FREE_NODE_GROUP)
  const tags = nodes.map((item) => item.tag)
  assert.equal(free?.type, 'selector')
  assert.equal(free?.url, undefined)
  assert.equal(free?.interval, undefined)
  assert.deepEqual(free?.outbounds, tags)
  assert.equal(select?.outbounds?.includes(FREE_NODE_GROUP), true)
  for (const tag of tags) {
    assert.equal(select?.outbounds?.includes(tag), false)
    assert.equal(auto?.outbounds?.includes(tag), false)
    assert.equal(country?.outbounds?.includes(tag), false)
    assert.equal(fallback?.outbounds?.includes(tag), false)
    assert.equal(global?.outbounds?.includes(tag), false)
  }
  assert.equal(auto?.outbounds?.includes(FREE_NODE_GROUP), false)
  assert.equal(country?.outbounds?.includes(FREE_NODE_GROUP), false)
  assert.equal(fallback?.outbounds?.includes(FREE_NODE_GROUP), false)
  assert.equal(global?.outbounds?.includes(FREE_NODE_GROUP), false)
  assert.equal(outbounds.filter((item) => item.tag === FREE_NODE_GROUP).length, 1)
  assert.equal(select?.outbounds?.[0], '🎈 自动选择')
  assert.equal(select?.outbounds?.at(-1), FREE_NODE_GROUP)
})

test('free nodes never join a urltest or fallback and are not the default', () => {
  const nodes = rankFreeNodes([node('香港 1', 'hk.example', 20)])
  const tags = nodes.map((item) => item.tag)
  const outbounds = attachFreeGroup(
    [
      {
        type: 'selector',
        tag: '🚀 节点选择',
        default: FREE_NODE_GROUP,
        outbounds: ['🎈 自动选择', 'direct'],
      },
      {
        type: 'urltest',
        tag: '🎈 自动选择',
        outbounds: ['paid', ...tags, FREE_NODE_GROUP],
        url: 'https://www.gstatic.com/generate_204',
        interval: '3m',
      },
      {
        type: 'urltest',
        tag: '🚀 节点选择-测速',
        outbounds: ['🎈 自动选择', FREE_NODE_GROUP],
        url: 'https://www.gstatic.com/generate_204',
        interval: '3m',
      },
      { type: 'fallback', tag: '🐟 漏网之鱼', outbounds: ['🚀 节点选择', FREE_NODE_GROUP, ...tags] },
      { type: 'loadbalance', tag: 'loadbalance', outbounds: [FREE_NODE_GROUP, ...tags] },
      { type: 'trojan', tag: 'paid', server: 'paid.example', server_port: 443 },
    ],
    nodes,
  )
  const select = outbounds.find((item) => item.type === 'selector' && item.tag === '🚀 节点选择')
  assert.equal(select?.default, '🎈 自动选择')
  assert.equal(select?.outbounds?.at(-1), FREE_NODE_GROUP)
  const free = outbounds.find((item) => item.tag === FREE_NODE_GROUP)
  assert.equal(free?.type, 'selector')
  assert.equal(free?.url, undefined)
  assert.equal(free?.interval, undefined)
  for (const item of outbounds) {
    if (!isAutomaticGroup(item)) continue
    assert.equal(item.outbounds?.includes(FREE_NODE_GROUP), false)
    for (const tag of tags) assert.equal(item.outbounds?.includes(tag), false)
  }
  const regions = buildCountryUrltests(['free-香港 1', FREE_NODE_GROUP, 'HK-01'])
  const regionText = JSON.stringify(regions)
  assert.equal(regionText.includes('free-'), false)
  assert.equal(regionText.includes(FREE_NODE_GROUP), false)
  assert.equal(regions.some((group) => group.outbounds.includes('HK-01')), true)
})

test('vmess and shadowsocks share links become outbounds and rules do not', () => {
  const vmess = Buffer.from(
    JSON.stringify({ add: '1.2.3.4', port: '443', id: 'uuid-1', ps: '备用', net: 'ws', path: '/ws', tls: 'tls', sni: 'a.com' }),
  ).toString('base64')
  const ss = Buffer.from('aes-256-gcm:secret@5.6.7.8:8388').toString('base64')
  const parsed = normalizeSubscriptionProxies([
    { base64: Buffer.from(`vmess://${vmess}\nss://${ss}#ss节点`, 'utf8').toString('base64') },
    { type: 'dns', tag: 'dns-out' },
  ])
  assert.equal(parsed.length, 2)
  assert.equal(parsed[0]?.type, 'vmess')
  assert.equal(parsed[0]?.tag, '备用')
  assert.equal((parsed[0]?.transport as { type: string }).type, 'ws')
  assert.equal(parsed[1]?.type, 'shadowsocks')
  assert.equal(parsed[1]?.tag, 'ss节点')
  assert.equal(parsed[1]?.server, '5.6.7.8')
})

test('WeChat, Douyin, and domestic routes ignore the free group', () => {
  const base = sampleBaseConfig()
  base.outbounds = [
    { type: 'selector', tag: FREE_NODE_GROUP, outbounds: ['free-a'] },
    { type: 'trojan', tag: 'free-a', server: 'free.example', server_port: 443, password: 'x' },
    { type: 'selector', tag: '🚀 节点选择', outbounds: ['🎈 自动选择', FREE_NODE_GROUP] },
    { type: 'urltest', tag: '🎈 自动选择', outbounds: ['paid'] },
    { type: 'trojan', tag: 'paid', server: 'paid.example', server_port: 443, password: 'x' },
    { type: 'direct', tag: 'direct' },
  ]
  base.route.rules = [
    { action: 'route', clash_mode: 'global', outbound: 'direct' },
    { action: 'route', rule_set: 'geosite-cn', outbound: 'direct' },
  ]
  base.route.final = 'direct'
  const routed = applyAppRouting(base, {
    programs: [{ id: 'chrome', name: 'Chrome', exe: 'chrome.exe', mode: 'proxy' }],
    pinnedRoutes: { wechat: 'direct', douyin: 'direct' },
    domesticDirect: true,
  })
  const rules = routed.route.rules as Record<string, unknown>[]
  const outboundFor = (name: string) =>
    rules.find((rule) => {
      const value = rule.process_name
      const list = Array.isArray(value) ? value.map(String) : value ? [String(value)] : []
      return rule.action === 'route' && !rule.rule_set && list.includes(name)
    })?.outbound
  assert.equal(outboundFor('Weixin.exe'), 'direct')
  assert.equal(outboundFor('douyin.exe'), 'direct')
  assert.equal(outboundFor('chrome.exe'), '🚀 节点选择')
  assert.equal(routed.route.final, 'direct')
  const cn = rules.find((rule) => rule.rule_set === 'geosite-cn' || (Array.isArray(rule.rule_set) && rule.rule_set.includes('geosite-cn')))
  if (cn) assert.equal(cn.outbound, 'direct')
  assert.equal(JSON.stringify(routed.route.rules).includes(FREE_NODE_GROUP), false)
})
