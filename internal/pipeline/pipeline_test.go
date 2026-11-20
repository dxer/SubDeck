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
