package services

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"subdeck/internal/database"
	"subdeck/internal/models"
)

type Scheduler struct {
	db      *database.DB
	fetcher *FetcherService
	stopCh  chan struct{}
}

func NewScheduler(db *database.DB, fetcher *FetcherService) *Scheduler {
	return &Scheduler{
		db:      db,
		fetcher: fetcher,
		stopCh:  make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	ticker := time.NewTicker(10 * time.Minute)
	go func() {
		time.Sleep(15 * time.Second)
		s.checkAndUpdate()
		s.checkQuotaWarnings()

		for {
			select {
			case <-ticker.C:
				s.checkAndUpdate()
				s.checkQuotaWarnings()
			case <-s.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
}

const schedulerConcurrency = 4

func shouldUpdateSubscription(sub *models.Subscription, now time.Time) bool {
	if sub == nil || !sub.AutoUpdate {
		return false
	}
	if sub.Status == "pending" {
		return true
	}

	interval := time.Duration(sub.IntervalHours) * time.Hour
	if interval <= 0 {
		interval = 12 * time.Hour
	}

	if sub.Status == "error" {
		failures := sub.ConsecutiveFailures
		if failures < 1 {
			failures = 1
		}
		shift := failures - 1
		if shift > 5 {
			shift = 5
		}
		backoff := (10 * time.Minute) << shift
		maxBackoff := 4 * time.Hour
		if interval < maxBackoff {
			maxBackoff = interval
		}
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
		lastAttempt := sub.LastAttemptAt
		if lastAttempt.IsZero() {
			lastAttempt = sub.UpdatedAt
		}
		return now.Sub(lastAttempt) >= backoff
	}

	return now.Sub(sub.LastUpdated) >= interval
}

func (s *Scheduler) checkAndUpdate() {
	subs, err := s.db.ListSubscriptions(string(models.SubscriptionTypeRemote))
	if err != nil {
		log.Printf("[Scheduler] Error listing subscriptions: %v\n", err)
		return
	}

	now := time.Now()
	var due []*models.Subscription
	for _, sub := range subs {
		if shouldUpdateSubscription(sub, now) {
			due = append(due, sub)
		}
	}
	if len(due) == 0 {
		return
	}

	jobs := make(chan *models.Subscription, len(due))
	for _, sub := range due {
		jobs <- sub
	}
	close(jobs)

	workers := schedulerConcurrency
	if len(due) < workers {
		workers = len(due)
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for sub := range jobs {
				log.Printf("[Scheduler] Auto-updating remote subscription '%s' (ID: %s)...\n", sub.Name, sub.ID)
				if _, err := s.fetcher.FetchSubscription(sub.ID); err != nil {
					log.Printf("[Scheduler] Failed auto-updating '%s': %v\n", sub.Name, err)
				} else {
					log.Printf("[Scheduler] Successfully updated '%s'\n", sub.Name)
				}
			}
		}()
	}
	wg.Wait()
}

// checkQuotaWarnings inspects every remote subscription's quota header and sends
// at most one expiry/traffic warning per subscription per day when the expiry is
// within the configured day threshold or usage exceeds the percent threshold.
func (s *Scheduler) checkQuotaWarnings() {
	settings := s.db.GetSystemSettings()
	if settings == nil || settings.WebhookURL == "" {
		return
	}
	subs, err := s.db.ListSubscriptions(string(models.SubscriptionTypeRemote))
	if err != nil {
		log.Printf("[Scheduler] Error listing subscriptions for quota check: %v\n", err)
		return
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	for _, sub := range subs {
		upload, download, total, expire, ok := parseUserInfo(sub.UserInfo)
		if !ok {
			continue
		}
		var reasons []string
		if expire > 0 {
			expiry := time.Unix(expire, 0)
			if !expiry.After(now) {
				reasons = append(reasons, fmt.Sprintf("套餐已过期（%s）", expiry.Format("2006-01-02")))
			} else if days := int(expiry.Sub(now).Hours() / 24); days <= settings.QuotaWarnDays {
				reasons = append(reasons, fmt.Sprintf("套餐将于 %d 天后到期（%s）", days, expiry.Format("2006-01-02")))
			}
		}
		if total > 0 {
			usedPercent := (upload + download) * 100 / total
			if usedPercent >= int64(settings.QuotaWarnPercent) {
				reasons = append(reasons, fmt.Sprintf("流量已用 %.1f GB / %.1f GB（%d%%）",
					float64(upload+download)/(1<<30), float64(total)/(1<<30), usedPercent))
			}
		}
		if len(reasons) == 0 {
			continue
		}

		// Daily dedup: at most one combined warning per subscription per day.
		key := "quota_warn_last_" + sub.ID
		if last, err := s.db.GetSetting(key); err == nil && last == today {
			continue
		}
		if err := s.db.SetSetting(key, today); err != nil {
			continue
		}
		log.Printf("[Scheduler] Quota warning for '%s': %s\n", sub.Name, strings.Join(reasons, "；"))
		title := fmt.Sprintf("SubDeck 套餐预警: %s", sub.Name)
		msg := fmt.Sprintf("订阅 [%s] %s。请及时续费或更换订阅源。", sub.Name, strings.Join(reasons, "；"))
		go func(url, t, m string) {
			_ = SendWebhookNotification(url, "subscription_quota_warning", t, m)
		}(settings.WebhookURL, title, msg)
	}
}
