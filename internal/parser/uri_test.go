package parser

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"subdeck/internal/generator"
	"subdeck/internal/models"
)

func TestParseAllProtocols(t *testing.T) {
	// 1. Shadowsocks
	ssAuth := base64.URLEncoding.EncodeToString([]byte("aes-256-gcm:mysecretpassword"))
	ssURI := "ss://" + ssAuth + "@1.2.3.4:8388#SS-Node"
	node, err := ParseURI(ssURI)
	if err != nil {
		t.Fatalf("Failed to parse SS: %v", err)
	}
	if node.Type != models.TypeShadowsocks || node.Server != "1.2.3.4" || node.Port != 8388 || node.Password != "mysecretpassword" {
		t.Errorf("Unexpected SS node: %+v", node)
	}

	// 2. VMess
	vmessData := map[string]interface{}{
		"v":    "2",
		"ps":   "VMess-Node",
		"add":  "vmess.example.com",
		"port": 443,
		"id":   "b831381d-6324-4d53-ad4f-8cda48b30811",
		"aid":  0,
		"scy":  "auto",
		"net":  "ws",
		"type": "none",
		"host": "vmess.example.com",
		"path": "/ws",
		"tls":  "tls",
		"sni":  "vmess.example.com",
	}
	vBytes, _ := json.Marshal(vmessData)
	vmessURI := "vmess://" + base64.StdEncoding.EncodeToString(vBytes)
	node, err = ParseURI(vmessURI)
	if err != nil {
		t.Fatalf("Failed to parse VMess: %v", err)
	}
	if node.Type != models.TypeVMess || node.Server != "vmess.example.com" || !node.TLS || node.WSOptions == nil || node.WSOptions.Path != "/ws" {
		t.Errorf("Unexpected VMess node: %+v", node)
	}

	// 3. VLESS
	vlessURI := "vless://b831381d-6324-4d53-ad4f-8cda48b30811@vless.example.com:443?type=tcp&security=reality&pbk=12345678&sid=abcdef&sni=vless.example.com&flow=xtls-rprx-vision#VLESS-Reality"
	node, err = ParseURI(vlessURI)
	if err != nil {
		t.Fatalf("Failed to parse VLESS: %v", err)
	}
	if node.Type != models.TypeVLESS || node.Reality == nil || node.Reality.PublicKey != "12345678" || node.Flow != "xtls-rprx-vision" {
		t.Errorf("Unexpected VLESS node: %+v", node)
	}

	// 4. Trojan
	trojanURI := "trojan://password123@trojan.example.com:443?sni=trojan.example.com&type=ws&path=%2Ftrojan-ws#Trojan-Node"
	node, err = ParseURI(trojanURI)
	if err != nil {
		t.Fatalf("Failed to parse Trojan: %v", err)
	}
	if node.Type != models.TypeTrojan || node.Password != "password123" || !node.TLS {
		t.Errorf("Unexpected Trojan node: %+v", node)
	}

	// 5. Hysteria 2
	hy2URI := "hysteria2://mypassword@hy2.example.com:443?sni=hy2.example.com&insecure=1&obfs=salamander&obfs-password=obfspass#Hy2-Node"
	node, err = ParseURI(hy2URI)
	if err != nil {
		t.Fatalf("Failed to parse Hy2: %v", err)
	}
	if node.Type != models.TypeHysteria2 || node.Hy2Obfs != "salamander" || node.Hy2ObfsPassword != "obfspass" {
		t.Errorf("Unexpected Hy2 node: %+v", node)
	}

		// AnyTLS
	anytlsURI := "anytls://mypassword@anytls.example.com:443?sni=anytls.example.com&insecure=1&fp=chrome&alpn=h2,http/1.1#AnyTLS-Node"
	node, err = ParseURI(anytlsURI)
	if err != nil {
		t.Fatalf("Failed to parse AnyTLS: %v", err)
	}
	if node.Type != models.TypeAnyTLS || node.Password != "mypassword" || node.Fingerprint != "chrome" || !node.Insecure {
		t.Errorf("Unexpected AnyTLS node: %+v", node)
	}

	// 6. TUIC
	tuicURI := "tuic://b831381d-6324-4d53-ad4f-8cda48b30811:tuicpass@tuic.example.com:8443?congestion_control=bbr&sni=tuic.example.com#TUIC-Node"
	node, err = ParseURI(tuicURI)
	if err != nil {
		t.Fatalf("Failed to parse TUIC: %v", err)
	}
	if node.Type != models.TypeTUIC || node.Password != "tuicpass" || node.TuicCongestion != "bbr" {
		t.Errorf("Unexpected TUIC node: %+v", node)
	}

	// 7. WireGuard
	wgURI := "wireguard://WGPrivateKeyHere=@wg.example.com:51820?public_key=WGPubKeyHere=&ip=10.0.0.2/32&mtu=1420#WG-Node"
	node, err = ParseURI(wgURI)
	if err != nil {
		t.Fatalf("Failed to parse WireGuard: %v", err)
	}
	if node.Type != models.TypeWireGuard || node.WGPrivateKey != "WGPrivateKeyHere=" || node.WGPublicKey != "WGPubKeyHere=" {
		t.Errorf("Unexpected WireGuard node: %+v", node)
	}

	// 8. Snell
	snellURI := "snell://snell.example.com:1234?psk=snellsecret&version=4#Snell-Node"
	node, err = ParseURI(snellURI)
	if err != nil {
		t.Fatalf("Failed to parse Snell: %v", err)
	}
	if node.Type != models.TypeSnell || node.SnellPSK != "snellsecret" || node.SnellVersion != 4 {
		t.Errorf("Unexpected Snell node: %+v", node)
	}
}

func TestGenerators(t *testing.T) {
	node1 := &models.Node{
		Name:     "HK-Node-01",
		Type:     models.TypeShadowsocks,
		Server:   "1.1.1.1",
		Port:     8388,
		Cipher:   "aes-256-gcm",
		Password: "pass",
	}
	node2 := &models.Node{
		Name:     "SG-Node-02",
		Type:     models.TypeTrojan,
		Server:   "2.2.2.2",
		Port:     443,
		Password: "pass",
		TLS:      true,
		SNI:      "sg.example.com",
	}

	nodes := []*models.Node{node1, node2}

	// Test Clash
	clashYaml, err := generator.GenerateClashFullYAML(nodes, "TestProfile")
	if err != nil || len(clashYaml) == 0 {
		t.Fatalf("Failed to generate Clash YAML: %v", err)
	}

	// Test Singbox
	singboxJSON, err := generator.GenerateSingboxFullJSON(nodes, "TestProfile")
	if err != nil || len(singboxJSON) == 0 {
		t.Fatalf("Failed to generate Singbox JSON: %v", err)
	}

	// Test Base64
	b64 := generator.GenerateBase64(nodes)
	if len(b64) == 0 {
		t.Fatalf("Base64 output is empty")
	}
}


func TestURIEdgeCasesAndSafety(t *testing.T) {
	// 1. Missing userinfo (@) should return an error without panicking
	malformed := []string{
		"vless://example.com:443",
		"trojan://example.com:443",
		"hysteria2://example.com:443",
		"tuic://example.com:443",
		"wireguard://example.com:51820",
		"anytls://example.com:443",
		"vless://uuid@example.com", // missing port
	}
	for _, uri := range malformed {
		if _, err := ParseURI(uri); err == nil {
			t.Errorf("expected error for malformed URI %q, got nil", uri)
		}
	}

	// 2. IPv6 Shadowsocks and special chars (+, %) in fragment name
	ssAuth := base64.URLEncoding.EncodeToString([]byte("aes-256-gcm:secret"))
	ipv6SS := "ss://" + ssAuth + "@[2001:db8::1]:8388#HK+BGP%2099%25"
	node, err := ParseURI(ipv6SS)
	if err != nil {
		t.Fatalf("failed to parse IPv6 SS URI: %v", err)
	}
	if node.Server != "2001:db8::1" || node.Port != 8388 {
		t.Fatalf("unexpected IPv6 host/port: %s:%d", node.Server, node.Port)
	}
	if node.Name != "HK+BGP 99%" {
		t.Fatalf("expected fragment %q, got %q", "HK+BGP 99%", node.Name)
	}

	// 3. Roundtrip IPv6 formatting in NodeToURI
	exported := generator.NodeToURI(node)
	reparsed, err := ParseURI(exported)
	if err != nil {
		t.Fatalf("failed to reparse exported IPv6 URI %q: %v", exported, err)
	}
	if reparsed.Server != "2001:db8::1" || reparsed.Port != 8388 || reparsed.Name != "HK+BGP 99%" {
		t.Fatalf("unexpected reparsed node: %+v", reparsed)
	}
}

func TestHysteria2PeerObfsNoneAndOfficialBandwidthParams(t *testing.T) {
	// Real-world style link: peer (SNI alias), hpkp cert pin, obfs=none,
	// official upmbps/downmbps and mport port hopping.
	raw := "hysteria2://pw@hy.example.com:8881?peer=addons.mozilla.org&hpkp=BC:1B:F4:DB&obfs=none&upmbps=200&downmbps=1000&mport=8881,50000-51000#hy2"
	node, err := ParseURI(raw)
	if err != nil {
		t.Fatalf("parse hysteria2: %v", err)
	}
	if node.SNI != "addons.mozilla.org" {
		t.Fatalf("expected SNI from peer param, got %q", node.SNI)
	}
	if !node.Insecure {
		t.Fatal("expected insecure=true when a cert pin (hpkp) is present")
	}
	if node.Hy2Obfs != "" {
		t.Fatalf("expected obfs=none normalized to empty, got %q", node.Hy2Obfs)
	}
	if node.Hy2UpMbps != 200 || node.Hy2DownMbps != 1000 {
		t.Fatalf("expected upmbps/downmbps parsed, got up=%d down=%d", node.Hy2UpMbps, node.Hy2DownMbps)
	}
	if node.Hy2Ports != "8881,50000-51000" {
		t.Fatalf("expected mport preserved, got %q", node.Hy2Ports)
	}
}

func TestTUICAndAnyTLSPeerFallback(t *testing.T) {
	tuic, err := ParseURI("tuic://uuid:pw@t.example.com:443?peer=masq.example.com&congestion_control=bbr#tuic")
	if err != nil {
		t.Fatalf("parse tuic: %v", err)
	}
	if tuic.SNI != "masq.example.com" {
		t.Fatalf("expected TUIC SNI from peer, got %q", tuic.SNI)
	}

	anytls, err := ParseURI("anytls://pw@a.example.com:443?peer=masq.example.com&pinSHA256=9FEE02A3#at")
	if err != nil {
		t.Fatalf("parse anytls: %v", err)
	}
	if anytls.SNI != "masq.example.com" || !anytls.Insecure {
		t.Fatalf("expected AnyTLS peer SNI + insecure for pin, got sni=%q insecure=%v", anytls.SNI, anytls.Insecure)
	}
}
