package services

import (
	"fmt"
	"strconv"
	"strings"

	"subdeck/internal/database"
	"subdeck/internal/models"
	"subdeck/internal/pipeline"
)

func cloneNode(n *models.Node) *models.Node {
	if n == nil {
		return nil
	}
	cp := *n
	if len(n.ALPN) > 0 {
		cp.ALPN = append([]string(nil), n.ALPN...)
	}
	if len(n.WGReserved) > 0 {
		cp.WGReserved = append([]int(nil), n.WGReserved...)
	}
	if n.Reality != nil {
		r := *n.Reality
		cp.Reality = &r
	}
	if n.WSOptions != nil {
		ws := *n.WSOptions
		if len(n.WSOptions.Headers) > 0 {
			ws.Headers = make(map[string]string, len(n.WSOptions.Headers))
			for k, v := range n.WSOptions.Headers {
				ws.Headers[k] = v
			}
		}
		cp.WSOptions = &ws
	}
	if n.GRPCOptions != nil {
		g := *n.GRPCOptions
		cp.GRPCOptions = &g
	}
	if n.HTTPOptions != nil {
		h := *n.HTTPOptions
		if len(n.HTTPOptions.Path) > 0 {
			h.Path = append([]string(nil), n.HTTPOptions.Path...)
		}
		if len(n.HTTPOptions.Headers) > 0 {
			h.Headers = make(map[string]string, len(n.HTTPOptions.Headers))
			for k, v := range n.HTTPOptions.Headers {
				h.Headers[k] = v
			}
		}
		cp.HTTPOptions = &h
	}
	if n.LastProbed != nil {
		t := *n.LastProbed
		cp.LastProbed = &t
	}
	return &cp
}

func parseUserInfo(raw string) (upload, download, total, expire int64, ok bool) {
	if strings.TrimSpace(raw) == "" {
		return 0, 0, 0, 0, false
	}
	for _, part := range strings.Split(raw, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		val, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 64)
		if err != nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(kv[0])) {
		case "upload":
			upload = val
			ok = true
		case "download":
			download = val
			ok = true
		case "total":
			total = val
			ok = true
		case "expire":
			expire = val
			ok = true
		}
	}
	return upload, download, total, expire, ok
}

func aggregateUserInfo(subs []*models.Subscription) string {
	var validInfos []string
	for _, sub := range subs {
		if strings.TrimSpace(sub.UserInfo) != "" {
			validInfos = append(validInfos, sub.UserInfo)
		}
	}
	if len(validInfos) == 0 {
		return ""
	}
	if len(validInfos) == 1 {
		return validInfos[0]
	}

	var sumUpload, sumDownload, sumTotal, minExpire int64
	var anyParsed bool
	for _, raw := range validInfos {
		u, d, t, e, ok := parseUserInfo(raw)
		if !ok {
			continue
		}
		anyParsed = true
		sumUpload += u
		sumDownload += d
		sumTotal += t
		if e > 0 && (minExpire == 0 || e < minExpire) {
			minExpire = e
		}
	}
	if !anyParsed {
		return ""
	}
	return fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", sumUpload, sumDownload, sumTotal, minExpire)
}

// nodeAllowedBySelection reports whether a node joins the profile according to the
// per-subscription node selection. Selection entries match by node name (legacy)
// or stable selection key; a node qualifies when either matches. A missing or
// empty selection for the subscription means "keep every node".
func nodeAllowedBySelection(p *models.Profile, subID string, n *models.Node) bool {
	if n == nil {
		return false
	}
	if len(p.SelectedNodes) == 0 && len(p.SelectedNodeKeys) == 0 {
		return true
	}
	names, hasNames := p.SelectedNodes[subID]
	keys, hasKeys := p.SelectedNodeKeys[subID]
	if (!hasNames || len(names) == 0) && (!hasKeys || len(keys) == 0) {
		return true
	}
	for _, name := range names {
		if name == n.Name {
			return true
		}
	}
	nodeKey := n.SelectionKey
	if nodeKey == "" {
		nodeKey = n.StableSelectionKey()
	}
	if nodeKey != "" {
		for _, key := range keys {
			if key != "" && key == nodeKey {
				return true
			}
		}
	}
	return false
}

// participatingSubscriptions resolves the subscriptions feeding a profile.
// Legacy profiles (mode "" / "all") treat an empty subscription list as
// "aggregate every subscription". Profiles saved with the explicit selection
// mode honor an empty list as genuinely empty, so clearing the selection no
// longer accidentally exports every source.
func ParticipatingSubscriptions(allSubs []*models.Subscription, subMap map[string]*models.Subscription, profile *models.Profile) []*models.Subscription {
	explicit := profile.SubscriptionSelectionMode == "explicit"
	if len(profile.SubscriptionIDs) == 0 {
		if explicit {
			return nil
		}
		return allSubs
	}
	var subs []*models.Subscription
	for _, subID := range profile.SubscriptionIDs {
		if sub, ok := subMap[subID]; ok && sub != nil {
			subs = append(subs, sub)
		}
	}
	return subs
}

// AggregateProfileNodesFromSubs runs the profile pipeline entirely in memory using preloaded subscriptions
// to eliminate N+1 SQLite queries in list endpoints.
func AggregateProfileNodesFromSubs(allSubs []*models.Subscription, subMap map[string]*models.Subscription, profile *models.Profile) ([]*models.Node, string, []*models.Subscription) {
	subs := ParticipatingSubscriptions(allSubs, subMap, profile)

	var rawNodes []*models.Node
	for _, sub := range subs {
		for _, n := range sub.CachedNodes {
			if !nodeAllowedBySelection(profile, sub.ID, n) {
				continue
			}
			rawNodes = append(rawNodes, cloneNode(n))
		}
	}

	processed := pipeline.ProcessNodes(rawNodes, profile)
	if processed == nil {
		processed = []*models.Node{}
	}
	return processed, aggregateUserInfo(subs), subs
}

// AggregateProfileWithMeta gathers cached nodes and participating subscription metadata for ETag/telemetry.
func AggregateProfileWithMeta(db *database.DB, profile *models.Profile) ([]*models.Node, string, []*models.Subscription) {
	var subs []*models.Subscription

	explicit := profile.SubscriptionSelectionMode == "explicit"
	if len(profile.SubscriptionIDs) == 0 {
		if explicit {
			subs = nil
		} else if allSubs, err := db.ListSubscriptions(""); err == nil {
			subs = allSubs
		}
	} else {
		for _, subID := range profile.SubscriptionIDs {
			if sub, err := db.GetSubscription(subID); err == nil && sub != nil {
				subs = append(subs, sub)
			}
		}
	}

	var rawNodes []*models.Node
	for _, sub := range subs {
		for _, n := range sub.CachedNodes {
			if !nodeAllowedBySelection(profile, sub.ID, n) {
				continue
			}
			rawNodes = append(rawNodes, cloneNode(n))
		}
	}

	processed := pipeline.ProcessNodes(rawNodes, profile)
	if processed == nil {
		processed = []*models.Node{}
	}
	return processed, aggregateUserInfo(subs), subs
}

// AggregateProfileNodes gathers cached nodes from the profile's subscriptions
// (or all subscriptions when SubscriptionIDs is empty), applies pipeline rules,
// and returns the processed nodes along with an aggregated upstream Subscription-Userinfo header.
func AggregateProfileNodes(db *database.DB, profile *models.Profile) ([]*models.Node, string) {
	nodes, userInfo, _ := AggregateProfileWithMeta(db, profile)
	return nodes, userInfo
}
