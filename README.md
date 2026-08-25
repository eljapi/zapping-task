# Zapping — HLS Live Stream Simulator

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
| Signup / login pages | not started |
| User database + auth guard on the player | not started |
| Dockerfile | not started |

## Requirements

- Go 1.25 or newer.
- The 64 `.ts` segment files. **They are not in this repository** — 480 MB of
  media does not belong in git. Place them, together with `segment.m3u8`, in
  `hls test/hls test/` before running.

## Running

```bash
go run ./cmd/server
```

Paths are resolved relative to the working directory, so run it from the
repository root. The server listens on `:8080`.

Open <http://localhost:8080> for the player, or query the API directly:

```bash
curl localhost:8080/playlist.m3u8
curl -I localhost:8080/segments/segment0.ts
```

## Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/playlist.m3u8` | Live media playlist — 3-segment rolling window |
| GET | `/segments/{name}.ts` | One MPEG-TS segment, streamed from disk |
| GET | `/` | Static files from `web/` |

## Layout

```
cmd/server/main.go        entrypoint: load pool, start ticker, wire routes
internal/stream/
  pool.go                 Segment and Pool types (parsed once, then immutable)
  parser.go               m3u8 parsing and validation
  live.go                 LiveState: sliding window, ticker, RWMutex
  config.go               supported HLS version
internal/api/
  handlers.go             Stream type, playlist and segment handlers
  router.go               route constants and route registration
web/index.html            HLS.js player
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

**Memory.** Segment bytes are never preloaded — 480 MB of media stays on disk.
Only parsed metadata lives in RAM, and `http.ServeFile` streams each segment
in fixed-size chunks, so a request costs kilobytes of heap rather than the
size of the file.
