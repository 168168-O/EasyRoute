//go:build !windows

package bridge

func collectSecurity(int, string) securityReport {
	return unsupportedSecurity()
}
