# Phase 5 — Posts, comments, images

**Goal:** three privacy levels enforced server-side, comments, and JPEG/PNG/GIF upload.

---

## Tasks

### 1. `internal/platform/storage` — build this first
Posts, comments, and avatars all need it.
- [ ] `Save(file multipart.File, header *multipart.FileHeader) (path string, err error)`
- [ ] **Sniff the real type** with `http.DetectContentType` on the first 512 bytes. Never trust
      the filename extension or the client's `Content-Type` header.
- [ ] Allow exactly `image/jpeg`, `image/png`, `image/gif`. Reject everything else with 415.
- [ ] Random filename (uuid + extension derived from the *sniffed* type). Never reuse the
      uploaded name — that's a path-traversal hole and a collision waiting to happen.
- [ ] Write to `cfg.UploadDir` (`src/server/data/uploads/`), return the **relative** path to
      store in the DB.
- [ ] Size cap, enforced by `http.MaxBytesReader` before you read anything.
- [ ] Serve uploads: `GET /uploads/{file}` via `http.FileServer` over that dir. Serve it from a
      dedicated route, not from the app root.
- [ ] Retrofit the avatar into `auth.Register` now.

### 2. `internal/modules/posts`

**Create** — `POST /api/posts`, multipart (content, image?, privacy, viewer_ids[]?)
- [ ] `privacy='private'` → insert the chosen follower ids into `post_viewers`, in the same
      transaction as the post.
- [ ] Validate every id in `viewer_ids` is actually an accepted follower of the author. Don't
      trust the list.
- [ ] Empty content **and** no image → 400.

**Read — the one that matters.** A post is visible to viewer V when:

| privacy | visible to |
|---|---|
| `public` | everyone logged in |
| `almost_private` | the author + accepted followers of the author |
| `private` | the author + user ids listed in `post_viewers` |

- [ ] Express this as a **single reusable SQL predicate**, not as Go filtering after the fetch.
      Filtering in Go breaks pagination (you ask for 20 rows and hand back 7) and gets slow.

```sql
WHERE p.author_id = :viewer
   OR p.privacy = 'public'
   OR (p.privacy = 'almost_private' AND EXISTS (
         SELECT 1 FROM follows f
         WHERE f.follower_id = :viewer AND f.followee_id = p.author_id
           AND f.status = 'accepted'))
   OR (p.privacy = 'private' AND EXISTS (
         SELECT 1 FROM post_viewers pv
         WHERE pv.post_id = p.id AND pv.user_id = :viewer))
```

- [ ] `GET /api/posts` — the feed. Same predicate, `ORDER BY created_at DESC`, cursor or
      limit/offset pagination.
- [ ] `GET /api/posts/{id}` — 404 (not 403) when the viewer may not see it. A 403 confirms the
      post exists.

**Comments**
- [ ] `POST /api/posts/{id}/comments` — multipart, image optional.
- [ ] You may only comment on a post you can see — reuse the same predicate.
- [ ] `GET /api/posts/{id}/comments`.

### Tests
- [ ] `posts/service_test.go`: the visibility matrix, all three levels × (author, follower,
      stranger, listed viewer). Nine-ish cases, and they're the ones the audit hits hardest.
- [ ] `storage_test.go`: a `.png` that's really a PDF gets rejected.

---

## Done when

Three users (author, follower, stranger) with three posts (one per privacy level) produce
exactly the right 3×3 grid of visible/not-visible through the HTTP API, and an uploaded GIF
comes back from `/uploads/...`.
