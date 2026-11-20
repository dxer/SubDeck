package services

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"subdeck/internal/database"
	"subdeck/internal/models"
	"subdeck/internal/parser"
)

const maxSubscriptionBodyBytes = 10 << 20 // 10 MiB

type FetcherService struct {
	db             *database.DB
	secureClient   *http.Client
	insecureClient *http.Client
}

func NewFetcherService(db *database.DB) *FetcherService {
	return &FetcherService{
		db: db,
		secureClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		insecureClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: 20 * time.Second,
		},
	}
}

// FetchSubscription pulls content from the subscription URL (for remote) or parses RawContent (for local)
func (s *FetcherService) FetchSubscription(subID string) ([]*models.Node, error) {
	sub, err := s.db.GetSubscription(subID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}

	if sub.Type == models.SubscriptionTypeLocal {
		nodes, err := parser.ParseSubscriptionContent([]byte(sub.RawContent))
		if err != nil {
			_ = s.db.UpdateSubscriptionCache(sub.ID, nil, "error", fmt.Sprintf("parse failed: %v", err))
			return nil, err
		}
		for _, n := range nodes {
			n.Source = sub.Name
		}
		if err := s.db.UpdateSubscriptionCache(sub.ID, nodes, "ok", ""); err != nil {
			return nil, fmt.Errorf("failed to save cache: %w", err)
		}
		return nodes, nil
	}

	// Remote subscription
	req, err := http.NewRequest("GET", sub.URL, nil)
	if err != nil {
		_ = s.db.UpdateSubscriptionCache(sub.ID, nil, "error", err.Error())
		return nil, err
	}

	ua := sub.UserAgent
	if ua == "" {
		ua = "ClashMeta/v1.18.0 (SubDeck)"
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "*/*")

	client := s.secureClient
	if sub.InsecureSkipVerify {
		client = s.insecureClient
	}

	resp, err := client.Do(req)
	if err != nil {
		_ = s.db.UpdateSubscriptionCache(sub.ID, nil, "error", fmt.Sprintf("HTTP request failed: %v", err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg := fmt.Sprintf("server returned status: %d", resp.StatusCode)
		_ = s.db.UpdateSubscriptionCache(sub.ID, nil, "error", errMsg)
		return nil, errors.New(errMsg)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSubscriptionBodyBytes+1))
	if err != nil {
		_ = s.db.UpdateSubscriptionCache(sub.ID, nil, "error", fmt.Sprintf("read body failed: %v", err))
		return nil, err
	}
	if len(body) > maxSubscriptionBodyBytes {
		errMsg := "subscription response exceeds 10MB limit"
		_ = s.db.UpdateSubscriptionCache(sub.ID, nil, "error", errMsg)
		return nil, errors.New(errMsg)
	}

	nodes, err := parser.ParseSubscriptionContent(body)
	if err != nil {
		_ = s.db.UpdateSubscriptionCache(sub.ID, nil, "error", fmt.Sprintf("parse failed: %v", err))
		return nil, err
	}

	// Tag nodes with source
	for _, n := range nodes {
		n.Source = sub.Name
	}

	userInfo := strings.TrimSpace(resp.Header.Get("Subscription-Userinfo"))
	if err := s.db.UpdateSubscriptionCacheWithInfo(sub.ID, nodes, "ok", "", userInfo); err != nil {
		return nil, fmt.Errorf("failed to save cache: %w", err)
	}

	return nodes, nil
}
