package pico

// The built-in font: glyphs three pixels wide and five tall, drawn on a grid
// four wide and six tall so that letters and lines do not touch.
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

// glyph is one character as five rows of pixels, bit 2 being the leftmost.
type glyph [GlyphHeight]uint8

// glyphs is the parsed font. The common characters are a plain array lookup;
// the handful outside Latin-1 fall back to a map.
var (
	glyphsASCII [128]glyph
	glyphsWide  = map[rune]glyph{}
)

func init() {
	for r, art := range glyphArt {
		g := parseGlyph(art)
		if r < 128 {
			glyphsASCII[r] = g
			continue
		}
		glyphsWide[r] = g
	}
}

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
				g[row] |= 1 << uint(GlyphWidth-1-col)
			}
			col++
		default:
			col++
		}
	}
	return g
}

// glyphOf reports the pixels of one character, and whether it has any.
func glyphOf(r rune) (glyph, bool) {
	if r >= 0 && r < 128 {
		g := glyphsASCII[r]
		return g, g != glyph{}
	}
	g, ok := glyphsWide[r]
	return g, ok
}

// TextWidth reports how wide text is once drawn, counting the blank column
// after the last character so that centring with it looks even.
func TextWidth(text string) int {
	widest, line := 0, 0
	for _, r := range text {
		if r == '\n' {
			widest, line = max(widest, line), 0
			continue
		}
		line += CharWidth
	}
	return max(widest, line)
}

// TextHeight reports how tall text is once drawn.
func TextHeight(text string) int {
	lines := 1
	for _, r := range text {
		if r == '\n' {
			lines++
		}
	}
	return lines*LineHeight - (LineHeight - GlyphHeight)
}

// Print draws text with its top left corner at x, y and leaves the cursor on
// the line below, which is what makes a run of bare print() calls stack down
// the screen the way they do on the consoles this imitates.
func (c *Console) Print(text string, x, y int, col uint8) {
	penX, penY := x-c.camX, y-c.camY
	for _, r := range text {
		if r == '\n' {
			penX, penY = x-c.camX, penY+LineHeight
			continue
		}
		if g, ok := glyphOf(r); ok {
			for gy := 0; gy < GlyphHeight; gy++ {
				bits := g[gy]
				if bits == 0 {
					continue
				}
				for gx := 0; gx < GlyphWidth; gx++ {
					if bits&(1<<uint(GlyphWidth-1-gx)) != 0 {
						c.put(penX+gx, penY+gy, col)
					}
				}
			}
		}
		penX += CharWidth
	}
	c.cursorX, c.cursorY = x, y+LineHeight
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
