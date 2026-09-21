# Phase 6 — Groups, events, RSVP

**Goal:** the largest module. Group lifecycle, two different ways in, group content, events.

Budget three sittings. If `service.go` passes ~400 lines, split *within* the package
(`service_members.go`, `service_events.go`) — not into a new package (design doc §7).

---

## The membership state machine

One table, `group_members`, one `status` column. Everything below is a transition on it.

```
                 creator creates group
                          │
                          ▼
              (creator_id, status='member', role='creator')

  member invites user          non-member requests to join
          │                               │
          ▼                               ▼
    status='invited'                status='requested'
          │                               │
   invitee accepts │ declines      creator accepts │ declines
          ▼        ▼                      ▼        ▼
    status='member'  row deleted    status='member'  row deleted
```

Getting this into one table is why groups is big but not complicated. Two rules:
- **Anyone already a member may invite** (subject requirement, easy to miss).
- **Only the creator may accept join requests.**

---

## Tasks

### Group lifecycle
- [ ] `POST /api/groups` — title + description. Creator row inserted in the same transaction.
- [ ] `GET /api/groups` — browse **all** groups. The subject requires this: a user has to be
      able to find a group before requesting to join it. Include the caller's membership status
      in each row so the UI knows which button to show.
- [ ] `GET /api/groups/{id}` — details + members. Non-members see title/description/member count
      only.
- [ ] `DELETE /api/groups/{id}` — creator only. Optional.

### Invitations
- [ ] `POST /api/groups/{id}/invite` (body: `user_id`) — caller must be a member. Insert
      `status='invited'`, publish `group.invited`.
- [ ] `POST /api/groups/{id}/invitations/accept` → `member` · `.../decline` → delete row.
- [ ] `GET /api/invitations` — the caller's pending invitations.

### Join requests
- [ ] `POST /api/groups/{id}/join` — insert `status='requested'`, publish `group.join_requested`.
- [ ] `POST /api/groups/{id}/requests/{userID}/accept` | `/decline` — **creator only**.
- [ ] `GET /api/groups/{id}/requests` — creator only.
- [ ] `DELETE /api/groups/{id}/membership` — leave the group.

### Group content
- [ ] `POST /api/groups/{id}/posts` + `GET /api/groups/{id}/posts` — members only. No privacy
      column; membership *is* the privacy model.
- [ ] `POST /api/groups/{id}/posts/{postID}/comments` + `GET`.
- [ ] Image upload reuses `platform/storage` unchanged.

### Events
- [ ] `POST /api/groups/{id}/events` — title, description, event_time (RFC 3339). Members only.
      Publish `group.event_created` **to every member except the creator**.
- [ ] `GET /api/groups/{id}/events` — include each event's response counts and the caller's own
      response.
- [ ] `POST /api/events/{id}/respond` — body `{"response": "going" | "not_going"}`.
      `INSERT ... ON CONFLICT (event_id, user_id) DO UPDATE` so changing your mind is one query.

### The authorization helper
- [ ] `membership(ctx, groupID, userID) (status, role, error)` — one function, called at the top
      of every group service method. Every group endpoint's authorization reduces to checking
      its return value. Don't scatter `SELECT status FROM group_members` through the file.

### Tests
- [ ] Non-member cannot read group posts.
- [ ] Non-creator cannot accept a join request.
- [ ] A plain member **can** invite.
- [ ] Changing an RSVP updates rather than duplicates.

---

## Done when

Two users: A creates a group and invites B; B accepts. B invites C; C accepts. A third user D
requests to join and only A can accept. Everyone in the group sees the group's posts; D doesn't
until accepted. An event created by B shows correct going/not-going tallies.
