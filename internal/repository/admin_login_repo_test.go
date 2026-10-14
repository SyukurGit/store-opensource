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

	// 2. Record successful login
	err = repo.RecordLogin("admin", "180.244.10.20", "Mozilla/5.0 Windows", "Jakarta, ID", "Chrome on Windows", "SUCCESS", "Login berhasil")
	if err != nil {
		t.Fatalf("failed to record login: %v", err)
	}

	last, err = repo.GetLastLogin()
	if err != nil || last == nil {
		t.Fatalf("expected last login recorded, got err: %v", err)
	}
	if last.IPAddress != "180.244.10.20" || last.Status != "SUCCESS" || last.Location != "Jakarta, ID" || last.DeviceInfo != "Chrome on Windows" {
		t.Fatalf("unexpected last login data: %+v", last)
	}

	// 3. Record failed login attempt (brute force simulation)
	err = repo.RecordLogin("admin", "45.134.22.100", "Python-requests/2.28", "Moscow, RU", "Unknown on Linux", "FAILED", "Password salah")
	if err != nil {
		t.Fatalf("failed to record failed login: %v", err)
	}

	// 4. Test Security Stats
	stats, err := repo.GetSecurityStats()
	if err != nil {
		t.Fatalf("failed to get security stats: %v", err)
	}
	if stats.TotalSuccess != 1 {
		t.Fatalf("expected TotalSuccess=1, got %d", stats.TotalSuccess)
	}
	if stats.TotalFailed != 1 {
		t.Fatalf("expected TotalFailed=1, got %d", stats.TotalFailed)
	}
	if stats.LastFailedIP != "45.134.22.100" || stats.LastFailedUser != "admin" {
		t.Fatalf("unexpected LastFailed data: %+v", stats)
	}

	// 5. Test Filter by Status: FAILED
	failedLogs, totalFailed, err := repo.GetSecurityLogs("FAILED", "", "DESC", 10, 0)
	if err != nil || totalFailed != 1 || len(failedLogs) != 1 {
		t.Fatalf("expected 1 failed log, got err: %v, total: %d, len: %d", err, totalFailed, len(failedLogs))
	}
	if failedLogs[0].IPAddress != "45.134.22.100" || failedLogs[0].FailureReason != "Password salah" {
		t.Fatalf("unexpected failed log item: %+v", failedLogs[0])
	}

	// 6. Test Search Query: "Jakarta"
	jakartaLogs, totalJakarta, err := repo.GetSecurityLogs("ALL", "Jakarta", "DESC", 10, 0)
	if err != nil || totalJakarta != 1 || len(jakartaLogs) != 1 {
		t.Fatalf("expected 1 Jakarta log, got total: %d", totalJakarta)
	}
	if jakartaLogs[0].Location != "Jakarta, ID" {
		t.Fatalf("expected Jakarta location, got: %s", jakartaLogs[0].Location)
	}

	// 7. Test Order ASC vs DESC
	ascLogs, _, err := repo.GetSecurityLogs("ALL", "", "ASC", 10, 0)
	if err != nil || len(ascLogs) < 2 {
		t.Fatalf("expected at least 2 logs in ASC order, got: %v", err)
	}
	if ascLogs[0].ID > ascLogs[1].ID {
		t.Errorf("expected ASC order (id[0] < id[1]), got %d > %d", ascLogs[0].ID, ascLogs[1].ID)
	}
}
