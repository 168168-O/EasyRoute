import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

import {
  SECURITY_FIRST_DELAY_MS,
  SECURITY_INTERVAL_MS,
  SECURITY_NOTICE_TEXT,
  noticeForReds,
  runSecuritySweep,
  startSecurityTimer,
  storedNoticeKeys,
} from '../src/utils/securitySchedule.ts'
import { trayIconFile } from '../src/utils/trayIcon.ts'

const hosts = { id: 'hosts', level: 'red', text: 'hosts 改写了 google.com。' }
const hostsKey = 'hosts\nhosts 改写了 google.com。'

test('the first sweep is about a minute later, then every six hours', async () => {
  assert.equal(SECURITY_FIRST_DELAY_MS, 60_000)
  assert.equal(SECURITY_INTERVAL_MS, 6 * 60 * 60 * 1000)
  const delays: number[] = []
  const runs: number[] = []
  let pending: (() => void) | undefined
  const stop = startSecurityTimer({
    firstDelay: SECURITY_FIRST_DELAY_MS,
    interval: SECURITY_INTERVAL_MS,
    run: async () => {
      runs.push(runs.length + 1)
    },
    setTimer: (fn, ms) => {
      delays.push(ms)
      pending = fn
      return delays.length
    },
    clearTimer: () => {
      pending = undefined
    },
  })
  assert.deepEqual(delays, [60_000])
  pending?.()
  await new Promise((resolve) => setTimeout(resolve, 0))
  assert.deepEqual(runs, [1])
  assert.equal(delays.at(-1), SECURITY_INTERVAL_MS)
  pending?.()
  await new Promise((resolve) => setTimeout(resolve, 0))
  assert.deepEqual(runs, [1, 2])
  const parked = pending
  stop()
  parked?.()
  await new Promise((resolve) => setTimeout(resolve, 0))
  assert.deepEqual(runs, [1, 2])
})

test('a failed sweep still schedules the next one', async () => {
  const delays: number[] = []
  let pending: (() => void) | undefined
  startSecurityTimer({
    firstDelay: 60_000,
    interval: SECURITY_INTERVAL_MS,
    run: async () => {
      throw new Error('powershell failed')
    },
    setTimer: (fn, ms) => {
      delays.push(ms)
      pending = fn
      return 1
    },
    clearTimer: () => undefined,
  })
  pending?.()
  await new Promise((resolve) => setTimeout(resolve, 0))
  assert.deepEqual(delays, [60_000, SECURITY_INTERVAL_MS])
})

test('the same red problem notifies once, a new one notifies again', async () => {
  assert.equal(SECURITY_NOTICE_TEXT, 'VPN 安全检查发现问题，点击查看')
  const first = noticeForReds([], [hostsKey])
  assert.equal(first.notify, true)
  const again = noticeForReds(first.notified, [hostsKey])
  assert.equal(again.notify, false)
  assert.deepEqual(again.notified, [hostsKey])

  const changed = 'hosts\nhosts 改写了 youtube.com。'
  const next = noticeForReds(again.notified, [changed])
  assert.equal(next.notify, true)
  assert.deepEqual(next.notified, [changed])

  const gone = noticeForReds(next.notified, [])
  assert.equal(gone.notify, false)
  assert.deepEqual(gone.notified, [])
  assert.equal(noticeForReds([], [hostsKey]).notify, true)
})

test('a toast that fails to send is retried, and yellow never sends one', async () => {
  assert.deepEqual(storedNoticeKeys([hostsKey], [hostsKey], false), [hostsKey])
  const fresh = storedNoticeKeys([], [hostsKey], false)
  assert.deepEqual(fresh, [])
  assert.deepEqual(storedNoticeKeys([], [hostsKey], true), [hostsKey])

  let notices = 0
  let published: { level: string }[] = []
  const saved = await runSecuritySweep({
    check: async () => ({ items: [{ id: 'dns', level: 'yellow', text: '不认识的 DNS' }] }),
    publish: (items) => {
      published = items
    },
    previous: [],
    windows: true,
    notify: async () => {
      notices += 1
      return true
    },
  })
  assert.equal(notices, 0)
  assert.equal(published[0]?.level, 'yellow')
  assert.deepEqual(saved, [])
})

test('only a new windows red sends the notice, and the sweep never starts site probes', async () => {
  const calls: string[] = []
  const saved = await runSecuritySweep({
    check: async () => {
      calls.push('light')
      return { items: [hosts, { id: 'dns', level: 'green', text: '路由器' }] }
    },
    publish: () => calls.push('publish'),
    previous: [],
    windows: true,
    notify: async () => {
      calls.push('notify')
      return true
    },
  })
  assert.deepEqual(calls, ['light', 'publish', 'notify'])
  assert.deepEqual(saved, [hostsKey])

  const quiet = await runSecuritySweep({
    check: async () => ({ items: [hosts] }),
    publish: () => undefined,
    previous: saved,
    windows: true,
    notify: async () => {
      calls.push('notify-again')
      return true
    },
  })
  assert.equal(calls.includes('notify-again'), false)
  assert.deepEqual(quiet, [hostsKey])

  let linuxNotices = 0
  await runSecuritySweep({
    check: async () => ({ items: [{ id: 'proxy', level: 'red', text: '指向别处' }] }),
    publish: () => undefined,
    previous: [],
    windows: false,
    notify: async () => {
      linuxNotices += 1
      return true
    },
  })
  assert.equal(linuxNotices, 0)

  const auto = readFileSync(new URL('../src/utils/securityAuto.ts', import.meta.url), 'utf8')
  assert.equal(auto.includes('runNetworkCheck'), false)
  assert.equal(auto.includes('SecurityCheck'), true)
})

test('an unacknowledged problem switches the tray icon to the red badge', () => {
  const calm = trayIconFile({
    os: 'windows',
    theme: 'dark',
    running: true,
    tun: true,
    proxy: false,
    alert: false,
  })
  const alert = trayIconFile({
    os: 'windows',
    theme: 'dark',
    running: true,
    tun: true,
    proxy: false,
    alert: true,
  })
  assert.equal(calm, 'data/.cache/icons/tray_tun_dark.ico')
  assert.equal(alert, 'data/.cache/icons/tray_tun_alert_dark.ico')
  assert.equal(
    trayIconFile({ os: 'linux', theme: 'light', running: false, tun: false, proxy: false, alert: true }),
    'data/.cache/imgs/tray_normal_alert_light.png',
  )
})
