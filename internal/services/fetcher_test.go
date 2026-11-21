package services

import (
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
