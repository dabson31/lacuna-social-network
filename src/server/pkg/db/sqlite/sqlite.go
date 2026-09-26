package sqlite

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	// "github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/v4"
)

func Open(path string) (*sql.DB, error) {

	var (
		permission os.FileMode = 0o755 // the owner can RWE, group and others can only read and write
		fk = "?_foreignkeys=on"	// because FKs are off by default
		//! journaling or write-ahead logging method: 
		// first writes down a summary of the actions to be performed into a "log" before actually writing
		// it into the database. SQLite uses journaling mechanism, possibly blocking readers and writers 
		// WAL allows users to read while a writer is still writing 
		//! instead of modifying the db directly, writes are recorded in app.db-wal, then merges it into the main db
		journal = "&_journal_mode=WAL"	
		// if a write request and the db is closed, the lock forces a 5 seconds wait out
		// if this isnt implemented, then SQLite implements a BUSY; an err wont be passed in this case
		timeout = "&_busy_timeout=5000"
		// 
		txLock = "&_txlock=immediate"
	)

	// MkdirAll creates the entire directory path if needed, while mkdir falls apart if the parent directory isnt there
	// this helps if the folder structure were to be tampered with or when using Docker
	if err := os.MkdirAll(path, permission); err != nil {
		return nil, fmt.Errorf("Error: create db dir; %w", err)
	}

	dsn := path + fk + journal + timeout + txLock


	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("Error: open sqlite; %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("Error: ping sqlite; %w", err)
	}

	return db, nil
}

func NewMigrator(db *sql.DB, dir string) (*migrate.Migrate, error) {


	return nil, nil
}

func Migrate(db *sql.DB, dir string, log *slog.Logger) error {


	return nil
}
