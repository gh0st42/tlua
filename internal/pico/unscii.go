package pico

import (
	_ "embed"
	"strconv"
	"strings"
	"sync"
)

// Unscii: a second built-in font, eight pixels by eight, with most of Unicode
// in it — box drawing, blocks, arrows, Greek, Cyrillic, and the pile of
// "legacy computing" characters that old machines drew their graphics with.
//
// It is Viznut's unscii-8, from http://viznut.fi/unscii/, in the plain bitmap
// form that distribution ships: one line per character, the codepoint, a colon,
// and the rows as hexadecimal. All unscii variants but unscii-16-full are in
// the public domain.
//
// The bitmaps are embedded rather than the TrueType file of the same font. The
// outlines in that file are squares drawn around these very pixels, so a
// rasteriser would spend a dependency and three times the bytes to arrive back
// where it started — and this screen is one byte a pixel, with no shade of grey
// for it to arrive with.

//go:embed unscii-8.hex
var unsciiHex string

var (
	unsciiOnce sync.Once
	unsciiFont *Font
)

// Unscii reports the font, reading it the first time it is asked for. Most
// programs never are, and a font of three thousand characters is not worth
// building for them.
func Unscii() *Font {
	unsciiOnce.Do(func() { unsciiFont = parseHexFont("unscii", 8, 8, unsciiHex) })
	return unsciiFont
}

// parseHexFont reads a font in the .hex format: "0041:183C66667E666600", a
// codepoint and then one byte a row.
//
// Anything it cannot read is skipped rather than complained about. A font is
// not a program: a line nobody can parse costs one character that will not
// draw, and refusing the whole font over it would be worse.
func parseHexFont(name string, w, h int, data string) *Font {
	f := &Font{
		Name: name,
		W:    w, H: h,
		Advance: w, Line: h,
		wide: map[rune]glyph{},
	}

	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			continue
		}
		code, err := strconv.ParseInt(line[:colon], 16, 32)
		if err != nil || code < 0 {
			continue
		}
		bits := line[colon+1:]
		// Two hex digits a row. Anything wider is a double-width character,
		// which this font has one of and which a fixed grid cannot hold.
		if len(bits) != h*2 {
			continue
		}

		var g glyph
		blank := true
		for row := 0; row < h && row < maxGlyphRows; row++ {
			b, err := strconv.ParseUint(bits[row*2:row*2+2], 16, 8)
			if err != nil {
				blank = true
				break
			}
			g[row] = uint8(b)
			if b != 0 {
				blank = false
			}
		}
		if blank {
			continue // a character with no ink draws as the space it is
		}

		if r := rune(code); r >= 0 && r < 128 {
			f.ascii[r] = g
		} else {
			f.wide[rune(code)] = g
		}
	}
	return f
}

// Fonts are the built-in ones, by the names font() calls them.
func Fonts() []string { return []string{"small", "unscii"} }

// FontByName reports a built-in font, and whether there is one by that name.
func FontByName(name string) (*Font, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "small", "": // the one a console starts with
		return Small, true
	case "unscii", "unscii-8":
		return Unscii(), true
	}
	return nil, false
}
