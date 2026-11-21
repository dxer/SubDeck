package services

import (
	"log"
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

		for {
			select {
			case <-ticker.C:
				s.checkAndUpdate()
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
