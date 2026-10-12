//go:build windows

package bridge

import (
	"strings"
	"time"
)

// CleanupNetworkAfterStop clears leftover TUN adapters / proxy residue after
// sing-box exits so the PC can reach the internet again without a full
// "network reset". It never touches WeChat/Douyin process rules.
func (a *App) CleanupNetworkAfterStop() FlagResult {
	script := `
$ErrorActionPreference = 'SilentlyContinue'
# Drop orphan Wintun / sing-box style adapters left after an unclean TUN stop.
Get-NetAdapter | Where-Object {
  $_.InterfaceDescription -match 'Wintun|WireGuard|Meta|sing-box|singbox|Clash|TunSafe' -or
  $_.Name -match '^(sing|tun|Meta|wintun|clash)'
} | ForEach-Object {
  try { Disable-NetAdapter -Name $_.Name -Confirm:$false } catch {}
  try {
    $pnputil = Join-Path $env:SystemRoot 'System32\pnputil.exe'
    # Prefer Disable; Remove-NetAdapter is often denied for virtual NICs.
  } catch {}
}
# If a default route still points at a missing/disconnected interface, remove only those bad routes.
Get-NetRoute -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object {
  $_.DestinationPrefix -eq '0.0.0.0/0' -and
  (
    $_.NextHop -eq '0.0.0.0' -or
    -not (Get-NetAdapter -InterfaceIndex $_.InterfaceIndex -ErrorAction SilentlyContinue) -or
    ((Get-NetAdapter -InterfaceIndex $_.InterfaceIndex -ErrorAction SilentlyContinue).Status -eq 'Disconnected')
  )
} | ForEach-Object {
  try {
    Remove-NetRoute -DestinationPrefix $_.DestinationPrefix -InterfaceIndex $_.InterfaceIndex -NextHop $_.NextHop -Confirm:$false
  } catch {}
}
try { ipconfig /flushdns | Out-Null } catch {}
'ok'
`
	out := a.Exec(`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`, []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script}, ExecOptions{})
	if !out.Flag {
		// Still report success to the UI path — proxy clear already ran; this is best-effort.
		return FlagResult{Flag: true, Data: "cleanup-partial:" + out.Data}
	}
	time.Sleep(200 * time.Millisecond)
	return FlagResult{Flag: true, Data: strings.TrimSpace(out.Data)}
}
