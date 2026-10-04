package pico

// Text is drawn with a font, and a console has one in hand at a time. Small is
// the one it starts with; Unscii is the other built-in; and a surface of
// artwork can be a font too. See font.go's second half for the type.
//
// The small font: glyphs three pixels wide and five tall, drawn on a grid four
// wide and six tall so that letters and lines do not touch.
const (
	GlyphWidth  = 3
	GlyphHeight = 5
	CharWidth   = 4 // what one character advances the cursor
	LineHeight  = 6 // what a newline advances it
)

// glyphArt is the font, one line per character: five rows of three pixels,
// '#' where there is ink. Anything missing from this table prints as a blank,
// which is how space gets drawn.
//
// Three pixels is not enough width to tell a lower case letter from its capital
// the way a real typeface does, so the lower case is drawn as small capitals,
// four rows tall against the capitals' five. PICO-8's font makes the same
// compromise, and text drawn with it reads the way an arcade cabinet's does.
var glyphArt = map[rune]string{
	'!':  ".#.|.#.|.#.|...|.#.",
	'"':  "#.#|#.#|...|...|...",
	'#':  "#.#|###|#.#|###|#.#",
	'$':  "###|##.|###|.##|###",
	'%':  "#.#|..#|.#.|#..|#.#",
	'&':  "##.|##.|###|#.#|###",
	'\'': ".#.|.#.|...|...|...",
	'(':  ".#.|#..|#..|#..|.#.",
	')':  ".#.|..#|..#|..#|.#.",
	'*':  "#.#|.#.|#.#|...|...",
	'+':  "...|.#.|###|.#.|...",
	',':  "...|...|...|.#.|#..",
	'-':  "...|...|###|...|...",
	'.':  "...|...|...|...|.#.",
	'/':  "..#|..#|.#.|#..|#..",
	'0':  "###|#.#|#.#|#.#|###",
	'1':  ".#.|##.|.#.|.#.|###",
	'2':  "###|..#|###|#..|###",
	'3':  "###|..#|.##|..#|###",
	'4':  "#.#|#.#|###|..#|..#",
	'5':  "###|#..|###|..#|###",
	'6':  "###|#..|###|#.#|###",
	'7':  "###|..#|.#.|.#.|.#.",
	'8':  "###|#.#|###|#.#|###",
	'9':  "###|#.#|###|..#|###",
	':':  "...|.#.|...|.#.|...",
	';':  "...|.#.|...|.#.|#..",
	'<':  "..#|.#.|#..|.#.|..#",
	'=':  "...|###|...|###|...",
	'>':  "#..|.#.|..#|.#.|#..",
	'?':  "###|..#|.##|...|.#.",
	'@':  "###|#.#|###|#..|###",
	'A':  ".#.|#.#|###|#.#|#.#",
	'B':  "##.|#.#|##.|#.#|##.",
	'C':  ".##|#..|#..|#..|.##",
	'D':  "##.|#.#|#.#|#.#|##.",
	'E':  "###|#..|##.|#..|###",
	'F':  "###|#..|##.|#..|#..",
	'G':  ".##|#..|#.#|#.#|.##",
	'H':  "#.#|#.#|###|#.#|#.#",
	'I':  "###|.#.|.#.|.#.|###",
	'J':  "..#|..#|..#|#.#|###",
	'K':  "#.#|#.#|##.|#.#|#.#",
	'L':  "#..|#..|#..|#..|###",
	'M':  "#.#|###|#.#|#.#|#.#",
	'N':  "##.|#.#|#.#|#.#|#.#",
	'O':  "###|#.#|#.#|#.#|###",
	'P':  "###|#.#|###|#..|#..",
	'Q':  "###|#.#|#.#|###|..#",
	'R':  "###|#.#|##.|#.#|#.#",
	'S':  "###|#..|###|..#|###",
	'T':  "###|.#.|.#.|.#.|.#.",
	'U':  "#.#|#.#|#.#|#.#|###",
	'V':  "#.#|#.#|#.#|#.#|.#.",
	'W':  "#.#|#.#|#.#|###|#.#",
	'X':  "#.#|#.#|.#.|#.#|#.#",
	'Y':  "#.#|#.#|.#.|.#.|.#.",
	'Z':  "###|..#|.#.|#..|###",
	'[':  ".##|.#.|.#.|.#.|.##",
	'\\': "#..|#..|.#.|..#|..#",
	']':  "##.|.#.|.#.|.#.|##.",
	'^':  ".#.|#.#|...|...|...",
	'_':  "...|...|...|...|###",
	'`':  "#..|.#.|...|...|...",
	'a':  "...|.#.|#.#|###|#.#",
	'b':  "...|##.|#.#|##.|###",
	'c':  "...|.##|#..|#..|.##",
	'd':  "...|##.|#.#|#.#|##.",
	'e':  "...|###|##.|#..|###",
	'f':  "...|###|#..|##.|#..",
	'g':  "...|.##|#..|#.#|###",
	'h':  "...|#.#|#.#|###|#.#",
	'i':  "...|###|.#.|.#.|###",
	'j':  "...|..#|..#|#.#|###",
	'k':  "...|#.#|##.|##.|#.#",
	'l':  "...|#..|#..|#..|###",
	'm':  "...|###|###|#.#|#.#",
	'n':  "...|##.|#.#|#.#|#.#",
	'o':  "...|###|#.#|#.#|###",
	'p':  "...|###|#.#|###|#..",
	'q':  "...|###|#.#|###|..#",
	'r':  "...|###|#.#|##.|#.#",
	's':  "...|.##|##.|..#|##.",
	't':  "...|###|.#.|.#.|.#.",
	'u':  "...|#.#|#.#|#.#|###",
	'v':  "...|#.#|#.#|#.#|.#.",
	'w':  "...|#.#|#.#|###|###",
	'x':  "...|#.#|.#.|.#.|#.#",
	'y':  "...|#.#|#.#|.#.|.#.",
	'z':  "...|###|..#|#..|###",
	'{':  ".##|.#.|#..|.#.|.##",
	'|':  ".#.|.#.|.#.|.#.|.#.",
	'}':  "##.|.#.|..#|.#.|##.",
	'~':  "...|#..|###|..#|...",

	// A few shapes worth having in a game's text: a solid block for bars and
	// borders, and two marks for lives and score.
	'█': "###|###|###|###|###", // full block
	'•': "...|...|.#.|...|...", // bullet
	'♥': "#.#|###|###|.#.|...", // heart
}

/* --- what a font is --- */

// maxGlyphRows is as tall as a glyph drawn from bits can be. Both built-in
// fonts are shorter; a font that wants more is drawn from artwork instead.
const maxGlyphRows = 8

// glyph is one character as rows of pixels, bit 7 being the leftmost. Left
// aligned whatever the font's width, so that drawing does not have to know it.
type glyph [maxGlyphRows]uint8

// Font is a set of glyphs and how far they move the pen.
//
// Two kinds go through here. A built-in font is a table of bits, which costs
// nothing and draws in whatever colour print() is given. A font made from a
// surface is artwork: every pixel that is not colour 0 is ink, and it too is
// drawn in the colour asked for, so a program can recolour its own lettering
// without redrawing it.
type Font struct {
	// Name is what the font is called, and what font() reports. A font made
	// from artwork has no name of its own.
	Name string

	// W and H are the glyph box; Advance is what one character moves the pen
	// along, and Line what a newline moves it down. The gap between letters is
	// Advance less W, which a font built from artwork has drawn into it.
	W, H          int
	Advance, Line int

	ascii [128]glyph     // the fast path
	wide  map[rune]glyph // everything else

	// sheet is the artwork a font drawn as sprites comes from, and first is
	// the character its first cell stands for.
	sheet *Surface
	first rune
}

// Small is the font a console starts with: three pixels by five, the one the
// glyph table above draws.
var Small = buildSmall()

func buildSmall() *Font {
	f := &Font{
		Name: "small",
		W:    GlyphWidth, H: GlyphHeight,
		Advance: CharWidth, Line: LineHeight,
		wide: map[rune]glyph{},
	}
	for r, art := range glyphArt {
		g := parseGlyph(art)
		if r >= 0 && r < 128 {
			f.ascii[r] = g
			continue
		}
		f.wide[r] = g
	}
	return f
}

// parseGlyph reads one character out of the table above: rows separated by
// bars, '#' where there is ink.
func parseGlyph(art string) glyph {
	var g glyph
	row, col := 0, 0
	for _, r := range art {
		switch {
		case r == '|':
			row, col = row+1, 0
			if row >= GlyphHeight {
				return g
			}
		case r == '#':
			if col < GlyphWidth {
				g[row] |= 0x80 >> uint(col)
			}
			col++
		default:
			col++
		}
	}
	return g
}

// FromSheet makes a font out of artwork: a surface cut into cells, each cell
// one character, starting at first and counting up.
//
// Every pixel that is not colour 0 is ink. The spacing between letters is
// whatever the artist left inside the cell, which is why a glyph moves the pen
// by the width of its whole cell.
func FromSheet(s *Surface, first rune) *Font {
	if s == nil || !s.Gridded() {
		return nil
	}
	return &Font{
		W: s.CellW, H: s.CellH,
		Advance: s.CellW, Line: s.CellH,
		sheet: s,
		first: first,
	}
}

// Sheet reports the artwork a font was made from, and nothing for a built-in.
func (f *Font) Sheet() *Surface { return f.sheet }

// each calls fn for every ink pixel of a character, at offsets from its top
// left, and reports whether the font had anything to draw at all.
func (f *Font) each(r rune, fn func(dx, dy int)) bool {
	if f.sheet != nil {
		n := int(r - f.first)
		x, y, w, h, ok := f.sheet.Cell(n, 1, 1)
		if !ok {
			return false
		}
		for dy := 0; dy < h; dy++ {
			for dx := 0; dx < w; dx++ {
				if f.sheet.Get(x+dx, y+dy) != 0 {
					fn(dx, dy)
				}
			}
		}
		return true
	}

	g, ok := f.glyph(r)
	if !ok {
		return false
	}
	for dy := 0; dy < f.H && dy < maxGlyphRows; dy++ {
		bits := g[dy]
		if bits == 0 {
			continue
		}
		for dx := 0; dx < f.W; dx++ {
			if bits&(0x80>>uint(dx)) != 0 {
				fn(dx, dy)
			}
		}
	}
	return true
}

// glyph reports the pixels of one character, and whether the font has it.
func (f *Font) glyph(r rune) (glyph, bool) {
	if r >= 0 && r < 128 {
		g := f.ascii[r]
		return g, g != glyph{}
	}
	g, ok := f.wide[r]
	return g, ok
}

// Has reports whether a font can draw a character.
func (f *Font) Has(r rune) bool {
	if f.sheet != nil {
		_, _, _, _, ok := f.sheet.Cell(int(r-f.first), 1, 1)
		return ok
	}
	_, ok := f.glyph(r)
	return ok
}

// Width reports how wide text is once drawn, counting the blank column after
// the last character so that centring with it looks even.
func (f *Font) Width(text string) int {
	widest, line := 0, 0
	for _, r := range text {
		if r == '\n' {
			widest, line = max(widest, line), 0
			continue
		}
		line += f.Advance
	}
	return max(widest, line)
}

// Height reports how tall text is once drawn.
func (f *Font) Height(text string) int {
	lines := 1
	for _, r := range text {
		if r == '\n' {
			lines++
		}
	}
	return lines*f.Line - (f.Line - f.H)
}

/* --- drawing with one --- */

// Font reports the font text is being drawn with.
func (c *Console) Font() *Font {
	if c.font == nil {
		c.font = Small
	}
	return c.font
}

// SetFont puts a font in hand and reports the one it replaced. Nothing at all
// puts the small one back.
func (c *Console) SetFont(f *Font) *Font {
	was := c.Font()
	if f == nil {
		f = Small
	}
	c.font = f
	return was
}

// TextWidth and TextHeight measure text in the font this console is using.
func (c *Console) TextWidth(text string) int  { return c.Font().Width(text) }
func (c *Console) TextHeight(text string) int { return c.Font().Height(text) }

// Print draws text with its top left corner at x, y and leaves the cursor on
// the line below, which is what makes a run of bare print() calls stack down
// the screen the way they do on the consoles this imitates.
func (c *Console) Print(text string, x, y int, col uint8) {
	f := c.Font()
	penX, penY := x-c.camX, y-c.camY
	for _, r := range text {
		if r == '\n' {
			penX, penY = x-c.camX, penY+f.Line
			continue
		}
		atX, atY := penX, penY
		f.each(r, func(dx, dy int) { c.put(atX+dx, atY+dy, col) })
		penX += f.Advance
	}
	c.cursorX, c.cursorY = x, y+f.Line
}

// PrintLine draws text at the cursor and moves the cursor down a line.
func (c *Console) PrintLine(text string, col uint8) {
	c.Print(text, c.cursorX, c.cursorY, col)
}

// Cursor moves the text cursor, which is where a print() without a position
// draws, and where a newline returns to.
func (c *Console) Cursor(x, y int) {
	c.cursorX, c.cursorY = x, y
}

// CursorAt reports the text cursor.
func (c *Console) CursorAt() (x, y int) { return c.cursorX, c.cursorY }
