import assert from 'node:assert/strict'
import test from 'node:test'

import {
  acknowledgeProblem,
  hasUnackedRed,
  reconcileAck,
  securityProblemKey,
} from '../src/utils/securityAlert.ts'

const hosts = { id: 'hosts', level: 'red', text: 'hosts 改写了 google.com。' }
const proxy = { id: 'proxy', level: 'red', text: '系统代理指向别处。' }
const dns = { id: 'dns', level: 'green', text: '物理网卡用的是路由器或常见公共 DNS。' }

test('a red item blinks until that exact problem is acknowledged', () => {
  const items = [hosts, dns]
  assert.equal(hasUnackedRed(items, []), true)
  const acked = acknowledgeProblem([], hosts)
  assert.equal(hasUnackedRed(items, acked), false)
  assert.equal(acknowledgeProblem(acked, dns), acked)
})

test('the same problem stays quiet and a different one blinks again', () => {
  const acked = acknowledgeProblem([], hosts)
  const again = reconcileAck(acked, [securityProblemKey(hosts)])
  assert.deepEqual(again, acked)
  assert.equal(hasUnackedRed([hosts], again), false)

  const changed = { ...hosts, text: 'hosts 改写了 youtube.com。' }
  const kept = reconcileAck(acked, [securityProblemKey(changed)])
  assert.deepEqual(kept, [])
  assert.equal(hasUnackedRed([changed], kept), true)
})

test('a problem that disappears is forgotten, so it blinks if it returns', () => {
  const acked = acknowledgeProblem(acknowledgeProblem([], hosts), proxy)
  const onlyProxy = reconcileAck(acked, [securityProblemKey(proxy)])
  assert.deepEqual(onlyProxy, [securityProblemKey(proxy)])
  const returned = reconcileAck(onlyProxy, [securityProblemKey(hosts)])
  assert.deepEqual(returned, [])
  assert.equal(hasUnackedRed([hosts], returned), true)
})
