package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func InitDB(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// SQLite connection pooling: WAL mode allows concurrent readers and 1 writer
	db.SetMaxOpenConns(1)

	// Execute WAL and concurrency pragmas
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA foreign_keys = ON;",
	}

	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			log.Printf("Warning: failed to execute %s: %v", p, err)
		}
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	// Run migrations
	if err := RunMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Printf("SQLite database connected successfully at %s", dbPath)
	return &DB{db}, nil
}
