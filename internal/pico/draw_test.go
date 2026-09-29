package pico

import (
	"fmt"
	"testing"
)

func TestRectDrawsItsOutlineAndItsFill(t *testing.T) {
	c := New(8, 6)
	c.Rect(1, 1, 6, 4, 7)
	wantScreen(t, c, `
		........
		.777777.
		.7....7.
		.7....7.
		.777777.
		........`)

	c.Cls(0)
	c.RectFill(2, 2, 5, 3, 8)
	wantScreen(t, c, `
		........
		........
		..8888..
		..8888..
		........
		........`)
}

func TestRectAcceptsItsCornersInAnyOrder(t *testing.T) {
	c := New(5, 3)
	c.RectFill(3, 2, 1, 1, 4)
	wantScreen(t, c, `
		.....
		.444.
		.444.`)
}

func TestRectOfNoSizeIsOnePixel(t *testing.T) {
	c := New(3, 3)
	c.Rect(1, 1, 1, 1, 5)
	wantScreen(t, c, `
		...
		.5.
		...`)
}

func TestLineDrawsBothEnds(t *testing.T) {
	c := New(7, 5)
	c.Line(1, 1, 5, 3, 6)
	got := dump(c.Screen)
	if c.Screen.Get(1, 1) != 6 || c.Screen.Get(5, 3) != 6 {
		t.Errorf("the ends of the line are missing:\n%s", got)
	}
}

func TestLineDrawsTheStraightAndTheDiagonalCases(t *testing.T) {
	c := New(6, 6)
	c.Line(0, 0, 5, 5, 7) // down and right
	c.Line(5, 0, 0, 5, 8) // down and left
	wantScreen(t, c, `
		7....8
		.7..8.
		..78..
		..87..
		.8..7.
		8....7`)

	c.Cls(0)
	c.Line(0, 2, 5, 2, 3)
	c.Line(2, 0, 2, 5, 4)
	wantScreen(t, c, `
		..4...
		..4...
		334333
		..4...
		..4...
		..4...`)
}

func TestLineToAWildCoordinateDrawsWhatIsOnScreen(t *testing.T) {
	// A program working in world coordinates can easily ask for a line that
	// runs a million pixels off screen, and it must cost what the visible part
	// costs rather than what was asked for.
	c := New(8, 8)
	c.Line(0, 0, 1<<30, 1<<30, 7)
	for i := 0; i < 8; i++ {
		if c.Screen.Get(i, i) != 7 {
			t.Fatalf("the diagonal is missing at %d,%d:\n%s", i, i, dump(c.Screen))
		}
	}
}

func TestLineEntirelyOffScreenDrawsNothing(t *testing.T) {
	c := New(4, 4)
	c.Line(-10, -10, -2, -3, 7)
	c.Line(20, 0, 30, 3, 7)
	wantScreen(t, c, `
		....
		....
		....
		....`)
}

func TestCircleIsRoundAndSymmetric(t *testing.T) {
	c := New(11, 11)
	c.Circ(5, 5, 4, 7)
	// Three pixels across at the pole and nine across the middle: the widths
	// the midpoint algorithm gives, which is what a circle of this size looks
	// like on the consoles being imitated.
	wantScreen(t, c, `
		...........
		....777....
		..77...77..
		..7.....7..
		.7.......7.
		.7.......7.
		.7.......7.
		..7.....7..
		..77...77..
		....777....
		...........`)

	c.Cls(0)
	c.CircFill(5, 5, 3, 8)
	for _, p := range [][2]int{{5, 2}, {5, 8}, {2, 5}, {8, 5}} {
		if c.Screen.Get(p[0], p[1]) != 8 {
			t.Errorf("the filled circle is missing its edge at %d,%d:\n%s", p[0], p[1], dump(c.Screen))
		}
	}
	// Symmetry in both axes is the cheapest check that the rounding is even.
	for y := 0; y < 11; y++ {
		for x := 0; x < 11; x++ {
			if c.Screen.Get(x, y) != c.Screen.Get(10-x, y) || c.Screen.Get(x, y) != c.Screen.Get(x, 10-y) {
				t.Fatalf("the filled circle is lopsided at %d,%d:\n%s", x, y, dump(c.Screen))
			}
		}
	}
}

func TestCircleOfNoRadiusIsOnePixel(t *testing.T) {
	c := New(3, 3)
	c.Circ(1, 1, 0, 9)
	wantScreen(t, c, `
		...
		.9.
		...`)
}

func TestOvalOutlineIsExactlyTheEdgeOfTheFill(t *testing.T) {
	// The outline should be the pixels of the filled shape that touch the
	// outside, no more and no less: no gaps where the shape turns, and no
	// stray pixels inside it. Deriving what to expect from the fill keeps the
	// two in step for shapes far too fiddly to write out by hand.
	sizes := [][4]int{
		{2, 2, 20, 12}, {2, 2, 21, 13}, {1, 1, 3, 12},
		{1, 1, 22, 3}, {5, 5, 6, 6}, {5, 5, 5, 5}, {0, 0, 23, 23},
	}
	for _, s := range sizes {
		t.Run(fmt.Sprint(s), func(t *testing.T) {
			filled, outlined := New(24, 24), New(24, 24)
			filled.OvalFill(s[0], s[1], s[2], s[3], 7)
			outlined.Oval(s[0], s[1], s[2], s[3], 7)

			inside := func(x, y int) bool { return filled.Screen.Get(x, y) != 0 }
			for y := 0; y < 24; y++ {
				for x := 0; x < 24; x++ {
					edge := inside(x, y) &&
						(!inside(x-1, y) || !inside(x+1, y) || !inside(x, y-1) || !inside(x, y+1))
					if got := outlined.Screen.Get(x, y) != 0; got != edge {
						t.Fatalf("pixel %d,%d is %v, want %v\noutline:\n%s\nfill:\n%s",
							x, y, got, edge, dump(outlined.Screen), dump(filled.Screen))
					}
				}
			}
		})
	}
}

func TestTriangleFillsBetweenItsCorners(t *testing.T) {
	c := New(7, 5)
	c.TriFill(3, 0, 0, 4, 6, 4, 7)
	wantScreen(t, c, `
		...7...
		...7...
		..777..
		.77777.
		7777777`)
}

func TestTriangleTakesItsCornersInAnyOrder(t *testing.T) {
	want := ""
	for i, corners := range [][6]int{
		{3, 0, 0, 4, 6, 4},
		{0, 4, 6, 4, 3, 0},
		{6, 4, 3, 0, 0, 4},
	} {
		c := New(7, 5)
		c.TriFill(corners[0], corners[1], corners[2], corners[3], corners[4], corners[5], 7)
		got := dump(c.Screen)
		if i == 0 {
			want = got
			continue
		}
		if got != want {
			t.Errorf("corner order %v drew\n%s\nwant\n%s", corners, got, want)
		}
	}
}

func TestTriangleOutlineIsThreeLines(t *testing.T) {
	c := New(7, 5)
	c.Tri(0, 0, 6, 0, 3, 4, 7)
	if c.Screen.Get(3, 2) != 0 {
		t.Errorf("the outline should be hollow:\n%s", dump(c.Screen))
	}
	for _, p := range [][2]int{{0, 0}, {6, 0}, {3, 4}} {
		if c.Screen.Get(p[0], p[1]) != 7 {
			t.Errorf("corner %v is missing:\n%s", p, dump(c.Screen))
		}
	}
}

func TestClipConfinesEveryShape(t *testing.T) {
	c := New(8, 8)
	want := Rect{2, 2, 5, 5}
	c.Clip(want.X0, want.Y0, want.X1-want.X0, want.Y1-want.Y0, false)

	// Every kind of drawing there is, all of it aimed well outside the window.
	// cls() is left out on purpose: it is defined to lift the clip.
	c.RectFill(0, 0, 7, 7, 7)
	c.Rect(0, 0, 7, 7, 7)
	c.Circ(4, 4, 6, 8)
	c.CircFill(0, 0, 3, 8)
	c.OvalFill(0, 6, 7, 7, 8)
	c.Line(0, 7, 7, 0, 9)
	c.TriFill(0, 0, 7, 0, 0, 7, 9)
	c.Print("XX", 0, 0, 10)
	s := NewSurface(8, 8)
	s.Fill(11)
	c.Spr(s, 0, 0, false, false)

	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			inside := x >= want.X0 && x < want.X1 && y >= want.Y0 && y < want.Y1
			if drawn := c.Screen.Get(x, y) != 0; drawn != inside {
				t.Fatalf("pixel %d,%d drawn=%v, want %v (clip is %+v):\n%s",
					x, y, drawn, inside, want, dump(c.Screen))
			}
		}
	}
}

func TestClsIgnoresClippingAsTheConsolesDid(t *testing.T) {
	// cls() is the one call that reaches the whole screen whatever the clip
	// says, because it is how a program starts a frame from nothing.
	c := New(4, 2)
	c.Clip(1, 0, 1, 1, false)
	c.Cls(3)
	wantScreen(t, c, `
		3333
		3333`)
}

func TestClipCanBeNarrowedAndRestored(t *testing.T) {
	c := New(8, 4)
	old := c.Clip(0, 0, 4, 4, false)
	c.Clip(2, 0, 4, 4, true) // only the overlap of the two survives
	c.RectFill(0, 0, 7, 3, 7)
	wantScreen(t, c, `
		..77....
		..77....
		..77....
		..77....`)

	c.SetClip(old)
	c.Cls(0)
	c.RectFill(0, 0, 7, 0, 8)
	if got := dump(c.Screen)[:8]; got != "88888888" {
		t.Errorf("restoring the clip did not lift it: %q", got)
	}
}

func TestClipIsNotMovedByTheCamera(t *testing.T) {
	// The clipping rectangle belongs to the view, so scrolling the world moves
	// what is seen through the window rather than moving the window.
	c := New(6, 3)
	c.Clip(3, 0, 3, 3, false)

	c.RectFill(0, 0, 2, 2, 7) // left of the window, so nothing shows
	wantScreen(t, c, `
		......
		......
		......`)

	c.Camera(-3, 0)           // the world slides right...
	c.RectFill(0, 0, 2, 2, 7) // ...and the same rectangle is in the window
	wantScreen(t, c, `
		...777
		...777
		...777`)
}

func TestCameraShiftsWhatIsDrawn(t *testing.T) {
	c := New(5, 3)
	c.Camera(2, 1)
	c.RectFill(2, 1, 3, 2, 7)
	wantScreen(t, c, `
		77...
		77...
		.....`)

	oldX, oldY := c.Camera(0, 0)
	if oldX != 2 || oldY != 1 {
		t.Errorf("Camera reported %d,%d as the previous offset", oldX, oldY)
	}
}

func TestFillPatternDithers(t *testing.T) {
	c := New(4, 4)
	// 0xa5a5 is a checkerboard: 1010 0101 1010 0101.
	c.Fillp(0xa5a5, false)
	c.SetPenAlt(8)
	c.RectFill(0, 0, 3, 3, 7)
	wantScreen(t, c, `
		8787
		7878
		8787
		7878`)
}

func TestFillPatternCanLeaveHolesInstead(t *testing.T) {
	c := New(4, 2)
	c.Cls(3)
	c.Fillp(0xa5a5, true)
	c.RectFill(0, 0, 3, 1, 7)
	wantScreen(t, c, `
		3737
		7373`)
}

func TestFillPatternIsAnchoredToTheScreenNotTheShape(t *testing.T) {
	// A pattern that moved with the shape would crawl as a filled rectangle
	// slid about; anchoring it to the pixel grid is what makes it look like a
	// texture.
	c := New(4, 1)
	c.Fillp(0xa5a5, true)
	c.RectFill(1, 0, 3, 0, 7)
	wantScreen(t, c, `.7.7`)
}

func TestFillPatternDoesNotTouchSpritesOrText(t *testing.T) {
	c := New(4, 2)
	c.Fillp(0xffff, true) // would erase everything it applied to
	s := NewSurface(2, 1)
	s.Set(0, 0, 7)
	s.Set(1, 0, 7)
	c.Spr(s, 0, 0, false, false)
	c.Print(".", 0, 1, 7)
	if c.Screen.Get(0, 0) != 7 || c.Screen.Get(1, 0) != 7 {
		t.Errorf("the sprite was dithered away:\n%s", dump(c.Screen))
	}
}

func TestFillpReportsThePatternItReplaced(t *testing.T) {
	c := New(1, 1)
	c.Fillp(0x1234, false)
	if old := c.Fillp(0, false); old != 0x1234 {
		t.Errorf("Fillp reported %#04x as the previous pattern", old)
	}
}

func TestRoundedRectangleTakesAWidthAndAHeight(t *testing.T) {
	// Unlike the other rectangles, these are given a size rather than a second
	// corner, which is how Picotron spells them.
	c := New(8, 6)
	c.RRectFill(1, 1, 6, 4, 1, 7)
	wantScreen(t, c, `
		........
		..7777..
		.777777.
		.777777.
		..7777..
		........`)

	c.Cls(0)
	c.RRect(1, 1, 6, 4, 1, 8)
	wantScreen(t, c, `
		........
		..8888..
		.8....8.
		.8....8.
		..8888..
		........`)
}

func TestARadiusOfNothingIsAPlainRectangle(t *testing.T) {
	plain, rounded := New(8, 5), New(8, 5)
	plain.RectFill(1, 1, 6, 3, 7)
	rounded.RRectFill(1, 1, 6, 3, 0, 7)
	if got, want := dump(rounded.Screen), dump(plain.Screen); got != want {
		t.Errorf("rounded by nothing:\n%s\nplain:\n%s", got, want)
	}
}

func TestARadiusIsHeldToWhatFits(t *testing.T) {
	// Asked for more rounding than the shape can take, the corners meet in the
	// middle and it becomes a circle rather than turning inside out.
	c := New(11, 11)
	c.RRectFill(0, 0, 11, 11, 40, 7)

	round := New(11, 11)
	round.CircFill(5, 5, 5, 7)
	if got, want := dump(c.Screen), dump(round.Screen); got != want {
		t.Errorf("over-rounded:\n%s\nwant the circle:\n%s", got, want)
	}
}

func TestRoundedRectangleCornersMatchACircle(t *testing.T) {
	// A rounded corner is a quarter of the circle circ() would draw, so that
	// the two sit together without one of them looking flatter.
	const r = 6
	rect, circle := New(24, 24), New(24, 24)
	rect.RRectFill(2, 2, 20, 20, r, 7)
	circle.CircFill(2+r, 2+r, r, 7)

	for y := 2; y <= 2+r; y++ {
		for x := 2; x <= 2+r; x++ {
			if rect.Screen.Get(x, y) != circle.Screen.Get(x, y) {
				t.Fatalf("the corner differs from the circle at %d,%d:\n%s\n%s",
					x, y, dump(rect.Screen), dump(circle.Screen))
			}
		}
	}
}

func TestRoundedRectangleOutlineIsExactlyTheEdgeOfItsFill(t *testing.T) {
	sizes := [][5]int{
		{2, 2, 20, 12, 4}, {2, 2, 21, 13, 5}, {1, 1, 22, 22, 11},
		{3, 3, 8, 18, 3}, {2, 2, 5, 5, 2}, {2, 2, 4, 4, 1}, {2, 2, 18, 9, 0},
	}
	for _, s := range sizes {
		t.Run(fmt.Sprint(s), func(t *testing.T) {
			filled, outlined := New(24, 24), New(24, 24)
			filled.RRectFill(s[0], s[1], s[2], s[3], s[4], 7)
			outlined.RRect(s[0], s[1], s[2], s[3], s[4], 7)

			inside := func(x, y int) bool { return filled.Screen.Get(x, y) != 0 }
			for y := 0; y < 24; y++ {
				for x := 0; x < 24; x++ {
					edge := inside(x, y) &&
						(!inside(x-1, y) || !inside(x+1, y) || !inside(x, y-1) || !inside(x, y+1))
					if got := outlined.Screen.Get(x, y) != 0; got != edge {
						t.Fatalf("pixel %d,%d is %v, want %v\noutline:\n%s\nfill:\n%s",
							x, y, got, edge, dump(outlined.Screen), dump(filled.Screen))
					}
				}
			}
		})
	}
}

func TestARoundedRectangleOfNoSizeDrawsNothing(t *testing.T) {
	c := New(4, 4)
	c.RRectFill(1, 1, 0, 3, 1, 7)
	c.RRect(1, 1, 3, -2, 1, 7)
	wantScreen(t, c, `
		....
		....
		....
		....`)
}

func TestRoundedRectanglesFollowTheCameraAndTheClip(t *testing.T) {
	c := New(8, 6)
	c.Clip(0, 0, 4, 6, false)
	c.Camera(-2, 0)
	c.RRectFill(0, 0, 4, 4, 1, 7)
	// Drawn two to the right by the camera, and cut off by the clip at four:
	// the left half of a rounded square, corners and all.
	wantScreen(t, c, `
		...7....
		..77....
		..77....
		...7....
		........
		........`)
}
