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

	reader  reader
	overlay overlay

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

	// The window's own keys are decided before the program is given anything,
	// so that the keys they use can be kept from it.
	a.reader.poll()
	action, used := hostShortcut(a.reader.justPressed, held)
	a.apply(action)

	if err := a.rt.Tick(a.reader.frame(a.view, used)); err != nil {
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

// hostAction is what a key belonging to the window, rather than to the program,
// asks for.
type hostAction int

const (
	actionNone hostAction = iota
	actionFullscreen
	actionOverlay
	actionQuit
)

// hostShortcut reports what the window's own keys ask for this tick, and which
// keys that used up.
//
// These belong to the window rather than to the program, and the program is not
// shown the keys they take: a game that reads enter, or D as a direction for a
// second player, would otherwise find alt-enter and ctrl-D meaning two things at
// once. Taking only the key and not the modifier is deliberate — a program
// watching for shift or control is watching for something else.
func hostShortcut(pressed func(ebiten.Key) bool, down func(...ebiten.Key) bool) (hostAction, []ebiten.Key) {
	alt := down(ebiten.KeyAltLeft, ebiten.KeyAltRight)
	// Control or command: whichever of the two the machine's owner reaches for.
	ctrl := down(ebiten.KeyControlLeft, ebiten.KeyControlRight, ebiten.KeyMetaLeft, ebiten.KeyMetaRight)

	switch {
	case pressed(ebiten.KeyF11):
		return actionFullscreen, []ebiten.Key{ebiten.KeyF11}
	case alt && pressed(ebiten.KeyEnter):
		return actionFullscreen, []ebiten.Key{ebiten.KeyEnter}
	case ctrl && pressed(ebiten.KeyD):
		return actionOverlay, []ebiten.Key{ebiten.KeyD}
	case ctrl && pressed(ebiten.KeyQ):
		return actionQuit, []ebiten.Key{ebiten.KeyQ}
	}
	return actionNone, nil
}

// apply carries out a window shortcut.
func (a *app) apply(action hostAction) {
	switch action {
	case actionFullscreen:
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	case actionOverlay:
		a.overlay.toggle()
	case actionQuit:
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

	a.overlay.draw(screen, a.view, frame.W, frame.H)
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
