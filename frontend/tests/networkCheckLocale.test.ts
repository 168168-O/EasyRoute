import assert from 'node:assert/strict'
import test from 'node:test'

import zh from '../src/lang/locale/zh.ts'

const keys = [
  'title',
  'start',
  'openSpeed',
  'openDns',
  'openIp',
  'domainPh',
  'lookup',
  'idle',
] as const

test('overview network check is Chinese, not raw i18n keys', () => {
  const block = zh.home.networkCheck
  for (const key of keys) {
    const text = block[key]
    assert.equal(typeof text, 'string')
    assert.notEqual(text, `home.networkCheck.${key}`)
    assert.equal(text.includes('home.networkCheck'), false)
    assert.equal(/[\u4e00-\u9fff]/.test(text), true)
  }
})
