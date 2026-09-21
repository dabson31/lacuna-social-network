# API contract — reference

Every endpoint in one place. Keep this updated as you build; it becomes the spec the frontend
track works against, and it's what you'd hand to `shared/contracts/`.

- All paths prefixed `/api` except `/ws` and `/uploads/*`.
- All authenticated routes use the session cookie (`credentials: "include"` on the client side).
- `A` = requires auth. Everything is `A` except register and login.
- Error envelope, everywhere: `{"error": {"code": "...", "message": "...", "fields": {...}}}`

| Status | Meaning here |
|---|---|
| 400 | malformed input |
| 401 | no/expired session |
| 403 | authenticated but not allowed |
| 404 | not found **or** not visible to you (privacy — never leak existence) |
| 409 | conflict: duplicate email, already following, already a member |
| 413 | upload too large |
| 415 | not a JPEG/PNG/GIF |

---

## Auth — Phase 3
| Method | Path | A | Notes |
|---|---|---|---|
| POST | `/auth/register` | | multipart; avatar/nickname/about_me optional |
| POST | `/auth/login` | | email + password |
| POST | `/auth/logout` | A | destroys row **and** clears cookie |
| GET | `/auth/me` | A | session bootstrap |

## Users & follows — Phase 4
| Method | Path | A | Notes |
|---|---|---|---|
| GET | `/users` | A | browse / search |
| GET | `/users/{id}` | A | full or stub, per `CanView` |
| GET | `/users/{id}/posts` | A | filtered by visibility |
| PATCH | `/users/me` | A | edit profile |
| PATCH | `/users/me/privacy` | A | public ↔ private toggle |
| POST | `/users/{id}/follow` | A | auto-accept if target public |
| DELETE | `/users/{id}/follow` | A | unfollow, or cancel a pending request |
| GET | `/users/{id}/followers` | A | |
| GET | `/users/{id}/following` | A | |
| GET | `/follow-requests` | A | incoming, pending |
| POST | `/follow-requests/{id}/accept` | A | followee only |
| POST | `/follow-requests/{id}/decline` | A | followee only |

## Posts — Phase 5
| Method | Path | A | Notes |
|---|---|---|---|
| POST | `/posts` | A | multipart; `privacy`, `viewer_ids[]` when private |
| GET | `/posts` | A | feed, visibility predicate + pagination |
| GET | `/posts/{id}` | A | 404 when not visible |
| POST | `/posts/{id}/comments` | A | multipart, image optional |
| GET | `/posts/{id}/comments` | A | |
| GET | `/uploads/{file}` | | static file server |

## Groups — Phase 6
| Method | Path | A | Notes |
|---|---|---|---|
| POST | `/groups` | A | |
| GET | `/groups` | A | browse all + caller's membership status |
| GET | `/groups/{id}` | A | non-members get a stub |
| POST | `/groups/{id}/invite` | A | any member may invite |
| POST | `/groups/{id}/invitations/accept` \| `/decline` | A | invitee |
| GET | `/invitations` | A | caller's pending invites |
| POST | `/groups/{id}/join` | A | creates a join request |
| GET | `/groups/{id}/requests` | A | creator only |
| POST | `/groups/{id}/requests/{userID}/accept` \| `/decline` | A | creator only |
| DELETE | `/groups/{id}/membership` | A | leave |
| POST/GET | `/groups/{id}/posts` | A | members only |
| POST/GET | `/groups/{id}/posts/{postID}/comments` | A | members only |
| POST/GET | `/groups/{id}/events` | A | members only |
| POST | `/events/{id}/respond` | A | `going` \| `not_going`, upsert |

## Chat — Phase 7
| Method | Path | A | Notes |
|---|---|---|---|
| GET | `/ws` | A | single multiplexed socket; auth **before** upgrade |
| GET | `/conversations` | A | chat partners + unread counts |
| POST | `/messages` | A | DM; requires a follow relation either way |
| GET | `/messages/{userID}` | A | history |
| POST/GET | `/groups/{id}/messages` | A | members only |

## Notifications — Phase 8
| Method | Path | A | Notes |
|---|---|---|---|
| GET | `/notifications` | A | + `unread_count` |
| POST | `/notifications/{id}/read` | A | |
| POST | `/notifications/read-all` | A | |

---

## WebSocket message types

Tagged union on `type`. Mirror these constants in `src/shared/constants/`.

| `type` | Direction | Payload |
|---|---|---|
| `dm.message` | S→C | message, sender, conversation |
| `dm.typing` | C↔S | optional |
| `group.message` | S→C | message, sender, group_id |
| `notification.new` | S→C | notification row |
| `presence.online` / `presence.offline` | S→C | optional |
| `ping` / `pong` | C↔S | keepalive |

Notifications and DMs stay separate types and separate counters — the subject requires them to
be distinguishable.
