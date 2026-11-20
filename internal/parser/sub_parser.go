package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"subdeck/internal/models"

	"gopkg.in/yaml.v3"
)

// ParseSubscriptionContent parses raw response text into a list of Node pointers
func ParseSubscriptionContent(content []byte) ([]*models.Node, error) {
	str := strings.TrimSpace(string(content))
	if str == "" {
		return nil, nil
	}

	// 1. Try Clash YAML first if it contains "proxies:"
	if strings.Contains(str, "proxies:") {
		nodes, err := parseClashYAML(content)
		if err == nil && len(nodes) > 0 {
			return nodes, nil
		}
	}

	// 2. Try Sing-box JSON if it starts with "{" and contains "outbounds"
	if strings.HasPrefix(str, "{") && strings.Contains(str, "outbounds") {
		nodes, err := parseSingboxJSON(content)
		if err == nil && len(nodes) > 0 {
			return nodes, nil
		}
	}

	// 3. Try Base64 decode (standard subscription format)
	decoded, err := SafeBase64Decode(str)
	if err == nil && len(decoded) > 0 {
		decodedStr := string(decoded)
		lines := strings.Split(decodedStr, "\n")
		var nodes []*models.Node
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			node, err := ParseURI(line)
			if err == nil && node != nil {
				nodes = append(nodes, node)
			}
		}
		if len(nodes) > 0 {
			return nodes, nil
		}
	}

	// 4. Try plain URI list (line by line)
	lines := strings.Split(str, "\n")
	var nodes []*models.Node
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		node, err := ParseURI(line)
		if err == nil && node != nil {
			nodes = append(nodes, node)
		}
	}

	if len(nodes) > 0 {
		return nodes, nil
	}

	return nil, fmt.Errorf("unable to parse subscription: unrecognized format")
}

// Parse Clash YAML proxies
type clashConfig struct {
	Proxies []map[string]interface{} `yaml:"proxies"`
}

func parseClashYAML(content []byte) ([]*models.Node, error) {
	var cfg clashConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, err
	}

	var nodes []*models.Node
	for _, p := range cfg.Proxies {
		node := clashMapToNode(p)
		if node != nil {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func clashMapToNode(m map[string]interface{}) *models.Node {
	name, _ := m["name"].(string)
	pType, _ := m["type"].(string)
	server, _ := m["server"].(string)
	port := toInt(m["port"])

	if server == "" || port == 0 {
		return nil
	}

	node := &models.Node{
		Name:   name,
		Server: server,
		Port:   port,
	}

	switch strings.ToLower(pType) {
	case "ss":
		node.Type = models.TypeShadowsocks
		node.Cipher, _ = m["cipher"].(string)
		node.Password, _ = m["password"].(string)
	case "vmess":
		node.Type = models.TypeVMess
		node.UUID, _ = m["uuid"].(string)
		node.AlterId = toInt(m["alterId"])
		node.Cipher, _ = m["cipher"].(string)
		if node.Cipher == "" {
			node.Cipher = "auto"
		}
		node.Network, _ = m["network"].(string)
		tls, _ := m["tls"].(bool)
		node.TLS = tls
		node.SNI, _ = m["servername"].(string)
		if wsOpts, ok := m["ws-opts"].(map[string]interface{}); ok {
			path, _ := wsOpts["path"].(string)
			node.WSOptions = &models.WSOptions{Path: path}
		}
	case "vless":
		node.Type = models.TypeVLESS
		node.UUID, _ = m["uuid"].(string)
		node.Network, _ = m["network"].(string)
		node.Flow, _ = m["flow"].(string)
		tls, _ := m["tls"].(bool)
		node.TLS = tls
		node.SNI, _ = m["servername"].(string)
		if realityOpts, ok := m["reality-opts"].(map[string]interface{}); ok {
			pbk, _ := realityOpts["public-key"].(string)
			sid, _ := realityOpts["short-id"].(string)
			node.Reality = &models.Reality{
				PublicKey: pbk,
				ShortID:   sid,
			}
		}
	case "trojan":
		node.Type = models.TypeTrojan
		node.Password, _ = m["password"].(string)
		node.SNI, _ = m["sni"].(string)
		node.Network, _ = m["network"].(string)
		node.TLS = true
	case "hysteria2":
		node.Type = models.TypeHysteria2
		node.Password, _ = m["password"].(string)
		node.SNI, _ = m["sni"].(string)
		node.TLS = true
		node.Hy2Obfs, _ = m["obfs"].(string)
		node.Hy2ObfsPassword, _ = m["obfs-password"].(string)
		if ports, ok := m["ports"].(string); ok {
			node.Hy2Ports = ports
		}
	case "anytls":
		node.Type = models.TypeAnyTLS
		node.Password, _ = m["password"].(string)
		node.SNI, _ = m["sni"].(string)
		node.TLS = true
		node.Fingerprint, _ = m["client-fingerprint"].(string)
		if insecure, ok := m["skip-cert-verify"].(bool); ok {
			node.Insecure = insecure
		}
	case "tuic":
		node.Type = models.TypeTUIC
		node.UUID, _ = m["uuid"].(string)
		node.Password, _ = m["password"].(string)
		node.SNI, _ = m["sni"].(string)
		node.TLS = true
		node.TuicCongestion, _ = m["congestion-controller"].(string)
	case "wireguard":
		node.Type = models.TypeWireGuard
		node.WGPrivateKey, _ = m["private-key"].(string)
		node.WGPublicKey, _ = m["public-key"].(string)
		node.WGLocalAddr, _ = m["ip"].(string)
		node.WGMTU = toInt(m["mtu"])
	case "snell":
		node.Type = models.TypeSnell
		node.SnellPSK, _ = m["psk"].(string)
		node.SnellVersion = toInt(m["version"])
	default:
		return nil
	}

	return node
}

// Parse Sing-box JSON outbounds
type singboxConfig struct {
	Outbounds []map[string]interface{} `json:"outbounds"`
}

func parseSingboxJSON(content []byte) ([]*models.Node, error) {
	var cfg singboxConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, err
	}

	var nodes []*models.Node
	for _, ob := range cfg.Outbounds {
		tag, _ := ob["tag"].(string)
		obType, _ := ob["type"].(string)
		server, _ := ob["server"].(string)
		port := toInt(ob["server_port"])

		if server == "" || port == 0 {
			continue
		}

		node := &models.Node{
			Name:   tag,
			Server: server,
			Port:   port,
		}

		switch obType {
		case "shadowsocks":
			node.Type = models.TypeShadowsocks
			node.Cipher, _ = ob["method"].(string)
			node.Password, _ = ob["password"].(string)
		case "vmess":
			node.Type = models.TypeVMess
			node.UUID, _ = ob["uuid"].(string)
			node.AlterId = toInt(ob["alter_id"])
			node.Cipher, _ = ob["security"].(string)
		case "vless":
			node.Type = models.TypeVLESS
			node.UUID, _ = ob["uuid"].(string)
			node.Flow, _ = ob["flow"].(string)
			if tlsMap, ok := ob["tls"].(map[string]interface{}); ok {
				enabled, _ := tlsMap["enabled"].(bool)
				node.TLS = enabled
				node.SNI, _ = tlsMap["server_name"].(string)
				if realityMap, ok := tlsMap["reality"].(map[string]interface{}); ok {
					pbk, _ := realityMap["public_key"].(string)
					sid, _ := realityMap["short_id"].(string)
					node.Reality = &models.Reality{
						PublicKey: pbk,
						ShortID:   sid,
					}
				}
			}
		case "trojan":
			node.Type = models.TypeTrojan
			node.Password, _ = ob["password"].(string)
			node.TLS = true
		case "hysteria2":
			node.Type = models.TypeHysteria2
			node.Password, _ = ob["password"].(string)
			node.TLS = true
		case "anytls":
			node.Type = models.TypeAnyTLS
			node.Password, _ = ob["password"].(string)
			node.TLS = true
			if tlsMap, ok := ob["tls"].(map[string]interface{}); ok {
				node.SNI, _ = tlsMap["server_name"].(string)
				if utlsMap, ok := tlsMap["utls"].(map[string]interface{}); ok {
					node.Fingerprint, _ = utlsMap["fingerprint"].(string)
				}
			}
		case "tuic":
			node.Type = models.TypeTUIC
			node.UUID, _ = ob["uuid"].(string)
			node.Password, _ = ob["password"].(string)
			node.TLS = true
		case "wireguard":
			node.Type = models.TypeWireGuard
			node.WGPrivateKey, _ = ob["private_key"].(string)
			node.WGPublicKey, _ = ob["peer_public_key"].(string)
			node.WGMTU = toInt(ob["mtu"])
		default:
			continue
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}
