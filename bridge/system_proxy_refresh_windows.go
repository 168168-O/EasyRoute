//go:build windows

package bridge

import "golang.org/x/sys/windows"

// refreshWindowsInternetSettings tells WinINET to re-read the registry proxy.
func refreshWindowsInternetSettings() {
	proc := windows.NewLazySystemDLL("wininet.dll").NewProc("InternetSetOptionW")
	// INTERNET_OPTION_SETTINGS_CHANGED = 39, INTERNET_OPTION_REFRESH = 37.
	proc.Call(0, 39, 0, 0)
	proc.Call(0, 37, 0, 0)
}
