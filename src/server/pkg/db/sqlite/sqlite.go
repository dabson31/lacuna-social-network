package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	// "github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
)

func Open(path string) (*sql.DB, error) {
	var (
		// the owner can RWE, group and others can only read and write
		permission os.FileMode = 0o755

		// because FKs are off by default, else the program would not work
		fk = "?_foreign_keys=on"

		//! journaling or write-ahead logging method:
		// first writes down a summary of the actions to be performed into a "log" before actually writing
		// it into the database. SQLite uses journaling mechanism, possibly blocking readers and writers
		// WAL allows users to read while a writer is still writing
		//instead of modifying the db directly, writes are recorded in app.db-wal, then merges it into the main db
		//? once a change is acknowledged, it will persist and replay logged changes after recovery
		journal = "&_journal_mode=WAL"

		// if a write request and the db is closed, the lock forces a 5 seconds wait out
		// if this isnt implemented, then SQLite implements a BUSY; an err wont be passed in this case
		// if the write "lock" is reserved by another user, it waits until it has been unlocked
		// and it only looks for operations like INSERT or UPDATE. read doesnt count here
		// after 5 seconds it fails.
		timeout = "&_busy_timeout=5000"

		// SQLite supports multiple concurrent read transactions coming from different db connections, using separate threads and processes (goroutine, in this case)
		// however, it can only support one simultaneous write operation/transaction
		// so, in this case, when the program receives the message typed by the user, the handler requests to write from DB.
		// if the DB is locked, then it waits for 5 seconds before giving out an error message, else it waits for the write lock: "BEGIN_IMMEDIATE"
		txLock = "&_txlock=immediate"
	)

	// MkdirAll creates the entire directory path if needed, while mkdir falls apart if the parent directory isnt there
	// this helps if the folder structure were to be tampered with or when using Docker
	//todo: this has to be checked, actually check for only "path" in argument
	if err := os.MkdirAll(filepath.Dir(path), permission); err != nil {
		return nil, fmt.Errorf("Error: create db dir; %w", err)
	}

	// concat everything for the 
	dsn := path + fk + journal + timeout + txLock

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("Error: open sqlite; %w", err)
	}

	// db.Open is used to create a new database handle without establishing a connection
	// db.Ping is used to verify that the connection to the database is alive and operational
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("Error: ping sqlite; %w", err)
	}

	return db, nil
}

func NewMigrator(db *sql.DB, dir string) (*migrate.Migrate, error) {
	// todo: basically we need to handle the escape characters like %10 or whatever, since it takes it from the url and not the local dir
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("Error: resolve migrations dir: %v", err)
	}

	// here, we're using the same the pool instead of creating a new one
	driver, err := migratesqlite.WithInstance(db, &migratesqlite.Config{})
	if err != nil {
		return nil, fmt.Errorf("Error: migrate server; %v", err)
	}

	// this motherfucking shit cannot be "files"; has to be "file"
	migrate, err := migrate.NewWithDatabaseInstance("file://"+abs, "sqlite3", driver)
	if err != nil {
		return nil, fmt.Errorf("Error: migrate init: %v", err)
	}

	return migrate, nil
}

// todo: handle this
func Migrate(db *sql.DB, dir string, log *slog.Logger) error {
	m, err := NewMigrator(db, dir)
	if err != nil {
		return fmt.Errorf("Error: There was an error with using the NewMigrator in Migrate; %v", err)
	}

	before, dirty, err := m.Version()
	// ErrNilVersion only refers to a new database; doesn't refer to a failure
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("Error: migration version; %v", err)
	}

	// "dirty" is basically if the migration failed halfway through
	// so if it does fails, run the previous version (version - 1)
	if dirty {
		return fmt.Errorf("Error: database is dirty (stinky db) at version; %d.\nTo fix the SQL, run: go run ./src/server/cmd/migrate force %d.\n%v", before, before-1, err)
	}

	// ErrNoChange shows up with every normal restart, meaning it shouldnt be treated
	// as a nornmal error; the server wouldnt run otherwise
	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		// this means that there is no error with the migration
		//todo: check for err later
		log.Info("migration is up to date; %w", err)
		return nil
	}

	if err != nil {
		return fmt.Errorf("migrate up; %w", err)
	}

	after, _, _ := m.Version()
	log.Info("migration applied", "from", "before", "to", after)
	return nil
}
