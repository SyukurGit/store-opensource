package database

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrationsUpgradeOldDatabase(t *testing.T) {
	// 1. Create in-memory or temp SQLite DB
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	// 2. Pre-create old admin_login_logs table WITHOUT 'status' and 'failure_reason'
	oldTableSchema := `CREATE TABLE admin_login_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL,
		ip_address TEXT NOT NULL,
		user_agent TEXT NOT NULL,
		location TEXT DEFAULT '',
		device_info TEXT DEFAULT '',
		login_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(oldTableSchema); err != nil {
		t.Fatalf("failed to create old table: %v", err)
	}

	// 3. Run RunMigrations - this must succeed without error
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed on old database: %v", err)
	}

	// 4. Test insert with status & failure_reason to ensure columns and indexes exist
	_, err = db.Exec(`INSERT INTO admin_login_logs (username, ip_address, user_agent, status, failure_reason)
		VALUES ('testadmin', '127.0.0.1', 'Mozilla/5.0', 'FAILED', 'Password salah')`)
	if err != nil {
		t.Fatalf("failed to insert into migrated table: %v", err)
	}
}
