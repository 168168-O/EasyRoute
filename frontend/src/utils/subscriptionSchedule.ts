export const SUBSCRIPTION_UPDATE_TASK_ID = 'dhagn-sub-12h'

/** Every 12 hours. notification stays off so a failed refresh is silent. */
export const subscriptionUpdateTask = () => ({
  id: SUBSCRIPTION_UPDATE_TASK_ID,
  name: '订阅自动更新',
  type: 'update::all::subscription' as const,
  subscriptions: [] as string[],
  rulesets: [] as string[],
  plugins: [] as string[],
  script: '',
  cron: '0 0 */12 * * *',
  notification: false,
  disabled: false,
  lastTime: 0,
})
