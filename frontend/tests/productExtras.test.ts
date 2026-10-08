import assert from 'node:assert/strict'
import test from 'node:test'

import { explainDomainRoute, sampleBaseConfig, applyAppRouting } from '../src/utils/appRouting.ts'
import { attachCountryGroups, buildCountryUrltests, classifyNode } from '../src/utils/countryGroups.ts'
import { migrateAppSettings } from '../src/utils/settingsMigration.ts'
import { SUBSCRIPTION_UPDATE_TASK_ID, subscriptionUpdateTask } from '../src/utils/subscriptionSchedule.ts'
import { subscriptionHealth } from '../src/utils/subscriptionStatus.ts'

const DAY = 24 * 60 * 60 * 1000
const now = Date.UTC(2026, 9, 8)

test('existing exitOnClose true is migrated once and a later choice stays', () => {
  const first = migrateAppSettings({ exitOnClose: true, kernel: { sortByDelay: false }, addGroupToMenu: false }, true)
  assert.equal(first.exitOnClose, false)
  assert.equal(first.exitOnCloseMigrated, true)
  assert.equal(first.kernel.sortByDelay, true)
  assert.equal(first.addGroupToMenu, true)
  assert.equal(first.simpleMode, true)
  assert.equal(first.domesticDirect, true)

  const later = migrateAppSettings(
    {
      exitOnClose: true,
      exitOnCloseMigrated: true,
      sortByDelayDefaulted: true,
      addGroupToMenuDefaulted: true,
      addGroupToMenu: false,
      simpleMode: false,
      domesticDirect: false,
      kernel: { sortByDelay: false },
    },
    true,
  )
  assert.equal(later.exitOnClose, true)
  assert.equal(later.kernel.sortByDelay, false)
  assert.equal(later.addGroupToMenu, false)
  assert.equal(later.simpleMode, false)
  assert.equal(later.domesticDirect, false)
})

test('subscription health warns under 7 days or under 10 percent', () => {
  assert.equal(subscriptionHealth({ name: 'off', disabled: true, upload: 0, download: 0, total: 1, expire: 1 }, now), null)
  const soon = subscriptionHealth(
    { name: '快到期', upload: 0, download: 0, total: 100, expire: now + 3 * DAY },
    now,
  )
  assert.equal(soon?.warn, true)
  assert.equal(soon?.reason, 'expire')
  assert.equal(soon?.daysLeft, 3)

  const low = subscriptionHealth(
    { name: '流量少', upload: 50, download: 41, total: 100, expire: now + 30 * DAY },
    now,
  )
  assert.equal(low?.remain, 9)
  assert.equal(low?.reason, 'traffic')

  const both = subscriptionHealth(
    { name: '都紧', upload: 95, download: 0, total: 100, expire: now + 1 * DAY },
    now,
  )
  assert.equal(both?.reason, 'both')

  const ok = subscriptionHealth(
    { name: '够用', upload: 10, download: 0, total: 100, expire: now + 7 * DAY },
    now,
  )
  assert.equal(ok?.warn, false)
  assert.equal(ok?.daysLeft, 7)
})

test('built-in subscription task is silent and runs every 12 hours', () => {
  const task = subscriptionUpdateTask()
  assert.equal(task.id, SUBSCRIPTION_UPDATE_TASK_ID)
  assert.equal(task.cron, '0 0 */12 * * *')
  assert.equal(task.notification, false)
  assert.equal(task.disabled, false)
  assert.equal(task.type, 'update::all::subscription')
})

test('domain lookup keeps WeChat and Douyin on their saved rule', () => {
  const wechat = explainDomainRoute('https://weixin.qq.com/a', { pinned: { wechat: 'direct', douyin: 'proxy' } })
  assert.equal(wechat[0]?.scope, '微信')
  assert.equal(wechat[0]?.route, 'direct')
  assert.equal(wechat[1]?.route, 'direct')
  const qq = explainDomainRoute('qq.com')
  assert.equal(qq[0]?.scope, '微信')
  const douyin = explainDomainRoute('www.douyin.com', { pinned: { wechat: 'direct', douyin: 'proxy' } })
  assert.equal(douyin[0]?.scope, '抖音')
  assert.equal(douyin[0]?.route, 'proxy')

  const baidu = explainDomainRoute('www.baidu.com')
  assert.deepEqual(
    baidu.map((row) => row.route),
    ['direct', 'direct'],
  )
  assert.equal(baidu[0]?.rule, '国内网站走本地')
  const off = explainDomainRoute('baidu.com', { domesticDirect: false })
  assert.equal(off[0]?.route, 'proxy')
  assert.equal(off[1]?.route, 'direct')

  for (const host of ['telegram.org', 'https://t.me/x']) {
    const rows = explainDomainRoute(host)
    assert.equal(rows[0]?.scope, '走代理的程序', host)
    assert.equal(rows[0]?.route, 'proxy', host)
    assert.equal(rows[1]?.route, 'direct', host)
  }
  assert.deepEqual(explainDomainRoute('1.2.3.4'), [])
  assert.deepEqual(explainDomainRoute('qpic.cn')[0]?.scope, '微信')
})

test('country groups pick a region and sit in front of 节点选择', () => {
  assert.equal(classifyNode('HK-01'), 'hk')
  assert.equal(classifyNode('NHK'), 'other')
  assert.equal(classifyNode('日本-东京'), 'jp')
  assert.equal(classifyNode('Singapore 1'), 'sg')
  assert.equal(classifyNode('台湾-01'), 'tw')
  assert.equal(classifyNode('USA-1'), 'us')
  assert.equal(classifyNode('英国'), 'other')

  const groups = buildCountryUrltests(['HK-01', '日本-01', '备用'])
  assert.deepEqual(
    groups.map((group) => group.tag),
    ['🇭🇰 香港', '🇯🇵 日本', '🌐 其他'],
  )
  assert.equal(groups.every((group) => group.interval === '10m' && group.tolerance === 50), true)
  assert.equal(groups.some((group) => group.tag.includes('新加坡')), false)

  const base = sampleBaseConfig()
  const routed = applyAppRouting(base, {
    programs: [{ id: 'chrome', name: 'Chrome', exe: 'chrome.exe', mode: 'proxy' }],
  })
  const outbounds = attachCountryGroups([
    {
      type: 'selector',
      tag: '🚀 节点选择',
      outbounds: ['自动选择', 'direct'],
    },
    { type: 'urltest', tag: '自动选择', outbounds: ['HK-01', '日本-01', '🇸🇬 新加坡A'] },
    ...routed.outbounds,
  ])
  const select = outbounds.find((item) => item.tag === '🚀 节点选择')
  assert.deepEqual(select?.outbounds?.slice(0, 3), ['🇭🇰 香港', '🇯🇵 日本', '🇸🇬 新加坡'])
  assert.equal(select?.outbounds?.includes('自动选择'), true)
  assert.equal(outbounds.filter((item) => item.tag === '🇭🇰 香港').length, 1)
  const again = attachCountryGroups(outbounds)
  assert.equal(again.filter((item) => item.tag === '🇭🇰 香港').length, 1)
})
