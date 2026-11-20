package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"subdeck/internal/database"
	"subdeck/internal/models"

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

	exportH := NewExportHandler(db)
	profH := NewProfileHandler(db)

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
