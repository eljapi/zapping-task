package chat

import (
	"sync"
	"time"
)

/*
The tags after each field tell encoding/json the name to use on the wire.
Without them the JSON keys would come out capitalised, since only exported
fields are visible to the encoder
*/
type Message struct {
	Author string    `json:"author"`
	Text   string    `json:"text"`
	SentAt time.Time `json:"sentAt"`
}

/*
Same shape as LiveState: shared mutable state guarded by an RWMutex, because
many pollers read while an occasional POST writes
*/
type ChatState struct {
	mu       sync.RWMutex
	messages []Message
	maxSize  int
}

func NewChatState(maxSize int) *ChatState {
	return &ChatState{maxSize: maxSize}
}

/*
Chat lives in memory only, so it has to be bounded or it grows until the
process dies. We keep the newest maxSize messages and drop the rest
*/
func (cs *ChatState) Add(author, text string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.messages = append(cs.messages, Message{Author: author, Text: text, SentAt: time.Now()})
	if len(cs.messages) > cs.maxSize {
		cs.messages = cs.messages[len(cs.messages)-cs.maxSize:]
	}
}

/*
Returning cs.messages directly would hand the caller a slice that still points
at our backing array, and Add could rewrite it after the lock is released.
We copy under the read lock so the caller owns what it gets
*/
func (cs *ChatState) Messages() []Message {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	out := make([]Message, len(cs.messages))
	copy(out, cs.messages)
	return out
}
