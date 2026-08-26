package auth

import (
	"context"
	"net/http"

	"zapping-task/internal/db"
)

type contextKey struct{}

var userKey contextKey

type Auth struct {
	store         *db.Store
	secureCookies bool
}

func New(store *db.Store, secureCookies bool) *Auth {
	return &Auth{store: store, secureCookies: secureCookies}
}

func UserFrom(ctx context.Context) (*db.User, bool) {
	user, ok := ctx.Value(userKey).(*db.User)
	return user, ok
}

func (a *Auth) RequirePage(next http.Handler) http.Handler {
	return a.require(next, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

func (a *Auth) RequireAPI(next http.Handler) http.Handler {
	return a.require(next, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

func (a *Auth) require(next http.Handler, reject http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookie)
		if err != nil {
			reject(w, r)
			return
		}

		user, err := a.store.SessionUser(r.Context(), cookie.Value)
		if err != nil {
			reject(w, r)
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, user)))
	})
}
