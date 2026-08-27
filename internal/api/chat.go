package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"zapping-task/internal/auth"
	"zapping-task/internal/chat"
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

func (c *Chat) SendHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		http.Error(w, "empty message", http.StatusBadRequest)
		return
	}

	c.state.Add(user.Name, text)
	w.WriteHeader(http.StatusNoContent)
}
