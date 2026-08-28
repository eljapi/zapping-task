package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"zapping-task/internal/auth"
	"zapping-task/internal/chat"
	"zapping-task/internal/weberr"
)

type Chat struct {
	state *chat.ChatState
}

func NewChat(state *chat.ChatState) *Chat {
	return &Chat{state: state}
}

func (c *Chat) MessagesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c.state.Messages())
}

/*
The author is taken from the session the middleware put in the context, never
from the form, otherwise anybody could post under someone else's name
*/
func (c *Chat) SendHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFrom(r.Context())
	if !ok {
		weberr.Unauthorized(w)
		return
	}

	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		weberr.BadRequest(w, "empty message")
		return
	}

	c.state.Add(user.Name, text)
	w.WriteHeader(http.StatusNoContent)
}
