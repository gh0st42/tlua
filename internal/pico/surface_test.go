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
	r, g, b := Default.RGB(8)
	img.Set(0, 0, color.RGBA{r, g, b, 255})           // exactly a palette colour
	img.Set(1, 0, color.RGBA{r - 2, g, b + 1, 255})   // near enough to be the same
	img.Set(2, 0, color.RGBA{0xff, 0xff, 0xff, 0x10}) // too faint to count
	s := FromImage(img, Default)
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

func TestASheetIsCutIntoCells(t *testing.T) {
	// Twenty across by sixteen down, in cells of eight: two cells across and
	// two down, with four pixels along the right edge belonging to nothing.
	s := NewSurface(20, 16)
	if s.Gridded() {
		t.Error("a surface starts out as a picture, not a sheet")
	}
	s.SetGrid(8, 8)

	w, h, count := s.Grid()
	if w != 8 || h != 8 || count != 4 {
		t.Errorf("grid is %dx%d with %d cells, want 8x8 with 4", w, h, count)
	}
	if got := s.Across(); got != 2 {
		t.Errorf("%d cells fit across, want 2", got)
	}

	cases := []struct {
		n, wide, tall int
		x, y, cw, ch  int
	}{
		{0, 1, 1, 0, 0, 8, 8},
		{1, 1, 1, 8, 0, 8, 8},
		{2, 1, 1, 0, 8, 8, 8},
		{3, 1, 1, 8, 8, 8, 8},
		{0, 2, 2, 0, 0, 16, 16}, // spanning widens the rectangle
		{1, 2, 1, 8, 0, 16, 8},  // and may run off the sheet, which is empty
	}
	for _, c := range cases {
		x, y, cw, ch, ok := s.Cell(c.n, c.wide, c.tall)
		if !ok {
			t.Errorf("sprite %d is not on the sheet", c.n)
			continue
		}
		if x != c.x || y != c.y || cw != c.cw || ch != c.ch {
			t.Errorf("sprite %d (%dx%d cells) is %d,%d %dx%d; want %d,%d %dx%d",
				c.n, c.wide, c.tall, x, y, cw, ch, c.x, c.y, c.cw, c.ch)
		}
	}

	for _, n := range []int{-1, 4, 99} {
		if _, _, _, _, ok := s.Cell(n, 1, 1); ok {
			t.Errorf("sprite %d should not be on a sheet of four", n)
		}
	}
}

func TestASheetWithoutAGridHasNoSprites(t *testing.T) {
	s := NewSurface(16, 16)
	if _, _, count := s.Grid(); count != 0 {
		t.Errorf("a picture holds %d sprites", count)
	}
	if _, _, _, _, ok := s.Cell(0, 1, 1); ok {
		t.Error("a picture has no sprite 0")
	}

	s.SetGrid(8, 8)
	s.SetGrid(0, 0) // and can be made a picture again
	if s.Gridded() {
		t.Error("the grid was not taken off")
	}
}

func TestACellOfNoSizeIsRefused(t *testing.T) {
	s := NewSurface(16, 16)
	s.SetGrid(-4, 8)
	if s.Gridded() {
		t.Error("a cell cannot be smaller than a pixel")
	}
}

func TestTheGridSurvivesCopyingAndResizing(t *testing.T) {
	s := NewSurface(16, 16)
	s.SetGrid(8, 8)

	if w, h, _ := s.Clone().Grid(); w != 8 || h != 8 {
		t.Errorf("a copy has a grid of %dx%d", w, h)
	}

	s.Resize(32, 16)
	w, h, count := s.Grid()
	if w != 8 || h != 8 || count != 8 {
		t.Errorf("after resizing, the grid is %dx%d with %d cells, want 8x8 with 8", w, h, count)
	}
}

func TestDrawingOneSpriteOfASheet(t *testing.T) {
	sheet, err := ParseSprite(`
		1122
		1122
		3344
		3344`)
	if err != nil {
		t.Fatal(err)
	}
	sheet.SetGrid(2, 2)

	c := New(4, 2)
	c.SprCell(sheet, 0, 0, 0, 1, 1, false, false) // the first cell
	c.SprCell(sheet, 3, 2, 0, 1, 1, false, false) // and the last
	wantScreen(t, c, `
		1144
		1144`)

	// A sprite that is not there draws nothing, rather than a sliver of
	// whatever is next to it.
	c.Cls(0)
	c.SprCell(sheet, 9, 0, 0, 1, 1, false, false)
	c.SprCell(nil, 0, 0, 0, 1, 1, false, false)
	wantScreen(t, c, `
		....
		....`)
}

func TestDrawingASpriteThatSpansCells(t *testing.T) {
	sheet, err := ParseSprite(`
		1122
		1122
		3344
		3344`)
	if err != nil {
		t.Fatal(err)
	}
	sheet.SetGrid(2, 2)

	c := New(4, 4)
	c.SprCell(sheet, 0, 0, 0, 2, 2, false, false) // all four cells at once
	wantScreen(t, c, `
		1122
		1122
		3344
		3344`)

	// Flipping a span turns the whole block over, not each cell.
	c.Cls(0)
	c.SprCell(sheet, 0, 0, 0, 2, 2, true, false)
	wantScreen(t, c, `
		2211
		2211
		4433
		4433`)
}
