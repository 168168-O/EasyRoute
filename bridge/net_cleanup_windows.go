//go:build windows

package bridge

import (
	"log"
	"strings"
	"time"
)

// CleanupNetworkAfterStop clears leftover TUN adapters / proxy residue after
// sing-box exits so the PC can reach the internet again without a full
// "network reset". It never touches WeChat/Douyin process rules.
func (a *App) CleanupNetworkAfterStop() FlagResult {
	script := windowsNetworkCleanupScript()
	out := a.Exec(`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`, []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script}, ExecOptions{})
	log.Printf("CleanupNetworkAfterStop: flag=%v data=%s", out.Flag, strings.TrimSpace(out.Data))
	if !out.Flag {
		// Still report success to the UI path — proxy clear already ran; this is best-effort.
		return FlagResult{Flag: true, Data: "cleanup-partial:" + out.Data}
	}
	time.Sleep(200 * time.Millisecond)
	return FlagResult{Flag: true, Data: strings.TrimSpace(out.Data)}
}
