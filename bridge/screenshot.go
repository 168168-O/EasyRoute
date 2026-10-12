package bridge

// ApplyScreenshotHotkeys registers the screenshot and screenshot-translate shortcuts.
// A true flag with text in Data is a warning (invalid combo, conflict, or the key is taken).
// A false flag means the hotkey service itself could not start.
func (a *App) ApplyScreenshotHotkeys(shot, translate, deepseekKey, coreProxy string) FlagResult {
	return applyScreenshotHotkeys(shot, translate, deepseekKey, coreProxy)
}
