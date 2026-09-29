// Package picolua puts a Picotron-flavoured game API on a Lua state: cls, spr,
// btn, print and the rest, drawing into a pico.Console.
//
// Nothing here opens a window. The host — package game — gives the runtime a
// frame of input and asks it to run _update and _draw; a test can do the same
// and then look at the pixels, which is how the whole API is checked.
//
// The API follows PICO-8 and Picotron closely enough that code written for them
// mostly reads the same: angles are turns rather than radians, sin() runs the
// same way round as the screen's y axis, a colour handed to a drawing call
// becomes the pen colour, and print() draws on the screen rather than on the
// terminal — printh() is the one that writes where a program was started from.
package picolua

import (
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"

	lua "github.com/yuin/gopher-lua"

	"tlua/internal/pico"
)

// Callbacks the program defines and the runtime calls.
const (
	CallbackInit   = "_init"
	CallbackUpdate = "_update"
	CallbackDraw   = "_draw"
)

// Options configures a runtime.
type Options struct {
	// Width and Height are the size of the screen; zero means the console's
	// own 480x270.
	Width, Height int

	// Title is what the window is called until the program says otherwise.
	Title string

	// Out is where printh() writes, standard output by default.
	Out io.Writer

	// Clock reports how long the program has been running, in seconds. The
	// default counts ticks at sixty a second rather than reading a wall clock,
	// so that motion stays in step with what is drawn and a test can run a
	// hundred frames without waiting for them.
	Clock func() float64

	// FPS reports the frame rate for fps(); zero when the host has no idea.
	FPS func() float64

	// Seed starts the random number generator. Zero picks one, the way a
	// console with no clock would have to.
	Seed int64
}

// Window is what a program has asked its window to be.
type Window struct {
	Title      string
	Scale      int // 0 means "whatever the host thinks best"
	Fullscreen bool
}

// Runtime is a Lua program with the console API installed.
type Runtime struct {
	L   *lua.LState
	Vid *pico.Console
	In  *pico.Input

	out   io.Writer
	clock func() float64
	fps   func() float64
	rng   *rand.Rand

	window        Window
	windowChanged bool

	quit     bool
	quitCode int

	frame int

	// currentTarget is the surface Lua last redirected drawing to, kept so
	// that target() can hand back what it replaced.
	currentTarget lua.LValue

	surfaceMeta *lua.LTable
}

// New installs the API on a Lua state and reports the runtime that drives it.
func New(L *lua.LState, opts Options) *Runtime {
	if opts.Width <= 0 {
		opts.Width = pico.ScreenWidth
	}
	if opts.Height <= 0 {
		opts.Height = pico.ScreenHeight
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.Title == "" {
		opts.Title = "tlua"
	}
	seed := opts.Seed
	if seed == 0 {
		seed = rand.Int63()
	}

	r := &Runtime{
		L:             L,
		Vid:           pico.New(opts.Width, opts.Height),
		In:            pico.NewInput(),
		out:           opts.Out,
		fps:           opts.FPS,
		rng:           rand.New(rand.NewSource(seed)),
		window:        Window{Title: opts.Title},
		currentTarget: lua.LNil,
	}
	r.clock = opts.Clock
	if r.clock == nil {
		// Sixty ticks to the second, counted rather than measured.
		r.clock = func() float64 { return float64(r.frame) / 60 }
	}
	if r.fps == nil {
		r.fps = func() float64 { return 0 }
	}

	r.installSurfaceType()
	r.installDrawing()
	r.installInput()
	r.installSystem()
	r.installStdlib()
	return r
}

// Screen reports the framebuffer the program draws on.
func (r *Runtime) Screen() *pico.Surface { return r.Vid.Screen }

// Frame reports how many ticks have run.
func (r *Runtime) Frame() int { return r.frame }

// Has reports whether the program defined one of the callbacks.
func (r *Runtime) Has(callback string) bool {
	_, ok := r.L.GetGlobal(callback).(*lua.LFunction)
	return ok
}

// Init runs the program's _init, if it has one.
func (r *Runtime) Init() error { return r.call(CallbackInit) }

// Tick takes in a frame of input and runs the program's _update.
func (r *Runtime) Tick(f pico.Frame) error {
	r.In.Update(f)
	err := r.call(CallbackUpdate)
	r.frame++
	return err
}

// Draw runs the program's _draw.
func (r *Runtime) Draw() error { return r.call(CallbackDraw) }

// call runs one of the program's callbacks under protection, so that an error
// in it stops the game with a message rather than taking the process down.
func (r *Runtime) call(name string) error {
	fn := r.L.GetGlobal(name)
	if fn == lua.LNil {
		return nil
	}
	return r.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true})
}

// Quitting reports whether the program called exit(), and with what status.
func (r *Runtime) Quitting() (bool, int) { return r.quit, r.quitCode }

// Window reports what the program has asked its window to be, and whether it
// has asked for anything since the last time this was called.
func (r *Runtime) Window() (Window, bool) {
	changed := r.windowChanged
	r.windowChanged = false
	return r.window, changed
}

/* --- reading arguments --- */

// coordLimit is as far off screen as a coordinate is allowed to be. A program
// working in world coordinates can hand over something absurd, and the drawing
// code multiplies coordinates together while clipping; keeping them well inside
// the range of an int is what stops that arithmetic wrapping round.
const coordLimit = 1 << 24

// toCoord turns a Lua number into a pixel coordinate. It rounds down, as the
// consoles' fixed point arithmetic did, so that -0.5 is the pixel left of zero
// rather than zero itself.
func toCoord(v float64) int {
	switch {
	case math.IsNaN(v):
		return 0
	case v > coordLimit:
		return coordLimit
	case v < -coordLimit:
		return -coordLimit
	}
	return int(math.Floor(v))
}

func coord(L *lua.LState, n int) int { return toCoord(float64(L.CheckNumber(n))) }

func optCoord(L *lua.LState, n, def int) int {
	if isNone(L, n) {
		return def
	}
	return coord(L, n)
}

// toColorIndex wraps a number into the palette, so that colour 70 is colour 6
// rather than an error or a crash.
func toColorIndex(v float64) uint8 {
	if math.IsNaN(v) {
		return 0
	}
	return uint8(((int(math.Floor(v)) % pico.Colors) + pico.Colors) % pico.Colors)
}

// isNone reports whether an argument was left out or passed as nil.
func isNone(L *lua.LState, n int) bool {
	return L.GetTop() < n || L.Get(n) == lua.LNil
}

// penArg reads a colour argument for a drawing call. Passing one changes the
// pen, as it does on PICO-8, so that later calls without a colour follow it.
func (r *Runtime) penArg(L *lua.LState, n int) uint8 {
	if isNone(L, n) {
		return r.Vid.Pen()
	}
	col := toColorIndex(float64(L.CheckNumber(n)))
	r.Vid.Color(col)
	return col
}

// optColorIndex reads a colour that does not touch the pen, such as the one
// cls() clears to.
func optColorIndex(L *lua.LState, n int, def uint8) uint8 {
	if isNone(L, n) {
		return def
	}
	return toColorIndex(float64(L.CheckNumber(n)))
}

// register puts functions in the global table, which is where a program written
// for these consoles expects to find them.
func (r *Runtime) register(funcs map[string]lua.LGFunction) {
	for name, fn := range funcs {
		r.L.SetGlobal(name, r.L.NewFunction(fn))
	}
}

// tostr renders a value the way the console's print does: numbers without a
// trailing ".0", and everything else as Lua would describe it.
func tostr(v lua.LValue) string {
	switch v := v.(type) {
	case lua.LString:
		return string(v)
	case lua.LNumber:
		if float64(v) == math.Trunc(float64(v)) && math.Abs(float64(v)) < 1e15 {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", float64(v))
	case lua.LBool:
		if v {
			return "true"
		}
		return "false"
	case *lua.LNilType:
		return "nil"
	default:
		return v.String()
	}
}
