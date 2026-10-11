# QA Reference Map

A working index built while triaging Ali Watheqi's QA pass. It is a *lookup table*, not an
explanation — `Docs/Structure Walkthrough.md` is the explanation. The last section records the
verdict on each reported problem and where the fix landed.

---

## 1. Backend: route → handler → service → store

Every route is mounted in [`internal/app/router.go`](../src/server/internal/app/router.go); one
`mount*` function per module, and `app.New` is the only place that knows all seven modules exist.

| route | handler | service | the rule that decides |
|---|---|---|---|
| `POST /api/auth/register` | `auth.Handler.Register` | `auth.Service.Register` | `validateRegister`, bcrypt, `storage.Save` |
| `POST /api/auth/login` | `auth.Handler.Login` | `auth.Service.Login` | one `ErrInvalidCredentials` for both halves, dummy-hash compare |
| `POST /api/auth/logout` | `auth.Handler.Logout` | `auth.Service.Logout` | `session.Store.Destroy` + `ClearCookie` |
| `GET /api/auth/me` | `auth.Handler.Me` | `auth.Service.Me` | `RequireAuth` already proved the session |
| `GET /api/users/{id}` | `users.Handler.Get` | `users.Service.Get` | `canView = isSelf \|\| isPublic \|\| status=="accepted"` |
| `PATCH /api/users/me` | `users.Handler.UpdateMe` | `users.Service.Update` | `validateUpdate` |
| `PATCH /api/users/me/privacy` | `users.Handler.SetPrivacy` | `users.Service.SetPrivacy` | self only, by construction |
| `GET /api/users` | `users.Handler.Search` | `users.Service.Search` | clamps limit, drops self |
| `POST/DELETE /api/users/{id}/follow` | `follows.Handler.Follow/Unfollow` | `follows.Service` | public ⇒ `accepted`, private ⇒ `pending` |
| `GET /api/users/{id}/followers\|following` | `follows.Handler` | `follows.Service.Followers/Following` | **`follows.Service.canView`** — the one owner of "can A see B?" |
| `GET /api/follow-requests/*` | `follows.Handler` | `follows.Service.Incoming/Accept/Decline` | only the addressee may decide |
| `POST /api/posts`, `GET /api/posts` | `posts.Handler.Create/Feed` | `posts.Service` | `validatePost`; SQL `visible` predicate |
| `GET /api/users/{id}/posts` | `posts.Handler.ByAuthor` | `posts.Service.AuthorPosts` | `visible` **+ profile visibility** (added — §4, item 3) |
| `DELETE /api/posts/{id}` | `posts.Handler.Delete` | `posts.Service.Delete` | author-scoped in the `WHERE` |
| `POST/GET /api/posts/{id}/comments` | `posts.Handler` | `posts.Service.Comment/Comments` | `store.canSee` reuses `visible` |
| `POST /api/groups`, `GET /api/groups`, `GET /api/groups/{id}` | `groups.Handler` | `groups.Service` | `validateGroup` |
| group invite / join / requests / membership | `groups.Handler` | `groups.Service` + `service_members.go` | `requireMember`, `requireCreator` |
| `POST/GET /api/groups/{id}/posts` (+comments) | `groups.Handler` | `groups.Service` | `requireMember` |
| `POST /api/groups/{id}/events` | `groups.Handler.CreateEvent` | `groups.Service.CreateEvent` | title, **description** and RFC-3339 `event_time` all required |
| `GET /api/groups/{id}/events` | `groups.Handler.Events` | `groups.Service.Events` | `requireMember` |
| `POST /api/events/{id}/respond` | `groups.Handler.Respond` | `groups.Service.Respond` | member of the event's group |
| `POST /api/messages`, `GET /api/messages/{userID}` | `chat.Handler` | `chat.Service` | ports: `FollowChecker`, `ProfileChecker` |
| `GET /api/conversations` | `chat.Handler.Inbox` | `chat.Service.Inbox` | `store.conversations` + `store.groupConversations` |
| `POST /api/groups/{id}/messages` (+read) | `chat.Handler` | `chat.Service` | port: `GroupChecker` |
| `GET /api/notifications`, `read`, `read-all` | `notifications.Handler` | `notifications.Service` | owner-scoped in SQL |
| `GET /ws` | `realtime.Gateway.ServeWS` | — | behind `requireAuth`, `CheckOrigin` = `ALLOWED_ORIGIN` |

### Platform pieces worth knowing by name

- [`platform/session/session.go`](../src/server/internal/platform/session/session.go) —
  `Create` (sweeps expired rows, then inserts), `Get`, `Destroy`, `SetCookie`, `ClearCookie`,
  and the `WithUserID`/`UserID` context pair. **No unique constraint on `sessions.user_id`**, so
  one account can hold many live sessions.
- [`app/middleware/middleware.go`](../src/server/internal/app/middleware/middleware.go) —
  `RequestLog` (+`statusRecorder`, which re-exposes `Hijack`), `Recover`, `CORS`, `MaxBodyBytes`,
  `RequireAuth`. `RequireAuth` clears a dead cookie before answering 401.
- [`platform/realtime/`](../src/server/internal/platform/realtime) — `Hub` keys
  `map[userID]map[*Conn]struct{}`, so two browsers on one account both receive. `Gateway`
  upgrades; `Conn` runs the read/write pumps.
- [`platform/events/events.go`](../src/server/internal/platform/events/events.go) — the bus.
  Four kinds: `FollowRequested`, `GroupInvited`, `GroupJoinRequested`, `EventCreated`. Publish is
  fire-and-forget; `notifications.Service.Record` is the only subscriber.

---

## 2. Frontend: route → page → data source

| route | file | where its data comes from |
|---|---|---|
| `/login`, `/register` | `app/(auth)/*/page.tsx` | own `SessionProvider`, `features/auth/api.ts` |
| `/feed` | `app/(main)/feed/page.tsx` | `useCursorList` over `listFeed` |
| `/profile` | `app/(main)/profile/page.tsx` | redirects to own id |
| `/profile/[userId]` | `app/(main)/profile/[userId]/page.tsx` | `getProfile`, `listByAuthor`, `listFollowers/Following` |
| `/directory` | `app/(main)/directory/page.tsx` | `searchUsers` |
| `/groups` | `app/(main)/groups/page.tsx` | `useBadges().myGroups` |
| `/groups/[groupId]` | `app/(main)/groups/[groupId]/page.tsx` | `getGroup`, `useEventFanIn` |
| `/groups/[groupId]/chat` | `app/(main)/groups/[groupId]/chat/page.tsx` | `useThread` |
| `/events` | `app/(main)/events/page.tsx` | `useEventFanIn`, `useBadges().myGroups` |
| `/chat` | `app/(main)/chat/page.tsx` | `useBadges().inbox` |
| `/chat/[conversationId]` | `app/(main)/chat/[conversationId]/page.tsx` | `useThread` |
| `/notifications` | `app/(main)/notifications/page.tsx` | `features/notifications/api.ts` |

### The four providers, in nesting order

`app/(main)/layout.tsx` →
`SessionProvider` › `ToastProvider` › `SocketProvider` › `BadgeProvider` › `Shell`.

- **`lib/session/SessionProvider.tsx`** — `loading | authenticated | anonymous`; registers the
  global 401 handler. `useUser()` throws outside an authenticated route, which is why `Shell`
  renders children only on `authenticated`.
- **`components/ui/Toast.tsx`** — `useToast()(message)`, 2600 ms. The only popup primitive.
- **`lib/realtime/SocketProvider.tsx`** — one socket; `useFrame(type, handler, deps)` subscribes
  to one frame type with a narrowed payload.
- **`components/layout/BadgeProvider.tsx`** — the counters every route shows: `pending`
  (notifications), `unread` (DMs), `unreadGroups` (group rooms), `groups`, plus `notifications`,
  `inbox`, `myGroups` and the three `reload*` callbacks. Also where live frames become toasts.

### Shell chrome

`Chrome` (top bar: clock, socket readout, notices link, logout) · `RailLeft` (the seven
`CHANNELS` tiles, counts from `useBadges`) · `RailRight` (nine panels, `PendingPanel` first) ·
`StatusBar` · `MainHead` (crumb from `channelFor(pathname)`).

### The one fetch and the one error model

`lib/api/client.ts` — `json()`, `form()`, `query()`, `uploadUrl()`, `setUnauthorizedHandler()`.
Always `credentials: 'include'`; `isAuthProbe` keeps `/api/auth/login` and `/api/auth/me` out of
the global 401 redirect. `lib/api/errors.ts` — `ApiError{status, code, fields}`, `NetworkError`,
`isApiError`, `fieldErrors`. `lib/api/describe.ts` — `describeError` → `notice | empty | fields |
auth`. `components/ui/ErrorView.tsx` — `ErrorView`, `FieldError`, `FormError`.

---

## 3. Where each audit requirement is actually enforced

| requirement | enforced in |
|---|---|
| sessions, two browsers independent | `platform/session`, `RequireAuth` |
| follow a private user ⇒ request | `follows.Service.Follow` |
| accept/decline a follow request | `follows.Service.Accept/Decline` |
| private profile hidden from non-followers | `users.Service.Get` (`canView`), `follows.Service.Followers/Following`, `posts.Service.AuthorPosts` |
| three post privacy levels | the `visible` SQL predicate in `posts/store.go` |
| private post names its viewers | `posts.Service.Create` + `store.acceptedAmong` |
| any member may invite | `groups` service_members, never gated on role |
| event needs title, description, day/time, two options | `groups.Service.CreateEvent`, `EventCard` |
| no chat between unconnected users | `chat.Service` via its three ports |
| only the targeted user receives a DM | `realtime.Hub.SendToUser` |
| notifications on every page | `(main)/layout.tsx` → `BadgeProvider` + `RailRight.PendingPanel` + `Chrome` notices link |
| four required notification kinds | `platform/events` → `notifications.Service.Record` |

---

## 4. Verdict on the reported problems

| # | report | verdict |
|---|---|---|
| 1 | logout does not redirect | already fixed before this pass — `Chrome.onLogout` awaits logout, calls `signOut()`, then `router.replace('/login')`, and `Shell` redirects on `anonymous` as a second guard. No change made. |
| 2 | same account in a second browser ⇒ 401 | **not reproducible; no change made.** Three concurrent sessions for one account were opened against a live server and `GET /api/auth/me` answered 200 on all three; `sessions.user_id` carries no unique constraint and the hub holds a *set* of sockets per user. A dead cookie is already cleared by `RequireAuth` before it answers 401. The 401 visible in the network monitor is the deliberate `/api/auth/me` cold-load probe, which `isAuthProbe` keeps out of the global 401 redirect. Left alone rather than guessed at. |
| 3 | private profile shown to the public | **confirmed and fixed.** `GET /api/users/{id}/posts` applied only per-post privacy, so a stranger opening a private account's profile saw that account's `public` posts listed under the "This record is private" notice. Gated by profile visibility — see `posts/ports.go`. |
| 4 | event form fails silently on a missing field | **confirmed and fixed.** `description` had no `required` and the catch showed a generic toast. |
| 5 | per-group unread counts not shown | **confirmed and fixed.** The API already returned `inbox.groups[].unread`; nothing outside `/chat` read it, and the Messages tile counted DMs only. |
| 6 | no pop-up notifications | **added.** Live frames now raise a toast, and the chrome carries a notices count top-right. |

---

## 5. Fix log

Two lines per item: what was wrong, and what was done about it.

**1 · Logging out did not redirect to the login page.**
- Issue: fixed in an earlier pass, before this triage — `signOut()` flipped the session to
  `anonymous` synchronously while `router.replace('/login')` was still in flight, and the route
  re-rendered in the gap.
- Fix: none needed now. `Chrome.onLogout` awaits the logout call, calls `signOut()`, then
  `router.replace('/login')`; `(main)/layout.tsx` gates children on `authenticated` and
  redirects on `anonymous` as a second guard; `RequireAuth` clears the dead cookie so
  `middleware.ts` cannot bounce the reader back to `/feed`.

**2 · Logging in with the same account in a second browser showed 401.**
- Issue: not reproducible. Three concurrent sessions were opened for one account against a
  running server and `GET /api/auth/me` answered 200 on all three; `sessions.user_id` has no
  unique constraint and `realtime.Hub` holds a *set* of sockets per user id, so neither layer
  evicts the first browser. The 401 in the network monitor is `/api/auth/me` on a cold page
  load, which is the deliberate "nobody is logged in yet" answer and is excluded from the
  global 401 redirect by `isAuthProbe` in `lib/api/client.ts`.
- Fix: **none made, on purpose.** Changing session handling on a guess would risk the two
  behaviours the audit does test — two browsers staying independent, and sessions surviving a
  restart. Needs the exact reproduction steps and the failing request's path before touching it.

**3 · A private account's profile was shown to the public.**
- Issue: `GET /api/users/{id}/posts` applied only the per-post `visible` predicate, under which
  a private account's `public` posts pass for anyone. A stranger opening a private profile read
  "This record is private" and then that account's posts listed directly underneath it.
- Fix: `posts.Service.AuthorPosts` now applies the profile rule before the post rule, through
  the new `posts.ProfileVisibility` port satisfied by `follows.Service.CanViewProfile` — so the
  post list, the profile header and the follower lists are all decided by the one owner of
  "can A see B?". A viewer who may not look gets an empty page, not an error, because the page
  must still render the follow button. The feed is untouched: a public post stays public.

**4 · A missing field on the event form failed silently.**
- Issue: `description` carried no `required` attribute while `groups.Service.CreateEvent`
  rejects an empty one, and the catch block showed the generic toast "that event did not save",
  so the per-field reasons in the 400 were thrown away and a failed submit looked like nothing
  had happened at all.
- Fix: `required` on the description textarea, so the browser's own popup blocks the submit,
  and the catch now reads `fieldErrors(err)` and names what the service rejected —
  "that event is incomplete · description is required".

**5 · Per-group unread message counts were not shown.**
- Issue: `GET /api/conversations` has always returned `groups[].unread` per room, counted off
  `group_members.last_read_message_id`, but only `/chat` read it. The Messages tile summed
  direct messages only, and `/groups` showed no unread state at all, so a room with unread
  messages looked identical to a quiet one.
- Fix: `BadgeProvider` now exposes `unreadGroups` alongside `unread`; the Messages tile counts
  both; `/groups` draws a lit dot, an "N new" tag and an "N new" suffix on the group-chat
  action per group; and the `/chat` group bar reports its own unread total.

**6 · No pop-up notifications.**
- Issue: every live frame already arrived over the one socket and updated a counter, but
  nothing announced itself — a reader had to be looking at the right tile or the right rail
  panel, and the right rail is below the fold on a phone.
- Fix: `BadgeProvider` raises a toast on `notification.new` (which covers all four required
  kinds: follow requests, group invites, join requests and new events), on `dm.message` and on
  `group.message`, skipping the sender's own echo and the thread already on screen; and the
  chrome carries live `notices` and `inbox` counts top-right on every route, in accent when
  non-zero, using only classes `lacuna.css` already defines.
