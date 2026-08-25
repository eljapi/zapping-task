package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"zapping-task/internal/stream"
)

/*
With this we replace "closure", no need to return func(w,r){...} to both
pass shared attributes to a Handler and fulfill the signature of net/http.
Shared state of our App
*/
type Stream struct {
	pool          *stream.Pool
	liveState     *stream.LiveState
	segmentsDir   string
	validSegments map[string]struct{}
}

func NewStream(pool *stream.Pool, liveState *stream.LiveState, segmentsDir string) *Stream {
	validSegments := make(map[string]struct{}, len(pool.Segments))
	/*To check that a name exists, we add it to a map and we do not care about the value, only existence*/
	for _, segment := range pool.Segments {
		validSegments[segment.Name] = struct{}{}
	}

	return &Stream{
		pool:          pool,
		liveState:     liveState,
		segmentsDir:   segmentsDir,
		validSegments: validSegments,
	}
}

func (s *Stream) StreamHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")

	fmt.Fprintf(w, "#EXTM3U\n")
	fmt.Fprintf(w, "#EXT-X-VERSION:%d\n", stream.SupportedVersion)
	fmt.Fprintf(w, "#EXT-X-TARGETDURATION:%d\n", s.pool.TargetDuration)
	fmt.Fprintf(w, "#EXT-X-MEDIA-SEQUENCE:%d\n", s.liveState.MediaSequence())

	/*We loop the slice returned by method Window and we build the m3u8 EXTINF section*/
	for _, segment := range s.liveState.Window() {
		fmt.Fprintf(w, "#EXTINF:%f,\n", segment.Duration)
		fmt.Fprintf(w, "%s%s\n", SegmentsPrefix, segment.Name)
	}
}

func (s *Stream) SegmentHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, SegmentsPrefix)

	/*Security check for malicious path traversal like ../../etc/pwd*/
	if _, ok := s.validSegments[name]; !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "video/mp2t")

	/*
		Instead of using os.ReadFile(path), that will cause complete load of file on heap before send the file through tcp
		With http.ServeFile we send a fixed chunk 32KB (or 0 with page cache to socket)continiously until full file is sent.
		Every request will use 32KB of heap when sending the file. With ReadFile this cost the
		complete size of the file.

	*/
	http.ServeFile(w, r, filepath.Join(s.segmentsDir, name))
}
