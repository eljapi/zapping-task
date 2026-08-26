package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"zapping-task/internal/db"
)

func (a *Auth) SignupHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	if name == "" || !strings.Contains(email, "@") {
		http.Redirect(w, r, "/signup?error=fields", http.StatusSeeOther)
		return
	}

	if len(password) < MinPasswordLength || len(password) > MaxPasswordLength {
		http.Redirect(w, r, "/signup?error=password", http.StatusSeeOther)
		return
	}

	hash, err := HashPassword(password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	user, err := a.store.CreateUser(r.Context(), name, email, hash)
	if err != nil {
		if errors.Is(err, db.ErrEmailTaken) {
			http.Redirect(w, r, "/signup?error=taken", http.StatusSeeOther)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
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
		http.Redirect(w, r, "/login?error=invalid", http.StatusSeeOther)
		return
	}

	if err := CheckPassword(user.PasswordHash, password); err != nil {
		http.Redirect(w, r, "/login?error=invalid", http.StatusSeeOther)
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
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	expires := time.Now().Add(SessionLifetime)
	if err := a.store.CreateSession(r.Context(), id, userID, expires); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	a.setSessionCookie(w, id, expires)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
