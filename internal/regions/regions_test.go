package regions

import "testing"

func TestMatchIndexFirstMatchWins(t *testing.T) {
	cases := map[string]int{
		"香港 01":       0,
		"HK-Node":     0,
		"🇹🇼 TW 02":    1,
		"JP Tokyo":    2,
		"新加坡 狮城":      3,
		"US-West":     4,
		"美国 01":       4,
		"Korea KR":    5,
		"unclassified": -1,
	}
	for name, want := range cases {
		if got := MatchIndex(name); got != want {
			t.Fatalf("MatchIndex(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestMatchAnyCodesAndKeywords(t *testing.T) {
	if !MatchAny("香港 IEPL-01", []string{"HK", "US"}) {
		t.Fatalf("code selector should match")
	}
	if !MatchAny("美国 02", []string{"US"}) {
		t.Fatalf("US code should match 美国")
	}
	if MatchAny("香港 01", []string{"US", "SG"}) {
		t.Fatalf("non-matching selector must not match")
	}
	if !MatchAny("vip节点", []string{"vip"}) {
		t.Fatalf("raw keyword selector should match")
	}
	if MatchAny("anything", []string{"", "  "}) {
		t.Fatalf("blank selectors must never match")
	}
}
