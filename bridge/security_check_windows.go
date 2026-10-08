//go:build windows

package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

func collectSecurity(ownPort int, tunName string) securityReport {
	facts := securityFacts{OwnPort: ownPort, TunNames: securityTunNames(tunName)}
	if raw, err := runHidden(8*time.Second, "powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", securityProbeScript); err == nil {
		applySecurityProbe(&facts, raw)
	}
	applyHosts(&facts)
	if ip := net.ParseIP(canonicalIP(facts.Gateway)); ip != nil && ip.To4() != nil && !ip.IsUnspecified() {
		output, err := runHidden(8*time.Second, "ping", "-n", "5", "-w", "800", ip.To4().String())
		facts.PingOutput = output
		facts.PingFailed = err != nil && strings.TrimSpace(output) == ""
	}
	return evaluateSecurity(facts)
}

func securityTunNames(extra string) []string {
	names := []string{"tun0"}
	extra = strings.TrimSpace(extra)
	if extra != "" && !strings.EqualFold(extra, "tun0") {
		names = append(names, extra)
	}
	return names
}

const securityProbeScript = `
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = 'SilentlyContinue'
$adapters = @(Get-NetAdapter | ForEach-Object {
  [PSCustomObject]@{
    Name = [string]$_.Name
    Description = [string]$_.InterfaceDescription
    Virtual = [bool]$_.Virtual
    Hardware = [bool]$_.HardwareInterface
    Status = [string]$_.Status
  }
})
$dhcp = @{}
Get-NetIPInterface -AddressFamily IPv4 | ForEach-Object { $dhcp[$_.InterfaceAlias] = [string]$_.Dhcp }
$dns = @(Get-DnsClientServerAddress -AddressFamily IPv4 | ForEach-Object {
  [PSCustomObject]@{
    Name = [string]$_.InterfaceAlias
    Servers = @($_.ServerAddresses)
    Dhcp = [string]$dhcp[$_.InterfaceAlias]
  }
})
$route = Get-NetRoute -DestinationPrefix '0.0.0.0/0' -AddressFamily IPv4 | Sort-Object { $_.RouteMetric + $_.InterfaceMetric } | Select-Object -First 1
$proxy = Get-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings'
[PSCustomObject]@{
  adapters = $adapters
  dns = $dns
  gateway = [string]$route.NextHop
  proxyEnable = [int]$proxy.ProxyEnable
  proxyServer = [string]$proxy.ProxyServer
  autoConfig = [string]$proxy.AutoConfigURL
} | ConvertTo-Json -Compress -Depth 5
`

func runHidden(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	SetCmdWindowHidden(cmd)
	out, err := cmd.Output()
	text := decodeCommandOutput(out)
	if err != nil {
		if len(strings.TrimSpace(text)) > 0 {
			return text, err
		}
		return "", err
	}
	return text, nil
}

func decodeCommandOutput(out []byte) string {
	if len(out) >= 2 && out[0] == 0xFF && out[1] == 0xFE {
		return strings.TrimSpace(string(utf16LE(out[2:])))
	}
	if len(out) >= 3 && out[0] == 0xEF && out[1] == 0xBB && out[2] == 0xBF {
		out = out[3:]
	}
	return strings.TrimSpace(string(out))
}

func utf16LE(raw []byte) []rune {
	if len(raw)%2 == 1 {
		raw = raw[:len(raw)-1]
	}
	out := make([]rune, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		out = append(out, rune(raw[i])|rune(raw[i+1])<<8)
	}
	return out
}

type securityProbeJSON struct {
	Adapters    json.RawMessage `json:"adapters"`
	DNS         json.RawMessage `json:"dns"`
	Gateway     string          `json:"gateway"`
	ProxyEnable int             `json:"proxyEnable"`
	ProxyServer string          `json:"proxyServer"`
	AutoConfig  string          `json:"autoConfig"`
}

func applySecurityProbe(facts *securityFacts, raw string) {
	var probe securityProbeJSON
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		return
	}
	facts.ReadAdapters = true
	facts.Gateway = probe.Gateway
	facts.ProxyEnable = probe.ProxyEnable != 0
	facts.ProxyServer = probe.ProxyServer
	facts.AutoConfig = probe.AutoConfig
	for _, row := range rawObjects(probe.Adapters) {
		var adapter struct {
			Name        string `json:"Name"`
			Description string `json:"Description"`
			Virtual     bool   `json:"Virtual"`
			Hardware    bool   `json:"Hardware"`
			Status      string `json:"Status"`
		}
		if json.Unmarshal(row, &adapter) != nil {
			continue
		}
		status := strings.ToLower(adapter.Status)
		facts.Adapters = append(facts.Adapters, securityAdapter{
			Name:        adapter.Name,
			Description: adapter.Description,
			Virtual:     adapter.Virtual,
			Hardware:    adapter.Hardware,
			Up:          status == "up" || status == "已连接",
		})
	}
	for _, row := range rawObjects(probe.DNS) {
		var item struct {
			Name    string          `json:"Name"`
			Servers json.RawMessage `json:"Servers"`
			Dhcp    string          `json:"Dhcp"`
		}
		if json.Unmarshal(row, &item) != nil {
			continue
		}
		facts.DNS = append(facts.DNS, securityDNS{
			Name:    item.Name,
			Servers: rawStrings(item.Servers),
			DHCP:    strings.EqualFold(item.Dhcp, "Enabled"),
		})
	}
}

func rawObjects(raw json.RawMessage) []json.RawMessage {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if raw[0] == '[' {
		var rows []json.RawMessage
		if json.Unmarshal(raw, &rows) != nil {
			return nil
		}
		return rows
	}
	return []json.RawMessage{raw}
}

func rawStrings(raw json.RawMessage) []string {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if raw[0] == '[' {
		var rows []string
		if json.Unmarshal(raw, &rows) != nil {
			return nil
		}
		return rows
	}
	var one string
	if json.Unmarshal(raw, &one) != nil {
		return nil
	}
	if strings.TrimSpace(one) == "" {
		return nil
	}
	return []string{one}
}

func applyHosts(facts *securityFacts) {
	path := os.Getenv("SystemRoot")
	if path == "" {
		path = `C:\Windows`
	}
	path += `\System32\drivers\etc\hosts`
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			facts.HostsMissing = true
			return
		}
		facts.HostsUnreadable = true
		return
	}
	defer file.Close()
	buf := make([]byte, 512<<10)
	n, err := file.Read(buf)
	if n == 0 && err != nil && !errors.Is(err, io.EOF) {
		facts.HostsUnreadable = true
		return
	}
	facts.Hosts = string(buf[:n])
}
