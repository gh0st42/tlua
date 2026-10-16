package pcm

import (
	"encoding/binary"
	"testing"
)

func TestWAVIsSixteenBitStereo(t *testing.T) {
	s := &Sound{Left: []float32{0, 1, -1, 2}, Right: []float32{0.5, -0.5, 0, -2}, Rate: 44100}
	w := s.WAV()
	if string(w[:4]) != "RIFF" || string(w[8:16]) != "WAVEfmt " || string(w[36:40]) != "data" {
		t.Fatalf("not a WAV header: %q", w[:44])
	}
	if ch, rate := binary.LittleEndian.Uint16(w[22:]), binary.LittleEndian.Uint32(w[24:]); ch != 2 || rate != 44100 {
		t.Fatalf("channels %d, rate %d", ch, rate)
	}
	if n := binary.LittleEndian.Uint32(w[40:]); n != 16 || len(w) != 44+16 {
		t.Fatalf("data %d bytes of %d", n, len(w))
	}
	sample := func(i int) int16 { return int16(binary.LittleEndian.Uint16(w[44+2*i:])) }
	// 1 and -1 at full scale, and what is beyond them held there.
	for i, want := range []int16{0, 16384, 32767, -16384, -32767, 0, 32767, -32768} {
		if got := sample(i); got != want {
			t.Errorf("sample %d: %d, want %d", i, got, want)
		}
	}
	if s.Seconds() != 4.0/44100 || s.Peak() != 2 {
		t.Errorf("seconds %v, peak %v", s.Seconds(), s.Peak())
	}
}
