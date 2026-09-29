package game

import (
	"github.com/hajimehoshi/ebiten/v2"

	"tlua/internal/picolua"
)

// app is the Ebitengine side of a running program: it turns ticks into calls to
// _update and _draw, and the console's framebuffer into pixels on the window.
type app struct {
	s  *session
	rt *picolua.Runtime

	// picture is the console's framebuffer as a texture, and pixels the buffer
	// it is written through. Both are kept between frames: a new texture every
	// frame would be a megabyte of rubbish a second.
	picture *ebiten.Image
	pixels  []byte

	// view is where the picture last landed on the window, which is also how a
	// mouse position is turned back into a console pixel.
	view view

	reader reader

	status   int
	failed   bool // the program stopped with an error; draw nothing more
	stopping bool // the person asked for the window to close
}

func (a *app) Update() error {
	if a.failed || a.stopping {
		return ebiten.Termination
	}

	select {
	case <-a.s.interrupt:
		// The shell's own convention for a program stopped by a signal.
		a.status = 130
		return ebiten.Termination
	default:
	}

	a.handleHostKeys()

	if err := a.rt.Tick(a.reader.frame(a.view)); err != nil {
		a.failed = true
		a.status = a.s.report(err)
		return ebiten.Termination
	}
	if quit, code := a.rt.Quitting(); quit {
		a.status = code
		return ebiten.Termination
	}
	a.applyWindow()
	return nil
}

// handleHostKeys deals with the two keys that belong to the window rather than
// to the program.
func (a *app) handleHostKeys() {
	if a.reader.justPressed(ebiten.KeyF11) ||
		(a.reader.justPressed(ebiten.KeyEnter) && held(ebiten.KeyAltLeft, ebiten.KeyAltRight)) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	if a.reader.justPressed(ebiten.KeyQ) && held(ebiten.KeyControlLeft, ebiten.KeyControlRight, ebiten.KeyMetaLeft, ebiten.KeyMetaRight) {
		a.stopping = true
	}
}

func held(keys ...ebiten.Key) bool {
	for _, k := range keys {
		if ebiten.IsKeyPressed(k) {
			return true
		}
	}
	return false
}

// applyWindow carries out what the program asked of its window.
func (a *app) applyWindow() {
	win, changed := a.rt.Window()
	if !changed {
		return
	}
	ebiten.SetWindowTitle(win.Title)
	if ebiten.IsFullscreen() != win.Fullscreen {
		ebiten.SetFullscreen(win.Fullscreen)
	}
	screen := a.rt.Screen()
	if win.Scale > 0 && !ebiten.IsFullscreen() {
		ebiten.SetWindowSize(screen.W*win.Scale, screen.H*win.Scale)
	}
	ebiten.SetWindowSizeLimits(screen.W/2, screen.H/2, -1, -1)
}

func (a *app) Draw(screen *ebiten.Image) {
	if a.failed || a.stopping {
		return
	}
	if err := a.rt.Draw(); err != nil {
		a.failed = true
		a.status = a.s.report(err)
		return
	}

	frame := a.rt.Screen()
	if a.picture == nil || a.picture.Bounds().Dx() != frame.W || a.picture.Bounds().Dy() != frame.H {
		// The program can change the resolution, so the texture follows it.
		if a.picture != nil {
			a.picture.Deallocate()
		}
		a.picture = ebiten.NewImage(max(frame.W, 1), max(frame.H, 1))
		a.pixels = make([]byte, frame.W*frame.H*4)
	}
	if n := a.rt.Vid.Pixels(a.pixels); n > 0 {
		a.picture.WritePixels(a.pixels)
	}

	bounds := screen.Bounds()
	a.view = fit(bounds.Dx(), bounds.Dy(), frame.W, frame.H)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(a.view.scale), float64(a.view.scale))
	op.GeoM.Translate(float64(a.view.x), float64(a.view.y))
	// Nearest neighbour, because a console pixel is a square of colour and
	// smoothing it is exactly what this is not for.
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(a.picture, op)
}

// LayoutF reports the screen in real pixels rather than in the window's
// device-independent ones, so that one console pixel is a whole number of pixels
// on the glass even where the system is scaling the desktop.
func (a *app) LayoutF(outsideWidth, outsideHeight float64) (float64, float64) {
	scale := 1.0
	if m := ebiten.Monitor(); m != nil && m.DeviceScaleFactor() > 0 {
		scale = m.DeviceScaleFactor()
	}
	return max(outsideWidth*scale, 1), max(outsideHeight*scale, 1)
}

// Layout is what Ebitengine falls back on where LayoutF is not used.
func (a *app) Layout(outsideWidth, outsideHeight int) (int, int) {
	return max(outsideWidth, 1), max(outsideHeight, 1)
}

// view is where the console's picture sits on the window, and how big each of
// its pixels is.
type view struct {
	scale, x, y int
}

// fit works out the largest whole number of screen pixels one console pixel can
// be without spilling over the window, and centres what is left.
//
// Whole numbers matter: at two and a half pixels per pixel, half of the picture's
// rows would be drawn twice and half three times, and a straight line would
// visibly wobble. The margin is left black, the way a picture that does not
// match the screen is shown anywhere else.
func fit(screenW, screenH, w, h int) view {
	if w <= 0 || h <= 0 {
		return view{scale: 1}
	}
	scale := max(min(screenW/w, screenH/h), 1)
	return view{
		scale: scale,
		x:     (screenW - w*scale) / 2,
		y:     (screenH - h*scale) / 2,
	}
}

// consolePixel turns a position on the window into the console pixel under it,
// which is what the program means by mouse().
func (v view) consolePixel(x, y int) (int, int) {
	scale := max(v.scale, 1)
	return floorDiv(x-v.x, scale), floorDiv(y-v.y, scale)
}

// floorDiv divides and rounds down, so that a pointer just off the left edge of
// the picture reads as -1 rather than 0.
func floorDiv(a, b int) int {
	if a < 0 {
		return -((-a + b - 1) / b)
	}
	return a / b
}
