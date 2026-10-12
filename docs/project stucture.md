# Structure Walkthrough

How this repo is laid out, how a request moves through it, and why each boundary is where it is.

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
13. [Frameworks: what each one is actually doing](#13-frameworks-what-each-one-is-actually-doing)
14. [Where the code actually is right now](#14-where-the-code-actually-is-right-now)

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
├── app/                      # App Router — URLs only, 14 routes
│   ├── layout.tsx            #   <html>, the two stylesheets, the pre-paint scheme script
│   ├── error.tsx             #   root error boundary
│   ├── (auth)/               #   login, register — a bare centred card
│   └── (main)/               #   feed, profile/[userId], directory, groups/[groupId],
│                             #   groups/[groupId]/chat, events, chat/[conversationId],
│                             #   notifications — all inside the shell
├── features/                 # ← mirrors internal/modules/ one for one
│   └── auth/ profile/ follows/ posts/ groups/ chat/ notifications/
│       ├── api.ts            #   the calls this feature makes
│       ├── components/       #   feature-specific UI
│       └── hooks/            #   feature-specific state
├── components/
│   ├── ui/                   # domain-free: Avatar, Empty, ErrorView, NavButton, Toast
│   └── layout/               # the shell: Chrome, RailLeft, RailRight, StatusBar, the panels
├── lib/
│   ├── api/                  # client.ts, errors.ts, describe.ts, useFetch.ts
│   ├── realtime/             # SocketProvider.tsx — one socket for the whole app
│   ├── session/              # SessionProvider.tsx — who is logged in
│   └── utils/                # identity.ts, time.ts, count.ts
├── hooks/                    # genuinely global: useDebounced, usePrefersReducedMotion
├── styles/                   # lacuna.css (the design) + additions.css (the only exception)
└── middleware.ts             # route guard — framework requires this exact location
```

The seven `features/` names are the seven `internal/modules/` names, with the one deliberate
rename: the server's `users` is the web's `profile`, because that is what the audit and the UI
both call it and **a folder should be named after the thing on screen**.

### `app/` is routing; `features/` is the application

Files in `app/` read the params, check the session, compose components from `features/`, stop.

**Why.** `app/`'s shape is dictated by URLs, and URLs change. Moving `/groups/[id]` to `/g/[slug]`
should be a folder rename, not a code migration. The second reason matters more day to day: a
component written directly into a route folder is **trapped** there — nothing else can reach it
without an import that looks wrong. That is the standard failure mode of App Router projects, and
the fix is structural rather than disciplinary.

`(auth)` and `(main)` are route groups: parentheses mean they do not appear in the URL. They exist
so the two halves of the app can have different layouts, and that is also how a requirement becomes
structural rather than remembered. The audit wants notifications visible **on every page**, so the
Pending panel and the logout control live in `(main)/layout.tsx` once and cannot be forgotten on a
new route.

That layout also enforces the session invariant, and the way it does so is worth recording because
getting it wrong broke logout:

```tsx
{state.status === 'authenticated' ? children : <Loading label={...} />}
```
[`(main)/layout.tsx:74`](../src/web/app/%28main%29/layout.tsx#L74)

Children render **only** when there is a user, because that is exactly what `useUser()` promises
its callers and exactly what it throws over. An earlier version gated on "not loading", which let
the third state through: `signOut()` flips the session to `anonymous` synchronously while
`router.replace('/login')` is still in flight, React re-renders in the gap, and the first line of
the page calls `useUser()`. The error boundary then replaced the route before the navigation
landed. The race is not winnable by reordering, because `signOut()` is synchronous and navigation
never is — **gating the render removes the race instead of betting on it.**

### `components/ui/` versus `features/*/components/`

A component belongs in `components/ui/` only when it is **generic — when it has no idea what a post
or a group is**. `Empty`, `Toast` and `NavButton` qualify. `PostCard` does not, so it lives in
`features/posts/components/`.

The test: **if it imports a feature type, it is not a primitive.** `Avatar` sits right on the line —
it takes a name and an optional path, not a `User`, which is what keeps it in `ui/`.

### `lib/` is infrastructure, not helpers

Something in `lib/` **owns a concern**: HTTP transport, socket lifecycle, session reading. It is not
a bucket for loose functions — that is `lib/utils/`, and it stays small.

**`lib/api/client.ts`** — the only `fetch` in the application. One line is the reason it exists:

```ts
const res = await fetch(`${API_BASE}${path}`, { ...init, credentials: 'include' })
```
[`client.ts:40`](../src/web/lib/api/client.ts#L40)

The session cookie is `HttpOnly` and scoped to `:8080` while the page is on `:3000`. Without
`credentials: "include"` the browser attaches nothing and every authenticated route answers 401
while the request looks perfectly correct in the network tab. It is set here, once, where no
feature can forget it.

Two other things this file settles for everyone. `form()` deliberately sets **no** `Content-Type`,
because the browser has to generate the multipart boundary itself — a hand-written
`multipart/form-data` has no boundary and `ParseMultipartForm` answers *"expected a
multipart/form-data body"*, the most misleading error in the API. And a 204 returns early rather
than calling `res.json()`, because 204 is the success case for roughly fifteen endpoints and
`json()` on an empty body throws.

**`lib/api/describe.ts`** — one error taxonomy for the whole app.

```ts
export function describeError(e: unknown): ErrorFace
```
[`describe.ts:42`](../src/web/lib/api/describe.ts#L42)

It returns data, never JSX, so the mapping is testable and the drawing stays in
`components/ui/ErrorView.tsx` — the same split as handlers and services on the Go side. Every page
that can fail renders `<ErrorView error={e} onRetry={refetch} />` and nothing else, which means no
page ever writes an `if (status === 403)` of its own:

| status | shape | where it lands |
|---|---|---|
| 0 unreachable, 5xx | `notice` | the red `.notice` bar, with a retry |
| 400 / 409 / 413 / 415 with `fields` | `fields` | under the input the server named |
| 401 `invalid_credentials` | `notice` | the login form, in place |
| 401 `unauthenticated` | `auth` | session cleared, redirect |
| 403 | `notice` | in place, no retry — asking again will not help |
| 404 | `empty` | the `.empty` block |

The 404 copy never says "deleted". A 404 from this API means *not found **or** not visible to you*
— that is the documented behaviour on posts, profiles, groups and follow requests — so "deleted"
would be a lie every time a stranger opens an `almost_private` post.

**`lib/session/SessionProvider.tsx`** — three states, not two:

```ts
export type SessionState =
  | { status: 'loading' } | { status: 'authenticated'; user: User } | { status: 'anonymous' }
```
[`SessionProvider.tsx:14`](../src/web/lib/session/SessionProvider.tsx#L14)

A boolean cannot tell "not yet known" from "not logged in", which is the bug where a reader with a
perfectly good cookie watches the login page flash on every hard refresh.

**`lib/realtime/SocketProvider.tsx`** — one connection for the whole app, shared by chat *and*
notifications. Two sockets would be two reconnect loops and two sets of duplicate frames, and the
hub keys connections by user id so it would hold both. **This is where TypeScript earns its place:**

```ts
default: return assertNever(frame)
```
[`SocketProvider.tsx:68`](../src/web/lib/realtime/SocketProvider.tsx#L68)

Add a variant to `ServerFrame` and forget the branch, and the build fails instead of the feature
silently doing nothing.

The client also never treats the socket as the source of truth. `chat/service.go` commits the row
and *then* pushes, best effort, so REST is the record and the socket is a live upgrade on top of
it. `POST /api/messages` returns 201 with the saved row **and** the socket pushes the same row, so
the sender receives their own message twice by design and dedupes on `id`.

**`middleware.ts`** — checks that the cookie exists:

```ts
const hasSession = Boolean(request.cookies.get('sid')?.value)
```
[`middleware.ts:17`](../src/web/middleware.ts#L17)

That is all it can do and all it is for. The sessions table lives in Go and `RequireAuth` re-reads
it on every single request. **It is a UX guard, not a security boundary** — and §2 is why the layout
makes that obvious: `web/` owns no state, so it cannot be authoritative about anything. It also
cannot tell a *valid* cookie from a *stale* one, which is why `(main)/layout.tsx` redirects on
`anonymous` rather than trusting the guard.

### Two pagination schemes, on purpose

The backend offers both and they are not interchangeable:

| scheme | used by | why |
|---|---|---|
| cursor — `before` is a row id | feed, author posts, DM and group history | the list grows at the top |
| offset — `limit` + `offset` | notifications, the directory | the list is stable and ordered by name |

[`useCursorList.ts:30`](../src/web/features/posts/hooks/useCursorList.ts#L30) implements the first.
With an offset the page boundary slides as new rows arrive, so you get shown the same post twice and
miss one; `before` anchors to a row and cannot drift. A short page is the only end-of-list signal
the API gives — there is no total anywhere — so the UI draws the design's `.eof` marker and never
claims "page 3 of 9".

Chat inverts it. The store queries `ORDER BY id DESC` to get the most recent page and then calls
`reverse()` before returning, so the response is already chronological and
[`useThread.ts`](../src/web/features/chat/hooks/useThread.ts) prepends it unchanged. Reading only
the `ORDER BY` and reversing again on the client scrambles every page — which it did, once.

### The stylesheet is the design, and one file is not

`styles/lacuna.css` is 2,454 lines lifted byte for byte out of the single-file mockup in `Docs/`.
It is **read only**. A component that needs a class it does not define is a component drifting from
the design, and the fix is to re-read the mockup rather than add a rule. That is checkable, and the
check is one grep of every `className` against every selector.

`styles/additions.css` is the one exception, 262 lines, covering the two things the mockup never
needed because it had no server and never failed: a real `<img>` inside `.av` and `.plate`, and a
form outside the composer. Every value in it is lifted from a rule or token `lacuna.css` already
defines. Keeping it separate rather than appending means the design file stays exactly what the
mockup had, and the whole addition is one deletable file.

The same discipline produced a class of bug worth naming. Several of the design's rules are scoped
to an **element**, not a class — `.presence button`, `.seg button`, `.post__who .nm button`,
`.main__hd .crumb button`. Rendering a Next `<Link>` there produces an `<a>`, which picks up none of
it and lands as underlined default-blue text. `components/ui/NavButton.tsx` exists for exactly those
places: a real `<button>` that navigates.

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
├── contracts/    # users, follows, posts, groups, chat, notifications, ws
└── constants/    # domain.ts, ws.ts
```

`constants/` holds the values where a typo is a **silent** bug rather than a loud one: post privacy
(`public` / `almost_private` / `private`), follow states, group membership states, notification
types, WebSocket frame names. These strings are written by Go and read by TypeScript. Keep them in
one place and a rename is caught by the compiler; duplicate them and it is caught by a user staring
at an empty feed.

`contracts/` is hand-written TypeScript, one file per module, each mirroring the Go struct's JSON
tags one for one. The rule that makes it safe: **every pointer field in the Go struct is a nullable
column and must be `| null` here.** Get one wrong and you crash on the single user who skipped their
nickname, which is the least reproducible bug shape there is.

`contracts/ws.ts` is the load-bearing file. It carries the tagged union the socket speaks, plus
`assertNever` and `parseServerFrame` — one real validation at the boundary, because anything off a
socket is untrusted input. The two message types are declared there and **re-exported** by
`contracts/chat.ts` rather than redeclared, because a row fetched from history and a frame pushed
over the socket are deliberately the same shape and two declarations would drift.

Generation from an OpenAPI spec is still the escape hatch if hand-drift bites. It has not.

`shared/` is copied into the **web** Docker build, which is the other reason it is top-level rather
than nested under either side.

---

## 13. Frameworks: what each one is actually doing

Seven dependencies across both languages. The short version is that this project uses frameworks for
transport and rendering and almost nothing else — the rules, the SQL, the state and the styling are
all first-party on purpose.

### Go — four dependencies

| package | what it does here | what it is *not* doing |
|---|---|---|
| **stdlib `net/http`** | the entire router and server | — |
| **`gorilla/websocket`** | the HTTP→WS upgrade and frame read/write | it does not hold connections; the hub does |
| **`golang-migrate/migrate`** | applies `.up.sql` at boot, `cmd/migrate` for up/down/force | no schema DSL, the migrations are plain SQL |
| **`mattn/go-sqlite3`** | the driver | not an ORM; every query is hand-written in a `store.go` |
| **`x/crypto/bcrypt`** | hash and compare | two functions, isolated in `platform/password` |

**There is no router library, and that is a Go 1.22 decision rather than asceticism.** The stdlib
`ServeMux` now takes a method and wildcards in the pattern itself, which is the only thing a
third-party router was ever needed for:

```go
mux.Handle("POST /api/groups/{id}/requests/{userID}/accept", s.requireAuth(h.AcceptRequest))
```
[`router.go:127`](../src/server/internal/app/router.go#L127)

Two path variables, a method constraint, and `r.PathValue("userID")` to read them back. Adding
chi or gorilla/mux would buy middleware chaining, which [§6](#6-how-a-request-actually-travels)
already does with four structs that each hold a `Next`.

**`gorilla/websocket` is used as narrowly as possible.** It owns the handshake and the byte frames;
everything above that is ours. `platform/realtime` holds the connection registry, the per-user
fan-out, the read and write pumps and the ping timer, and it is deliberately ignorant of what a
"message" is — `chat` and `notifications` define their own frame types and encode through
`realtime.Encode`. That is what keeps the transport a transport. The one place the library's default
would have been a security bug is `CheckOrigin`, which returns true for everything out of the box:
that is CSRF over a socket, because the WebSocket handshake is not subject to the same-origin policy
the way `fetch` is, so it is replaced with an explicit allow-list.

### TypeScript and the web — three runtime dependencies

`next`, `react`, `react-dom`. That is the whole `dependencies` block; `typescript` and the `@types`
packages are dev-only.

**Next.js (App Router)** is used for five specific things and nothing else:

1. **File-system routing** — 14 routes, one folder each, so a URL change is a folder rename.
2. **Route groups** — `(auth)` and `(main)` give the two halves different layouts without appearing
   in the URL, which is what makes "notifications on every page" structural.
3. **Nested layouts and error boundaries** — `layout.tsx`, `error.tsx` and `not-found.tsx` per
   segment, so the shell is mounted once and a crash inside a route keeps the navigation around it.
4. **`middleware.ts`** — the pre-render cookie check, at the root because the framework requires
   that exact path.
5. **`output: 'standalone'`** — the Docker runtime stage copies a minimal tree with its own
   `server.js` instead of all of `node_modules`.

What it is deliberately **not** used for is data fetching. Every page is a client component and
every call goes through `lib/api`. The reason is concrete rather than stylistic: the session is an
`HttpOnly` cookie on a different origin, and the browser is the only thing holding it. A server
component would have to forward `Cookie` by hand on every call, and this app has no SEO and no
first-paint budget to justify that.

**React 19** is used as hooks and components. No class components, no portals, no `Suspense` for
data — the single `Suspense` boundary in the app wraps `useSearchParams` on the login page because
that hook opts a route into client rendering.

**TypeScript** is on `strict`. The place it pays for itself is the socket union described in §11;
the place it pays second is `shared/contracts/`, where `| null` on every nullable column turns a
class of runtime crash into a compile error.

### What is absent, and why

This doc already has a habit of explaining a layout by what is missing
([§9](#9-modules-versus-platform-the-deciding-test)). The same applies to the dependency list.

| not used | what it would have done | why not |
|---|---|---|
| an ORM | generate the SQL | the visibility predicate in `posts/store.go` is a four-branch `EXISTS` chain that has to run **in the database** so pagination stays correct. Expressing that through an ORM is harder to read than the SQL |
| a router library | method + path params | the stdlib does both since Go 1.22 |
| TanStack Query / SWR | caching, revalidation | the one thing a cache would buy is a live notification count, and the socket already keeps that live. `useFetch` is forty lines |
| Redux / Zustand | global state | there are four contexts — session, socket, badges, toast — and everything else is local to a page |
| Tailwind / CSS modules | styling | the design arrived as one finished stylesheet. Tailwind would mean deleting it, CSS modules would mean renaming 400 selectors for nothing |
| a component library | buttons, modals, inputs | the design **is** the component library. Importing MUI would mean overriding it back to this |
| a form library | validation, field state | the server is the validator. `httpx.ValidationErrors` returns one message per field name and the form prints what it is handed, so duplicating the rules client-side would create two sets that drift |

The pattern across the whole table is the same: a dependency is worth taking when it owns something
genuinely hard and external — a wire protocol, a C database driver, a password KDF, a rendering
runtime. It is not worth taking to avoid writing forty lines you then have to configure anyway.

---

## 14. Where the code actually is right now

Both sides are built and running against each other. What follows is what is verified, what is
assumed, and what is known to be broken.

**Backend — complete.** All nine phases. Every route in `plan/api-contract.md` is mounted in
`router.go` and answering: auth, users, follows, posts, groups, chat, notifications, `/ws`,
`/api/health` and `/uploads/{name}`. The event bus has four subscribers and the hub is wired into
both `chat` and `notifications`, so all four required notifications are stored **and** pushed live.

Three additions were made after the frontend existed, each because the design's own UI needed
something the API did not expose:

| addition | files | reason |
|---|---|---|
| `viewer_count` on `posts.Post` | `models.go`, `store.go` | the private-post tag reads `selected · N` |
| `DELETE /api/posts/{id}` | `store.go`, `service.go`, `handler.go`, `router.go` | the design's `withdraw` action had no endpoint |
| group chat entry points | frontend only | the rooms already existed; nothing could reach them |

The delete is author-scoped in the `WHERE` rather than in a preceding `SELECT`, so there is no
window between "is it yours" and "delete it", and it unlinks the post's image **and** every comment
image before the cascade removes the rows that name them.

**Frontend — complete.** 14 routes, the ported stylesheet, the fetch wrapper and error model, the
session and socket providers, all seven feature folders, `middleware.ts`, and `src/web/Dockerfile`
with the frontend service wired into `docker-compose.yml`. `tsc --noEmit` is clean on `strict`.

**Verified end to end, in a browser against the running backend:** register with and without the
three optional fields, the full 3×3 post-visibility matrix, cursor pagination with no overlap or
gap, the follow request round trip, group invite and join-request flows, RSVP, the `403
chat_not_allowed` gate between two unconnected users, emoji round-tripping, and a DM and a
`notification.new` arriving **live over the socket** while a page sat open. Responsive folds
correctly at 1180px and 390px with no horizontal overflow.

### Known gaps, stated plainly

- **There are no tests.** `testutil/` is empty and no `*_test.go` exists. The plan calls for
  `service_test.go` beside each `service.go`, and `plan/07` asks for `go test -race ./...` on the
  hub specifically — the race detector finds the hub bugs that otherwise surface as a crash during
  an audit. This is the largest outstanding item.
- **The frontend Docker image has never been built.** `docker compose config` validates and the
  exact runtime layout was reproduced and run by hand, but no daemon was available to build it.
- **The migrations path is broken on Windows.** [`sqlite.go:87`](../src/server/pkg/db/sqlite/sqlite.go#L87)
  builds `"file://" + abs`, which on Windows yields `file://C:\Users\...` — the URL parser reads
  `C` as the host and `:\Users\...` as the port and rejects it. It works on Unix only because an
  absolute path there happens to start with `/`. Confirmed on a real machine; `docker compose up`
  is unaffected.
- **The socket retries forever after a 401.** A handshake refused before the upgrade never becomes a
  WebSocket, so the browser reports close code 1006 — indistinguishable from a dropped network — and
  the `giveUp` check keys on 1008, which never arrives. Noise rather than a broken flow.
- **A few synthetic readouts stay synthetic**, and keep the design's own `sim` tag: Carrier, the
  Loop player, segment and node identifiers, and the presence dots. The hub knows who is connected
  through `IsOnline`, but no presence frame is ever sent — `chat/service.go` leaves it as a todo —
  so the client has no way to learn it.

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
9. One `fetch`, one error taxonomy, one socket.
10. `lacuna.css` is the design and is read only. A class it does not define is drift.
