package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"
)

const (
	SessionCookie   = "session"
	SessionLifetime = 24 * time.Hour
	sessionIDBytes  = 32
)

func NewSessionID() (string, error) {
	buf := make([]byte, sessionIDBytes)
	/*Create random 32Bytes*/
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

/*
HttpOnly keeps the value out of document.cookie, so a XSS cannot read the session.
Secure refuses to send it over plain HTTP (configurable, because docker on a
non-localhost host has no TLS). SameSite=Strict means the cookie does not travel
on requests started by another site, which is what stops CSRF here
*/
func (a *Auth) setSessionCookie(w http.ResponseWriter, id string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    id,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   a.secureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}

/*
A cookie is deleted by overwriting it with MaxAge -1, and the attributes must
match the ones it was set with or the browser writes a second cookie instead
*/
func (a *Auth) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.secureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}
