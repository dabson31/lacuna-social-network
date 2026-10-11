// Package testutil holds fixtures for tests. it sits under internal/ so no real binary can
// ever import it by accident
package testutil

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"testing"

	"social/src/server/pkg/db/sqlite"
)

// NewDB opens a throwaway database and runs every migration on it, going through the
// project's own Open and Migrate so the test exercises the real startup path
// ! a temp file and not :memory: on purpose. database/sql keeps a pool of connections and
// ! every connection to :memory: gets its OWN empty database, so a test writes on one
// ! connection and reads on another, sees nothing, and the failure looks like a broken query
func NewDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	// t.TempDir cleans up the file itself, this just closes the handle
	t.Cleanup(func() { db.Close() })

	if err := sqlite.Migrate(db, migrationsDir(t), Discard()); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return db
}

// InsertUser puts a minimal user row in, enough to satisfy the foreign keys
// ! foreign_keys is ON in Open(), so inserting a message from a user id that doesnt exist
// ! fails with a FK error instead of quietly working. every test that touches a store needs
// ! its users to actually be there
func InsertUser(t *testing.T, db *sql.DB, id int64, firstName string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO users (id, email, password_hash, first_name, last_name, date_of_birth)
		 VALUES (?, ?, 'x', ?, 'Test', '2000-01-01')`,
		id, fmt.Sprintf("user%d@test.local", id), firstName)
	if err != nil {
		t.Fatalf("insert user %d: %v", id, err)
	}
}

// migrationsDir walks up from this source file to the repo root, so a test finds the .sql
// files no matter which directory go test happens to run it from
// ? runtime.Caller(0) gives the path of THIS file at compile time, which is the only
// ? reliable anchor, os.Getwd() would give the package being tested instead
func migrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate testutil source file")
	}
	// .../src/server/internal/testutil/db.go, so four levels up is the repo root
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
	return filepath.Join(root, "src", "server", "pkg", "db", "migrations", "sqlite")
}

// Discard is a logger for the components that insist on having one, a passing test should
// be silent
func Discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
