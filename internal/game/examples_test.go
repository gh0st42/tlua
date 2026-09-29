package game

import (
	"path/filepath"
	"testing"

	"tlua/internal/pico"
)

// examplesDir is where the programs shipped with tlua live, from this package.
const examplesDir = "../../examples/pico"

// TestEveryExampleRuns loads each example and runs two seconds of it with
// something pressed, without ever opening a window.
//
// This is the only test that exercises the whole stack — interpreter, console
// API, drawing — against real programs, and it is the one that would catch an
// example that was left broken, or an API change that quietly stopped meaning
// what the examples say it means.
func TestEveryExampleRuns(t *testing.T) {
	scripts, err := filepath.Glob(filepath.Join(examplesDir, "*.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if len(scripts) < 10 {
		t.Fatalf("found %d examples in %s; they seem to have gone missing", len(scripts), examplesDir)
	}

	for _, script := range scripts {
		t.Run(filepath.Base(script), func(t *testing.T) {
			s, err := load(Options{Script: script})
			if err != nil {
				t.Fatalf("loading: %v", err)
			}
			defer s.close()

			if err := s.runMain(); err != nil {
				t.Fatalf("starting: %v", err)
			}

			const frames = 120
			for i := 0; i < frames; i++ {
				if err := s.rt.Tick(playing(i)); err != nil {
					t.Fatalf("frame %d, _update: %v", i, err)
				}
				if err := s.rt.Draw(); err != nil {
					t.Fatalf("frame %d, _draw: %v", i, err)
				}
				if quit, _ := s.rt.Quitting(); quit {
					t.Fatalf("frame %d: the example asked to exit", i)
				}
			}

			if blank(s.rt.Screen()) {
				t.Error("two seconds in and the screen is still empty")
			}
		})
	}
}

// playing is a frame of somebody playing: each button in turn, the mouse
// sweeping across the screen with a button down, and the wheel turning. It is
// not clever, but it reaches the parts of an example that input drives — a
// snake that turns, a player that jumps, a brush that paints.
func playing(i int) pico.Frame {
	var f pico.Frame
	f.Buttons[0][i/10%pico.Buttons] = true
	if i%20 < 10 {
		f.Buttons[1][pico.BtnLeft] = true
	}

	f.Keys = []string{"z", "c", "1"}
	f.MouseX = i * 3 % 480
	f.MouseY = i * 2 % 270
	if i%3 != 0 {
		f.MouseButtons = pico.MouseLeft
	} else if i%7 == 0 {
		f.MouseButtons = pico.MouseRight
	}
	f.WheelY = float64(i%5) - 2
	return f
}

// blank reports whether nothing at all has been drawn.
func blank(s *pico.Surface) bool {
	for _, col := range s.Pix {
		if col != 0 {
			return false
		}
	}
	return true
}
