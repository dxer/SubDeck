package parser

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"subdeck/internal/models"
)

// SafeBase64Decode decodes base64 string handling missing padding and URL-safe variants
func SafeBase64Decode(str string) ([]byte, error) {
	str = strings.TrimSpace(str)
	if b, err := base64.StdEncoding.DecodeString(str); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(str); err == nil {
		return b, nil
	}
	padLen := (4 - (len(str) % 4)) % 4
	padded := str + strings.Repeat("=", padLen)
	if b, err := base64.StdEncoding.DecodeString(padded); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(padded); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(str); err == nil {
		return b, nil
	}
	return base64.RawURLEncoding.DecodeString(str)
}

func safeUsername(u *url.URL) string {
	if u == nil || u.User == nil {
		return ""
	}
	return u.User.Username()
}

func safePassword(u *url.URL) string {
	if u == nil || u.User == nil {
		return ""
	}
	pass, _ := u.User.Password()
	return pass
}

func parseFragmentName(u *url.URL, fallback string) string {
	if u != nil && u.Fragment != "" {
		return u.Fragment
	}
	return fallback
}

// ParseURI parses any supported proxy URI string into a unified Node
func ParseURI(raw string) (*models.Node, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty URI")
	}

	parts := strings.SplitN(raw, "://", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid URI scheme: %s", raw)
	}

	scheme := strings.ToLower(parts[0])
	body := parts[1]

	var node *models.Node
	var err error

	switch scheme {
	case "vmess":
		node, err = parseVMess(body)
	case "vless":
		node, err = parseVLESS(raw)
	case "ss":
		node, err = parseShadowsocks(raw)
	case "trojan":
		node, err = parseTrojan(raw)
	case "hysteria2", "hy2":
		node, err = parseHysteria2(raw)
	case "anytls":
		node, err = parseAnyTLS(raw)
	case "tuic":
		node, err = parseTUIC(raw)
	case "wireguard", "wg":
		node, err = parseWireGuard(raw)
	case "snell":
		node, err = parseSnell(raw)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", scheme)
	}

	if err != nil {
		return nil, err
	}
	if node.Server == "" || node.Port <= 0 || node.Port > 65535 {
		return nil, fmt.Errorf("invalid server or port in URI: %s", raw)
	}
	if err := validateNodeCredentials(node); err != nil {
		return nil, fmt.Errorf("%w: %s", err, raw)
	}
	node.RawURI = raw
	return node, nil
}

func validateNodeCredentials(n *models.Node) error {
	switch n.Type {
	case models.TypeVMess, models.TypeVLESS:
		if n.UUID == "" {
			return errors.New("missing uuid")
		}
	case models.TypeTrojan, models.TypeHysteria2, models.TypeAnyTLS:
		if n.Password == "" {
			return errors.New("missing password")
		}
	case models.TypeTUIC:
		if n.UUID == "" {
			return errors.New("missing uuid")
		}
	case models.TypeWireGuard:
		if n.WGPrivateKey == "" {
			return errors.New("missing wireguard private key")
		}
	case models.TypeShadowsocks:
		if n.Cipher == "" || n.Password == "" {
			return errors.New("missing shadowsocks cipher or password")
		}
	}
	return nil
}

// VMess: vmess://base64(json)
type vmessJSON struct {
	V    interface{} `json:"v"`
	PS   string      `json:"ps"`
	Add  string      `json:"add"`
	Port interface{} `json:"port"`
	ID   string      `json:"id"`
	Aid  interface{} `json:"aid"`
	Scy  string      `json:"scy"`
	Net  string      `json:"net"`
	Type string      `json:"type"`
	Host string      `json:"host"`
	Path string      `json:"path"`
	TLS  interface{} `json:"tls"`
	SNI  string      `json:"sni"`
	ALPN string      `json:"alpn"`
	FP   string      `json:"fp"`
}

func isVMessTLS(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		s := strings.ToLower(strings.TrimSpace(val))
		return s == "tls" || s == "true" || s == "1"
	default:
		return false
	}
}

func parseVMess(body string) (*models.Node, error) {
	data, err := SafeBase64Decode(body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode vmess base64: %w", err)
	}

	var v vmessJSON
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("failed to parse vmess json: %w", err)
	}

	port := toInt(v.Port)
	alterID := toInt(v.Aid)

	node := &models.Node{
		Type:        models.TypeVMess,
		Name:        v.PS,
		Server:      v.Add,
		Port:        port,
		UUID:        v.ID,
		AlterId:     alterID,
		Cipher:      v.Scy,
		Network:     v.Net,
		TLS:         isVMessTLS(v.TLS),
		SNI:         v.SNI,
		Fingerprint: v.FP,
	}

	if node.Name == "" {
		node.Name = fmt.Sprintf("%s:%d", v.Add, port)
	}
	if node.Cipher == "" {
		node.Cipher = "auto"
	}
	if node.SNI == "" && v.Host != "" && node.TLS {
		node.SNI = v.Host
	}

	if v.ALPN != "" {
		node.ALPN = strings.Split(v.ALPN, ",")
	}

	switch strings.ToLower(v.Net) {
	case "ws":
		headers := make(map[string]string)
		if v.Host != "" {
			headers["Host"] = v.Host
		}
		node.WSOptions = &models.WSOptions{
			Path:    v.Path,
			Headers: headers,
		}
	case "grpc":
		node.GRPCOptions = &models.GRPCOptions{
			ServiceName: v.Path,
		}
	case "http", "tcp":
		if v.Type == "http" {
			headers := make(map[string]string)
			if v.Host != "" {
				headers["Host"] = v.Host
			}
			node.HTTPOptions = &models.HTTPOptions{
				Headers: headers,
			}
			if v.Path != "" {
				node.HTTPOptions.Path = []string{v.Path}
			}
		}
	}

	return node, nil
}

// VLESS: vless://uuid@host:port?type=...&security=...#name
func parseVLESS(raw string) (*models.Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	port, _ := strconv.Atoi(u.Port())
	name := parseFragmentName(u, u.Host)

	security := q.Get("security")
	node := &models.Node{
		Type:        models.TypeVLESS,
		Name:        name,
		Server:      u.Hostname(),
		Port:        port,
		UUID:        safeUsername(u),
		Network:     q.Get("type"),
		Flow:        q.Get("flow"),
		TLS:         security == "tls" || security == "reality",
		SNI:         q.Get("sni"),
		Fingerprint: q.Get("fp"),
		Insecure:    q.Get("allowInsecure") == "1" || q.Get("insecure") == "1",
	}

	if node.Network == "" {
		node.Network = "tcp"
	}

	if alpn := q.Get("alpn"); alpn != "" {
		node.ALPN = strings.Split(alpn, ",")
	}

	if security == "reality" {
		node.Reality = &models.Reality{
			PublicKey: q.Get("pbk"),
			ShortID:   q.Get("sid"),
		}
	}

	if node.Network == "ws" {
		headers := make(map[string]string)
		if host := q.Get("host"); host != "" {
			headers["Host"] = host
		}
		node.WSOptions = &models.WSOptions{
			Path:    q.Get("path"),
			Headers: headers,
		}
	} else if node.Network == "grpc" {
		node.GRPCOptions = &models.GRPCOptions{
			ServiceName: q.Get("serviceName"),
		}
	}

	return node, nil
}

// Shadowsocks: ss://base64(method:password@host:port)#name or ss://base64(method:password)@host:port#name
func parseShadowsocks(raw string) (*models.Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	name := parseFragmentName(u, "")
	body := strings.TrimPrefix(raw, "ss://")
	if idx := strings.Index(body, "#"); idx != -1 {
		body = body[:idx]
	}

	var cipher, password, server string
	var port int

	if strings.Contains(body, "@") {
		parts := strings.SplitN(body, "@", 2)
		authData, err := SafeBase64Decode(parts[0])
		if err != nil {
			authParts := strings.SplitN(parts[0], ":", 2)
			if len(authParts) == 2 {
				cipher = authParts[0]
				password = authParts[1]
			}
		} else {
			authParts := strings.SplitN(string(authData), ":", 2)
			if len(authParts) == 2 {
				cipher = authParts[0]
				password = authParts[1]
			}
		}

		serverPort := parts[1]
		if qIdx := strings.Index(serverPort, "?"); qIdx != -1 {
			serverPort = serverPort[:qIdx]
		}
		if host, portStr, splitErr := net.SplitHostPort(serverPort); splitErr == nil {
			server = host
			port, _ = strconv.Atoi(portStr)
		}
	} else {
		decoded, err := SafeBase64Decode(body)
		if err != nil {
			return nil, fmt.Errorf("invalid shadowsocks base64: %w", err)
		}
		str := string(decoded)
		parts := strings.SplitN(str, "@", 2)
		if len(parts) == 2 {
			authParts := strings.SplitN(parts[0], ":", 2)
			if len(authParts) == 2 {
				cipher = authParts[0]
				password = authParts[1]
			}
			if host, portStr, splitErr := net.SplitHostPort(parts[1]); splitErr == nil {
				server = host
				port, _ = strconv.Atoi(portStr)
			}
		}
	}

	if name == "" {
		name = fmt.Sprintf("SS-%s:%d", server, port)
	}

	return &models.Node{
		Type:     models.TypeShadowsocks,
		Name:     name,
		Server:   server,
		Port:     port,
		Cipher:   cipher,
		Password: password,
	}, nil
}

// Trojan: trojan://password@host:port?sni=...#name
func parseTrojan(raw string) (*models.Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	port, _ := strconv.Atoi(u.Port())
	name := parseFragmentName(u, u.Host)

	q := u.Query()
	sni := q.Get("sni")
	if sni == "" {
		sni = q.Get("peer")
	}
	if sni == "" {
		sni = u.Hostname()
	}

	node := &models.Node{
		Type:        models.TypeTrojan,
		Name:        name,
		Server:      u.Hostname(),
		Port:        port,
		Password:    safeUsername(u),
		TLS:         true,
		SNI:         sni,
		Fingerprint: q.Get("fp"),
		Insecure:    q.Get("allowInsecure") == "1" || q.Get("insecure") == "1",
		Network:     q.Get("type"),
	}

	if alpn := q.Get("alpn"); alpn != "" {
		node.ALPN = strings.Split(alpn, ",")
	}

	if node.Network == "ws" {
		headers := make(map[string]string)
		if host := q.Get("host"); host != "" {
			headers["Host"] = host
		}
		node.WSOptions = &models.WSOptions{
			Path:    q.Get("path"),
			Headers: headers,
		}
	} else if node.Network == "grpc" {
		node.GRPCOptions = &models.GRPCOptions{
			ServiceName: q.Get("serviceName"),
		}
	}

	return node, nil
}

// Hysteria2: hysteria2://password@host:port?sni=...&insecure=...&obfs=...&obfs-password=...#name
func parseHysteria2(raw string) (*models.Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	port, _ := strconv.Atoi(u.Port())
	name := parseFragmentName(u, u.Host)

	q := u.Query()
	sni := q.Get("sni")
	if sni == "" {
		sni = u.Hostname()
	}

	up, _ := strconv.Atoi(q.Get("up"))
	down, _ := strconv.Atoi(q.Get("down"))

	alpnList := []string{"h3"}
	if alpn := q.Get("alpn"); alpn != "" {
		alpnList = strings.Split(alpn, ",")
	}

	return &models.Node{
		Type:            models.TypeHysteria2,
		Name:            name,
		Server:          u.Hostname(),
		Port:            port,
		Password:        safeUsername(u),
		TLS:             true,
		SNI:             sni,
		Insecure:        q.Get("insecure") == "1" || q.Get("allowInsecure") == "1",
		ALPN:            alpnList,
		Hy2UpMbps:       up,
		Hy2DownMbps:     down,
		Hy2Obfs:         q.Get("obfs"),
		Hy2ObfsPassword: q.Get("obfs-password"),
		Hy2Ports:        q.Get("mport"),
	}, nil
}

// TUIC: tuic://uuid:password@host:port?congestion_control=...&alpn=...&sni=...#name
func parseTUIC(raw string) (*models.Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	port, _ := strconv.Atoi(u.Port())
	name := parseFragmentName(u, u.Host)

	q := u.Query()
	alpnList := []string{"h3"}
	if alpn := q.Get("alpn"); alpn != "" {
		alpnList = strings.Split(alpn, ",")
	}

	return &models.Node{
		Type:           models.TypeTUIC,
		Name:           name,
		Server:         u.Hostname(),
		Port:           port,
		UUID:           safeUsername(u),
		Password:       safePassword(u),
		TLS:            true,
		SNI:            q.Get("sni"),
		ALPN:           alpnList,
		Insecure:       q.Get("allow_insecure") == "1" || q.Get("insecure") == "1",
		TuicCongestion: q.Get("congestion_control"),
		TuicUDPDrastic: q.Get("udp_relay_mode") == "quic",
		TuicZeroRTT:    q.Get("zero_rtt_handshake") == "1",
	}, nil
}

// WireGuard: wireguard://private_key@host:port?public_key=...&ip=...&mtu=...#name
func parseWireGuard(raw string) (*models.Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	port, _ := strconv.Atoi(u.Port())
	name := parseFragmentName(u, u.Host)

	q := u.Query()
	mtu, _ := strconv.Atoi(q.Get("mtu"))
	if mtu == 0 {
		mtu = 1420
	}

	localAddr := q.Get("ip")
	if localAddr == "" {
		localAddr = q.Get("address")
	}

	var reserved []int
	if resStr := q.Get("reserved"); resStr != "" {
		for _, s := range strings.Split(resStr, ",") {
			if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				reserved = append(reserved, v)
			}
		}
	}

	return &models.Node{
		Type:         models.TypeWireGuard,
		Name:         name,
		Server:       u.Hostname(),
		Port:         port,
		WGPrivateKey: safeUsername(u),
		WGPublicKey:  q.Get("public_key"),
		WGLocalAddr:  localAddr,
		WGMTU:        mtu,
		WGReserved:   reserved,
	}, nil
}

// Snell: snell://server:port?psk=...&version=...#name or snell://password@server:port?version=...#name
func parseSnell(raw string) (*models.Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	port, _ := strconv.Atoi(u.Port())
	name := parseFragmentName(u, u.Host)

	q := u.Query()
	psk := q.Get("psk")
	if psk == "" {
		psk = safeUsername(u)
	}

	ver, _ := strconv.Atoi(q.Get("version"))
	if ver == 0 {
		ver = 4
	}

	return &models.Node{
		Type:         models.TypeSnell,
		Name:         name,
		Server:       u.Hostname(),
		Port:         port,
		SnellPSK:     psk,
		SnellVersion: ver,
	}, nil
}

// AnyTLS: anytls://password@host:port?sni=...&fp=...&insecure=...&alpn=...#name
func parseAnyTLS(raw string) (*models.Node, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	port, _ := strconv.Atoi(u.Port())
	name := parseFragmentName(u, u.Host)

	q := u.Query()
	sni := q.Get("sni")
	if sni == "" {
		sni = u.Hostname()
	}

	var alpnList []string
	if alpn := q.Get("alpn"); alpn != "" {
		alpnList = strings.Split(alpn, ",")
	}

	return &models.Node{
		Type:        models.TypeAnyTLS,
		Name:        name,
		Server:      u.Hostname(),
		Port:        port,
		Password:    safeUsername(u),
		TLS:         true,
		SNI:         sni,
		Fingerprint: q.Get("fp"),
		Insecure:    q.Get("insecure") == "1" || q.Get("allowInsecure") == "1",
		ALPN:        alpnList,
	}, nil
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		i, _ := strconv.Atoi(val)
		return i
	default:
		return 0
	}
}
