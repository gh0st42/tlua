package pico

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
)

// Surface is a rectangle of palette indices: the screen, a sprite sheet, or
// anything a program draws on. One byte per pixel is how the consoles this
// imitates worked, and it is what makes a palette swap or a fill pattern a
// cheap trick rather than a shader.
type Surface struct {
	W, H int
	Pix  []uint8 // W*H indices, row major, top row first

	// CellW and CellH are the size of one sprite when the surface is a sheet
	// cut into a grid of them. Zero means it is a single picture, which is a
	// different thing to draw: a sheet is drawn a cell at a time, by number,
	// and a picture is drawn all at once.
	CellW, CellH int

	// flags is eight bits a sprite, meaning whatever the game decides they
	// mean. It is kept beside the pixels because that is where the artwork
	// says it: whoever drew the tile is who knows whether it can be walked on.
	flags []uint8
}

// Flags reports the eight flags of a sprite, all off for one that has none.
func (s *Surface) Flags(n int) uint8 {
	if n < 0 || n >= len(s.flags) {
		return 0
	}
	return s.flags[n]
}

// SetFlags sets all eight at once.
func (s *Surface) SetFlags(n int, mask uint8) {
	if n < 0 || n > maxSprites {
		return
	}
	for len(s.flags) <= n {
		s.flags = append(s.flags, 0)
	}
	s.flags[n] = mask
}

// Flag reports one flag of a sprite, counted from zero.
func (s *Surface) Flag(n, bit int) bool {
	if bit < 0 || bit > 7 {
		return false
	}
	return s.Flags(n)&(1<<uint(bit)) != 0
}

// SetFlag sets or clears one of them.
func (s *Surface) SetFlag(n, bit int, on bool) {
	if bit < 0 || bit > 7 {
		return
	}
	mask := s.Flags(n)
	if on {
		mask |= 1 << uint(bit)
	} else {
		mask &^= 1 << uint(bit)
	}
	s.SetFlags(n, mask)
}

// AnyFlags reports whether any sprite on the sheet carries a flag, which is
// how a program can tell artwork that came with them from artwork that did not.
func (s *Surface) AnyFlags() bool {
	for _, mask := range s.flags {
		if mask != 0 {
			return true
		}
	}
	return false
}

// maxSprites is as far as a flag will be remembered. A sheet of more than this
// many sprites is not a sheet, it is a typo.
const maxSprites = 1 << 16

// NewSurface makes a surface filled with colour 0, which is also the colour
// sprite drawing treats as transparent by default.
func NewSurface(w, h int) *Surface {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return &Surface{W: w, H: h, Pix: make([]uint8, w*h)}
}

// In reports whether a pixel is on the surface.
func (s *Surface) In(x, y int) bool { return x >= 0 && y >= 0 && x < s.W && y < s.H }

// Get reads one pixel, and reports 0 outside the surface so that callers
// sampling a sprite sheet need no bounds check of their own.
func (s *Surface) Get(x, y int) uint8 {
	if !s.In(x, y) {
		return 0
	}
	return s.Pix[y*s.W+x]
}

// Set writes one pixel and ignores anything off the surface.
func (s *Surface) Set(x, y int, col uint8) {
	if !s.In(x, y) {
		return
	}
	s.Pix[y*s.W+x] = col
}

// Fill paints the whole surface one colour.
func (s *Surface) Fill(col uint8) {
	for i := range s.Pix {
		s.Pix[i] = col
	}
}

// Clone copies a surface, pixels, grid and all.
func (s *Surface) Clone() *Surface {
	out := NewSurface(s.W, s.H)
	copy(out.Pix, s.Pix)
	out.CellW, out.CellH = s.CellW, s.CellH
	out.flags = append([]uint8(nil), s.flags...)
	return out
}

/* --- a surface as a sheet of sprites --- */

// SetGrid cuts the surface into cells of a size, making it a sprite sheet.
// A size of nothing makes it a single picture again.
func (s *Surface) SetGrid(w, h int) {
	if w <= 0 || h <= 0 {
		s.CellW, s.CellH = 0, 0
		return
	}
	s.CellW, s.CellH = w, h
}

// Gridded reports whether the surface is a sheet rather than a picture.
func (s *Surface) Gridded() bool { return s.CellW > 0 && s.CellH > 0 }

// Grid reports the size of a cell and how many of them there are.
func (s *Surface) Grid() (w, h, count int) {
	if !s.Gridded() {
		return 0, 0, 0
	}
	return s.CellW, s.CellH, s.Across() * (s.H / s.CellH)
}

// Across reports how many cells fit along the top of the sheet, which is what
// turns a sprite's number into a place on it.
//
// Cells that do not fit whole are not counted: a sheet 20 pixels wide holds two
// sprites of 8 and the four pixels left over belong to nothing. Counting a part
// of a cell would give a sprite number that draws a sliver of the sheet, which
// is never what was meant.
func (s *Surface) Across() int {
	if !s.Gridded() {
		return 0
	}
	return s.W / s.CellW
}

// Cell reports the rectangle sprite n occupies, numbered from zero, left to
// right and then down. Spanning more than one cell widens the rectangle rather
// than moving it, so that a big sprite is the cells to the right of and below
// the one named — which is how these sheets have always been read.
func (s *Surface) Cell(n, wide, tall int) (x, y, w, h int, ok bool) {
	_, _, count := s.Grid()
	if count == 0 || n < 0 || n >= count {
		return 0, 0, 0, 0, false
	}
	across := s.Across()
	return (n % across) * s.CellW, (n / across) * s.CellH,
		max(wide, 1) * s.CellW, max(tall, 1) * s.CellH, true
}

// Resize grows or shrinks a surface in place, keeping the pixels that still
// fit. The screen uses it when a program asks for a different resolution.
func (s *Surface) Resize(w, h int) {
	if w == s.W && h == s.H {
		return
	}
	next := NewSurface(w, h)
	next.CellW, next.CellH = s.CellW, s.CellH
	next.flags = s.flags
	for y := 0; y < h && y < s.H; y++ {
		copy(next.Pix[y*w:y*w+min(w, s.W)], s.Pix[y*s.W:])
	}
	*s = *next
}

// spriteDigits is the alphabet ParseSprite reads: a palette index per
// character, so a sprite can be written out as text in the program that uses
// it. It covers the first 16 colours, as many as there are hex digits and as
// many as a hand-drawn sprite is likely to want; anything beyond them needs a
// surface and Set.
const spriteDigits = "0123456789abcdef"

// ParseSprite reads a sprite drawn as text, one character per pixel:
//
//	0-9 a-f    palette colours 0 to 15
//	. or space transparent, which is to say colour 0
//	|          ends a row, as a newline does
//
// Blank lines at either end are dropped and the indentation the lines share is
// removed, so the art can sit indented inside the program that draws it and
// still mean what it looks like. Short rows are padded with transparent pixels.
// Writing sprites this way keeps an example a single file with no binary
// alongside it, which is the whole reason it exists.
func ParseSprite(art string) (*Surface, error) {
	lines := strings.Split(strings.ReplaceAll(art, "|", "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\r")
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil, errors.New("sprite art is empty")
	}

	// Only the indentation every row shares is layout; what is left of it is
	// transparent pixels the artist meant.
	indent := -1
	for _, line := range lines {
		if line == "" {
			continue
		}
		n := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent < 0 || n < indent {
			indent = n
		}
	}

	rows := make([][]uint8, 0, len(lines))
	width := 0
	for _, line := range lines {
		if len(line) > indent {
			line = line[indent:]
		} else {
			line = ""
		}
		row := make([]uint8, 0, len(line))
		for _, r := range line {
			switch {
			case r == '.' || r == ' ' || r == '\t':
				row = append(row, 0)
			default:
				idx := strings.IndexRune(spriteDigits, r)
				if idx < 0 {
					return nil, fmt.Errorf("sprite art: %q is not a colour (use 0-9, a-f, '.' or ' ')", r)
				}
				row = append(row, uint8(idx))
			}
		}
		if len(row) > width {
			width = len(row)
		}
		rows = append(rows, row)
	}
	if width == 0 {
		return nil, errors.New("sprite art is empty")
	}

	s := NewSurface(width, len(rows))
	for y, row := range rows {
		copy(s.Pix[y*width:], row)
	}
	return s, nil
}

// FromImage reduces an ordinary image to a palette. Pixels that are more
// transparent than not become colour 0, the one sprite drawing skips.
func FromImage(img image.Image, pal *Palette) *Surface {
	b := img.Bounds()
	s := NewSurface(b.Dx(), b.Dy())
	// Real artwork repeats colours heavily, so remembering the last answer
	// saves most of the 64-entry search.
	var lastKey uint32 = 1 << 31
	var lastCol uint8
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			r16, g16, b16, a16 := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if a16 < 0x8000 {
				continue // already 0
			}
			r, g, bb := uint8(r16>>8), uint8(g16>>8), uint8(b16>>8)
			key := uint32(r)<<16 | uint32(g)<<8 | uint32(bb)
			if key != lastKey {
				lastKey, lastCol = key, pal.Nearest(r, g, bb)
			}
			s.Pix[y*s.W+x] = lastCol
		}
	}
	return s
}

// DecodePNG reads a PNG from memory into a surface, reduced to a palette.
//
// Memory rather than a path because a program's artwork does not always come
// from the disk: a game fused into one executable carries its own, and reads it
// out of itself. PNG is the one format built in — image/png needs no cgo and no
// third-party decoder, which is the constraint the whole of tlua is built
// under.
func DecodePNG(data []byte, pal *Palette) (*Surface, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	s := FromImage(img, pal)
	// A sheet drawn in an editor says how big its sprites are and what flags
	// they carry, in a text chunk the image decoder ignores. Reading it here
	// means loadpng("tiles") comes back ready to use.
	s.Apply(ReadPNGMeta(data))
	return s, nil
}

// LoadPNG reads a PNG file from disk.
func LoadPNG(path string, pal *Palette) (*Surface, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s, err := DecodePNG(data, pal)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}
