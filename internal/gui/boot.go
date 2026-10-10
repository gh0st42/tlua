// Package gui is the desktop GUI module for tlua: forms and controls written
// in Lua, put on screen with FLTK.
package gui

import (
	"fmt"
	"os"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"tlua/internal/interp"
)

// A script can use the gui module the way it uses any other: require it,
// build a form and call form:show(), which returns when the form is closed.
// That works in a plain script, with -e and at the prompt.
//
// bootgui() is how a program says it is a GUI application instead, the way
// boot() says it is a game. From then on form:show() puts a form up and
// returns at once, so a program can show several, and the event loop runs
// once the file has finished — for as long as any form is open. That is what
// lets the handlers be written after the call to show(), below it in the file.

// Boot is a program that may say bootgui() while it runs.
type Boot struct {
	app  *app
	want bool
	late bool
}

// Ready installs the gui module and bootgui() into the interpreter before
// the script runs. read is where a fused program's own files come from, its
// archive before the disk; nil for a program on disk.
func Ready(in *interp.Interp, read func(string) ([]byte, error)) *Boot {
	b := &Boot{app: Open(in.L)}
	b.app.read = read
	// TLUA_SCHEME is the user's choice of look, over the program's.
	if name := os.Getenv("TLUA_SCHEME"); name != "" && !in.NoEnv() {
		if IsScheme(name) {
			b.app.userScheme = name
		} else {
			fmt.Fprintf(os.Stderr, "tlua: TLUA_SCHEME=%s is not a scheme; it is one of %s\n", name, strings.Join(Schemes, ", "))
		}
	}
	in.L.SetGlobal("bootgui", in.L.NewFunction(func(L *lua.LState) int {
		if b.late {
			L.RaiseError("bootgui: too late to ask for a window; a program asks while it is running, not after")
		}
		b.want = true
		b.app.booted = true
		// The module, so that `local gui = bootgui()` is all a program needs.
		L.Push(L.GetGlobal("require"))
		L.Push(lua.LString("gui"))
		L.Call(1, 1)
		return 1
	}))
	return b
}

// Wanted reports whether the script asked to be a GUI application.
func (b *Boot) Wanted() bool { return b != nil && b.want }

// TooLate says the script has finished, so bootgui() should no longer accept
// new requests.
func (b *Boot) TooLate() { b.late = true }

// Show runs the event loop until every form the program showed is closed,
// and returns the first error a handler raised. From then on form:show()
// waits for its form again, as it does in a program that never said bootgui.
func (b *Boot) Show() error {
	a := b.app
	a.loop(func() bool { return !a.anyShown() })
	a.booted = false
	return a.takeErr()
}
