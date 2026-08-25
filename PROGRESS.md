# Progress — Zapping HLS task

Last updated: 2026-08-23

## Current state: backend playlist + segment serving works

Run it:

```bash
go run ./cmd/server          # from repo root (paths are CWD-relative)
curl localhost:8080/playlist.m3u8
```

Verified working:

- `GET /playlist.m3u8` returns a live playlist with a 3-segment window.
- The window slides every 10s: `MEDIA-SEQUENCE` goes 0 -> 1, segments go
  `[0,1,2]` -> `[1,2,3]`.
- `GET /segments/segment0.ts` returns 200, `Content-Type: video/mp2t`,
  streamed from disk (never fully in RAM).
- `Range: bytes=0-99` returns 206 Partial Content.
- Path traversal blocked: raw, URL-encoded and double-encoded all 404.
- `GET /segments/segment.m3u8` returns 404 — the file exists on disk but is
  not in the allowlist. Allowlist beats sanitizing.

## File layout

```
cmd/server/main.go        entrypoint: load pool, start ticker, wire routes
internal/stream/
  pool.go                 Segment + Pool types (static, parsed once)
  parser.go               LoadPool / parseHeader / parseSegments / checkVersionLine
  live.go                 LiveState: sliding window, ticker, mutex
  config.go               SupportedVersion constant
internal/api/
  handlers.go             Stream type + StreamHandler + SegmentHandler
  router.go               route constants + RegisterRoutes
```

## Key design decisions (and why)

**Pool is static, LiveState is mutable.** `Pool` holds only what is reusable
raw material for generating playlists: `Segments` and a computed
`TargetDuration`. Header tags from the source file are validated but not
stored — we generate our own playlist, we do not echo the source one.

**`currentIndex` != `mediaSequence`.** `currentIndex` wraps with `% total`
because we recycle 64 physical files. `mediaSequence` never wraps — HLS
requires it to grow monotonically or the player thinks the stream broke.
In a real live stream these would be the same number; they diverge here only
because we simulate infinity from finite input.

**RWMutex, one writer many readers.** `Advance()` (ticker, every 10s) takes
`Lock`. `Window()` and `MediaSequence()` take `RLock`. The lock is not about
protecting `pool.Segments` (immutable after parse) — it protects
`currentIndex` and `mediaSequence`, and guarantees the three reads inside
`Window()` all see the same index, so the window is never torn.

**Two-phase parser.** `parseHeader` consumes until the first `#EXTINF:` and
returns that line; `parseSegments` continues from it. RFC 8216 only
guarantees that header tags precede segments — not their relative order —
so reading a fixed number of header lines would break on valid files.

**Dependency injection over globals.** `Stream` holds `pool`, `liveState`,
`segmentsDir`, `validSegments`. Handlers are methods on it, so no closures
and no package-level mutable state. Own `http.NewServeMux()` instead of
`DefaultServeMux` — the default is global and any imported package can
register routes on it (e.g. `net/http/pprof`).

**Allowlist over sanitizing.** `validSegments` is a `map[string]struct{}`
built once in `NewStream` from the pool. Requests for anything not in it get
404, which is stronger than `filepath.Base` — that would still serve any
file inside the directory.

**RAM.** The 64 `.ts` files total ~480MB, so segment bytes are never
preloaded. Only parsed metadata lives in memory; `http.ServeFile` streams
from disk in 32KB chunks per request.

## Go concepts covered so far

Modules and `cmd/`+`internal/` layout; goroutines and `go func(){}()`;
`sync.RWMutex` (`Lock` vs `RLock`); data races need at least one write;
happens-before via goroutine creation; interfaces are implicit
(`io.Writer`, `http.ResponseWriter`); `fmt.Fprintf` and format verbs;
pointers vs values; multiple and named return values; `defer`; `bufio.Scanner`;
`strings.CutPrefix` and the comma-ok idiom; `map[string]struct{}` as a set;
`for i := range 3`; struct literals with named fields; method values
(`s.StreamHandler`); closures vs methods for handler dependencies;
shadowing (local variable vs imported package name); `time.Ticker` and channels.

## Next steps

1. **`SEGMENTS_DIR` as env var.** Currently hardcoded in `main.go` as
   `"hls test/hls test"`. This is real config — it changes between local and
   Docker — unlike the route paths, which are API contract and stay constants.
2. **Frontend**: 3 pages (signup, login, player) with HLS.js pointed at
   `/playlist.m3u8`.
3. **Auth**: HttpOnly cookies, middleware guarding the player route,
   `users` + `sessions` tables in Postgres.
4. **Dockerfile** — keep it simple.

Deferred on purpose: N livestreams via `map[string]*LiveState`; a parser
factory keyed by HLS version.
