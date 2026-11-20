package pipeline

import (
	"fmt"
	"regexp"
	"strings"

	"subdeck/internal/models"
)

// ProcessNodes applies combination profile rules to the raw aggregated node list
func ProcessNodes(nodes []*models.Node, profile *models.Profile) []*models.Node {
	if len(nodes) == 0 {
		return nil
	}

	// 1. Protocol filtering
	filtered := filterProtocols(nodes, profile.AllowedProtocols)

	// 2. Deduplication
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

	// 6. Ensure all node names are unique (Clash & Singbox require unique tags)
	filtered = ensureUniqueNames(filtered)

	return filtered
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

func deduplicateNodes(nodes []*models.Node) []*models.Node {
	seen := make(map[string]bool)
	var result []*models.Node

	for _, n := range nodes {
		key := fmt.Sprintf("%s:%d:%s", strings.ToLower(n.Server), n.Port, n.Type)
		if !seen[key] {
			seen[key] = true
			result = append(result, n)
		}
	}
	return result
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
