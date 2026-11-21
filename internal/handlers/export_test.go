package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"subdeck/internal/database"
	"subdeck/internal/models"
	"subdeck/internal/services"

	"github.com/gin-gonic/gin"
)

func setupTestHandlersDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestExportLandingPageEscapesXSSAndDetectsClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestHandlersDB(t)

	sub := &models.Subscription{
		Name:     "RemoteSub",
		Type:     models.SubscriptionTypeRemote,
		URL:      "https://example.com/sub",
		UserInfo: "upload=100; download=200; total=1000; expire=9999999",
		CachedNodes: []*models.Node{
			{
				Name:     "HK-01",
				Type:     models.TypeShadowsocks,
				Server:   "1.2.3.4",
				Port:     8388,
				Cipher:   "aes-256-gcm",
				Password: "secret",
			},
		},
		NodeCount: 1,
		Status:    "ok",
	}
	if err := db.CreateSubscription(sub); err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}

	xssName := `<script>alert('xss')</script>`
	profile := &models.Profile{
		Name:            xssName,
		SubscriptionIDs: []string{sub.ID},
		Deduplicate:     true,
	}
	if err := db.CreateProfile(profile); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}

	exportH := NewExportHandler(db, nil)
	profH := NewProfileHandler(db, nil)

	r := gin.New()
	r.GET("/s/:token", exportH.ExportSubscription)
	r.GET("/api/profiles/:id/preview", profH.Preview)

	// 1. Browser request should render escaped HTML without raw <script>
	req := httptest.NewRequest(http.MethodGet, "/s/"+profile.Token, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/130.0")
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for browser landing page, got %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, xssName) {
		t.Fatalf("raw XSS payload was not escaped in landing page HTML")
	}
	if !strings.Contains(body, "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;") {
		t.Fatalf("expected escaped profile name in HTML")
	}

	// 2. Sing-box UA auto-detection + upstream Subscription-Userinfo propagation
	reqSB := httptest.NewRequest(http.MethodGet, "/s/"+profile.Token, nil)
	reqSB.Header.Set("User-Agent", "sing-box 1.11.0")
	wSB := httptest.NewRecorder()
	r.ServeHTTP(wSB, reqSB)

	if wSB.Code != http.StatusOK {
		t.Fatalf("expected 200 for sing-box request, got %d", wSB.Code)
	}
	if got := wSB.Header().Get("subscription-userinfo"); got != sub.UserInfo {
		t.Fatalf("expected upstream userinfo %q, got %q", sub.UserInfo, got)
	}
	if !strings.Contains(wSB.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("expected JSON content-type for sing-box, got %q", wSB.Header().Get("Content-Type"))
	}

	// 3. Preview endpoint returns JSON array
	reqPrev := httptest.NewRequest(http.MethodGet, "/api/profiles/"+profile.ID+"/preview", nil)
	wPrev := httptest.NewRecorder()
	r.ServeHTTP(wPrev, reqPrev)
	if wPrev.Code != http.StatusOK {
		t.Fatalf("expected 200 for preview, got %d", wPrev.Code)
	}
	var previewNodes []models.Node
	if err := json.Unmarshal(wPrev.Body.Bytes(), &previewNodes); err != nil {
		t.Fatalf("expected JSON array from preview, got %s (err=%v)", wPrev.Body.String(), err)
	}
	if len(previewNodes) != 1 || previewNodes[0].Name != "HK-01" {
		t.Fatalf("unexpected preview nodes: %+v", previewNodes)
	}
}

func newExportFixture(t *testing.T) (*database.DB, *gin.Engine, *models.Profile, *models.Subscription) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupTestHandlersDB(t)

	sub := &models.Subscription{
		Name:     "RemoteSub",
		Type:     models.SubscriptionTypeRemote,
		URL:      "https://example.com/sub",
		UserInfo: "upload=100; download=200; total=1000; expire=9999999",
		CachedNodes: []*models.Node{
			{
				Name:     "HK-01",
				Type:     models.TypeShadowsocks,
				Server:   "1.2.3.4",
				Port:     8388,
				Cipher:   "aes-256-gcm",
				Password: "secret",
				RawURI:   "ss://raw-should-not-leak",
			},
		},
		NodeCount: 1,
		Status:    "ok",
	}
	if err := db.CreateSubscription(sub); err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}
	profile := &models.Profile{
		Name:            "json-profile",
		SubscriptionIDs: []string{sub.ID},
		Deduplicate:     true,
	}
	if err := db.CreateProfile(profile); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}

	exportH := NewExportHandler(db, nil)
	r := gin.New()
	r.GET("/s/:token", exportH.ExportSubscription)
	return db, r, profile, sub
}

func TestExportPlainJSONFormat(t *testing.T) {
	_, r, profile, _ := newExportFixture(t)

	req := httptest.NewRequest(http.MethodGet, "/s/"+profile.Token+"?format=json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json, got %q", ct)
	}
	var nodes []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &nodes); err != nil {
		t.Fatalf("expected JSON array, got %s (err=%v)", w.Body.String(), err)
	}
	if len(nodes) != 1 || nodes[0]["name"] != "HK-01" || nodes[0]["type"] != "ss" {
		t.Fatalf("unexpected nodes: %+v", nodes)
	}
	if _, leaks := nodes[0]["raw_uri"]; leaks {
		t.Fatalf("raw_uri must not appear in plain JSON export")
	}
	if _, leaks := nodes[0]["selection_key"]; leaks {
		t.Fatalf("selection_key must not appear in plain JSON export")
	}
}

func TestExportETagNotModified(t *testing.T) {
	_, r, profile, _ := newExportFixture(t)

	req := httptest.NewRequest(http.MethodGet, "/s/"+profile.Token+"?format=clash", nil)
	req.Header.Set("User-Agent", "ClashMeta/1.18.0")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("expected ETag header")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/s/"+profile.Token+"?format=clash", nil)
	req2.Header.Set("User-Agent", "ClashMeta/1.18.0")
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotModified {
		t.Fatalf("expected 304 for matching ETag, got %d", w2.Code)
	}
}

func TestExportSingboxProviderReturnsBareArray(t *testing.T) {
	_, r, profile, _ := newExportFixture(t)

	req := httptest.NewRequest(http.MethodGet, "/s/"+profile.Token+"?format=singbox&type=provider", nil)
	req.Header.Set("User-Agent", "sing-box 1.11.0")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var arr []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &arr); err != nil {
		t.Fatalf("provider mode sing-box must be a bare array, got %s", w.Body.String())
	}
	if len(arr) != 1 || arr[0]["type"] != "shadowsocks" {
		t.Fatalf("unexpected outbounds: %+v", arr)
	}
}

func TestExportRefreshHeaderWithoutFetcher(t *testing.T) {
	_, r, profile, _ := newExportFixture(t)

	req := httptest.NewRequest(http.MethodGet, "/s/"+profile.Token+"?format=clash&refresh=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// No fetcher injected: refresh is a no-op, request still succeeds from cache.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from cache, got %d", w.Code)
	}
	if got := w.Header().Get("X-Subdeck-Refresh"); got != "" {
		t.Fatalf("refresh header must be absent without a fetcher, got %q", got)
	}
}

func TestExportRefreshFetchesUpstream(t *testing.T) {
	db, _, profile, _ := newExportFixture(t)

	var version atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if version.Load() == 0 {
			_, _ = w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjEwODAx#V1-Node"))
		} else {
			_, _ = w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzc3dvcmRAMTI3LjAuMC4xOjEwODAx#V2-Node"))
		}
	}))
	defer srv.Close()

	// Point the profile's source at the live upstream; initial cache holds the stale node.
	subs, _ := db.ListSubscriptions("")
	subs[0].URL = srv.URL
	subs[0].URLs = []string{srv.URL}
	subs[0].Status = "ok"
	subs[0].CachedNodes = []*models.Node{{Name: "V1-Node", Type: models.TypeShadowsocks, Server: "1.2.3.4", Port: 8388, Cipher: "aes-256-gcm", Password: "secret"}}
	subs[0].NodeCount = 1
	if err := db.UpdateSubscription(subs[0]); err != nil {
		t.Fatalf("UpdateSubscription failed: %v", err)
	}
	// Backdate the last attempt so the refresh cooldown does not skip the fetch.
	subs[0].LastAttemptAt = time.Now().Add(-5 * time.Minute)
	if err := db.UpdateSubscription(subs[0]); err != nil {
		t.Fatalf("backdate failed: %v", err)
	}

	fetcher := services.NewFetcherService(db)
	fetcher.AllowPrivateNetwork = true
	exportH := NewExportHandler(db, fetcher)
	r2 := gin.New()
	r2.GET("/s/:token", exportH.ExportSubscription)

	version.Store(1)
	req := httptest.NewRequest(http.MethodGet, "/s/"+profile.Token+"?format=json&refresh=1", nil)
	w := httptest.NewRecorder()
	r2.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("X-Subdeck-Refresh"); !strings.Contains(got, "refreshed=1") {
		t.Fatalf("expected refreshed=1 in header, got %q", got)
	}
	var nodes []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &nodes); err != nil {
		t.Fatalf("expected JSON array, got %s", w.Body.String())
	}
	if len(nodes) != 1 || nodes[0]["name"] != "V2-Node" {
		t.Fatalf("refresh=1 must serve fresh upstream data, got %+v", nodes)
	}
}
