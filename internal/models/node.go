package models

import "time"

// ProxyType represents the proxy protocol type
type ProxyType string

const (
	TypeShadowsocks ProxyType = "ss"
	TypeVMess       ProxyType = "vmess"
	TypeVLESS       ProxyType = "vless"
	TypeTrojan      ProxyType = "trojan"
	TypeHysteria2   ProxyType = "hysteria2"
	TypeAnyTLS      ProxyType = "anytls"
	TypeTUIC        ProxyType = "tuic"
	TypeWireGuard   ProxyType = "wireguard"
	TypeSnell       ProxyType = "snell"
)

// Node is the unified internal representation for any proxy node
type Node struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      ProxyType `json:"type"`
	Server    string    `json:"server"`
	Port      int       `json:"port"`
	Password  string    `json:"password,omitempty"`
	UUID      string    `json:"uuid,omitempty"`
	Cipher    string    `json:"cipher,omitempty"`
	AlterId   int       `json:"alter_id,omitempty"`
	Network   string    `json:"network,omitempty"` // tcp, ws, grpc, http, etc.
	TLS       bool      `json:"tls,omitempty"`
	SNI       string    `json:"sni,omitempty"`
	ALPN      []string  `json:"alpn,omitempty"`
	Insecure  bool      `json:"insecure,omitempty"`
	Flow      string    `json:"flow,omitempty"`       // xtls-rprx-vision
	Fingerprint string  `json:"fingerprint,omitempty"`// chrome, firefox, etc.
	Reality   *Reality  `json:"reality,omitempty"`

	// Transport options
	WSOptions   *WSOptions   `json:"ws_options,omitempty"`
	GRPCOptions *GRPCOptions `json:"grpc_options,omitempty"`
	HTTPOptions *HTTPOptions `json:"http_options,omitempty"`

	// Hysteria 2 specific
	Hy2UpMbps   int      `json:"hy2_up_mbps,omitempty"`
	Hy2DownMbps int      `json:"hy2_down_mbps,omitempty"`
	Hy2Obfs     string   `json:"hy2_obfs,omitempty"`
	Hy2ObfsPassword string `json:"hy2_obfs_password,omitempty"`
	Hy2Ports    string   `json:"hy2_ports,omitempty"`

	// TUIC specific
	TuicCongestion  string `json:"tuic_congestion,omitempty"`
	TuicUDPDrastic  bool   `json:"tuic_udp_drastic,omitempty"`
	TuicZeroRTT     bool   `json:"tuic_zero_rtt,omitempty"`

	// WireGuard specific
	WGPublicKey  string   `json:"wg_public_key,omitempty"`
	WGPrivateKey string   `json:"wg_private_key,omitempty"`
	WGLocalAddr  string   `json:"wg_local_addr,omitempty"`
	WGMTU        int      `json:"wg_mtu,omitempty"`
	WGReserved   []int    `json:"wg_reserved,omitempty"`

	// Snell specific
	SnellVersion int    `json:"snell_version,omitempty"`
	SnellPSK     string `json:"snell_psk,omitempty"`

	RawURI    string `json:"raw_uri,omitempty"`
	Source    string `json:"source,omitempty"` // Subscription name
}

type Reality struct {
	PublicKey string `json:"public_key"`
	ShortID   string `json:"short_id,omitempty"`
}

type WSOptions struct {
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
}

type GRPCOptions struct {
	ServiceName string `json:"service_name"`
}

type HTTPOptions struct {
	Method  string            `json:"method,omitempty"`
	Path    []string          `json:"path,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// SubscriptionType: "remote" or "local"
type SubscriptionType string

const (
	SubscriptionTypeRemote SubscriptionType = "remote"
	SubscriptionTypeLocal  SubscriptionType = "local"
)

// Subscription represents either a Remote subscription (URL) or a Local subscription (Batch Imported Nodes)
type Subscription struct {
	ID                 string           `json:"id"`
	Name               string           `json:"name"`
	Type               SubscriptionType `json:"type"`          // "remote" or "local"
	URL                string           `json:"url,omitempty"` // for remote
	RawContent         string           `json:"raw_content,omitempty"` // for local (raw URI text)
	UserAgent          string           `json:"user_agent,omitempty"`
	InsecureSkipVerify bool             `json:"insecure_skip_verify"`
	AutoUpdate         bool             `json:"auto_update"`
	IntervalHours      int              `json:"interval_hours"` // e.g. 12
	NodeCount          int              `json:"node_count"`
	UserInfo           string           `json:"user_info,omitempty"`
	LastUpdated        time.Time        `json:"last_updated"`
	Status             string           `json:"status"` // "ok", "error", "pending"
	ErrorMsg           string           `json:"error_msg,omitempty"`
	CachedNodes        []*Node          `json:"cached_nodes,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

// Profile represents a combination profile that aggregates multiple Subscriptions
type Profile struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Token           string   `json:"token"` // secure access token
	SubscriptionIDs []string `json:"subscription_ids"` // IDs of aggregated Subscriptions (both local and remote)

	// Filter and processing rules
	Deduplicate        bool     `json:"deduplicate"`
	IncludeRegex       string   `json:"include_regex,omitempty"`
	ExcludeRegex       string   `json:"exclude_regex,omitempty"`
	AllowedProtocols   []string `json:"allowed_protocols,omitempty"` // empty = all
	RenamePrefix       string   `json:"rename_prefix,omitempty"`
	RenameSuffix       string   `json:"rename_suffix,omitempty"`
	RenameRegexSearch  string   `json:"rename_regex_search,omitempty"`
	RenameRegexReplace string   `json:"rename_regex_replace,omitempty"`

	// Export settings
	FullConfigDefault bool `json:"full_config_default"` // if true, export full config by default

	NodeCount int       `json:"node_count,omitempty"` // dynamic computed count
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User represents admin auth
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// DashboardStats
type DashboardStats struct {
	TotalSubCount  int `json:"total_sub_count"`
	RemoteSubCount int `json:"remote_sub_count"`
	LocalSubCount  int `json:"local_sub_count"`
	ProfileCount   int `json:"profile_count"`
	TotalNodeCount int `json:"total_node_count"`
}
