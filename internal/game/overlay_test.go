package game

import (
	"testing"

	"tlua/internal/pico"
)

// TestTheFrameRateStaysInTheSmallFont keeps the window's own writing out of the
// program's hands.
//
// A font belongs to the console it is set on, and the overlay has one of its
// own, so a game that prints its text in unscii cannot make the counter grow or
// push it out of its corner. This is the test of that arrangement, which is
// easy to lose by reaching for whatever font is to hand.
func TestTheFrameRateStaysInTheSmallFont(t *testing.T) {
	var o overlay
	o.render(60, 60, 480, 270, 2)

	if o.con.Font() != pico.Small {
		t.Errorf("the overlay is drawing in %q", o.con.Font().Name)
	}

	// Its size is measured in that font too, not in whatever a program chose.
	wide := o.con.Screen.W
	o.con.SetFont(pico.Unscii()) // as a misguided change might
	again := o.render(60, 60, 480, 270, 2)
	if again.W != wide {
		t.Errorf("the counter is %d wide after the font changed, was %d", again.W, wide)
	}
	if o.con.Font() != pico.Small {
		t.Error("rendering should have put the small font back")
	}
}
