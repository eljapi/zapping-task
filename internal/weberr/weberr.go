package weberr

import "net/http"

/*
Every HTTP error the handlers can answer with lives here, so a 401 reads the
same whether it comes from the auth middleware or from the chat endpoint
*/

/*
Code is the ?error= value the signup and login pages read back from the URL.
A named string type instead of a plain string so the compiler rejects any
literal that is not one of the constants below. web/static/auth.js maps these
same four strings to the sentence shown to the user
*/
type Code string

const (
	CodeInvalid  Code = "invalid"
	CodeFields   Code = "fields"
	CodePassword Code = "password"
	CodeTaken    Code = "taken"
)

/*
Form pages answer with 303 and a redirect rather than an error body: the browser
re-issues the request as a GET, so a refresh does not repost the credentials
*/
func Redirect(w http.ResponseWriter, r *http.Request, path string, code Code) {
	http.Redirect(w, r, path+"?error="+string(code), http.StatusSeeOther)
}

func Unauthorized(w http.ResponseWriter) {
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

func BadRequest(w http.ResponseWriter, message string) {
	http.Error(w, message, http.StatusBadRequest)
}

func NotFound(w http.ResponseWriter) {
	http.Error(w, "not found", http.StatusNotFound)
}

func Internal(w http.ResponseWriter) {
	http.Error(w, "internal error", http.StatusInternalServerError)
}
