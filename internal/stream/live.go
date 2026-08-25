package stream

import (
	"sync"
	"time"
)

type LiveState struct {
	mu            sync.RWMutex
	pool          *Pool
	currentIndex  int
	mediaSequence int
}

/*
First letter Uppercase means visible from other packages, otherwise is "private"
The same for struct attributes
This is the constructor, receives Pool, returns a pointer to LiveState
*/
func NewLiveState(pool *Pool) *LiveState {

	liveState := &LiveState{pool: pool}

	return liveState

}

/*
This function reads the shared mutable currentIndex, needs Read Lock
so the three reads see a consistent value
*/
func (ls *LiveState) Window() []Segment {
	/* when reading, RLock/RUnlock*/
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	total := len(ls.pool.Segments)
	window := make([]Segment, 3)
	for i := range 3 {
		//We restart the stream when reaching the last segment
		window[i] = ls.pool.Segments[(ls.currentIndex+i)%total]
	}

	return window
}

/*When writing, Lock/Unlock, protects against writter vs reader*/
func (ls *LiveState) Advance() {

	ls.mu.Lock()
	defer ls.mu.Unlock()

	total := len(ls.pool.Segments)

	/* I want to loop the stream forever, index must be within the total of segments */
	ls.currentIndex = (ls.currentIndex + 1) % total
	/* media sequence needs to only increase to follow the rules of m3u8 */
	ls.mediaSequence++
}

/* Every response has to write current mediaSequence onto httpHeaders*/
func (ls *LiveState) MediaSequence() int {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	return ls.mediaSequence
}

/*
We need to run the ticker on a separate go rutine in order to not block all the server
*/
func (ls *LiveState) StartTicker(interval time.Duration) {

	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			ls.Advance()
		}
	}()

}
