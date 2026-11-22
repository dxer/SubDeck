package generator

import (
	"fmt"

	"subdeck/internal/models"

	"gopkg.in/yaml.v3"
)

func applyClashTransport(m map[string]interface{}, n *models.Node) {
	if n.WSOptions != nil {
		m["network"] = "ws"
		wsOpts := map[string]interface{}{
			"path": n.WSOptions.Path,
		}
		if len(n.WSOptions.Headers) > 0 {
			wsOpts["headers"] = n.WSOptions.Headers
		}
		m["ws-opts"] = wsOpts
	} else if n.GRPCOptions != nil {
		m["network"] = "grpc"
		m["grpc-opts"] = map[string]interface{}{
			"grpc-service-name": n.GRPCOptions.ServiceName,
		}
	}
}

// clashUDPEnabled resolves the Clash udp relay flag: the profile-level override
// wins when present, otherwise the legacy default (enabled) applies.
func clashUDPEnabled(n *models.Node) bool {
	if n != nil && n.UDP != nil {
		return *n.UDP
	}
	return true
}

// NodeToClashProxy converts our internal Node into Clash Meta / Mihomo compatible map
func NodeToClashProxy(n *models.Node) map[string]interface{} {
	if n == nil {
		return nil
	}

	m := make(map[string]interface{})
	m["name"] = n.Name
	m["server"] = n.Server
	m["port"] = n.Port

	switch n.Type {
	case models.TypeShadowsocks:
		m["type"] = "ss"
		m["cipher"] = n.Cipher
		m["password"] = n.Password
		m["udp"] = clashUDPEnabled(n)

	case models.TypeVMess:
		m["type"] = "vmess"
		m["uuid"] = n.UUID
		m["alterId"] = n.AlterId
		m["cipher"] = n.Cipher
		if n.Cipher == "" {
			m["cipher"] = "auto"
		}
		m["udp"] = clashUDPEnabled(n)
		m["network"] = n.Network
		if n.TLS {
			m["tls"] = true
			if n.SNI != "" {
				m["servername"] = n.SNI
			}
			if n.Insecure {
				m["skip-cert-verify"] = true
			}
			if len(n.ALPN) > 0 {
				m["alpn"] = n.ALPN
			}
		}
		if n.Fingerprint != "" {
			m["client-fingerprint"] = n.Fingerprint
		}
		applyClashTransport(m, n)

	case models.TypeVLESS:
		m["type"] = "vless"
		m["uuid"] = n.UUID
		m["udp"] = clashUDPEnabled(n)
		m["network"] = n.Network
		if n.Flow != "" {
			m["flow"] = n.Flow
		}
		if n.TLS {
			m["tls"] = true
			if n.SNI != "" {
				m["servername"] = n.SNI
			}
			if n.Insecure {
				m["skip-cert-verify"] = true
			}
			if len(n.ALPN) > 0 {
				m["alpn"] = n.ALPN
			}
		}
		if n.Fingerprint != "" {
			m["client-fingerprint"] = n.Fingerprint
		}
		if n.Reality != nil {
			m["tls"] = true
			m["reality-opts"] = map[string]interface{}{
				"public-key": n.Reality.PublicKey,
				"short-id":   n.Reality.ShortID,
			}
		}
		applyClashTransport(m, n)

	case models.TypeTrojan:
		m["type"] = "trojan"
		m["password"] = n.Password
		m["udp"] = clashUDPEnabled(n)
		if n.SNI != "" {
			m["sni"] = n.SNI
		}
		if n.Fingerprint != "" {
			m["client-fingerprint"] = n.Fingerprint
		}
		if n.Insecure {
			m["skip-cert-verify"] = true
		}
		if len(n.ALPN) > 0 {
			m["alpn"] = n.ALPN
		}
		applyClashTransport(m, n)

	case models.TypeHysteria2:
		m["type"] = "hysteria2"
		m["password"] = n.Password
		if n.SNI != "" {
			m["sni"] = n.SNI
		}
		if n.Insecure {
			m["skip-cert-verify"] = true
		}
		if len(n.ALPN) > 0 {
			m["alpn"] = n.ALPN
		}
		if n.Hy2Ports != "" {
			m["ports"] = n.Hy2Ports
		}
		if n.Hy2UpMbps > 0 {
			m["up"] = fmt.Sprintf("%d Mbps", n.Hy2UpMbps)
		}
		if n.Hy2DownMbps > 0 {
			m["down"] = fmt.Sprintf("%d Mbps", n.Hy2DownMbps)
		}
		if n.Hy2ObfsEnabled() {
			m["obfs"] = n.Hy2Obfs
			m["obfs-password"] = n.Hy2ObfsPassword
		}

	case models.TypeAnyTLS:
		m["type"] = "anytls"
		m["password"] = n.Password
		m["udp"] = clashUDPEnabled(n)
		if n.SNI != "" {
			m["sni"] = n.SNI
		}
		if n.Fingerprint != "" {
			m["client-fingerprint"] = n.Fingerprint
		}
		if n.Insecure {
			m["skip-cert-verify"] = true
		}
		if len(n.ALPN) > 0 {
			m["alpn"] = n.ALPN
		}

	case models.TypeTUIC:
		m["type"] = "tuic"
		m["uuid"] = n.UUID
		m["password"] = n.Password
		m["udp"] = clashUDPEnabled(n)
		if n.SNI != "" {
			m["sni"] = n.SNI
		}
		if n.Insecure {
			m["skip-cert-verify"] = true
		}
		if len(n.ALPN) > 0 {
			m["alpn"] = n.ALPN
		}
		if n.TuicCongestion != "" {
			m["congestion-controller"] = n.TuicCongestion
		}

	case models.TypeWireGuard:
		m["type"] = "wireguard"
		m["private-key"] = n.WGPrivateKey
		m["public-key"] = n.WGPublicKey
		m["ip"] = n.WGLocalAddr
		m["udp"] = clashUDPEnabled(n)
		if n.WGMTU > 0 {
			m["mtu"] = n.WGMTU
		}
		if len(n.WGReserved) > 0 {
			m["reserved"] = n.WGReserved
		}

	case models.TypeSnell:
		m["type"] = "snell"
		m["psk"] = n.SnellPSK
		m["version"] = n.SnellVersion

	default:
		return nil
	}

	return m
}

// GenerateClashProxiesYAML outputs pure `proxies:` format for Proxy Provider
func GenerateClashProxiesYAML(nodes []*models.Node) ([]byte, error) {
	var proxies []map[string]interface{}
	for _, n := range nodes {
		p := NodeToClashProxy(n)
		if p != nil {
			proxies = append(proxies, p)
		}
	}

	result := map[string]interface{}{
		"proxies": proxies,
	}

	return yaml.Marshal(result)
}

// GenerateClashFullYAML outputs a complete ready-to-run Clash / Mihomo configuration
func GenerateClashFullYAML(nodes []*models.Node, profileName string) ([]byte, error) {
	return GenerateClashFullYAMLWithConfig(nodes, profileName, ExportOptions{})
}

// GenerateClashFullYAMLWithOptions outputs a complete Clash / Mihomo configuration
// with DNS defaults and optional auto-generated regional url-test groups.
func GenerateClashFullYAMLWithOptions(nodes []*models.Node, profileName string, regionGroups bool) ([]byte, error) {
	return GenerateClashFullYAMLWithConfig(nodes, profileName, ExportOptions{RegionGroups: regionGroups})
}

func buildClashServiceGroup(groupName string, regionGroupNames, proxyNames []string) map[string]interface{} {
	choices := make([]string, 0, 3+len(regionGroupNames)+len(proxyNames))
	choices = append(choices, "🚀 节点选择", "⚡ 自动选择")
	choices = append(choices, regionGroupNames...)
	choices = append(choices, proxyNames...)
	return map[string]interface{}{
		"name":    groupName,
		"type":    "select",
		"proxies": choices,
	}
}

func buildClashRules(preset string) []string {
	var rules []string
	if preset == RulePresetAIStreaming {
		for _, d := range aiDomainSuffixes {
			rules = append(rules, fmt.Sprintf("DOMAIN-SUFFIX,%s,🤖 AI 服务", d))
		}
		for _, d := range streamingDomainSuffixes {
			rules = append(rules, fmt.Sprintf("DOMAIN-SUFFIX,%s,🎬 流媒体", d))
		}
		for _, d := range telegramDomainSuffixes {
			rules = append(rules, fmt.Sprintf("DOMAIN-SUFFIX,%s,💬 电报消息", d))
		}
		for _, cidr := range telegramIPCIDRs {
			rules = append(rules, fmt.Sprintf("IP-CIDR,%s,💬 电报消息,no-resolve", cidr))
		}
	}
	rules = append(rules, "GEOIP,LAN,DIRECT,no-resolve")
	if preset != RulePresetGlobal {
		// Domain-level CN split first (mihomo downloads geosite data on demand),
		// then ad blocking, then the IP-level fallback.
		rules = append(rules,
			"GEOSITE,category-ads-all,REJECT",
			"GEOSITE,cn,DIRECT",
			"GEOIP,CN,DIRECT",
		)
	}
	rules = append(rules, "MATCH,🚀 节点选择")
	return rules
}

// GenerateClashFullYAMLWithConfig outputs a complete Clash / Mihomo configuration:
// the dynamic parts (proxies, proxy-groups, rules per preset and region groups)
// are injected into the built-in base template, then an optional user template
// overlays on top.
func GenerateClashFullYAMLWithConfig(nodes []*models.Node, profileName string, opts ExportOptions) ([]byte, error) {
	var proxies []map[string]interface{}
	var proxyNames []string

	for _, n := range nodes {
		p := NodeToClashProxy(n)
		if p != nil {
			proxies = append(proxies, p)
			proxyNames = append(proxyNames, n.Name)
		}
	}

	if len(proxyNames) == 0 {
		proxyNames = append(proxyNames, "DIRECT")
	}

	proxyGroupAuto := map[string]interface{}{
		"name":     "⚡ 自动选择",
		"type":     "url-test",
		"url":      "http://www.gstatic.com/generate_204",
		"interval": 300,
		"proxies":  proxyNames,
	}

	proxyGroupFallback := map[string]interface{}{
		"name":     "🛡️ 故障转移",
		"type":     "fallback",
		"url":      "http://www.gstatic.com/generate_204",
		"interval": 300,
		"proxies":  proxyNames,
	}

	selectList := []string{"⚡ 自动选择", "🛡️ 故障转移"}
	var extraGroups []map[string]interface{}
	var regionGroupNames []string
	if opts.RegionGroups {
		for _, rg := range BuildRegionGroups(proxyNames) {
			selectList = append(selectList, rg.Name)
			regionGroupNames = append(regionGroupNames, rg.Name)
			extraGroups = append(extraGroups, map[string]interface{}{
				"name":     rg.Name,
				"type":     "url-test",
				"url":      "http://www.gstatic.com/generate_204",
				"interval": 300,
				"proxies":  rg.Nodes,
			})
		}
	}
	selectList = append(selectList, "DIRECT")
	selectList = append(selectList, proxyNames...)

	proxyGroupProxy := map[string]interface{}{
		"name":    "🚀 节点选择",
		"type":    "select",
		"proxies": selectList,
	}

	preset := NormalizeRulePreset(opts.RulePreset)
	groups := []map[string]interface{}{
		proxyGroupProxy,
	}
	if preset == RulePresetAIStreaming {
		groups = append(groups,
			buildClashServiceGroup("🤖 AI 服务", regionGroupNames, proxyNames),
			buildClashServiceGroup("🎬 流媒体", regionGroupNames, proxyNames),
			buildClashServiceGroup("💬 电报消息", regionGroupNames, proxyNames),
		)
	}
	groups = append(groups, proxyGroupAuto, proxyGroupFallback)
	groups = append(groups, extraGroups...)

	// Dynamic payload only — the static foundation (ports, dns, sniffer, tun,
	// profile) comes from the built-in base template below.
	config := map[string]interface{}{
		"proxies":      proxies,
		"proxy-groups": groups,
		"rules":        buildClashRules(preset),
	}

	baseTmpl, err := DefaultClashTemplate()
	if err != nil {
		return nil, err
	}
	if err := ApplyClashTemplate(config, baseTmpl); err != nil {
		return nil, err
	}
	if opts.ClashTemplate != nil {
		if err := ApplyClashTemplate(config, opts.ClashTemplate); err != nil {
			return nil, err
		}
	}

	out, err := yaml.Marshal(config)
	if err != nil {
		return nil, err
	}

	// Add header comment
	header := fmt.Sprintf("# Generated by SubDeck Profile: %s\n# Nodes: %d\n\n", profileName, len(proxies))
	return append([]byte(header), out...), nil
}
