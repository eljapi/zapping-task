package chat

import (
	"sync"
	"time"
)

type Message struct {
	Author string    `json:"author"`
	Text   string    `json:"text"`
	SentAt time.Time `json:"sentAt"`
}

type ChatState struct {
	mu       sync.RWMutex
	messages []Message
	maxSize  int
}

func NewChatState(maxSize int) *ChatState {
	return &ChatState{maxSize: maxSize}
}

func (cs *ChatState) Add(author, text string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.messages = append(cs.messages, Message{Author: author, Text: text, SentAt: time.Now()})
	if len(cs.messages) > cs.maxSize {
		cs.messages = cs.messages[len(cs.messages)-cs.maxSize:]
	}
}

func (cs *ChatState) Messages() []Message {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	out := make([]Message, len(cs.messages))
	copy(out, cs.messages)
	return out
}
