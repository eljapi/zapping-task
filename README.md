# Infinite Stream — HLS Live Stream Simulator

A Go microservice that simulates an HLS live stream from a fixed set of
pre-encoded segments, plus a browser player built on HLS.js.

The source material is a finite VOD playlist of 64 ten-second segments. The
service turns it into an endless live stream: it serves a rolling 30-second
window (3 segments) and slides that window forward every 10 seconds,
incrementing `EXT-X-MEDIA-SEQUENCE` on each slide, exactly as a real live
encoder would.

## Status

| Part | State |
|---|---|
| HLS playlist generation + sliding window | done |
| Segment serving (streamed from disk, range requests, path-traversal guard) | done |
| Player page (HLS.js) | done |
| Postgres data access layer + goose migrations | done |
| Password hashing, session cookie, auth middleware | done |
| Signup / login pages | done |
| Config from environment (`internal/config`) | done |
| Dockerfile + docker compose (app + db) | done |

## Requirements

- Go 1.25 or newer (only for running the app outside Docker).
- Docker with Compose — runs Postgres alone, or the whole stack.
- The HLS media archive. **None of it is in this repository** — half a gigabyte
  of video does not belong in git, and neither does the playlist that indexes it.
  Unzip it wherever you like; `SEGMENTS_DIR` is how you say where. The server
  refuses to start if that directory is missing a file the playlist names.

## Running

### Full stack in Docker

Unzip the media archive anywhere on the machine, then:

```bash
cp .env.example .env
# set SEGMENTS_DIR to the directory the archive produced, the one
# holding segment.m3u8 and the 64 .ts files it names
docker compose up --build
```

`.env.example` is otherwise ready to use — every other line in it already
carries the value the app would pick on its own, so `SEGMENTS_DIR` is the only
one to fill in. It takes an unquoted path, spaces included. Open
<http://localhost:8080>, register an account and the player starts.

Passing the variable inline instead (`SEGMENTS_DIR=… docker compose up`) works
just as well for the first command, but compose interpolates the file on every
invocation, so `logs`, `ps` and `down` would each need it too. The `.env` file
is there so they do not.

`docker compose up` builds the app image and starts two services: `db`
(plain Postgres 17) and `app` (the Go binary, frontend included). The app runs
its embedded goose migrations against the database on startup, so the schema is
created on first boot and left alone afterwards. The `.ts` files are not in the
image — they are bind-mounted read-only into `/segments` from `SEGMENTS_DIR`.
A relative path there is resolved by compose against `docker-compose.yml`, not
against the shell's working directory, so an absolute one is the safer thing to
give it. Leaving the variable unset stops compose before anything is built,
naming what it wanted.

### App on the host, Postgres in Docker

```bash
docker compose up -d db                                  # Postgres on :5432
SEGMENTS_DIR="/path/to/hls test" go run ./cmd/server     # listens on :8080
```

`WEB_DIR` defaults to a relative path, so run the server from the repository
root. The server applies migrations itself, so a bare `db`
container with an empty volume is enough.

### Migrations

Schema changes live in `internal/db/migrations/` as numbered goose files and are
embedded into the binary (`//go:embed`). `db.Migrate` runs the pending ones at
startup, tracking applied versions in a `goose_db_version` table, so a restart
is a no-op. A new migration is just a numbered file with `-- +goose Up` and
`-- +goose Down` sections; the goose CLI scaffolds one without being a project
dependency:

```bash
go run github.com/pressly/goose/v3/cmd/goose@latest \
  -dir internal/db/migrations create add_something sql
```

### Configuration

`SEGMENTS_DIR` is required; every other setting falls back to a working local
default, so it is the only one that has to be set for development.
`internal/config` reads them once at startup, parses and validates, and fails
fast on a bad value.

| Variable | Default | Notes |
|---|---|---|
| `LISTEN_ADDR` | `:8080` | Address the HTTP server binds to |
| `DATABASE_URL` | `postgres://zapping:zapping@localhost:5432/zapping` | |
| `SEGMENTS_DIR` | **required** | Directory holding `segment.m3u8` and the `.ts` files it names |
| `WEB_DIR` | `web` | Directory served for pages and `/static/` |
| `COOKIE_SECURE` | `true` | `false` to send the session cookie over plain HTTP from a non-localhost host |
| `CHAT_HISTORY_SIZE` | `50` | Chat messages kept in memory |

Fixed values that are protocol or security invariants stay as named constants
in code, not environment variables: the 3-segment window, the HLS version and
the tick interval (`internal/stream`), session-id entropy and password bounds
(`internal/auth`), and the HTTP server timeouts and DB connect timeout
(`cmd/server`).

The compose file also reads `POSTGRES_USER`, `POSTGRES_PASSWORD`,
`POSTGRES_DB`, `DB_PORT` and `APP_PORT` from `.env` — see `.env.example`.

Open <http://localhost:8080> for the player, or query the API directly:

```bash
curl localhost:8080/playlist.m3u8
curl -I localhost:8080/segments/segment0.ts
```

## Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/static/…` | Stylesheets and scripts |
| GET | `/signup`, `/login` | Public pages |
| POST | `/signup`, `/login`, `/logout` | Auth actions |
| GET | `/playlist.m3u8` | Live media playlist — 3-segment rolling window *(auth)* |
| GET | `/segments/{name}.ts` | One MPEG-TS segment, streamed from disk *(auth)* |
| GET | `/` | Player page *(auth)* |

## Layout

```
cmd/server/main.go        entrypoint: load config, load pool, start ticker, wire routes
internal/config/
  config.go               Config struct, Load() reads and validates the environment
internal/stream/
  pool.go                 Segment and Pool types (parsed once, then immutable)
  parser.go               m3u8 parsing and validation
  live.go                 LiveState: sliding window, ticker, RWMutex
  config.go               supported HLS version
internal/api/
  stream.go               Stream type, playlist and segment handlers
  chat.go                 in-memory chat handlers
  router.go               route constants and route registration
internal/weberr/
  weberr.go               every HTTP error response and ?error= code, in one place
internal/auth/
  password.go             bcrypt hashing
  session.go              session id generation, cookie handling
  middleware.go           Auth type, RequirePage, RequireAPI
  handlers.go             signup, login, logout
internal/db/
  db.go                   Store type, connection, sentinel errors
  users.go                User type, CreateUser, UserByEmail
  sessions.go             CreateSession, SessionUser, DeleteSession
  migrate.go              embedded goose migrations, run at startup
  migrations/*.sql        numbered up/down schema migrations
web/index.html            HLS.js player
web/login.html            sign-in page
web/signup.html           registration page
web/static/               stylesheets and scripts, served publicly
```
