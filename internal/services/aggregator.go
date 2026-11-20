package services

import (
	"subdeck/internal/database"
	"subdeck/internal/models"
	"subdeck/internal/pipeline"
)

// AggregateProfileNodes gathers cached nodes from the profile's subscriptions
// (or all subscriptions when SubscriptionIDs is empty), applies pipeline rules,
// and returns the processed nodes along with an optional upstream Subscription-Userinfo header.
func AggregateProfileNodes(db *database.DB, profile *models.Profile) ([]*models.Node, string) {
	var subs []*models.Subscription

	if len(profile.SubscriptionIDs) == 0 {
		if allSubs, err := db.ListSubscriptions(""); err == nil {
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
	var userInfo string
	for _, sub := range subs {
		for _, n := range sub.CachedNodes {
			copyNode := *n
			rawNodes = append(rawNodes, &copyNode)
		}
	}
	if len(subs) == 1 && subs[0].UserInfo != "" {
		userInfo = subs[0].UserInfo
	}

	processed := pipeline.ProcessNodes(rawNodes, profile)
	if processed == nil {
		processed = []*models.Node{}
	}
	return processed, userInfo
}
