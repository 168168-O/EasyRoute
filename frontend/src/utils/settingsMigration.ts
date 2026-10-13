/**
 * Product shell. Glass + the bull head are always on.
 * A saved light/custom theme cannot drop an existing install back to the stock look.
 * Other choices (exit, sort, menus, simple mode) stay once the user has set them.
 */
export const applyGlassBullBaseline = (settings: Record<string, any>) => {
  settings.theme = 'dark'
  settings.themePalette = 'gold-sea'
  settings.color = 'default'
  settings.dynamicBackground = true
  return settings
}

/** One-time product defaults. A later switch by the user is kept. */
export const migrateAppSettings = (settings: Record<string, any>, existed: boolean) => {
  if (!existed || settings.exitOnCloseMigrated !== true) {
    settings.exitOnClose = false
    settings.exitOnCloseMigrated = true
  }
  settings.kernel = settings.kernel || {}
  if (!existed || settings.sortByDelayDefaulted !== true) {
    settings.kernel.sortByDelay = true
    settings.sortByDelayDefaulted = true
  }
  if (!existed || settings.addGroupToMenuDefaulted !== true) {
    settings.addGroupToMenu = true
    settings.addGroupToMenuDefaulted = true
  }
  if (settings.simpleMode === undefined) settings.simpleMode = true
  if (settings.domesticDirect === undefined) settings.domesticDirect = true
  // First launch after this update prefers the system proxy and leaves TUN off.
  // A later choice to turn TUN on is kept.
  if (!existed || settings.preferSystemProxyDefaulted !== true) {
    settings.tunEnabled = false
    settings.autoSetSystemProxy = true
    settings.preferSystemProxyDefaulted = true
  }
  applyGlassBullBaseline(settings)
  return settings
}
