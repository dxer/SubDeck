package generator

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"subdeck/internal/models"
)

// NodeToURI converts a unified Node struct into a standard shareable URI
func NodeToURI(n *models.Node) string {
	if n == nil {
		return ""
	}

	name := url.PathEscape(n.Name)
	hostPort := net.JoinHostPort(n.Server, strconv.Itoa(n.Port))

	switch n.Type {
	case models.TypeShadowsocks:
		// ss://base64(cipher:password)@server:port#name
		userInfo := base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", n.Cipher, n.Password)))
		return fmt.Sprintf("ss://%s@%s#%s", userInfo, hostPort, name)

	case models.TypeVMess:
		// vmess://base64(json)
		v := map[string]interface{}{
			"v":    "2",
			"ps":   n.Name,
			"add":  n.Server,
			"port": n.Port,
			"id":   n.UUID,
			"aid":  n.AlterId,
			"scy":  n.Cipher,
			"net":  n.Network,
			"type": "none",
			"tls":  "",
		}
		if n.TLS {
			v["tls"] = "tls"
			v["sni"] = n.SNI
		}
		if len(n.ALPN) > 0 {
			v["alpn"] = strings.Join(n.ALPN, ",")
		}
		if n.Fingerprint != "" {
			v["fp"] = n.Fingerprint
		}
		if n.WSOptions != nil {
			v["path"] = n.WSOptions.Path
			if host, ok := n.WSOptions.Headers["Host"]; ok {
				v["host"] = host
			}
		} else if n.GRPCOptions != nil {
			v["path"] = n.GRPCOptions.ServiceName
		}
		data, _ := json.Marshal(v)
		return fmt.Sprintf("vmess://%s", base64.StdEncoding.EncodeToString(data))

	case models.TypeVLESS:
		// vless://uuid@server:port?type=...&security=...#name
		q := url.Values{}
		if n.Network != "" {
			q.Set("type", n.Network)
		}
		if n.Flow != "" {
			q.Set("flow", n.Flow)
		}
		if n.Reality != nil {
			q.Set("security", "reality")
			q.Set("pbk", n.Reality.PublicKey)
			if n.Reality.ShortID != "" {
				q.Set("sid", n.Reality.ShortID)
			}
		} else if n.TLS {
			q.Set("security", "tls")
		}
		if n.SNI != "" {
			q.Set("sni", n.SNI)
		}
		if n.Insecure {
			q.Set("allowInsecure", "1")
		}
		if len(n.ALPN) > 0 {
			q.Set("alpn", strings.Join(n.ALPN, ","))
		}
		if n.Fingerprint != "" {
			q.Set("fp", n.Fingerprint)
		}
		if n.WSOptions != nil {
			q.Set("path", n.WSOptions.Path)
			if host, ok := n.WSOptions.Headers["Host"]; ok {
				q.Set("host", host)
			}
		} else if n.GRPCOptions != nil {
			q.Set("serviceName", n.GRPCOptions.ServiceName)
		}
		return fmt.Sprintf("vless://%s@%s?%s#%s", url.PathEscape(n.UUID), hostPort, q.Encode(), name)

	case models.TypeTrojan:
		// trojan://password@server:port?sni=...#name
		q := url.Values{}
		if n.SNI != "" {
			q.Set("sni", n.SNI)
		}
		if n.Insecure {
			q.Set("allowInsecure", "1")
		}
		if n.Network != "" {
			q.Set("type", n.Network)
		}
		if n.Fingerprint != "" {
			q.Set("fp", n.Fingerprint)
		}
		if len(n.ALPN) > 0 {
			q.Set("alpn", strings.Join(n.ALPN, ","))
		}
		if n.WSOptions != nil {
			q.Set("path", n.WSOptions.Path)
			if host, ok := n.WSOptions.Headers["Host"]; ok {
				q.Set("host", host)
			}
		} else if n.GRPCOptions != nil {
			q.Set("serviceName", n.GRPCOptions.ServiceName)
		}
		return fmt.Sprintf("trojan://%s@%s?%s#%s", url.QueryEscape(n.Password), hostPort, q.Encode(), name)

	case models.TypeHysteria2:
		// hysteria2://password@server:port?sni=...#name
		q := url.Values{}
		if n.SNI != "" {
			q.Set("sni", n.SNI)
		}
		if n.Insecure {
			q.Set("insecure", "1")
		}
		if len(n.ALPN) > 0 {
			q.Set("alpn", strings.Join(n.ALPN, ","))
		}
		if n.Hy2UpMbps > 0 {
			q.Set("upmbps", strconv.Itoa(n.Hy2UpMbps))
		}
		if n.Hy2DownMbps > 0 {
			q.Set("downmbps", strconv.Itoa(n.Hy2DownMbps))
		}
		if n.Hy2ObfsEnabled() {
			q.Set("obfs", n.Hy2Obfs)
			q.Set("obfs-password", n.Hy2ObfsPassword)
		}
		if n.Hy2Ports != "" {
			q.Set("mport", n.Hy2Ports)
		}
		return fmt.Sprintf("hysteria2://%s@%s?%s#%s", url.QueryEscape(n.Password), hostPort, q.Encode(), name)

	case models.TypeAnyTLS:
		q := url.Values{}
		if n.SNI != "" {
			q.Set("sni", n.SNI)
		}
		if n.Fingerprint != "" {
			q.Set("fp", n.Fingerprint)
		}
		if len(n.ALPN) > 0 {
			q.Set("alpn", strings.Join(n.ALPN, ","))
		}
		if n.Insecure {
			q.Set("insecure", "1")
		}
		if len(n.ALPN) > 0 {
			q.Set("alpn", strings.Join(n.ALPN, ","))
		}
		return fmt.Sprintf("anytls://%s@%s?%s#%s", url.QueryEscape(n.Password), hostPort, q.Encode(), name)

	case models.TypeTUIC:
		q := url.Values{}
		if n.SNI != "" {
			q.Set("sni", n.SNI)
		}
		if n.Insecure {
			q.Set("allow_insecure", "1")
		}
		if n.TuicCongestion != "" {
			q.Set("congestion_control", n.TuicCongestion)
		}
		if len(n.ALPN) > 0 {
			q.Set("alpn", strings.Join(n.ALPN, ","))
		}
		return fmt.Sprintf("tuic://%s:%s@%s?%s#%s", url.PathEscape(n.UUID), url.QueryEscape(n.Password), hostPort, q.Encode(), name)

	case models.TypeWireGuard:
		q := url.Values{}
		if n.WGPublicKey != "" {
			q.Set("public_key", n.WGPublicKey)
		}
		if n.WGLocalAddr != "" {
			q.Set("ip", n.WGLocalAddr)
		}
		if n.WGMTU != 0 {
			q.Set("mtu", strconv.Itoa(n.WGMTU))
		}
		return fmt.Sprintf("wireguard://%s@%s?%s#%s", url.PathEscape(n.WGPrivateKey), hostPort, q.Encode(), name)

	case models.TypeSnell:
		q := url.Values{}
		if n.SnellPSK != "" {
			q.Set("psk", n.SnellPSK)
		}
		if n.SnellVersion != 0 {
			q.Set("version", strconv.Itoa(n.SnellVersion))
		}
		return fmt.Sprintf("snell://%s?%s#%s", hostPort, q.Encode(), name)

	default:
		if n.RawURI != "" {
			return n.RawURI
		}
		return ""
	}
}

// GenerateBase64 generates a base64-encoded string of multi-line URIs
func GenerateBase64(nodes []*models.Node) string {
	var lines []string
	for _, n := range nodes {
		uri := NodeToURI(n)
		if uri != "" {
			lines = append(lines, uri)
		}
	}
	content := strings.Join(lines, "\n")
	return base64.StdEncoding.EncodeToString([]byte(content))
}
