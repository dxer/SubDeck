package pipeline

import (
	"testing"

	"subdeck/internal/models"
)

func TestPipeline(t *testing.T) {
	nodes := []*models.Node{
		{Name: "香港 01", Type: models.TypeShadowsocks, Server: "1.1.1.1", Port: 8388},
		{Name: "香港 01 备用", Type: models.TypeShadowsocks, Server: "1.1.1.1", Port: 8388}, // duplicate server+port+type
		{Name: "日本 01", Type: models.TypeVMess, Server: "2.2.2.2", Port: 443},
		{Name: "官网通知 - 到期时间", Type: models.TypeTrojan, Server: "3.3.3.3", Port: 443},
		{Name: "美国 01", Type: models.TypeHysteria2, Server: "4.4.4.4", Port: 443},
	}

	profile := &models.Profile{
		Deduplicate:  true,
		ExcludeRegex: "官网|到期",
		IncludeRegex: "香港|日本|美国",
		AllowedProtocols: []string{"ss", "vmess", "hysteria2"},
		RenamePrefix: "[SubLite] ",
	}

	processed := ProcessNodes(nodes, profile)

	if len(processed) != 3 {
		t.Fatalf("Expected 3 nodes after pipeline, got %d", len(processed))
	}

	for _, n := range processed {
		if n.Type == models.TypeTrojan {
			t.Errorf("Trojan should have been excluded by protocol filter")
		}
		if n.Name == "官网通知 - 到期时间" {
			t.Errorf("Notice node should have been excluded by regex")
		}
		if n.Name[:10] != "[SubLite] " {
			t.Errorf("Node should have prefix, got %s", n.Name)
		}
	}
}

func TestEnsureUniqueNamesNoCollision(t *testing.T) {
	nodes := []*models.Node{
		{Name: "HK", Type: models.TypeShadowsocks, Server: "1.1.1.1", Port: 8388},
		{Name: "HK (1)", Type: models.TypeShadowsocks, Server: "1.1.1.2", Port: 8388},
		{Name: "HK", Type: models.TypeShadowsocks, Server: "1.1.1.3", Port: 8388},
		{Name: "HK", Type: models.TypeShadowsocks, Server: "1.1.1.4", Port: 8388},
	}
	res := ProcessNodes(nodes, &models.Profile{Deduplicate: false})
	seen := make(map[string]bool)
	for _, n := range res {
		if seen[n.Name] {
			t.Fatalf("duplicate node name produced: %q", n.Name)
		}
		seen[n.Name] = true
	}
}

func TestDeduplicateMultiTenantSameHostDifferentCredentials(t *testing.T) {
	nodes := []*models.Node{
		{Name: "Tenant A", Type: models.TypeVLESS, Server: "shared.example.com", Port: 443, UUID: "uuid-a", SNI: "a.example.com"},
		{Name: "Tenant B", Type: models.TypeVLESS, Server: "shared.example.com", Port: 443, UUID: "uuid-b", SNI: "b.example.com"},
		{Name: "Tenant A Copy", Type: models.TypeVLESS, Server: "shared.example.com", Port: 443, UUID: "uuid-a", SNI: "a.example.com"},
		{Name: "Invalid Port", Type: models.TypeVLESS, Server: "bad.example.com", Port: 0, UUID: "uuid-x"},
	}

	res := ProcessNodes(nodes, &models.Profile{Deduplicate: true})
	if len(res) != 2 {
		t.Fatalf("expected 2 distinct credential nodes (and drop invalid port=0), got %d", len(res))
	}
}

func TestSortByRegion(t *testing.T) {
	nodes := []*models.Node{
		{Name: "US Los Angeles", Type: models.TypeTrojan, Server: "us.example.com", Port: 443, Password: "p"},
		{Name: "HK IEPL 01", Type: models.TypeTrojan, Server: "hk.example.com", Port: 443, Password: "p"},
		{Name: "JP Tokyo", Type: models.TypeTrojan, Server: "jp.example.com", Port: 443, Password: "p"},
	}

	res := ProcessNodes(nodes, &models.Profile{SortMode: "region"})
	if len(res) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(res))
	}
	if res[0].Name != "HK IEPL 01" || res[1].Name != "JP Tokyo" || res[2].Name != "US Los Angeles" {
		t.Fatalf("unexpected region sort order: %s, %s, %s", res[0].Name, res[1].Name, res[2].Name)
	}
}

func TestFilterInfoNodesHealthyNodesAndRegexValidation(t *testing.T) {
	nodes := []*models.Node{
		{Name: "剩余流量：85.5 GB", Type: models.TypeTrojan, Server: "info.example.com", Port: 443, Password: "p"},
		{Name: "套餐到期：2026-12-31", Type: models.TypeTrojan, Server: "expire.example.com", Port: 443, Password: "p"},
		{Name: "HK-Dead", Type: models.TypeTrojan, Server: "dead.example.com", Port: 443, Password: "p", LatencyMs: -1},
		{Name: "HK-Fast", Type: models.TypeTrojan, Server: "fast.example.com", Port: 443, Password: "p", LatencyMs: 32},
		{Name: "JP-Untested", Type: models.TypeTrojan, Server: "untested.example.com", Port: 443, Password: "p", LatencyMs: 0},
	}

	res := ProcessNodes(nodes, &models.Profile{
		FilterInfoNodes: true,
		OnlyHealthy:     true,
	})
	if len(res) != 2 {
		t.Fatalf("expected 2 nodes (HK-Fast + JP-Untested), got %d: %+v", len(res), res)
	}
	if res[0].Name != "HK-Fast" || res[1].Name != "JP-Untested" {
		t.Fatalf("unexpected nodes kept: %+v", res)
	}

	if err := ValidateProfileRegexes(&models.Profile{IncludeRegex: "["}); err == nil {
		t.Fatalf("expected invalid include regex to return error")
	}
	if err := ValidateProfileRegexes(&models.Profile{IncludeRegex: "HK|JP"}); err != nil {
		t.Fatalf("expected valid regex to pass, got %v", err)
	}
}

func TestRegionIncludeExcludeFilter(t *testing.T) {
	nodes := []*models.Node{
		{Name: "香港 01", Server: "1.1.1.1", Port: 443},
		{Name: "US-West", Server: "1.1.1.2", Port: 443},
		{Name: "日本 03", Server: "1.1.1.3", Port: 443},
		{Name: "未分类", Server: "1.1.1.4", Port: 443},
	}

	p := &models.Profile{RegionInclude: []string{"US"}}
	out := ProcessNodes(nodes, p)
	if len(out) != 1 || out[0].Name != "US-West" {
		t.Fatalf("region include failed: %+v", out)
	}

	p = &models.Profile{RegionExclude: []string{"HK", "JP"}}
	out = ProcessNodes(nodes, p)
	if len(out) != 2 {
		t.Fatalf("region exclude failed: %+v", out)
	}
	for _, n := range out {
		if n.Name == "香港 01" || n.Name == "日本 03" {
			t.Fatalf("excluded region leaked: %s", n.Name)
		}
	}
}

func TestRegexPrioritySort(t *testing.T) {
	nodes := []*models.Node{
		{Name: "B-普通", Server: "1.1.1.1", Port: 443},
		{Name: "HK-1", Server: "1.1.1.2", Port: 443},
		{Name: "A-普通", Server: "1.1.1.3", Port: 443},
		{Name: "US-1", Server: "1.1.1.4", Port: 443},
		{Name: "HK-2", Server: "1.1.1.5", Port: 443},
	}

	p := &models.Profile{SortMode: "regex", RegexSort: []string{"US", "HK"}}
	out := ProcessNodes(nodes, p)
	got := make([]string, 0, len(out))
	for _, n := range out {
		got = append(got, n.Name)
	}
	want := []string{"US-1", "HK-1", "HK-2", "B-普通", "A-普通"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("regex sort order = %v, want %v", got, want)
		}
	}
}

func TestProfileOverridesApplied(t *testing.T) {
	nodes := []*models.Node{
		{Name: "A", Server: "1.1.1.1", Port: 443, Type: models.TypeTrojan, Password: "p"},
		{Name: "B", Server: "1.1.1.2", Port: 443, Type: models.TypeVLESS, UUID: "u", TLS: true},
	}
	fresh := func() []*models.Node {
		cp := make([]*models.Node, len(nodes))
		for i, n := range nodes {
			c := *n
			cp[i] = &c
		}
		return cp
	}

	sni := "override.example.com"
	insecure := true
	udp := false
	p := &models.Profile{OverrideSNI: &sni, OverrideInsecure: &insecure, OverrideUDP: &udp}
	out := ProcessNodes(fresh(), p)
	for _, n := range out {
		if n.SNI != "override.example.com" {
			t.Fatalf("SNI override not applied: %s", n.Name)
		}
		if !n.Insecure {
			t.Fatalf("insecure override not applied: %s", n.Name)
		}
		if n.UDP == nil || *n.UDP {
			t.Fatalf("udp override not applied: %s", n.Name)
		}
	}

	// nil overrides must leave nodes untouched
	p2 := &models.Profile{}
	out2 := ProcessNodes(fresh(), p2)
	for _, n := range out2 {
		if n.SNI != "" || n.Insecure || n.UDP != nil {
			t.Fatalf("nil override mutated node: %+v", n)
		}
	}
}

func TestValidateRegexSortPatterns(t *testing.T) {
	p := &models.Profile{RegexSort: []string{"US", "["}}
	if err := ValidateProfileRegexes(p); err == nil {
		t.Fatalf("expected invalid regex-sort pattern to be rejected")
	}
}
