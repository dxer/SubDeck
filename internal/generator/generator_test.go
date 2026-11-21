package generator

import (
	"encoding/json"
	"strings"
	"testing"

	"subdeck/internal/models"

	"gopkg.in/yaml.v3"
)

func sampleNodes() []*models.Node {
	return []*models.Node{
		{
			Name:        "香港 01",
			Type:        models.TypeVLESS,
			Server:      "hk.example.com",
			Port:        443,
			UUID:        "11111111-1111-1111-1111-111111111111",
			TLS:         true,
			SNI:         "hk.example.com",
			ALPN:        []string{"h2", "http/1.1"},
			Fingerprint: "chrome",
			Network:     "ws",
			WSOptions: &models.WSOptions{
				Path:    "/ws",
				Headers: map[string]string{"Host": "cdn.hk.example.com"},
			},
		},
		{
			Name:        "日本 Hy2",
			Type:        models.TypeHysteria2,
			Server:      "jp.example.com",
			Port:        443,
			Password:    "pw",
			SNI:         "jp.example.com",
			ALPN:        []string{"h3"},
			Hy2Ports:    "20000-30000",
			Hy2UpMbps:   50,
			Hy2DownMbps: 200,
		},
	}
}

func TestGenerateClashFullYAMLWithRegionGroups(t *testing.T) {
	data, err := GenerateClashFullYAMLWithOptions(sampleNodes(), "TestProfile", true)
	if err != nil {
		t.Fatalf("GenerateClashFullYAMLWithOptions failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid generated YAML: %v", err)
	}

	dnsMap, ok := parsed["dns"].(map[string]interface{})
	if !ok || dnsMap["enhanced-mode"] != "fake-ip" {
		t.Fatalf("expected dns.enhanced-mode=fake-ip, got %v", parsed["dns"])
	}

	groups, ok := parsed["proxy-groups"].([]interface{})
	if !ok {
		t.Fatalf("missing proxy-groups")
	}

	var hasHKGroup, hasJPGroup bool
	for _, g := range groups {
		gm, _ := g.(map[string]interface{})
		name, _ := gm["name"].(string)
		if name == "🇭🇰 香港节点" {
			hasHKGroup = true
		}
		if name == "🇯🇵 日本节点" {
			hasJPGroup = true
		}
	}
	if !hasHKGroup || !hasJPGroup {
		t.Fatalf("expected HK and JP region groups, got %v", groups)
	}
}

func TestGenerateSingbox112FullJSONWithRegionGroups(t *testing.T) {
	data, err := GenerateSingboxFullJSONWithOptions(sampleNodes(), "TestProfile", true)
	if err != nil {
		t.Fatalf("GenerateSingboxFullJSONWithOptions failed: %v", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid generated JSON: %v", err)
	}

	// Ensure deprecated block/dns outbounds are NOT emitted for Sing-box 1.12+
	rawStr := string(data)
	if strings.Contains(rawStr, `"type": "block"`) || strings.Contains(rawStr, `"type": "dns"`) {
		t.Fatalf("unexpected deprecated block/dns outbound in Sing-box 1.12+ config:\n%s", rawStr)
	}

	outbounds, _ := cfg["outbounds"].([]interface{})
	var foundHKRegion, foundHy2Ports, foundVLESSUTLS bool
	for _, ob := range outbounds {
		m, _ := ob.(map[string]interface{})
		if m["tag"] == "🇭🇰 香港节点" && m["type"] == "urltest" {
			foundHKRegion = true
		}
		if m["type"] == "hysteria2" && m["server_ports"] != nil {
			foundHy2Ports = true
			// sing-box requires "start:end" ranges; bare ports / dash ranges are fatal
			ports, _ := m["server_ports"].([]interface{})
			if len(ports) == 0 {
				t.Fatalf("expected non-empty server_ports, got: %v", m["server_ports"])
			}
			for _, p := range ports {
				s, _ := p.(string)
				if !strings.Contains(s, ":") {
					t.Fatalf("server_ports entry %q must use sing-box \"start:end\" format", s)
				}
			}
		}
		if m["type"] == "vless" {
			if tlsMap, ok := m["tls"].(map[string]interface{}); ok {
				if tlsMap["utls"] != nil && tlsMap["alpn"] != nil {
					foundVLESSUTLS = true
				}
			}
		}
	}
	if !foundHKRegion || !foundHy2Ports || !foundVLESSUTLS {
		t.Fatalf("missing expected 1.12 fields: hk=%v hy2Ports=%v vlessUtls=%v", foundHKRegion, foundHy2Ports, foundVLESSUTLS)
	}

	route, _ := cfg["route"].(map[string]interface{})
	if route["rule_set"] == nil {
		t.Fatalf("expected route.rule_set for Sing-box 1.12+")
	}
}

func TestBase64URIGeneratorIncludesALPNAndBandwidth(t *testing.T) {
	nodes := sampleNodes()
	vlessURI := NodeToURI(nodes[0])
	hy2URI := NodeToURI(nodes[1])
	if !strings.Contains(vlessURI, "alpn=h2%2Chttp%2F1.1") && !strings.Contains(vlessURI, "alpn=h2,http/1.1") {
		t.Fatalf("expected vless URI to contain alpn, got: %s", vlessURI)
	}
	if !strings.Contains(hy2URI, "upmbps=50") || !strings.Contains(hy2URI, "downmbps=200") || !strings.Contains(hy2URI, "mport=20000-30000") {
		t.Fatalf("expected hysteria2 URI to contain upmbps/downmbps/mport, got: %s", hy2URI)
	}
	if encoded := GenerateBase64(nodes); encoded == "" {
		t.Fatalf("expected non-empty base64 output")
	}
}

func TestHysteria2ObfsNoneIsDroppedFromAllFormats(t *testing.T) {
	nodes := []*models.Node{{
		Name:            "node-a",
		Type:            models.TypeHysteria2,
		Server:          "a.example.com",
		Port:            443,
		Password:        "pw",
		Hy2Obfs:         "none",
		Hy2ObfsPassword: "",
		Hy2Ports:        "443,50000-51000",
	}}

	clashBytes, err := GenerateClashFullYAML(nodes, "t")
	if err != nil {
		t.Fatalf("clash full yaml: %v", err)
	}
	if strings.Contains(string(clashBytes), "obfs:") || strings.Contains(string(clashBytes), "obfs-password:") {
		t.Fatalf("expected no obfs key in clash yaml, got:\n%s", clashBytes)
	}
	if !strings.Contains(string(clashBytes), "ports: 443,50000-51000") {
		t.Fatalf("expected port hopping preserved in clash yaml, got:\n%s", clashBytes)
	}

	provBytes, err := GenerateClashProxiesYAML(nodes)
	if err != nil {
		t.Fatalf("clash provider yaml: %v", err)
	}
	if strings.Contains(string(provBytes), "obfs:") || strings.Contains(string(provBytes), "obfs-password:") {
		t.Fatalf("expected no obfs key in provider yaml, got:\n%s", provBytes)
	}

	uri := NodeToURI(nodes[0])
	if strings.Contains(uri, "obfs=") {
		t.Fatalf("expected no obfs param in URI round-trip, got: %s", uri)
	}
	if !strings.Contains(uri, "mport=443%2C50000-51000") && !strings.Contains(uri, "mport=443,50000-51000") {
		t.Fatalf("expected mport preserved in URI round-trip, got: %s", uri)
	}
}

func TestHysteria2SalamanderObfsKeptWhenComplete(t *testing.T) {
	nodes := []*models.Node{{
		Name:            "node-b",
		Type:            models.TypeHysteria2,
		Server:          "a.example.com",
		Port:            443,
		Password:        "pw",
		Hy2Obfs:         "salamander",
		Hy2ObfsPassword: "obfspw",
	}}

	clashBytes, err := GenerateClashFullYAML(nodes, "t")
	if err != nil {
		t.Fatalf("clash full yaml: %v", err)
	}
	if !strings.Contains(string(clashBytes), "obfs: salamander") || !strings.Contains(string(clashBytes), "obfs-password: obfspw") {
		t.Fatalf("expected salamander obfs in clash yaml, got:\n%s", clashBytes)
	}

	// password-less obfs must be dropped (mihomo rejects it: "missing obfs password")
	nodes[0].Hy2ObfsPassword = ""
	clashBytes2, err := GenerateClashFullYAML(nodes, "t")
	if err != nil {
		t.Fatalf("clash full yaml (no pw): %v", err)
	}
	if strings.Contains(string(clashBytes2), "obfs:") || strings.Contains(string(clashBytes2), "obfs-password:") {
		t.Fatalf("expected obfs dropped without password, got:\n%s", clashBytes2)
	}
}

func TestRulePresetsAIStreamingAndMirror(t *testing.T) {
	opts := ExportOptions{
		RegionGroups:  true,
		RulePreset:    RulePresetAIStreaming,
		RulesetMirror: "https://ghproxy.net",
	}

	clashOut, err := GenerateClashFullYAMLWithConfig(sampleNodes(), "AIProfile", opts)
	if err != nil {
		t.Fatalf("GenerateClashFullYAMLWithConfig failed: %v", err)
	}
	var parsedClash map[string]interface{}
	if err := yaml.Unmarshal(clashOut, &parsedClash); err != nil {
		t.Fatalf("invalid Clash YAML: %v", err)
	}
	groups, _ := parsedClash["proxy-groups"].([]interface{})
	var hasAIGroup bool
	for _, g := range groups {
		gm, _ := g.(map[string]interface{})
		if gm["name"] == "🤖 AI 服务" {
			hasAIGroup = true
			break
		}
	}
	rules, _ := parsedClash["rules"].([]interface{})
	var hasAIRule bool
	for _, r := range rules {
		if r == "DOMAIN-SUFFIX,openai.com,🤖 AI 服务" {
			hasAIRule = true
			break
		}
	}
	if !hasAIGroup || !hasAIRule {
		t.Fatalf("expected AI service group (%v) & rule (%v) in Clash output", hasAIGroup, hasAIRule)
	}

	sbOut, err := GenerateSingboxFullJSONWithConfig(sampleNodes(), "AIProfile", opts)
	if err != nil {
		t.Fatalf("GenerateSingboxFullJSONWithConfig failed: %v", err)
	}
	sbStr := string(sbOut)
	if !strings.Contains(sbStr, `"tag": "ai"`) || !strings.Contains(sbStr, "https://ghproxy.net/https://raw.githubusercontent.com/") {
		t.Fatalf("expected AI selector outbound & mirrored .srs URL in Sing-box output:\n%s", sbStr)
	}
}

func TestApplyClashTemplateMerge(t *testing.T) {
	cfg := map[string]interface{}{
		"mixed-port":   7890,
		"proxies":      []interface{}{map[string]interface{}{"name": "node-a", "type": "ss"}},
		"proxy-groups": []interface{}{map[string]interface{}{"name": "🚀 节点选择"}},
		"rules":        []interface{}{"MATCH,🚀 节点选择"},
		"dns":          map[string]interface{}{"enable": true, "fake-ip-range": "198.18.0.1/16"},
	}
	tmpl := map[string]interface{}{
		"mixed-port": 7897,
		"dns":        map[string]interface{}{"fake-ip-range": "10.0.0.1/24", "ipv6": true},
		"rules":      []interface{}{"DOMAIN-SUFFIX,example.com,DIRECT", "MATCH,🚀 节点选择"},
		"proxies": []interface{}{
			map[string]interface{}{"name": "tpl-extra", "type": "ss", "server": "2.2.2.2", "port": 8388},
		},
		"tun": nil, // null deletes
	}

	if err := ApplyClashTemplate(cfg, tmpl); err != nil {
		t.Fatalf("template apply failed: %v", err)
	}
	if cfg["mixed-port"] != 7897 {
		t.Fatalf("scalar replace failed: %v", cfg["mixed-port"])
	}
	dns := cfg["dns"].(map[string]interface{})
	if dns["fake-ip-range"] != "10.0.0.1/24" || dns["ipv6"] != true || dns["enable"] != true {
		t.Fatalf("nested dns merge failed: %+v", dns)
	}
	rules := cfg["rules"].([]interface{})
	if len(rules) != 2 || rules[0] != "DOMAIN-SUFFIX,example.com,DIRECT" {
		t.Fatalf("rules array replace failed: %+v", rules)
	}
	proxies := cfg["proxies"].([]interface{})
	if len(proxies) != 2 || proxies[0].(map[string]interface{})["name"] != "node-a" || proxies[1].(map[string]interface{})["name"] != "tpl-extra" {
		t.Fatalf("proxies must keep generated nodes then append template entries: %+v", proxies)
	}
	if _, exists := cfg["tun"]; exists {
		t.Fatalf("null template key must delete base key")
	}

	// Name collision must be rejected
	err := ApplyClashTemplate(cfg, map[string]interface{}{
		"proxies": []interface{}{map[string]interface{}{"name": "node-a", "type": "ss"}},
	})
	if err == nil || !strings.Contains(err.Error(), "node-a") {
		t.Fatalf("expected node name collision error, got %v", err)
	}
}

func TestApplySingboxTemplateMerge(t *testing.T) {
	cfg := map[string]interface{}{
		"outbounds": []interface{}{
			map[string]interface{}{"tag": "node-a", "type": "shadowsocks"},
			map[string]interface{}{"tag": "proxy", "type": "selector"},
		},
		"route": map[string]interface{}{"final": "proxy"},
	}
	tmpl := map[string]interface{}{
		"route": map[string]interface{}{"final": "my-select", "auto_detect_interface": false},
		"outbounds": []interface{}{
			map[string]interface{}{"tag": "my-select", "type": "selector", "outbounds": []interface{}{"node-a"}},
		},
	}

	if err := ApplySingboxTemplate(cfg, tmpl); err != nil {
		t.Fatalf("template apply failed: %v", err)
	}
	route := cfg["route"].(map[string]interface{})
	if route["final"] != "my-select" || route["auto_detect_interface"] != false {
		t.Fatalf("route merge failed: %+v", route)
	}
	obs := cfg["outbounds"].([]interface{})
	if len(obs) != 3 || obs[0].(map[string]interface{})["tag"] != "node-a" || obs[1].(map[string]interface{})["tag"] != "proxy" || obs[2].(map[string]interface{})["tag"] != "my-select" {
		t.Fatalf("outbounds must keep generated then append template: %+v", obs)
	}

	err := ApplySingboxTemplate(cfg, map[string]interface{}{
		"outbounds": []interface{}{map[string]interface{}{"tag": "node-a"}},
	})
	if err == nil || !strings.Contains(err.Error(), "node-a") {
		t.Fatalf("expected tag collision error, got %v", err)
	}
}

func TestFullConfigWithClashTemplate(t *testing.T) {
	nodes := []*models.Node{{Name: "A", Type: models.TypeShadowsocks, Server: "1.1.1.1", Port: 8388, Cipher: "aes-128-gcm", Password: "p"}}
	tmpl, err := ParseClashTemplate("mixed-port: 9090\nrules:\n  - MATCH,DIRECT\n")
	if err != nil {
		t.Fatalf("template parse failed: %v", err)
	}
	data, err := GenerateClashFullYAMLWithConfig(nodes, "tpl", ExportOptions{ClashTemplate: tmpl})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "mixed-port: 9090") {
		t.Fatalf("template override missing from output")
	}
	if !strings.Contains(out, "name: A") {
		t.Fatalf("generated node missing from templated output")
	}
}
