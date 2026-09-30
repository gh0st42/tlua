package game

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tlua/internal/interp"
	"tlua/internal/pico"
)

// script writes a program to a temp file and runs it the way `tlua <file>`
// does: as an ordinary script, with boot() available and nothing else of the
// console until it asks.
func script(t *testing.T, src string) (*Boot, *interp.Interp, error) {
	t.Helper()
	useATempHome(t)

	path := filepath.Join(t.TempDir(), "main.lua")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	in := interp.New(&interp.Options{Script: path})
	t.Cleanup(in.Close)

	b := Ready(in, Options{Script: path}, nil)
	return b, in, in.DoScript(path, nil)
}

func TestAScriptThatAsksForAWindowGetsTheConsole(t *testing.T) {
	b, in, err := script(t, `
		boot{ title = "asked for", scale = 2 }

		width, height = screen()
		drawn = 0
		function _init() started = true end
		function _update() drawn = drawn + 1 end
		function _draw() cls(3) end`)
	if err != nil {
		t.Fatal(err)
	}

	if !b.Wanted() {
		t.Fatal("the program asked for a window and was not heard")
	}
	// The console arrived while the program was still running, which is what
	// lets the rest of the file use it.
	if got := in.L.GetGlobal("width").String(); got != "480" {
		t.Errorf("screen() gave %q", got)
	}
	// _init has not run yet: the window opens after the file finishes.
	if got := in.L.GetGlobal("started").String(); got != "nil" {
		t.Errorf("_init ran too early: %q", got)
	}
	if win, _ := b.s.rt.Window(); win.Title != "asked for" || win.Scale != 2 {
		t.Errorf("the window was asked to be %+v", win)
	}

	// Everything Show does except opening the window: start at _init, then
	// run frames.
	if err := b.s.rt.Start(nil, nil); err != nil {
		t.Fatalf("starting: %v", err)
	}
	if got := in.L.GetGlobal("started").String(); got != "true" {
		t.Errorf("_init did not run: %q", got)
	}
	for i := 0; i < 3; i++ {
		if err := b.s.rt.Tick(pico.Frame{}); err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
	}
	if got := in.L.GetGlobal("drawn").String(); got != "3" {
		t.Errorf("_update ran %q times, want 3", got)
	}
	if got := b.s.rt.Screen().Get(0, 0); got != 3 {
		t.Errorf("_draw left colour %d on the screen, want 3", got)
	}
}

func TestAnOrdinaryScriptIsLeftAlone(t *testing.T) {
	// The console is not installed unless it is asked for. print() in
	// particular has to keep writing to the terminal, which is what every
	// script that is not a game expects of it.
	b, in, err := script(t, `
		kind = type(print)
		console = tostring(cls)`)
	if err != nil {
		t.Fatal(err)
	}
	if b.Wanted() {
		t.Error("a script that never called boot() asked for a window")
	}
	if got := in.L.GetGlobal("kind").String(); got != "function" {
		t.Errorf("print is %q", got)
	}
	if got := in.L.GetGlobal("console").String(); got != "nil" {
		t.Errorf("the console API was installed anyway: cls is %q", got)
	}
}

func TestBootingTwiceIsFine(t *testing.T) {
	// The second call is the console's own, since installing it takes the name
	// over. Both apply their window options, so neither is a surprise.
	b, _, err := script(t, `
		boot{ title = "first" }
		boot{ title = "second", fullscreen = true }`)
	if err != nil {
		t.Fatal(err)
	}
	win, _ := b.s.rt.Window()
	if win.Title != "second" || !win.Fullscreen {
		t.Errorf("the window ended up %+v", win)
	}
}

func TestAskingTooLate(t *testing.T) {
	// After the program has finished there is nothing to open a window in
	// front of, and a call that quietly did nothing would be worse than one
	// that says so.
	b, in, err := script(t, `nothing = true`)
	if err != nil {
		t.Fatal(err)
	}
	b.TooLate()

	err = in.DoString(`boot()`, "=(later)")
	if err == nil {
		t.Fatal("booting after the program finished was allowed")
	}
	if !strings.Contains(err.Error(), "too late") {
		t.Errorf("it said %q", err)
	}
}

func TestAProgramInAFolderBootsAsItsFolder(t *testing.T) {
	// `tlua play dir/` and `tlua dir/main.lua` should name the window the same
	// thing, since it is the same program.
	useATempHome(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, EntryName), []byte("boot()"), 0o644); err != nil {
		t.Fatal(err)
	}

	in := interp.New(&interp.Options{Script: dir})
	t.Cleanup(in.Close)
	b := Ready(in, Options{Script: dir}, nil)
	if err := in.DoScript(filepath.Join(dir, EntryName), nil); err != nil {
		t.Fatal(err)
	}

	win, _ := b.s.rt.Window()
	if win.Title != filepath.Base(dir) {
		t.Errorf("the window is called %q, want %q", win.Title, filepath.Base(dir))
	}
}
