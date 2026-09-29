package pico

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Colors is how many colours a pixel can hold. A pixel is a byte, so the
// console can address 256 of them however many the palette in use defines;
// being a power of two, an index is masked into range rather than checked,
// which matters in the inner loop of every drawing call.
const Colors = 256

// Palette is the set of colours a program draws with: what each index looks
// like on the screen.
//
// The console stores an index per pixel, so changing a palette entry changes
// every pixel already drawn in it — which is how a whole picture fades or
// flashes for the cost of one write.
type Palette struct {
	// Name is what the palette is called, for a program that wants to say.
	Name string

	rgb  [Colors]uint32  // 0xRRGGBB per index
	pix  [Colors][4]byte // the same, as the bytes a texture wants
	size int             // how many of them the palette defines
}

// NewPalette builds a palette from colours given as 0xRRGGBB. Anything past
// the end of the list is black, and a list longer than 256 is cut short.
func NewPalette(name string, colors []uint32) *Palette {
	p := &Palette{Name: name}
	p.size = min(len(colors), Colors)
	for i := 0; i < p.size; i++ {
		p.Set(uint8(i), colors[i])
	}
	return p
}

// Size reports how many colours the palette defines. Colour numbers from a
// program wrap around it, so a palette of 16 makes colour 17 colour 1.
func (p *Palette) Size() int { return p.size }

// Set changes one entry, and reports what it was.
func (p *Palette) Set(i uint8, rgb uint32) uint32 {
	old := p.rgb[i]
	rgb &= 0xffffff
	p.rgb[i] = rgb
	p.pix[i] = [4]byte{byte(rgb >> 16), byte(rgb >> 8), byte(rgb), 0xff}
	if int(i) >= p.size {
		p.size = int(i) + 1
	}
	return old
}

// Hex reports one entry as 0xRRGGBB.
func (p *Palette) Hex(i uint8) uint32 { return p.rgb[i] }

// RGB reports one entry as three channels.
func (p *Palette) RGB(i uint8) (r, g, b uint8) {
	e := p.pix[i]
	return e[0], e[1], e[2]
}

// Clone copies a palette, so that a program changing entries does not change
// the built-in everyone else shares.
func (p *Palette) Clone() *Palette {
	out := *p
	return &out
}

// Nearest reports the entry closest to an RGB value, which is how an imported
// image is reduced to the palette. The comparison is a sum of squares in RGB
// space, weighted towards green because that is the channel the eye reads
// detail from: crude next to a perceptual metric, but predictable, and artwork
// drawn in these colours maps back to them exactly.
func (p *Palette) Nearest(r, g, b uint8) uint8 {
	best, bestDist := uint8(0), int32(1<<31-1)
	for i := 0; i < p.size; i++ {
		e := p.pix[i]
		dr, dg, db := int32(r)-int32(e[0]), int32(g)-int32(e[1]), int32(b)-int32(e[2])
		d := 2*dr*dr + 4*dg*dg + 3*db*db
		if d < bestDist {
			best, bestDist = uint8(i), d
		}
	}
	return best
}

/* --- the palettes that come with the console --- */

// Default is the console's own palette.
//
// The first 16 are PICO-8's palette and the next 16 its extended ("secret")
// one, both reproduced exactly, because the artwork and the colour lore of
// that world is what anyone writing against this API will have in mind.
// Picotron's own remaining colours are not published as a list, so 32-63 here
// are tlua's: four ramps and a grey scale, chosen to be useful for shading
// rather than to imitate anything.
var Default = NewPalette("default", []uint32{
	// 0-15: PICO-8.
	0x000000, 0x1d2b53, 0x7e2553, 0x008751,
	0xab5236, 0x5f574f, 0xc2c3c7, 0xfff1e8,
	0xff004d, 0xffa300, 0xffec27, 0x00e436,
	0x29adff, 0x83769c, 0xff77a8, 0xffccaa,

	// 16-31: PICO-8's extended palette, the darker and dustier companions to
	// the first sixteen.
	0x291814, 0x111d35, 0x422136, 0x125359,
	0x742f29, 0x49333b, 0xa28879, 0xf3ef7d,
	0xbe1250, 0xff6c24, 0xa8e72e, 0x00b543,
	0x065ab5, 0x754665, 0xff6e59, 0xff9d81,

	// 32-39: a grey scale, for interfaces and fades.
	0x0d0d0d, 0x262626, 0x3d3d3d, 0x595959,
	0x757575, 0x949494, 0xb8b8b8, 0xe8e8e8,

	// 40-45: night to daylight blue.
	0x041033, 0x0b2266, 0x1240a8, 0x2f6fd8,
	0x63a7f2, 0xa8d8ff,

	// 46-51: undergrowth to new leaf.
	0x03210f, 0x0a4a1e, 0x137a2c, 0x27b043,
	0x63dd6b, 0xb6f5a0,

	// 52-57: embers to candlelight.
	0x2b0a12, 0x5e1022, 0x9c1f2d, 0xd4442f,
	0xf58a3c, 0xffd07a,

	// 58-63: deep violet to orchid.
	0x1a0a2b, 0x371159, 0x5e2394, 0x8f45c9,
	0xc07fe8, 0xe8c0ff,
})

// VGA is the 256 colours an IBM VGA card came up in, which is what a DOS game
// in mode 13h had to work with.
//
// The first 16 are the EGA colours and the next 16 the grey scale, both as they
// were. The 216 after them are built the way the card's own table was laid
// out — three tiers of brightness, each with three of saturation, each running
// twenty-four hues round the colour wheel from blue — rather than copied out of
// a table entry by entry, so a few of them are a shade off what a real card
// would have put on the screen. The last eight are black, as they were there.
var VGA = NewPalette("vga", vgaColors())

func vgaColors() []uint32 {
	out := make([]uint32, 0, Colors)

	// The EGA sixteen: two thirds brightness, then full.
	for _, c := range []uint32{
		0x000000, 0x0000aa, 0x00aa00, 0x00aaaa,
		0xaa0000, 0xaa00aa, 0xaa5500, 0xaaaaaa,
		0x555555, 0x5555ff, 0x55ff55, 0x55ffff,
		0xff5555, 0xff55ff, 0xffff55, 0xffffff,
	} {
		out = append(out, c)
	}

	// Sixteen greys, at the levels the card used, in sixths of a sixty-fourth.
	for _, v := range []int{0, 5, 8, 11, 14, 17, 20, 24, 28, 32, 36, 40, 45, 50, 56, 63} {
		g := uint32(v * 255 / 63)
		out = append(out, g<<16|g<<8|g)
	}

	// Nine blocks of twenty-four: brightness outermost, then saturation, then
	// the hue, a quarter of the way round each group of four.
	for _, value := range []int{63, 45, 28} {
		for _, floor := range []int{0, value / 2, value * 3 / 4} {
			for hue := 0; hue < 24; hue++ {
				r, g, b := wheel(hue, value, floor)
				out = append(out, uint32(r)<<16|uint32(g)<<8|uint32(b))
			}
		}
	}

	for len(out) < Colors {
		out = append(out, 0)
	}
	return out
}

// wheel reports one of twenty-four hues, given the brightest and dimmest a
// channel may be. It walks the six edges of the colour cube from blue, four
// steps to an edge.
func wheel(hue, value, floor int) (r, g, b uint8) {
	const steps = 4
	segment, step := hue/steps, hue%steps

	// Blue, magenta, red, yellow, green, cyan, and back to blue. Each corner
	// says which channels are at the top and which at the bottom.
	corners := [7][3]int{
		{0, 0, 1}, {1, 0, 1}, {1, 0, 0}, {1, 1, 0},
		{0, 1, 0}, {0, 1, 1}, {0, 0, 1},
	}
	from, to := corners[segment], corners[segment+1]

	channel := func(i int) uint8 {
		lo, hi := floor, value
		start, end := lo, lo
		if from[i] == 1 {
			start = hi
		}
		if to[i] == 1 {
			end = hi
		}
		v := start + (end-start)*step/steps
		return uint8(v * 255 / 63)
	}
	return channel(0), channel(1), channel(2)
}

// Builtin reports a palette that comes with the console, by name.
func Builtin(name string) (*Palette, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "default", "pico", "picotron":
		return Default, true
	case "vga", "dos", "mode13h":
		return VGA, true
	}
	return nil, false
}

// BuiltinNames lists the palettes that come with the console.
func BuiltinNames() []string { return []string{"default", "vga"} }

/* --- palettes from disk --- */

// LoadGPL reads a GIMP palette file, which is what nearly every pixel art tool
// will export: a header, then a line of red, green and blue per colour.
func LoadGPL(path string) (*Palette, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	p, err := ParseGPL(f, name)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

// ParseGPL reads a GIMP palette. The name in the file wins over the one passed
// in, which is only a fallback for a file that does not say.
func ParseGPL(r io.Reader, name string) (*Palette, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	colors := []uint32{}
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if first {
			first = false
			// The format's own first line. A file without it is almost
			// certainly not a palette, and reading it as one would produce
			// nonsense rather than an error.
			if !strings.HasPrefix(line, "GIMP Palette") {
				return nil, errors.New("not a GIMP palette (no \"GIMP Palette\" line)")
			}
			continue
		}

		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "Name:"):
			if v := strings.TrimSpace(strings.TrimPrefix(line, "Name:")); v != "" {
				name = v
			}
			continue
		case strings.HasPrefix(line, "Columns:"):
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue // a stray line, or a comment without its hash
		}
		var channels [3]uint32
		ok := true
		for i := 0; i < 3; i++ {
			v, err := strconv.Atoi(fields[i])
			if err != nil || v < 0 || v > 255 {
				ok = false
				break
			}
			channels[i] = uint32(v)
		}
		if !ok {
			continue
		}
		colors = append(colors, channels[0]<<16|channels[1]<<8|channels[2])
		if len(colors) == Colors {
			break // a palette larger than the console can hold
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(colors) == 0 {
		return nil, errors.New("the palette has no colours in it")
	}
	return NewPalette(name, colors), nil
}
