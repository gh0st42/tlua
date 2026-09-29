package pico

import (
	"strings"
	"testing"
	"unicode"
)

func TestPrintDrawsAGlyph(t *testing.T) {
	c := New(4, 6)
	c.Print("A", 0, 0, 7)
	wantScreen(t, c, `
		.7..
		7.7.
		777.
		7.7.
		7.7.
		....`)
}

func TestPrintAdvancesOneCellPerCharacter(t *testing.T) {
	c := New(9, 5)
	c.Print("11", 0, 0, 7)
	// Two digits, four pixels apart, with the fifth column blank between them.
	if c.Screen.Get(4, 4) == 0 || c.Screen.Get(3, 4) != 0 {
		t.Errorf("characters are not a cell apart:\n%s", dump(c.Screen))
	}
}

func TestPrintStartsANewLineAtTheStartingColumn(t *testing.T) {
	c := New(8, 12)
	c.Print("A\nA", 2, 0, 7)
	// The second line begins under the first, not at the left edge.
	if c.Screen.Get(3, 0) == 0 || c.Screen.Get(3, LineHeight) == 0 {
		t.Errorf("the two lines are not aligned:\n%s", dump(c.Screen))
	}
}

func TestPrintLeavesTheCursorOnTheNextLine(t *testing.T) {
	c := New(20, 20)
	c.Print("hi", 3, 4, 7)
	if x, y := c.CursorAt(); x != 3 || y != 4+LineHeight {
		t.Errorf("cursor at %d,%d, want 3,%d", x, y, 4+LineHeight)
	}
	// So a run of bare prints stacks down the screen.
	c.Cls(0)
	c.Cursor(0, 0)
	c.PrintLine("1", 7)
	c.PrintLine("1", 7)
	if c.Screen.Get(1, 0) == 0 || c.Screen.Get(1, LineHeight) == 0 {
		t.Errorf("the second line did not follow the first:\n%s", dump(c.Screen))
	}
}

func TestPrintUsesTheCameraAndTheClip(t *testing.T) {
	c := New(8, 8)
	c.Camera(0, 2)
	c.Print("A", 0, 2, 7) // two rows down in the world, so at the top of the screen
	if c.Screen.Get(1, 0) == 0 {
		t.Errorf("text ignored the camera:\n%s", dump(c.Screen))
	}
}

func TestTextWidthAndHeightMatchWhatIsDrawn(t *testing.T) {
	cases := []struct {
		text string
		w, h int
	}{
		{"", 0, GlyphHeight},
		{"A", CharWidth, GlyphHeight},
		{"AB", 2 * CharWidth, GlyphHeight},
		{"A\nBC", 2 * CharWidth, LineHeight + GlyphHeight},
	}
	for _, c := range cases {
		if got := TextWidth(c.text); got != c.w {
			t.Errorf("TextWidth(%q) = %d, want %d", c.text, got, c.w)
		}
		if got := TextHeight(c.text); got != c.h {
			t.Errorf("TextHeight(%q) = %d, want %d", c.text, got, c.h)
		}
	}
}

func TestTextWidthMeasuresTheWidestLine(t *testing.T) {
	if got, want := TextWidth("AAAA\nA"), 4*CharWidth; got != want {
		t.Errorf("TextWidth = %d, want %d", got, want)
	}
}

func TestEveryPrintableAsciiHasAGlyph(t *testing.T) {
	// A missing glyph prints as a hole in the middle of a sentence, which is
	// the kind of thing nobody notices until a game says it.
	for r := rune(' ' + 1); r < 127; r++ {
		if _, ok := glyphOf(r); !ok {
			t.Errorf("%q (%d) has no glyph", r, r)
		}
	}
	if _, ok := glyphOf(' '); ok {
		t.Error("space should have no ink")
	}
}

func TestGlyphsFitTheirCell(t *testing.T) {
	for r, art := range glyphArt {
		rows := strings.Split(art, "|")
		if len(rows) != GlyphHeight {
			t.Errorf("%q has %d rows, want %d", r, len(rows), GlyphHeight)
		}
		for i, row := range rows {
			if len(row) != GlyphWidth {
				t.Errorf("%q row %d is %q, want %d pixels", r, i, row, GlyphWidth)
			}
			if strings.Trim(row, ".#") != "" {
				t.Errorf("%q row %d is %q; only '.' and '#' are pixels", r, i, row)
			}
		}
	}
}

func TestLowerCaseIsShorterThanUpperCase(t *testing.T) {
	// The lower case is drawn as small capitals, so every one of them should
	// leave the top row of the cell empty.
	for r := 'a'; r <= 'z'; r++ {
		g, _ := glyphOf(r)
		if g[0] != 0 {
			t.Errorf("%q has ink on its top row; the lower case should be short", r)
		}
		if g[1] == 0 {
			t.Errorf("%q is missing its second row", r)
		}
	}
	for r := 'A'; r <= 'Z'; r++ {
		g, _ := glyphOf(r)
		if g[0] == 0 {
			t.Errorf("%q has no ink on its top row", r)
		}
	}
}

func TestUpperAndLowerCaseAreToldApart(t *testing.T) {
	seen := map[glyph]rune{}
	for r := rune(33); r < 127; r++ {
		if !unicode.IsLetter(r) {
			continue
		}
		g, _ := glyphOf(r)
		if first, ok := seen[g]; ok {
			t.Errorf("%q and %q are drawn identically", first, r)
		}
		seen[g] = r
	}
}

func TestTextThatRunsOffTheScreenIsClipped(t *testing.T) {
	c := New(4, 6)
	c.Print("AAAA", -6, 0, 7) // starts off the left edge
	c.Print("AAAA", 0, 20, 7) // and below the bottom
	if len(dump(c.Screen)) != 4*6+5 {
		t.Error("printing off screen changed the size of the screen")
	}
}
