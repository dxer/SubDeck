package generator

import (
	"strings"

	"subdeck/internal/regions"
)

const (
	RulePresetCNDirect    = "cn_direct"
	RulePresetAIStreaming = "ai_streaming"
	RulePresetGlobal      = "global"
)

type ExportOptions struct {
	RegionGroups  bool
	RulePreset    string
	RulesetMirror string
	// ClashTemplate / SingboxTemplate are optional parsed user overlays applied
	// to full-config output right before serialization. Provider, base64 and
	// plain-JSON outputs never apply templates.
	ClashTemplate   map[string]interface{}
	SingboxTemplate map[string]interface{}
}

func NormalizeRulePreset(preset string) string {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case RulePresetAIStreaming:
		return RulePresetAIStreaming
	case RulePresetGlobal:
		return RulePresetGlobal
	default:
		return RulePresetCNDirect
	}
}

// ApplyRulesetMirror prepends a configured proxy/mirror prefix (e.g. "https://ghproxy.net/") to GitHub raw URLs.
func ApplyRulesetMirror(rawURL, mirror string) string {
	m := strings.TrimSpace(mirror)
	if m == "" || strings.HasPrefix(rawURL, m) {
		return rawURL
	}
	if !strings.HasSuffix(m, "/") {
		m += "/"
	}
	return m + rawURL
}

var aiDomainSuffixes = []string{
	"openai.com",
	"chatgpt.com",
	"oaistatic.com",
	"oaiusercontent.com",
	"anthropic.com",
	"claude.ai",
	"gemini.google.com",
	"generativelanguage.googleapis.com",
	"ai.google.dev",
	"aistudio.google.com",
	"copilot.microsoft.com",
	"perplexity.ai",
	"groq.com",
}

var streamingDomainSuffixes = []string{
	"youtube.com",
	"youtu.be",
	"googlevideo.com",
	"ytimg.com",
	"netflix.com",
	"netflix.net",
	"nflxvideo.net",
	"nflximg.net",
	"disneyplus.com",
	"dssott.com",
	"bamgrid.com",
	"tiktokv.com",
	"tiktokcdn.com",
	"spotify.com",
	"scdn.co",
	"twitch.tv",
}

var telegramDomainSuffixes = []string{
	"t.me",
	"tdesktop.com",
	"telegra.ph",
	"telegram.me",
	"telegram.org",
	"telesco.pe",
}

var telegramIPCIDRs = []string{
	"91.108.4.0/22",
	"91.108.8.0/22",
	"91.108.12.0/22",
	"91.108.16.0/22",
	"91.108.56.0/22",
	"149.154.160.0/20",
}

type RegionGroup struct {
	Name  string
	Nodes []string
}

// BuildRegionGroups groups node names by region, returning only non-empty groups.
// Classification delegates to the shared regions package; group order and names
// match the legacy implementation.
func BuildRegionGroups(nodeNames []string) []RegionGroup {
	buckets := make([][]string, len(regions.Rules))
	for _, name := range nodeNames {
		if idx := regions.MatchIndex(name); idx >= 0 {
			buckets[idx] = append(buckets[idx], name)
		}
	}

	var groups []RegionGroup
	for idx, list := range buckets {
		if len(list) > 0 {
			groups = append(groups, RegionGroup{
				Name:  regions.Rules[idx].Name,
				Nodes: list,
			})
		}
	}
	return groups
}
