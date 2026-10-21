package gui

import (
	"testing"

	"tlua/internal/interp"
)

// gui.colors keeps what it is asked for and says it back; FLTK is only told
// when a form is shown, so nothing here opens a window.
func TestColors(t *testing.T) {
	in := interp.New(&interp.Options{})
	defer in.Close()
	Ready(in, nil)
	err := in.L.DoString(`
		gui = require "gui"
		local none = gui.colors()
		assert(next(none) == nil, "nothing asked for yet")
		local got = gui.colors { background = "#202020", foreground = "white" }
		assert(got.background == "#202020" and got.foreground == "#ffffff" and got.background2 == nil)
		-- Asking again adds to it, and leaves what is not named.
		got = gui.colors { background2 = "#141414", foreground = "#d0d0d0" }
		assert(got.background == "#202020" and got.background2 == "#141414" and got.foreground == "#d0d0d0")
		assert(gui.colors().background2 == "#141414")
		local ok, why = pcall(gui.colors, { backgroud = "#000000" })
		assert(not ok and why:find("background2") and why:find("backgroud"), why)
		ok, why = pcall(gui.colors, { background = "darkish" })
		assert(not ok and why:find("colour must be"), why)
		assert(gui.colors().background == "#202020", "a mistake changes nothing")
	`)
	if err != nil {
		t.Fatal(err)
	}
}
