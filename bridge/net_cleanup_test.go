package bridge

import (
	"strings"
	"testing"
)

func TestLeftoverTunAdapter(t *testing.T) {
	yes := [][2]string{
		{"sing-box", ""},
		{"singbox", "Virtual"},
		{"tun0", "TAP-Windows Adapter"},
		{"TUN", ""},
		{"wintun", ""},
		{"Clash", "Intel"},
		{"本地连接", "Wintun Userspace Tunnel"},
		{"eth0", "sing-box Tunnel"},
	}
	for _, pair := range yes {
		if !isLeftoverTunAdapter(pair[0], pair[1]) {
			t.Fatalf("expected tun %q %q", pair[0], pair[1])
		}
	}
	no := [][2]string{
		{"Ethernet", "Realtek PCIe GbE Family Controller"},
		{"Wi-Fi", "Intel(R) Wi-Fi 6"},
		{"wg0", "WireGuard Tunnel"},
		{"Meta", "Meta Tunnel"},
		{"sing", "Something"},
		{"tunnel", ""},
		{"single", ""},
	}
	for _, pair := range no {
		if isLeftoverTunAdapter(pair[0], pair[1]) {
			t.Fatalf("ordinary adapter matched %q %q", pair[0], pair[1])
		}
	}
}

func TestLoopbackProxyAndTunDNS(t *testing.T) {
	for _, server := range []string{"127.0.0.1:7890", "socks=127.0.0.1:2080", "localhost:1080", "[::1]:1080", "::1"} {
		if !isLoopbackProxy(server) {
			t.Fatalf("expected loopback %q", server)
		}
	}
	for _, server := range []string{"", "192.168.1.1:8080", "10.0.0.1:7890", "8.8.8.8:80"} {
		if isLoopbackProxy(server) {
			t.Fatalf("not loopback %q", server)
		}
	}
	for _, server := range []string{"172.18.0.2", "172.19.0.2", "198.18.0.2", "198.19.1.1"} {
		if !isTunDNS(server) {
			t.Fatalf("expected tun dns %q", server)
		}
	}
	for _, server := range []string{"8.8.8.8", "1.1.1.1", "172.16.0.1", "192.168.1.1"} {
		if isTunDNS(server) {
			t.Fatalf("ordinary dns %q", server)
		}
	}
}

func TestWindowsCleanupScriptLeavesPhysicalRoutes(t *testing.T) {
	script := windowsNetworkCleanupScript()
	for _, want := range []string{
		"Wintun",
		"0.0.0.0/1",
		"128.0.0.0/1",
		"ProxyEnable",
		"flushdns",
		"InternetSetOption",
		"winhttp",
		tunNamePattern,
		tunDNSPattern,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q", want)
		}
	}
	for _, banned := range []string{
		"NextHop -eq '0.0.0.0'",
		"NextHop -eq \"0.0.0.0\"",
		"WireGuard",
		"|Meta|",
	} {
		if strings.Contains(script, banned) {
			t.Fatalf("script still contains %q", banned)
		}
	}
}
