package pico

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestParseSpriteReadsTheArtAsItLooks(t *testing.T) {
	s, err := ParseSprite(`
		.888.
		87778
		.888.`)
	if err != nil {
		t.Fatal(err)
	}
	if s.W != 5 || s.H != 3 {
		t.Fatalf("sprite is %dx%d, want 5x3", s.W, s.H)
	}
	if got := dump(s); got != ".888.\n87778\n.888." {
		t.Errorf("sprite:\n%s", got)
	}
}

func TestParseSpriteHandlesIndentationSpacesAndShortRows(t *testing.T) {
	// The art is indented like the code around it, uses spaces for holes, and
	// has a row that stops early.
	s, err := ParseSprite("\n    1 1\n    111\n    1\n  ")
	if err != nil {
		t.Fatal(err)
	}
	if got := dump(s); got != "1.1\n111\n1.." {
		t.Errorf("sprite:\n%s", got)
	}
}

func TestParseSpriteTakesBarsAsRowBreaks(t *testing.T) {
	s, err := ParseSprite("ab|cd")
	if err != nil {
		t.Fatal(err)
	}
	if got := dump(s); got != "ab\ncd" {
		t.Errorf("sprite:\n%s", got)
	}
}

func TestParseSpriteRejectsWhatIsNotAColour(t *testing.T) {
	if _, err := ParseSprite("12z"); err == nil {
		t.Error("'z' is not a colour and should be refused")
	} else if !strings.Contains(err.Error(), "colour") {
		t.Errorf("error is %q; it should say what is wrong", err)
	}
	if _, err := ParseSprite("\n\n"); err == nil {
		t.Error("empty art should be refused")
	}
}

func TestSurfaceReadsAndWritesWithinItsBounds(t *testing.T) {
	s := NewSurface(2, 2)
	s.Set(1, 1, 5)
	s.Set(-1, 0, 9) // off the surface, and must be ignored
	s.Set(0, 7, 9)
	if got := s.Get(1, 1); got != 5 {
		t.Errorf("pixel is %d, want 5", got)
	}
	if got := s.Get(9, 9); got != 0 {
		t.Errorf("reading off the surface gave %d, want 0", got)
	}
	if got := dump(s); got != "..\n.5" {
		t.Errorf("surface:\n%s", got)
	}
}

func TestSurfaceResizeKeepsWhatFits(t *testing.T) {
	s := NewSurface(3, 2)
	s.Fill(4)
	s.Resize(2, 3)
	if got := dump(s); got != "44\n44\n.." {
		t.Errorf("surface:\n%s", got)
	}
}

func TestSprDrawsAtItsOwnSizeAndSkipsColourZero(t *testing.T) {
	c := New(6, 4)
	c.Cls(1)
	s, err := ParseSprite("7.7|.7.|7.7")
	if err != nil {
		t.Fatal(err)
	}
	c.Spr(s, 1, 1, false, false)
	wantScreen(t, c, `
		111111
		171711
		117111
		171711`)
}

func TestSprCanBeFlipped(t *testing.T) {
	s, err := ParseSprite("12|34")
	if err != nil {
		t.Fatal(err)
	}
	c := New(2, 2)
	c.Spr(s, 0, 0, true, false)
	if got := dump(c.Screen); got != "21\n43" {
		t.Errorf("flipped across: %q", got)
	}
	c.Cls(0)
	c.Spr(s, 0, 0, false, true)
	if got := dump(c.Screen); got != "34\n12" {
		t.Errorf("flipped down: %q", got)
	}
	c.Cls(0)
	c.Spr(s, 0, 0, true, true)
	if got := dump(c.Screen); got != "43\n21" {
		t.Errorf("flipped both ways: %q", got)
	}
}

func TestPaltChoosesWhatIsTransparent(t *testing.T) {
	s, err := ParseSprite("07")
	if err != nil {
		t.Fatal(err)
	}
	c := New(2, 1)
	c.Cls(1)
	c.Spr(s, 0, 0, false, false)
	if got := c.Screen.Get(0, 0); got != 1 {
		t.Errorf("pixel is %d; colour 0 should be transparent to begin with", got)
	}

	c.Cls(1)
	c.Palt(0, false) // colour 0 now draws
	c.Palt(7, true)  // and 7 does not
	c.Spr(s, 0, 0, false, false)
	if got := c.Screen.Get(0, 0); got != 0 {
		t.Errorf("pixel is %d, want the sprite's 0 now that it is opaque", got)
	}
	if got := c.Screen.Get(1, 0); got != 1 {
		t.Errorf("pixel is %d, want the background 1 now that 7 is transparent", got)
	}

	c.Cls(1)
	c.PaltNone()
	c.Spr(s, 0, 0, false, false)
	if got := c.Screen.Get(0, 0); got != 0 || c.Screen.Get(1, 0) != 7 {
		t.Errorf("with nothing transparent the whole sprite should draw:\n%s", dump(c.Screen))
	}
}

func TestSSprStretchesAndShrinks(t *testing.T) {
	s, err := ParseSprite("12|34")
	if err != nil {
		t.Fatal(err)
	}
	c := New(4, 4)
	c.SSpr(s, 0, 0, 2, 2, 0, 0, 4, 4, false, false)
	wantScreen(t, c, `
		1122
		1122
		3344
		3344`)

	big := NewSurface(4, 4)
	big.Fill(6)
	c.Cls(0)
	c.SSpr(big, 0, 0, 4, 4, 0, 0, 2, 2, false, false)
	wantScreen(t, c, `
		66..
		66..
		....
		....`)
}

func TestSSprTakesPartOfASheet(t *testing.T) {
	sheet, err := ParseSprite(`
		1122
		1122
		3344
		3344`)
	if err != nil {
		t.Fatal(err)
	}
	c := New(2, 2)
	c.SSpr(sheet, 2, 2, 2, 2, 0, 0, 2, 2, false, false) // the bottom right tile
	if got := dump(c.Screen); got != "44\n44" {
		t.Errorf("screen:\n%s", got)
	}
}

func TestBlitIgnoresEmptyAndBackwardsRectangles(t *testing.T) {
	c := New(2, 2)
	s := NewSurface(2, 2)
	s.Fill(7)
	c.Blit(s, 0, 0, 2, 2, 0, 0, 0, 2, false, false)   // no width
	c.Blit(s, 0, 0, 2, 2, 0, 0, -4, -4, false, false) // backwards
	c.Blit(s, 0, 0, 0, 0, 0, 0, 2, 2, false, false)   // nothing to read
	c.Blit(nil, 0, 0, 2, 2, 0, 0, 2, 2, false, false)
	if got := dump(c.Screen); got != "..\n.." {
		t.Errorf("screen:\n%s", got)
	}
}

func TestBlitOfAnEnormousDestinationCostsOnlyTheScreen(t *testing.T) {
	// A program can scale a sprite by a silly factor; only the pixels that can
	// be seen should be touched, so that this returns rather than grinding.
	c := New(8, 8)
	s := NewSurface(2, 2)
	s.Fill(7)
	c.SSpr(s, 0, 0, 2, 2, -1<<20, -1<<20, 1<<21, 1<<21, false, false)
	if got := c.Screen.Get(4, 4); got != 7 {
		t.Errorf("the middle of the screen is %d, want 7", got)
	}
}

func TestDrawingIntoASurfaceThenDrawingItBack(t *testing.T) {
	c := New(6, 3)
	stamp := NewSurface(2, 2)
	c.SetTarget(stamp)
	c.RectFill(0, 0, 1, 1, 9)
	c.SetTarget(nil)
	for i := 0; i < 3; i++ {
		c.Spr(stamp, i*2, 1, false, false)
	}
	wantScreen(t, c, `
		......
		999999
		999999`)
}

func TestFromImageReducesToThePalette(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 1))
	r, g, b := RGB(8)
	img.Set(0, 0, color.RGBA{r, g, b, 255})           // exactly a palette colour
	img.Set(1, 0, color.RGBA{r - 2, g, b + 1, 255})   // near enough to be the same
	img.Set(2, 0, color.RGBA{0xff, 0xff, 0xff, 0x10}) // too faint to count
	s := FromImage(img)
	if got := s.Get(0, 0); got != 8 {
		t.Errorf("exact colour became %d, want 8", got)
	}
	if got := s.Get(1, 0); got != 8 {
		t.Errorf("near colour became %d, want 8", got)
	}
	if got := s.Get(2, 0); got != 0 {
		t.Errorf("transparent pixel became %d, want 0", got)
	}
}
