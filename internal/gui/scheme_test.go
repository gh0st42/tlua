package gui

import (
	"testing"

	"tlua/internal/interp"
)

// gui.scheme picks a look; TLUA_SCHEME, the user's, wins over it, and -E
// leaves the variable alone. Nothing here shows a window, so FLTK is only
// told when a form is.
func TestScheme(t *testing.T) {
	run := func(env string, noEnv bool, script string) {
		t.Helper()
		t.Setenv("TLUA_SCHEME", env)
		in := interp.New(&interp.Options{NoEnv: noEnv})
		defer in.Close()
		Ready(in, nil)
		if err := in.L.DoString("gui = require 'gui'\n" + script); err != nil {
			t.Errorf("TLUA_SCHEME=%q: %v", env, err)
		}
	}
	run("", false, `
		assert(gui.scheme() == "base")
		assert(gui.scheme("oxy") == "oxy" and gui.scheme() == "oxy")
		local ok, err = pcall(gui.scheme, "fancy")
		assert(not ok and err:find("gleam"))`)
	run("gleam", false, `
		assert(gui.scheme() == "gleam")
		assert(gui.scheme("oxy") == "gleam", "the user's wins")`)
	run("gleam", true, `assert(gui.scheme() == "base", "-E ignores it")`)
	run("nonsense", false, `assert(gui.scheme() == "base", "an unknown one is ignored")`)
}
