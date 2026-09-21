# Phase 9 — Docker & final sweep

**Goal:** `docker compose up` produces two non-empty containers and a reachable app.

Backend half only; the frontend image is the other track's job, but compose wires both.

---

## The CGO problem — settle it before writing the Dockerfile

`mattn/go-sqlite3` requires `CGO_ENABLED=1`, so the binary is **not** static. Pick one:

| Option | Build stage | Runtime stage | Note |
|---|---|---|---|
| **A (recommended)** | `golang:1.27-alpine` + `apk add gcc musl-dev`, `CGO_ENABLED=1` | `alpine:3.20` | musl on both sides, image ~20 MB |
| B | `golang:1.27` (glibc) | `debian:bookworm-slim` | bigger, fewer surprises |
| C | swap to a pure-Go driver | `CGO_ENABLED=0` | fully static + `scratch`, but not on the allowed-packages list — **don't** |

Mixing A's build with B's runtime is the classic failure: `exec format error` or
`no such file or directory` on a binary that plainly exists.

---

## Tasks

### `src/server/Dockerfile`
- [ ] Multi-stage. Build stage per the table above.
- [ ] `COPY go.mod go.sum ./ && go mod download` **before** copying source — layer caching.
- [ ] Runtime stage copies: the binary, `pkg/db/migrations/sqlite/`, and `ca-certificates`.
      **Forgetting the migrations is the #1 "works locally, empty DB in Docker" bug** — the
      migrator reads `.sql` files from disk at runtime, they aren't compiled in.
      (Alternative: `//go:embed` the migrations. Cleaner, but then verify golang-migrate is
      pointed at the embedded FS rather than a `file://` path.)
- [ ] Non-root user, `mkdir -p /app/data/uploads` owned by it.
- [ ] `EXPOSE 8080`.

### `docker-compose.yml` (currently empty)
- [ ] `backend`: build `./src/server`, port `8080:8080`, env from `.env`,
      named volume on `/app/data` so the DB and uploads survive `down`.
- [ ] `frontend`: build `./src/web`, port `3000:3000`, `depends_on: backend`.
- [ ] Shared network; the frontend reaches the backend at `http://backend:8080` from inside,
      `http://localhost:8080` from the browser. Two different URLs — a server-side env var and a
      public one. This trips everyone up once.
- [ ] Delete the empty root `Dockerfile` or repurpose it (design doc §6).

### Scripts (audit bonus)
- [ ] `scripts/build.sh` — build both images.
- [ ] `scripts/dev.sh` — run the Go server locally with a hot-reload loop.

### `.env.example`
- [ ] Every key the config validates, with safe defaults. It's empty right now.

---

## Final audit sweep

Walk [`audits/audit/README.md`](../audits/audit/README.md) top to bottom with two browsers open.
The backend-relevant ones most often failed:

- [ ] `docker ps -a` shows two containers, both non-zero size
- [ ] `sqlite3 <db>` from inside the container shows all tables, migrations applied
- [ ] Wrong password and wrong email both rejected, same message
- [ ] Duplicate registration → 409, not a 500
- [ ] Two browsers stay logged in as different users across refreshes
- [ ] Private profile invisible to a non-follower, visible after accept
- [ ] All three post privacy levels enforced **server-side** (test with curl, not the UI)
- [ ] Private post: only the chosen followers see it
- [ ] Group posts invisible to non-members
- [ ] A plain member can invite; only the creator accepts join requests
- [ ] Unconnected users cannot open a chat
- [ ] Three browsers: a DM reaches only the target
- [ ] Group message reaches all members in realtime
- [ ] Emoji round-trip
- [ ] All four notification types fire
- [ ] `go vet ./...` and `go test -race ./...` clean
- [ ] Only allowed packages in `go.mod` — `cat go.mod` and check every line against the list in
      `audits/README.md`

---

## Done when

```bash
docker compose up --build
curl localhost:8080/api/health
docker compose down && docker compose up   # data still there
```
