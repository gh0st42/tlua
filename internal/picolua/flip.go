package picolua

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"tlua/internal/pico"
)

// The program runs inside one coroutine, for the whole of its life, driven by
// the loop below. That is what makes flip() possible: it suspends the program
// wherever it has got to — halfway through _update, inside a dialog three calls
// deep — and the host picks it up again on the next tick.
//
// Without it the only way back to the host would be to return, so a program
// could not run a loop of its own: no modal dialog that stays put until it is
// answered, no cutscene, no "press any key". Those are the shapes this is for.
//
// One coroutine rather than one a tick matters: a fresh one costs a hundred
// kilobytes and ten microseconds, and this costs neither.
const driverSource = `
local main, args = ...
if main then main(unpack(args)) end
if _init then _init() end
__ready()
while true do
	if _update then _update() end
	if _draw then _draw() end
	__ready()
end`

// The two names the driver needs, which are not part of the API a program is
// written against.
const (
	driverName = "=(the loop)"
	readyName  = "__ready"
)

// installFlip puts flip() in place, and the yield the driver ends a tick with.
func (r *Runtime) installFlip() {
	yield := r.L.NewFunction(func(L *lua.LState) int {
		// Yielding anything other than the program's own coroutine would hand
		// control to whoever resumed that one, which is not the host. Better
		// to say so than to behave strangely.
		if L != r.co {
			L.RaiseError("flip: this is inside a coroutine of your own, " +
				"which the loop cannot be suspended from")
		}
		return L.Yield()
	})

	// flip() shows what has been drawn and waits for the next tick. The
	// program carries on from there, with a tick's worth of fresh input.
	r.L.SetGlobal("flip", yield)
	r.L.SetGlobal(readyName, yield)
}

// Start runs the program: its main chunk, and then _init. Both happen inside
// the coroutine, so either may take the loop over with flip().
//
// It reports when the program has settled — either finished starting up, or
// suspended in a loop of its own — and everything after that is Tick.
//
// A nil chunk starts at _init instead. That is for a program whose main chunk
// has already run as an ordinary script and asked for a window part way
// through with boot(): there is no chunk left to run, only the callbacks it
// left behind.
func (r *Runtime) Start(chunk *lua.LFunction, args []string) error {
	driver, err := r.L.Load(strings.NewReader(driverSource), driverName)
	if err != nil {
		return fmt.Errorf("the loop will not compile, which cannot happen: %w", err)
	}

	list := r.L.NewTable()
	for _, a := range args {
		list.Append(lua.LString(a))
	}

	var main lua.LValue = lua.LNil
	if chunk != nil {
		main = chunk
	}

	r.co, _ = r.L.NewThread()
	return r.settle(r.L.Resume(r.co, driver, main, list))
}

// Tick takes in a frame of input and runs the program until it next gives the
// loop back — the end of its _update and _draw, or a flip() somewhere inside
// them.
func (r *Runtime) Tick(f pico.Frame) error {
	r.In.Update(f)
	defer func() { r.frame++ }()

	if r.co == nil || r.done {
		return nil
	}
	return r.settle(r.L.Resume(r.co, nil))
}

// settle takes in what a resume reported.
func (r *Runtime) settle(state lua.ResumeState, err error, _ []lua.LValue) error {
	switch {
	case err != nil:
		r.done = true
		return err
	case state != lua.ResumeYield:
		// The loop only ends by erroring, so this is a program that stopped
		// itself: nothing more to run, and nothing wrong.
		r.done = true
	}
	return nil
}

// Running reports whether there is any more of the program to run.
func (r *Runtime) Running() bool { return r.co != nil && !r.done }
