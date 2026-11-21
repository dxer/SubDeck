package parser

import (
	"testing"

	"subdeck/internal/models"
)

func TestParseClashYAMLPreservesFields(t *testing.T) {
	yamlContent := `
proxies:
  - name: "HK VLESS WS"
    type: vless
    server: hk.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    tls: true
    servername: cdn.hk.example.com
    skip-cert-verify: true
    client-fingerprint: chrome
    alpn:
      - h2
      - http/1.1
    network: ws
    ws-opts:
      path: /ws-edge
      headers:
        Host: ws.hk.example.com

  - name: "JP Hy2"
    type: hysteria2
    server: jp.example.com
    port: 8443
    password: secret
    ports: 20000-30000
    up: 50 Mbps
    down: 200 Mbps
    obfs: salamander
    obfs-password: obfs-pw
    sni: jp.example.com
    alpn: [h3]

  - name: "SG TUIC"
    type: tuic
    server: sg.example.com
    port: 443
    uuid: 22222222-2222-2222-2222-222222222222
    password: tuic-pw
    congestion-controller: bbr
    alpn: [h3]

  - name: "US WG"
    type: wireguard
    server: us.example.com
    port: 51820
    public-key: pubkey123
    private-key: privkey123
    ip: 172.16.0.2/32
    mtu: 1420
    reserved: [12, 34, 56]
`

	nodes, err := ParseSubscriptionContent([]byte(yamlContent))
	if err != nil {
		t.Fatalf("ParseSubscriptionContent failed: %v", err)
	}
	if len(nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(nodes))
	}

	vless := nodes[0]
	if vless.Type != models.TypeVLESS || vless.SNI != "cdn.hk.example.com" || !vless.Insecure {
		t.Fatalf("unexpected vless base fields: %+v", vless)
	}
	if vless.Fingerprint != "chrome" || len(vless.ALPN) != 2 || vless.ALPN[0] != "h2" {
		t.Fatalf("unexpected vless tls/alpn: fp=%q alpn=%v", vless.Fingerprint, vless.ALPN)
	}
	if vless.WSOptions == nil || vless.WSOptions.Path != "/ws-edge" || vless.WSOptions.Headers["Host"] != "ws.hk.example.com" {
		t.Fatalf("unexpected vless ws-opts: %+v", vless.WSOptions)
	}

	hy2 := nodes[1]
	if hy2.Hy2UpMbps != 50 || hy2.Hy2DownMbps != 200 || hy2.Hy2Ports != "20000-30000" || hy2.Hy2Obfs != "salamander" {
		t.Fatalf("unexpected hy2 fields: %+v", hy2)
	}

	tuic := nodes[2]
	if tuic.TuicCongestion != "bbr" || len(tuic.ALPN) != 1 || tuic.ALPN[0] != "h3" {
		t.Fatalf("unexpected tuic fields: %+v", tuic)
	}

	wg := nodes[3]
	if wg.WGLocalAddr != "172.16.0.2/32" || wg.WGMTU != 1420 || len(wg.WGReserved) != 3 || wg.WGReserved[1] != 34 {
		t.Fatalf("unexpected wireguard fields: %+v", wg)
	}
}

func TestParseSingboxJSONPreservesFields(t *testing.T) {
	sbJSON := `{
  "outbounds": [
    {"type": "direct", "tag": "direct"},
    {
      "type": "vless",
      "tag": "SG Reality gRPC",
      "server": "1.2.3.4",
      "server_port": 443,
      "uuid": "33333333-3333-3333-3333-333333333333",
      "tls": {
        "enabled": true,
        "server_name": "www.cloudflare.com",
        "alpn": ["h2"],
        "utls": {"enabled": true, "fingerprint": "firefox"},
        "reality": {"enabled": true, "public_key": "pbk123", "short_id": "abcd"}
      },
      "transport": {
        "type": "grpc",
        "service_name": "grpc-tunnel"
      }
    },
    {
      "type": "wireguard",
      "tag": "EU WG",
      "server": "5.6.7.8",
      "server_port": 51820,
      "local_address": ["10.0.0.2/32"],
      "private_key": "priv",
      "peer_public_key": "pub",
      "reserved": [1, 2, 3],
      "mtu": 1280
    }
  ]
}`

	nodes, err := ParseSubscriptionContent([]byte(sbJSON))
	if err != nil {
		t.Fatalf("ParseSubscriptionContent failed: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 proxy nodes (excluding direct), got %d", len(nodes))
	}

	vless := nodes[0]
	if vless.Reality == nil || vless.Reality.PublicKey != "pbk123" || vless.Reality.ShortID != "abcd" {
		t.Fatalf("missing reality info: %+v", vless.Reality)
	}
	if vless.Fingerprint != "firefox" || len(vless.ALPN) != 1 || vless.ALPN[0] != "h2" {
		t.Fatalf("missing utls/alpn: fp=%q alpn=%v", vless.Fingerprint, vless.ALPN)
	}
	if vless.GRPCOptions == nil || vless.GRPCOptions.ServiceName != "grpc-tunnel" {
		t.Fatalf("missing grpc transport: %+v", vless.GRPCOptions)
	}

	wg := nodes[1]
	if wg.WGLocalAddr != "10.0.0.2/32" || len(wg.WGReserved) != 3 || wg.WGReserved[2] != 3 {
		t.Fatalf("unexpected wg local/reserved: %+v", wg)
	}
}

func TestParseSingboxHysteria2ServerPortsNormalized(t *testing.T) {
	sbJSON := `{
  "outbounds": [
    {
      "type": "hysteria2",
      "tag": "hop",
      "server": "1.2.3.4",
      "server_port": 8881,
      "password": "pw",
      "server_ports": ["8881:8881", "50000:51000"],
      "tls": {"enabled": true, "insecure": true}
    }
  ]
}`

	nodes, err := ParseSubscriptionContent([]byte(sbJSON))
	if err != nil {
		t.Fatalf("ParseSubscriptionContent failed: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	// sing-box "start:end" must be normalized to internal mihomo style for round-trip
	if nodes[0].Hy2Ports != "8881,50000-51000" {
		t.Fatalf("expected normalized Hy2Ports %q, got %q", "8881,50000-51000", nodes[0].Hy2Ports)
	}
}
