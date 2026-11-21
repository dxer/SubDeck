// Package regions holds the shared region classification rules used by both the
// pipeline (region filters) and the generator (regional url-test groups).
// Matching semantics are deliberately simple and identical to the legacy
// implementation: case-insensitive substring matching on the node name.
package regions

import "strings"

// Rule describes one region bucket. Code is the canonical selector (HK, TW...);
// Keywords additionally cover Chinese names and flag emoji.
type Rule struct {
	Code     string
	Name     string
	Keywords []string
}

var Rules = []Rule{
	{Code: "HK", Name: "🇭🇰 香港节点", Keywords: []string{"香港", "HK", "HONG KONG", "🇭🇰"}},
	{Code: "TW", Name: "🇹🇼 台湾节点", Keywords: []string{"台湾", "台灣", "TW", "TAIWAN", "🇹🇼"}},
	{Code: "JP", Name: "🇯🇵 日本节点", Keywords: []string{"日本", "JP", "JAPAN", "🇯🇵"}},
	{Code: "SG", Name: "🇸🇬 新加坡节点", Keywords: []string{"新加坡", "狮城", "SG", "SINGAPORE", "🇸🇬"}},
	{Code: "US", Name: "🇺🇸 美国节点", Keywords: []string{"美国", "美國", "US", "USA", "🇺🇸"}},
	{Code: "KR", Name: "🇰🇷 韩国节点", Keywords: []string{"韩国", "韓國", "KR", "KOREA", "🇰🇷"}},
}

// MatchIndex returns the index of the first rule matching the name, or -1.
// First match wins; nodes matching no rule are unclassified.
func MatchIndex(name string) int {
	upper := strings.ToUpper(name)
	for idx, rule := range Rules {
		for _, kw := range rule.Keywords {
			if strings.Contains(name, kw) || strings.Contains(upper, kw) {
				return idx
			}
		}
	}
	return -1
}

func codeIndex(token string) (int, bool) {
	for idx, rule := range Rules {
		if token == rule.Code {
			return idx, true
		}
	}
	return -1, false
}

// MatchAny reports whether the name matches any selector. A selector is either a
// canonical region code (HK, TW...) or a raw keyword/emoji substring.
func MatchAny(name string, selectors []string) bool {
	if len(selectors) == 0 {
		return false
	}
	matched := MatchIndex(name)
	for _, sel := range selectors {
		trimmed := strings.TrimSpace(sel)
		token := strings.ToUpper(trimmed)
		if token == "" {
			continue
		}
		if idx, isCode := codeIndex(token); isCode {
			if matched == idx {
				return true
			}
			continue
		}
		if strings.Contains(name, trimmed) || strings.Contains(strings.ToUpper(name), token) {
			return true
		}
	}
	return false
}

// SupportedCodes lists the canonical region codes in rule order.
func SupportedCodes() []string {
	codes := make([]string, len(Rules))
	for i, r := range Rules {
		codes[i] = r.Code
	}
	return codes
}
