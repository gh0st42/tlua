package pico

import (
	"os"
	"path/filepath"
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
	r, g, b := Default.RGB(3)
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
	for _, p := range []*Palette{Default, VGA} {
		for i := 0; i < p.Size(); i++ {
			r, g, b := p.RGB(uint8(i))
			if got := p.Nearest(r, g, b); p.Hex(got) != p.Hex(uint8(i)) {
				t.Errorf("%s: colour %d (%02x%02x%02x) came back as %d", p.Name, i, r, g, b, got)
			}
		}
	}
}

func TestTheBuiltInPalettesAreWhatTheySay(t *testing.T) {
	if Default.Size() != 64 {
		t.Errorf("the default palette has %d colours, want 64", Default.Size())
	}
	if VGA.Size() != 256 {
		t.Errorf("the VGA palette has %d colours, want 256", VGA.Size())
	}

	// The parts of the VGA palette that are not generated are the ones that
	// have to be right.
	for i, want := range map[uint8]uint32{
		0: 0x000000, 1: 0x0000aa, 7: 0xaaaaaa, 15: 0xffffff,
		16: 0x000000, 31: 0xffffff,
		255: 0x000000, // the last eight are black, as they were on the card
	} {
		if got := VGA.Hex(i); got != want {
			t.Errorf("VGA colour %d is %06x, want %06x", i, got, want)
		}
	}
	// Index 32 starts the colour wheel, at full brightness and saturation.
	if got := VGA.Hex(32); got != 0x0000ff {
		t.Errorf("VGA colour 32 is %06x, want pure blue", got)
	}

	// The default palette is meant to be 64 different colours. The VGA one is
	// not: white is in it twice and black many times over, because that is how
	// the card's table was laid out.
	seen := map[uint32]int{}
	for i := 0; i < Default.Size(); i++ {
		c := Default.Hex(uint8(i))
		if first, ok := seen[c]; ok {
			t.Errorf("default colours %d and %d are both %06x", first, i, c)
		}
		seen[c] = i
	}

	distinct := map[uint32]bool{}
	for i := 0; i < VGA.Size(); i++ {
		distinct[VGA.Hex(uint8(i))] = true
	}
	if len(distinct) < 200 {
		t.Errorf("the VGA palette has only %d different colours in it", len(distinct))
	}
}

func TestBuiltinPalettesAreFoundByName(t *testing.T) {
	for name, want := range map[string]*Palette{
		"default": Default, "DEFAULT": Default, " picotron ": Default, "": Default,
		"vga": VGA, "mode13h": VGA,
	} {
		if got, ok := Builtin(name); !ok || got != want {
			t.Errorf("Builtin(%q) = %v, %v", name, got, ok)
		}
	}
	if _, ok := Builtin("nothing like it"); ok {
		t.Error("an unknown name should not find a palette")
	}
}

func TestChangingThePaletteChangesWhatIsAlreadyDrawn(t *testing.T) {
	c := New(1, 1)
	c.Pset(0, 0, 8)

	custom := NewPalette("two", []uint32{0x000000, 0x111111})
	custom.Set(8, 0x123456)
	old := c.SetPalette(custom)
	if old.Name != Default.Name {
		t.Errorf("SetPalette reported %q as the palette it replaced", old.Name)
	}

	if got := c.Screen.Get(0, 0); got != 8 {
		t.Errorf("the pixel is %d; a palette must not touch the framebuffer", got)
	}
	buf := make([]byte, 4)
	c.Pixels(buf)
	if buf[0] != 0x12 || buf[1] != 0x34 || buf[2] != 0x56 {
		t.Errorf("pixel is %v, want the new palette's 123456", buf[:3])
	}

	// Setting an entry past the end stretches the palette to reach it.
	if custom.Size() != 9 {
		t.Errorf("the palette holds %d colours, want 9", custom.Size())
	}

	// The console took a copy, so changing the one it was handed changes
	// nothing on screen.
	custom.Set(8, 0xffffff)
	c.Pixels(buf)
	if buf[0] != 0x12 {
		t.Error("the console shares its palette with the caller")
	}
}

func TestABuiltInPaletteCannotBeChangedByAccident(t *testing.T) {
	c := New(1, 1)
	c.Palette().Set(0, 0xabcdef)
	if Default.Hex(0) != 0x000000 {
		t.Error("changing a console's palette changed the built-in one")
	}
}

func TestANewPaletteStartsWithNoRemapsLeftOver(t *testing.T) {
	c := New(1, 1)
	c.Pal(1, 2, false)
	c.Pal(3, 4, true)
	c.Palt(7, true)

	c.SetPalette(VGA)
	if c.drawPal[1] != 1 || c.screenPal[3] != 3 || c.transparent[7] {
		t.Error("changing the palette should leave the remaps behind")
	}
	if !c.transparent[0] {
		t.Error("colour 0 should still be the transparent one")
	}
}

func TestClonedPalettesAreTheirOwn(t *testing.T) {
	mine := Default.Clone()
	mine.Set(0, 0xffffff)
	if Default.Hex(0) != 0x000000 {
		t.Error("changing a clone changed the built-in palette")
	}
	if mine.Size() != Default.Size() {
		t.Error("a clone should be the same size")
	}
}

func TestTheVideoModes(t *testing.T) {
	// Picotron's own numbering, so that the same call means the same thing in
	// both, plus the one that is ours.
	cases := []struct {
		mode int
		w, h int
	}{
		{0, 480, 270}, // the console's own size
		{1, 320, 180}, // listed as planned in Picotron
		{2, 240, 180}, // listed as planned in Picotron
		{3, 240, 135},
		{4, 160, 90},
		{13, 320, 200}, // what a VGA card called mode 13h
	}
	for _, c := range cases {
		w, h, ok := Mode(c.mode)
		if !ok || w != c.w || h != c.h {
			t.Errorf("Mode(%d) = %dx%d, %v; want %dx%d", c.mode, w, h, ok, c.w, c.h)
		}
		if got := ModeOf(c.w, c.h); got != c.mode {
			t.Errorf("ModeOf(%d, %d) = %d, want %d", c.w, c.h, got, c.mode)
		}
	}

	if _, _, ok := Mode(7); ok {
		t.Error("there is no mode 7")
	}
	if _, _, ok := Mode(5); ok {
		t.Error("there is no mode 5")
	}
	if got := ModeOf(321, 200); got != -1 {
		t.Errorf("a size that is not a mode reported %d, want -1", got)
	}
	if got := len(Modes()); got != len(cases) {
		t.Errorf("Modes lists %d of them, want %d", got, len(cases))
	}
}

func TestParsingAGimpPalette(t *testing.T) {
	const file = `GIMP Palette
Name: Dusk
Columns: 4
#
  0   0   0	Black
255   0   0	Red
  0 255   0 Green
 17  34  51
not a colour at all
`
	p, err := ParseGPL(strings.NewReader(file), "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Dusk" {
		t.Errorf("the palette is called %q, want the name in the file", p.Name)
	}
	if p.Size() != 4 {
		t.Fatalf("it has %d colours, want 4", p.Size())
	}
	for i, want := range []uint32{0x000000, 0xff0000, 0x00ff00, 0x112233} {
		if got := p.Hex(uint8(i)); got != want {
			t.Errorf("colour %d is %06x, want %06x", i, got, want)
		}
	}
}

func TestAPaletteFileHasToBeOne(t *testing.T) {
	for _, file := range []string{
		"",
		"#!/bin/sh\necho hello\n",
		"GIMP Palette\nName: Empty\n",
	} {
		if _, err := ParseGPL(strings.NewReader(file), "x"); err == nil {
			t.Errorf("%q should not have been read as a palette", file)
		}
	}
}

func TestLoadingAPaletteFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "two colours.gpl")
	if err := os.WriteFile(path, []byte("GIMP Palette\n1 2 3\n4 5 6\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadGPL(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Size() != 2 || p.Hex(1) != 0x040506 {
		t.Errorf("palette is %d colours, second %06x", p.Size(), p.Hex(1))
	}
	// A file that does not say what it is called is named after itself.
	if p.Name != "two colours" {
		t.Errorf("the palette is called %q", p.Name)
	}

	if _, err := LoadGPL(filepath.Join(dir, "nothing.gpl")); err == nil {
		t.Error("a missing file should be an error")
	}
}
