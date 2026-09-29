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
}

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

// Clone copies a surface, pixels and all.
func (s *Surface) Clone() *Surface {
	out := NewSurface(s.W, s.H)
	copy(out.Pix, s.Pix)
	return out
}

// Resize grows or shrinks a surface in place, keeping the pixels that still
// fit. The screen uses it when a program asks for a different resolution.
func (s *Surface) Resize(w, h int) {
	if w == s.W && h == s.H {
		return
	}
	next := NewSurface(w, h)
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
	return FromImage(img, pal), nil
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
