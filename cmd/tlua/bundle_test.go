package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// bundleApp packs src into a bundle called name and returns its path.
func bundleApp(t *testing.T, src, name string, extra ...string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), name)
	args := append([]string{"bundle", "-o", out}, extra...)
	args = append(args, src)
	if got, err := exec.Command(bin(t), args...).CombinedOutput(); err != nil {
		t.Fatalf("bundle %v: %v\n%s", args, err, got)
	}
	return out
}

// A bundle runs the way the binary fused from it would: its modules, its
// files and its arguments, with arg[0] the bundle itself.
func TestRunABundle(t *testing.T) {
	src := sampleApp(t)
	for _, name := range []string{"app.ztl", "app.zip", "app.app"} {
		app := bundleApp(t, src, name)
		got := runCLI(t, t.TempDir(), "", app, "one", "two")
		if got.code != 0 {
			t.Fatalf("%s: exit %d: %s", name, got.code, got.stderr)
		}
		for _, want := range []string{
			"args:\tone\ttwo", "sum:\t10", "kind:\tzip",
			"asset:\thello from the archive", "levels:\tforest",
		} {
			if !strings.Contains(got.stdout, want) {
				t.Errorf("%s: output does not say %q:\n%s", name, want, got.stdout)
			}
		}
	}
}

func TestABundlesArgZeroIsTheBundle(t *testing.T) {
	src := filepath.Join(t.TempDir(), "main.lua")
	write(t, src, "print(arg[0], arg[1], select('#', ...))")
	app := bundleApp(t, src, "a.ztl")
	got := runCLI(t, t.TempDir(), "", "--", app, "x")
	if want := app + "\tx\t1"; strings.TrimSpace(got.stdout) != want {
		t.Errorf("got %q, want %q (stderr %s)", got.stdout, want, got.stderr)
	}
}

func TestABundleTakesNoOptionsAroundIt(t *testing.T) {
	src := filepath.Join(t.TempDir(), "main.lua")
	write(t, src, "print('ran')")
	app := bundleApp(t, src, "a.ztl")
	got := runCLI(t, t.TempDir(), "", "-e", "x=1", app)
	if got.code == 0 || !strings.Contains(got.stderr, "on its own") {
		t.Errorf("exit %d, stderr %q: -e with a bundle should be refused", got.code, got.stderr)
	}
}

// A file that is not named as a bundle is a script, whatever is in it.
func TestOnlyABundlesNameMakesItOne(t *testing.T) {
	src := filepath.Join(t.TempDir(), "main.lua")
	write(t, src, "print('ran')")
	data, err := os.ReadFile(bundleApp(t, src, "a.ztl"))
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "a.dat")
	if err := os.WriteFile(other, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := runCLI(t, t.TempDir(), "", other); got.code == 0 {
		t.Error("a zip named .dat should be read as a Lua script, and fail")
	}
}

const consoleGame = `
local w, h = screen()
printh("screen is " .. w .. "x" .. h)
function _init() exit(7) end
function _draw() cls() end
`

// A bundle made with -play opens a window, run either way.
func TestRunAGameBundle(t *testing.T) {
	src := filepath.Join(t.TempDir(), "game.lua")
	write(t, src, consoleGame)
	app := bundleApp(t, src, "game.ztl", "-play")
	for _, args := range [][]string{{app}, {"play", app}} {
		got := runCLI(t, t.TempDir(), "", args...)
		if got.code != 7 || !strings.Contains(got.stdout, "screen is 480x270") {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, got.code, got.stdout, got.stderr)
		}
	}
}

// A zip made by hand has no -play mark, and a program in it written for the
// console still opens its window: there is no command line to say so on.
func TestAHandMadeGameBundleOpensAWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.zip")
	writeZip(t, path, map[string]string{"game/main.lua": consoleGame})
	got := runCLI(t, t.TempDir(), "", path)
	if got.code != 7 || !strings.Contains(got.stdout, "screen is 480x270") {
		t.Errorf("exit %d, stdout %q, stderr %q", got.code, got.stdout, got.stderr)
	}
}

// A desktop application in a bundle reads its layout out of the bundle.
func TestRunAGUIBundle(t *testing.T) {
	if os.Getenv("TLUA_GUI_TESTS") == "" {
		t.Skip("set TLUA_GUI_TESTS=1 to run tests that open windows")
	}
	dir := t.TempDir()
	mkdirs(t, filepath.Join(dir, "forms"))
	write(t, filepath.Join(dir, "main.lua"), `
local gui = bootgui()
local frm = gui.load "forms/Main"
print("loaded", frm.Hello.caption)
frm:show()
gui.after(0.3, function() print("shown"); frm:close() end)
`)
	write(t, filepath.Join(dir, "forms", "Main.form.lua"), `return {
  kind = "Form", name = "Main", caption = "Bundled", width = 200, height = 100,
  { kind = "Label", name = "Hello", caption = "from the bundle", left = 10, top = 10, width = 180 },
}
`)
	app := bundleApp(t, dir, "notes.ztl")
	got := runCLI(t, t.TempDir(), "", app)
	if got.code != 0 || !strings.Contains(got.stdout, "loaded\tfrom the bundle") ||
		!strings.Contains(got.stdout, "shown") {
		t.Errorf("exit %d, stdout %q, stderr %q", got.code, got.stdout, got.stderr)
	}
}
