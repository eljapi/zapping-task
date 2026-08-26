package stream

import (
	"fmt"
	"strings"
	"testing"
)

func testPool(size int) *Pool {
	pool := &Pool{TargetDuration: 10}
	for i := range size {
		pool.Segments = append(pool.Segments, Segment{Name: fmt.Sprintf("s%d", i), Duration: 10})
	}

	return pool
}

func render(p Playlist) string {
	parts := []string{}
	for _, s := range p.Segments {
		if s.Discontinuity {
			parts = append(parts, "DISC")
		}
		parts = append(parts, s.Name)
	}

	return strings.Join(parts, " ")
}

func TestDiscontinuityMarksTheWrap(t *testing.T) {
	want := []string{
		"s0 s1 s2",
		"s1 s2 s3",
		"s2 s3 s4",
		"s3 s4 DISC s0",
		"s4 DISC s0 s1",
		"DISC s0 s1 s2",
		"s1 s2 s3",
	}

	ls := NewLiveState(testPool(5))
	for tick, expected := range want {
		if got := render(ls.Playlist()); got != expected {
			t.Errorf("tick %d: got %q, want %q", tick, got, expected)
		}
		ls.Advance()
	}
}

func TestFirstPlaylistHasNoDiscontinuity(t *testing.T) {
	for _, s := range NewLiveState(testPool(5)).Playlist().Segments {
		if s.Discontinuity {
			t.Fatal("the very first playlist must not carry a discontinuity")
		}
	}
}

func TestDiscontinuityNumberIsStableAcrossReloads(t *testing.T) {
	ls := NewLiveState(testPool(5))
	seen := map[int]int{}

	for range 40 {
		p := ls.Playlist()
		number := p.DiscontinuitySequence
		for i, s := range p.Segments {
			if s.Discontinuity {
				number++
			}

			absolute := p.MediaSequence + i
			if previous, ok := seen[absolute]; ok && previous != number {
				t.Fatalf("segment %d was numbered %d, now %d", absolute, previous, number)
			}
			seen[absolute] = number
		}
		ls.Advance()
	}
}

func TestMediaSequenceNeverWraps(t *testing.T) {
	ls := NewLiveState(testPool(5))
	previous := -1

	for range 20 {
		got := ls.Playlist().MediaSequence
		if got != previous+1 {
			t.Fatalf("media sequence jumped from %d to %d", previous, got)
		}
		previous = got
		ls.Advance()
	}
}
