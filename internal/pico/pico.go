// Package pico is a fantasy console: an indexed-colour framebuffer, a palette,
// and the drawing calls a small game needs.
//
// It knows nothing about Lua and nothing about a window, which is what makes it
// testable — every call here can be checked by looking at the pixels it left
// behind. Package picolua puts a Picotron-flavoured Lua API on top of it, and
// package game puts it on screen with Ebitengine.
//
// The model is the one PICO-8 and Picotron share: one screen, one pen colour,
// one camera offset, one clipping rectangle, and a palette that can be remapped
// either as things are drawn or as the frame reaches the display. Drawing state
// lives on the Console rather than the Surface, so redirecting drawing to an
// offscreen surface changes nothing else about how the calls behave.
package pico

import "encoding/binary"

// The console's display. Picotron's 480x270 is 16:9, a sixteenth of 1920x1080,
// so it scales to a full screen by an exact integer on the usual monitors.
const (
	ScreenWidth  = 480
	ScreenHeight = 270
)

// Buttons are the six a player has, numbered as PICO-8 numbers them, which is
// the order a program indexes btn() with.
const (
	BtnLeft = iota
	BtnRight
	BtnUp
	BtnDown
	BtnO // z, c or n on a keyboard; the lower face button on a pad
	BtnX // x, v or m on a keyboard; the right face button on a pad

	Buttons
)

// Players is how many sets of buttons the console tracks.
const Players = 4

// Rect is a clipping rectangle, half open: X1 and Y1 are the first column and
// row outside it.
type Rect struct{ X0, Y0, X1, Y1 int }

// Console holds the screen, whatever is being drawn on, and all the state that
// changes what the next drawing call does.
type Console struct {
	// Screen is what reaches the display.
	Screen *Surface

	target *Surface
	clip   Rect

	pen    uint8 // the colour drawing uses
	penAlt uint8 // the colour the set bits of a fill pattern use

	camX, camY int

	fill            uint16 // a 4x4 dither, bit 15 being the top left pixel
	fillTransparent bool   // set bits leave the pixel alone instead of drawing

	// drawPal remaps a colour as it is drawn, which is how a sprite is
	// recoloured without touching its pixels. screenPal remaps as the frame
	// reaches the display, which is how the whole picture fades at once.
	drawPal   [Colors]uint8
	screenPal [Colors]uint8

	// transparent marks the colours sprite drawing skips.
	transparent [Colors]bool

	// pal is what the indices look like. It is the console's hardware palette:
	// changing an entry changes every pixel already drawn in that colour.
	pal *Palette

	// The text cursor print() advances when it is called without a position.
	cursorX, cursorY int

	// font is what print() draws with. Nothing means the small built-in one.
	font *Font
}

// New creates a console with a screen of the given size, ready to draw on: pen
// colour 6 on a black screen, no camera, no clipping, colour 0 transparent in
// sprites.
func New(w, h int) *Console {
	c := &Console{Screen: NewSurface(w, h), pal: Default.Clone()}
	c.target = c.Screen
	c.Reset()
	return c
}

// Reset returns every drawing setting to its starting value without touching
// the pixels, so that a program restarting does not inherit a camera offset or
// a palette swap from whatever ran before it.
func (c *Console) Reset() {
	c.target = c.Screen
	c.pen, c.penAlt = 6, 0
	c.camX, c.camY = 0, 0
	c.fill, c.fillTransparent = 0, false
	c.ResetPal()
	c.cursorX, c.cursorY = 0, 0
	c.font = Small
	c.ClipReset()
}

// ResetPal restores both palettes and puts transparency back to colour 0 alone.
func (c *Console) ResetPal() {
	for i := 0; i < Colors; i++ {
		c.drawPal[i] = uint8(i)
		c.screenPal[i] = uint8(i)
		c.transparent[i] = false
	}
	c.transparent[0] = true
}

// Palette reports the colours in use.
func (c *Console) Palette() *Palette { return c.pal }

// SetPalette changes what the indices look like, and reports the palette it
// replaced. Every pixel on the screen keeps its index and so changes colour.
//
// The palette is copied, so that a console always owns its own: a program
// changing one entry of the built-in palette would otherwise change it for
// everything else using it.
//
// The draw and screen palettes go back to passing colours through untouched: a
// remap of one index onto another rarely means the same thing in a palette it
// was not written for, and a swap left over from the last one is harder to find
// than one that has to be asked for again.
func (c *Console) SetPalette(p *Palette) *Palette {
	if p == nil {
		p = Default
	}
	old := c.pal
	c.pal = p.Clone()
	c.ResetPal()
	return old
}

/* --- what is being drawn on --- */

// Target reports the surface drawing lands on.
func (c *Console) Target() *Surface { return c.target }

// SetTarget redirects drawing to a surface, or back to the screen when given
// nil. Clipping is reset to the whole of the new target, since a rectangle that
// made sense on the screen rarely makes sense on a 16x16 sprite; the camera is
// left alone, so a scene can be redrawn into an offscreen surface unchanged.
func (c *Console) SetTarget(s *Surface) {
	if s == nil {
		s = c.Screen
	}
	c.target = s
	c.ClipReset()
}

// Resize changes the resolution of the screen, keeping what still fits.
func (c *Console) Resize(w, h int) {
	onScreen := c.target == c.Screen
	c.Screen.Resize(w, h)
	if onScreen {
		c.target = c.Screen
		c.ClipReset()
	}
}

/* --- drawing state --- */

// Color sets the pen colour and reports the one it replaced, which is what lets
// the Lua binding return the previous colour the way PICO-8's color() does.
func (c *Console) Color(col uint8) uint8 {
	old := c.pen
	c.pen = col
	return old
}

// Pen reports the current pen colour.
func (c *Console) Pen() uint8 { return c.pen }

// SetPenAlt sets the colour the set bits of a fill pattern draw in.
func (c *Console) SetPenAlt(col uint8) { c.penAlt = col }

// Camera shifts every later drawing call by -x, -y, so a program can draw a
// world in world coordinates and move the view instead of the world.
func (c *Console) Camera(x, y int) (oldX, oldY int) {
	oldX, oldY = c.camX, c.camY
	c.camX, c.camY = x, y
	return
}

// CameraAt reports the camera offset.
func (c *Console) CameraAt() (x, y int) { return c.camX, c.camY }

// Clip confines drawing to a rectangle of the target, in screen coordinates:
// the camera does not move it, because it is a property of the view rather than
// of the world. When previous is set the rectangle is intersected with the one
// already in force, which is how a panel clips inside a window.
func (c *Console) Clip(x, y, w, h int, previous bool) Rect {
	old := c.clip
	next := Rect{x, y, x + w, y + h}
	if previous {
		next = intersect(next, old)
	}
	c.clip = intersect(next, c.bounds())
	return old
}

// ClipReset lifts clipping back to the whole target.
func (c *Console) ClipReset() { c.clip = c.bounds() }

// SetClip puts back a rectangle Clip reported, for code that saves and restores
// it around a nested draw.
func (c *Console) SetClip(r Rect) { c.clip = intersect(r, c.bounds()) }

// ClipRect reports the clipping rectangle in force.
func (c *Console) ClipRect() Rect { return c.clip }

func (c *Console) bounds() Rect { return Rect{0, 0, c.target.W, c.target.H} }

func intersect(a, b Rect) Rect {
	out := Rect{max(a.X0, b.X0), max(a.Y0, b.Y0), min(a.X1, b.X1), min(a.Y1, b.Y1)}
	if out.X1 < out.X0 {
		out.X1 = out.X0
	}
	if out.Y1 < out.Y0 {
		out.Y1 = out.Y0
	}
	return out
}

// Pal remaps colour from to colour to. A draw-time remap recolours whatever is
// drawn next; a screen remap recolours the finished frame on its way to the
// display, leaving the framebuffer alone.
func (c *Console) Pal(from, to uint8, screen bool) {
	if screen {
		c.screenPal[from] = to
		return
	}
	c.drawPal[from] = to
}

// Palt marks a colour transparent, or opaque again, for sprite drawing.
func (c *Console) Palt(col uint8, transparent bool) {
	c.transparent[col] = transparent
}

// PaltNone makes every colour opaque, including colour 0.
func (c *Console) PaltNone() {
	for i := range c.transparent {
		c.transparent[i] = false
	}
}

// Fillp sets the 4x4 dither pattern later fills use. Bit 15 of the pattern is
// its top left pixel and bit 0 its bottom right. Where a bit is set the pixel
// is either left alone or drawn in the second pen colour, depending on
// transparent; a pattern of 0 turns dithering off.
func (c *Console) Fillp(pattern uint16, transparent bool) uint16 {
	old := c.fill
	c.fill, c.fillTransparent = pattern, transparent
	return old
}

/* --- the innermost loop --- */

// plot draws one pixel of a filled shape: clipped, dithered by the fill
// pattern, and remapped by the draw palette.
func (c *Console) plot(x, y int, col uint8) {
	if x < c.clip.X0 || x >= c.clip.X1 || y < c.clip.Y0 || y >= c.clip.Y1 {
		return
	}
	if c.fill != 0 && c.fill&(0x8000>>uint((y&3)*4+(x&3))) != 0 {
		if c.fillTransparent {
			return
		}
		col = c.penAlt
	}
	c.target.Pix[y*c.target.W+x] = c.drawPal[col]
}

// put draws one pixel without the fill pattern, for sprites and text, where a
// dither would eat the picture rather than shade it.
func (c *Console) put(x, y int, col uint8) {
	if x < c.clip.X0 || x >= c.clip.X1 || y < c.clip.Y0 || y >= c.clip.Y1 {
		return
	}
	c.target.Pix[y*c.target.W+x] = c.drawPal[col]
}

// span fills a horizontal run, which is where a filled shape spends its time.
// Without a fill pattern it is a memset over the row; with one it goes pixel by
// pixel.
func (c *Console) span(x0, x1, y int, col uint8) {
	if y < c.clip.Y0 || y >= c.clip.Y1 || x1 < x0 {
		return
	}
	x0, x1 = max(x0, c.clip.X0), min(x1, c.clip.X1-1)
	if x1 < x0 {
		return
	}
	if c.fill != 0 {
		for x := x0; x <= x1; x++ {
			c.plot(x, y, col)
		}
		return
	}
	row := c.target.Pix[y*c.target.W:]
	mapped := c.drawPal[col]
	for x := x0; x <= x1; x++ {
		row[x] = mapped
	}
}

/* --- the frame on its way out --- */

// Pixels writes the screen into dst as RGBA bytes, applying the screen palette,
// and reports how many bytes it filled. dst must hold 4 bytes per pixel. The
// buffer is the caller's so that a host can hand the same one to the graphics
// card every frame without allocating.
func (c *Console) Pixels(dst []byte) int {
	n := len(c.Screen.Pix) * 4
	if len(dst) < n {
		return 0
	}
	// The screen palette and the colours behind it are two indirections per
	// pixel, so they are folded into one table of ready-made pixels per frame
	// instead. Each is packed into a word, which turns the inner loop into one
	// load and one store rather than four of each.
	var lut [Colors]uint32
	for i := 0; i < Colors; i++ {
		e := c.pal.pix[c.screenPal[i]]
		lut[i] = uint32(e[0]) | uint32(e[1])<<8 | uint32(e[2])<<16 | uint32(e[3])<<24
	}
	for i, col := range c.Screen.Pix {
		binary.LittleEndian.PutUint32(dst[i*4:], lut[col])
	}
	return n
}
