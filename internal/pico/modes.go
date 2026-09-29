package pico

// Video modes, as vid() picks them.
//
// 0, 3 and 4 are Picotron's, with its numbers: it is a small enough table that
// having the same program mean the same thing in both is worth more than a
// tidier ordering. 1 and 2 are the two Picotron lists as planned rather than
// supported, and are here already, so that a program written for them runs.
//
// 13 is the odd one out on purpose: 320x200 is what an IBM VGA card called
// mode 13h, the resolution a great deal of DOS pixel art was drawn at, and it
// goes with the VGA palette.
var modes = []struct {
	n    int
	w, h int
}{
	{0, ScreenWidth, ScreenHeight}, // 480x270
	{1, 320, 180},                  // planned in Picotron
	{2, 240, 180},                  // planned in Picotron
	{3, 240, 135},
	{4, 160, 90},
	{13, 320, 200}, // mode 13h
}

// Mode reports the size of a video mode, and whether there is one.
func Mode(n int) (w, h int, ok bool) {
	for _, m := range modes {
		if m.n == n {
			return m.w, m.h, true
		}
	}
	return 0, 0, false
}

// ModeOf reports which mode a screen size is, or -1 for a size that was asked
// for some other way.
func ModeOf(w, h int) int {
	for _, m := range modes {
		if m.w == w && m.h == h {
			return m.n
		}
	}
	return -1
}

// Modes lists the video modes there are, in order, for an error message that
// has to say what was expected.
func Modes() []int {
	out := make([]int, 0, len(modes))
	for _, m := range modes {
		out = append(out, m.n)
	}
	return out
}
