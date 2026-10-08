import assert from 'node:assert/strict'
import test from 'node:test'

import {
  SITE_PROBES,
  classifyChains,
  formatCoreProxy,
  matchConnection,
  parseExit,
  routeForHost,
  runNetworkCheck,
  selectedNode,
  siteStatus,
  type NetworkCheckDeps,
  type ProbeResponse,
} from '../src/utils/networkCheck.ts'

const directPinned = { wechat: 'direct' as const, douyin: 'direct' as const }

const ok = (body: string, elapsedMs = 40): ProbeResponse => ({
  ok: true,
  status: 200,
  body,
  elapsedMs,
})

const fail = (elapsedMs = 20): ProbeResponse => ({
  ok: false,
  status: 0,
  body: '',
  elapsedMs,
  error: 'timeout',
})

test('parseExit reads ipip text and geo json', () => {
  assert.deepEqual(
    parseExit('当前 IP：36.5.2.1  来自于：中国 广东 深圳  电信'),
    { ip: '36.5.2.1', place: '中国 广东 深圳  电信' },
  )
  assert.deepEqual(parseExit('8.8.8.8\n'), { ip: '8.8.8.8', place: '' })
  assert.deepEqual(parseExit('{"ip":"1.2.3.4","country":"JP","region":"Tokyo","city":"Tokyo"}'), {
    ip: '1.2.3.4',
    place: 'JP Tokyo',
  })
  assert.deepEqual(
    parseExit({ query: '9.9.9.9', country: '美国', regionName: '弗吉尼亚州', city: 'Ashburn' }),
    { ip: '9.9.9.9', place: '美国 弗吉尼亚州 Ashburn' },
  )
  assert.equal(parseExit('not an address'), null)
  assert.equal(parseExit(''), null)
})

test('routeForHost keeps Baidu direct and follows the saved WeChat and Douyin choice', () => {
  assert.equal(routeForHost('www.baidu.com', directPinned), 'direct')
  assert.equal(routeForHost('weixin.qq.com', directPinned), 'direct')
  assert.equal(routeForHost('www.douyin.com', directPinned), 'direct')
  assert.equal(routeForHost('www.google.com', directPinned), 'proxy')
  assert.equal(routeForHost('www.youtube.com', directPinned), 'proxy')
  assert.equal(routeForHost('github.com', directPinned), 'proxy')
  assert.equal(routeForHost('web.telegram.org', directPinned), 'proxy')

  const proxyPinned = { wechat: 'proxy' as const, douyin: 'proxy' as const }
  assert.equal(routeForHost('weixin.qq.com', proxyPinned), 'proxy')
  assert.equal(routeForHost('www.douyin.com', proxyPinned), 'proxy')
  assert.equal(routeForHost('www.baidu.com', proxyPinned), 'direct')
  assert.equal(routeForHost('weixin.qq.com', { wechat: 'proxy', douyin: 'direct' }), 'proxy')
  assert.equal(routeForHost('www.douyin.com', { wechat: 'proxy', douyin: 'direct' }), 'direct')
})

test('classifyChains treats a direct tag as local and any node tag as proxy', () => {
  assert.equal(classifyChains(['direct']), 'direct')
  assert.equal(classifyChains(['DIRECT']), 'direct')
  assert.equal(classifyChains(['hk-01', 'proxy']), 'proxy')
  assert.equal(classifyChains([]), null)
  assert.equal(matchConnection([{ host: 'www.google.com', chains: ['direct'] }, { host: 'WWW.GOOGLE.COM', chains: ['node'] }], 'www.google.com')?.chains[0], 'node')
})

test('site status is green, amber when slow, and red on failure', () => {
  assert.equal(siteStatus(true, 40), 'green')
  assert.equal(siteStatus(true, 800), 'amber')
  assert.equal(siteStatus(false, 10), 'red')
})

test('formatCoreProxy and selectedNode', () => {
  assert.equal(formatCoreProxy(undefined), '')
  assert.equal(
    formatCoreProxy({ schema: 'http', host: '0.0.0.0', port: 20122 }),
    'http://127.0.0.1:20122',
  )
  assert.equal(
    formatCoreProxy({ schema: 'socks5', host: '127.0.0.1', port: 1080, username: 'a b', password: 'x' }),
    'socks5://a%20b:x@127.0.0.1:1080',
  )
  assert.equal(selectedNode({ proxy: { type: 'Selector', now: 'HK-01' } }), 'HK-01')
  assert.equal(selectedNode({ other: { type: 'Selector', now: 'Tokyo' } }), 'Tokyo')
  assert.equal(selectedNode({}), '')
})

const deps = (overrides: Partial<NetworkCheckDeps> = {}): NetworkCheckDeps => ({
  directGet: async () => ok('当前 IP：1.1.1.1  来自于：中国 北京'),
  proxyGet: async () => ok('{"ip":"8.8.4.4","country":"US","city":"Ashburn"}'),
  connections: async () => [],
  lookup: async () => ({ ok: true, address: '1.2.3.4' }),
  core: { running: true, tun: true, mode: 'rule', node: 'HK-01', proxy: 'http://127.0.0.1:20122' },
  pinned: directPinned,
  ...overrides,
})

test('runNetworkCheck uses direct for the local exit and Baidu, and the proxy for Google', async () => {
  const directUrls: string[] = []
  const proxyUrls: string[] = []
  const report = await runNetworkCheck(
    deps({
      directGet: async (url) => {
        directUrls.push(url)
        return ok('当前 IP：1.1.1.1  来自于：中国 北京', 30)
      },
      proxyGet: async (url) => {
        proxyUrls.push(url)
        return ok('{"ip":"8.8.4.4","country":"US","region":"Virginia"}', 120)
      },
    }),
  )

  assert.equal(report.local.status, 'green')
  assert.equal(report.local.ip, '1.1.1.1')
  assert.equal(report.local.place, '中国 北京')
  assert.equal(report.proxy.ip, '8.8.4.4')
  assert.equal(report.proxy.place, 'US Virginia')
  assert.equal(report.dns.status, 'green')
  assert.equal(report.dns.address, '1.2.3.4')
  assert.deepEqual(report.core, { running: true, tun: true, mode: 'rule', node: 'HK-01' })

  assert.ok(directUrls.includes('https://myip.ipip.net'))
  assert.ok(directUrls.includes('https://www.baidu.com/robots.txt'))
  assert.equal(directUrls.includes('https://www.google.com/generate_204'), false)
  assert.ok(proxyUrls.includes('https://ipinfo.io/json'))
  assert.ok(proxyUrls.includes('https://www.google.com/generate_204'))
  assert.equal(proxyUrls.includes('https://www.baidu.com/robots.txt'), false)

  const byId = Object.fromEntries(report.sites.map((site) => [site.id, site]))
  assert.equal(byId.baidu?.route, 'direct')
  assert.equal(byId.baidu?.status, 'green')
  assert.equal(byId.google?.route, 'proxy')
  assert.equal(byId.wechat?.route, 'direct')
  assert.equal(byId.youtube?.elapsedMs, 120)
  assert.deepEqual(
    report.sites.map((site) => site.id),
    SITE_PROBES.map((site) => site.id),
  )
})

test('a saved proxy for WeChat is not rewritten, and a live chain can correct the label', async () => {
  const report = await runNetworkCheck(
    deps({
      pinned: { wechat: 'proxy', douyin: 'direct' },
      connections: async () => [
        { host: 'weixin.qq.com', chains: ['direct'] },
        { host: 'www.google.com', chains: ['HK-01', 'proxy'] },
      ],
      directGet: async (url) => (url.includes('weixin') ? fail() : ok('1.1.1.1 来自于：中国', 30)),
      proxyGet: async (url) => (url.includes('weixin') ? ok('ok', 50) : ok('{"ip":"8.8.8.8","country":"US"}', 900)),
    }),
  )
  const byId = Object.fromEntries(report.sites.map((site) => [site.id, site]))
  assert.equal(byId.wechat?.route, 'direct')
  assert.equal(byId.douyin?.route, 'direct')
  assert.equal(byId.google?.route, 'proxy')
  assert.equal(byId.google?.status, 'amber')
  assert.equal(byId.youtube?.status, 'amber')
})

test('missing proxy, failed dns, and a bad first exit fall back or fail closed', async () => {
  let localCalls = 0
  const report = await runNetworkCheck(
    deps({
      core: { running: false, tun: false, mode: '', node: '', proxy: '' },
      lookup: async (host) => (host === 'www.baidu.com' ? { ok: false, address: '' } : { ok: true, address: '9.9.9.9' }),
      directGet: async (url) => {
        localCalls += 1
        if (url === 'https://myip.ipip.net') return fail()
        if (url === 'http://myip.ipip.net') return ok('当前 IP：2.2.2.2  来自于：中国 上海')
        return fail()
      },
    }),
  )
  assert.equal(report.local.ip, '2.2.2.2')
  assert.equal(report.proxy.status, 'red')
  assert.equal(report.dns.address, '9.9.9.9')
  assert.equal(report.core.running, false)
  assert.equal(report.core.tun, false)
  const google = report.sites.find((site) => site.id === 'google')
  assert.equal(google?.route, 'direct')
  assert.equal(google?.status, 'red')
  assert.ok(localCalls >= 2)
})

test('the probes start together instead of waiting in a queue', async () => {
  let active = 0
  let peak = 0
  const enter = async (body: string) => {
    active += 1
    peak = Math.max(peak, active)
    await new Promise((resolve) => setTimeout(resolve, 30))
    active -= 1
    return ok(body, 30)
  }
  await runNetworkCheck(
    deps({
      directGet: () => enter('当前 IP：1.1.1.1  来自于：中国'),
      proxyGet: () => enter('{"ip":"8.8.8.8","country":"US"}'),
      lookup: async () => {
        active += 1
        peak = Math.max(peak, active)
        await new Promise((resolve) => setTimeout(resolve, 30))
        active -= 1
        return { ok: true, address: '1.1.1.1' }
      },
    }),
  )
  assert.ok(peak >= 8, `peak concurrency ${peak}`)
})
