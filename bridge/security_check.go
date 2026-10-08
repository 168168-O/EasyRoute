package bridge

import (
	"encoding/json"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// SecurityCheck is a one-shot, read-only look at the Windows network setup.
// It does not change settings and it does not stay running.

type securityItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level string `json:"level"`
	Text  string `json:"text"`
}

type securityReport struct {
	Supported bool           `json:"supported"`
	Items     []securityItem `json:"items"`
}

type securityAdapter struct {
	Name        string
	Description string
	Virtual     bool
	Hardware    bool
	Up          bool
}

type securityDNS struct {
	Name    string
	Servers []string
	DHCP    bool
}

type securityFacts struct {
	OwnPort         int
	TunNames        []string
	ReadAdapters    bool
	Adapters        []securityAdapter
	DNS             []securityDNS
	ProxyEnable     bool
	ProxyServer     string
	AutoConfig      string
	Hosts           string
	HostsMissing    bool
	HostsUnreadable bool
	Gateway         string
	PingOutput      string
	PingFailed      bool
}

func (a *App) SecurityCheck(ownPort int, tunName string) FlagResult {
	report := collectSecurity(ownPort, tunName)
	raw, err := json.Marshal(report)
	if err != nil {
		return FlagResult{Flag: false, Data: "安全检查没有完成"}
	}
	return FlagResult{Flag: true, Data: string(raw)}
}

func unsupportedSecurity() securityReport {
	return securityReport{
		Supported: false,
		Items: []securityItem{{
			ID:    "platform",
			Name:  "安全检查",
			Level: "yellow",
			Text:  "安全检查只在 Windows 上读取本机设置。",
		}},
	}
}

func evaluateSecurity(facts securityFacts) securityReport {
	return securityReport{
		Supported: true,
		Items: []securityItem{
			evaluateDNS(facts),
			evaluateProxy(facts),
			evaluateHosts(facts),
			evaluateAdapters(facts),
			evaluateGateway(facts),
		},
	}
}

var knownPublicDNS = map[string]struct{}{
	"223.5.5.5": {}, "223.6.6.6": {},
	"119.29.29.29": {}, "119.28.28.28": {},
	"114.114.114.114": {}, "114.114.115.115": {},
	"180.76.76.76": {},
	"1.1.1.1":      {}, "1.0.0.1": {},
	"8.8.8.8": {}, "8.8.4.4": {},
	"9.9.9.9": {}, "149.112.112.112": {},
	"208.67.222.222": {}, "208.67.220.220": {},
	"2400:3200::1": {}, "2400:3200:baba::1": {},
}

func canonicalIP(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.String()
}

func isRouterDNS(ip net.IP) bool {
	return ip.IsPrivate()
}

func evaluateDNS(facts securityFacts) securityItem {
	item := securityItem{ID: "dns", Name: "DNS"}
	if !facts.ReadAdapters {
		item.Level = "yellow"
		item.Text = "没有读到物理网卡的 DNS。"
		return item
	}
	up := upPhysical(facts.Adapters)
	if len(up) == 0 {
		item.Level = "yellow"
		item.Text = "没有正在使用的物理网卡。"
		return item
	}
	byName := map[string]securityDNS{}
	for _, row := range facts.DNS {
		byName[strings.ToLower(row.Name)] = row
	}
	var shown, loopback, unknown []string
	dhcpOnly := true
	sawServer := false
	for _, adapter := range up {
		row, ok := byName[strings.ToLower(adapter.Name)]
		if !ok || len(cleanServers(row.Servers)) == 0 {
			if ok && row.DHCP {
				continue
			}
			dhcpOnly = false
			continue
		}
		dhcpOnly = false
		for _, server := range cleanServers(row.Servers) {
			sawServer = true
			ip := net.ParseIP(server)
			switch {
			case ip == nil:
				unknown = append(unknown, server)
			case ip.IsLoopback() || ip.IsUnspecified():
				loopback = append(loopback, canonicalIP(server))
			case isRouterDNS(ip):
				shown = append(shown, canonicalIP(server))
			case isKnownPublic(server):
				shown = append(shown, canonicalIP(server))
			default:
				unknown = append(unknown, canonicalIP(server))
			}
		}
	}
	switch {
	case len(loopback) > 0:
		item.Level = "red"
		item.Text = "物理网卡的 DNS 指向本机：" + joinLimited(unique(loopback), 3) + "。"
	case len(unknown) > 0:
		item.Level = "yellow"
		item.Text = "物理网卡有不认识的 DNS：" + joinLimited(unique(unknown), 3) + "。"
	case sawServer:
		item.Level = "green"
		item.Text = "物理网卡用的是路由器或常见公共 DNS：" + joinLimited(unique(shown), 3) + "。"
	case dhcpOnly:
		item.Level = "green"
		item.Text = "物理网卡的 DNS 由路由器自动分配。"
	default:
		item.Level = "yellow"
		item.Text = "没有读到物理网卡的 DNS。"
	}
	return item
}

func cleanServers(servers []string) []string {
	out := make([]string, 0, len(servers))
	for _, server := range servers {
		server = strings.TrimSpace(server)
		if server != "" {
			out = append(out, server)
		}
	}
	return out
}

func isKnownPublic(raw string) bool {
	ip := canonicalIP(raw)
	if ip == "" {
		return false
	}
	_, ok := knownPublicDNS[ip]
	return ok
}

func upPhysical(adapters []securityAdapter) []securityAdapter {
	out := make([]securityAdapter, 0, len(adapters))
	for _, adapter := range adapters {
		if adapter.Virtual || !adapter.Up {
			continue
		}
		if !adapter.Hardware && adapter.Virtual {
			continue
		}
		out = append(out, adapter)
	}
	return out
}

func evaluateProxy(facts securityFacts) securityItem {
	item := securityItem{ID: "proxy", Name: "系统代理"}
	if strings.TrimSpace(facts.AutoConfig) != "" {
		item.Level = "red"
		item.Text = "系统设置了自动配置脚本。"
		return item
	}
	if !facts.ProxyEnable {
		item.Level = "green"
		item.Text = "系统代理关着，也没有自动配置脚本。"
		return item
	}
	server := strings.TrimSpace(facts.ProxyServer)
	if server == "" {
		item.Level = "red"
		item.Text = "系统代理开着，但没有填写服务器。"
		return item
	}
	if facts.OwnPort <= 0 {
		item.Level = "yellow"
		item.Text = "系统代理开着，但读不到本软件的混合端口。"
		return item
	}
	if proxyPointsAt(server, facts.OwnPort) {
		item.Level = "green"
		item.Text = "系统代理指向本软件的混合端口 " + strconv.Itoa(facts.OwnPort) + "。"
		return item
	}
	item.Level = "red"
	item.Text = "系统代理开着，指向 " + server + "，不是本软件的端口。"
	return item
}

func proxyPointsAt(raw string, port int) bool {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ';' || r == ' ' })
	matched := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, rest, ok := strings.Cut(part, "="); ok {
			part = rest
		}
		host, portText, ok := splitHostPortLoose(part)
		if !ok {
			return false
		}
		got, err := strconv.Atoi(portText)
		if err != nil || got != port || !isLoopbackHost(host) {
			return false
		}
		matched++
	}
	return matched > 0
}

func splitHostPortLoose(raw string) (string, string, bool) {
	if host, port, err := net.SplitHostPort(raw); err == nil {
		return host, port, true
	}
	host, port, ok := strings.Cut(raw, ":")
	if !ok || host == "" || port == "" {
		return "", "", false
	}
	return host, port, true
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

var commonHosts = []string{
	"google.com", "googleapis.com", "gstatic.com", "youtube.com", "youtu.be",
	"facebook.com", "instagram.com", "twitter.com", "x.com",
	"github.com", "microsoft.com", "windows.com", "live.com", "office.com",
	"apple.com", "icloud.com", "baidu.com", "qq.com", "weixin.qq.com", "wechat.com",
	"douyin.com", "tiktok.com", "telegram.org", "wikipedia.org",
	"cloudflare.com", "amazon.com", "netflix.com", "bing.com",
	"taobao.com", "alipay.com", "jd.com",
}

func evaluateHosts(facts securityFacts) securityItem {
	item := securityItem{ID: "hosts", Name: "hosts"}
	if facts.HostsMissing {
		item.Level = "yellow"
		item.Text = "没有找到 hosts 文件，一般不影响上网。"
		return item
	}
	if facts.HostsUnreadable {
		item.Level = "yellow"
		item.Text = "读不了 hosts 文件，没有改系统设置。"
		return item
	}
	hits := hijackedHosts(facts.Hosts)
	if len(hits) == 0 {
		item.Level = "green"
		item.Text = "hosts 没有改常见网站。"
		return item
	}
	item.Level = "red"
	item.Text = "hosts 改写了这些常见网站：" + joinLimited(hits, 4) + "。"
	return item
}

func hijackedHosts(content string) []string {
	var hits []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || net.ParseIP(fields[0]) == nil {
			continue
		}
		for _, host := range fields[1:] {
			if strings.HasPrefix(host, "#") {
				break
			}
			if isCommonHost(host) {
				hits = append(hits, strings.ToLower(strings.TrimSuffix(host, ".")))
			}
		}
	}
	return unique(hits)
}

func isCommonHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	for _, domain := range commonHosts {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func evaluateAdapters(facts securityFacts) securityItem {
	item := securityItem{ID: "adapters", Name: "虚拟网卡"}
	if !facts.ReadAdapters {
		item.Level = "yellow"
		item.Text = "没有读到网卡列表。"
		return item
	}
	var names []string
	for _, adapter := range facts.Adapters {
		if ours(adapter, facts.TunNames) || microsoftBuiltin(adapter) {
			continue
		}
		if adapter.Virtual || looksLikeExtraTunnel(adapter) {
			label := strings.TrimSpace(adapter.Name)
			if label == "" {
				label = strings.TrimSpace(adapter.Description)
			}
			if label != "" {
				names = append(names, label)
			}
		}
	}
	names = unique(names)
	if len(names) == 0 {
		item.Level = "green"
		item.Text = "没有多余的虚拟网卡。"
		return item
	}
	item.Level = "yellow"
	item.Text = "还有这些虚拟网卡：" + joinLimited(names, 4) + "。"
	return item
}

func ours(adapter securityAdapter, tunNames []string) bool {
	for _, name := range tunNames {
		if name != "" && strings.EqualFold(strings.TrimSpace(adapter.Name), strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func microsoftBuiltin(adapter securityAdapter) bool {
	text := strings.ToLower(adapter.Name + " " + adapter.Description)
	for _, key := range []string{
		"microsoft", "wan miniport", "teredo", "wi-fi direct", "wifi direct",
		"hyper-v", "vethernet", "isatap", "6to4", "ip-https", "bluetooth",
	} {
		if strings.Contains(text, key) {
			return true
		}
	}
	return false
}

func looksLikeExtraTunnel(adapter securityAdapter) bool {
	text := strings.ToLower(adapter.Name + " " + adapter.Description)
	for _, key := range []string{"tap-windows", "tap adapter", "wintun", "wireguard", "openvpn", "tun2socks", "clash", "sing-box", "singbox"} {
		if strings.Contains(text, key) {
			return true
		}
	}
	return false
}

var pingMSPattern = regexp.MustCompile(`(?i)=\s*([0-9]+(?:\.[0-9]+)?)\s*ms`)
var pingLossPattern = regexp.MustCompile(`\(([0-9]+(?:\.[0-9]+)?)%`)

func parsePing(output string) (avg float64, loss float64, ok bool) {
	loss = -1
	if match := pingLossPattern.FindStringSubmatch(output); len(match) == 2 {
		parsed, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			loss = parsed
		}
	}
	matches := pingMSPattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		if loss >= 0 {
			return 0, loss, true
		}
		return 0, 100, false
	}
	parsed, err := strconv.ParseFloat(matches[len(matches)-1][1], 64)
	if err != nil {
		return 0, 100, false
	}
	if loss < 0 {
		loss = 0
	}
	return parsed, loss, true
}

func evaluateGateway(facts securityFacts) securityItem {
	item := securityItem{ID: "gateway", Name: "网关"}
	gateway := canonicalIP(facts.Gateway)
	parsed := net.ParseIP(gateway)
	if parsed == nil || parsed.IsUnspecified() {
		item.Level = "yellow"
		item.Text = "没有读到局域网网关。"
		return item
	}
	if parsed.To4() == nil {
		item.Level = "yellow"
		item.Text = "网关是 IPv6，没有测延迟。"
		return item
	}
	if facts.PingFailed && strings.TrimSpace(facts.PingOutput) == "" {
		item.Level = "red"
		item.Text = "网关 " + gateway + " 没有回应。"
		return item
	}
	avg, loss, ok := parsePing(facts.PingOutput)
	if !ok || loss >= 100 {
		item.Level = "red"
		item.Text = "网关 " + gateway + " 没有回应。"
		return item
	}
	avgText := strconv.Itoa(int(math.Round(avg)))
	lost := int(math.Round(loss / 100 * 5))
	switch {
	case loss > 0 && avg > 20:
		item.Level = "yellow"
		item.Text = "网关 " + gateway + " 平均 " + avgText + "ms，5 次丢了 " + strconv.Itoa(lost) + " 次，无线网络偏弱。"
	case avg > 20:
		item.Level = "yellow"
		item.Text = "网关 " + gateway + " 平均 " + avgText + "ms，无线网络偏弱。"
	case loss > 0:
		item.Level = "yellow"
		item.Text = "网关 " + gateway + " 平均 " + avgText + "ms，5 次丢了 " + strconv.Itoa(lost) + " 次。"
	default:
		item.Level = "green"
		item.Text = "网关 " + gateway + " 平均 " + avgText + "ms，5 次都通。"
	}
	return item
}

func unique(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func joinLimited(items []string, limit int) string {
	if len(items) > limit {
		return strings.Join(items[:limit], "、") + " 等"
	}
	return strings.Join(items, "、")
}
