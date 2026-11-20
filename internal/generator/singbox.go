package generator

import (
	"encoding/json"
	"fmt"
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
		if n.TLS {
			tlsMap := map[string]interface{}{
				"enabled":     true,
				"server_name": n.SNI,
			}
			if n.Insecure {
				tlsMap["insecure"] = true
			}
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
		tlsMap := map[string]interface{}{
			"enabled": n.TLS,
		}
		if n.SNI != "" {
			tlsMap["server_name"] = n.SNI
		}
		if n.Insecure {
			tlsMap["insecure"] = true
		}
		if n.Reality != nil {
			tlsMap["enabled"] = true
			tlsMap["reality"] = map[string]interface{}{
				"enabled":    true,
				"public_key": n.Reality.PublicKey,
				"short_id":   n.Reality.ShortID,
			}
		}
		if n.TLS || n.Reality != nil {
			ob["tls"] = tlsMap
		}
		if tr := buildSingboxTransport(n); tr != nil {
			ob["transport"] = tr
		}

	case models.TypeTrojan:
		ob["type"] = "trojan"
		ob["password"] = n.Password
		tlsMap := map[string]interface{}{
			"enabled": true,
		}
		if n.SNI != "" {
			tlsMap["server_name"] = n.SNI
		}
		if n.Insecure {
			tlsMap["insecure"] = true
		}
		ob["tls"] = tlsMap
		if tr := buildSingboxTransport(n); tr != nil {
			ob["transport"] = tr
		}

	case models.TypeHysteria2:
		ob["type"] = "hysteria2"
		ob["password"] = n.Password
		tlsMap := map[string]interface{}{
			"enabled": true,
		}
		if n.SNI != "" {
			tlsMap["server_name"] = n.SNI
		}
		if n.Insecure {
			tlsMap["insecure"] = true
		}
		ob["tls"] = tlsMap
		if n.Hy2UpMbps > 0 || n.Hy2DownMbps > 0 {
			ob["up_mbps"] = n.Hy2UpMbps
			ob["down_mbps"] = n.Hy2DownMbps
		}
		if n.Hy2Obfs != "" {
			ob["obfs"] = map[string]interface{}{
				"type":     n.Hy2Obfs,
				"password": n.Hy2ObfsPassword,
			}
		}

	case models.TypeAnyTLS:
		ob["type"] = "anytls"
		ob["password"] = n.Password
		tlsMap := map[string]interface{}{
			"enabled": true,
		}
		if n.SNI != "" {
			tlsMap["server_name"] = n.SNI
		}
		if n.Insecure {
			tlsMap["insecure"] = true
		}
		if n.Fingerprint != "" {
			tlsMap["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": n.Fingerprint,
			}
		}
		ob["tls"] = tlsMap

	case models.TypeTUIC:
		ob["type"] = "tuic"
		ob["uuid"] = n.UUID
		ob["password"] = n.Password
		tlsMap := map[string]interface{}{
			"enabled": true,
		}
		if n.SNI != "" {
			tlsMap["server_name"] = n.SNI
		}
		if n.Insecure {
			tlsMap["insecure"] = true
		}
		ob["tls"] = tlsMap
		if n.TuicCongestion != "" {
			ob["congestion_control"] = n.TuicCongestion
		}

	case models.TypeWireGuard:
		ob["type"] = "wireguard"
		ob["private_key"] = n.WGPrivateKey
		ob["peer_public_key"] = n.WGPublicKey
		ob["local_address"] = []string{n.WGLocalAddr}
		if n.WGMTU > 0 {
			ob["mtu"] = n.WGMTU
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

// GenerateSingboxFullJSON generates a complete sing-box configuration
func GenerateSingboxFullJSON(nodes []*models.Node, profileName string) ([]byte, error) {
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

	selectorOutbound := map[string]interface{}{
		"type":      "selector",
		"tag":       "proxy",
		"outbounds": append([]string{"auto", "direct"}, nodeTags...),
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

	blockOutbound := map[string]interface{}{
		"type": "block",
		"tag":  "block",
	}

	dnsOutbound := map[string]interface{}{
		"type": "dns",
		"tag":  "dns-out",
	}

	allOutbounds := []map[string]interface{}{
		selectorOutbound,
		urltestOutbound,
		directOutbound,
		blockOutbound,
		dnsOutbound,
	}
	allOutbounds = append(allOutbounds, proxyOutbounds...)

	config := map[string]interface{}{
		"log": map[string]interface{}{
			"level":  "info",
			"output": "sing-box.log",
		},
		"inbounds": []map[string]interface{}{
			{
				"type":        "mixed",
				"tag":         "mixed-in",
				"listen":      "::",
				"listen_port": 2080,
			},
		},
		"outbounds": allOutbounds,
		"route": map[string]interface{}{
			"rules": []map[string]interface{}{
				{
					"protocol": "dns",
					"outbound": "dns-out",
				},
				{
					"geoip":    []string{"private", "cn"},
					"outbound": "direct",
				},
				{
					"geosite":  []string{"cn"},
					"outbound": "direct",
				},
			},
			"final":                 "proxy",
			"auto_detect_interface": true,
		},
	}

	_ = fmt.Sprintf("Profile: %s", profileName)
	return json.MarshalIndent(config, "", "  ")
}
