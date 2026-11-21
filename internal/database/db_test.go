package database

import (
	"path/filepath"
	"testing"

	"subdeck/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func TestDBSecretProfileAndBackupRestore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "subs.db")

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// 1. Persisted random JWT secret
	s1, err := db.GetOrCreateJWTSecret()
	if err != nil || len(s1) != 64 {
		t.Fatalf("unexpected JWT secret: len=%d err=%v", len(s1), err)
	}
	s2, err := db.GetOrCreateJWTSecret()
	if err != nil || s1 != s2 {
		t.Fatalf("expected same persisted JWT secret, got %q vs %q (err=%v)", s1, s2, err)
	}

	// 2. Profile CRUD & GetProfileByID
	p := &models.Profile{
		Name:        "Test-Profile",
		Deduplicate: true,
	}
	if err := db.CreateProfile(p); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}
	fetched, err := db.GetProfileByID(p.ID)
	if err != nil || fetched.Name != "Test-Profile" {
		t.Fatalf("GetProfileByID failed: %+v err=%v", fetched, err)
	}

	// 3. BackupBytes
	backup, err := db.BackupBytes()
	if err != nil || len(backup) < 16 {
		t.Fatalf("BackupBytes failed: len=%d err=%v", len(backup), err)
	}

	// 4. Reject invalid restore data without breaking live DB
	if err := db.RestoreFromBytes([]byte("not a sqlite database")); err == nil {
		t.Fatalf("expected error restoring invalid bytes")
	}
	if err := db.RestoreFromBytes([]byte("SQLite format 3\x00corrupted")); err == nil {
		t.Fatalf("expected integrity error restoring corrupted sqlite bytes")
	}

	// 5. Modify DB then restore clean snapshot
	if err := db.DeleteProfile(p.ID); err != nil {
		t.Fatalf("DeleteProfile failed: %v", err)
	}
	if _, err := db.GetProfileByID(p.ID); err == nil {
		t.Fatalf("expected profile to be deleted before restore")
	}

	if err := db.RestoreFromBytes(backup); err != nil {
		t.Fatalf("RestoreFromBytes failed: %v", err)
	}

	restored, err := db.GetProfileByID(p.ID)
	if err != nil || restored.Name != "Test-Profile" {
		t.Fatalf("expected profile after restore, got %+v err=%v", restored, err)
	}
}

func TestSyncAdminUserWithConfig(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "admin.db")

	// 1. Fresh DB with custom admin user & password
	db, usedDefault, err := InitDBWithAdmin(dbPath, "opsadmin", "initial-pass-123")
	if err != nil {
		t.Fatalf("InitDBWithAdmin failed: %v", err)
	}
	if usedDefault {
		t.Fatalf("expected usedDefault=false when custom password is provided")
	}

	u, err := db.GetUserByUsername("opsadmin")
	if err != nil || u.Username != "opsadmin" {
		t.Fatalf("expected opsadmin user, got %+v err=%v", u, err)
	}

	// 2. Simulate password change via Web UI
	if err := db.UpdatePassword("opsadmin", "webui-changed-pass"); err != nil {
		t.Fatalf("UpdatePassword failed: %v", err)
	}
	_ = db.Close()

	// 3. Restart with unchanged config -> Web UI password must be preserved
	db2, _, err := InitDBWithAdmin(dbPath, "opsadmin", "initial-pass-123")
	if err != nil {
		t.Fatalf("reopen InitDBWithAdmin failed: %v", err)
	}
	u2, err := db2.GetUserByUsername("opsadmin")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u2.PasswordHash), []byte("webui-changed-pass")); err != nil {
		t.Fatalf("expected Web UI password to be preserved across restart: %v", err)
	}
	if err := db2.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 4. Restart with updated config -> username & password sync cleanly
	db3, _, err := InitDBWithAdmin(dbPath, "superadmin", "new-config-pass")
	if err != nil {
		t.Fatalf("reopen with changed config failed: %v", err)
	}
	defer db3.Close()
	u3, err := db3.GetUserByUsername("superadmin")
	if err != nil {
		t.Fatalf("expected updated username superadmin, got err=%v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u3.PasswordHash), []byte("new-config-pass")); err != nil {
		t.Fatalf("expected updated config password to take effect: %v", err)
	}
}

func TestProfileSelectedNodesRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "profile-nodes.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	p := &models.Profile{
		Name:            "picked",
		SubscriptionIDs: []string{"sub-a", "sub-b"},
		SelectedNodes:   map[string][]string{"sub-a": {"colo1-hy2", "colo1-tuic"}},
	}
	defer db.Close()
	if err := db.CreateProfile(p); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}

	got, err := db.GetProfileByID(p.ID)
	if err != nil {
		t.Fatalf("GetProfileByID failed: %v", err)
	}
	if len(got.SelectedNodes) != 1 {
		t.Fatalf("SelectedNodes lost on read: %+v", got.SelectedNodes)
	}
	names := got.SelectedNodes["sub-a"]
	if len(names) != 2 || names[0] != "colo1-hy2" || names[1] != "colo1-tuic" {
		t.Fatalf("SelectedNodes[sub-a] = %v, want [colo1-hy2 colo1-tuic]", names)
	}

	// Clearing the selection persists back to "whole sources" semantics.
	got.SelectedNodes = nil
	if err := db.UpdateProfile(got); err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}
	reloaded, err := db.GetProfileByID(p.ID)
	if err != nil {
		t.Fatalf("re-read failed: %v", err)
	}
	if len(reloaded.SelectedNodes) != 0 {
		t.Fatalf("cleared SelectedNodes came back: %+v", reloaded.SelectedNodes)
	}
}
