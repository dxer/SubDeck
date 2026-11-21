package pipeline

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"subdeck/internal/models"
	"subdeck/internal/regions"
)

var infoNodePattern = regexp.MustCompile(`(?i)(剩余流量|套餐到期|距离下次重置|下次重置|流量重置|过期时间|到期时间|官网|最新域名|最新网址|备用网址|发布页|防失联|订阅地址|加入电报|Telegram群|TG群|客服|通知|公告|Expire|Traffic|Reset|Website|Domain)`)

// ValidateProfileRegexes verifies that all regex rules on a Profile compile cleanly.
func ValidateProfileRegexes(p *models.Profile) error {
	if p == nil {
		return nil
	}
	checks := []struct {
		label   string
		pattern string
	}{
		{"包含正则", p.IncludeRegex},
		{"排除正则", p.ExcludeRegex},
		{"重命名匹配正则", p.RenameRegexSearch},
	}
	for _, c := range checks {
		if strings.TrimSpace(c.pattern) != "" {
			if _, err := regexp.Compile(c.pattern); err != nil {
				return fmt.Errorf("%s语法无效: %v", c.label, err)
			}
		}
	}
	for i, pattern := range p.RegexSort {
		if strings.TrimSpace(pattern) != "" {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("正则排序第 %d 条语法无效: %v", i+1, err)
			}
		}
	}
	return nil
}

// ProcessNodes applies combination profile rules to the raw aggregated node list
func ProcessNodes(nodes []*models.Node, profile *models.Profile) []*models.Node {
	if len(nodes) == 0 {
		return nil
	}

	// Drop structurally invalid entries early
	var valid []*models.Node
	for _, n := range nodes {
		if n != nil && strings.TrimSpace(n.Server) != "" && n.Port > 0 {
			valid = append(valid, n)
		}
	}
	if len(valid) == 0 {
		return nil
	}

	// 0a. Optional info/status dummy node filtering
	if profile.FilterInfoNodes {
		valid = filterInfoNodes(valid)
	}

	// 0b. Optional health filter (keeps untested latency=0 and reachable latency>0; excludes probed dead latency<0)
	if profile.OnlyHealthy {
		valid = filterHealthyNodes(valid)
	}

	// 1. Protocol filtering
	filtered := filterProtocols(valid, profile.AllowedProtocols)

	// 2. Precise Deduplication
	if profile.Deduplicate {
		filtered = deduplicateNodes(filtered)
	}

	// 3. Include Regex filtering
	if strings.TrimSpace(profile.IncludeRegex) != "" {
		if re, err := regexp.Compile(profile.IncludeRegex); err == nil {
			var kept []*models.Node
			for _, n := range filtered {
				if re.MatchString(n.Name) {
					kept = append(kept, n)
				}
			}
			filtered = kept
		}
	}

	// 4. Exclude Regex filtering
	if strings.TrimSpace(profile.ExcludeRegex) != "" {
		if re, err := regexp.Compile(profile.ExcludeRegex); err == nil {
			var kept []*models.Node
			for _, n := range filtered {
				if !re.MatchString(n.Name) {
					kept = append(kept, n)
				}
			}
			filtered = kept
		}
	}

	// 4b. Region / flag keyword filtering (matches original cached names, before rename)
	if len(profile.RegionInclude) > 0 {
		var kept []*models.Node
		for _, n := range filtered {
			if regions.MatchAny(n.Name, profile.RegionInclude) {
				kept = append(kept, n)
			}
		}
		filtered = kept
	}
	if len(profile.RegionExclude) > 0 {
		var kept []*models.Node
		for _, n := range filtered {
			if !regions.MatchAny(n.Name, profile.RegionExclude) {
				kept = append(kept, n)
			}
		}
		filtered = kept
	}

	// 5. Renaming (Regex search & replace, Prefix, Suffix)
	var reSearch *regexp.Regexp
	if strings.TrimSpace(profile.RenameRegexSearch) != "" {
		reSearch, _ = regexp.Compile(profile.RenameRegexSearch)
	}

	for _, n := range filtered {
		name := n.Name
		if reSearch != nil {
			name = reSearch.ReplaceAllString(name, profile.RenameRegexReplace)
		}
		if profile.RenamePrefix != "" {
			name = profile.RenamePrefix + name
		}
		if profile.RenameSuffix != "" {
			name = name + profile.RenameSuffix
		}
		n.Name = strings.TrimSpace(name)
	}

	// 5b. Optional profile-level attribute overrides. Applied after deduplication
	// (SNI participates in the dedup fingerprint) and renaming; only present
	// values are touched so unset overrides never mutate nodes.
	applyOverrides(filtered, profile)

	// 6. Optional Sorting
	sortNodes(filtered, profile)

	// 7. Ensure all node names are unique (Clash & Singbox require unique tags)
	filtered = ensureUniqueNames(filtered)

	return filtered
}

// applyOverrides applies the optional bulk attribute overrides (SNI, skip-cert-verify,
// UDP relay flag) to cloned pipeline nodes. A present but empty SNI string clears SNI.
func applyOverrides(nodes []*models.Node, profile *models.Profile) {
	for _, n := range nodes {
		if profile.OverrideSNI != nil {
			n.SNI = strings.TrimSpace(*profile.OverrideSNI)
		}
		if profile.OverrideInsecure != nil {
			n.Insecure = *profile.OverrideInsecure
		}
		if profile.OverrideUDP != nil {
			udp := *profile.OverrideUDP
			n.UDP = &udp
		}
	}
}

func filterInfoNodes(nodes []*models.Node) []*models.Node {
	var result []*models.Node
	for _, n := range nodes {
		if !infoNodePattern.MatchString(n.Name) {
			result = append(result, n)
		}
	}
	return result
}

func filterHealthyNodes(nodes []*models.Node) []*models.Node {
	var result []*models.Node
	for _, n := range nodes {
		if n.LatencyMs >= 0 {
			result = append(result, n)
		}
	}
	return result
}

func filterProtocols(nodes []*models.Node, allowed []string) []*models.Node {
	if len(allowed) == 0 {
		return nodes
	}
	allowedMap := make(map[string]bool)
	for _, p := range allowed {
		allowedMap[strings.ToLower(strings.TrimSpace(p))] = true
	}

	var result []*models.Node
	for _, n := range nodes {
		if allowedMap[strings.ToLower(string(n.Type))] {
			result = append(result, n)
		}
	}
	return result
}

func nodeFingerprint(n *models.Node) string {
	cred := n.UUID
	if cred == "" {
		cred = n.Password
	}
	if cred == "" {
		cred = n.SnellPSK
	}
	if cred == "" {
		cred = n.WGPrivateKey
	}

	var path string
	if n.WSOptions != nil {
		path = n.WSOptions.Path
	} else if n.GRPCOptions != nil {
		path = n.GRPCOptions.ServiceName
	}

	return fmt.Sprintf("%s|%s|%d|%s|%s|%s",
		strings.ToLower(string(n.Type)),
		strings.ToLower(strings.TrimSpace(n.Server)),
		n.Port,
		cred,
		strings.ToLower(strings.TrimSpace(n.SNI)),
		path,
	)
}

func deduplicateNodes(nodes []*models.Node) []*models.Node {
	seen := make(map[string]bool, len(nodes))
	var result []*models.Node

	for _, n := range nodes {
		key := nodeFingerprint(n)
		if !seen[key] {
			seen[key] = true
			result = append(result, n)
		}
	}
	return result
}

func regionWeight(name string) int {
	upper := strings.ToUpper(name)
	switch {
	case strings.Contains(name, "香港") || strings.Contains(upper, "HK") || strings.Contains(upper, "HONG KONG") || strings.Contains(name, "🇭🇰"):
		return 1
	case strings.Contains(name, "台湾") || strings.Contains(name, "台灣") || strings.Contains(upper, "TW") || strings.Contains(upper, "TAIWAN") || strings.Contains(name, "🇹🇼"):
		return 2
	case strings.Contains(name, "日本") || strings.Contains(upper, "JP") || strings.Contains(upper, "JAPAN") || strings.Contains(name, "🇯🇵"):
		return 3
	case strings.Contains(name, "新加坡") || strings.Contains(name, "狮城") || strings.Contains(upper, "SG") || strings.Contains(upper, "SINGAPORE") || strings.Contains(name, "🇸🇬"):
		return 4
	case strings.Contains(name, "美国") || strings.Contains(name, "美國") || strings.Contains(upper, "US") || strings.Contains(upper, "USA") || strings.Contains(name, "🇺🇸"):
		return 5
	case strings.Contains(name, "韩国") || strings.Contains(name, "韓國") || strings.Contains(upper, "KR") || strings.Contains(upper, "KOREA") || strings.Contains(name, "🇰🇷"):
		return 6
	case strings.Contains(name, "英国") || strings.Contains(name, "德国") || strings.Contains(name, "法国") || strings.Contains(upper, "UK") || strings.Contains(upper, "DE") || strings.Contains(upper, "FR"):
		return 7
	default:
		return 99
	}
}

func sortNodes(nodes []*models.Node, profile *models.Profile) {
	switch strings.ToLower(strings.TrimSpace(profile.SortMode)) {
	case "region":
		sort.SliceStable(nodes, func(i, j int) bool {
			wi, wj := regionWeight(nodes[i].Name), regionWeight(nodes[j].Name)
			if wi != wj {
				return wi < wj
			}
			return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
		})
	case "name":
		sort.SliceStable(nodes, func(i, j int) bool {
			return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
		})
	case "regex":
		sortNodesByRegex(nodes, profile.RegexSort)
	}
}

// sortNodesByRegex orders nodes by the first matching pattern in the ordered list.
// Earlier patterns rank higher; unmatched nodes sort after every matched group.
// Ties keep the incoming (stable) order. Patterns are compiled once per call.
func sortNodesByRegex(nodes []*models.Node, patterns []string) {
	var compiled []*regexp.Regexp
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		if re, err := regexp.Compile(pattern); err == nil {
			compiled = append(compiled, re)
		}
	}
	if len(compiled) == 0 {
		return
	}
	type ranked struct {
		node *models.Node
		rank int
	}
	list := make([]ranked, len(nodes))
	for i, n := range nodes {
		r := len(compiled)
		for idx, re := range compiled {
			if re.MatchString(n.Name) {
				r = idx
				break
			}
		}
		list[i] = ranked{node: n, rank: r}
	}
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].rank < list[j].rank
	})
	for i := range list {
		nodes[i] = list[i].node
	}
}

func ensureUniqueNames(nodes []*models.Node) []*models.Node {
	used := make(map[string]bool, len(nodes))
	nextIdx := make(map[string]int, len(nodes))

	for _, n := range nodes {
		base := n.Name
		if base == "" {
			base = fmt.Sprintf("%s-%s", n.Type, n.Server)
		}
		if !used[base] {
			used[base] = true
			n.Name = base
			continue
		}

		idx := nextIdx[base]
		if idx == 0 {
			idx = 1
		}
		for {
			candidate := fmt.Sprintf("%s (%d)", base, idx)
			idx++
			if !used[candidate] {
				used[candidate] = true
				nextIdx[base] = idx
				n.Name = candidate
				break
			}
		}
	}
	return nodes
}
