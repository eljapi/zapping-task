package weberr

import "net/http"

type Code string

const (
	CodeInvalid  Code = "invalid"
	CodeFields   Code = "fields"
	CodePassword Code = "password"
	CodeTaken    Code = "taken"
)

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
