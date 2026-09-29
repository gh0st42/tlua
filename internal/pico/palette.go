package pico

// Colors is the number of entries in the palette. Picotron has 64, and a
// power of two means a colour index can be masked into range rather than
// checked, which matters in the inner loop of every drawing call.
const Colors = 64

// palette is the console's fixed 64 colours, as 0xRRGGBB.
//
// The first 16 are PICO-8's palette and the next 16 its extended ("secret")
// one, both reproduced exactly, because artwork and colour lore from that
// world is what anyone writing against this API will have in mind. Picotron's
// own 32 remaining colours are not published as a list, so 32-63 here are
// tlua's: four ramps plus a grey scale, chosen to be useful for shading rather
// than to imitate anything.
var palette = [Colors]uint32{
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
}

// rgba holds the palette as the bytes a texture wants, so turning a frame into
// pixels is a table lookup and three stores rather than any arithmetic.
var rgba = func() [Colors][4]byte {
	var out [Colors][4]byte
	for i, c := range palette {
		out[i] = [4]byte{byte(c >> 16), byte(c >> 8), byte(c), 0xff}
	}
	return out
}()

// RGB reports one palette entry as red, green and blue.
func RGB(col uint8) (r, g, b uint8) {
	e := rgba[col&(Colors-1)]
	return e[0], e[1], e[2]
}

// Hex reports one palette entry as 0xRRGGBB, which is what a Lua program sees.
func Hex(col uint8) uint32 { return palette[col&(Colors-1)] }

// Nearest reports the palette entry closest to an RGB value, which is how an
// imported image is reduced to the console's colours. The comparison is a
// plain sum of squares in RGB space: crude next to a perceptual metric, but
// predictable, and artwork drawn in this palette maps back to it exactly.
func Nearest(r, g, b uint8) uint8 {
	best, bestDist := uint8(0), int32(1<<31-1)
	for i := range palette {
		e := rgba[i]
		dr, dg, db := int32(r)-int32(e[0]), int32(g)-int32(e[1]), int32(b)-int32(e[2])
		// Weighted towards green, the channel the eye reads detail from.
		d := 2*dr*dr + 4*dg*dg + 3*db*db
		if d < bestDist {
			best, bestDist = uint8(i), d
		}
	}
	return best
}
