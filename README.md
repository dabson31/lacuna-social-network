# social-network

A Facebook-like social network: followers with request-and-accept, public and private profiles,
posts at three privacy levels, groups with invitations and events, private and group chat over a
single WebSocket, and live notifications.

Go and SQLite on one side, Next.js and TypeScript on the other, two Docker images.

## Run it

```bash
./build.sh
```

Builds both images, waits for both healthchecks, then prints the containers, their sizes and a
reachability check. `--fresh` discards the database volume first, `--down` stops everything and
keeps it. By hand it is `docker compose up -d --wait`.

The app is at **http://localhost:3000**, the API at **http://localhost:8080**.

## Run it without Docker

Two terminals. Migrations run at boot, so there is no setup step.

```bash
cp .env.example .env
go run ./src/server/cmd/server          # :8080
```

```bash
cd src/web && npm install
npm run dev                             # :3000
```

`go run ./src/server/cmd/migrate up | down | force <version>` drives migrations by hand.

> **On Windows this fails at startup.** `sqlite.go` builds its migrations URL as
> `"file://" + <absolute path>`, and `file://C:\Users\...` is not a parseable URL — the parser
> reads `C` as the host. Use Docker there. Detail in `Docs/Structure Walkthrough.md` §14.

## Layout

```
src/
├── server/   # Go: HTTP, the WebSocket hub, SQLite. the only thing that owns state
│            #   cmd/ binaries · internal/{app,modules,platform} · pkg/db/migrations
├── web/      # Next.js: routing and rendering, owns no state
│            #   app/ 14 routes · features/ · components/ · lib/
└── shared/   # the contract both sides read: TS types and the strings they agree on
```

`modules/` and `features/` carry the same seven names — auth, users/profile, follows, posts,
groups, chat, notifications — so a change maps to one folder per side. Folders are named for
what they are responsible for rather than where they run; the docs below argue the case.

## Configuration

`.env.example` documents the backend variables; Docker sets them in `docker-compose.yml`.

| variable | default | notes |
|---|---|---|
| `APP_ENV` | `development` | `production` turns on JSON logs and Secure cookies |
| `PORT` | `8080` | |
| `DB_PATH` | `src/server/data/social.db` | the parent directory is created if missing |
| `MIGRATIONS_PATH` | `src/server/pkg/db/migrations/sqlite` | read at every boot |
| `UPLOAD_DIR` | `src/server/data/uploads` | served from `/uploads/{name}` |
| `SESSION_TTL` | `168h` | sessions are rows, so they survive a restart |
| `ALLOWED_ORIGIN` | `http://localhost:3000` | gates CORS **and** the WebSocket handshake |

The frontend takes one, `NEXT_PUBLIC_API_URL`. It is inlined into the browser bundle at build
time, so it has to be the URL the *browser* can reach: `localhost:8080`, never `backend:8080`.

## Docs

- `Docs/Architectural Design.md` — the contract. What each folder is for and what was rejected.
- `Docs/Structure Walkthrough.md` — the explanation. How a request travels, why the layering is
  one-directional, which framework does what, and the current state with its known gaps.

## Known gaps

- **No tests.** `testutil/` is empty. The hub in particular wants `go test -race ./...`.
- **Neither image has actually been built** — compose validates, every build input was checked
  and the frontend's runtime layout was reproduced by hand, but no Docker daemon was available.
- **Windows**, as above.
- The socket retries forever after a 401: a refused handshake reports close code 1006, not the
  1008 the client checks for. Noise, not a broken flow.
