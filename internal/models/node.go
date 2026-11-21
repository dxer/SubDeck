package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

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
	SelectionKey string `json:"selection_key,omitempty"`
	UDP *bool `json:"udp,omitempty"`
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

	RawURI     string     `json:"raw_uri,omitempty"`
	Source     string     `json:"source,omitempty"` // Subscription name
	LatencyMs  int        `json:"latency_ms,omitempty"` // 0: untested, >0: TCP RTT ms, -1: unreachable
	LastProbed *time.Time `json:"last_probed,omitempty"`
}

// Hy2ObfsEnabled reports whether the Hysteria2 node carries a usable obfs layer.
// Some providers emit `obfs=none` to mean "no obfs"; both mihomo and sing-box
// reject an obfs type without a password, so treat "none" or missing password as absent.
func (n *Node) Hy2ObfsEnabled() bool {
	return n.Hy2Obfs != "" && !strings.EqualFold(n.Hy2Obfs, "none") && strings.TrimSpace(n.Hy2ObfsPassword) != ""
}

// StableSelectionKey returns a deterministic key for selecting a specific cached node.
// It intentionally excludes the display name so renames do not invalidate a selection.
func (n *Node) StableSelectionKey() string {
	if n == nil {
		return ""
	}
	identity := struct {
		Type string `json:"type"`
		Server string `json:"server"`
		Port int `json:"port"`
		Password string `json:"password"`
		UUID string `json:"uuid"`
		Cipher string `json:"cipher"`
		Network string `json:"network"`
		SNI string `json:"sni"`
		Flow string `json:"flow"`
		Reality *Reality `json:"reality,omitempty"`
		WS *WSOptions `json:"ws,omitempty"`
		GRPC *GRPCOptions `json:"grpc,omitempty"`
	}{string(n.Type), strings.ToLower(strings.TrimSpace(n.Server)), n.Port, n.Password, n.UUID, n.Cipher, n.Network, strings.ToLower(strings.TrimSpace(n.SNI)), n.Flow, n.Reality, n.WSOptions, n.GRPCOptions}
	data, _ := json.Marshal(identity)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:16])
}

func (n *Node) EnsureSelectionKey() {
	if n != nil && n.SelectionKey == "" {
		n.SelectionKey = n.StableSelectionKey()
	}
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

type Subscription struct {
	ID                 string           `json:"id"`
	Name               string           `json:"name"`
	Type               SubscriptionType `json:"type"`          // "remote" or "local"
	URL                string           `json:"url,omitempty"` // legacy first URL
	URLs               []string         `json:"urls,omitempty"`
	FetchProxyURL      string           `json:"fetch_proxy_url,omitempty"`
	FailurePolicy      string           `json:"failure_policy,omitempty"` // "strict" or "skip"
	RawContent         string           `json:"raw_content,omitempty"` // for local (raw URI text)
	UserAgent          string           `json:"user_agent,omitempty"`
	InsecureSkipVerify bool             `json:"insecure_skip_verify"`
	AutoUpdate         bool             `json:"auto_update"`
	IntervalHours      int              `json:"interval_hours"` // e.g. 12
	NodeCount          int              `json:"node_count"`
	UserInfo           string           `json:"user_info,omitempty"`
	LastUpdated         time.Time  `json:"last_updated"`
	LastAttemptAt       time.Time  `json:"last_attempt_at"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	Status              string     `json:"status"` // "ok", "error", "pending"
	ErrorMsg            string     `json:"error_msg,omitempty"`
	CachedNodes         []*Node    `json:"cached_nodes,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// Profile represents a combination profile that aggregates multiple Subscriptions
type Profile struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Token           string   `json:"token"` // secure access token
	SubscriptionIDs []string `json:"subscription_ids"` // IDs of aggregated Subscriptions (both local and remote)
	// SelectedNodes restricts which cached nodes of a subscription join the profile,
	// keyed by subscription ID holding the selected node names.
	// A missing or empty entry keeps the current behavior: aggregate every node of that subscription.
	SelectedNodes map[string][]string `json:"selected_nodes,omitempty"`
	SelectedNodeKeys map[string][]string `json:"selected_node_keys,omitempty"`
	SubscriptionSelectionMode string `json:"subscription_selection_mode,omitempty"` // "all" (legacy) or "explicit"
	RegionInclude []string `json:"region_include,omitempty"`
	RegionExclude []string `json:"region_exclude,omitempty"`
	RegexSort []string `json:"regex_sort,omitempty"`
	OverrideSNI *string `json:"override_sni,omitempty"`
	OverrideInsecure *bool `json:"override_insecure,omitempty"`
	OverrideUDP *bool `json:"override_udp,omitempty"`
	ClashTemplate string `json:"clash_template,omitempty"`
	SingboxTemplate string `json:"singbox_template,omitempty"`
	ClashTemplateURL string `json:"clash_template_url,omitempty"`
	SingboxTemplateURL string `json:"singbox_template_url,omitempty"`
	ClashTemplateCache string `json:"-"`
	SingboxTemplateCache string `json:"-"`
	TemplateStatus string `json:"template_status,omitempty"`

	// Filter and processing rules
	Deduplicate        bool     `json:"deduplicate"`
	FilterInfoNodes    bool     `json:"filter_info_nodes"`
	OnlyHealthy        bool     `json:"only_healthy"`
	IncludeRegex       string   `json:"include_regex,omitempty"`
	ExcludeRegex       string   `json:"exclude_regex,omitempty"`
	AllowedProtocols   []string `json:"allowed_protocols,omitempty"` // empty = all
	SortMode           string   `json:"sort_mode,omitempty"`         // "", "region", "name"
	RegionGroups       bool     `json:"region_groups"`               // auto-generate regional url-test groups
	RulePreset         string   `json:"rule_preset,omitempty"`       // "", "cn_direct", "ai_streaming", "global"
	RenamePrefix       string   `json:"rename_prefix,omitempty"`
	RenameSuffix       string   `json:"rename_suffix,omitempty"`
	RenameRegexSearch  string   `json:"rename_regex_search,omitempty"`
	RenameRegexReplace string   `json:"rename_regex_replace,omitempty"`

	// Export settings & telemetry
	FullConfigDefault bool       `json:"full_config_default"`
	ExportMode        string     `json:"export_mode,omitempty"` // "", "full", "provider"
	LastPullAt        *time.Time `json:"last_pull_at,omitempty"`
	LastPullClient    string     `json:"last_pull_client,omitempty"`
	LastPullIP        string     `json:"last_pull_ip,omitempty"`
	PullCount         int        `json:"pull_count"`

	NodeCount int       `json:"node_count,omitempty"` // dynamic computed count
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProfileFeatures is the persisted representation of the newer optional profile
// settings (selection mode, region filters, regex sort, node overrides, templates).
// It is stored as a single JSON column so additive settings do not require one
// SQLite column each. Field JSON tags match the flat Profile API contract.
type ProfileFeatures struct {
	SubscriptionSelectionMode string              `json:"subscription_selection_mode,omitempty"` // "" / "all" legacy, "explicit"
	SelectedNodeKeys          map[string][]string `json:"selected_node_keys,omitempty"`
	RegionInclude             []string            `json:"region_include,omitempty"`
	RegionExclude             []string            `json:"region_exclude,omitempty"`
	RegexSort                 []string            `json:"regex_sort,omitempty"`
	OverrideSNI               *string             `json:"override_sni,omitempty"`
	OverrideInsecure          *bool               `json:"override_insecure,omitempty"`
	OverrideUDP               *bool               `json:"override_udp,omitempty"`
	ClashTemplate             string              `json:"clash_template,omitempty"`
	SingboxTemplate           string              `json:"singbox_template,omitempty"`
	ClashTemplateURL          string              `json:"clash_template_url,omitempty"`
	SingboxTemplateURL        string              `json:"singbox_template_url,omitempty"`
	ClashTemplateCache        string              `json:"clash_template_cache,omitempty"`
	SingboxTemplateCache      string              `json:"singbox_template_cache,omitempty"`
	TemplateStatus            string              `json:"template_status,omitempty"`
}

func (p *Profile) GetFeatures() ProfileFeatures {
	if p == nil {
		return ProfileFeatures{}
	}
	return ProfileFeatures{
		SubscriptionSelectionMode: p.SubscriptionSelectionMode,
		SelectedNodeKeys:          p.SelectedNodeKeys,
		RegionInclude:             p.RegionInclude,
		RegionExclude:             p.RegionExclude,
		RegexSort:                 p.RegexSort,
		OverrideSNI:               p.OverrideSNI,
		OverrideInsecure:          p.OverrideInsecure,
		OverrideUDP:               p.OverrideUDP,
		ClashTemplate:             p.ClashTemplate,
		SingboxTemplate:           p.SingboxTemplate,
		ClashTemplateURL:          p.ClashTemplateURL,
		SingboxTemplateURL:        p.SingboxTemplateURL,
		ClashTemplateCache:        p.ClashTemplateCache,
		SingboxTemplateCache:      p.SingboxTemplateCache,
		TemplateStatus:            p.TemplateStatus,
	}
}

func (p *Profile) SetFeatures(f ProfileFeatures) {
	if p == nil {
		return
	}
	p.SubscriptionSelectionMode = f.SubscriptionSelectionMode
	p.SelectedNodeKeys = f.SelectedNodeKeys
	p.RegionInclude = f.RegionInclude
	p.RegionExclude = f.RegionExclude
	p.RegexSort = f.RegexSort
	p.OverrideSNI = f.OverrideSNI
	p.OverrideInsecure = f.OverrideInsecure
	p.OverrideUDP = f.OverrideUDP
	p.ClashTemplate = f.ClashTemplate
	p.SingboxTemplate = f.SingboxTemplate
	p.ClashTemplateURL = f.ClashTemplateURL
	p.SingboxTemplateURL = f.SingboxTemplateURL
	p.ClashTemplateCache = f.ClashTemplateCache
	p.SingboxTemplateCache = f.SingboxTemplateCache
	p.TemplateStatus = f.TemplateStatus
}

// EffectiveURLs returns the ordered remote URL list. Legacy rows store a single
// URL in the url column; URLs is the additive multi-URL list.
func (s *Subscription) EffectiveURLs() []string {
	if s == nil {
		return nil
	}
	if len(s.URLs) > 0 {
		return s.URLs
	}
	if strings.TrimSpace(s.URL) != "" {
		return []string{s.URL}
	}
	return nil
}

const (
	FetchFailureStrict = "strict"
	FetchFailureSkip   = "skip"
)

// SystemSettings stores runtime-configurable system preferences in SQLite.
type SystemSettings struct {
	WebhookURL        string `json:"webhook_url"`
	RulesetMirror     string `json:"ruleset_mirror"`
	AllowPrivateFetch bool   `json:"allow_private_fetch"`
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
