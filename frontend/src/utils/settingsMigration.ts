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
  return settings
}
