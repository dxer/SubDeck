package generator

import (
	"encoding/json"
	"fmt"
	"strings"

	"subdeck/internal/models"
)

func buildSingboxTransport(n *models.Node) map[string]interface{} {
	if n.WSOptions != nil {
		tr := map[string]interface{}{
			"type": "ws",
			"path": n.WSOptions.Path,
		}
		if len(n.WSOptions.Headers) > 0 {
			tr["headers"] = n.WSOptions.Headers
		}
		return tr
	}
	if n.GRPCOptions != nil {
		return map[string]interface{}{
			"type":         "grpc",
			"service_name": n.GRPCOptions.ServiceName,
		}
	}
	return nil
}

func buildSingboxTLS(n *models.Node, forceEnabled bool) map[string]interface{} {
	enabled := forceEnabled || n.TLS || n.Reality != nil
	if !enabled {
		return nil
	}
	tlsMap := map[string]interface{}{
		"enabled": true,
	}
	if n.SNI != "" {
		tlsMap["server_name"] = n.SNI
	}
	if n.Insecure {
		tlsMap["insecure"] = true
	}
	if len(n.ALPN) > 0 {
		tlsMap["alpn"] = n.ALPN
	}
	if n.Fingerprint != "" {
		tlsMap["utls"] = map[string]interface{}{
			"enabled":     true,
			"fingerprint": n.Fingerprint,
		}
	}
	if n.Reality != nil {
		tlsMap["reality"] = map[string]interface{}{
			"enabled":    true,
			"public_key": n.Reality.PublicKey,
			"short_id":   n.Reality.ShortID,
		}
	}
	return tlsMap
}

// toSingboxPortRange converts an internal mihomo-style port token
// ("443" or "50000-51000") into sing-box's required "start:end" form.
// sing-box rejects bare ports and dash ranges ("bad port range").
func toSingboxPortRange(token string) string {
	if i := strings.IndexByte(token, ':'); i >= 0 {
		return token // already sing-box style
	}
	if i := strings.IndexByte(token, '-'); i >= 0 {
		return token[:i] + ":" + token[i+1:]
	}
	return token + ":" + token
}

// NodeToSingboxOutbound converts internal Node to Sing-box outbound object
func NodeToSingboxOutbound(n *models.Node) map[string]interface{} {
	if n == nil {
		return nil
	}

	ob := make(map[string]interface{})
	ob["tag"] = n.Name
	ob["server"] = n.Server
	ob["server_port"] = n.Port

	switch n.Type {
	case models.TypeShadowsocks:
		ob["type"] = "shadowsocks"
		ob["method"] = n.Cipher
		ob["password"] = n.Password

	case models.TypeVMess:
		ob["type"] = "vmess"
		ob["uuid"] = n.UUID
		ob["alter_id"] = n.AlterId
		ob["security"] = n.Cipher
		if tlsMap := buildSingboxTLS(n, false); tlsMap != nil {
			ob["tls"] = tlsMap
		}
		if tr := buildSingboxTransport(n); tr != nil {
			ob["transport"] = tr
		}

	case models.TypeVLESS:
		ob["type"] = "vless"
		ob["uuid"] = n.UUID
		if n.Flow != "" {
			ob["flow"] = n.Flow
		}
		if tlsMap := buildSingboxTLS(n, false); tlsMap != nil {
			ob["tls"] = tlsMap
		}
		if tr := buildSingboxTransport(n); tr != nil {
			ob["transport"] = tr
		}

	case models.TypeTrojan:
		ob["type"] = "trojan"
		ob["password"] = n.Password
		ob["tls"] = buildSingboxTLS(n, true)
		if tr := buildSingboxTransport(n); tr != nil {
			ob["transport"] = tr
		}

	case models.TypeHysteria2:
		ob["type"] = "hysteria2"
		ob["password"] = n.Password
		ob["tls"] = buildSingboxTLS(n, true)
		if n.Hy2Ports != "" {
			var ports []string
			for _, p := range strings.Split(n.Hy2Ports, ",") {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					ports = append(ports, toSingboxPortRange(trimmed))
				}
			}
			if len(ports) > 0 {
				ob["server_ports"] = ports
			}
		}
		if n.Hy2UpMbps > 0 || n.Hy2DownMbps > 0 {
			ob["up_mbps"] = n.Hy2UpMbps
			ob["down_mbps"] = n.Hy2DownMbps
		}
		if n.Hy2ObfsEnabled() {
			ob["obfs"] = map[string]interface{}{
				"type":     n.Hy2Obfs,
				"password": n.Hy2ObfsPassword,
			}
		}

	case models.TypeAnyTLS:
		ob["type"] = "anytls"
		ob["password"] = n.Password
		ob["tls"] = buildSingboxTLS(n, true)

	case models.TypeTUIC:
		ob["type"] = "tuic"
		ob["uuid"] = n.UUID
		ob["password"] = n.Password
		ob["tls"] = buildSingboxTLS(n, true)
		if n.TuicCongestion != "" {
			ob["congestion_control"] = n.TuicCongestion
		}

	case models.TypeWireGuard:
		ob["type"] = "wireguard"
		ob["private_key"] = n.WGPrivateKey
		ob["peer_public_key"] = n.WGPublicKey
		if n.WGLocalAddr != "" {
			ob["local_address"] = []string{n.WGLocalAddr}
		}
		if n.WGMTU > 0 {
			ob["mtu"] = n.WGMTU
		}
		if len(n.WGReserved) > 0 {
			ob["reserved"] = n.WGReserved
		}

	default:
		return nil
	}

	return ob
}

// GenerateSingboxOutboundsJSON generates an outbounds array
func GenerateSingboxOutboundsJSON(nodes []*models.Node) ([]byte, error) {
	var outbounds []map[string]interface{}
	for _, n := range nodes {
		ob := NodeToSingboxOutbound(n)
		if ob != nil {
			outbounds = append(outbounds, ob)
		}
	}
	return json.MarshalIndent(outbounds, "", "  ")
}

// GenerateSingboxFullJSON generates a complete sing-box 1.12+ compatible configuration
func GenerateSingboxFullJSON(nodes []*models.Node, profileName string) ([]byte, error) {
	return GenerateSingboxFullJSONWithConfig(nodes, profileName, ExportOptions{})
}

// GenerateSingboxFullJSONWithOptions generates a modern sing-box 1.12+ configuration
// with rule_set routing, hijack-dns action, and optional regional urltest groups.
func GenerateSingboxFullJSONWithOptions(nodes []*models.Node, profileName string, regionGroups bool) ([]byte, error) {
	return GenerateSingboxFullJSONWithConfig(nodes, profileName, ExportOptions{RegionGroups: regionGroups})
}

func buildSingboxServiceSelector(tag string, regionNames, nodeTags []string) map[string]interface{} {
	choices := make([]string, 0, 2+len(regionNames)+len(nodeTags))
	choices = append(choices, "proxy", "auto")
	choices = append(choices, regionNames...)
	choices = append(choices, nodeTags...)
	return map[string]interface{}{
		"type":      "selector",
		"tag":       tag,
		"outbounds": choices,
	}
}

// GenerateSingboxFullJSONWithConfig generates a modern sing-box 1.12+ configuration
// supporting rule presets, optional ruleset mirror prefix, and regional urltest groups.
func GenerateSingboxFullJSONWithConfig(nodes []*models.Node, profileName string, opts ExportOptions) ([]byte, error) {
	var nodeTags []string
	var proxyOutbounds []map[string]interface{}

	for _, n := range nodes {
		ob := NodeToSingboxOutbound(n)
		if ob != nil {
			proxyOutbounds = append(proxyOutbounds, ob)
			nodeTags = append(nodeTags, n.Name)
		}
	}

	if len(nodeTags) == 0 {
		nodeTags = append(nodeTags, "direct")
	}

	selectorList := []string{"auto"}
	var regionOutbounds []map[string]interface{}
	var regionNames []string
	if opts.RegionGroups {
		for _, rg := range BuildRegionGroups(nodeTags) {
			selectorList = append(selectorList, rg.Name)
			regionNames = append(regionNames, rg.Name)
			regionOutbounds = append(regionOutbounds, map[string]interface{}{
				"type":      "urltest",
				"tag":       rg.Name,
				"outbounds": rg.Nodes,
				"url":       "http://www.gstatic.com/generate_204",
				"interval":  "300s",
			})
		}
	}
	selectorList = append(selectorList, "direct")
	selectorList = append(selectorList, nodeTags...)

	selectorOutbound := map[string]interface{}{
		"type":      "selector",
		"tag":       "proxy",
		"outbounds": selectorList,
	}

	urltestOutbound := map[string]interface{}{
		"type":      "urltest",
		"tag":       "auto",
		"outbounds": nodeTags,
		"url":       "http://www.gstatic.com/generate_204",
		"interval":  "300s",
	}

	directOutbound := map[string]interface{}{
		"type": "direct",
		"tag":  "direct",
	}

	preset := NormalizeRulePreset(opts.RulePreset)
	allOutbounds := []map[string]interface{}{
		selectorOutbound,
		urltestOutbound,
	}
	if preset == RulePresetAIStreaming {
		allOutbounds = append(allOutbounds,
			buildSingboxServiceSelector("ai", regionNames, nodeTags),
			buildSingboxServiceSelector("streaming", regionNames, nodeTags),
			buildSingboxServiceSelector("telegram", regionNames, nodeTags),
		)
	}
	allOutbounds = append(allOutbounds, regionOutbounds...)
	allOutbounds = append(allOutbounds, directOutbound)
	allOutbounds = append(allOutbounds, proxyOutbounds...)

	routeRules := []map[string]interface{}{
		{
			"action": "sniff",
		},
		{
			"protocol": "dns",
			"action":   "hijack-dns",
		},
		{
			"ip_is_private": true,
			"outbound":      "direct",
		},
	}
	if preset == RulePresetAIStreaming {
		routeRules = append(routeRules,
			map[string]interface{}{
				"domain_suffix": aiDomainSuffixes,
				"outbound":      "ai",
			},
			map[string]interface{}{
				"domain_suffix": streamingDomainSuffixes,
				"outbound":      "streaming",
			},
			map[string]interface{}{
				"domain_suffix": telegramDomainSuffixes,
				"outbound":      "telegram",
			},
			map[string]interface{}{
				"ip_cidr":  telegramIPCIDRs,
				"outbound": "telegram",
			},
		)
	}

	var dnsRules []map[string]interface{}
	var ruleSets []map[string]interface{}
	if preset != RulePresetGlobal {
		routeRules = append(routeRules, map[string]interface{}{
			"rule_set": []string{"geosite-cn", "geoip-cn"},
			"outbound": "direct",
		})
		dnsRules = append(dnsRules, map[string]interface{}{
			"rule_set": "geosite-cn",
			"server":   "dns_direct",
		})
		ruleSets = []map[string]interface{}{
			{
				"tag":             "geosite-cn",
				"type":            "remote",
				"format":          "binary",
				"url":             ApplyRulesetMirror("https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs", opts.RulesetMirror),
				"download_detour": "proxy",
			},
			{
				"tag":             "geoip-cn",
				"type":            "remote",
				"format":          "binary",
				"url":             ApplyRulesetMirror("https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs", opts.RulesetMirror),
				"download_detour": "proxy",
			},
		}
	}

	routeCfg := map[string]interface{}{
		"rules":                   routeRules,
		"final":                   "proxy",
		"auto_detect_interface":   true,
		"default_domain_resolver": "dns_direct",
	}
	if len(ruleSets) > 0 {
		routeCfg["rule_set"] = ruleSets
	}

	// Dynamic payload only — log, dns servers, inbounds and experimental
	// (cache_file, clash_api) come from the built-in base template below.
	config := map[string]interface{}{
		"outbounds": allOutbounds,
		"route":     routeCfg,
	}
	if len(dnsRules) > 0 {
		config["dns"] = map[string]interface{}{"rules": dnsRules}
	}

	baseTmpl, err := DefaultSingboxTemplate()
	if err != nil {
		return nil, err
	}
	if err := ApplySingboxTemplate(config, baseTmpl); err != nil {
		return nil, err
	}
	if opts.SingboxTemplate != nil {
		if err := ApplySingboxTemplate(config, opts.SingboxTemplate); err != nil {
			return nil, err
		}
	}

	_ = fmt.Sprintf("Profile: %s", profileName)
	return json.MarshalIndent(config, "", "  ")
}
