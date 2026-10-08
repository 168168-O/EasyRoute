import { computed, ref } from 'vue'

import type { SecurityItem } from '@/bridge'

import {
  acknowledgeProblem,
  hasUnackedRed,
  isUnackedRed,
  reconcileAck,
  redProblemKeys,
  type SecurityAlertItem,
} from './securityAlert.ts'

const ACK_KEY = 'dhagn-security-ack'

const readAck = () => {
  try {
    const parsed = JSON.parse(localStorage.getItem(ACK_KEY) || '[]') as unknown
    return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === 'string') : []
  } catch {
    return []
  }
}

const writeAck = (keys: string[]) => {
  try {
    localStorage.setItem(ACK_KEY, JSON.stringify(keys))
  } catch {
    // A full or blocked store still blinks for this session.
  }
}

const items = ref<SecurityItem[]>([])
const acked = ref<string[]>(typeof localStorage === 'undefined' ? [] : readAck())

export const securityAlertItems = items

export const securityAlerting = computed(() => hasUnackedRed(items.value, acked.value))

export const publishSecurityItems = (next: SecurityItem[]) => {
  const red = redProblemKeys(next)
  acked.value = reconcileAck(acked.value, red)
  writeAck(acked.value)
  items.value = next
}

export const acknowledgeSecurityItem = (item: SecurityAlertItem) => {
  const next = acknowledgeProblem(acked.value, item)
  if (next === acked.value) return
  acked.value = next
  writeAck(next)
}

export const securityItemAlerting = (item: SecurityAlertItem) => isUnackedRed(item, acked.value)

let visibilityBound = false

export const bindSecurityAlertVisibility = () => {
  if (visibilityBound || typeof document === 'undefined') return
  visibilityBound = true
  const apply = () => document.documentElement.classList.toggle('window-hidden', document.hidden)
  document.addEventListener('visibilitychange', apply)
  apply()
}
