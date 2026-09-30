package game

import (
	"os"
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
					t.Fatalf("frame %d: %v", i, err)
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

// playerColor is what the platformer draws its player in, which is how a test
// can watch it without reaching inside the program.
const playerColor = 14

// playerTop reports the topmost row the player is drawn on, or -1.
func playerTop(s *pico.Surface) int {
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			if s.Get(x, y) == playerColor {
				return y
			}
		}
	}
	return -1
}

// TestTheJumpButtonIsNotMissedNearTheGround plays the platformer for a few
// seconds and presses jump while still falling.
//
// A jump that is only taken on the tick the button goes down, and only while
// already standing, throws that press away — which is what "the jump does not
// always work" is: the button was pressed a few hundredths of a second early.
// The example remembers a press for a few frames for that reason, and this is
// the test of it.
func TestTheJumpButtonIsNotMissedNearTheGround(t *testing.T) {
	s, err := load(Options{Script: filepath.Join(examplesDir, "platformer.lua")})
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	if err := s.runMain(); err != nil {
		t.Fatal(err)
	}

	var jump pico.Frame
	jump.Buttons[0][pico.BtnO] = true

	step := func(f pico.Frame) int {
		if err := s.rt.Tick(f); err != nil {
			t.Fatal(err)
		}
		return playerTop(s.rt.Screen())
	}

	// Let it fall to the ground and settle.
	ground := 0
	for i := 0; i < 120; i++ {
		ground = step(pico.Frame{})
	}
	if ground <= 0 {
		t.Fatalf("the player is not on screen (top row %d)", ground)
	}

	// One press, one jump.
	top := step(jump)
	for i := 0; i < 6; i++ {
		top = step(pico.Frame{})
	}
	if top >= ground {
		t.Fatalf("pressing jump while standing did nothing: %d, still at %d", top, ground)
	}

	// Come back down, and press again while still in the air, close enough to
	// the ground that a press taken only on its own tick would be lost.
	pressed := false
	for i := 0; i < 120 && !pressed; i++ {
		was := top
		top = step(pico.Frame{})
		falling := top > was
		if falling && top < ground && ground-top <= 12 {
			top = step(jump)
			pressed = true
		}
	}
	if !pressed {
		t.Fatal("never found a moment just above the ground to press in")
	}

	// Landing, and then off again. What matters is the height reached after
	// touching down: measuring from the press itself would only measure the
	// fall it was made during, which proves nothing.
	landed, highest := -1, ground
	for i := 0; i < 40; i++ {
		top = step(pico.Frame{})
		if landed < 0 {
			if top >= ground {
				landed = i
			}
			continue
		}
		highest = min(highest, top)
	}
	if landed < 0 {
		t.Fatal("the player never came back down")
	}
	if highest >= ground-2 {
		t.Errorf("the press made just before landing was lost: after landing it got no higher than %d, standing is %d",
			highest, ground)
	}
}

// TestTheProjectExamplesRun plays the examples that are a folder rather than a
// file — the ones with artwork and a level beside them, which is what a game
// looks like once it is more than one screen.
//
// It is the only test that reads a real map, its tileset and its PNG off the
// disk, together, the way a game does.
func TestTheProjectExamplesRun(t *testing.T) {
	entries, err := os.ReadDir(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	dirs := []string{}
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(examplesDir, e.Name()))
		}
	}
	if len(dirs) == 0 {
		t.Fatalf("no project examples in %s", examplesDir)
	}

	for _, dir := range dirs {
		t.Run(filepath.Base(filepath.Clean(dir)), func(t *testing.T) {
			s, err := load(Options{Script: dir})
			if err != nil {
				t.Fatalf("loading: %v", err)
			}
			defer s.close()
			if err := s.runMain(); err != nil {
				t.Fatalf("starting: %v", err)
			}

			for i := 0; i < 120; i++ {
				if err := s.rt.Tick(playing(i)); err != nil {
					t.Fatalf("frame %d: %v", i, err)
				}
			}
			if blank(s.rt.Screen()) {
				t.Error("two seconds in and the screen is still empty")
			}
		})
	}
}
