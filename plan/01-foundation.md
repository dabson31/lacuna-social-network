# Phase 1 — Foundation

**Goal:** `go run ./src/server/cmd/server` boots, creates `data/social.db`, applies migrations,
and answers `GET /api/health` with `{"status":"ok"}`.

Nothing domain-specific here. This is the plumbing every later phase assumes.

---

## Tasks

### 1. Settle the module path
- [x] Decide: `go.mod` stays at repo root (`module social`, imports `social/src/server/...`)
      **or** moves to `src/server/` (`module social/server`).
- [ ] If moving: `mv go.mod src/server/` and re-run `go mod tidy` from there.
- [ ] Do this before any other file exists. Changing it later is a repo-wide find/replace.

### 2. `internal/platform/config`
- [ ] `Config` struct: `Port`, `DBPath`, `UploadDir`, `SessionTTL`, `Env`, `AllowedOrigin`.
- [ ] `Load() (Config, error)` reads env vars with defaults, **validates at boot**, returns error
      on anything missing or malformed.
- [ ] Fill `.env.example` (currently empty) with every key and a safe default.

> Why validate at boot: a missing var should kill the process on start, not produce a nil-pointer
> panic on someone's first login three hours later.

### 3. `internal/platform/logger`
- [ ] Thin wrapper over `log/slog`. JSON handler in prod, text handler in dev.
- [ ] Return a `*slog.Logger` that `app/` passes down. No package-level global.

### 4. `pkg/db/sqlite/sqlite.go`
- [ ] `Open(path string) (*sql.DB, error)` — opens with these pragmas, all of them:
  ```
  _foreign_keys=ON      FK constraints are OFF by default in SQLite. Non-negotiable.
  _journal_mode=WAL     Lets reads proceed during a write. Required for websockets + HTTP together.
  _busy_timeout=5000    SQLite is single-writer; without this concurrent writes return SQLITE_BUSY.
  ```
- [ ] `db.SetMaxOpenConns(1)` for writes is the crude fix if you still hit `database is locked`.
      Try WAL + busy_timeout first.
- [ ] `Migrate(db *sql.DB, migrationsPath string) error` — runs `golang-migrate` up, logs how
      many migrations were applied, returns nil on `ErrNoChange`.
- [ ] Create the parent dir of `DBPath` if it doesn't exist.

### 5. `internal/platform/httpx`
- [ ] `WriteJSON(w, status, v)` and `WriteError(w, status, code, msg)`.
- [ ] One error envelope, used everywhere: `{"error": {"code": "...", "message": "..."}}`.
- [ ] `DecodeJSON(r, &dst)` with `DisallowUnknownFields()` and a body size limit.
- [ ] A small `ValidationErrors map[string]string` type for field-level form errors — registration
      needs it immediately.

> Define the envelope now. If you don't, every handler invents its own and the frontend ends up
> with seven error-parsing branches.

### 6. `internal/app`
- [ ] `New(cfg, db, logger, ...) http.Handler` — the composition root. Builds stores → services →
      handlers, mounts routes, wraps the middleware chain.
- [ ] `routes.go` with one `mountX()` call per module. Empty stubs for now.

### 7. `internal/app/middleware`
- [ ] `Recover` — catch panics, log with stack, return 500. **Write this first**; a panic in a
      chat handler must not kill the process mid-audit.
- [ ] `RequestLog` — method, path, status, duration.
- [ ] `CORS` — reads `AllowedOrigin` from config, and **must** set
      `Access-Control-Allow-Credentials: true`. Without it the browser silently drops the session
      cookie and every auth test fails in a way that looks like a backend bug.
- [ ] `MaxBodyBytes` — cap uploads.
- [ ] `RequireAuth` — stub now, filled in Phase 3.

### 8. Rewrite `cmd/server/main.go`
Currently it parses `os.Args` and serves a nil handler. Replace with exactly four steps:
- [ ] `config.Load()`
- [ ] `sqlite.Open()` +zzzz
### 10. Gitignore the runtime artifacts
Append to `.gitignore` (design doc §3.7):
```gitignore
src/server/data/*.db*
src/server/data/uploads/*
!src/server/data/uploads/.gitkeep
```

---

## Done when

```bash
go run ./src/server/cmd/server
curl -i localhost:8080/api/health          # 200 {"status":"ok"}
sqlite3 src/server/data/social.db ".tables" # shows schema_migrations
```
Kill it, run it again — no error, no duplicate migration.
