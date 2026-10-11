package repository

import (
	"path/filepath"
	"testing"

	"storeku/internal/database"
)

func TestAdminLoginRepository(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repo := NewAdminLoginRepository(db)

	// 1. Initial check when empty
	last, err := repo.GetLastLogin()
	if err != nil {
		t.Fatalf("unexpected error when empty: %v", err)
	}
	if last != nil {
		t.Fatalf("expected nil last login when empty")
	}

	// 2. Record first login
	err = repo.RecordLogin("admin", "180.244.10.20", "Mozilla/5.0 Windows", "Jakarta, ID", "Chrome on Windows")
	if err != nil {
		t.Fatalf("failed to record login: %v", err)
	}

	last, err = repo.GetLastLogin()
	if err != nil || last == nil {
		t.Fatalf("expected last login recorded, got err: %v", err)
	}
	if last.IPAddress != "180.244.10.20" || last.Location != "Jakarta, ID" || last.DeviceInfo != "Chrome on Windows" {
		t.Fatalf("unexpected last login data: %+v", last)
	}

	// 3. Record second login
	err = repo.RecordLogin("admin", "114.122.5.1", "Mozilla/5.0 Android", "Surabaya, ID", "Chrome on Android")
	if err != nil {
		t.Fatalf("failed to record second login: %v", err)
	}

	last, err = repo.GetLastLogin()
	if err != nil || last == nil {
		t.Fatalf("failed to get last login: %v", err)
	}
	if last.IPAddress != "114.122.5.1" || last.Location != "Surabaya, ID" {
		t.Fatalf("expected Surabaya login as most recent, got: %+v", last)
	}

	// 4. History list
	history, err := repo.GetLoginHistory(5)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got: %d", len(history))
	}
	if history[0].IPAddress != "114.122.5.1" {
		t.Fatalf("expected newest login first in history")
	}
}
