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
		if got := Small.Width(c.text); got != c.w {
			t.Errorf("Small.Width(%q) = %d, want %d", c.text, got, c.w)
		}
		if got := Small.Height(c.text); got != c.h {
			t.Errorf("Small.Height(%q) = %d, want %d", c.text, got, c.h)
		}
	}
}

func TestTextWidthMeasuresTheWidestLine(t *testing.T) {
	if got, want := Small.Width("AAAA\nA"), 4*CharWidth; got != want {
		t.Errorf("TextWidth = %d, want %d", got, want)
	}
}

func TestEveryPrintableAsciiHasAGlyph(t *testing.T) {
	// A missing glyph prints as a hole in the middle of a sentence, which is
	// the kind of thing nobody notices until a game says it.
	for r := rune(' ' + 1); r < 127; r++ {
		if _, ok := Small.glyph(r); !ok {
			t.Errorf("%q (%d) has no glyph", r, r)
		}
	}
	if _, ok := Small.glyph(' '); ok {
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
		g, _ := Small.glyph(r)
		if g[0] != 0 {
			t.Errorf("%q has ink on its top row; the lower case should be short", r)
		}
		if g[1] == 0 {
			t.Errorf("%q is missing its second row", r)
		}
	}
	for r := 'A'; r <= 'Z'; r++ {
		g, _ := Small.glyph(r)
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
		g, _ := Small.glyph(r)
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

func TestUnsciiIsAWholeEightByEightFont(t *testing.T) {
	f := Unscii()
	if f.Name != "unscii" || f.W != 8 || f.H != 8 || f.Advance != 8 || f.Line != 8 {
		t.Fatalf("unscii is %+v", f)
	}

	// Everything printable, which is the least a font has to manage.
	for r := rune(' ' + 1); r < 127; r++ {
		if !f.Has(r) {
			t.Errorf("%q (%d) has no glyph", r, r)
		}
	}
	// And the reason for having it: the characters the small font cannot fit.
	for _, r := range []rune{'é', 'ü', 'ñ', 'ß', 'Γ', 'Ω', 'Д', 'ж', '→', '↑', '░', '▓', '█', '╔', '═', '▲', '♦'} {
		if !f.Has(r) {
			t.Errorf("%q is missing", r)
		}
	}
	if f.Has(' ') {
		t.Error("space should have no ink")
	}

	// A letter really is the letter: unscii's A, row by row.
	want := []string{
		"...##...",
		"..####..",
		".##..##.",
		".##..##.",
		".######.",
		".##..##.",
		".##..##.",
		"........",
	}
	var got []string
	for row := 0; row < 8; row++ {
		line := []byte("........")
		g, _ := f.glyph('A')
		for col := 0; col < 8; col++ {
			if g[row]&(0x80>>uint(col)) != 0 {
				line[col] = '#'
			}
		}
		got = append(got, string(line))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d of 'A' is %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAConsoleCanChangeFont(t *testing.T) {
	c := New(40, 20)
	if c.Font() != Small {
		t.Fatalf("a console starts with %v", c.Font().Name)
	}

	was := c.SetFont(Unscii())
	if was != Small {
		t.Error("SetFont should report the font it replaced")
	}
	if got := c.TextWidth("abc"); got != 24 {
		t.Errorf("three unscii characters are %d wide, want 24", got)
	}
	if got := c.TextHeight("a\nb"); got != 16 {
		t.Errorf("two unscii lines are %d tall, want 16", got)
	}

	// The text really is drawn bigger: the small font's 'I' cannot reach the
	// eighth column, and unscii's 'H' does.
	c.Cls(0)
	c.Print("H", 0, 0, 7)
	tall := 0
	for y := 0; y < 20; y++ {
		if c.Screen.Get(1, y) != 0 {
			tall++
		}
	}
	if tall != 7 {
		t.Errorf("unscii's H is %d rows tall in its left column, want 7", tall)
	}

	// Nothing puts the small one back.
	c.SetFont(nil)
	if c.Font() != Small {
		t.Error("SetFont(nil) should go back to the small font")
	}
	if got := c.TextWidth("abc"); got != 3*CharWidth {
		t.Errorf("back on the small font three characters are %d wide", got)
	}
}

func TestAFontMadeOfArtwork(t *testing.T) {
	// Three characters of 4x5, starting at 'A', drawn in colour 9 — and drawn
	// on the screen in whatever colour print() is given, not in 9.
	sheet, err := ParseSprite(`
		.99.9999.99.
		9..99..99...
		9..99999....
		9..99...9...
		.99.9....99.
	`)
	if err != nil {
		t.Fatal(err)
	}
	sheet.SetGrid(4, 5)

	f := FromSheet(sheet, 'A')
	if f == nil || f.W != 4 || f.H != 5 || f.Advance != 4 || f.Line != 5 {
		t.Fatalf("the font is %+v", f)
	}
	if !f.Has('A') || !f.Has('C') {
		t.Error("its three characters should be there")
	}
	if f.Has('Z') {
		t.Error("there is no fourth cell")
	}

	c := New(20, 8)
	c.SetFont(f)
	c.Print("A", 0, 0, 12)

	if got := c.Screen.Get(1, 0); got != 12 {
		t.Errorf("the top of the A is colour %d, want the 12 it was printed in", got)
	}
	if got := c.Screen.Get(0, 0); got != 0 {
		t.Errorf("the corner of the cell is colour %d, want nothing", got)
	}
	if got := c.TextWidth("AB"); got != 8 {
		t.Errorf("two characters of it are %d wide, want 8", got)
	}

	// A picture with no grid is not a font.
	if FromSheet(NewSurface(8, 8), 'A') != nil {
		t.Error("a surface with no cells cannot be a font")
	}
	if FromSheet(nil, 'A') != nil {
		t.Error("nothing at all cannot be a font")
	}
}

func TestTheBuiltInFontsAreFoundByName(t *testing.T) {
	for name, want := range map[string]*Font{
		"small":    Small,
		"":         Small,
		"unscii":   Unscii(),
		"UNSCII":   Unscii(),
		"unscii-8": Unscii(),
		" small ":  Small,
	} {
		got, ok := FontByName(name)
		if !ok || got != want {
			t.Errorf("FontByName(%q) = %v, %v", name, got, ok)
		}
	}
	if _, ok := FontByName("comic sans"); ok {
		t.Error("there is no such font")
	}
	if len(Fonts()) != 2 {
		t.Errorf("the built-in fonts are %v", Fonts())
	}
}
