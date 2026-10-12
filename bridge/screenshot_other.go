//go:build !windows

package bridge

func applyScreenshotHotkeys(shot, translate, deepseekKey, coreProxy string) FlagResult {
	_, _, note := reviewHotkeys(shot, translate)
	if note != "" {
		return FlagResult{false, note}
	}
	return FlagResult{false, "截图快捷键仅在 Windows 上可用"}
}
