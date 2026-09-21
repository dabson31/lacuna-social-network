# Phase 8 — Notifications

**Goal:** the four required notifications, delivered live over the existing socket and readable
on any page.

Small phase — most of the work was done when you published events in Phases 4, 6 and 7.

---

## The event bus

This is what keeps `follows` and `groups` from importing `notifications` (design doc §3.4).

### `internal/platform/events`
- [ ] Tiny in-process pub/sub:
  ```go
  type Bus interface {
      Publish(ctx context.Context, e Event)
      Subscribe(kind string, h Handler)
  }
  ```
- [ ] `Publish` is **fire-and-forget and must never fail the caller**. A notification that
      doesn't get written must not roll back the follow request that caused it.
- [ ] Dispatch in a goroutine with its own recover. Subscribers run after the publisher's
      transaction commits — don't publish inside a `tx`.

### Wire-up in `app/`
`app.New` subscribes `notifications` to the four kinds. That's the only place both packages are
named together.

---

## The four required notifications

| Published by | Event kind | Recipient | Trigger |
|---|---|---|---|
| `follows` | `follow.requested` | the followee | someone requests to follow a **private** profile |
| `groups` | `group.invited` | the invitee | invited to a group |
| `groups` | `group.join_requested` | the group **creator** | someone asks to join |
| `groups` | `group.event_created` | every member except the creator | an event is created |

Extras are welcomed by the subject and are an audit bonus — `follow.accepted` and
`post.commented` are the cheap two, both one extra `Publish` call.

---

## Tasks

- [ ] `internal/modules/notifications` — `handler.go` / `service.go` / `store.go` / `model.go`.
- [ ] Subscriber writes a row to `notifications`, then `hub.SendToUser(recipientID, ...)` with
      `{"type": "notification.new", ...}`. Persist **before** pushing: an offline user must still
      see it on next login.
- [ ] `GET /api/notifications` — newest first, paginated, with `unread_count`.
- [ ] `POST /api/notifications/{id}/read` and `POST /api/notifications/read-all`.
- [ ] Actionable notifications (follow request, group invite, join request) carry `entity_id` so
      the frontend can render accept/decline inline without a second fetch.
- [ ] Verify the four Publish calls actually exist in Phases 4 and 6 — easiest thing in the
      project to leave as a TODO and forget:
      `grep -rn "Publish(" src/server/internal/modules/`
- [ ] "New notifications are different from new private messages" — keep them as distinct
      socket `type` values and distinct counters. Don't route DMs through `notifications`.

### Tests
- [ ] Publishing `follow.requested` writes exactly one row for the right recipient.
- [ ] A subscriber that panics doesn't take the process down.

---

## Done when

Two browsers/jars: a follow request to a private user, a group invite, a join request, and an
event creation each produce exactly one notification row for the right person, arrive live on
the socket, and `unread_count` drops when marked read.
