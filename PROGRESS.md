# Progress — Zapping HLS task

Last updated: 2026-08-25

Working notes for picking the task back up. Architecture and design rationale
live in [README.md](README.md); this file tracks state and what is left.

## Current state: backend + player work end to end

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
- The player at `/` plays continuously in the browser. After 8s: `readyState`
  4, `currentTime` 20.9, 1920x1080, ~50s buffered, 5 playlist reloads, no
  errors.

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
2. **Signup and login pages.** The spec asks for three pages; the player
   exists, these two do not.
3. **Auth**: HttpOnly cookies, middleware guarding the player route,
   `users` + `sessions` tables in a database. New Go ground here:
   `database/sql`, middleware as `func(http.Handler) http.Handler`, and
   request context.
4. **Dockerfile** — keep it simple. Open question: the `.ts` files are
   gitignored, so they must be mounted or copied in from outside the repo.

Deferred on purpose: N livestreams via `map[string]*LiveState`; a parser
factory keyed by HLS version.

## Notes on the spec

- The statement says the livestream is "generado en NodeJS" while its opening
  line asks for "un proyecto en Go lang" — a copy-paste artifact, not a real
  requirement.
- Its example playlist shows 4 segments with `TARGETDURATION:6`, but the prose
  asks for "30s de video por request (3 segmentos)". We follow the prose,
  which also matches the 10-second segments actually provided.
- "eliminar el último segmento (primero de la lista)" means the oldest one,
  which is first in the playlist. That is what `Advance()` does.
