const DAY = 24 * 60 * 60 * 1000

export interface SubscriptionSnapshot {
  name: string
  disabled?: boolean
  upload: number
  download: number
  total: number
  expire: number
}

export interface SubscriptionHealth {
  name: string
  remain: number
  total: number
  daysLeft: number | null
  warn: boolean
  reason: '' | 'expire' | 'traffic' | 'both'
}

export const subscriptionHealth = (sub: SubscriptionSnapshot, now = Date.now()): SubscriptionHealth | null => {
  if (sub.disabled) return null
  const used = Math.max(0, (sub.upload || 0) + (sub.download || 0))
  const total = Math.max(0, sub.total || 0)
  const remain = total > 0 ? Math.max(0, total - used) : 0
  const daysLeft = sub.expire > 0 ? Math.ceil((sub.expire - now) / DAY) : null
  const trafficLow = total > 0 && remain / total < 0.1
  const expireSoon = daysLeft != null && daysLeft < 7
  const reason = trafficLow && expireSoon ? 'both' : expireSoon ? 'expire' : trafficLow ? 'traffic' : ''
  return { name: sub.name, remain, total, daysLeft, warn: reason !== '', reason }
}
