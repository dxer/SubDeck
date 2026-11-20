package services

import (
	"log"
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

func (s *Scheduler) checkAndUpdate() {
	subs, err := s.db.ListSubscriptions(string(models.SubscriptionTypeRemote))
	if err != nil {
		log.Printf("[Scheduler] Error listing subscriptions: %v\n", err)
		return
	}

	now := time.Now()
	for _, sub := range subs {
		if !sub.AutoUpdate {
			continue
		}

		interval := time.Duration(sub.IntervalHours) * time.Hour
		if interval <= 0 {
			interval = 12 * time.Hour
		}

		if sub.Status == "pending" || now.Sub(sub.LastUpdated) >= interval {
			log.Printf("[Scheduler] Auto-updating remote subscription '%s' (ID: %s)...\n", sub.Name, sub.ID)
			_, err := s.fetcher.FetchSubscription(sub.ID)
			if err != nil {
				log.Printf("[Scheduler] Failed auto-updating '%s': %v\n", sub.Name, err)
			} else {
				log.Printf("[Scheduler] Successfully updated '%s'\n", sub.Name)
			}
		}
	}
}
