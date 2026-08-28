package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"zapping-task/internal/db"
	"zapping-task/internal/weberr"
)

func (a *Auth) SignupHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	if name == "" || !strings.Contains(email, "@") {
		weberr.Redirect(w, r, "/signup", weberr.CodeFields)
		return
	}

	if len(password) < MinPasswordLength || len(password) > MaxPasswordLength {
		weberr.Redirect(w, r, "/signup", weberr.CodePassword)
		return
	}

	hash, err := HashPassword(password)
	if err != nil {
		weberr.Internal(w)
		return
	}

	user, err := a.store.CreateUser(r.Context(), name, email, hash)
	if err != nil {
		if errors.Is(err, db.ErrEmailTaken) {
			weberr.Redirect(w, r, "/signup", weberr.CodeTaken)
			return
		}
		weberr.Internal(w)
		return
	}

	a.startSession(w, r, user.ID)
}

func (a *Auth) LoginHandler(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	user, err := a.store.UserByEmail(r.Context(), email)
	if err != nil {
		burnTime(password)
		weberr.Redirect(w, r, "/login", weberr.CodeInvalid)
		return
	}

	if err := CheckPassword(user.PasswordHash, password); err != nil {
		weberr.Redirect(w, r, "/login", weberr.CodeInvalid)
		return
	}

	a.startSession(w, r, user.ID)
}

func (a *Auth) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(SessionCookie); err == nil {
		a.store.DeleteSession(r.Context(), cookie.Value)
	}

	a.clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *Auth) startSession(w http.ResponseWriter, r *http.Request, userID int64) {
	id, err := NewSessionID()
	if err != nil {
		weberr.Internal(w)
		return
	}

	expires := time.Now().Add(SessionLifetime)
	if err := a.store.CreateSession(r.Context(), id, userID, expires); err != nil {
		weberr.Internal(w)
		return
	}

	a.setSessionCookie(w, id, expires)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
