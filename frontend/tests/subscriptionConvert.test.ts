import assert from 'node:assert/strict'
import test from 'node:test'

import {
  clashProxyToOutbound,
  decodeBase64Text,
  normalizeSubscriptionProxies,
} from '../src/utils/subscriptionConvert.ts'

test('clash anytls proxy becomes a sing-box anytls outbound', () => {
  const outbound = clashProxyToOutbound({
    name: '香港 01',
    type: 'anytls',
    server: 'hk.example.com',
    port: 443,
    password: 'secret',
    sni: 'www.cloudflare.com',
    'skip-cert-verify': false,
    alpn: ['h2'],
    'client-fingerprint': 'chrome',
    udp: true,
  })
  assert.ok(outbound)
  assert.equal(outbound.type, 'anytls')
  assert.equal(outbound.tag, '香港 01')
  assert.equal(outbound.server, 'hk.example.com')
  assert.equal(outbound.server_port, 443)
  assert.equal(outbound.password, 'secret')
  assert.deepEqual(outbound.tls, {
    enabled: true,
    server_name: 'www.cloudflare.com',
    alpn: ['h2'],
    utls: { enabled: true, fingerprint: 'chrome' },
  })
})

test('base64 anytls share links import without the conversion plugin', () => {
  const link =
    'anytls://s3cret@edge.example.com:8443?sni=edge.example.com&insecure=0&fp=chrome&alpn=h2#节点A'
  const body = Buffer.from(link + '\n', 'utf8').toString('base64')
  const proxies = normalizeSubscriptionProxies([{ base64: body }])
  assert.equal(proxies.length, 1)
  assert.equal(proxies[0]?.type, 'anytls')
  assert.equal(proxies[0]?.tag, '节点A')
  assert.equal(proxies[0]?.server, 'edge.example.com')
  assert.equal(proxies[0]?.server_port, 8443)
  assert.equal(proxies[0]?.password, 's3cret')
  assert.equal((proxies[0]?.tls as { server_name: string }).server_name, 'edge.example.com')
  assert.equal(proxies.some((proxy) => 'name' in proxy && !proxy.tag), false)
})

test('native sing-box outbounds pass through and unknown clash types are dropped', () => {
  const proxies = normalizeSubscriptionProxies([
    {
      type: 'anytls',
      tag: 'already',
      server: '1.2.3.4',
      server_port: 443,
      password: 'x',
    },
    { name: 'mystery', type: 'wireguard-plus', server: '1.1.1.1', port: 1 },
  ])
  assert.equal(proxies.length, 1)
  assert.equal(proxies[0]?.tag, 'already')
})

test('decodeBase64Text keeps utf-8 node names', () => {
  const encoded = Buffer.from('anytls://p@h:443#中文', 'utf8').toString('base64')
  assert.match(decodeBase64Text(encoded), /中文/)
})
