# Progress — Zapping HLS task

Last updated: 2026-08-25

Working notes for picking the task back up from a cold start. Architecture and
design rationale live in [README.md](README.md); this file tracks state, the
decisions already taken, and what is left.

## How to run everything

```bash
docker compose up -d        # Postgres 17 on :5432, applies schema.sql on first boot
go run ./cmd/server         # from repo root, listens on :8080
```

```bash
docker compose down         # stop db (add -v to wipe the data volume)
docker compose exec db psql -U zapping -d zapping    # sql shell
```

Environment variables (both have working local defaults, so nothing is
required for local dev):

| Variable | Default |
|---|---|
| `SEGMENTS_DIR` | `hls test/hls test` |
| `DATABASE_URL` | `postgres://zapping:zapping@localhost:5432/zapping` |

The 64 `.ts` files are gitignored (480 MB). They must already be in
`hls test/hls test/` or the server will not start.

## State

| Part | State |
|---|---|
| HLS playlist + sliding window | done |
| Segment serving (streamed, range requests, traversal guard) | done |
| Player page | done |
| Postgres + schema + docker compose | done |
| DB layer (users, sessions) | done, verified against a real Postgres |
| Password hashing (bcrypt) | done |
| Session cookie + auth middleware | done |
| Signup / login pages | done |
| Dockerfile for the app itself | **next** |

## Verified working

- `GET /playlist.m3u8` returns a live playlist with a 3-segment window.
- The window slides every 10s: `MEDIA-SEQUENCE` 0 -> 1, segments `[0,1,2]` ->
  `[1,2,3]`.
- `GET /segments/segment0.ts` returns 200, `Content-Type: video/mp2t`,
  streamed from disk (never fully in RAM).
- `Range: bytes=0-99` returns 206 Partial Content.
- Path traversal blocked: raw, URL-encoded and double-encoded all 404.
- `GET /segments/segment.m3u8` returns 404 — the file exists on disk but is
  not in the allowlist. Allowlist beats sanitizing.
- Player at `/` plays continuously. After 8s: `readyState` 4, `currentTime`
  20.9, 1920x1080, ~50s buffered, 5 playlist reloads, no errors.
- Playback survives the loop point. Starting three segments from the end and
  watching in a browser, `currentTime` climbed 6.09 -> 71.09 without a stall,
  crossing the seam at 24.6s with `readyState` 4 and no errors. The playlist
  shows `#EXT-X-DISCONTINUITY` appearing immediately before `segment0.ts` as it
  wraps in.
- `go test -race` with four concurrent readers against the ticker reports no
  data races.
- Server timeouts do not truncate streaming. With a 150 KB/s client, a 5.3 MB
  segment (~35s, well past the 15s global `WriteTimeout`) arrives complete.
  Removing the per-request deadline extension truncates it to 4,541,440 bytes
  **while still returning 200** — silent corruption, which is why the
  extension matters.
- Assets under `/static/` load without a session (200, correct MIME types)
  while `/`, `/playlist.m3u8` and `/segments/` stay guarded.
- No file outside `web/` is reachable: `/go.mod`, `/schema.sql` and
  `/internal/db/db.go` all 404 even with a valid session, and traversal
  attempts from `/static/` resolve back inside the root.
- Auth, verified end to end in a real browser: signing up lands on the player,
  the video plays (`readyState` 4, 1920x1080, ~50s buffered), signing out
  invalidates the session, and returning to `/` redirects to `/login`.
- **`document.cookie` returns empty on the player page while the video keeps
  playing** — proof that `HttpOnly` works: the browser sends the cookie, but
  JavaScript cannot read it.
- Unauthenticated access: `/` redirects (303), `/playlist.m3u8` and
  `/segments/` return 401. All three are guarded, not just the page.
- Signup rejects duplicate emails, short passwords and malformed emails, each
  with its own message.
- Login is case-insensitive on email and gives the *same* error for an unknown
  email and a wrong password. Timings measured identical (0.04s both), because
  the unknown-email path still runs a dummy bcrypt compare.
- Cookie is issued as `HttpOnly; Secure; SameSite=Strict; Path=/`, and logout
  clears it with `Max-Age=0`.
- Deleting a user cascades to their sessions.
- DB layer round trip: user created, duplicate email rejected with
  `ErrEmailTaken`, case-insensitive lookup, missing user gives `ErrNotFound`,
  valid session resolves to its user, **expired session rejected**, deleted
  session rejected.

## The auth design we settled on (and why)

**Opaque session ID, not JWT.** A JWT exists to avoid a database round trip in
a distributed system. This is one binary with one database that the spec
already requires, so a JWT buys nothing and costs signing, verification,
expiry and rotation logic. Worse, a JWT is awkward to revoke — there is no row
to delete, so a leaked token stays valid until it expires unless a denylist is
maintained. An opaque random ID is revoked with `DELETE FROM sessions`.

**Cookie, never `localStorage`.** OWASP is explicit: do not put session IDs or
tokens in `localStorage`/`sessionStorage`, because any JavaScript on the origin
can read them, so a single XSS leaks everything. The cookie must carry:

| Attribute | Why |
|---|---|
| `HttpOnly` | JavaScript cannot read it — this is the XSS defence |
| `Secure` | HTTPS only (browsers make an exception for localhost) |
| `SameSite=Strict` | The CSRF defence. `HttpOnly` does **not** cover CSRF: it stops JS reading the cookie, not the browser sending it |
| `Path=/` | Sent to player, playlist and segments alike |

**Session ID entropy.** OWASP requires at least 64 bits. Use 32 bytes from
`crypto/rand` (256 bits) — it is free. `math/rand` is predictable and must
never be used here.

**Guard all three routes.** The spec says only registered users may reach the
player. Protecting `/` alone is worthless: anyone who knows the URL could
still fetch `/playlist.m3u8` and `/segments/` directly and take the whole
stream without an account. The middleware has to wrap the player page, the
playlist and the segments.

**No ORM.** `pgx` with hand-written SQL. The `$1`/`$2` placeholders are what
prevent SQL injection — the value travels separately from the query and
Postgres never parses it as SQL. That protection comes from prepared
statements, not from an ORM.

**Plain HTML, no React.** React would drag Node and a build step into the
Dockerfile for two forms, and it pushes toward the SPA-plus-token pattern that
OWASP warns about. Server-served pages plus cookies make the secure path the
default one.

## Discontinuity notes

The pool is a finite VOD cut into 64 segments, so looping it means splicing
`segment63` (which starts at 631.4s) straight onto `segment0` (which starts at
1.4s). The presentation timestamps jump **backwards by 630 seconds** at that
seam. A player handles a rising media sequence fine, but it cannot handle a
timeline that runs backwards: it waits for a time that already passed and
freezes, while the playlist keeps advancing. That is the "stuck picture,
climbing media sequence" symptom, and it appears only after a full cycle —
64 x 10s = 640 seconds in.

RFC 8216 is explicit that `EXT-X-DISCONTINUITY` **MUST** be present when the
"timestamp sequence" changes, which is exactly this case. The tag tells the
player to reset its timeline, and neither it nor `EXT-X-DISCONTINUITY-SEQUENCE`
is gated to a protocol version, so `EXT-X-VERSION:3` stays valid.

Two subtleties, both covered by tests in `internal/stream/live_test.go`:

- **The very first playlist carries no discontinuity.** Segment 0 on the first
  pass is a genuine start, not a seam.
- **`EXT-X-DISCONTINUITY-SEQUENCE` counts seams *before* the playlist**, not
  including one inside it. Per the RFC a segment's discontinuity number is the
  header value plus the tags preceding it, so counting a seam in both places
  would make the same segment change number between reloads and desynchronise
  the player. The count is therefore `(mediaSequence - 1) / total`, not
  `mediaSequence / total`.

`Playlist()` replaced the old `Window()` and `MediaSequence()` pair. Those were
two separate lock acquisitions, so a tick landing between them would emit a
playlist whose `MEDIA-SEQUENCE` did not match its own segments. One method now
returns the whole snapshot under a single read lock.

## Timeout notes

`http.ListenAndServe` applies **no timeouts at all**, which leaves the server
open to Slowloris: a client that opens a socket and dribbles a byte every few
seconds holds a goroutine indefinitely. The server is now built explicitly:

| Field | Value | Purpose |
|---|---|---|
| `ReadHeaderTimeout` | 5s | The actual Slowloris defence |
| `ReadTimeout` | 10s | Whole request body |
| `WriteTimeout` | 15s | Whole response |
| `IdleTimeout` | 60s | Keep-alive connections |

The read-side timeouts are free for streaming, because a video request sends
nothing. `WriteTimeout` is the dangerous one: it is an absolute deadline on
the entire response, so a 5 MB segment on a slow connection would be cut.
Rather than loosening it globally, `SegmentHandler` extends its own deadline
with `http.NewResponseController(w).SetWriteDeadline(...)` (Go 1.20+): strict
by default, explicit exception only where streaming happens.

The database connection in `main` is likewise bounded by
`context.WithTimeout(..., 5*time.Second)`. Without it, a host that silently
drops packets would hang startup forever with no error.

## DB layer notes

```
internal/db/db.go        Store type, Connect, ErrEmailTaken / ErrNotFound
internal/db/users.go     User type, CreateUser, UserByEmail
internal/db/sessions.go  CreateSession, SessionUser, DeleteSession
schema.sql               users + sessions tables
```

- `pgxpool.New` is lazy and does not actually dial, so `Connect` calls `Ping`.
  Without it the server would start "fine" and only fail on the first request.
- Session expiry is checked **in SQL** (`AND s.expires_at > now()`), not in Go.
  An expired session simply returns no row, so it is impossible to forget to
  validate it.
- Emails are lowercased and trimmed on both write and read. Otherwise
  `Juan@x.com` and `juan@x.com` would be two accounts and the `UNIQUE`
  constraint would not catch it.
- Postgres errors are translated into domain errors at this layer, so handlers
  never see `pgx.ErrNoRows` or the `23505` unique-violation code.

## Auth layer notes

```
internal/auth/password.go    bcrypt hashing, dummy hash for timing
internal/auth/session.go     session id generation, cookie set/clear
internal/auth/middleware.go  Auth type, RequirePage, RequireAPI, UserFrom
internal/auth/handlers.go    signup, login, logout
web/login.html               public
web/signup.html              public
web/static/theme.css         shared tokens and base, all three pages
web/static/auth.css          form styling
web/static/player.css        player styling
web/static/auth.js           maps ?error= codes to messages
web/static/player.js         HLS.js setup
```

- **Two middlewares, not one.** `RequirePage` redirects to `/login`, which is
  right for a browser. `RequireAPI` returns 401, which is right for the
  playlist and segments — a redirect there would hand HLS.js an HTML page
  where it expects a playlist.
- **The dummy bcrypt compare.** A missing email would otherwise return
  immediately while a wrong password takes ~40ms of hashing, and that
  difference tells an attacker which emails are registered. The unknown-email
  path burns the same time on purpose.
- **Login errors are deliberately vague.** Unknown email and wrong password
  produce the identical message.
- **Errors travel as short codes** in the query string (`?error=taken`), and
  the page maps the code to a message. No user text in the URL, no template
  engine needed.
- **Static assets need their own public route.** Stylesheets and scripts live
  in `web/static/` and are served from `GET /static/`, registered *before* the
  `/` catch-all and deliberately outside the middleware. Serving them from `/`
  would put them behind `RequirePage`, and the login page would be redirected
  away before it could load its own styles. They hold no secrets, so exposing
  them costs nothing.
- **Error codes are unique across pages** so a single `auth.js` serves both
  forms: `invalid` for login, and `fields` / `password` / `taken` for signup.
- `COOKIE_SECURE` defaults to `true`. Browsers accept `Secure` cookies on
  `localhost`, so this works in development; set it to `false` only when
  serving over plain HTTP from a non-localhost host.

## Next steps

1. **Dockerfile** — the last thing the spec asks for. Multi-stage: build the
   binary, copy it into a small image. The `.ts` files are gitignored, so they
   have to be mounted as a volume rather than copied in. Add the app service to
   `docker-compose.yml` next to the db, with `depends_on` waiting on the db
   healthcheck.
2. **Show who is logged in.** The middleware already puts the user in the
   request context and `auth.UserFrom(ctx)` reads it back, but nothing displays
   it yet. Serving the player through `html/template` would let the page greet
   the user by name.

Deferred on purpose: N livestreams via `map[string]*LiveState`; a parser
factory keyed by HLS version; a cleanup job deleting expired session rows
(harmless for now, since expiry is enforced in the query).

## Go concepts covered so far

Modules and `cmd/`+`internal/` layout; goroutines and `go func(){}()`;
`sync.RWMutex` (`Lock` vs `RLock`); data races need at least one write;
happens-before via goroutine creation; interfaces are implicit
(`io.Writer`, `http.ResponseWriter`); `fmt.Fprintf` and format verbs;
pointers vs values; multiple and named return values; `defer`; `bufio.Scanner`;
`strings.CutPrefix` and the comma-ok idiom; `map[string]struct{}` as a set;
`for i := range 3`; struct literals with named fields; method values
(`s.StreamHandler`); closures vs methods for handler dependencies;
shadowing; `time.Ticker` and channels; `context.Context` as the first
parameter of anything doing I/O; error wrapping with `%w`; sentinel errors
with `errors.Is`; type-matching wrapped errors with `errors.As`; middleware as
`func(http.Handler) http.Handler`; unexported struct types as context keys;
`http.HandlerFunc` as an adapter; closures returning handlers (`servePage`);
Go 1.22 method-aware mux patterns (`"POST /login"`).

## Notes on the spec

- The statement says the livestream is "generado en NodeJS" while its opening
  line asks for "un proyecto en Go lang" — a copy-paste artifact, not a real
  requirement.
- Its example playlist shows 4 segments with `TARGETDURATION:6`, but the prose
  asks for "30s de video por request (3 segmentos)". We follow the prose,
  which also matches the 10-second segments actually provided.
- "eliminar el último segmento (primero de la lista)" means the oldest one,
  which is first in the playlist. That is what `Advance()` does.
- The frontend is not restricted to vanilla JS: "Se puede utilizar Bootstrap y
  Jquery, javascript. Lo que quieras."
