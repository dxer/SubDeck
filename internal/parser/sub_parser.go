package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
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

func toStringSlice(v interface{}) []string {
	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if val == "" {
			return nil
		}
		parts := strings.Split(val, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	default:
		return nil
	}
}

func toIntSlice(v interface{}) []int {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var out []int
	for _, item := range arr {
		out = append(out, toInt(item))
	}
	return out
}

func toStringMap(v interface{}) map[string]string {
	raw, ok := v.(map[string]interface{})
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, val := range raw {
		if s, ok := val.(string); ok {
			out[k] = s
		} else if val != nil {
			out[k] = fmt.Sprintf("%v", val)
		}
	}
	return out
}

var digitsRegex = regexp.MustCompile(`\d+`)

func parseBandwidthMbps(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		if m := digitsRegex.FindString(val); m != "" {
			n, _ := strconv.Atoi(m)
			return n
		}
		return 0
	default:
		return 0
	}
}

func parseClashTransport(m map[string]interface{}, node *models.Node) {
	if netType, _ := m["network"].(string); netType != "" {
		node.Network = strings.ToLower(netType)
	}
	if wsOpts, ok := m["ws-opts"].(map[string]interface{}); ok {
		node.Network = "ws"
		path, _ := wsOpts["path"].(string)
		headers := toStringMap(wsOpts["headers"])
		node.WSOptions = &models.WSOptions{
			Path:    path,
			Headers: headers,
		}
	} else if grpcOpts, ok := m["grpc-opts"].(map[string]interface{}); ok {
		node.Network = "grpc"
		svc, _ := grpcOpts["grpc-service-name"].(string)
		node.GRPCOptions = &models.GRPCOptions{
			ServiceName: svc,
		}
	}
}

func parseClashCommonTLS(m map[string]interface{}, node *models.Node) {
	if sni, _ := m["sni"].(string); sni != "" {
		node.SNI = sni
	} else if servername, _ := m["servername"].(string); servername != "" {
		node.SNI = servername
	}
	if fp, _ := m["client-fingerprint"].(string); fp != "" {
		node.Fingerprint = fp
	}
	if insecure, ok := m["skip-cert-verify"].(bool); ok {
		node.Insecure = insecure
	}
	if alpn := toStringSlice(m["alpn"]); len(alpn) > 0 {
		node.ALPN = alpn
	}
}

func clashMapToNode(m map[string]interface{}) *models.Node {
	name, _ := m["name"].(string)
	pType, _ := m["type"].(string)
	server, _ := m["server"].(string)
	port := toInt(m["port"])

	if server == "" || port <= 0 || port > 65535 {
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
		tls, _ := m["tls"].(bool)
		node.TLS = tls
		parseClashCommonTLS(m, node)
		parseClashTransport(m, node)
	case "vless":
		node.Type = models.TypeVLESS
		node.UUID, _ = m["uuid"].(string)
		node.Flow, _ = m["flow"].(string)
		tls, _ := m["tls"].(bool)
		node.TLS = tls
		parseClashCommonTLS(m, node)
		if realityOpts, ok := m["reality-opts"].(map[string]interface{}); ok {
			node.TLS = true
			pbk, _ := realityOpts["public-key"].(string)
			sid, _ := realityOpts["short-id"].(string)
			node.Reality = &models.Reality{
				PublicKey: pbk,
				ShortID:   sid,
			}
		}
		parseClashTransport(m, node)
	case "trojan":
		node.Type = models.TypeTrojan
		node.Password, _ = m["password"].(string)
		node.TLS = true
		parseClashCommonTLS(m, node)
		parseClashTransport(m, node)
	case "hysteria2":
		node.Type = models.TypeHysteria2
		node.Password, _ = m["password"].(string)
		node.TLS = true
		parseClashCommonTLS(m, node)
		node.Hy2Obfs, _ = m["obfs"].(string)
		node.Hy2ObfsPassword, _ = m["obfs-password"].(string)
		if strings.EqualFold(node.Hy2Obfs, "none") {
			node.Hy2Obfs, node.Hy2ObfsPassword = "", ""
		}
		node.Hy2UpMbps = parseBandwidthMbps(m["up"])
		node.Hy2DownMbps = parseBandwidthMbps(m["down"])
		if ports, ok := m["ports"].(string); ok {
			node.Hy2Ports = ports
		}
	case "anytls":
		node.Type = models.TypeAnyTLS
		node.Password, _ = m["password"].(string)
		node.TLS = true
		parseClashCommonTLS(m, node)
	case "tuic":
		node.Type = models.TypeTUIC
		node.UUID, _ = m["uuid"].(string)
		node.Password, _ = m["password"].(string)
		node.TLS = true
		parseClashCommonTLS(m, node)
		node.TuicCongestion, _ = m["congestion-controller"].(string)
	case "wireguard":
		node.Type = models.TypeWireGuard
		node.WGPrivateKey, _ = m["private-key"].(string)
		node.WGPublicKey, _ = m["public-key"].(string)
		node.WGLocalAddr, _ = m["ip"].(string)
		node.WGMTU = toInt(m["mtu"])
		node.WGReserved = toIntSlice(m["reserved"])
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

func parseSingboxTLS(ob map[string]interface{}, node *models.Node) {
	tlsMap, ok := ob["tls"].(map[string]interface{})
	if !ok {
		return
	}
	if enabled, ok := tlsMap["enabled"].(bool); ok {
		node.TLS = enabled
	}
	if sni, _ := tlsMap["server_name"].(string); sni != "" {
		node.SNI = sni
	}
	if insecure, ok := tlsMap["insecure"].(bool); ok {
		node.Insecure = insecure
	}
	if alpn := toStringSlice(tlsMap["alpn"]); len(alpn) > 0 {
		node.ALPN = alpn
	}
	if utlsMap, ok := tlsMap["utls"].(map[string]interface{}); ok {
		if fp, _ := utlsMap["fingerprint"].(string); fp != "" {
			node.Fingerprint = fp
		}
	}
	if realityMap, ok := tlsMap["reality"].(map[string]interface{}); ok {
		if enabled, _ := realityMap["enabled"].(bool); enabled || realityMap["public_key"] != nil {
			node.TLS = true
			pbk, _ := realityMap["public_key"].(string)
			sid, _ := realityMap["short_id"].(string)
			node.Reality = &models.Reality{
				PublicKey: pbk,
				ShortID:   sid,
			}
		}
	}
}

func parseSingboxTransport(ob map[string]interface{}, node *models.Node) {
	tr, ok := ob["transport"].(map[string]interface{})
	if !ok {
		return
	}
	trType, _ := tr["type"].(string)
	switch strings.ToLower(trType) {
	case "ws":
		node.Network = "ws"
		path, _ := tr["path"].(string)
		headers := toStringMap(tr["headers"])
		node.WSOptions = &models.WSOptions{
			Path:    path,
			Headers: headers,
		}
	case "grpc":
		node.Network = "grpc"
		svc, _ := tr["service_name"].(string)
		node.GRPCOptions = &models.GRPCOptions{
			ServiceName: svc,
		}
	}
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

		if server == "" || port <= 0 || port > 65535 {
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
			parseSingboxTLS(ob, node)
			parseSingboxTransport(ob, node)
		case "vless":
			node.Type = models.TypeVLESS
			node.UUID, _ = ob["uuid"].(string)
			node.Flow, _ = ob["flow"].(string)
			parseSingboxTLS(ob, node)
			parseSingboxTransport(ob, node)
		case "trojan":
			node.Type = models.TypeTrojan
			node.Password, _ = ob["password"].(string)
			node.TLS = true
			parseSingboxTLS(ob, node)
			parseSingboxTransport(ob, node)
		case "hysteria2":
			node.Type = models.TypeHysteria2
			node.Password, _ = ob["password"].(string)
			node.TLS = true
			parseSingboxTLS(ob, node)
			node.Hy2UpMbps = toInt(ob["up_mbps"])
			node.Hy2DownMbps = toInt(ob["down_mbps"])
			if ports := toStringSlice(ob["server_ports"]); len(ports) > 0 {
				normalized := make([]string, 0, len(ports))
				for _, p := range ports {
					p = strings.TrimSpace(p)
					if p == "" {
						continue
					}
					// sing-box uses "start:end"; internal Hy2Ports keeps mihomo style "start-end"/bare port
					if i := strings.IndexByte(p, ':'); i >= 0 {
						if p[:i] == p[i+1:] {
							p = p[:i]
						} else {
							p = p[:i] + "-" + p[i+1:]
						}
					}
					normalized = append(normalized, p)
				}
				node.Hy2Ports = strings.Join(normalized, ",")
			}
			if obfs, ok := ob["obfs"].(map[string]interface{}); ok {
				node.Hy2Obfs, _ = obfs["type"].(string)
				node.Hy2ObfsPassword, _ = obfs["password"].(string)
				if strings.EqualFold(node.Hy2Obfs, "none") {
					node.Hy2Obfs, node.Hy2ObfsPassword = "", ""
				}
			}
		case "anytls":
			node.Type = models.TypeAnyTLS
			node.Password, _ = ob["password"].(string)
			node.TLS = true
			parseSingboxTLS(ob, node)
		case "tuic":
			node.Type = models.TypeTUIC
			node.UUID, _ = ob["uuid"].(string)
			node.Password, _ = ob["password"].(string)
			node.TLS = true
			parseSingboxTLS(ob, node)
			node.TuicCongestion, _ = ob["congestion_control"].(string)
		case "wireguard":
			node.Type = models.TypeWireGuard
			node.WGPrivateKey, _ = ob["private_key"].(string)
			node.WGPublicKey, _ = ob["peer_public_key"].(string)
			if addrs := toStringSlice(ob["local_address"]); len(addrs) > 0 {
				node.WGLocalAddr = addrs[0]
			}
			node.WGMTU = toInt(ob["mtu"])
			node.WGReserved = toIntSlice(ob["reserved"])
		default:
			continue
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}
