package generator

import (
	"encoding/json"

	"subdeck/internal/models"
)

// ExportNode is the public plain-JSON representation of a processed node.
// It keeps every protocol parameter a client needs to rebuild the proxy while
// excluding internal bookkeeping fields (raw URI, stable selection key, probe
// timestamps) that should not leak through public export links.
type ExportNode struct {
	Name        string           `json:"name"`
	Type        string           `json:"type"`
	Server      string           `json:"server"`
	Port        int              `json:"port"`
	Password    string           `json:"password,omitempty"`
	UUID        string           `json:"uuid,omitempty"`
	Cipher      string           `json:"cipher,omitempty"`
	AlterId     int              `json:"alter_id,omitempty"`
	Network     string           `json:"network,omitempty"`
	TLS         bool             `json:"tls,omitempty"`
	SNI         string           `json:"sni,omitempty"`
	ALPN        []string         `json:"alpn,omitempty"`
	Insecure    bool             `json:"insecure,omitempty"`
	Flow        string           `json:"flow,omitempty"`
	Fingerprint string           `json:"fingerprint,omitempty"`
	Reality     *models.Reality  `json:"reality,omitempty"`
	WSOptions   *models.WSOptions   `json:"ws_options,omitempty"`
	GRPCOptions *models.GRPCOptions `json:"grpc_options,omitempty"`
	HTTPOptions *models.HTTPOptions `json:"http_options,omitempty"`

	Hy2UpMbps      int    `json:"hy2_up_mbps,omitempty"`
	Hy2DownMbps    int    `json:"hy2_down_mbps,omitempty"`
	Hy2Obfs        string `json:"hy2_obfs,omitempty"`
	Hy2Ports       string `json:"hy2_ports,omitempty"`
	TuicCongestion string `json:"tuic_congestion,omitempty"`

	WGPublicKey  string `json:"wg_public_key,omitempty"`
	WGLocalAddr  string `json:"wg_local_address,omitempty"`
	WGMTU        int    `json:"wg_mtu,omitempty"`
	WGReserved   []int  `json:"wg_reserved,omitempty"`
	SnellVersion int    `json:"snell_version,omitempty"`
	SnellPSK     string `json:"snell_psk,omitempty"`

	UDP       *bool  `json:"udp,omitempty"`
	Source    string `json:"source,omitempty"`
	LatencyMs int    `json:"latency_ms,omitempty"`
}

func toExportNode(n *models.Node) *ExportNode {
	if n == nil {
		return nil
	}
	return &ExportNode{
		Name:        n.Name,
		Type:        string(n.Type),
		Server:      n.Server,
		Port:        n.Port,
		Password:    n.Password,
		UUID:        n.UUID,
		Cipher:      n.Cipher,
		AlterId:     n.AlterId,
		Network:     n.Network,
		TLS:         n.TLS,
		SNI:         n.SNI,
		ALPN:        n.ALPN,
		Insecure:    n.Insecure,
		Flow:        n.Flow,
		Fingerprint: n.Fingerprint,
		Reality:     n.Reality,
		WSOptions:   n.WSOptions,
		GRPCOptions: n.GRPCOptions,
		HTTPOptions: n.HTTPOptions,

		Hy2UpMbps:      n.Hy2UpMbps,
		Hy2DownMbps:    n.Hy2DownMbps,
		Hy2Obfs:        n.Hy2Obfs,
		Hy2Ports:       n.Hy2Ports,
		TuicCongestion: n.TuicCongestion,

		WGPublicKey:  n.WGPublicKey,
		WGLocalAddr:  n.WGLocalAddr,
		WGMTU:        n.WGMTU,
		WGReserved:   n.WGReserved,
		SnellVersion: n.SnellVersion,
		SnellPSK:     n.SnellPSK,

		UDP:       n.UDP,
		Source:    n.Source,
		LatencyMs: n.LatencyMs,
	}
}

// GenerateNodesJSON serializes the processed node list as a plain JSON array.
// It is a debugging/scripting format independent of the full/provider modes.
func GenerateNodesJSON(nodes []*models.Node) ([]byte, error) {
	out := make([]*ExportNode, 0, len(nodes))
	for _, n := range nodes {
		if e := toExportNode(n); e != nil {
			out = append(out, e)
		}
	}
	return json.MarshalIndent(out, "", "  ")
}
