package services

import (
	"testing"

	"subdeck/internal/models"
)

func TestCloneNodeDeepCopy(t *testing.T) {
	orig := &models.Node{
		Name: "HK",
		ALPN: []string{"h2"},
		WSOptions: &models.WSOptions{
			Path:    "/ws",
			Headers: map[string]string{"Host": "a.example.com"},
		},
		Reality: &models.Reality{PublicKey: "pbk", ShortID: "sid"},
	}

	cloned := cloneNode(orig)
	cloned.Name = "Mutated"
	cloned.ALPN[0] = "http/1.1"
	cloned.WSOptions.Headers["Host"] = "mutated.example.com"
	cloned.Reality.PublicKey = "mutated-pbk"

	if orig.Name != "HK" || orig.ALPN[0] != "h2" || orig.WSOptions.Headers["Host"] != "a.example.com" || orig.Reality.PublicKey != "pbk" {
		t.Fatalf("mutating cloned node affected original: %+v", orig)
	}
}

func TestAggregateUserInfo(t *testing.T) {
	subs := []*models.Subscription{
		{UserInfo: "upload=100; download=200; total=1000; expire=1900000000"},
		{UserInfo: "upload=50; download=300; total=2000; expire=1800000000"},
		{UserInfo: ""},
	}

	got := aggregateUserInfo(subs)
	want := "upload=150; download=500; total=3000; expire=1800000000"
	if got != want {
		t.Fatalf("aggregateUserInfo = %q, want %q", got, want)
	}
}

func TestAggregateProfileNodesFromSubsSelectedNodes(t *testing.T) {
	subA := &models.Subscription{
		ID: "sub-a",
		CachedNodes: []*models.Node{
			{ID: "1", Name: "colo1-hy2", Server: "1.1.1.1", Port: 8881},
			{ID: "2", Name: "colo1-tuic", Server: "1.1.1.1", Port: 8882},
			{ID: "3", Name: "colo1-anytls", Server: "1.1.1.1", Port: 8883},
		},
	}
	subB := &models.Subscription{
		ID: "sub-b",
		CachedNodes: []*models.Node{
			{ID: "4", Name: "colo3-vless", Server: "2.2.2.2", Port: 8881},
		},
	}
	subMap := map[string]*models.Subscription{"sub-a": subA, "sub-b": subB}

	// No selection entry: every node of the subscription joins the profile (legacy behavior).
	p := &models.Profile{SubscriptionIDs: []string{"sub-a", "sub-b"}}
	nodes, _, _ := AggregateProfileNodesFromSubs(nil, subMap, p)
	if len(nodes) != 4 {
		t.Fatalf("no selection: got %d nodes, want 4", len(nodes))
	}

	// Selection entry on one subscription keeps only the named nodes; the other subscription stays whole.
	p = &models.Profile{
		SubscriptionIDs: []string{"sub-a", "sub-b"},
		SelectedNodes:   map[string][]string{"sub-a": {"colo1-hy2", "colo1-tuic"}},
	}
	nodes, _, _ = AggregateProfileNodesFromSubs(nil, subMap, p)
	if len(nodes) != 3 {
		t.Fatalf("partial selection: got %d nodes, want 3", len(nodes))
	}
	for _, n := range nodes {
		if n.Name == "colo1-anytls" {
			t.Fatalf("unselected node colo1-anytls leaked into profile")
		}
	}

	// Selection only applies to the subscription it names.
	p = &models.Profile{
		SubscriptionIDs: []string{"sub-b"},
		SelectedNodes:   map[string][]string{"sub-a": {"colo1-hy2"}},
	}
	nodes, _, _ = AggregateProfileNodesFromSubs(nil, subMap, p)
	if len(nodes) != 1 || nodes[0].Name != "colo3-vless" {
		t.Fatalf("selection for other sub leaked: %+v", nodes)
	}

	// Explicitly empty selection list also means "all nodes".
	p = &models.Profile{
		SubscriptionIDs: []string{"sub-a"},
		SelectedNodes:   map[string][]string{"sub-a": {}},
	}
	nodes, _, _ = AggregateProfileNodesFromSubs(nil, subMap, p)
	if len(nodes) != 3 {
		t.Fatalf("empty selection list: got %d nodes, want 3", len(nodes))
	}
}
