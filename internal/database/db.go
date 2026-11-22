package database

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"subdeck/internal/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type DB struct {
	mu     sync.RWMutex
	db     *sql.DB
	dbPath string
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func openSQLiteDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed to apply %s: %w", p, err)
		}
	}
	return db, nil
}

func InitDB(dbPath string) (*DB, error) {
	d, _, err := InitDBWithAdmin(dbPath, "admin", "")
	return d, err
}

func InitDBWithAdmin(dbPath, adminUsername, adminPassword string) (*DB, bool, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, false, fmt.Errorf("failed to create db dir: %w", err)
	}

	db, err := openSQLiteDB(dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	d := &DB{db: db, dbPath: dbPath}
	if err := d.migrate(); err != nil {
		_ = db.Close()
		return nil, false, fmt.Errorf("failed to run migrations: %w", err)
	}

	usedDefaultPass, err := d.SyncAdminUser(adminUsername, adminPassword)
	if err != nil {
		_ = db.Close()
		return nil, false, fmt.Errorf("failed to init admin: %w", err)
	}

	return d, usedDefaultPass, nil
}

// BackupBytes checkpoints WAL and returns a clean snapshot of SQLite db bytes
func (d *DB) BackupBytes() ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if _, err := d.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return nil, fmt.Errorf("failed to checkpoint wal: %w", err)
	}
	return os.ReadFile(d.dbPath)
}

// validateSQLiteBackup verifies integrity and expected schema on a temporary file before replacing the live DB.
func validateSQLiteBackup(path string) error {
	testDB, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("无法打开备份数据库: %w", err)
	}
	defer testDB.Close()

	var integrity string
	if err := testDB.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		return fmt.Errorf("备份文件完整性校验失败")
	}

	var count int
	if err := testDB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return fmt.Errorf("备份文件缺少有效的 SubDeck 数据表")
	}
	return nil
}

func removeSQLiteSidecars(dbPath string) {
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
}

// RestoreFromBytes replaces the database file with uploaded data after validation
func (d *DB) RestoreFromBytes(data []byte) error {
	if len(data) < 16 || string(data[:15]) != "SQLite format 3" {
		return fmt.Errorf("上传的文件不是合法的 SQLite 数据库文件 (缺少签名)")
	}

	tmpPath := d.dbPath + ".restore.tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("写入临时校验文件失败: %w", err)
	}
	defer os.Remove(tmpPath)

	if err := validateSQLiteBackup(tmpPath); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// 1. Flush & close current db
	_, _ = d.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	_ = d.db.Close()

	// 2. Backup current DB to .bak
	bakPath := d.dbPath + ".bak"
	if curData, err := os.ReadFile(d.dbPath); err == nil {
		_ = os.WriteFile(bakPath, curData, 0600)
	}
	removeSQLiteSidecars(d.dbPath)

	// 3. Replace with validated file
	if err := os.Rename(tmpPath, d.dbPath); err != nil {
		_ = os.Rename(bakPath, d.dbPath)
		d.db, _ = openSQLiteDB(d.dbPath)
		return fmt.Errorf("替换数据库文件失败: %w", err)
	}

	// 4. Reopen db
	newDB, err := openSQLiteDB(d.dbPath)
	if err != nil {
		_ = os.Rename(bakPath, d.dbPath)
		d.db, _ = openSQLiteDB(d.dbPath)
		return fmt.Errorf("重新打开数据库失败: %w", err)
	}
	d.db = newDB

	// 5. Ensure migrations apply if backup is from older schema
	if err := d.migrate(); err != nil {
		return fmt.Errorf("迁移恢复后的数据库失败: %w", err)
	}

	// 6. Bump token_version directly while holding d.mu.Lock() so stale sessions must re-authenticate
	var tvStr string
	_ = d.db.QueryRow("SELECT value FROM settings WHERE key = 'token_version'").Scan(&tvStr)
	tv, _ := strconv.Atoi(tvStr)
	if tv < 1 {
		tv = 1
	}
	_, _ = d.db.Exec(
		"INSERT INTO settings (key, value) VALUES ('token_version', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		strconv.Itoa(tv+1),
	)

	return nil
}

func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.db == nil {
		return nil
	}
	return d.db.Close()
}

func (d *DB) hasColumn(table, column string) bool {
	rows, err := d.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		_ = rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk)
		if name == column {
			return true
		}
	}
	return false
}

func (d *DB) ensureColumn(table, column, definition string) error {
	if d.hasColumn(table, column) {
		return nil
	}
	_, err := d.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	return err
}

func (d *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS subscriptions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'remote',
			url TEXT,
			raw_content TEXT,
			user_agent TEXT,
			insecure_skip_verify BOOLEAN NOT NULL DEFAULT 0,
			auto_update BOOLEAN NOT NULL DEFAULT 1,
			interval_hours INTEGER NOT NULL DEFAULT 12,
			node_count INTEGER NOT NULL DEFAULT 0,
			user_info TEXT DEFAULT '',
			last_updated DATETIME NOT NULL,
			status TEXT NOT NULL DEFAULT 'ok',
			error_msg TEXT,
			cached_nodes_json TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS profiles (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			token TEXT UNIQUE NOT NULL,
			subscription_ids_json TEXT NOT NULL,
			selected_nodes_json TEXT,
			deduplicate BOOLEAN NOT NULL DEFAULT 1,
			include_regex TEXT,
			exclude_regex TEXT,
			allowed_protocols_json TEXT,
			sort_mode TEXT DEFAULT '',
			region_groups BOOLEAN NOT NULL DEFAULT 0,
			rename_prefix TEXT,
			rename_suffix TEXT,
			rename_regex_search TEXT,
			rename_regex_replace TEXT,
			full_config_default BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
	}

	for _, q := range queries {
		if _, err := d.db.Exec(q); err != nil {
			return err
		}
	}

	// Auto-migrate schema updates for legacy tables
	if !d.hasColumn("profiles", "subscription_ids_json") {
		if err := d.ensureColumn("profiles", "subscription_ids_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
			return err
		}
		_, _ = d.db.Exec("UPDATE profiles SET subscription_ids_json = remote_sub_ids_json WHERE subscription_ids_json = '[]' AND remote_sub_ids_json IS NOT NULL")
	}

	if err := d.ensureColumn("subscriptions", "insecure_skip_verify", "BOOLEAN NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := d.ensureColumn("subscriptions", "user_info", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := d.ensureColumn("subscriptions", "last_attempt_at", "DATETIME"); err != nil {
		return err
	}
	if err := d.ensureColumn("subscriptions", "consecutive_failures", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := d.ensureColumn("profiles", "sort_mode", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := d.ensureColumn("profiles", "region_groups", "BOOLEAN NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := d.ensureColumn("profiles", "filter_info_nodes", "BOOLEAN NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := d.ensureColumn("profiles", "only_healthy", "BOOLEAN NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := d.ensureColumn("profiles", "rule_preset", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := d.ensureColumn("profiles", "export_mode", "TEXT DEFAULT 'full'"); err != nil {
		return err
	}
	if err := d.ensureColumn("profiles", "last_pull_at", "DATETIME"); err != nil {
		return err
	}
	if err := d.ensureColumn("profiles", "last_pull_client", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := d.ensureColumn("profiles", "last_pull_ip", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := d.ensureColumn("profiles", "pull_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := d.ensureColumn("profiles", "selected_nodes_json", "TEXT DEFAULT '{}'"); err != nil {
		return err
	}
	if err := d.ensureColumn("profiles", "features_json", "TEXT DEFAULT '{}'"); err != nil {
		return err
	}
	if err := d.ensureColumn("subscriptions", "urls_json", "TEXT DEFAULT '[]'"); err != nil {
		return err
	}
	if err := d.ensureColumn("subscriptions", "fetch_proxy_url", "TEXT DEFAULT ''"); err != nil {
		return err
	}
	if err := d.ensureColumn("subscriptions", "failure_policy", "TEXT DEFAULT 'strict'"); err != nil {
		return err
	}
	if err := d.ensureColumn("subscriptions", "fetch_mode", "TEXT DEFAULT 'merge'"); err != nil {
		return err
	}

	return nil
}

func adminConfigSignature(username, password string) string {
	sum := sha256.Sum256([]byte(username + "\x00" + password))
	return hex.EncodeToString(sum[:])
}

// SyncAdminUser creates the initial admin user or synchronizes username/password
// when the configured credentials change compared to the last synced signature.
func (d *DB) SyncAdminUser(username, password string) (usedDefaultPass bool, err error) {
	username = strings.TrimSpace(username)
	if username == "" {
		username = "admin"
	}

	var count int
	if err := d.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return false, err
	}

	sig := adminConfigSignature(username, password)

	if count == 0 {
		initialPass := password
		if initialPass == "" {
			initialPass = "admin123"
			usedDefaultPass = true
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(initialPass), bcrypt.DefaultCost)
		if err != nil {
			return false, err
		}
		if _, err := d.db.Exec(
			"INSERT INTO users (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)",
			uuid.New().String(), username, string(hash), time.Now(),
		); err != nil {
			return false, err
		}
		if err := d.SetSetting("config_admin_sig", sig); err != nil {
			return false, err
		}
		return usedDefaultPass, nil
	}

	prevSig, _ := d.GetSetting("config_admin_sig")
	if prevSig == sig {
		return false, nil
	}

	// Only sync existing admin account when config changed (never overwrite password with empty string)
	var primaryID, primaryUsername string
	if err := d.db.QueryRow("SELECT id, username FROM users ORDER BY created_at ASC LIMIT 1").Scan(&primaryID, &primaryUsername); err != nil {
		return false, err
	}

	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return false, err
		}
		if _, err := d.db.Exec("UPDATE users SET username = ?, password_hash = ? WHERE id = ?", username, string(hash), primaryID); err != nil {
			return false, err
		}
	} else if primaryUsername != username {
		if _, err := d.db.Exec("UPDATE users SET username = ? WHERE id = ?", username, primaryID); err != nil {
			return false, err
		}
	}

	if err := d.SetSetting("config_admin_sig", sig); err != nil {
		return false, err
	}
	return false, nil
}

// Settings methods
func (d *DB) GetSetting(key string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var val string
	err := d.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	if err != nil {
		return "", err
	}
	return val, nil
}

func (d *DB) SetSetting(key, value string) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, err := d.db.Exec(
		"INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	return err
}

// GetOrCreateJWTSecret returns a persisted random JWT signing secret from the settings table.
func (d *DB) GetOrCreateJWTSecret() (string, error) {
	if secret, err := d.GetSetting("jwt_secret"); err == nil && secret != "" {
		return secret, nil
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	secret := hex.EncodeToString(buf)
	if err := d.SetSetting("jwt_secret", secret); err != nil {
		return "", err
	}
	return secret, nil
}

// GetTokenVersion returns the persisted minimum valid JWT token version.
func (d *DB) GetTokenVersion() int {
	val, err := d.GetSetting("token_version")
	if err != nil || val == "" {
		return 1
	}
	v, err := strconv.Atoi(val)
	if err != nil || v < 1 {
		return 1
	}
	return v
}

// BumpTokenVersion increments token_version in SQLite to revoke all previously issued JWTs.
func (d *DB) BumpTokenVersion() (int, error) {
	next := d.GetTokenVersion() + 1
	if err := d.SetSetting("token_version", strconv.Itoa(next)); err != nil {
		return 0, err
	}
	return next, nil
}

// GetSystemSettings loads runtime preferences stored in the settings table.
func (d *DB) GetSystemSettings() *models.SystemSettings {
	webhook, _ := d.GetSetting("webhook_url")
	mirror, _ := d.GetSetting("ruleset_mirror")
	allowPrivStr, _ := d.GetSetting("allow_private_fetch")
	warnDaysStr, _ := d.GetSetting("quota_warn_days")
	warnDays, _ := strconv.Atoi(strings.TrimSpace(warnDaysStr))
	if warnDays <= 0 {
		warnDays = models.DefaultQuotaWarnDays
	}
	warnPercentStr, _ := d.GetSetting("quota_warn_percent")
	warnPercent, _ := strconv.Atoi(strings.TrimSpace(warnPercentStr))
	if warnPercent <= 0 {
		warnPercent = models.DefaultQuotaWarnPercent
	}
	return &models.SystemSettings{
		WebhookURL:        strings.TrimSpace(webhook),
		RulesetMirror:     strings.TrimSpace(mirror),
		AllowPrivateFetch: allowPrivStr == "true" || allowPrivStr == "1",
		QuotaWarnDays:     warnDays,
		QuotaWarnPercent:  warnPercent,
	}
}

// SaveSystemSettings persists runtime preferences into the settings table.
func (d *DB) SaveSystemSettings(s *models.SystemSettings) error {
	if s == nil {
		return nil
	}
	if err := d.SetSetting("webhook_url", strings.TrimSpace(s.WebhookURL)); err != nil {
		return err
	}
	if err := d.SetSetting("ruleset_mirror", strings.TrimSpace(s.RulesetMirror)); err != nil {
		return err
	}
	allowVal := "0"
	if s.AllowPrivateFetch {
		allowVal = "1"
	}
	if err := d.SetSetting("allow_private_fetch", allowVal); err != nil {
		return err
	}
	if s.QuotaWarnDays <= 0 {
		s.QuotaWarnDays = models.DefaultQuotaWarnDays
	}
	if s.QuotaWarnPercent <= 0 {
		s.QuotaWarnPercent = models.DefaultQuotaWarnPercent
	}
	if err := d.SetSetting("quota_warn_days", strconv.Itoa(s.QuotaWarnDays)); err != nil {
		return err
	}
	return d.SetSetting("quota_warn_percent", strconv.Itoa(s.QuotaWarnPercent))
}

// User methods
func (d *DB) GetUserByUsername(username string) (*models.User, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	row := d.db.QueryRow("SELECT id, username, password_hash, created_at FROM users WHERE username = ?", username)
	var u models.User
	var createdAt time.Time
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &createdAt)
	if err != nil {
		return nil, err
	}
	u.CreatedAt = createdAt
	return &u, nil
}

func (d *DB) UpdatePassword(username, newPassword string) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = d.db.Exec("UPDATE users SET password_hash = ? WHERE username = ?", string(hash), username)
	return err
}

const subscriptionSelectCols = "id, name, type, url, COALESCE(urls_json, '[]'), COALESCE(fetch_proxy_url, ''), COALESCE(failure_policy, 'strict'), COALESCE(fetch_mode, 'merge'), raw_content, user_agent, insecure_skip_verify, auto_update, interval_hours, node_count, COALESCE(user_info, ''), last_updated, last_attempt_at, COALESCE(consecutive_failures, 0), status, COALESCE(error_msg, ''), COALESCE(cached_nodes_json, ''), created_at, updated_at"

func scanSubscription(s rowScanner) (*models.Subscription, error) {
	var sub models.Subscription
	var cachedJSON, urlsJSON string
	var lastAttemptAt sql.NullTime
	err := s.Scan(
		&sub.ID, &sub.Name, &sub.Type, &sub.URL, &urlsJSON, &sub.FetchProxyURL, &sub.FailurePolicy, &sub.FetchMode,
		&sub.RawContent, &sub.UserAgent,
		&sub.InsecureSkipVerify, &sub.AutoUpdate, &sub.IntervalHours, &sub.NodeCount,
		&sub.UserInfo, &sub.LastUpdated, &lastAttemptAt, &sub.ConsecutiveFailures,
		&sub.Status, &sub.ErrorMsg, &cachedJSON,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lastAttemptAt.Valid {
		sub.LastAttemptAt = lastAttemptAt.Time
	} else {
		sub.LastAttemptAt = sub.LastUpdated
	}
	if urlsJSON != "" {
		_ = json.Unmarshal([]byte(urlsJSON), &sub.URLs)
	}
	if len(sub.URLs) == 0 && strings.TrimSpace(sub.URL) != "" {
		sub.URLs = []string{sub.URL}
	}
	if cachedJSON != "" {
		_ = json.Unmarshal([]byte(cachedJSON), &sub.CachedNodes)
	}
	// Backfill stable selection keys so per-node selection survives name collisions.
	for _, n := range sub.CachedNodes {
		n.EnsureSelectionKey()
	}
	return &sub, nil
}

// Subscription methods (Remote & Local)
func (d *DB) ListSubscriptions(subType string) ([]*models.Subscription, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := "SELECT " + subscriptionSelectCols + " FROM subscriptions"
	var args []interface{}
	if subType != "" {
		query += " WHERE type = ?"
		args = append(args, subType)
	}
	query += " ORDER BY created_at DESC"

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*models.Subscription
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

func (d *DB) GetSubscription(id string) (*models.Subscription, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	row := d.db.QueryRow("SELECT "+subscriptionSelectCols+" FROM subscriptions WHERE id = ?", id)
	return scanSubscription(row)
}

func (d *DB) CreateSubscription(s *models.Subscription) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	s.ID = uuid.New().String()
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	if s.Type == "" {
		s.Type = models.SubscriptionTypeRemote
	}
	if s.Status == "" {
		s.Status = "ok"
	}
	s.LastUpdated = time.Now()
	s.LastAttemptAt = s.LastUpdated
	if len(s.URLs) == 0 && strings.TrimSpace(s.URL) != "" {
		s.URLs = []string{strings.TrimSpace(s.URL)}
	}
	if len(s.URLs) > 0 {
		s.URL = s.URLs[0]
	}
	if strings.TrimSpace(s.FailurePolicy) == "" {
		s.FailurePolicy = models.FetchFailureStrict
	}
	if strings.TrimSpace(s.FetchMode) == "" {
		s.FetchMode = models.FetchModeMerge
	}
	cachedJSON, _ := json.Marshal(s.CachedNodes)
	urlsJSON, _ := json.Marshal(s.URLs)

	_, err := d.db.Exec(
		`INSERT INTO subscriptions (id, name, type, url, urls_json, fetch_proxy_url, failure_policy, fetch_mode, raw_content, user_agent, insecure_skip_verify, auto_update, interval_hours, node_count, user_info, last_updated, last_attempt_at, consecutive_failures, status, error_msg, cached_nodes_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, string(s.Type), s.URL, string(urlsJSON), s.FetchProxyURL, s.FailurePolicy, s.FetchMode, s.RawContent, s.UserAgent, s.InsecureSkipVerify,
		s.AutoUpdate, s.IntervalHours, s.NodeCount, s.UserInfo, s.LastUpdated, s.LastAttemptAt,
		s.ConsecutiveFailures, s.Status, s.ErrorMsg, string(cachedJSON), s.CreatedAt, s.UpdatedAt,
	)
	return err
}

func (d *DB) UpdateSubscription(s *models.Subscription) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	s.UpdatedAt = time.Now()
	if len(s.URLs) > 0 {
		s.URL = s.URLs[0]
	}
	if strings.TrimSpace(s.FailurePolicy) == "" {
		s.FailurePolicy = models.FetchFailureStrict
	}
	if strings.TrimSpace(s.FetchMode) == "" {
		s.FetchMode = models.FetchModeMerge
	}
	cachedJSON, _ := json.Marshal(s.CachedNodes)
	urlsJSON, _ := json.Marshal(s.URLs)
	_, err := d.db.Exec(
		`UPDATE subscriptions SET name = ?, type = ?, url = ?, urls_json = ?, fetch_proxy_url = ?, failure_policy = ?, fetch_mode = ?, raw_content = ?, user_agent = ?, insecure_skip_verify = ?, auto_update = ?, interval_hours = ?, node_count = ?, user_info = ?, last_updated = ?, last_attempt_at = ?, consecutive_failures = ?, status = ?, error_msg = ?, cached_nodes_json = ?, updated_at = ? WHERE id = ?`,
		s.Name, string(s.Type), s.URL, string(urlsJSON), s.FetchProxyURL, s.FailurePolicy, s.FetchMode, s.RawContent, s.UserAgent, s.InsecureSkipVerify,
		s.AutoUpdate, s.IntervalHours, s.NodeCount, s.UserInfo, s.LastUpdated, s.LastAttemptAt,
		s.ConsecutiveFailures, s.Status, s.ErrorMsg, string(cachedJSON), s.UpdatedAt, s.ID,
	)
	return err
}

func (d *DB) UpdateSubscriptionCache(id string, nodes []*models.Node, status, errorMsg string) error {
	return d.UpdateSubscriptionCacheWithInfo(id, nodes, status, errorMsg, "")
}

func (d *DB) UpdateSubscriptionCacheWithInfo(id string, nodes []*models.Node, status, errorMsg, userInfo string) error {
	return d.UpdateSubscriptionCacheWithOptions(id, nodes, status, errorMsg, userInfo, false)
}

// UpdateSubscriptionCacheWithOptions persists refreshed nodes. When clearUserInfo is
// set an empty userInfo actively clears the stored quota header (multi-URL sources
// must not keep a stale single-source quota).
func (d *DB) UpdateSubscriptionCacheWithOptions(id string, nodes []*models.Node, status, errorMsg, userInfo string, clearUserInfo bool) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	now := time.Now()
	cachedJSON, _ := json.Marshal(nodes)
	userInfoAssignment := "user_info" // keep existing by default
	var userInfoArg interface{}
	args := []interface{}{len(nodes)}
	switch {
	case userInfo != "":
		userInfoAssignment = "?"
		userInfoArg = userInfo
	case clearUserInfo:
		userInfoAssignment = "?"
		userInfoArg = ""
	}
	if userInfoAssignment == "?" {
		args = append(args, userInfoArg)
	}
	args = append(args, now, now, status, errorMsg, string(cachedJSON), now, id)
	query := `UPDATE subscriptions SET node_count = ?, user_info = ` + userInfoAssignment +
		`, last_updated = ?, last_attempt_at = ?, consecutive_failures = 0, status = ?, error_msg = ?, cached_nodes_json = ?, updated_at = ? WHERE id = ?`
	_, err := d.db.Exec(query, args...)
	return err
}

// MarkSubscriptionFetchError records a failed upstream sync while preserving the last known good cached nodes and last_updated timestamp.
func (d *DB) MarkSubscriptionFetchError(id, errorMsg string) (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	now := time.Now()
	_, err := d.db.Exec(
		`UPDATE subscriptions SET last_attempt_at = ?, consecutive_failures = COALESCE(consecutive_failures, 0) + 1, status = 'error', error_msg = ?, updated_at = ? WHERE id = ?`,
		now, errorMsg, now, id,
	)
	if err != nil {
		return 0, err
	}
	var failures int
	_ = d.db.QueryRow("SELECT COALESCE(consecutive_failures, 0) FROM subscriptions WHERE id = ?", id).Scan(&failures)
	return failures, nil
}

// UpdateSubscriptionNodesLatency persists node probe results without modifying sync timestamps or error status.
func (d *DB) UpdateSubscriptionNodesLatency(id string, nodes []*models.Node) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	cachedJSON, err := json.Marshal(nodes)
	if err != nil {
		return err
	}
	_, err = d.db.Exec(
		`UPDATE subscriptions SET cached_nodes_json = ?, updated_at = ? WHERE id = ?`,
		string(cachedJSON), time.Now(), id,
	)
	return err
}

func (d *DB) DeleteSubscription(id string) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, err := d.db.Exec("DELETE FROM subscriptions WHERE id = ?", id)
	return err
}

const profileSelectCols = "id, name, token, subscription_ids_json, COALESCE(selected_nodes_json, '{}'), COALESCE(features_json, '{}'), deduplicate, COALESCE(filter_info_nodes, 0), COALESCE(only_healthy, 0), COALESCE(include_regex, ''), COALESCE(exclude_regex, ''), COALESCE(allowed_protocols_json, '[]'), COALESCE(sort_mode, ''), COALESCE(region_groups, 0), COALESCE(rule_preset, ''), COALESCE(rename_prefix, ''), COALESCE(rename_suffix, ''), COALESCE(rename_regex_search, ''), COALESCE(rename_regex_replace, ''), full_config_default, COALESCE(export_mode, 'full'), last_pull_at, COALESCE(last_pull_client, ''), COALESCE(last_pull_ip, ''), COALESCE(pull_count, 0), created_at, updated_at"

func normalizeProfileExportMode(p *models.Profile) {
	mode := strings.ToLower(strings.TrimSpace(p.ExportMode))
	if mode == "provider" {
		p.ExportMode = "provider"
		p.FullConfigDefault = false
	} else {
		p.ExportMode = "full"
		p.FullConfigDefault = true
	}
}

func scanProfile(s rowScanner) (*models.Profile, error) {
	var p models.Profile
	var subIDsJSON, selectedNodesJSON, featuresJSON, protocolsJSON string
	var lastPullAt sql.NullTime
	err := s.Scan(
		&p.ID, &p.Name, &p.Token, &subIDsJSON, &selectedNodesJSON, &featuresJSON, &p.Deduplicate,
		&p.FilterInfoNodes, &p.OnlyHealthy,
		&p.IncludeRegex, &p.ExcludeRegex, &protocolsJSON,
		&p.SortMode, &p.RegionGroups, &p.RulePreset,
		&p.RenamePrefix, &p.RenameSuffix, &p.RenameRegexSearch, &p.RenameRegexReplace,
		&p.FullConfigDefault, &p.ExportMode,
		&lastPullAt, &p.LastPullClient, &p.LastPullIP, &p.PullCount,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lastPullAt.Valid {
		t := lastPullAt.Time
		p.LastPullAt = &t
	}
	normalizeProfileExportMode(&p)
	if subIDsJSON != "" {
		_ = json.Unmarshal([]byte(subIDsJSON), &p.SubscriptionIDs)
	}
	if selectedNodesJSON != "" {
		_ = json.Unmarshal([]byte(selectedNodesJSON), &p.SelectedNodes)
	}
	if featuresJSON != "" {
		var f models.ProfileFeatures
		if err := json.Unmarshal([]byte(featuresJSON), &f); err == nil {
			p.SetFeatures(f)
		}
	}
	if protocolsJSON != "" {
		_ = json.Unmarshal([]byte(protocolsJSON), &p.AllowedProtocols)
	}
	return &p, nil
}

// Profile methods
func (d *DB) ListProfiles() ([]*models.Profile, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query("SELECT " + profileSelectCols + " FROM profiles ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []*models.Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

func (d *DB) GetProfileByID(id string) (*models.Profile, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	row := d.db.QueryRow("SELECT "+profileSelectCols+" FROM profiles WHERE id = ?", id)
	return scanProfile(row)
}

func (d *DB) GetProfileByToken(token string) (*models.Profile, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	row := d.db.QueryRow("SELECT "+profileSelectCols+" FROM profiles WHERE token = ?", token)
	return scanProfile(row)
}

func (d *DB) CreateProfile(p *models.Profile) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	p.ID = uuid.New().String()
	if p.Token == "" {
		p.Token = uuid.New().String()
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	normalizeProfileExportMode(p)

	subIDsJSON, _ := json.Marshal(p.SubscriptionIDs)
	selectedNodesJSON, _ := json.Marshal(p.SelectedNodes)
	featuresJSON, _ := json.Marshal(p.GetFeatures())
	protocolsJSON, _ := json.Marshal(p.AllowedProtocols)

	_, err := d.db.Exec(
		`INSERT INTO profiles (id, name, token, subscription_ids_json, selected_nodes_json, features_json, deduplicate, filter_info_nodes, only_healthy, include_regex, exclude_regex, allowed_protocols_json, sort_mode, region_groups, rule_preset, rename_prefix, rename_suffix, rename_regex_search, rename_regex_replace, full_config_default, export_mode, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Token, string(subIDsJSON), string(selectedNodesJSON), string(featuresJSON), p.Deduplicate,
		p.FilterInfoNodes, p.OnlyHealthy,
		p.IncludeRegex, p.ExcludeRegex, string(protocolsJSON),
		p.SortMode, p.RegionGroups, p.RulePreset,
		p.RenamePrefix, p.RenameSuffix, p.RenameRegexSearch, p.RenameRegexReplace,
		p.FullConfigDefault, p.ExportMode, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (d *DB) UpdateProfile(p *models.Profile) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	p.UpdatedAt = time.Now()
	normalizeProfileExportMode(p)
	subIDsJSON, _ := json.Marshal(p.SubscriptionIDs)
	selectedNodesJSON, _ := json.Marshal(p.SelectedNodes)
	featuresJSON, _ := json.Marshal(p.GetFeatures())
	protocolsJSON, _ := json.Marshal(p.AllowedProtocols)

	_, err := d.db.Exec(
		`UPDATE profiles SET name = ?, subscription_ids_json = ?, selected_nodes_json = ?, features_json = ?, deduplicate = ?, filter_info_nodes = ?, only_healthy = ?, include_regex = ?, exclude_regex = ?, allowed_protocols_json = ?, sort_mode = ?, region_groups = ?, rule_preset = ?, rename_prefix = ?, rename_suffix = ?, rename_regex_search = ?, rename_regex_replace = ?, full_config_default = ?, export_mode = ?, updated_at = ? WHERE id = ?`,
		p.Name, string(subIDsJSON), string(selectedNodesJSON), string(featuresJSON), p.Deduplicate,
		p.FilterInfoNodes, p.OnlyHealthy,
		p.IncludeRegex, p.ExcludeRegex, string(protocolsJSON),
		p.SortMode, p.RegionGroups, p.RulePreset,
		p.RenamePrefix, p.RenameSuffix, p.RenameRegexSearch, p.RenameRegexReplace,
		p.FullConfigDefault, p.ExportMode, p.UpdatedAt, p.ID,
	)
	return err
}

// RecordProfilePull updates pull telemetry for a profile without touching updated_at (keeping ETag stable).
func (d *DB) RecordProfilePull(id, clientFormat, clientIP string) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, err := d.db.Exec(
		`UPDATE profiles SET last_pull_at = ?, last_pull_client = ?, last_pull_ip = ?, pull_count = COALESCE(pull_count, 0) + 1 WHERE id = ?`,
		time.Now(), clientFormat, clientIP, id,
	)
	return err
}

func (d *DB) ResetProfileToken(id string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	newToken := uuid.New().String()
	_, err := d.db.Exec("UPDATE profiles SET token = ?, updated_at = ? WHERE id = ?", newToken, time.Now(), id)
	if err != nil {
		return "", err
	}
	return newToken, nil
}

func (d *DB) DeleteProfile(id string) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, err := d.db.Exec("DELETE FROM profiles WHERE id = ?", id)
	return err
}

func (d *DB) GetStats() (*models.DashboardStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var stats models.DashboardStats
	_ = d.db.QueryRow("SELECT COUNT(*) FROM subscriptions").Scan(&stats.TotalSubCount)
	_ = d.db.QueryRow("SELECT COUNT(*) FROM subscriptions WHERE type = 'remote'").Scan(&stats.RemoteSubCount)
	_ = d.db.QueryRow("SELECT COUNT(*) FROM subscriptions WHERE type = 'local'").Scan(&stats.LocalSubCount)
	_ = d.db.QueryRow("SELECT COUNT(*) FROM profiles").Scan(&stats.ProfileCount)

	var subNodesTotal int
	_ = d.db.QueryRow("SELECT COALESCE(SUM(node_count), 0) FROM subscriptions").Scan(&subNodesTotal)
	stats.TotalNodeCount = subNodesTotal
	return &stats, nil
}
