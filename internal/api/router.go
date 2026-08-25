package api

import "net/http"

const (
	PlaylistPath   = "/playlist.m3u8"
	SegmentsPrefix = "/segments/"
)

/*
This registers our handlers onto the given ServeMux
*/

func RegisterRoutes(mux *http.ServeMux, s *Stream) {
	mux.HandleFunc(PlaylistPath, s.StreamHandler)
	mux.HandleFunc(SegmentsPrefix, s.SegmentHandler)
}
