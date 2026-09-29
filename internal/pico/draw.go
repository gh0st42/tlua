package pico

import "math"

// Cls clears the target to one colour, lifts clipping and puts the text cursor
// back in the corner — the fresh start PICO-8's cls() gives, so that a program
// calling it each frame cannot accumulate state it forgot about.
func (c *Console) Cls(col uint8) {
	c.target.Fill(c.drawPal[col&(Colors-1)])
	c.ClipReset()
	c.cursorX, c.cursorY = 0, 0
}

// Pset draws one pixel in world coordinates.
func (c *Console) Pset(x, y int, col uint8) {
	c.plot(x-c.camX, y-c.camY, col)
}

// Pget reads one pixel of the target, in the same world coordinates Pset uses.
// Outside the target it reports 0, the way reading past screen memory would.
func (c *Console) Pget(x, y int) uint8 {
	return c.target.Get(x-c.camX, y-c.camY)
}

// Line draws a line between two points, ends included.
func (c *Console) Line(x0, y0, x1, y1 int, col uint8) {
	x0, y0, x1, y1 = x0-c.camX, y0-c.camY, x1-c.camX, y1-c.camY

	// Clipping first, so that a line to a wild coordinate costs what the line
	// on screen costs rather than what the program asked for.
	cx0, cy0, cx1, cy1, ok := clipLine(x0, y0, x1, y1, c.clip)
	if !ok {
		return
	}
	x0, y0, x1, y1 = cx0, cy0, cx1, cy1

	dx, sx := x1-x0, 1
	if dx < 0 {
		dx, sx = -dx, -1
	}
	dy, sy := y1-y0, 1
	if dy < 0 {
		dy, sy = -dy, -1
	}
	err := dx - dy
	for {
		c.plot(x0, y0, col)
		if x0 == x1 && y0 == y1 {
			return
		}
		// The doubled error decides which axis steps, the standard integer
		// Bresenham with no division anywhere.
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// Rect draws the outline of a rectangle, corners included.
func (c *Console) Rect(x0, y0, x1, y1 int, col uint8) {
	x0, y0, x1, y1 = x0-c.camX, y0-c.camY, x1-c.camX, y1-c.camY
	x0, x1 = minmax(x0, x1)
	y0, y1 = minmax(y0, y1)
	c.span(x0, x1, y0, col)
	if y1 != y0 {
		c.span(x0, x1, y1, col)
	}
	for y := max(y0+1, c.clip.Y0); y <= min(y1-1, c.clip.Y1-1); y++ {
		c.plot(x0, y, col)
		if x1 != x0 {
			c.plot(x1, y, col)
		}
	}
}

// RectFill fills a rectangle, corners included.
func (c *Console) RectFill(x0, y0, x1, y1 int, col uint8) {
	x0, y0, x1, y1 = x0-c.camX, y0-c.camY, x1-c.camX, y1-c.camY
	x0, x1 = minmax(x0, x1)
	y0, y1 = minmax(y0, y1)
	for y := max(y0, c.clip.Y0); y <= min(y1, c.clip.Y1-1); y++ {
		c.span(x0, x1, y, col)
	}
}

// Circ draws the outline of a circle of radius r. As in PICO-8 the radius sets
// a bounding box, so circ(x,y,r) is the oval that fits x-r,y-r to x+r,y+r and a
// radius of 0 is a single pixel.
func (c *Console) Circ(x, y, r int, col uint8) {
	if r < 0 {
		return
	}
	c.Oval(x-r, y-r, x+r, y+r, col)
}

// CircFill fills a circle of radius r.
func (c *Console) CircFill(x, y, r int, col uint8) {
	if r < 0 {
		return
	}
	c.OvalFill(x-r, y-r, x+r, y+r, col)
}

// OvalFill fills the ellipse that touches the sides of a rectangle.
func (c *Console) OvalFill(x0, y0, x1, y1 int, col uint8) {
	o := c.ovalOf(x0, y0, x1, y1)
	for y := max(o.y0, c.clip.Y0); y <= min(o.y1, c.clip.Y1-1); y++ {
		l, r := o.extent(y)
		c.span(l, r, y, col)
	}
}

// Oval draws the outline of the ellipse that touches the sides of a rectangle.
//
// A pixel belongs to the outline when a neighbour of it is outside the ellipse.
// Taking the rows above and below into account matters: deciding from the row
// above alone leaves the bottom of the shape open wherever it narrows, and
// drawing only the two end pixels of each row leaves gaps wherever it widens
// faster than a pixel a row.
func (c *Console) Oval(x0, y0, x1, y1 int, col uint8) {
	o := c.ovalOf(x0, y0, x1, y1)
	for y := max(o.y0, c.clip.Y0); y <= min(o.y1, c.clip.Y1-1); y++ {
		l, r := o.extent(y)
		if l > r {
			continue
		}
		// The run this row shares with both neighbours is interior; whatever
		// sticks out of it is edge.
		inL, inR := math.MinInt32, math.MaxInt32
		for _, ny := range [2]int{y - 1, y + 1} {
			if ny < o.y0 || ny > o.y1 {
				inL, inR = math.MaxInt32, math.MinInt32 // no neighbour: all edge
				break
			}
			nl, nr := o.extent(ny)
			inL, inR = max(inL, nl), min(inR, nr)
		}
		c.span(l, min(r, inL-1), y, col)
		c.span(max(l, inR+1), r, y, col)
		c.plot(l, y, col) // the left and right ends are always edge
		c.plot(r, y, col)
	}
}

// oval is an ellipse resolved into a row-by-row half width, the one shape
// description both the filled and the outlined form need.
type oval struct {
	cx, cy, a, b float64
	y0, y1       int
}

// edgeBias grows the ellipse by a quarter of a pixel before it is sampled.
//
// Sampled exactly, a circle of radius four has a single pixel at each pole,
// because the pole is precisely one radius from the centre and nothing else in
// that row is within it: a shape with a spike on it rather than a circle. The
// midpoint algorithm these consoles drew with has no such spike, and a quarter
// of a pixel of slack reproduces its widths — three pixels at the pole of a
// radius of four, nine across the middle — without giving up the row-by-row
// form that the fill and the outline share.
const edgeBias = 0.25

func (c *Console) ovalOf(x0, y0, x1, y1 int) oval {
	x0, y0, x1, y1 = x0-c.camX, y0-c.camY, x1-c.camX, y1-c.camY
	x0, x1 = minmax(x0, x1)
	y0, y1 = minmax(y0, y1)
	return oval{
		cx: float64(x0+x1) / 2,
		cy: float64(y0+y1) / 2,
		a:  float64(x1-x0) / 2,
		b:  float64(y1-y0) / 2,
		y0: y0,
		y1: y1,
	}
}

// extent reports the first and last column of one row of the ellipse. Rows
// above and below the shape come back empty, with the first column past the
// last.
func (o oval) extent(y int) (int, int) {
	a, b := o.a+edgeBias, o.b+edgeBias
	dy := (float64(y) - o.cy) / b
	t := 1 - dy*dy
	if t < 0 {
		return 1, 0
	}
	w := a * math.Sqrt(t)
	l, r := int(math.Ceil(o.cx-w)), int(math.Floor(o.cx+w))
	if l > r {
		// A tall thin ellipse can taper to less than a pixel at the tip; the
		// row is inside the shape, so it gets the one pixel nearest its middle
		// rather than a gap.
		l = int(math.Round(o.cx))
		r = l
	}
	return l, r
}

// TriFill fills a triangle. Neither PICO-8 nor Picotron has one; a great deal
// of two-dimensional game drawing wants one anyway, so here it is.
func (c *Console) TriFill(x0, y0, x1, y1, x2, y2 int, col uint8) {
	x0, y0 = x0-c.camX, y0-c.camY
	x1, y1 = x1-c.camX, y1-c.camY
	x2, y2 = x2-c.camX, y2-c.camY

	// Each row is filled between the outermost places the three edges reach it.
	// Asking every edge, rather than following one long edge and two short
	// ones, is what keeps a flat edge whole: it meets its row at both ends at
	// once, and a scan expecting one crossing per edge would draw only one of
	// them.
	edges := [3][4]int{{x0, y0, x1, y1}, {x1, y1, x2, y2}, {x2, y2, x0, y0}}
	top, bottom := min(y0, min(y1, y2)), max(y0, max(y1, y2))

	for y := max(top, c.clip.Y0); y <= min(bottom, c.clip.Y1-1); y++ {
		lo, hi := math.MaxInt32, math.MinInt32
		for _, e := range edges {
			ax, ay, bx, by := e[0], e[1], e[2], e[3]
			if y < min(ay, by) || y > max(ay, by) {
				continue
			}
			if ay == by {
				lo, hi = min(lo, min(ax, bx)), max(hi, max(ax, bx))
				continue
			}
			// Always walk the edge downwards, so that the rounding does not
			// depend on which way round the corners were given.
			if by < ay {
				ax, ay, bx, by = bx, by, ax, ay
			}
			x := ax + (bx-ax)*(y-ay)/(by-ay)
			lo, hi = min(lo, x), max(hi, x)
		}
		if lo <= hi {
			c.span(lo, hi, y, col)
		}
	}
}

// Tri draws the outline of a triangle.
func (c *Console) Tri(x0, y0, x1, y1, x2, y2 int, col uint8) {
	c.Line(x0, y0, x1, y1, col)
	c.Line(x1, y1, x2, y2, col)
	c.Line(x2, y2, x0, y0, col)
}

/* --- sprites --- */

// Spr draws a surface at its own size, skipping the colours Palt marks
// transparent.
func (c *Console) Spr(src *Surface, x, y int, flipX, flipY bool) {
	if src == nil {
		return
	}
	c.Blit(src, 0, 0, src.W, src.H, x, y, src.W, src.H, flipX, flipY)
}

// SSpr draws part of a surface, stretched to fill the destination rectangle.
func (c *Console) SSpr(src *Surface, sx, sy, sw, sh, dx, dy, dw, dh int, flipX, flipY bool) {
	c.Blit(src, sx, sy, sw, sh, dx, dy, dw, dh, flipX, flipY)
}

// Blit copies a rectangle of src onto the target, scaling it to dw by dh,
// optionally mirrored, and skipping transparent colours.
//
// It walks the destination, not the source: that way the scaling has no gaps, a
// clipped sprite costs only what is on screen, and a program that asks for a
// destination the size of the world does not hang.
func (c *Console) Blit(src *Surface, sx, sy, sw, sh, dx, dy, dw, dh int, flipX, flipY bool) {
	if src == nil || sw <= 0 || sh <= 0 || dw <= 0 || dh <= 0 {
		return
	}
	dx, dy = dx-c.camX, dy-c.camY

	// Only the part inside both the clipping rectangle and the destination is
	// worth walking.
	x0, x1 := max(dx, c.clip.X0), min(dx+dw, c.clip.X1)
	y0, y1 := max(dy, c.clip.Y0), min(dy+dh, c.clip.Y1)
	if x0 >= x1 || y0 >= y1 {
		return
	}

	for py := y0; py < y1; py++ {
		row := py - dy
		if flipY {
			row = dh - 1 - row
		}
		srcY := sy + row*sh/dh
		for px := x0; px < x1; px++ {
			col := px - dx
			if flipX {
				col = dw - 1 - col
			}
			v := src.Get(sx+col*sw/dw, srcY)
			if c.transparent[v&(Colors-1)] {
				continue
			}
			c.target.Pix[py*c.target.W+px] = c.drawPal[v&(Colors-1)]
		}
	}
}

func minmax(a, b int) (int, int) {
	if a > b {
		return b, a
	}
	return a, b
}

// clipLine trims a line to a rectangle by the Cohen-Sutherland method, and
// reports false when none of it is inside.
func clipLine(x0, y0, x1, y1 int, r Rect) (int, int, int, int, bool) {
	// The rectangle is half open, so the last pixel inside it is X1-1.
	xmax, ymax := r.X1-1, r.Y1-1
	if xmax < r.X0 || ymax < r.Y0 {
		return 0, 0, 0, 0, false
	}

	const (
		left = 1 << iota
		right
		bottom
		top
	)
	code := func(x, y int) int {
		out := 0
		switch {
		case x < r.X0:
			out |= left
		case x > xmax:
			out |= right
		}
		switch {
		case y < r.Y0:
			out |= top
		case y > ymax:
			out |= bottom
		}
		return out
	}

	c0, c1 := code(x0, y0), code(x1, y1)
	for {
		switch {
		case c0|c1 == 0:
			return x0, y0, x1, y1, true
		case c0&c1 != 0:
			return 0, 0, 0, 0, false // both ends beyond the same edge
		}

		// Move whichever end is outside onto the edge it is outside of.
		out, x, y := c0, 0, 0
		if c0 == 0 {
			out = c1
		}
		switch {
		case out&left != 0:
			x, y = r.X0, y0+(y1-y0)*(r.X0-x0)/(x1-x0)
		case out&right != 0:
			x, y = xmax, y0+(y1-y0)*(xmax-x0)/(x1-x0)
		case out&top != 0:
			x, y = x0+(x1-x0)*(r.Y0-y0)/(y1-y0), r.Y0
		default:
			x, y = x0+(x1-x0)*(ymax-y0)/(y1-y0), ymax
		}
		if out == c0 {
			x0, y0, c0 = x, y, code(x, y)
		} else {
			x1, y1, c1 = x, y, code(x, y)
		}
	}
}
