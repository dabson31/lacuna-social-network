# Phase 4 — Users & follows

**Goal:** profiles, the public/private toggle, and the follow state machine — including
`CanView`, the single most reused rule in the app.

---

## Why follows comes before posts

`CanView(viewer, owner)` gates profiles, "almost private" posts, *and* who may start a chat.
Write it once here, in `modules/follows`, and the next three phases consume it through a
consumer-declared interface. Write it later and you'll have three subtly different copies.

```go
// declared in modules/posts, modules/chat, modules/users — each in its OWN package
type FollowChecker interface {
    IsFollowing(ctx context.Context, followerID, followeeID int64) (bool, error)
}
```
`app/` passes in the `follows` store. The provider never grows methods to please a consumer.

---

## Tasks

### `internal/modules/users`
- [ ] `GET /api/users/{id}` — profile. Response shape depends on visibility:
  - own profile → everything except `password_hash`
  - public profile → everything except password
  - private + viewer is an accepted follower → same
  - private + not a follower → **id, name, avatar, is_public only**, plus follower counts.
    Return 200 with the stub, not 403 — the UI needs to render a "request to follow" button.
- [ ] `GET /api/users/{id}/posts` — the author's posts, filtered by the viewer's access.
- [ ] `PATCH /api/users/me` — update profile fields.
- [ ] `PATCH /api/users/me/privacy` — the public↔private toggle. Own profile only.
- [ ] `GET /api/users` — browse/search users (the frontend needs it for invites and DMs).

### `internal/modules/follows`
- [ ] `POST /api/users/{id}/follow`
  - target is **public** → insert `status='accepted'`, done, no notification needed
  - target is **private** → insert `status='pending'`, publish `follow.requested` to the event bus
  - already following or already pending → 409, idempotent-ish
  - following yourself → 400 (the CHECK constraint backs you up, but fail cleanly)
- [ ] `DELETE /api/users/{id}/follow` — unfollow. Deletes the row whether accepted or pending
      (that doubles as "cancel my request").
- [ ] `POST /api/follow-requests/{id}/accept` → `status='accepted'`
- [ ] `POST /api/follow-requests/{id}/decline` → delete the row
- [ ] Both: only the **followee** may accept/decline. Check it in the service.
- [ ] `GET /api/follow-requests` — incoming pending requests.
- [ ] `GET /api/users/{id}/followers`, `GET /api/users/{id}/following` — subject to the same
      `CanView` rule as the profile itself.
- [ ] `CanView(ctx, viewerID, ownerID) (bool, error)` — true if owner is public, viewer *is* the
      owner, or viewer is an accepted follower. One function, one place.

### Tests
- [ ] `follows/service_test.go`: public auto-follow, private creates pending, non-followee can't
      accept, unfollow removes, self-follow rejected.
- [ ] `CanView` truth table — all four rows.

---

## Done when

Two curl sessions (two cookie jars) reproduce every follow checkbox in the audit:
- follow a public user → immediately following, no request
- follow a private user → request appears in their `/api/follow-requests`
- they accept → follower lists update on both sides
- unfollow works
- a non-follower fetching a private profile gets the stub, not the full record
