import { nextTick } from 'vue'

import { ShowMainWindow } from '@/bridge'
import router from '@/router'

export const SECURITY_FOCUS = 'dhagn-focus-security'

/** Show the window and land on the 网络检测 card, where the security rows live. */
export const openSecurityCheck = async () => {
  try {
    await ShowMainWindow()
  } catch {
    // The preview page has no desktop window.
  }
  if (router.currentRoute.value.name !== 'Overview') {
    await router.push({ name: 'Overview' })
  }
  await nextTick()
  window.dispatchEvent(new Event(SECURITY_FOCUS))
}
