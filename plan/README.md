# Backend Plan — start here

Private working folder (gitignored). Frontend is out of scope for everything in here.

**Stack:** Go 1.27 · SQLite · stdlib `net/http` · gorilla/websocket
**Target:** every checkbox in [`audits/audit/README.md`](../audits/audit/README.md), backend half.
**Layout contract:** [`Docs/Architectural Design.md`](../Docs/Architectural%20Design.md) — already decided, don't re-litigate it.

---

## Where the repo actually is right now

- Folder tree exists, every leaf is a `.gitkeep`. Zero Go code except `cmd/server/main.go`.
- `main.go` starts a server with a nil handler and no routes. It gets rewritten in Phase 1.
- `go.mod` = `module social`, no dependencies yet.
- `Dockerfile`, `docker-compose.yml`, `.env.example` are empty files.

So: nothing to undo, everything to build.

---

## Lock these five decisions before writing code

They cost an hour to change on day 1 and a weekend on day 20. Recommendation given; write your
final answer in the blank and move on.

| # | Decision | Recommended | Why | Chosen |
|---|---|---|---|---|
| 1 | SQLite driver | `mattn/go-sqlite3` | On the allowed-packages list. Needs `CGO_ENABLED=1`, which constrains the Dockerfile (§6 of the design doc). | ______ |
| 2 | Migrations | `golang-migrate` | Allowed, reads `file://` dirs, matches the layout the audit checks. | ______ |
| 3 | Router | stdlib `http.ServeMux` | Go 1.22+ does `"POST /api/posts/{id}"` natively. No third-party router is on the allowed list anyway. | ______ |
| 4 | Primary keys | `INTEGER PRIMARY KEY AUTOINCREMENT` | SQLite's native rowid, fast joins. UUIDs only for session tokens. | ______ |
| 5 | Default profile privacy | private (`is_public = 0`) | The request→accept flow is the main thing audited; default-private exercises it. | ______ |

Dependencies to add now, all at once:

```bash
cd src/server   # note: go.mod is at repo root, so run from root if you keep it there
go get github.com/mattn/go-sqlite3
go get github.com/golang-migrate/migrate/v4
go get github.com/gorilla/websocket
go get golang.org/x/crypto/bcrypt
go get github.com/google/uuid
```

> `go.mod` is currently at the **repo root** with `module social`. Either keep it there
> (imports look like `social/src/server/internal/...`) or move it to `src/server/` with
> `module social/server`. Decide in Phase 1, task 1 — it touches every import line.

---

## Phase order

Build **one vertical slice end-to-end first**, then widen. Phases 1–3 are the slice: after
Phase 3 you can register, log in, stay logged in across a restart, and hit an authenticated
endpoint. Everything after that is the same shape repeated.

| Phase | File | Ships | Rough size |
|---|---|---|---|
| 1 | [01-foundation.md](01-foundation.md) | config, logger, httpx, DB open + migrate on boot, `/api/health`, `cmd/migrate` | 1 sitting |
| 2 | [02-database.md](02-database.md) | all 15 migration pairs, schema frozen | 1 sitting |
| 3 | [03-auth.md](03-auth.md) | bcrypt, sessions, cookies, register/login/logout/me, auth middleware | 2 sittings |
| 4 | [04-users-follows.md](04-users-follows.md) | profiles, public/private toggle, follow requests, the `CanView` rule | 2 sittings |
| 5 | [05-posts.md](05-posts.md) | posts, 3 privacy levels, comments, image upload | 2 sittings |
| 6 | [06-groups.md](06-groups.md) | groups, invites, join requests, group posts, events, RSVP | 3 sittings |
| 7 | [07-realtime-chat.md](07-realtime-chat.md) | WebSocket hub, DMs, group rooms | 2 sittings |
| 8 | [08-notifications.md](08-notifications.md) | event bus, the 4 required notifications, read state | 1 sitting |
| 9 | [09-docker-polish.md](09-docker-polish.md) | two images, compose, seed data, final audit sweep | 1 sitting |

Reference, not a phase: [api-contract.md](api-contract.md) — every endpoint in one table.

---

## The rules that keep this from rotting

Straight from the design doc §8; repeated here because these are the ones you'll break under
time pressure:

1. **Handlers hold no rules. Services hold rules. Stores hold only SQL.** An `if` about who may
   see something inside a handler means it's in the wrong file.
2. **Modules never import modules.** Cross-module contact = `platform/events` (fire-and-forget)
   or an interface the *consumer* declares and `app/` wires.
3. **Every `.up.sql` has a `.down.sql`.**
4. **Authorization is decided in Go**, always, on every request.
5. `foreign_keys = ON` — SQLite ignores FK constraints without it. Set it in `Open()`.

---

## Your first 90 minutes

1. Settle the five decisions above. (10 min)
2. `go get` the five dependencies. (5 min)
3. Phase 1, tasks 1–4: config → logger → sqlite.Open → migrate on boot. (45 min)
4. Write migration `000001_create_users_table` and boot the server. (20 min)
5. `sqlite3 src/server/data/social.db ".schema"` — see `users` and `schema_migrations`. (10 min)

That last command is the audit's own smoke test. Passing it on day 1 means the riskiest
plumbing in the project is already behind you.

---

## Progress

- [ ] Phase 1 — Foundation
- [ ] Phase 2 — Database schema
- [ ] Phase 3 — Auth
- [ ] Phase 4 — Users & follows
- [ ] Phase 5 — Posts
- [ ] Phase 6 — Groups
- [ ] Phase 7 — Realtime & chat
- [ ] Phase 8 — Notifications
- [ ] Phase 9 — Docker & polish
