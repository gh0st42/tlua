package game

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"

	"tlua/internal/pico"
)

// overlay is the frame rate counter ctrl-D turns on.
//
// It is drawn on the window over the finished picture rather than into the
// console's framebuffer, so that it cannot smear into a program that does not
// clear the screen, cannot be read back by pget(), and cannot be recoloured by
// whatever palette the program has loaded. It has a small console of its own
// for exactly that reason: the drawing here is the console's own, so the
// counter looks like it belongs to the machine rather than to the window
// library.
type overlay struct {
	on bool

	con    *pico.Console
	img    *ebiten.Image
	pixels []byte
}

// Colours from the default palette, whatever the program is using.
const (
	overlayInk    = 7  // the text
	overlayDim    = 13 // what is less interesting than the number
	overlayGood   = 11 // a frame rate that is keeping up
	overlayFair   = 10
	overlayPoor   = 8
	overlayGround = 0 // behind it all
	overlayEdge   = 5
)

// overlayPad is the border between the text and the edge of the box.
const overlayPad = 3

// toggle turns the counter on or off.
func (o *overlay) toggle() { o.on = !o.on }

// rateColor reports how a frame rate should look: how far it has fallen matters
// more than the number, and colour says it at a glance.
func rateColor(rate, want float64) uint8 {
	switch {
	case rate >= want*0.95:
		return overlayGood
	case rate >= want*0.5:
		return overlayFair
	default:
		return overlayPoor
	}
}

// overlayLines is what the counter says: how fast the window is drawing, how
// fast the program is being run, and what it is being drawn at.
func overlayLines(fps, tps float64, w, h, scale int) []string {
	return []string{
		fmt.Sprintf("%3.0f fps  %3.0f tps", fps, tps),
		fmt.Sprintf("%dx%d  x%d", w, h, scale),
	}
}

// render draws the counter into its own little console and reports the picture
// of it. Keeping this apart from the blitting below is what lets it be looked
// at without opening a window.
func (o *overlay) render(fps, tps float64, w, h, scale int) *pico.Surface {
	lines := overlayLines(fps, tps, w, h, scale)

	width := 0
	for _, line := range lines {
		width = max(width, pico.TextWidth(line))
	}
	// The text measures a blank column after its last character, which is the
	// padding on that side already.
	width += overlayPad*2 - 1
	height := len(lines)*pico.LineHeight + overlayPad*2 - 1

	if o.con == nil {
		o.con = pico.New(width, height)
	} else if o.con.Screen.W != width || o.con.Screen.H != height {
		o.con.Resize(width, height)
	}

	o.con.Cls(overlayGround)
	o.con.Rect(0, 0, width-1, height-1, overlayEdge)

	// The rate itself is coloured by how well it is holding up; the rest of the
	// line is only there to say what the number means.
	rate := fmt.Sprintf("%3.0f", fps)
	o.con.Print(rate, overlayPad, overlayPad, rateColor(fps, float64(TPS)))
	o.con.Print(lines[0][len(rate):], overlayPad+pico.TextWidth(rate), overlayPad, overlayDim)
	o.con.Print(lines[1], overlayPad, overlayPad+pico.LineHeight, overlayInk)

	return o.con.Screen
}

// draw puts the counter in the top right of the picture.
func (o *overlay) draw(screen *ebiten.Image, v view, w, h int) {
	if !o.on {
		return
	}
	picture := o.render(ebiten.ActualFPS(), ebiten.ActualTPS(), w, h, v.scale)

	if o.img == nil || o.img.Bounds().Dx() != picture.W || o.img.Bounds().Dy() != picture.H {
		if o.img != nil {
			o.img.Deallocate()
		}
		o.img = ebiten.NewImage(max(picture.W, 1), max(picture.H, 1))
		o.pixels = make([]byte, picture.W*picture.H*4)
	}
	if n := o.con.Pixels(o.pixels); n > 0 {
		o.img.WritePixels(o.pixels)
	}

	// Drawn at the same scale as the picture, in its top right corner, so that
	// it is the same size to read whatever the window is doing.
	scale := max(v.scale, 1)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(scale), float64(scale))
	op.GeoM.Translate(
		float64(v.x+(w-picture.W)*scale-scale),
		float64(v.y+scale),
	)
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(o.img, op)
}
