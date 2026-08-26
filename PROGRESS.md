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
| Password hashing | **next** |
| Session cookie + auth middleware | **next** |
| Signup / login pages | not started |
| Dockerfile for the app itself | not started |

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

## Next steps

1. **`internal/auth`**
   - Hash passwords with `golang.org/x/crypto/bcrypt` (`GenerateFromPassword`,
     `CompareHashAndPassword`). Never SHA-256, never hand-rolled.
     `go mod tidy` dropped bcrypt from `go.mod` because nothing imports it yet;
     it comes back on first use.
   - `NewSessionID()` — 32 bytes from `crypto/rand`, hex or base64url encoded.
   - `Middleware` — `func(http.Handler) http.Handler`: read the cookie, call
     `SessionUser`, on failure redirect to `/login`, on success put the user in
     the request context and call the next handler.
2. **Auth handlers**: `POST /signup`, `POST /login`, `POST /logout`.
   Validate input, hash, create the session row, `http.SetCookie`.
   On logout: delete the row **and** expire the cookie.
   Login failure must not reveal whether the email exists — same message for
   unknown email and wrong password.
3. **Pages**: `web/signup.html`, `web/login.html`. Plain forms posting to the
   handlers above.
4. **Wire the middleware** around the player, the playlist and the segments.
5. **Dockerfile**. Multi-stage: build the binary, copy it into a small image.
   Open question: the `.ts` files are gitignored, so they must be mounted as a
   volume or copied in from outside the repo. Add the app to
   `docker-compose.yml` alongside the db.

Deferred on purpose: N livestreams via `map[string]*LiveState`; a parser
factory keyed by HLS version; a cleanup job deleting expired session rows.

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
with `errors.Is`; type-matching wrapped errors with `errors.As`.

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
