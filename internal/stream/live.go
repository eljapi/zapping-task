package stream

import (
	"sync"
	"time"
)

const WindowSize = 3

type LiveState struct {
	mu            sync.RWMutex
	pool          *Pool
	currentIndex  int
	mediaSequence int
}

type WindowSegment struct {
	Segment
	Discontinuity bool
}

type Playlist struct {
	MediaSequence         int
	DiscontinuitySequence int
	Segments              []WindowSegment
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
func (ls *LiveState) Playlist() Playlist {
	/* when reading, RLock/RUnlock*/
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	total := len(ls.pool.Segments)
	segments := make([]WindowSegment, WindowSize)
	for i := range WindowSize {
		//We restart the stream when reaching the last segment
		index := (ls.currentIndex + i) % total
		segments[i] = WindowSegment{
			Segment:       ls.pool.Segments[index],
			Discontinuity: index == 0 && ls.mediaSequence+i > 0,
		}
	}

	discontinuitySequence := 0
	if ls.mediaSequence > 0 {
		discontinuitySequence = (ls.mediaSequence - 1) / total
	}

	return Playlist{
		MediaSequence:         ls.mediaSequence,
		DiscontinuitySequence: discontinuitySequence,
		Segments:              segments,
	}
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
