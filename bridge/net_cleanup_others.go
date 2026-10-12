//go:build !windows

package bridge

// CleanupNetworkAfterStop is a no-op off Windows; system proxy clear is enough.
func (a *App) CleanupNetworkAfterStop() FlagResult {
	return FlagResult{Flag: true, Data: "noop"}
}
