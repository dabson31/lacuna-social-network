# Architecture & Folder Structure

> Scope: this document explains the layout under `src/` — what each folder is for, why it
> exists where it does, and which alternatives were considered. Nothing here is code; it is
> the contract the code will be written against.

---

## 1. Constraints this layout has to satisfy

Straight from `audits/README.md` and `audits/audit/README.md`:

| Requirement | Where it is satisfied |
| --- | --- |
| "well-organized structure ... clear separation of packages and migrations folders" | `src/server/pkg/db/migrations/sqlite/` + `src/server/internal/` |
| "clear separation of responsibilities among Server, App, and Database" | `cmd/server` (Server) · `internal/` (App) · `pkg/db` (Database) |
| Migrations applied every time the app runs | `pkg/db/sqlite` opens the DB **and** runs the migrator |
| Sessions + cookies | `internal/platform/session` |
| Image upload (JPEG/PNG/GIF), file on disk + path in DB | `internal/platform/storage` → `data/uploads/` |
| WebSocket real-time chat | `internal/platform/realtime` + `internal/modules/chat` |
| Two Docker images | `src/server/Dockerfile` · `src/web/Dockerfile` |
| Frontend file system "well organized", JS framework | `src/web/` (Next.js App Router) |

Plus your own constraints: **Go for the backend**, **TypeScript where TS earns its place**,
**Next.js + React**, **everything under `src/`**, **no `backend/` + `frontend/` split**,
**functional modularity**.

---

## 2. Top level: why not `backend/` and `frontend/`

```
src/
├── server/   # Go — HTTP + WebSocket + SQLite. The thing that owns state.
├── web/      # Next.js — rendering, routing, user interaction.
└── shared/   # Language-neutral contracts both sides agree on.
```

`backend` / `frontend` names a folder after **where it runs**, which is the least interesting
thing about it — and it stops being true the moment Next.js does server-side work. It does:
`middleware.ts`, Server Components, and route handlers all execute on a server. You end up with
"backend code in the frontend folder", which is exactly the confusion the split was meant to
prevent.

Naming by **responsibility** avoids that:

- **`server/`** — the system of record. It owns the database, sessions, and the socket hub. It
  is the only thing allowed to be authoritative.
- **`web/`** — the delivery layer. It renders, guards routes, and talks to `server/`. It may run
  code in a Node process; that does not make it a backend, because it owns no state.
- **`shared/`** — the seam. Values and shapes that would otherwise be duplicated and drift.

This also maps 1:1 onto the two Docker images the audit asks for: one image per top-level
folder, with `shared/` copied into the web build.

---

## 3. `src/server/` — the Go side

```
src/server/
├── cmd/
│   ├── server/          # main.go — process entry point (the "Server")
│   └── migrate/         # small CLI: migrate up/down/force, seed
├── internal/
│   ├── app/             # composition root
│   │   └── middleware/  # HTTP middleware chain
│   ├── modules/         # ← functional modules, one folder per feature
│   │   ├── auth/
│   │   ├── users/
│   │   ├── follows/
│   │   ├── posts/
│   │   ├── groups/
│   │   ├── chat/
│   │   └── notifications/
│   ├── platform/        # cross-cutting capability, zero domain knowledge
│   │   ├── config/
│   │   ├── httpx/
│   │   ├── session/
│   │   ├── password/
│   │   ├── storage/
│   │   ├── realtime/
│   │   ├── events/
│   │   └── logger/
│   └── testutil/
├── pkg/
│   └── db/
│       ├── migrations/sqlite/   # 000001_*.up.sql / 000001_*.down.sql
│       ├── seed/                # optional dataset for local dev (audit bonus)
│       └── sqlite/              # sqlite.go — Open() + Migrate()
└── data/
    └── uploads/                 # runtime volume: user images
```

### 3.1 `cmd/` — one folder per binary

Standard Go layout. `cmd/server/main.go` does four things and nothing else: read config, open
the DB, build the app, listen. Keeping `main` this thin is what makes the Server / App /
Database separation the audit asks about *visible* rather than merely claimed.

`cmd/migrate/` exists because "migrations run automatically on boot" and "I need to roll one
back while developing" are different needs. Sharing one entry point forces you to choose; two
entry points over one `pkg/db` implementation cost about forty lines.

### 3.2 `internal/` — a boundary the compiler enforces

Go refuses to let anything outside `src/server/` import a package under `internal/`. That is not
a convention you have to police in review; it is a build error. The feature code lives there so
the module boundaries below are real rather than aspirational.

### 3.3 `internal/app/` — the composition root

`app/` is where dependencies are constructed and wired, and where the router mounts each
module's routes. It is the **only** package that knows every module exists.

Everything else receives what it needs as a constructor argument. That is what makes a module
testable in isolation: `posts` never reaches out for a database, it is handed one.

`app/middleware/` holds the request pipeline — panic recovery, request logging, CORS, the
session/auth guard, upload size limits. Middleware is cross-cutting by definition, so it does
not belong to any single module; it belongs to the thing that assembles them.

### 3.4 `internal/modules/` — functional modularity, the core idea

Each folder is one **feature**, sliced vertically, holding its whole stack:

```
modules/posts/
├── handler.go     # HTTP: decode request, call service, write response. No rules here.
├── service.go     # the actual rules: who may see an "almost private" post?
├── store.go       # SQL for this feature only
├── model.go       # types this feature owns
└── service_test.go
```

The alternative — `handlers/`, `services/`, `repositories/` at the top level — groups files by
what they *are* instead of what they are *about*. Adding "events can be edited" then means
touching three distant folders, and `handlers/` slowly becomes a twenty-file directory where
nothing sits near anything it relates to. Here a feature is a folder: you open it and the whole
feature is in front of you. Deleting a feature is `rm -r` on one directory.

**Module boundaries chosen deliberately:**

- **`auth`** — register, login, logout, session issuing. Owns the credential half of a user.
- **`users`** — profile read/update, public↔private toggle. Owns the profile half.
- **`follows`** — follow, unfollow, request accept/decline, follower and following lists. Split
  out of `users` because "can A see B?" is the most reused rule in the app — it gates profiles,
  posts, *and* chat — and deserves one owner rather than three copies.
- **`posts`** — posts, comments, and post privacy. Comments are not a separate module: they have
  no lifecycle of their own, share the privacy model, and are always fetched with their post. A
  `comments/` folder would be a folder that only ever changes when `posts` does.
- **`groups`** — group, membership, invitations, join requests, group posts, events, RSVPs. The
  largest module and the one most likely to be split later (see §7).
- **`chat`** — private conversations and group rooms, message persistence, delivery rules.
- **`notifications`** — creation, fan-out, read state.

**Modules do not import each other.** When `groups` needs to notify someone it publishes to
`platform/events`; `notifications` subscribes. This single rule keeps functional modularity from
decaying into a dependency ball, and it is why the four required notifications (follow request,
group invite, join request, event created) don't force `follows` and `groups` to know what a
notification is.

Where a module genuinely needs data from another — `posts` needs "is A following B?" — it
declares a **small interface in its own package**, and `app/` passes in the implementation. The
consumer defines the interface, not the provider, so `follows` never grows methods that exist
only to please `posts`.

### 3.5 `internal/platform/` — capability, not domain

If a package would still make sense in a completely different application, it goes here.

| Package | Why it is its own package |
| --- | --- |
| `config` | Env parsing in one place, validated at boot, so a missing variable fails on start rather than on first request. |
| `httpx` | JSON decode/encode, error→status mapping, validation helpers. Without it every handler reinvents error responses and they drift. |
| `session` | Server-side session store plus cookie handling (`HttpOnly`, `SameSite`, expiry). The audit tests two browsers staying independently logged in — that behaviour lives in exactly one file. |
| `password` | bcrypt hash and compare. Two functions, but isolating them makes the cost factor a one-line change and keeps bcrypt out of every other import list. |
| `storage` | Saves uploads to `data/uploads/`, sniffs the real content type (JPEG/PNG/GIF), rejects the rest, returns the path to store in the DB. Every feature with an image — posts, comments, avatars — uses this one path. |
| `realtime` | The WebSocket hub: connection registry, per-user fan-out, room broadcast. Transport only; it does not know what a "message" or an "event" means. |
| `events` | In-process pub/sub. The decoupler described above. |
| `logger` | Structured logging via `log/slog`, so request logs are greppable. |

`testutil/` holds fixtures and an in-memory SQLite helper. It sits under `internal/` so it can
never be imported by a real binary.

### 3.6 `pkg/db/` — the one folder shaped by the audit, on purpose

The subject shows `backend/pkg/db/migrations/sqlite/` and says *"do not forget that the
application of migrations and the file organization will be tested"*. That path is reproduced
verbatim, minus the `backend/` prefix that `server/` replaces.

Strictly, Go idiom would put this in `internal/platform/database/`: nothing outside this module
imports it, and `pkg/` conventionally means "intended for external import". This is a deliberate
deviation — an auditor scanning for a recognisable migrations layout should find it immediately,
and the cost is one folder name. Documented here rather than left as a silent inconsistency.

- `migrations/sqlite/` — numbered pairs, `NNNNNN_description.up.sql` and `.down.sql`. Every
  `.up.sql` gets a `.down.sql`, even a trivial one; a migration you cannot reverse is a
  migration you cannot develop against.
- `sqlite/sqlite.go` — `Open()` (connection plus pragmas: `foreign_keys=ON`, WAL, busy timeout)
  and `Migrate()` (applies pending migrations at startup). SQLite is single-writer, so having
  that pragma setup in one place is not cosmetic.
- `seed/` — a dataset that fills the database for local testing. Kept **out of**
  `migrations/sqlite/` so a clean run doesn't inherit fake users; invoke it explicitly through
  `cmd/migrate`.

### 3.7 `data/`

Runtime state: the `.db` file and `uploads/`. It is a separate top-level folder because it is the
Docker volume mount point — the one directory that must survive a container rebuild.
`data/uploads/` keeps a `.gitkeep`; its contents should be git-ignored:

```gitignore
src/server/data/*.db*
src/server/data/uploads/*
!src/server/data/uploads/.gitkeep
```

---

## 4. `src/web/` — the Next.js side

```
src/web/
├── app/                    # App Router: URLs only
│   ├── (auth)/login, register
│   ├── (main)/feed, profile/[userId], groups/[groupId], chat/[conversationId], notifications
│   └── api/                # optional same-origin BFF route handlers
├── features/               # ← mirrors internal/modules/ one for one
│   └── auth/ profile/ follows/ posts/ groups/ chat/ notifications/
│       ├── components/     # feature-specific UI
│       ├── hooks/          # feature-specific state
│       ├── api.ts          # the calls this feature makes
│       └── types.ts
├── components/
│   ├── ui/                 # primitives: Button, Modal, Avatar, Input
│   └── layout/             # shell: Navbar, Sidebar, NotificationBell
├── lib/
│   ├── api/                # fetch wrapper: base URL, credentials, error normalisation
│   ├── realtime/           # WebSocket client, reconnect, typed event dispatch
│   ├── session/            # reading session state on server and client
│   └── utils/
├── hooks/                  # genuinely global hooks
├── styles/
├── public/
└── middleware.ts           # TS route guard — runs before the page
```

### 4.1 `app/` is routing; `features/` is the application

This is the most important rule on this side. Files in `app/` stay short: read the params, check
auth, compose components from `features/`, stop.

The reason is that `app/`'s shape is dictated by URLs, and URLs change. A page moving from
`/groups/[id]` to `/g/[slug]` should be a folder rename, not a code migration. It also means a
component is never trapped inside a route folder where nothing else can reach it — the standard
failure mode of writing logic directly into App Router directories.

Route groups `(auth)` and `(main)` do not appear in the URL; they exist so the two halves of the
app can have different layouts — a bare centred card for login and register, the full shell
(navbar, notification bell, sidebar) for everything else. The audit requires notifications to be
visible **on every page**; putting the bell in `(main)/layout.tsx` makes that structural instead
of something you can forget to add to a new page.

### 4.2 `features/` mirrors `internal/modules/`

The same seven names on both sides, so a change request maps to one folder per side. Adding
group event editing means touching `internal/modules/groups/` and `features/groups/` — and you
can predict that before opening the repo.

`profile` is the web-side name for the server's `users` module: it is what the audit and the UI
both call it, and a folder should be named after the thing on screen.

### 4.3 `components/` versus `features/*/components/`

A component belongs in `components/ui/` only when it is **generic** — when it has no idea what a
post or a group is. `PostCard` knows what a post is, so it lives in
`features/posts/components/`. `Modal` does not, so it lives in `components/ui/`.

Getting this wrong in the other direction is what turns a shared components folder into a
150-file directory that everything imports and nobody can change safely.

`components/layout/` is the app shell — used by a layout, not by any single feature.

### 4.4 `lib/`, and where TypeScript actually pays off

`lib/` is infrastructure, not helpers. The distinction: something in `lib/` owns a concern —
HTTP transport, socket lifecycle, session reading. It is not a bucket for loose functions; that
is `lib/utils/`, and it should stay small.

- **`lib/api/`** — one fetch wrapper. It sets `credentials: "include"` (without which the session
  cookie the entire audit depends on is never sent), prefixes the API base URL, and turns
  non-2xx responses into a single typed error shape. Every feature's `api.ts` calls through it,
  so changing auth handling or the error format is one edit.
- **`lib/realtime/`** — the WebSocket client. One connection for the whole app, shared by chat
  *and* notifications, with reconnect and backoff. TypeScript does real work here: the server
  sends a tagged union of event types over a single socket, and a discriminated union on `type`
  makes the client's dispatch exhaustive at compile time. This is the part most likely to break
  silently, and types are the cheapest defence available.
- **`middleware.ts`** — Next.js middleware, TypeScript, at the `web/` root because the framework
  requires that exact location. It checks the session cookie and redirects unauthenticated users
  away from `(main)` routes before any page renders. It is a **UX guard, not a security
  boundary**: the Go server re-checks the session on every request. Treating it as authorization
  is the classic mistake here.
- **`app/api/`** — optional. Useful if you want a same-origin proxy so cookies stay first-party
  and the browser never sees the Go server's URL. Left empty; delete it if you call Go directly.

---

## 5. `src/shared/`

```
src/shared/
├── contracts/    # request/response shapes, WebSocket event union
└── constants/    # values both sides must agree on
```

`constants/` holds the values where a typo is a silent bug: post privacy
(`public` / `almost_private` / `private`), follow request states, notification types, WebSocket
event names. These are written by Go handlers and read by TS components; keeping the strings in
one place means a rename is caught by the TypeScript compiler rather than by a user staring at
an empty feed.

`contracts/` is hand-written TypeScript by default — the API is small enough that generation is
overhead. If it grows, see §7.

---

## 6. Docker mapping

Two images, as required:

| Image | Build context | Contains |
| --- | --- | --- |
| backend | `src/server/` | Multi-stage: build the Go binary, copy it into a small runtime image along with the migrations. |
| frontend | `src/web/` plus `src/shared/` | Multi-stage: install, build Next.js, run the standalone output. |

`docker-compose.yml` at the repo root wires the two, exposes the ports, and mounts
`src/server/data/` as a named volume so the database and uploads survive `docker compose down`.

One thing to settle before writing the Dockerfile rather than after: `mattn/go-sqlite3` requires
CGO, so the build stage cannot be fully static by default. Either build with `CGO_ENABLED=1` on
a base image whose libc matches the runtime image, or switch to a pure-Go SQLite driver.

The empty `Dockerfile` currently at the repo root is now redundant — replace it with the two
per-image Dockerfiles, or keep it as a thin build orchestrator.

---

## 7. Possible modifications and tweaks

What this layout deliberately leaves open, with the trade-off attached.

**Split `groups/` when it gets heavy.** It carries membership, invitations, join requests, posts,
and events. If `service.go` passes roughly 400 lines, split *within* the module first —
`service_members.go`, `service_events.go` — before splitting into separate `groups/` and
`events/` packages. Same-package splits are free; package splits introduce import-cycle risk.

**Merge `auth` into `users`, or keep them apart.** They share a table. They are separate here
because their reasons to change are unrelated: `auth` changes when session or password policy
changes, `users` when the profile shape does. If you find yourself constantly editing both
together, merge them — that means the boundary was wrong.

**Generate `shared/contracts` instead of hand-writing it.** Once the API stabilises, emit an
OpenAPI spec from Go and generate the TS types. Do it when hand-drift actually bites; doing it on
day one adds a build step to an API you are still changing hourly.

**Move `pkg/db` to `internal/platform/database`.** Strictly more idiomatic Go (§3.6). Worth doing
after the audit, and only if nothing depends on the documented path.

**Swap the in-process `events` bus.** It is a goroutine and a channel. That is correct for a
single-process app and would only need replacing if you ran more than one server instance — at
which point the WebSocket hub needs replacing too, since both hold state in memory.

**Add `web/tests/`.** Left out because Go tests co-locate (`service_test.go` beside
`service.go`, which is idiomatic and keeps them from rotting). Playwright specs do not co-locate
naturally — add `src/web/tests/e2e/` if you write them.

**Promote `features/*/api.ts` to a folder.** When it grows past a screenful, make it
`features/x/api/` with one file per resource. Folder-versus-file should follow size, not be
decided upfront.

**Adding an OAuth provider (audit bonus).** It fits inside `modules/auth/` as `handler_oauth.go`
and `service_oauth.go` — it issues the same session, so it is a second door into an existing
module, not a new module.

---

## 8. Conventions worth stating once

1. **A feature is a folder, on both sides, with the same name.**
2. **Modules never import modules.** Cross-feature contact goes through `platform/events`, or an
   interface the *consumer* declares.
3. **`main` and route files stay thin.** They wire and delegate; they never decide.
4. **Handlers hold no rules; services do; stores only do SQL.** If a handler contains an `if`
   about who may see something, it is in the wrong file.
5. **Authorization is decided in Go.** `middleware.ts` improves the experience; it protects
   nothing.
6. **`components/ui/` is domain-free.** If it imports a feature type, it is not a primitive.
7. **Every `.up.sql` has a `.down.sql`.**
8. **Nothing lands in `utils/` that could name its own concern.**

---

## 9. Deliberately absent

- **No `models/` or `entities/` package.** A single package holding every struct gets imported by
  everything, couples every module to every other, and becomes the file with the most merge
  conflicts. Types live with the module that owns them.
- **No top-level `handlers/` / `services/` / `repositories/`.** See §3.4.
- **No `utils/` on the Go side.** Every helper here has a real home — `httpx`, `password`,
  `storage`. A Go `utils` package is a symptom, not a design.
- **No global state store on the web side.** Server state belongs to the fetching layer; the
  small amount of genuinely shared client state (session, unread count, socket) is context in
  `lib/`. Add Zustand or Redux only when that stops being true.
