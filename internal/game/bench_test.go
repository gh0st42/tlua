package game

import (
	"path/filepath"
	"testing"

	"tlua/internal/pico"
)

// benchExample runs whole frames of a real program: the Lua, the API calls
// behind it, and the drawing they ask for. This is the number that matters —
// everything else here is a part of it.
func benchExample(b *testing.B, name string) {
	s, err := load(Options{Script: filepath.Join(examplesDir, name)})
	if err != nil {
		b.Fatal(err)
	}
	defer s.close()
	if err := s.runMain(); err != nil {
		b.Fatal(err)
	}
	// Let it get going, so the benchmark is not measuring an empty world.
	for i := 0; i < 120; i++ {
		s.rt.Tick(playing(i))
	}

	pixels := make([]byte, s.rt.Screen().W*s.rt.Screen().H*4)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := s.rt.Tick(playing(i)); err != nil {
			b.Fatal(err)
		}
		// What the window does with the result, every frame, whatever the
		// program did.
		s.rt.Vid.Pixels(pixels)
	}
}

func BenchmarkExampleBounce(b *testing.B)     { benchExample(b, "bounce.lua") }
func BenchmarkExamplePlatformer(b *testing.B) { benchExample(b, "platformer.lua") }
func BenchmarkExampleStarfield(b *testing.B)  { benchExample(b, "starfield.lua") }
func BenchmarkExampleSnake(b *testing.B)      { benchExample(b, "snake.lua") }
func BenchmarkExamplePlasma(b *testing.B)     { benchExample(b, "plasma.lua") }
func BenchmarkExampleSprites(b *testing.B)    { benchExample(b, "sprites.lua") }

// BenchmarkInputFrame is what reading the keyboard, mouse and pads costs once a
// tick, before any of the program's own work.
func BenchmarkInputFrame(b *testing.B) {
	var rd reader
	v := view{scale: 2}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rd.poll()
		_ = rd.frame(v, nil)
	}
}

// BenchmarkButtonsHeld is the console working out which buttons a set of held
// keys amounts to, which happens for every key held, every tick.
func BenchmarkButtonsHeld(b *testing.B) {
	keys := []string{"arrowleft", "z", "shiftleft", "x", "controlleft"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = pico.ButtonsHeld(keys)
	}
}
