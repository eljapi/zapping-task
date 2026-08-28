package auth

import (
	"context"
	"net/http"

	"zapping-task/internal/db"
	"zapping-task/internal/weberr"
)

/*
An unexported empty struct as the context key. Using a plain string would let
another package store a value under the same key by accident; this type is only
nameable from here, so a collision is impossible
*/
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

/*
Two flavours of the same check. A browser hitting a page should land on /login,
but redirecting an HLS client to an HTML page would only confuse it, so the
playlist and the segments answer 401 instead
*/
func (a *Auth) RequirePage(next http.Handler) http.Handler {
	return a.require(next, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

func (a *Auth) RequireAPI(next http.Handler) http.Handler {
	return a.require(next, func(w http.ResponseWriter, r *http.Request) {
		weberr.Unauthorized(w)
	})
}

/*
Middleware in Go is a handler wrapping another handler. We take next, return a
new http.Handler, and only call next.ServeHTTP once the session resolves to a
user. That user is attached to the request context so the handlers downstream
read it without querying again
*/
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
