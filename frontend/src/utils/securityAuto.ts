import { watch } from 'vue'

import { EventsOn, IsNotificationAvailable, SecurityCheck, SendNotification } from '@/bridge'
import { OS } from '@/enums/app'
import { useEnvStore, useKernelApiStore } from '@/stores'
import { updateTrayAndMenus } from '@/utils'

import { openSecurityCheck } from './securityFocus'
import { publishSecurityItems, securityAlerting } from './securityAlertState'
import {
  SECURITY_FIRST_DELAY_MS,
  SECURITY_INTERVAL_MS,
  SECURITY_NOTICE_ID,
  SECURITY_NOTICE_TEXT,
  runSecuritySweep,
  startSecurityTimer,
} from './securitySchedule'

const NOTIFIED_KEY = 'dhagn-security-notified'

const readNotified = () => {
  try {
    const parsed = JSON.parse(localStorage.getItem(NOTIFIED_KEY) || '[]') as unknown
    return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === 'string') : []
  } catch {
    return []
  }
}

const writeNotified = (keys: string[]) => {
  try {
    localStorage.setItem(NOTIFIED_KEY, JSON.stringify(keys))
  } catch {
    // The next sweep will announce again if the store is unavailable.
  }
}

const sendSecurityNotice = async () => {
  try {
    if (!(await IsNotificationAvailable())) return false
    await SendNotification({ id: SECURITY_NOTICE_ID, title: SECURITY_NOTICE_TEXT })
    return true
  } catch {
    return false
  }
}

/** DNS, proxy, hosts, virtual adapters, and the gateway ping. No site probes. */
export const sweepSecurity = async () => {
  const kernel = useKernelApiStore()
  const env = useEnvStore()
  const port = Number(kernel.config['mixed-port'] || 0)
  const tun = kernel.config.tun?.device || 'tun0'
  const next = await runSecuritySweep({
    check: () => SecurityCheck(port, tun),
    publish: publishSecurityItems,
    previous: readNotified(),
    windows: env.env.os === OS.Windows,
    notify: sendSecurityNotice,
  })
  writeNotified(next)
}

let started = false

export const bindSecurityAuto = () => {
  if (started || typeof window === 'undefined') return () => {}
  started = true

  const offTray = EventsOn('onTrayClick', () => {
    if (securityAlerting.value) void openSecurityCheck()
  })
  const offNotice = EventsOn('onSecurityNotice', () => {
    void openSecurityCheck()
  })
  const stopWatch = watch(securityAlerting, () => {
    void updateTrayAndMenus()
  })
  const stopTimer = startSecurityTimer({
    firstDelay: SECURITY_FIRST_DELAY_MS,
    interval: SECURITY_INTERVAL_MS,
    run: sweepSecurity,
  })

  return () => {
    offTray()
    offNotice()
    stopWatch()
    stopTimer()
    started = false
  }
}
