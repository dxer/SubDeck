package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"subdeck/internal/database"
	"subdeck/internal/models"
)

func setupFetcherTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.InitDB(filepath.Join(t.TempDir(), "fetcher_test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestFetchPreservesCacheOnUpstreamFailure(t *testing.T) {
	db := setupFetcherTestDB(t)
	var returnFail atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if returnFail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjEwODAy#HK-01"))
	}))
	defer srv.Close()

	sub := &models.Subscription{
		Name:          "TestSub",
		Type:          models.SubscriptionTypeRemote,
		URL:           srv.URL,
		AutoUpdate:    true,
		IntervalHours: 12,
		Status:        "pending",
	}
	if err := db.CreateSubscription(sub); err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}

	fetcher := NewFetcherService(db)
	fetcher.AllowPrivateNetwork = true

	nodes, err := fetcher.FetchSubscription(sub.ID)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("initial fetch failed: nodes=%d err=%v", len(nodes), err)
	}

	okSub, _ := db.GetSubscription(sub.ID)
	lastSuccess := okSub.LastUpdated

	// Simulate upstream outage
	returnFail.Store(true)
	if _, err := fetcher.FetchSubscription(sub.ID); err == nil {
		t.Fatalf("expected error on upstream 502")
	}

	afterErr, _ := db.GetSubscription(sub.ID)
	if afterErr.Status != "error" || afterErr.ConsecutiveFailures != 1 {
		t.Fatalf("expected status=error failures=1, got status=%s failures=%d", afterErr.Status, afterErr.ConsecutiveFailures)
	}
	if afterErr.NodeCount != 1 || len(afterErr.CachedNodes) != 1 || afterErr.CachedNodes[0].Name != "HK-01" {
		t.Fatalf("expected previously cached node to be preserved on fetch failure, got count=%d nodes=%+v", afterErr.NodeCount, afterErr.CachedNodes)
	}
	if !afterErr.LastUpdated.Equal(lastSuccess) {
		t.Fatalf("expected last_updated to remain unchanged on failure")
	}
}

func TestFetchSSRFBlocksLoopbackByDefault(t *testing.T) {
	db := setupFetcherTestDB(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjEwODAy#Local"))
	}))
	defer srv.Close()

	sub := &models.Subscription{
		Name:   "LoopbackSub",
		Type:   models.SubscriptionTypeRemote,
		URL:    srv.URL,
		Status: "pending",
	}
	if err := db.CreateSubscription(sub); err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}

	fetcher := NewFetcherService(db)
	// AllowPrivateNetwork stays false
	_, err := fetcher.FetchSubscription(sub.ID)
	if err == nil || !strings.Contains(err.Error(), "blocked connection") {
		t.Fatalf("expected SSRF private/loopback block error, got %v", err)
	}
}

func TestFetchSingleflightDeduplicatesConcurrentPulls(t *testing.T) {
	db := setupFetcherTestDB(t)
	var hits atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(80 * time.Millisecond)
		_, _ = w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjEwODAy#HK-01"))
	}))
	defer srv.Close()

	sub := &models.Subscription{
		Name:   "SingleflightSub",
		Type:   models.SubscriptionTypeRemote,
		URL:    srv.URL,
		Status: "pending",
	}
	if err := db.CreateSubscription(sub); err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}

	fetcher := NewFetcherService(db)
	fetcher.AllowPrivateNetwork = true

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = fetcher.FetchSubscription(sub.ID)
		}()
	}
	wg.Wait()

	if hits.Load() != 1 {
		t.Fatalf("expected singleflight to collapse 5 concurrent pulls into 1 HTTP request, got %d", hits.Load())
	}
}

func TestSchedulerBackoffOnError(t *testing.T) {
	now := time.Now()
	sub := &models.Subscription{
		AutoUpdate:          true,
		IntervalHours:       12,
		Status:              "error",
		ConsecutiveFailures: 2, // 20m backoff
		LastUpdated:         now.Add(-24 * time.Hour),
		LastAttemptAt:       now.Add(-5 * time.Minute),
	}
	if shouldUpdateSubscription(sub, now) {
		t.Fatalf("expected backoff to defer retry before 20m")
	}
	sub.LastAttemptAt = now.Add(-25 * time.Minute)
	if !shouldUpdateSubscription(sub, now) {
		t.Fatalf("expected retry after 20m backoff elapsed")
	}
}

func setupTwoURLSub(t *testing.T, db *database.DB, policy string) (*models.Subscription, *atomic.Bool, *atomic.Bool) {
	t.Helper()
	var firstFail atomic.Bool
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if firstFail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjEwODAx#NODE-A"))
	}))
	var secondFail atomic.Bool
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secondFail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjEwODAy#NODE-B"))
	}))
	t.Cleanup(func() { good.Close(); bad.Close() })

	sub := &models.Subscription{
		Name:          "MultiSub",
		Type:          models.SubscriptionTypeRemote,
		URL:           good.URL,
		URLs:          []string{good.URL, bad.URL},
		FailurePolicy: policy,
		Status:        "pending",
	}
	if err := db.CreateSubscription(sub); err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}
	return sub, &firstFail, &secondFail
}

func TestMultiURLStrictPreservesCacheOnAnyFailure(t *testing.T) {
	db := setupFetcherTestDB(t)
	sub, _, secondFail := setupTwoURLSub(t, db, models.FetchFailureStrict)

	fetcher := NewFetcherService(db)
	fetcher.AllowPrivateNetwork = true

	if nodes, err := fetcher.FetchSubscription(sub.ID); err != nil || len(nodes) != 2 {
		t.Fatalf("initial fetch failed: nodes=%d err=%v", len(nodes), err)
	}

	secondFail.Store(true)
	if _, err := fetcher.FetchSubscription(sub.ID); err == nil {
		t.Fatalf("expected strict policy to fail when any URL fails")
	}
	after, _ := db.GetSubscription(sub.ID)
	if after.Status != "error" || after.ConsecutiveFailures != 1 {
		t.Fatalf("expected single failure count, got status=%s failures=%d", after.Status, after.ConsecutiveFailures)
	}
	if after.NodeCount != 2 || len(after.CachedNodes) != 2 || after.CachedNodes[0].Name != "NODE-A" || after.CachedNodes[1].Name != "NODE-B" {
		t.Fatalf("strict failure must preserve whole cache, got %+v", after.CachedNodes)
	}
}

func TestMultiURLSkipCommitsSuccessfulURLs(t *testing.T) {
	db := setupFetcherTestDB(t)
	sub, _, secondFail := setupTwoURLSub(t, db, models.FetchFailureSkip)

	fetcher := NewFetcherService(db)
	fetcher.AllowPrivateNetwork = true

	if _, err := fetcher.FetchSubscription(sub.ID); err != nil {
		t.Fatalf("initial fetch failed: %v", err)
	}
	secondFail.Store(true)
	nodes, err := fetcher.FetchSubscription(sub.ID)
	if err != nil {
		t.Fatalf("skip policy should succeed with partial results: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "NODE-A" {
		t.Fatalf("expected only successful URL nodes, got %+v", nodes)
	}
	after, _ := db.GetSubscription(sub.ID)
	if after.Status != "partial" || after.ErrorMsg == "" {
		t.Fatalf("expected partial status with message, got status=%s msg=%q", after.Status, after.ErrorMsg)
	}
	if after.ConsecutiveFailures != 0 {
		t.Fatalf("partial success must not count as failure, got %d", after.ConsecutiveFailures)
	}
}

func TestMultiURLSkipAllFailPreservesCache(t *testing.T) {
	db := setupFetcherTestDB(t)
	sub, firstFail, secondFail := setupTwoURLSub(t, db, models.FetchFailureSkip)

	fetcher := NewFetcherService(db)
	fetcher.AllowPrivateNetwork = true
	if _, err := fetcher.FetchSubscription(sub.ID); err != nil {
		t.Fatalf("initial fetch failed: %v", err)
	}
	firstFail.Store(true)
	secondFail.Store(true)

	if _, err := fetcher.FetchSubscription(sub.ID); err == nil {
		t.Fatalf("expected error when every URL fails")
	}
	after, _ := db.GetSubscription(sub.ID)
	if after.NodeCount != 2 || len(after.CachedNodes) != 2 {
		t.Fatalf("all-fail must preserve cache, got count=%d", after.NodeCount)
	}
}

func TestMultiURLPreservesURLOrder(t *testing.T) {
	db := setupFetcherTestDB(t)
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjEwODAz#NODE-B"))
	}))
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjEwODA0#NODE-C"))
	}))
	defer second.Close()
	defer first.Close()

	sub := &models.Subscription{
		Name:   "OrderSub",
		Type:   models.SubscriptionTypeRemote,
		URLs:   []string{first.URL, second.URL},
		Status: "pending",
	}
	if err := db.CreateSubscription(sub); err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}

	fetcher := NewFetcherService(db)
	fetcher.AllowPrivateNetwork = true
	nodes, err := fetcher.FetchSubscription(sub.ID)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if len(nodes) != 2 || nodes[0].Name != "NODE-C" || nodes[1].Name != "NODE-B" {
		t.Fatalf("URL order must be preserved, got %+v", nodes)
	}
}

func TestFetchProxyFailsClosedWithoutDirectFallback(t *testing.T) {
	db := setupFetcherTestDB(t)
	hits := atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjEwODA1#Direct"))
	}))
	defer srv.Close()

	sub := &models.Subscription{
		Name:   "ProxySub",
		Type:   models.SubscriptionTypeRemote,
		URL:    srv.URL,
		Status: "pending",
	}
	if err := db.CreateSubscription(sub); err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}

	fetcher := NewFetcherService(db)

	// Private proxy without the private-fetch toggle: fail closed.
	sub.FetchProxyURL = "http://127.0.0.1:10809"
	if err := db.UpdateSubscription(sub); err != nil {
		t.Fatalf("UpdateSubscription failed: %v", err)
	}
	if _, err := fetcher.FetchSubscription(sub.ID); err == nil || !strings.Contains(err.Error(), "fetch proxy") {
		t.Fatalf("expected fail-closed proxy error, got %v", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("must not fall back to direct connection, upstream hit %d times", hits.Load())
	}

	// Unsupported scheme: fail closed.
	sub.FetchProxyURL = "ftp://127.0.0.1:21"
	_ = db.UpdateSubscription(sub)
	if _, err := fetcher.FetchSubscription(sub.ID); err == nil || !strings.Contains(err.Error(), "unsupported fetch proxy scheme") {
		t.Fatalf("expected unsupported scheme error, got %v", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("must not fall back to direct connection, upstream hit %d times", hits.Load())
	}
}

func TestFetchProxyErrorRedactsCredentials(t *testing.T) {
	db := setupFetcherTestDB(t)
	sub := &models.Subscription{
		Name:   "RedactSub",
		Type:   models.SubscriptionTypeRemote,
		URL:    "http://127.0.0.1:1/unreachable",
		Status: "pending",
	}
	if err := db.CreateSubscription(sub); err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}

	fetcher := NewFetcherService(db)
	fetcher.AllowPrivateNetwork = true
	sub.FetchProxyURL = "http://alice:secret-pw@127.0.0.1:1"
	_ = db.UpdateSubscription(sub)

	_, err := fetcher.FetchSubscription(sub.ID)
	if err == nil {
		t.Fatalf("expected proxy connect failure")
	}
	if strings.Contains(err.Error(), "secret-pw") {
		t.Fatalf("error must not leak proxy credentials: %v", err)
	}
}

func TestValidateProxyURLRules(t *testing.T) {
	if _, err := ValidateProxyURL("http://user:pw@proxy.example.com:8080", false); err != nil {
		t.Fatalf("public proxy should pass: %v", err)
	}
	if _, err := ValidateProxyURL("socks5://127.0.0.1:1080", false); err == nil {
		t.Fatalf("private proxy must be blocked without allowPrivate")
	}
	if _, err := ValidateProxyURL("socks5://127.0.0.1:1080", true); err != nil {
		t.Fatalf("private proxy allowed with allowPrivate: %v", err)
	}
	if _, err := ValidateProxyURL("ftp://proxy.example.com", false); err == nil {
		t.Fatalf("unsupported scheme must fail")
	}
}

func setupFailoverSub(t *testing.T, db *database.DB) (*models.Subscription, *atomic.Bool, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	var firstFail atomic.Bool
	var firstHits, secondHits atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstHits.Add(1)
		if firstFail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjEwODAx#MIRROR-A"))
	}))
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits.Add(1)
		_, _ = w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjEwODAy#MIRROR-B"))
	}))
	t.Cleanup(func() { first.Close(); second.Close() })

	sub := &models.Subscription{
		Name:          "FailoverSub",
		Type:          models.SubscriptionTypeRemote,
		URLs:          []string{first.URL, second.URL},
		FetchMode:     models.FetchModeFailover,
		FailurePolicy: models.FetchFailureStrict,
		Status:        "pending",
	}
	if err := db.CreateSubscription(sub); err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}
	return sub, &firstFail, &firstHits, &secondHits
}

func TestFetchModeFailoverAdoptsFirstSuccess(t *testing.T) {
	db := setupFetcherTestDB(t)
	sub, _, firstHits, secondHits := setupFailoverSub(t, db)

	fetcher := NewFetcherService(db)
	fetcher.AllowPrivateNetwork = true
	nodes, err := fetcher.FetchSubscription(sub.ID)
	if err != nil {
		t.Fatalf("failover fetch failed: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "MIRROR-A" {
		t.Fatalf("failover must adopt only the first successful mirror, got %+v", nodes)
	}
	if firstHits.Load() != 1 || secondHits.Load() != 0 {
		t.Fatalf("failover must stop after first success, hits first=%d second=%d", firstHits.Load(), secondHits.Load())
	}
	after, _ := db.GetSubscription(sub.ID)
	if after.Status != "ok" {
		t.Fatalf("failover success status must be ok, got %s", after.Status)
	}
}

func TestFetchModeFailoverSkipsFailingMirror(t *testing.T) {
	db := setupFetcherTestDB(t)
	sub, firstFail, firstHits, secondHits := setupFailoverSub(t, db)

	fetcher := NewFetcherService(db)
	fetcher.AllowPrivateNetwork = true
	firstFail.Store(true)

	nodes, err := fetcher.FetchSubscription(sub.ID)
	if err != nil {
		t.Fatalf("failover should fall back to the second mirror: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "MIRROR-B" {
		t.Fatalf("expected second mirror nodes, got %+v", nodes)
	}
	if firstHits.Load() != 1 || secondHits.Load() != 1 {
		t.Fatalf("each mirror tried once, hits first=%d second=%d", firstHits.Load(), secondHits.Load())
	}
	after, _ := db.GetSubscription(sub.ID)
	if after.Status != "ok" || after.ConsecutiveFailures != 0 {
		t.Fatalf("expected clean success, got status=%s failures=%d", after.Status, after.ConsecutiveFailures)
	}
}

func TestFetchModeFailoverAllMirrorsFailPreservesCache(t *testing.T) {
	db := setupFetcherTestDB(t)
	sub, firstFail, _, _ := setupFailoverSub(t, db)

	fetcher := NewFetcherService(db)
	fetcher.AllowPrivateNetwork = true
	if _, err := fetcher.FetchSubscription(sub.ID); err != nil {
		t.Fatalf("initial fetch failed: %v", err)
	}
	firstFail.Store(true)
	// Break the second mirror by pointing the sub at an unresolvable host.
	// Re-read from DB so the cached nodes survive the row update.
	fresh, err := db.GetSubscription(sub.ID)
	if err != nil {
		t.Fatalf("GetSubscription failed: %v", err)
	}
	deadURL := strings.Replace(sub.URLs[1], "127.0.0.1:", "127.0.0.1.1:", 1)
	fresh.URLs = []string{sub.URLs[0], deadURL}
	if err := db.UpdateSubscription(fresh); err != nil {
		t.Fatalf("UpdateSubscription failed: %v", err)
	}

	if _, err := fetcher.FetchSubscription(sub.ID); err == nil || !strings.Contains(err.Error(), "mirror URL(s) failed") {
		t.Fatalf("expected all-mirrors-failed error, got %v", err)
	}
	after, _ := db.GetSubscription(sub.ID)
	if after.Status != "error" || after.NodeCount != 1 {
		t.Fatalf("all-fail must keep cache and mark error, got status=%s count=%d", after.Status, after.NodeCount)
	}
}

// webhookRecorder captures webhook events with their payload fields.
type webhookRecorder struct {
	srv    *httptest.Server
	events chan map[string]string
}

func newWebhookRecorder(t *testing.T) *webhookRecorder {
	t.Helper()
	rec := &webhookRecorder{events: make(chan map[string]string, 16)}
	rec.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		ev := map[string]string{}
		for k, v := range payload {
			if s, ok := v.(string); ok {
				ev[k] = s
			}
		}
		select {
		case rec.events <- ev:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() { rec.srv.Close() })
	return rec
}

// waitForEvent polls the recorder channel for an event type within the timeout.
func (rec *webhookRecorder) waitForEvent(t *testing.T, event string) map[string]string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case ev := <-rec.events:
			if ev["event"] == event {
				return ev
			}
		case <-time.After(50 * time.Millisecond):
		}
	}
	t.Fatalf("timed out waiting for webhook event %q", event)
	return nil
}

func TestFetchAlertAndRecoveryNotifications(t *testing.T) {
	db := setupFetcherTestDB(t)
	rec := newWebhookRecorder(t)
	if err := db.SetSetting("webhook_url", rec.srv.URL); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}
	SetWebhookAllowPrivateNetwork(true)
	t.Cleanup(func() { SetWebhookAllowPrivateNetwork(false) })

	var upstreamFail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if upstreamFail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjEwODAz#RECOVER-1"))
	}))
	defer srv.Close()

	sub := &models.Subscription{
		Name:   "NotifySub",
		Type:   models.SubscriptionTypeRemote,
		URL:    srv.URL,
		Status: "pending",
	}
	if err := db.CreateSubscription(sub); err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}

	fetcher := NewFetcherService(db)
	fetcher.AllowPrivateNetwork = true

	// Three consecutive failures: alert fires on the 3rd only.
	upstreamFail.Store(true)
	for i := 0; i < 3; i++ {
		if _, err := fetcher.FetchSubscription(sub.ID); err == nil {
			t.Fatalf("fetch %d should fail", i+1)
		}
	}
	alert := rec.waitForEvent(t, "subscription_fetch_failed")
	if !strings.Contains(alert["message"], "连续 3 次") {
		t.Fatalf("alert should report 3 consecutive failures, got %q", alert["message"])
	}

	// Recovery after the alerting state must notify once.
	upstreamFail.Store(false)
	if _, err := fetcher.FetchSubscription(sub.ID); err != nil {
		t.Fatalf("recovery fetch failed: %v", err)
	}
	recovered := rec.waitForEvent(t, "subscription_recovered")
	if !strings.Contains(recovered["message"], "已恢复正常") {
		t.Fatalf("recovery message unexpected: %q", recovered["message"])
	}

	after, _ := db.GetSubscription(sub.ID)
	if after.Status != "ok" || after.ConsecutiveFailures != 0 {
		t.Fatalf("expected clean state after recovery, got status=%s failures=%d", after.Status, after.ConsecutiveFailures)
	}
}

func TestSchedulerQuotaWarningDailyDedup(t *testing.T) {
	db := setupFetcherTestDB(t)
	rec := newWebhookRecorder(t)
	if err := db.SetSetting("webhook_url", rec.srv.URL); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}
	SetWebhookAllowPrivateNetwork(true)
	t.Cleanup(func() { SetWebhookAllowPrivateNetwork(false) })

	// Expiry 71h out truncates to "2 days left" — below the default 7-day bar.
	// Usage 100% of total: upload 10 GiB + download 10 GiB / total 20 GiB.
	expiringSoon := time.Now().Add(71 * time.Hour).Unix()
	warnSub := &models.Subscription{
		Name:          "QuotaWarnSub",
		Type:          models.SubscriptionTypeRemote,
		URL:           "http://example.invalid/sub",
		Status:        "ok",
		AutoUpdate:    false,
		UserInfo:      fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", 10<<30, 10<<30, 20<<30, expiringSoon),
		LastUpdated:   time.Now(),
		LastAttemptAt: time.Now(),
	}
	if err := db.CreateSubscription(warnSub); err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}
	healthySub := &models.Subscription{
		Name:          "HealthySub",
		Type:          models.SubscriptionTypeRemote,
		URL:           "http://example.invalid/sub",
		Status:        "ok",
		AutoUpdate:    false,
		UserInfo:      "upload=0; download=0; total=107374182400; expire=1893456000",
		LastUpdated:   time.Now(),
		LastAttemptAt: time.Now(),
	}
	if err := db.CreateSubscription(healthySub); err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}

	sched := NewScheduler(db, NewFetcherService(db))
	sched.checkQuotaWarnings()

	ev := rec.waitForEvent(t, "subscription_quota_warning")
	if !strings.Contains(ev["message"], "2 天后到期") || !strings.Contains(ev["message"], "100%") {
		t.Fatalf("warning should mention expiry and usage, got %q", ev["message"])
	}

	// Second run on the same day must not re-alert (daily dedup).
	sched.checkQuotaWarnings()
	select {
	case extra := <-rec.events:
		t.Fatalf("quota warning must fire once per day, got extra event %+v", extra)
	case <-time.After(300 * time.Millisecond):
	}
}
