# Phase 7 — WebSocket hub & chat

**Goal:** one socket per user carrying both chat messages and notifications, with DMs and group
rooms persisted to SQLite.

The riskiest phase. The audit repeatedly checks "did the chat crash the server?" — so
concurrency correctness matters more than features here.

---

## Design

**One connection per user, not one per feature.** Chat and notifications share it. The client
opens `GET /ws` once; the server multiplexes by a `type` field.

```json
{ "type": "dm.message", "payload": { ... } }
```

`type` is a **tagged union**. Put the string constants in `src/shared/constants/` so the
frontend's discriminated union can't drift (design doc §5).

### `internal/platform/realtime` — transport only, zero domain knowledge

```go
type Hub struct {
    mu    sync.RWMutex
    conns map[int64]map[*Conn]struct{}  // userID → that user's open sockets
}

func (h *Hub) Register(userID int64, c *Conn)
func (h *Hub) Unregister(userID int64, c *Conn)
func (h *Hub) SendToUser(userID int64, msg []byte)
func (h *Hub) SendToUsers(userIDs []int64, msg []byte)
func (h *Hub) IsOnline(userID int64) bool
```

The nested map (not `map[int64]*Conn`) is deliberate: the audit has the same person open in two
browsers, and both must receive.

The hub knows nothing about messages, groups or notifications. It moves bytes to user ids.

### The four rules that stop the crashes

1. **One writer goroutine per connection.** `gorilla/websocket` panics on concurrent writes.
   Every send goes through a buffered `send chan []byte`; the write pump is the only caller of
   `WriteMessage`.
2. **Buffered send channel + drop-on-full.** If a client stalls, a `select` with a `default`
   that closes the connection beats blocking the hub forever.
3. **Every map access holds the mutex.** Register, unregister and broadcast all touch `conns`.
4. **`defer Unregister` in the read pump.** A closed browser tab must not leave a dead entry
   that later gets written to.

Ping/pong keepalive with read deadlines, so half-open connections get collected.

---

## Tasks

### `internal/platform/realtime`
- [ ] `Hub` as above, with `sync.RWMutex`.
- [ ] `Conn` with read pump, write pump, send channel, ping/pong deadlines.
- [ ] `Upgrade` handler with a `CheckOrigin` that validates against `cfg.AllowedOrigin` —
      the default `CheckOrigin` accepts everything.

### Route
- [ ] `GET /ws` behind `RequireAuth`. **Authenticate before upgrading** — the session cookie is
      sent on the handshake, so this is a normal middleware check, no token-in-query needed.

### `internal/modules/chat`

**Direct messages**
- [ ] `POST /api/messages` (`recipient_id`, `content`) → persist, then
      `hub.SendToUsers([sender, recipient])` so the sender's other tabs stay in sync.
- [ ] **The gate:** at least one of the two must follow the other, *or* the recipient's profile
      is public. Enforce in the service via the `FollowChecker` interface from Phase 4. The audit
      explicitly tests that two unconnected users cannot chat.
- [ ] `GET /api/messages/{userID}` — history, paginated, newest last.
- [ ] `GET /api/conversations` — who you can chat with + last message + unread count.
- [ ] Only the two participants receive it. The audit opens three browsers to check this.

**Group chat**
- [ ] `POST /api/groups/{id}/messages` → persist, fan out to **current members only** (query
      `group_members` at send time, don't cache a room list — membership changes).
- [ ] `GET /api/groups/{id}/messages` — history, members only.

**Emoji:** nothing to do. Store content as UTF-8 `TEXT` and don't sanitise it into oblivion.
Just confirm a 4-byte emoji round-trips through SQLite and back. (It does — but check.)

### Tests
- [ ] Non-connected users → 403 on DM.
- [ ] Hub: register two conns for one user, broadcast, both receive.
- [ ] Run the package with `go test -race ./...`. **Do this one for real** — the race detector
      finds the hub bugs that otherwise show up as a crash during the audit.

---

## Done when

Three `websocat` (or browser) sessions:
- A→B DM: B receives instantly, C receives nothing
- A and B in a group, C not: a group message reaches B only
- Kill B's connection mid-conversation: the server stays up, A can keep sending
- `go test -race ./...` is clean
