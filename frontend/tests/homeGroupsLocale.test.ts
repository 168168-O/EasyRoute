import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

import zh from '../src/lang/locale/zh.ts'

const home = zh.home as Record<string, any>

test('policy group labels are Chinese, not raw keys', () => {
  const groups = home.groups
  assert.equal(groups.selector, '手动选择')
  assert.equal(groups.urltest, '自动选择')
  assert.equal(groups.fallback, '故障转移')
  assert.equal(groups.loadbalance, '负载均衡')
  assert.equal(groups.direct, '直连')
  assert.equal(groups.block, '拦截')
  assert.equal(groups.global, '全局')
  assert.equal(groups.current, '当前')
  for (const value of Object.values(groups)) {
    assert.equal(String(value).includes('home.groups'), false)
  }
})

test('subscription status labels are Chinese', () => {
  const sub = home.subscription
  for (const key of ['remain', 'expire', 'expired', 'noExpire', 'noTraffic', 'unknown', 'daysLeft', 'warnBoth', 'warnExpire', 'warnTraffic']) {
    assert.equal(typeof sub[key], 'string')
    assert.equal(sub[key].includes('home.subscription'), false)
    assert.match(sub[key], /[\u4e00-\u9fff]/)
  }
})

test('the strategy sheet covers the traffic chart instead of showing through it', () => {
  const css = readFileSync(new URL('../src/assets/styles/glass.css', import.meta.url), 'utf8')
  assert.match(css, /\.home-view > \.controller-panel\s*\{[^}]*background:\s*var\(--base-gradient\)/)
  assert.match(css, /#app \.controller-panel \.lux-row\.sticky\s*\{[^}]*background:\s*#152033/)
})
