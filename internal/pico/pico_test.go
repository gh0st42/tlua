package pico

import (
	"strings"
	"testing"
)

// dump renders a surface the way ParseSprite reads one, so a test can state the
// shape it expects as a picture instead of as a list of coordinates.
func dump(s *Surface) string {
	var b strings.Builder
	for y := 0; y < s.H; y++ {
		if y > 0 {
			b.WriteByte('\n')
		}
		for x := 0; x < s.W; x++ {
			col := s.Pix[y*s.W+x]
			switch {
			case col == 0:
				b.WriteByte('.')
			case int(col) < len(spriteDigits):
				b.WriteByte(spriteDigits[col])
			default:
				b.WriteByte('#')
			}
		}
	}
	return b.String()
}

// art normalises a picture written in a test so that it can be indented with
// the code around it.
func art(s string) string {
	lines := strings.Split(strings.Trim(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.Join(lines, "\n")
}

// wantScreen compares the screen with a picture, reporting both in full when
// they differ: with a shape, seeing it is the whole point.
func wantScreen(t *testing.T, c *Console, want string) {
	t.Helper()
	got := dump(c.Screen)
	if got != art(want) {
		t.Errorf("screen:\n%s\nwant:\n%s", got, art(want))
	}
}

func TestNewConsoleStartsBlank(t *testing.T) {
	c := New(4, 3)
	if c.Screen.W != 4 || c.Screen.H != 3 {
		t.Fatalf("screen is %dx%d", c.Screen.W, c.Screen.H)
	}
	if c.Target() != c.Screen {
		t.Error("drawing should start out landing on the screen")
	}
	if got := c.ClipRect(); got != (Rect{0, 0, 4, 3}) {
		t.Errorf("clip %+v, want the whole screen", got)
	}
	wantScreen(t, c, `
		....
		....
		....`)
}

func TestClsFillsAndResetsClipping(t *testing.T) {
	c := New(4, 2)
	c.Clip(1, 0, 1, 1, false)
	c.Cls(3)
	wantScreen(t, c, `
		3333
		3333`)
	if got := c.ClipRect(); got != (Rect{0, 0, 4, 2}) {
		t.Errorf("cls left the clip at %+v; it should lift it", got)
	}
}

func TestPsetAndPgetUseTheSameCoordinates(t *testing.T) {
	c := New(4, 4)
	c.Camera(1, 2)
	c.Pset(2, 3, 9)
	if got := c.Pget(2, 3); got != 9 {
		t.Errorf("pget reported %d, want the 9 that pset drew", got)
	}
	// The pixel itself landed a camera's worth up and to the left.
	if got := c.Screen.Get(1, 1); got != 9 {
		t.Errorf("screen pixel at 1,1 is %d, want 9", got)
	}
}

func TestPaletteRemapsAsThingsAreDrawn(t *testing.T) {
	c := New(3, 1)
	c.Pal(8, 12, false)
	c.Pset(0, 0, 8)
	c.Pset(1, 0, 9)
	if got := c.Screen.Get(0, 0); got != 12 {
		t.Errorf("colour 8 was drawn as %d, want the 12 it is mapped to", got)
	}
	if got := c.Screen.Get(1, 0); got != 9 {
		t.Errorf("colour 9 was drawn as %d; only 8 was remapped", got)
	}
}

func TestScreenPaletteRemapsOnlyOnTheWayOut(t *testing.T) {
	c := New(1, 1)
	c.Pset(0, 0, 8)
	c.Pal(8, 3, true)

	if got := c.Screen.Get(0, 0); got != 8 {
		t.Errorf("framebuffer holds %d; a screen palette must not touch it", got)
	}
	buf := make([]byte, 4)
	if n := c.Pixels(buf); n != 4 {
		t.Fatalf("Pixels wrote %d bytes, want 4", n)
	}
	r, g, b := RGB(3)
	if buf[0] != r || buf[1] != g || buf[2] != b || buf[3] != 0xff {
		t.Errorf("pixel is %v, want colour 3 (%d,%d,%d,255)", buf, r, g, b)
	}
}

func TestPixelsRefusesATooSmallBuffer(t *testing.T) {
	c := New(2, 2)
	if n := c.Pixels(make([]byte, 15)); n != 0 {
		t.Errorf("wrote into a buffer of 15 bytes for 16 bytes of pixels (n=%d)", n)
	}
}

func TestResetReturnsEveryDrawingSettingButNotThePixels(t *testing.T) {
	c := New(4, 4)
	c.Cls(5)
	c.Camera(3, 3)
	c.Clip(1, 1, 1, 1, false)
	c.Color(9)
	c.Fillp(0xa5a5, true)
	c.Pal(1, 2, false)
	c.Palt(7, true)
	c.Cursor(8, 8)
	target := NewSurface(2, 2)
	c.SetTarget(target)

	c.Reset()

	if x, y := c.CameraAt(); x != 0 || y != 0 {
		t.Errorf("camera at %d,%d", x, y)
	}
	if c.Target() != c.Screen {
		t.Error("target should be the screen again")
	}
	if got := c.ClipRect(); got != (Rect{0, 0, 4, 4}) {
		t.Errorf("clip %+v", got)
	}
	if c.Pen() != 6 {
		t.Errorf("pen %d, want 6", c.Pen())
	}
	if c.drawPal[1] != 1 || !c.transparent[0] || c.transparent[7] {
		t.Error("palette and transparency should be back to their defaults")
	}
	if x, y := c.CursorAt(); x != 0 || y != 0 {
		t.Errorf("cursor at %d,%d", x, y)
	}
	if got := dump(c.Screen)[:4]; got != "5555" {
		t.Errorf("reset cleared the screen (%q); it should leave the pixels alone", got)
	}
}

func TestSetTargetRedirectsDrawingAndResetsClipping(t *testing.T) {
	c := New(8, 8)
	s := NewSurface(3, 2)
	c.Clip(4, 4, 2, 2, false)
	c.SetTarget(s)
	if got := c.ClipRect(); got != (Rect{0, 0, 3, 2}) {
		t.Errorf("clip %+v, want the whole new target", got)
	}
	c.RectFill(0, 0, 9, 9, 7)
	if got := dump(s); got != "777\n777" {
		t.Errorf("target:\n%s", got)
	}
	if dump(c.Screen)[0] != '.' {
		t.Error("the screen should be untouched while drawing elsewhere")
	}

	c.SetTarget(nil)
	if c.Target() != c.Screen {
		t.Error("nil should mean the screen")
	}
}

func TestNearestPicksTheColourItself(t *testing.T) {
	for i := 0; i < Colors; i++ {
		r, g, b := RGB(uint8(i))
		if got := Nearest(r, g, b); got != uint8(i) {
			t.Errorf("colour %d (%02x%02x%02x) came back as %d", i, r, g, b, got)
		}
	}
}

func TestPaletteHasNoDuplicates(t *testing.T) {
	seen := map[uint32]int{}
	for i, c := range palette {
		if first, ok := seen[c]; ok {
			t.Errorf("colours %d and %d are both %06x", first, i, c)
		}
		seen[c] = i
	}
}

func TestColourIndexWrapsRatherThanEscaping(t *testing.T) {
	c := New(1, 1)
	c.Pset(0, 0, Colors+3)
	if got := c.Screen.Get(0, 0); got != 3 {
		t.Errorf("colour %d wrapped to %d, want 3", Colors+3, got)
	}
}
