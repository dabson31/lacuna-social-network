# Phase 3 — Authentication

**Goal:** register → login → the cookie survives a browser refresh and a server restart → logout.
This closes the vertical slice. Everything after Phase 3 is this same pattern with different SQL.

The audit spends more checkboxes here than anywhere else. Get it exactly right.

---

## Tasks

### 1. `internal/platform/password`
- [ ] `Hash(plain string) (string, error)` — bcrypt, cost 10–12.
- [ ] `Compare(hash, plain string) error`.
- [ ] On login failure return the **same** error whether the email is unknown or the password is
      wrong. Different messages leak which emails are registered.

### 2. `internal/platform/session`
- [ ] `Create(ctx, userID) (Session, error)` — uuid v4 token, row in `sessions`, TTL from config.
- [ ] `Get(ctx, token) (Session, error)` — rejects expired rows.
- [ ] `Destroy(ctx, token) error`.
- [ ] `SetCookie(w, s)` / `ClearCookie(w)`. Cookie attributes, all of them:
  - `HttpOnly: true` — JS must not read it
  - `Path: "/"`
  - `SameSite: Lax` — `None` requires `Secure`, which breaks on plain-HTTP localhost
  - `Secure: cfg.Env == "production"`
  - `Expires` matching the DB row
- [ ] Background sweep of expired rows, or delete-on-read. Either is fine; pick one.

> The whole "two browsers stay independently logged in" section of the audit is this one file.
> A bug here looks like a bug in five other features.

### 3. `internal/app/middleware.RequireAuth`
- [ ] Read cookie → `session.Get` → load user → put the user ID in the request `context`.
- [ ] 401 with the standard error envelope when absent or expired.
- [ ] `UserIDFrom(ctx) (int64, bool)` helper so handlers never touch context keys directly.

### 4. `internal/modules/auth`

`POST /api/auth/register`
- [ ] Required: email, password, first_name, last_name, date_of_birth.
      Optional: avatar, nickname, about_me — **must be accepted as empty**, the audit checks it.
- [ ] Multipart, not JSON — the avatar is a file. Use `platform/storage` (built in Phase 5;
      until then accept registration without an avatar and wire it in there).
- [ ] Validate: email shape, password min length, DOB parses as `YYYY-MM-DD` and is in the past.
- [ ] Duplicate email → **409**, not 500. The audit tests re-registering the same user, and a
      raw UNIQUE-constraint 500 fails that check.
- [ ] Return field-level errors (`{"email": "already registered"}`) so the frontend can mark the
      input rather than show a toast.
- [ ] On success: create session, set cookie, return the user.

`POST /api/auth/login`
- [ ] Email + password → session + cookie + user.
- [ ] Wrong email **or** wrong password → 401, identical message.

`POST /api/auth/logout`
- [ ] Destroy the session row **and** clear the cookie. Doing only one of the two is the classic
      half-logout bug: the cookie is gone but the session is still valid, or vice versa.

`GET /api/auth/me`
- [ ] Behind `RequireAuth`. Returns the current user. The frontend's session bootstrap depends on
      this existing.

### 5. Tests
- [ ] `internal/testutil` — in-memory SQLite (`:memory:`) + migrations applied, one helper.
- [ ] `auth/service_test.go`: register happy path, duplicate email, wrong password, expired
      session rejected.

---

## Done when

```bash
curl -c jar -X POST localhost:8080/api/auth/register -F email=a@b.c -F password=secret123 \
     -F first_name=A -F last_name=B -F date_of_birth=1990-01-01
curl -b jar localhost:8080/api/auth/me        # 200, the user
# restart the server
curl -b jar localhost:8080/api/auth/me        # still 200 — sessions live in SQLite, not memory
curl -b jar -X POST localhost:8080/api/auth/logout
curl -b jar localhost:8080/api/auth/me        # 401
```

Re-registering the same email returns 409 with a readable message.
