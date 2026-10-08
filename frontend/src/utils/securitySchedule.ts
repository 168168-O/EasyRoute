import { redProblemKeys, type SecurityAlertItem } from './securityAlert.ts'

/** One minute after the window is up, then every six hours. */
export const SECURITY_FIRST_DELAY_MS = 60 * 1000
export const SECURITY_INTERVAL_MS = 6 * 60 * 60 * 1000

export const SECURITY_NOTICE_ID = 'dhagn-security-alert'
export const SECURITY_NOTICE_TEXT = 'VPN 安全检查发现问题，点击查看'

export const noticeForReds = (notified: string[], redKeys: string[]) => {
  const live = new Set(redKeys)
  const kept = notified.filter((key) => live.has(key))
  const fresh = redKeys.filter((key) => !kept.includes(key))
  return { notify: fresh.length > 0, notified: [...kept, ...fresh] }
}

/** A failed toast must not mark the new problem as already announced. */
export const storedNoticeKeys = (previous: string[], redKeys: string[], sent: boolean) => {
  const decision = noticeForReds(previous, redKeys)
  if (decision.notify && !sent) {
    const live = new Set(redKeys)
    return previous.filter((key) => live.has(key))
  }
  return decision.notified
}

export const runSecuritySweep = async <T extends SecurityAlertItem>(deps: {
  check: () => Promise<{ items?: T[] }>
  publish: (items: T[]) => void
  previous: string[]
  windows: boolean
  notify: () => Promise<boolean>
}) => {
  const report = await deps.check()
  const items = report.items || []
  deps.publish(items)
  const red = redProblemKeys(items)
  const decision = noticeForReds(deps.previous, red)
  let sent = !decision.notify || !deps.windows
  if (decision.notify && deps.windows) sent = await deps.notify()
  return storedNoticeKeys(deps.previous, red, sent)
}

export const startSecurityTimer = (opts: {
  firstDelay: number
  interval: number
  run: () => Promise<void>
  setTimer?: (fn: () => void, ms: number) => unknown
  clearTimer?: (handle: unknown) => void
}) => {
  const setTimer = opts.setTimer ?? ((fn, ms) => setTimeout(fn, ms))
  const clearTimer = opts.clearTimer ?? ((handle) => clearTimeout(handle as ReturnType<typeof setTimeout>))
  let stopped = false
  let handle: unknown

  const arm = (delay: number) => {
    handle = setTimer(() => {
      void (async () => {
        if (stopped) return
        try {
          await opts.run()
        } catch {
          // A failed read stays quiet and tries again on the next interval.
        }
        if (stopped) return
        arm(opts.interval)
      })()
    }, delay)
  }

  arm(opts.firstDelay)
  return () => {
    stopped = true
    clearTimer(handle)
  }
}
