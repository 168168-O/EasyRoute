package bridge

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/process"
)

// Leftover TUN adapters only. A physical NIC, WireGuard, or a bare "Meta"
// name is never treated as ours.
const (
	tunDescriptionPattern = `Wintun|sing-box|singbox|Clash|TunSafe`
	tunNamePattern        = `(?i)^(sing-box|singbox|sing-tun|wintun|clash|tun\d*)$`
	tunDNSPattern         = `^(172\.(18|19)\.|198\.(18|19)\.)`
)

var (
	tunDescriptionRe = regexp.MustCompile(tunDescriptionPattern)
	tunNameRe        = regexp.MustCompile(tunNamePattern)
	tunDNSRe         = regexp.MustCompile(tunDNSPattern)
)

// isLeftoverTunAdapter reports adapters created by sing-box / Clash TUN.
// It does not match WireGuard or an ordinary Ethernet/Wi-Fi adapter.
func isLeftoverTunAdapter(name, description string) bool {
	if tunDescriptionRe.MatchString(description) {
		return true
	}
	return tunNameRe.MatchString(strings.TrimSpace(name))
}

// isLoopbackProxy is a system proxy still aimed at this app (127.0.0.1 / localhost / ::1).
func isLoopbackProxy(server string) bool {
	s := strings.TrimSpace(strings.ToLower(server))
	if s == "" {
		return false
	}
	if strings.Contains(s, "127.0.0.1") || strings.Contains(s, "localhost") || strings.Contains(s, "[::1]") {
		return true
	}
	return s == "::1" || strings.HasPrefix(s, "::1:")
}

// isTunDNS is a resolver left on the TUN subnet or the fake-ip range.
func isTunDNS(server string) bool {
	return tunDNSRe.MatchString(strings.TrimSpace(server))
}

// windowsNetworkCleanupScript removes hijack routes only from leftover TUN
// adapters (including 0.0.0.0/1 and 128.0.0.0/1), disables those adapters,
// resets fake-ip DNS on other adapters, and clears a loopback WinINET/WinHTTP
// proxy. It never deletes a physical NIC default route just because NextHop
// is 0.0.0.0.
func windowsNetworkCleanupScript() string {
	script := `
$ErrorActionPreference = 'SilentlyContinue'
$descRe = '{{DESC}}'
$nameRe = '{{NAME}}'
$dnsRe = '{{DNS}}'
$steal = @('0.0.0.0/0','0.0.0.0/1','128.0.0.0/1','::/0','::/1','8000::/1')

function Test-LeftoverTun([string]$name, [string]$desc) {
  if ($desc -match $descRe) { return $true }
  if ($name -match $nameRe) { return $true }
  return $false
}

$indexes = @{}
Get-NetAdapter -ErrorAction SilentlyContinue | ForEach-Object {
  if (Test-LeftoverTun ([string]$_.Name) ([string]$_.InterfaceDescription)) {
    $indexes[[int]$_.ifIndex] = [string]$_.Name
  }
}

Get-NetRoute -ErrorAction SilentlyContinue | ForEach-Object {
  $idx = [int]$_.InterfaceIndex
  $onTun = $indexes.ContainsKey($idx)
  $adapter = Get-NetAdapter -InterfaceIndex $idx -ErrorAction SilentlyContinue
  $prefix = [string]$_.DestinationPrefix
  $orphanSplit = (-not $adapter) -and ($steal -contains $prefix) -and ($prefix -ne '0.0.0.0/0') -and ($prefix -ne '::/0')
  if ($onTun -or $orphanSplit) {
    try {
      Remove-NetRoute -DestinationPrefix $prefix -InterfaceIndex $idx -NextHop $_.NextHop -Confirm:$false -ErrorAction SilentlyContinue
    } catch {}
  }
}

foreach ($name in @($indexes.Values)) {
  try { Disable-NetAdapter -Name $name -Confirm:$false -ErrorAction SilentlyContinue } catch {}
}

Get-DnsClientServerAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue | ForEach-Object {
  $idx = [int]$_.InterfaceIndex
  if ($indexes.ContainsKey($idx)) { return }
  $bad = $false
  foreach ($s in @($_.ServerAddresses)) {
    if ([string]$s -match $dnsRe) { $bad = $true }
  }
  if ($bad) {
    try { Set-DnsClientServerAddress -InterfaceIndex $idx -ResetServerAddresses -Confirm:$false } catch {}
  }
}

$reg = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings'
try {
  $server = [string](Get-ItemProperty -Path $reg -Name ProxyServer -ErrorAction SilentlyContinue).ProxyServer
  if ($server -match '(?i)(127\.0\.0\.1|localhost|\[::1\]|(^|\s)::1(:|$))') {
    Set-ItemProperty -Path $reg -Name ProxyEnable -Type DWord -Value 0
    Set-ItemProperty -Path $reg -Name ProxyServer -Value ''
  }
} catch {}

Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class DahuoInet {
  [DllImport("wininet.dll", SetLastError=true)]
  public static extern bool InternetSetOption(System.IntPtr h, int opt, System.IntPtr buf, int len);
}
'@
try { [DahuoInet]::InternetSetOption([IntPtr]::Zero, 39, [IntPtr]::Zero, 0) | Out-Null } catch {}
try { [DahuoInet]::InternetSetOption([IntPtr]::Zero, 37, [IntPtr]::Zero, 0) | Out-Null } catch {}

$netsh = Join-Path $env:SystemRoot 'System32\netsh.exe'
$winhttp = & $netsh winhttp show proxy | Out-String
if ($winhttp -match '(?i)(127\.0\.0\.1|localhost|\[::1\])') {
  & $netsh winhttp reset proxy | Out-Null
}

$ipconfig = Join-Path $env:SystemRoot 'System32\ipconfig.exe'
& $ipconfig /flushdns | Out-Null
'ok'
`
	script = strings.ReplaceAll(script, "{{DESC}}", tunDescriptionPattern)
	script = strings.ReplaceAll(script, "{{NAME}}", tunNamePattern)
	script = strings.ReplaceAll(script, "{{DNS}}", tunDNSPattern)
	return script
}

// SingBoxCoreRunning is true when the pid file still points at a live sing-box.
// Exit cleanup skips the network restore while that process is meant to stay up.
func SingBoxCoreRunning() bool {
	raw, err := os.ReadFile(resolvePath("data/sing-box/pid.txt"))
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return false
	}
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return false
	}
	name, err := proc.Name()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(name), "sing-box")
}
