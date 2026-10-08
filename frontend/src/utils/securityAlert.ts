export interface SecurityAlertItem {
  id: string
  level: string
  text: string
}

/** Same id with different text is a different problem, so the blink starts again. */
export const securityProblemKey = (item: SecurityAlertItem) => `${item.id}\n${item.text}`

export const redProblemKeys = (items: SecurityAlertItem[]) =>
  items.filter((item) => item.level === 'red').map(securityProblemKey)

/** Drop acknowledgements for problems that are gone. A problem that comes back later blinks again. */
export const reconcileAck = (acked: string[], redKeys: string[]) => {
  const live = new Set(redKeys)
  return acked.filter((key) => live.has(key))
}

export const isUnackedRed = (item: SecurityAlertItem, acked: string[]) =>
  item.level === 'red' && !acked.includes(securityProblemKey(item))

export const hasUnackedRed = (items: SecurityAlertItem[], acked: string[]) =>
  items.some((item) => isUnackedRed(item, acked))

export const acknowledgeProblem = (acked: string[], item: SecurityAlertItem) => {
  if (item.level !== 'red') return acked
  const key = securityProblemKey(item)
  if (acked.includes(key)) return acked
  return [...acked, key]
}
