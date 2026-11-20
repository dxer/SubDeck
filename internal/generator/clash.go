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

	case models.TypeVMess:
		m["type"] = "vmess"
		m["uuid"] = n.UUID
		m["alterId"] = n.AlterId
		m["cipher"] = n.Cipher
		if n.Cipher == "" {
			m["cipher"] = "auto"
		}
		m["network"] = n.Network
		if n.TLS {
			m["tls"] = true
			if n.SNI != "" {
				m["servername"] = n.SNI
			}
			if n.Insecure {
				m["skip-cert-verify"] = true
			}
		}
		if n.Fingerprint != "" {
			m["client-fingerprint"] = n.Fingerprint
		}
		applyClashTransport(m, n)

	case models.TypeVLESS:
		m["type"] = "vless"
		m["uuid"] = n.UUID
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
		if n.SNI != "" {
			m["sni"] = n.SNI
		}
		if n.Fingerprint != "" {
			m["client-fingerprint"] = n.Fingerprint
		}
		if n.Insecure {
			m["skip-cert-verify"] = true
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
		if n.Hy2Ports != "" {
			m["ports"] = n.Hy2Ports
		}
		if n.Hy2UpMbps > 0 {
			m["up"] = fmt.Sprintf("%d Mbps", n.Hy2UpMbps)
		}
		if n.Hy2DownMbps > 0 {
			m["down"] = fmt.Sprintf("%d Mbps", n.Hy2DownMbps)
		}
		if n.Hy2Obfs != "" {
			m["obfs"] = n.Hy2Obfs
			m["obfs-password"] = n.Hy2ObfsPassword
		}

	case models.TypeAnyTLS:
		m["type"] = "anytls"
		m["password"] = n.Password
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
		if n.SNI != "" {
			m["sni"] = n.SNI
		}
		if n.Insecure {
			m["skip-cert-verify"] = true
		}
		if n.TuicCongestion != "" {
			m["congestion-controller"] = n.TuicCongestion
		}

	case models.TypeWireGuard:
		m["type"] = "wireguard"
		m["private-key"] = n.WGPrivateKey
		m["public-key"] = n.WGPublicKey
		m["ip"] = n.WGLocalAddr
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

	selectList := append([]string{"⚡ 自动选择", "🛡️ 故障转移", "DIRECT"}, proxyNames...)
	proxyGroupProxy := map[string]interface{}{
		"name":    "🚀 节点选择",
		"type":    "select",
		"proxies": selectList,
	}

	config := map[string]interface{}{
		"mixed-port":          7890,
		"allow-lan":           true,
		"mode":                "rule",
		"log-level":           "info",
		"external-controller": "127.0.0.1:9090",
		"proxies":             proxies,
		"proxy-groups": []map[string]interface{}{
			proxyGroupProxy,
			proxyGroupAuto,
			proxyGroupFallback,
		},
		"rules": []string{
			"GEOIP,LAN,DIRECT,no-resolve",
			"GEOIP,CN,DIRECT",
			"MATCH,🚀 节点选择",
		},
	}

	out, err := yaml.Marshal(config)
	if err != nil {
		return nil, err
	}

	// Add header comment
	header := fmt.Sprintf("# Generated by SubDeck Profile: %s\n# Nodes: %d\n\n", profileName, len(proxies))
	return append([]byte(header), out...), nil
}
