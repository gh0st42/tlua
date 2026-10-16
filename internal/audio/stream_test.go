package audio

import (
	"encoding/binary"
	"io"
	"math"
	"testing"

	"tlua/internal/pcm"
)

func sound(vals ...float32) *pcm.Sound {
	return &pcm.Sound{Left: vals, Right: vals, Rate: 4}
}

// frames reads n frames, and what their left sides were.
func frames(t *testing.T, s *stream, n int) ([]float32, error) {
	t.Helper()
	buf := make([]byte, n*8)
	got, err := s.Read(buf)
	out := []float32{}
	for i := 0; i+8 <= got; i += 8 {
		out = append(out, math.Float32frombits(binary.LittleEndian.Uint32(buf[i:])))
	}
	return out, err
}

func TestAStreamLoopsAndSwapsAtTheEnd(t *testing.T) {
	s := &stream{cur: sound(1, 2, 3), loop: true}
	got, _ := frames(t, s, 5)
	if want := []float32{1, 2, 3, 1, 2}; !same(got, want) {
		t.Fatalf("looping: %v, want %v", got, want)
	}
	// A new sound waits for the end of the one playing.
	s.next = sound(7, 8)
	got, _ = frames(t, s, 4)
	if want := []float32{3, 7, 8, 7}; !same(got, want) {
		t.Fatalf("swapped: %v, want %v", got, want)
	}
	if s.played != 9 {
		t.Errorf("played %d", s.played)
	}
}

func TestAStreamThatDoesNotLoopEnds(t *testing.T) {
	s := &stream{cur: sound(1, 2)}
	got, err := frames(t, s, 4)
	if !same(got, []float32{1, 2}) || err != nil {
		t.Fatalf("%v %v", got, err)
	}
	if _, err := frames(t, s, 4); err != io.EOF {
		t.Fatalf("after the end: %v", err)
	}
}

func same(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
