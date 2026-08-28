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
| `TICK_INTERVAL` | `10s` | How often the live window advances one segment (Go duration syntax) |
| `CHAT_HISTORY_SIZE` | `50` | Chat messages kept in memory |

Fixed values that are protocol or security invariants stay as named constants
in code, not environment variables: the 3-segment window and HLS version
(`internal/stream`), session-id entropy and password bounds (`internal/auth`),
and the HTTP server timeouts and DB connect timeout (`cmd/server`).

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

## Design notes

**Static pool, mutable live state.** `Pool` holds the reusable raw material —
the segment list and a computed target duration. Header tags from the source
playlist are validated but not stored, because the service generates its own
playlist rather than echoing the input.

**`currentIndex` is not `mediaSequence`.** `currentIndex` wraps modulo the
segment count because the same 64 files are recycled forever. `mediaSequence`
never wraps: HLS requires it to grow monotonically, and a player that sees it
go backwards treats the stream as broken. In a real live stream the two
numbers would coincide; they diverge here only because infinity is being
simulated from finite input.

**One writer, many readers.** The ticker calls `Advance()` under a write lock
every 10 seconds; `Window()` and `MediaSequence()` take read locks. The lock
does not protect `pool.Segments`, which is immutable after parsing — it
protects `currentIndex` and `mediaSequence`, and guarantees that the three
reads inside `Window()` all observe the same index, so a window is never torn
across a slide.

**Two-phase parser.** `parseHeader` consumes lines until the first `#EXTINF:`
and hands that line to `parseSegments`, which continues from it. RFC 8216
guarantees only that header tags precede segments, not their relative order
and not that every optional tag is present, so consuming a fixed number of
header lines would break on otherwise valid playlists.

**Dependency injection over globals.** Handlers are methods on a `Stream`
value that owns everything they need, so there is no package-level mutable
state and no closures wrapping handlers. The server builds its own
`http.NewServeMux()` rather than using `DefaultServeMux`, which is global and
can be written to by any imported package.

**Allowlist over sanitising.** The set of servable segment names is built once
at startup from the parsed pool. Anything not in it gets a 404. This is
strictly stronger than normalising the path with `filepath.Base`, which would
still happily serve any other file that happens to sit in the segments
directory.

**Fail at boot, not at the first segment.** Reaching the playlist proves
nothing about the media beside it: a partial unzip, or a directory holding the
`.m3u8` alone, parses 64 valid segments with nothing behind them. Left alone,
the server would boot, answer the playlist with a 200 and 404 every segment, and
the player would spin forever with nothing to report. `LoadPool` stats every
file it just parsed and refuses to start, naming the first one missing and the
directory it looked in. `SEGMENTS_DIR` itself is required for the same reason —
it names half a gigabyte that cannot ship with the code, so any default would be
a path that merely happens to be wrong.

**Memory.** Segment bytes are never preloaded — 480 MB of media stays on disk.
Only parsed metadata lives in RAM, and `http.ServeFile` streams each segment
in fixed-size chunks, so a request costs kilobytes of heap rather than the
size of the file.

**Sessions over JWTs.** Authentication uses an opaque random session
identifier stored in an `HttpOnly; Secure; SameSite=Strict` cookie, with the
session row held in Postgres. A JWT exists to avoid a database round trip in a
distributed system; this is a single binary with a database the specification
already requires, so a JWT would add signing, verification and rotation
without buying anything — and it is awkward to revoke, since there is no row
to delete. Session identifiers come from `crypto/rand`, well above the 64 bits
of entropy OWASP requires.

**Parameterised queries, no ORM.** All SQL is hand-written with `$1`-style
placeholders. The value travels separately from the statement and is never
parsed as SQL, which is what actually prevents injection. Postgres errors are
translated into domain errors inside the data layer, so handlers never see a
driver type. Session expiry is enforced in SQL rather than in Go, so an
expired session simply returns no row and cannot be forgotten.

**Migrations, not an init script.** The schema is a set of numbered goose
migrations under `internal/db/migrations/`, embedded into the binary with
`//go:embed` and applied by `db.Migrate` on startup. goose records applied
versions in `goose_db_version`, so booting against an existing database only
runs what is new. This replaces the earlier `schema.sql` mounted into the
container's `docker-entrypoint-initdb.d`, which fired exactly once per data
volume and could never express a change to an existing table. Only the
`postgres` dialect is imported; goose's other drivers are test-only and never
reach the binary. A separate `database/sql` handle is opened just for the
migration and closed straight after — the app itself keeps using `pgxpool`.

**Every protected route, not just the page.** The specification asks that only
registered users reach the player. Guarding `/` alone would be theatre, since
anyone knowing the URLs could still pull `/playlist.m3u8` and `/segments/`
directly and take the whole stream without an account. All three sit behind the
middleware, which comes in two flavours: page requests are redirected to
`/login`, while the playlist and segment routes answer `401`, because
redirecting an HLS client to an HTML page would only confuse it.

**Timing-equalised login.** An unknown email would otherwise return
immediately while a wrong password spends ~45ms in bcrypt, and that gap reveals
which addresses are registered. The unknown-email path deliberately runs a
throwaway comparison so both take the same time, and both return the same
message. That comparison's hash is generated at startup rather than hardcoded,
because bcrypt stores the cost inside the hash string: a pasted literal would
quietly stop matching the cost real passwords use the day that cost is raised,
and the gap would come back wider than before. Its salt differs on every boot,
which changes nothing — bcrypt runs 2^cost iterations regardless.

**Cost rotation is not implemented.** Raising the bcrypt cost never invalidates
stored passwords, since each hash carries the cost it was made with and
verification reads it from there. It does leave two generations in the table,
and their different verification times leak how old an account is, which the
throwaway comparison cannot mask. Closing that means re-hashing on successful
login, the only moment the plaintext is in hand, whenever the stored cost is
below the current one. It is left out deliberately: the cost has never moved
here, and a rotation path with nothing to rotate is dead code.

**Signalling the loop point.** Recycling a finite pool means splicing the last
segment onto the first, where the media timestamps drop back by the length of
the whole asset. A player tolerates a rising media sequence but not a timeline
that runs backwards; without a marker it simply freezes while the playlist
keeps moving. `EXT-X-DISCONTINUITY` is emitted at that seam, which RFC 8216
requires whenever the timestamp sequence changes, and the accompanying
`EXT-X-DISCONTINUITY-SEQUENCE` counts only the seams that fall *before* the
current playlist, so a segment keeps the same discontinuity number across
reloads.

**One snapshot per playlist.** `Playlist()` returns the media sequence, the
discontinuity sequence and the segment window together under a single read
lock. Reading them through separate calls would let a tick land in between and
produce a playlist whose header disagreed with its own segment list.

**One place for HTTP errors.** `internal/weberr` holds every error response the
handlers can send: `Unauthorized`, `BadRequest`, `NotFound`, `Internal`, and
`Redirect` for the form pages. Handlers never call `http.Error` directly, so a
401 body reads the same whether it comes from the auth middleware or the chat
endpoint. The `?error=` codes the signup and login pages use — `invalid`,
`fields`, `password`, `taken` — are `weberr.Code` constants there too, the
single source of truth; `web/static/auth.js` only maps those same code strings
to the English sentence shown to the user. Domain sentinels
(`db.ErrNotFound`, `auth.ErrInvalidCredentials`) stay in the package that
raises them, which is the idiomatic Go split.
