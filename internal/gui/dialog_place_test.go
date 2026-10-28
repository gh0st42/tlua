//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import (
	"testing"

	"github.com/pwiecz/go-fltk"
)

// A dialog opens in the middle of the form that asked for it, on that form's
// screen, not in the middle of the main screen: on a machine with several
// screens the form can be on another one, and the program then waits for a
// dialog nobody can see.
func TestDialogsOpenOverTheirForm(t *testing.T) {
	onScreen(t, `
first = gui.Form{caption = "First", width = 300, height = 150, left = 80, top = 90}
firstButton = first:Button{caption = "ask", left = 10, top = 10, width = 80, height = 28, onClick = function() end}
second = gui.Form{caption = "Second", width = 260, height = 140, left = 460, top = 260}
secondButton = second:Button{caption = "ask", left = 10, top = 10, width = 80, height = 28, onClick = function() end}
first:show()
second:show()`, func(s *scene) {
		centre := func(name string) (int, int) {
			win := window(s.obj(name))
			return win.XRoot() + win.W()/2, win.YRoot() + win.H()/2
		}
		// Where a window of this size goes, from a form's point of view.
		const w, h = 232, 128
		for _, name := range []string{"first", "second"} {
			x, y := centeredOn(w, h, s.obj(name))
			cx, cy := centre(name)
			sx, sy, sw, sh := fltk.ScreenWorkArea(fltk.ScreenNum(cx, cy))
			// In the middle, unless that would leave the screen.
			wantX := max(sx, min(cx-w/2, sx+sw-w))
			wantY := max(sy, min(cy-h/2, sy+sh-h))
			if x != wantX || y != wantY {
				t.Errorf("%s: a %dx%d window goes at %d,%d; want %d,%d (the form's middle is %d,%d)", name, w, h, x, y, wantX, wantY, cx, cy)
			}
			if x < sx || y < sy || x+w > sx+sw || y+h > sy+sh {
				t.Errorf("%s: the window %d,%d is off the screen %d,%d %dx%d", name, x, y, sx, sy, sw, sh)
			}
		}
		// Nothing to be over: the main screen.
		x, y := centeredOn(w, h, nil)
		wantX, wantY := centered(w, h)
		if x != wantX || y != wantY {
			t.Errorf("with no form: %d,%d, want %d,%d", x, y, wantX, wantY)
		}

		// The form that asks is the one whose control's handler is running.
		s.a.fire(s.obj("firstButton"), "onClick")
		if got := s.a.anchor(nil); got != s.obj("first") {
			t.Errorf("after a click in the first form the anchor is %v", got)
		}
		s.a.fire(s.obj("secondButton"), "onClick")
		if got := s.a.anchor(nil); got != s.obj("second") {
			t.Errorf("after a click in the second form the anchor is %v", got)
		}
		// A form being made does not take its own place as the anchor.
		if got := s.a.anchor(s.obj("second")); got == s.obj("second") {
			t.Errorf("the form skipped is its own anchor")
		}
		s.lua(`second:close()`)
		s.pump()
		if got := s.a.anchor(nil); got != s.obj("first") {
			t.Errorf("with the second closed the anchor is %v, not the first", got)
		}
	})
}
