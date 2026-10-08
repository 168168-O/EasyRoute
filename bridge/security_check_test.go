package bridge

import (
	"runtime"
	"strings"
	"testing"
)

func itemByID(items []securityItem, id string) securityItem {
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return securityItem{}
}

func TestSecurityLevelsForACleanMachine(t *testing.T) {
	report := evaluateSecurity(securityFacts{
		OwnPort:      7890,
		TunNames:     []string{"tun0"},
		ReadAdapters: true,
		Adapters: []securityAdapter{
			{Name: "以太网", Description: "Realtek PCIe", Hardware: true, Up: true},
			{Name: "tun0", Description: "Wintun", Virtual: true, Up: true},
			{Name: "Wi-Fi Direct", Description: "Microsoft Wi-Fi Direct Virtual Adapter", Virtual: true},
		},
		DNS: []securityDNS{{Name: "以太网", Servers: []string{"192.168.1.1", "223.5.5.5"}, DHCP: true}},
		Hosts: `
# comment
127.0.0.1 localhost
`,
		Gateway: "192.168.1.1",
		PingOutput: `
Packets: Sent = 5, Received = 5, Lost = 0 (0% loss),
Minimum = 1ms, Maximum = 4ms, Average = 2ms
`,
	})
	if !report.Supported {
		t.Fatal("expected a windows report")
	}
	for _, id := range []string{"dns", "proxy", "hosts", "adapters", "gateway"} {
		item := itemByID(report.Items, id)
		if item.Level != "green" {
			t.Fatalf("%s = %+v", id, item)
		}
	}
}

func TestSecurityFlagsHijacksAndWeakWiFi(t *testing.T) {
	report := evaluateSecurity(securityFacts{
		OwnPort:      7890,
		TunNames:     []string{"tun0"},
		ReadAdapters: true,
		Adapters: []securityAdapter{
			{Name: "WLAN", Description: "Intel Wi-Fi", Hardware: true, Up: true},
			{Name: "TAP", Description: "TAP-Windows Adapter V9", Virtual: true},
			{Name: "wg0", Description: "WireGuard Tunnel", Virtual: true},
		},
		DNS:         []securityDNS{{Name: "WLAN", Servers: []string{"127.0.0.1"}}},
		ProxyEnable: true,
		ProxyServer: "10.1.1.8:8888",
		AutoConfig:  "http://example.invalid/proxy.pac",
		Hosts:       "0.0.0.0 google.com www.youtube.com\n127.0.0.1 example.local\n",
		Gateway:     "192.168.0.1",
		PingOutput:  "最短 = 30ms，最长 = 80ms，平均 = 41ms\n丢失 = 1 (20% 丢失)",
	})
	dns := itemByID(report.Items, "dns")
	if dns.Level != "red" || !strings.Contains(dns.Text, "127.0.0.1") {
		t.Fatalf("dns = %+v", dns)
	}
	proxy := itemByID(report.Items, "proxy")
	if proxy.Level != "red" || !strings.Contains(proxy.Text, "自动配置") {
		t.Fatalf("proxy = %+v", proxy)
	}
	hosts := itemByID(report.Items, "hosts")
	if hosts.Level != "red" || !strings.Contains(hosts.Text, "google.com") || strings.Contains(hosts.Text, "example.local") {
		t.Fatalf("hosts = %+v", hosts)
	}
	adapters := itemByID(report.Items, "adapters")
	if adapters.Level != "yellow" || !strings.Contains(adapters.Text, "TAP") || !strings.Contains(adapters.Text, "wg0") {
		t.Fatalf("adapters = %+v", adapters)
	}
	gateway := itemByID(report.Items, "gateway")
	if gateway.Level != "yellow" || !strings.Contains(gateway.Text, "41ms") || !strings.Contains(gateway.Text, "丢了 1 次") {
		t.Fatalf("gateway = %+v", gateway)
	}
}

func TestSecurityAcceptsOurPortAndUnknownDNS(t *testing.T) {
	report := evaluateSecurity(securityFacts{
		OwnPort:      2080,
		ReadAdapters: true,
		Adapters:     []securityAdapter{{Name: "以太网", Description: "Realtek", Hardware: true, Up: true}},
		DNS:          []securityDNS{{Name: "以太网", Servers: []string{"1.2.3.4", "8.8.8.8"}}},
		ProxyEnable:  true,
		ProxyServer:  "http=127.0.0.1:2080;https=127.0.0.1:2080",
		HostsMissing: true,
		Gateway:      "10.0.0.1",
		PingOutput:   "Average = 20ms\n(0% loss)",
	})
	if itemByID(report.Items, "dns").Level != "yellow" {
		t.Fatalf("dns = %+v", itemByID(report.Items, "dns"))
	}
	if itemByID(report.Items, "proxy").Level != "green" {
		t.Fatalf("proxy = %+v", itemByID(report.Items, "proxy"))
	}
	hosts := itemByID(report.Items, "hosts")
	if hosts.Level != "yellow" || !strings.Contains(hosts.Text, "没有找到") {
		t.Fatalf("hosts = %+v", hosts)
	}
	if itemByID(report.Items, "gateway").Level != "green" {
		t.Fatalf("gateway = %+v", itemByID(report.Items, "gateway"))
	}
}

func TestSecurityGatewaySilenceIsRed(t *testing.T) {
	report := evaluateSecurity(securityFacts{
		ReadAdapters: true,
		Gateway:      "192.168.1.1",
		PingOutput:   "Packets: Sent = 5, Received = 0, Lost = 5 (100% loss),",
	})
	item := itemByID(report.Items, "gateway")
	if item.Level != "red" || !strings.Contains(item.Text, "没有回应") {
		t.Fatalf("gateway = %+v", item)
	}
}

func TestSecurityCheckOnThisOSDoesNotChangeSettings(t *testing.T) {
	report := collectSecurity(7890, "tun0")
	if runtime.GOOS == "windows" {
		if !report.Supported || len(report.Items) != 5 {
			t.Fatalf("windows report = %+v", report)
		}
		return
	}
	if report.Supported || len(report.Items) != 1 || report.Items[0].Level != "yellow" {
		t.Fatalf("other os report = %+v", report)
	}
}
