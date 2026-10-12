# Structure Walkthrough

How this repo is laid out, how a request moves through it, and why each boundary is where it is.

`Docs/Architectural Design.md` is the *contract* — it states the rules. This document is the
*explanation* — it shows the rules operating in the code that exists, names the failure each rule
prevents, and marks where the code has not caught up to the plan yet.

Read with `plan/README.md` open: its §"The rules that keep this from rotting" is the five-line
version of everything below.

---

## Table of contents

1. [The constraints that shaped the layout](#1-the-constraints-that-shaped-the-layout)
2. [Top level: naming by responsibility, not by location](#2-top-level-naming-by-responsibility-not-by-location)
3. [Server · App · Database — the three-way split](#3-server--app--database--the-three-way-split)
4. [Functional modularity: why a feature is a folder](#4-functional-modularity-why-a-feature-is-a-folder)
5. [Inside a module: the four files and the layer rule](#5-inside-a-module-the-four-files-and-the-layer-rule)
6. [How a request actually travels](#6-how-a-request-actually-travels)
7. [Why it is one-directional](#7-why-it-is-one-directional)
8. [Cross-module contact without cross-module imports](#8-cross-module-contact-without-cross-module-imports)
9. [`modules/` versus `platform/`: the deciding test](#9-modules-versus-platform-the-deciding-test)
10. [`pkg/db/` and `data/`: the two folders shaped by outside forces](#10-pkgdb-and-data-the-two-folders-shaped-by-outside-forces)
11. [The frontend: the same idea, different axis](#11-the-frontend-the-same-idea-different-axis)
12. [`shared/`: the seam between two languages](#12-shared-the-seam-between-two-languages)
13. [Where the code actually is right now](#13-where-the-code-actually-is-right-now)

---

## 1. The constraints that shaped the layout

Nothing here is a style preference. Four pressures produced this tree, and knowing which pressure
produced which folder is what lets you tell a load-bearing decision from an arbitrary one.

| Pressure | What it forced |
|---|---|
| The audit wants "clear separation of responsibilities among Server, App, and Database" | three distinct top-level Go folders: `cmd/`, `internal/`, `pkg/db/` |
| The audit wants a recognisable migrations folder, and says the file organisation is tested | `pkg/db/migrations/sqlite/` reproduced from the subject verbatim |
| The audit tests two browsers logged in independently, and three browsers in one chat | session handling in exactly one file; a socket hub that owns a connection registry |
| Two Docker images | one per top-level folder under `src/`, with `shared/` copied into the web build |

Plus self-imposed constraints: Go backend, Next.js frontend, everything under `src/`, no
`backend/` + `frontend/` split, functional modularity.

The useful consequence: when something in this tree looks un-idiomatic, there is a named reason,
and §10 is where the one deliberate deviation is written down rather than left to look like a
mistake.

---

## 2. Top level: naming by responsibility, not by location

```
src/
├── server/   # Go — owns the database, sessions, the socket hub. Authoritative.
├── web/      # Next.js — renders, routes, guards. Owns no state.
└── shared/   # Contracts both sides agree on.
```

**Why not `backend/` and `frontend/`.** Those names describe *where code runs*, which is both the
least interesting property of a file and — with Next.js — not even stable. `middleware.ts`, Server
Components, and route handlers all execute on a server. Name the folders by location and you
immediately own a folder called `frontend/` full of server code, which is precisely the confusion
the split was supposed to prevent.

Naming by **responsibility** survives that:

- **`server/`** is the system of record. It is the only thing allowed to be authoritative about
  whether you are logged in, whether you may see a post, or what the follower list is.
- **`web/`** is the delivery layer. It may run code in a Node process; that does not promote it to
  a backend, because it owns no state. Everything it displays it got by asking `server/`.
- **`shared/`** is the seam — the values that would otherwise be typed twice and drift apart.

The practical payoff is the authorization rule in §11: because `web/` owns no state, it is
structurally obvious that `middleware.ts` cannot be a security boundary. A `frontend/` folder makes
that an opinion you have to keep repeating in review.

---

## 3. Server · App · Database — the three-way split

```
src/server/
├── cmd/          ← the Server:   process lifecycle
│   ├── server/   │  main.go — boot, listen, shut down
│   └── migrate/  │  dev CLI — up/down/force/seed
├── internal/     ← the App:      everything that decides anything
└── pkg/db/       ← the Database: connection, pragmas, migrations, seed
```

### `cmd/` — one folder per binary

`cmd/server/main.go` does four things and refuses the fifth:

```go
cfg, err := config.Load()                       // 1. read and validate configuration
db, err := sqlite.Open(cfg.DBPath)               // 2. open the database
sqlite.Migrate(db, cfg.MigrationsPath, log)      // 3. bring the schema up to date
handler, err := app.New(app.Dependencies{...})   // 4. build the app — then listen
```
[`cmd/server/main.go:27-49`](../src/server/cmd/server/main.go#L27-L49)

**Why `main` stays this thin.** The Server/App/Database separation the audit asks about becomes
*visible* rather than merely claimed. You can read 93 lines and see all three. The moment `main`
starts constructing a store or deciding a route, the three-way split exists only in a diagram.

Everything else in `main.go` is lifecycle, not logic, and each piece is there for a stated reason:

- `ReadHeaderTimeout` — a client dribbling headers one byte at a time is a slowloris; without the
  timeout it holds a connection indefinitely.
- `IdleTimeout` — reclaim connections nobody is using.
- `ErrorLog: slog.NewLogLogger(...)` — `net/http`'s internal errors join the structured log instead
  of going to stderr in a different format.
- `signal.NotifyContext` + `server.Shutdown` — Ctrl-C sends SIGINT, `docker stop` sends SIGTERM then
  SIGKILL ten seconds later. Catching both means an in-flight request finishes instead of being cut
  mid-write.

**Why migrate runs before listening, not alongside it.** The comment at
[`main.go:40-41`](../src/server/cmd/server/main.go#L40-L41) is the reason: if the server accepted
connections while the schema was still moving, the first request could hit a table that does not
exist yet, and the failure would look like a bug in a handler.

**Why `cmd/migrate/` exists separately.** "Migrations apply automatically on boot" and "I need to
roll one back while developing" are different needs pulling in opposite directions. One entry point
forces you to choose; two entry points over one `pkg/db` implementation cost about forty lines.

### `internal/` — a boundary the compiler enforces

Go refuses to let anything outside `src/server/` import a package under `internal/`. That is not a
convention somebody has to police in review — it is a build error. Putting the feature code there
makes every module boundary below real rather than aspirational.

---

## 4. Functional modularity: why a feature is a folder

```
internal/modules/
├── auth/     ├── posts/        ├── chat/
├── users/    ├── groups/       └── notifications/
└── follows/
```

Each folder is one **feature, sliced vertically**, holding its whole stack from HTTP down to SQL.

### The alternative, and what it costs

The layered layout — top-level `handlers/`, `services/`, `repositories/` — groups files by **what
they are** instead of **what they are about**. Two things go wrong, both slowly:

1. **Every change fans out.** "Events can be edited" means touching three distant folders, and
   nothing in any of them sits near anything it relates to.
2. **The folders grow without bound.** `handlers/` becomes a twenty-file directory sorted
   alphabetically, so `group_handler.go` sits next to `follow_handler.go` for no reason other than
   the letter G.

Vertical slicing inverts both. A feature is a folder: you open `follows/` and the entire feature —
routes, rules, SQL, types — is in front of you. Deleting a feature is `rm -r` on one directory.
Adding one touches one directory plus a single line in `app/router.go`.

### Why these seven boundaries

The names are not arbitrary; each split is a bet about **what changes together**.

- **`auth`** — register, login, logout, session issuing. Owns the *credential* half of a user.
- **`users`** — profile read/update, the public↔private toggle. Owns the *profile* half.

  `auth` and `users` share the `users` table, which is usually an argument for merging them. They
  are apart because their *reasons to change* are unrelated: `auth` changes when password or session
  policy changes, `users` when the profile shape does. If you find yourself always editing both
  together, the boundary was wrong — merge them. That is the actual test, not table ownership.

- **`follows`** — follow, unfollow, accept/decline, follower and following lists. Split out of
  `users` because **"can A see B?" is the most reused rule in the application**: it gates profiles,
  posts, *and* who may open a chat. One owner beats three copies that drift. You can see the rule
  in its single home at [`follows/service.go:102-121`](../src/server/internal/modules/follows/service.go#L102-L121).
- **`posts`** — posts, comments, post privacy. **Comments are deliberately not a module:** they have
  no lifecycle of their own, share the post's privacy model, and are always fetched with their post.
  A `comments/` folder would be a folder that only ever changes when `posts` does — which is the
  definition of a boundary that is costing you more than it returns.
- **`groups`** — group, membership, invitations, join requests, group posts, events, RSVPs. The
  largest module, and the one most likely to split later. Note the escape hatch already in place:
  `service_members.go` sits beside `service.go`. **Split within the package before splitting the
  package** — a same-package split is free, a package split introduces import-cycle risk.
- **`chat`** — private conversations, group rooms, message persistence, delivery rules.
- **`notifications`** — creation, fan-out, read state.

---

## 5. Inside a module: the four files and the layer rule

```
modules/follows/
├── handler.go   # HTTP in, HTTP out. No rules.
├── service.go   # the rules. No HTTP, no SQL.
├── store.go     # SQL only. No rules.
└── model.go     # the types this feature owns
```

The four names repeat in every module, so the file you want is predictable before you open the
folder. What makes them more than a naming scheme is that **each one is defined by what it is
forbidden to contain.**

### `handler.go` — translation only

A handler reads the request, calls one service method, and maps the outcome to a status code:

```go
func (h *Handler) Follow(w http.ResponseWriter, r *http.Request) {
    viewerID, _ := session.UserID(r.Context())      // who is asking (put there by RequireAuth)
    targetID, err := idParam(r, "id")               // what they are asking about
    if err != nil { httpx.WriteError(w, 400, "bad_request", "user id must be a number"); return }

    res, err := h.svc.Follow(r.Context(), viewerID, targetID)   // the one call out
    if err != nil { h.writeErr(w, r, err); return }
    httpx.WriteJSON(w, http.StatusOK, res)
}
```
[`follows/handler.go:24-37`](../src/server/internal/modules/follows/handler.go#L24-L37)

The only `if`s are about *shape* — "is this a number?" — never about *permission*. The test from the
design doc: **an `if` in a handler about who may see something means the code is in the wrong
file**, because that rule is now unreachable from anywhere that is not an HTTP request, including a
test and a WebSocket path.

Handlers do get to hold one piece of real knowledge: the error→status mapping, in `writeErr`
([`handler.go:128-144`](../src/server/internal/modules/follows/handler.go#L128-L144)). That belongs
here precisely because status codes are an HTTP concept, and the service must not know them.

### `service.go` — the rules, and nothing that knows about the web

`follows/service.go` imports exactly one package: `context`. No `net/http`, no `database/sql`. That
single-line import block is the strongest available evidence that the layer is clean — you cannot
accidentally write a rule that depends on a request shape if the request type is not in scope.

```go
func (s *Service) Follow(ctx context.Context, followerID, targetID int64) (FollowResult, error) {
    if followerID == targetID { return FollowResult{}, errSelf }  // rule
    isPublic, err := s.store.targetState(ctx, targetID)           // ask the store for a fact
    if err != nil { return FollowResult{}, err }

    status := "pending"                                            // rule: private → needs approval
    if isPublic { status = "accepted" }                            // rule: public → auto-accept
    ...
}
```
[`follows/service.go:18-39`](../src/server/internal/modules/follows/service.go#L18-L39)

Note what the service returns: `errSelf`, a package sentinel — not a 400. It states *what is wrong
in domain terms* and leaves the translation to the layer whose job that is.

`Accept` shows the same shape with the authorization rule made explicit:

```go
if followeeID != viewerID || status != "pending" {
    return errNoRequest
}
```
[`follows/service.go:61-63`](../src/server/internal/modules/follows/service.go#L61-L63)

"Only the person the request was aimed at may accept it" and "a request you may not touch is
indistinguishable from one that does not exist" — one rule, one place.

### `store.go` — SQL and nothing else

Every method is a query plus a scan, and the only judgement it makes is translating *database*
conditions into *domain* sentinels:

```go
err = s.db.QueryRowContext(ctx, `SELECT is_public FROM users WHERE id = ?`, id).Scan(&isPublic)
if errors.Is(err, sql.ErrNoRows) {
    return false, errUserMissing          // "no rows" is a database fact; "no such user" is a domain fact
}
```
[`follows/store.go:32-41`](../src/server/internal/modules/follows/store.go#L32-L41)

Same at [`store.go:50-52`](../src/server/internal/modules/follows/store.go#L50-L52), where a UNIQUE
violation becomes `errAlready`. This conversion is the store's real contribution: above this line,
nothing needs to know that SQLite exists, so swapping the driver does not reach the service.

**Why the store reads the `users` table at all.** `follows/store.go` queries `users.is_public`, and
at a glance that looks like `follows` reaching into `users`' territory. It is not a module import —
it is one package reading one column of a shared schema. The rule being protected is *no Go
dependency between modules*; the database is a shared resource, and forcing this through a call into
a `users` service would create exactly the coupling §8 exists to prevent.

### `model.go` — the types this feature owns

`RequestItem`, `UserCard`, `FollowResult`, with their JSON tags. These are **response shapes, not
database rows**: `UserCard` is what a follower list entry looks like on the wire, which is why it
carries `IsPublic` (the UI needs it to pick a button) and not `password_hash`.

**Why there is no top-level `models/` package.** A single package holding every struct gets imported
by everything, which couples every module to every other through the type system and makes that one
file the permanent merge-conflict hotspot. Types live with the module that owns them; a type two
modules both need is a signal to look at §8, not to create `models/`.

---

## 6. How a request actually travels

Take `POST /api/auth/register` with an avatar — the one fully wired path in the repo today.

```
Browser
  │  POST /api/auth/register   (multipart/form-data, Origin: http://localhost:3000)
  ▼
┌──────────────────────────────────────────────────────────────────────┐
│ RequestLog      starts a timer, swaps in statusRecorder              │
│ ┌──────────────────────────────────────────────────────────────────┐ │
│ │ Recover       defers catchPanic                                  │ │
│ │ ┌──────────────────────────────────────────────────────────────┐ │ │
│ │ │ CORS        Origin matches? → Allow-Origin + Allow-Creds     │ │ │
│ │ │ ┌──────────────────────────────────────────────────────────┐ │ │ │
│ │ │ │ MaxBodyBytes   r.Body = MaxBytesReader(..., 10 MB)       │ │ │ │
│ │ │ │ ┌──────────────────────────────────────────────────────┐ │ │ │ │
│ │ │ │ │ mux        "POST /api/auth/register" → h.Register    │ │ │ │ │
│ │ │ │ └──────────────────────────────────────────────────────┘ │ │ │ │
│ │ │ └──────────────────────────────────────────────────────────┘ │ │ │
│ │ └──────────────────────────────────────────────────────────────┘ │ │
│ └──────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────┘
                              │
   auth.Handler.Register      │  ParseMultipartForm → RegisterInput
                              ▼
   auth.Service.Register         trim, validate, bcrypt, storage.Save, store.CreateUser
                              ▼
   auth.Store.CreateUser         INSERT INTO users ... → id
                              ▼
   session.Store.Create          INSERT INTO sessions ... → Session
                              ▲
   handler                      sessions.SetCookie(w, sess); WriteJSON(201, user)
                              ▲
   RequestLog                   logs method, path, status=201, duration
                              ▼
Browser   Set-Cookie: sid=...; HttpOnly; SameSite=Lax
```

### The chain, and why the order is what it is

The chain is assembled by nesting, not by a list — each middleware is a struct holding its settings
plus the `Next` handler it delegates to
([`app.go:77-89`](../src/server/internal/app/app.go#L77-L89)). Indentation in the source *is* the
request order.

1. **`RequestLog` outermost.** It must see the *final* status, including a 500 that `Recover`
   produced below it. Put it inside `Recover` and a panicking request logs nothing at all.
2. **`Recover` second.** It catches a panic from anything below — every middleware, the mux, and
   every handler. Without it, `net/http` recovers the panic itself and silently drops the
   connection: the client sees a broken socket and the log shows nothing useful.
3. **`CORS` before the mux.** A preflight is `OPTIONS`, and the mux has no `OPTIONS` route — it
   would answer 405 before any CORS header was set, and the real request would never be sent. CORS
   answers the preflight itself with 204 and returns ([`middleware.go:161-170`](../src/server/internal/app/middleware/middleware.go#L161-L170)).
4. **`MaxBodyBytes` last before routing.** It swaps the body for a capped reader, so the limit is in
   force no matter which handler runs.
5. **`mux`** resolves the route. Go 1.22+ patterns mean `"POST /api/auth/register"` and
   `"GET /uploads/{name}"` need no third-party router.

Three details in the chain are worth knowing because each fixes a bug that is hard to diagnose:

- **`statusRecorder` wraps the `ResponseWriter`** so `RequestLog` can read the status back. A
  handler that calls `Write` without `WriteHeader` gets an implicit 200, so the recorder fills that
  in too — otherwise those requests log `status=0`.
- **`statusRecorder` re-exposes `Hijack`**
  ([`middleware.go:46-52`](../src/server/internal/app/middleware/middleware.go#L46-L52)). `gorilla/websocket`
  upgrades a connection by type-asserting the writer to `http.Hijacker`. A wrapper that does not
  forward `Hijack` makes **every** WebSocket upgrade fail with "response does not implement
  http.Hijacker" — and the cause is in the logging middleware, nowhere near chat.
- **`Recover` sets `Connection: close`** before writing the 500. The panicking handler may have
  written a partial body; reusing that connection would splice the leftover bytes into the next
  response.

### `RequireAuth` is per-route, not in the chain

Note what is *absent* above: there is no auth middleware in the global chain, because `register` and
`login` must work without a session. Instead `app.New` builds a wrapper once and routes opt in:

```go
requireAuth: func(next http.HandlerFunc) http.Handler {
    return middleware.RequireAuth{Sessions: sessions, Log: d.Log, Next: next}
}
```
[`app.go:49-51`](../src/server/internal/app/app.go#L49-L51)

```go
mux.HandleFunc("POST /api/auth/register", h.Register)           // open
mux.Handle("POST /api/auth/logout", s.requireAuth(h.Logout))    // guarded
```
[`router.go:38-41`](../src/server/internal/app/router.go#L38-L41)

Authentication becomes **visible at the route table**: one line per endpoint tells you whether it is
protected. A blanket middleware with a path-prefix allowlist hides that decision in a string
comparison, and the failure mode — a new route silently unprotected — is invisible in review.

`RequireAuth` reads the `sid` cookie, loads the session, and on success does the one thing that
makes the whole layering work:

```go
ctx := session.WithUserID(r.Context(), sess.UserID)
m.Next.ServeHTTP(w, r.WithContext(ctx))
```
[`middleware.go:218-219`](../src/server/internal/app/middleware/middleware.go#L218-L219)

### Why the context helpers live in `platform/session`

This is a small decision with a structural reason, stated at
[`session.go:119-122`](../src/server/internal/platform/session/session.go#L119-L122). The context
key is *written* by middleware (in `app/`) and *read* by handlers (in `modules/`). If the helpers
lived in `app/middleware`, every module would import `app/` — but `app/` imports every module. That
is an import cycle, and Go will not compile it.

Putting `WithUserID`/`UserID` in `platform/session` gives both sides a package *below* them to
share. The key is an unexported `type ctxKey struct{}`, so no other package can collide with it even
by accident.

### The return trip

Nothing new happens on the way out — the response is return values unwinding the same call stack:

| Layer | Going down | Coming back |
|---|---|---|
| middleware | wraps writer, caps body, injects user id | reads status, logs one line |
| handler | decodes request → typed input | maps result or sentinel error → status + JSON |
| service | applies rules, calls store | returns value or domain sentinel |
| store | runs SQL | returns rows or `errUserMissing`/`errAlready` |

Error translation happens exactly once per boundary, in the layer above. `writeErr` is where domain
errors become HTTP:

```go
case errors.Is(err, errUserMissing), errors.Is(err, errNoRequest):
    // a missing user and a request you may not touch both read as not found
    httpx.WriteError(w, http.StatusNotFound, "not_found", "not found")
```
[`handler.go:132-135`](../src/server/internal/modules/follows/handler.go#L132-L135)

That collapse is a privacy rule, not laziness: **404 for "does not exist" and for "exists but is
not yours" are deliberately identical**, because distinguishing them tells an attacker which
private things exist. The API contract codifies it — 404 means *not found or not visible to you*.

The `default` branch is the other half: an unrecognised error is logged with its real detail and the
client gets a generic 500. Internal errors never reach the wire.

---

## 7. Why it is one-directional

"One-directional" means three separate things here. All three are checkable, and the second one is
checkable by the compiler.

### 7.1 Calls only go down; values come back up

`handler → service → store`, never the reverse. No layer holds a reference to the layer above it:
`Service` holds `*Store` ([`service.go:7-9`](../src/server/internal/modules/follows/service.go#L7-L9)),
`Store` holds `*sql.DB`, and neither has a field pointing back.

So a store cannot write a response, and a service cannot set a status code — not by convention, but
because the `http.ResponseWriter` is never passed down. The request's own copy of `*http.Request`
stops at the handler; only `r.Context()` and plain values travel further.

Two things follow:

- **A service is testable without HTTP.** `Follow(ctx, 1, 2)` is a function call.
- **A rule has exactly one entry point.** When chat needs "can A see B?", it must go through the
  service that owns it; it has no way to re-enter from the side.

### 7.2 Imports only point inward

```
cmd/  →  internal/app/  →  internal/modules/*  →  internal/platform/*,  pkg/db/*
```

Verified against the code as it stands:

- No module imports another module.
- Nothing in `platform/` or `pkg/` imports `modules/` or `app/`.
- `internal/app/router.go` is the **only** file outside `modules/` that imports a module at all.

That last point is the design doc's claim that `app/` is "the only package that knows every module
exists", and it is currently literally true: one import line.

**Why the direction matters.** Dependencies point from the specific toward the general, so the
general half has no idea the specific half exists. `httpx` does not know what a follow is; `session`
does not know what a group is. A cycle at any of these arrows would be both a compile error and a
sign that two things that change for different reasons had been fused.

### 7.3 Cross-module signals go one way and never come back

When `follows` creates a pending request, somebody should be notified. The tempting call is
`notifications.Create(...)` — and it would put a permanent arrow from `follows` to
`notifications`, then another from `groups`, then `chat`, until the module graph is a ball.

Instead the publisher fires an event into `platform/events` and forgets it. From `plan/08`:

- `Publish` is **fire-and-forget and must never fail the caller**. A notification that fails to
  write must not roll back the follow request that caused it.
- Dispatch happens in a goroutine with its own `recover`, after the publisher's transaction commits.

So the arrow is `follows → events`, and separately `events → notifications`. `follows` names no
consumer; `notifications` names no producer. `app.New` subscribes the two, and that is the only
place both packages appear together.

The placeholder for this is already in the service, written as a comment rather than a stub:
[`service.go:35-37`](../src/server/internal/modules/follows/service.go#L35-L37).

### What "back and forth" does *not* mean

A request goes down and a response comes back up **the same stack**. Nothing re-enters sideways:
no handler calls another handler, no service calls a sibling service, no store is shared between
modules. A single HTTP request is one pass down and one pass up — which is why reading a feature
top to bottom is enough to know everything that happens during it.

---

## 8. Cross-module contact without cross-module imports

Two mechanisms, for two genuinely different situations. The distinction is **who needs to wait**.

### When nobody needs to wait: `platform/events`

"Someone requested to follow you" — the follow succeeded; the notification is a consequence. Covered
in §7.3.

### When the caller needs an answer: a consumer-declared interface

`posts` needs to know "is A following B?" *before* it can decide whether to return a post. There is
no fire-and-forget version of that question.

The answer is an interface — but **declared in the package that needs it**, not the one that
provides it:

```go
// declared in modules/posts, modules/chat, modules/users — each in its OWN package
type FollowChecker interface {
    IsFollowing(ctx context.Context, followerID, followeeID int64) (bool, error)
}
```
`plan/04-users-follows.md:14-19`

`app/` passes the `follows` store in as the implementation. This is what `chat/ports.go` is reserved
for — currently a package declaration awaiting Phase 7.

**Why the consumer declares it.** Three consequences, in increasing order of importance:

1. **`follows` never grows methods to please `posts`.** The provider does not know it is being
   consumed; it satisfies the interface by accident of having the right method.
2. **Each consumer asks for only what it needs.** `posts` declares one method, so a test fake is one
   method, not the whole `follows` surface.
3. **The import still points the right way.** `posts` imports nothing from `follows` — it imports
   its own interface, and `app/` supplies something that fits. The arrow §7.2 forbids never appears.

The alternative — `follows` exporting a `FollowChecker` for others to import — recreates the
dependency the rule exists to prevent, just with an interface in the middle.

---

## 9. `modules/` versus `platform/`: the deciding test

Both sit under `internal/`. The question "which one does this go in?" has a one-line answer:

> **If the package would still make sense in a completely different application, it is
> `platform/`. If it would be meaningless outside this one, it is `modules/`.**

`storage` saves images and returns a path — a forum, a wiki, or a photo gallery would all want it
unchanged. `follows` is about follow requests against private profiles; it is meaningless anywhere
else. That is the whole test.

| `platform/` package | Why it is its own package — the failure it prevents |
|---|---|
| `config` | Env parsing in one place, **validated at boot**. A missing variable kills the process on start instead of surfacing as a nil-pointer panic on someone's first login three hours later. `Load()` collects *every* problem with `errors.Join` rather than returning the first, so you fix one round of errors instead of five. |
| `httpx` | One JSON encode/decode path and **one error envelope**. Without it every handler invents its own error shape and the frontend grows seven parsing branches. `DecodeJSON` sets `DisallowUnknownFields`, which turns a typo'd field into a 400 instead of a silent default — and blocks mass assignment, where a client sends `is_admin` and the decoder helpfully fills it in. |
| `session` | Server-side session rows plus cookie flags (`HttpOnly`, `SameSite=Lax`, `Secure` in production only). The audit tests two browsers staying independently logged in; that behaviour lives in one file. Sessions are rows, not signed cookies, so logout is a real `DELETE` and sessions survive a restart. |
| `password` | bcrypt hash and compare. Two functions — but isolating them makes the cost factor a one-line change and keeps bcrypt out of every other import list. |
| `storage` | Writes uploads to `data/uploads/`, **sniffs the real content type** instead of trusting the extension or the client's header, rejects anything that is not JPEG/PNG/GIF, and names files randomly so no upload can overwrite another or escape the folder via `../`. Every feature with an image — posts, comments, avatars — gets the same guarantees from one path. |
| `realtime` | The WebSocket hub: connection registry, per-user fan-out, room broadcast. **Transport only** — it does not know what a "message" or an "event" means, which is what lets chat *and* notifications share one socket. |
| `events` | In-process pub/sub. The decoupler of §7.3. |
| `logger` | `log/slog` wrapper: JSON in production, text in development. Returns a `*slog.Logger` that `app/` passes down — **no package-level global**, so a test can capture output and nothing acquires a hidden dependency on logging being configured. |

Two things worth noticing in that table. `RequestLog` logs `r.URL.Path` and deliberately not the
query string ([`middleware.go:76`](../src/server/internal/app/middleware/middleware.go#L76)), because
queries can carry tokens. And body size is capped at three different depths for three different
reasons: 10 MB for any request (`MaxBodyBytes`), 1 MB for a JSON body (`httpx.maxJSONBody`), 1 MB of
*memory* for a multipart upload before it spills to a temp file (`auth.multipartMemory`).

`testutil/` holds fixtures and an in-memory SQLite helper. It lives under `internal/` so a real
binary can never import it.

### The two folders that are deliberately absent

- **No `utils/` on the Go side.** Every helper here has a real home: JSON goes to `httpx`, hashing
  to `password`, files to `storage`. A Go `utils` package is a symptom — it means somebody could not
  name the concern, and once it exists everything imports it.
- **No `models/`.** See §5.

---

## 10. `pkg/db/` and `data/`: the two folders shaped by outside forces

```
pkg/db/
├── migrations/sqlite/   # 000001_create_users_table.up.sql / .down.sql ... 000015
├── seed/                # optional dev dataset
└── sqlite/sqlite.go     # Open() + Migrate()
```

### The one deliberate deviation from Go idiom

By Go convention `pkg/` means "intended for import by other projects", and nothing outside this
module imports this code — so idiomatically it belongs in `internal/platform/database/`.

It is here anyway because the subject shows `backend/pkg/db/migrations/sqlite/` and says the file
organisation will be tested. An auditor scanning for a recognisable migrations layout should find it
immediately. The cost is one folder name, and §7 of the design doc already lists moving it as
post-audit work. **This is written down precisely so it does not read as an accident** — an
un-idiomatic choice with a stated reason is a decision; the same choice unexplained is a mistake.

### Why `Open()` is more than `sql.Open`

Four pragmas, each fixing a specific SQLite behaviour
([`sqlite.go:18-56`](../src/server/pkg/db/sqlite/sqlite.go#L18-L56)):

| Pragma | Without it |
|---|---|
| `_foreign_keys=on` | **SQLite silently ignores every FK constraint.** Off by default. Every `REFERENCES` in all fifteen migrations would be decoration. |
| `_journal_mode=WAL` | Readers block during a write. With HTTP and WebSockets on one database, that is a visible stall. WAL writes to `social.db-wal` and merges later, so reads proceed. |
| `_busy_timeout=5000` | A concurrent write returns `SQLITE_BUSY` immediately. SQLite is single-writer; with the timeout the second writer waits up to 5s instead of failing. |
| `_txlock=immediate` | A transaction that starts reading and later writes can fail to upgrade its lock. Taking the write lock up front avoids that. |

Having this in one function is the point: "SQLite is configured correctly" is one place to verify,
and `cmd/server` and `cmd/migrate` cannot drift apart.

`Open` also runs `os.MkdirAll(filepath.Dir(path))`, so a fresh clone or a fresh container does not
need `data/` to exist in advance.

### Migrations: numbered pairs, every up with a down

Fifteen pairs, `000001` through `000015`. **Every `.up.sql` gets a `.down.sql`, even a trivial one** —
a migration you cannot reverse is a migration you cannot develop against, because the only way to
retry it is to delete the database.

`seed/` is kept **out of** `migrations/sqlite/` so a clean run does not inherit fake users; it is
invoked explicitly through `cmd/migrate`.

### `data/`

Runtime state: the `.db` file (plus `-wal` and `-shm`) and `uploads/`. It is a separate top-level
folder because it is **the Docker volume mount point** — the one directory that must survive a
container rebuild. `uploads/` keeps a `.gitkeep` and its contents are gitignored.

Uploads are served by a route, not by the static file server:
`mux.HandleFunc("GET /uploads/{name}", s.uploads.Serve)`. The `{name}` wildcard cannot contain `/`
and the `fs.FS` rejects `..`, so the handler cannot be walked out of the folder
([`storage.go:81-86`](../src/server/internal/platform/storage/storage.go#L81-L86)).

---

## 11. The frontend: the same idea, different axis

```
src/web/
├── app/            # App Router — URLs only
│   ├── (auth)/     #   login, register
│   ├── (main)/     #   feed, profile/[userId], groups/[groupId], chat/[conversationId], notifications
│   └── api/        #   optional same-origin BFF
├── features/       # ← mirrors internal/modules/ one for one
│   └── auth/ profile/ follows/ posts/ groups/ chat/ notifications/
│       ├── components/   hooks/   api.ts   types.ts
├── components/
│   ├── ui/         # primitives: Button, Modal, Avatar
│   └── layout/     # shell: Navbar, Sidebar, NotificationBell
├── lib/
│   ├── api/        # fetch wrapper
│   ├── realtime/   # WebSocket client
│   ├── session/    # reading session state
│   └── utils/
├── hooks/          # genuinely global hooks
└── middleware.ts   # route guard — framework requires this exact location
```

> **Status:** every directory above exists and every leaf is a `.gitkeep`. There is no
> `package.json`, no `middleware.ts`, no component. The plan folder declares the frontend out of
> scope for all nine backend phases. What follows is therefore the *intent* of the layout — useful
> because the folders are already committed, but none of it is running yet.

### `app/` is routing; `features/` is the application

The most important rule on this side. Files in `app/` stay short: read the params, check auth,
compose components from `features/`, stop.

**Why.** `app/`'s shape is dictated by URLs, and URLs change. Moving `/groups/[id]` to `/g/[slug]`
should be a folder rename, not a code migration. The second reason matters more day to day: a
component written directly into a route folder is **trapped** there — nothing else can reach it
without an import that looks wrong. That is the standard failure mode of App Router projects, and
the fix is structural rather than disciplinary.

`(auth)` and `(main)` are route groups: parentheses mean they do not appear in the URL. They exist
so the two halves of the app can have different layouts — a bare centred card for login, the full
shell for everything else. This is also how a requirement gets enforced by structure: the audit
wants notifications visible **on every page**, so the bell goes in `(main)/layout.tsx` once and
cannot be forgotten on a new page.

### `features/` mirrors `internal/modules/`

The same seven names on both sides, so a change request maps to one folder per side. "Group events
can be edited" means `internal/modules/groups/` and `features/groups/` — and you know that before
opening the repo.

The one rename is deliberate: the server's `users` is the web's `profile`, because that is what both
the audit and the UI call it, and **a folder should be named after the thing on screen**.

### `components/ui/` versus `features/*/components/`

A component belongs in `components/ui/` only when it is **generic — when it has no idea what a post
or a group is**. `Modal` qualifies. `PostCard` does not, so it lives in
`features/posts/components/`.

The test: **if it imports a feature type, it is not a primitive.** Getting this wrong in the
permissive direction is what turns a shared components folder into a 150-file directory that
everything imports and nobody can change safely.

### `lib/` is infrastructure, not helpers

Something in `lib/` **owns a concern**: HTTP transport, socket lifecycle, session reading. It is not
a bucket for loose functions — that is `lib/utils/`, and it should stay small.

- **`lib/api/`** — one fetch wrapper. It sets `credentials: "include"` (**without which the session
  cookie the entire audit depends on is never sent**), prefixes the API base URL, and normalises
  non-2xx responses into one typed error. Because `httpx` guarantees a single envelope server-side,
  this is one parser. Every feature's `api.ts` calls through it, so changing auth handling or error
  format is one edit.
- **`lib/realtime/`** — one WebSocket connection for the whole app, shared by chat *and*
  notifications, with reconnect and backoff. **This is where TypeScript earns its place:** the server
  sends a tagged union of event types over a single socket, and a discriminated union on `type` makes
  the client's dispatch exhaustive at compile time. Add a server event and forget the client branch,
  and the build fails instead of the feature silently doing nothing.
- **`middleware.ts`** — checks the session cookie and redirects unauthenticated users away from
  `(main)` before any page renders. It is at the `web/` root because Next.js requires that exact
  location. **It is a UX guard, not a security boundary** — the Go server re-checks the session on
  every request, and `RequireAuth` is the only thing actually protecting anything. Treating
  `middleware.ts` as authorization is the classic mistake here, and §2 is why the folder layout makes
  it obvious: `web/` owns no state, so it cannot be authoritative about anything.
- **`app/api/`** — optional. A same-origin proxy keeps cookies first-party and hides the Go server's
  URL from the browser. Left empty; delete it if you call Go directly.

### Why the two sides are not symmetric

They mirror in *naming* but not in *shape*, and the difference is instructive. The Go side slices by
layer within a feature (`handler`/`service`/`store`) because the layers have different
responsibilities worth isolating. The web side slices by *kind of code* within a feature
(`components`/`hooks`/`api`) because React's units are components and hooks, not layers. Both sides
answer "where does this feature live?" with one folder; what is inside matches the language.

---

## 12. `shared/`: the seam between two languages

```
src/shared/
├── contracts/    # request/response shapes, the WebSocket event union
└── constants/    # values both sides must agree on
```

> **Status:** `.gitkeep` only.

`constants/` holds the values where a typo is a **silent** bug rather than a loud one: post privacy
(`public` / `almost_private` / `private`), follow states (`pending` / `accepted`), notification
types, WebSocket event names. These strings are written by Go and read by TypeScript. Keep them in
one place and a rename is caught by the compiler; duplicate them and it is caught by a user staring
at an empty feed.

Those exact strings are already live in the backend — `"pending"` and `"accepted"` appear as literals
in `follows/service.go` and `follows/store.go`. That is the duplication `constants/` is meant to
absorb.

`contracts/` is hand-written TypeScript by default: the API is small enough that code generation is
overhead. Generate from an OpenAPI spec once hand-drift actually bites — adding a build step on day
one to an API you are changing hourly costs more than it saves.

`shared/` is copied into the **web** Docker build, which is the other reason it is top-level rather
than nested under either side.

---

## 13. Where the code actually is right now

The tree is complete; the code is not. Knowing the difference keeps you from reading an empty file
as a design statement.

**Working end to end** — config, logger, `sqlite.Open`/`Migrate`, `httpx`, `storage`, `session`,
`password`, the full middleware chain, all fifteen migration pairs, `cmd/server`, `cmd/migrate`, and
the `auth` module (register / login / logout / me) plus `/api/health` and `/uploads/{name}`.
That is Phases 1–3 of the plan.

**Written but not reachable** — `modules/follows` is complete across all four files (handler,
service, store, model), but [`router.go:45`](../src/server/internal/app/router.go#L45) is still
`func mountFollows(...) {}`. The module compiles and nothing routes to it. Wiring it is the
remaining step of Phase 4, and it is a good illustration of §3: the composition root is the single
place that decides a module is live, so "written" and "serving traffic" are genuinely separate
states.

**Package declaration only** — `modules/groups` (5 files) and `modules/chat` (4 files, including the
`ports.go` reserved for the `FollowChecker` interface of §8). `modules/chat/model.go` is the
exception: its three types are written.

**Still `.gitkeep`** — `modules/users`, `modules/posts`, `modules/notifications`,
`platform/events`, `platform/realtime`, `internal/testutil`, `pkg/db/seed`, and the whole of
`src/web/` and `src/shared/`.

Two consequences worth stating plainly:

- **The event bus does not exist yet**, so §7.3 describes a mechanism that is currently a comment at
  `follows/service.go:35-37` and a specification in `plan/08`. The rule it enforces is nonetheless
  already satisfied — no module imports another, because there is nothing to import yet. The bus is
  what keeps that true once `notifications` exists.
- **There are no tests.** `testutil/` is empty and no `*_test.go` file exists. The plan calls for
  `service_test.go` beside each `service.go`, and `plan/07` specifically asks for
  `go test -race ./...` on the hub — the race detector finds the hub bugs that otherwise appear as a
  crash during the audit.

### The shortest version of all of it

1. A feature is a folder, on both sides, with the same name.
2. Handlers hold no rules. Services hold rules. Stores hold only SQL.
3. Modules never import modules — `platform/events` or a consumer-declared interface instead.
4. `main` and route files wire; they never decide.
5. Authorization is decided in Go, on every request. `middleware.ts` improves the experience and
   protects nothing.
6. `components/ui/` is domain-free. If it imports a feature type, it is not a primitive.
7. Every `.up.sql` has a `.down.sql`.
8. Nothing lands in `utils/` that could name its own concern.
