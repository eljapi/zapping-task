package api

import (
	"net/http"
	"path/filepath"

	"zapping-task/internal/auth"
)

const (
	PlaylistPath   = "/playlist.m3u8"
	SegmentsPrefix = "/segments/"
)

/*
This registers our handlers onto the given ServeMux
*/

func RegisterRoutes(mux *http.ServeMux, s *Stream, c *Chat, a *auth.Auth, webDir string) {
	static := http.FileServer(http.Dir(filepath.Join(webDir, "static")))
	mux.Handle("GET /static/", http.StripPrefix("/static/", static))

	mux.HandleFunc("GET /login", servePage(webDir, "login.html"))
	mux.HandleFunc("GET /signup", servePage(webDir, "signup.html"))

	mux.HandleFunc("POST /login", a.LoginHandler)
	mux.HandleFunc("POST /signup", a.SignupHandler)
	mux.HandleFunc("POST /logout", a.LogoutHandler)

	mux.Handle(PlaylistPath, a.RequireAPI(http.HandlerFunc(s.StreamHandler)))
	mux.Handle(SegmentsPrefix, a.RequireAPI(http.HandlerFunc(s.SegmentHandler)))
	mux.Handle("GET /chat/messages", a.RequireAPI(http.HandlerFunc(c.MessagesHandler)))
	mux.Handle("POST /chat", a.RequireAPI(http.HandlerFunc(c.SendHandler)))
	mux.Handle("/", a.RequirePage(http.FileServer(http.Dir(webDir))))
}

func servePage(dir, name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(dir, name))
	}
}
