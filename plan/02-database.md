# Phase 2 — Database schema

**Goal:** all 15 migration pairs written and applied. Schema frozen before feature code starts.

Write them all now, even for features you build in week 3. Designing the schema in one sitting
catches the contradictions (can a group post be "almost private"? no — decided below) while
they're still free to fix.

Path: `src/server/pkg/db/migrations/sqlite/NNNNNN_description.{up,down}.sql`
Every `.up.sql` gets a `.down.sql`. The audit checks the layout, not just that it works.

---

## Conventions

- `id INTEGER PRIMARY KEY AUTOINCREMENT`
- Timestamps: `TEXT NOT NULL DEFAULT (datetime('now'))` — ISO-8601 UTC, sorts lexically.
- Enums: `TEXT` + `CHECK (col IN (...))`. SQLite has no enum type; the CHECK is your only guard.
- `ON DELETE CASCADE` on every FK pointing at `users` or `groups`.
- Index every FK you filter or join on. SQLite does **not** auto-index foreign keys.

---

## The 15 migrations

### 000001 users
```
id, email TEXT NOT NULL UNIQUE COLLATE NOCASE, password_hash TEXT NOT NULL,
first_name TEXT NOT NULL, last_name TEXT NOT NULL, date_of_birth TEXT NOT NULL,
avatar_path TEXT, nickname TEXT UNIQUE, about_me TEXT,
is_public INTEGER NOT NULL DEFAULT 0, created_at
```
`COLLATE NOCASE` on email so `Bob@x.com` can't register over `bob@x.com` — the audit explicitly
tests duplicate registration. Nullable columns = the three optional form fields.

### 000002 sessions
```
id TEXT PRIMARY KEY,                     -- uuid, the value in the cookie
user_id → users ON DELETE CASCADE, created_at, expires_at TEXT NOT NULL
INDEX (user_id), INDEX (expires_at)
```
Server-side sessions, not JWT. The audit tests two browsers logged in as different users
simultaneously and surviving a refresh — a row per session is the straightforward way.

### 000003 follows
```
id, follower_id → users, followee_id → users,
status TEXT CHECK (status IN ('pending','accepted')),
created_at, UNIQUE (follower_id, followee_id), CHECK (follower_id <> followee_id)
```
One table for both requests and accepted follows. Public profile → insert `accepted` directly;
private → insert `pending` and notify. Unfollow = delete the row.

### 000004 posts
```
id, author_id → users, content TEXT NOT NULL, image_path TEXT,
privacy TEXT NOT NULL CHECK (privacy IN ('public','almost_private','private')),
created_at, INDEX (author_id, created_at DESC)
```

### 000005 post_viewers
```
post_id → posts ON DELETE CASCADE, user_id → users ON DELETE CASCADE,
PRIMARY KEY (post_id, user_id)
```
Only populated for `privacy = 'private'` — the hand-picked followers.

### 000006 comments
```
id, post_id → posts ON DELETE CASCADE, author_id → users,
content TEXT NOT NULL, image_path TEXT, created_at, INDEX (post_id, created_at)
```
Comments carry images too — the audit tests it separately from post images.

### 000007 groups
```
id, creator_id → users, title TEXT NOT NULL, description TEXT NOT NULL, created_at
```

### 000008 group_members
```
id, group_id → groups ON DELETE CASCADE, user_id → users ON DELETE CASCADE,
status TEXT CHECK (status IN ('invited','requested','member')),
role   TEXT CHECK (role IN ('creator','member')),
invited_by INTEGER NULL → users, created_at,
UNIQUE (group_id, user_id)
```
One table covers membership, pending invitations, and pending join requests. The `status` column
is the whole groups state machine — see Phase 6.

### 000009 group_posts / 000010 group_comments
Same shape as `posts`/`comments` minus `privacy` (group posts are visible to members, full stop),
plus `group_id`.

> **Tradeoff, decided:** separate tables rather than `posts.group_id NULL`. Keeps the `groups`
> module owning its own SQL (design doc §3.4, "modules never import modules") and keeps the
> global feed query free of `WHERE group_id IS NULL`. Cost: ~40 lines of near-duplicate store
> code. Accepted.

### 000011 events
```
id, group_id → groups ON DELETE CASCADE, creator_id → users,
title TEXT NOT NULL, description TEXT NOT NULL, event_time TEXT NOT NULL, created_at
```

### 000012 event_responses
```
event_id → events ON DELETE CASCADE, user_id → users ON DELETE CASCADE,
response TEXT CHECK (response IN ('going','not_going')),
PRIMARY KEY (event_id, user_id)
```
Composite PK = one vote per user, changeable via `INSERT ... ON CONFLICT DO UPDATE`.

### 000013 direct_messages
```
id, sender_id → users, recipient_id → users, content TEXT NOT NULL,
created_at, read_at TEXT NULL,
INDEX (sender_id, recipient_id, created_at)
```
No `conversations` table. A conversation is just the message pairs; adding a parent table buys
nothing until you need per-conversation metadata.

### 000014 group_messages
```
id, group_id → groups ON DELETE CASCADE, sender_id → users,
content TEXT NOT NULL, created_at, INDEX (group_id, created_at)
```

### 000015 notifications
```
id, user_id → users ON DELETE CASCADE,          -- recipient
type TEXT CHECK (type IN ('follow_request','group_invite','group_join_request','event_created')),
actor_id → users, entity_id INTEGER NULL,        -- the follow/group/event row it points at
read_at TEXT NULL, created_at,
INDEX (user_id, read_at)
```
`entity_id` is deliberately untyped — `type` tells you what table it points into. A polymorphic
FK isn't expressible in SQLite anyway.

---

## Tasks

- [ ] Write all 15 up/down pairs.
- [ ] `go run ./src/server/cmd/migrate up` → clean.
- [ ] `... migrate down` all the way → clean, empty DB. **This is the test that catches bad
      down-migrations**, and you only get one chance to find out before you need it.
- [ ] `... migrate up` again → clean.
- [ ] Sketch the ERD (design doc §"entity relationship diagram") into `Docs/` — the audit
      mentions it, and drawing it surfaces missing indexes.
- [ ] `pkg/db/seed/` — a few users who follow each other, a group, some posts. Not a migration;
      invoked explicitly via `cmd/migrate seed`. Audit bonus, and you'll use it every day.

## Done when

```bash
sqlite3 src/server/data/social.db ".schema" | grep -c "CREATE TABLE"   # 16 (15 + schema_migrations)
sqlite3 src/server/data/social.db "PRAGMA foreign_key_check;"          # silence
```
