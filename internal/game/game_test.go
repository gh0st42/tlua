package game

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tlua/internal/pico"
)

func TestFitPicksAWholeNumberOfPixelsAndCentresIt(t *testing.T) {
	cases := []struct {
		name                   string
		screenW, screenH, w, h int
		want                   view
	}{
		{"exactly twice", 960, 540, 480, 270, view{2, 0, 0}},
		{"room for two and a bit", 1100, 600, 480, 270, view{2, 70, 30}},
		{"wide window, bars at the sides", 1920, 540, 480, 270, view{2, 480, 0}},
		{"one to one", 480, 270, 480, 270, view{1, 0, 0}},
		{"smaller than the console", 200, 100, 480, 270, view{1, -140, -85}},
		{"nothing to draw", 800, 600, 0, 0, view{1, 0, 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := fit(c.screenW, c.screenH, c.w, c.h); got != c.want {
				t.Errorf("fit(%d,%d,%d,%d) = %+v, want %+v",
					c.screenW, c.screenH, c.w, c.h, got, c.want)
			}
		})
	}
}

func TestAPointerIsTurnedBackIntoAConsolePixel(t *testing.T) {
	// The program is told where the mouse is in its own pixels, whatever size
	// the window is; a pointer off the left of the picture has to read as a
	// negative number rather than as zero, or a program watching for the edge
	// never sees it leave.
	v := view{scale: 3, x: 30, y: 12}
	cases := []struct{ wx, wy, cx, cy int }{
		{30, 12, 0, 0},
		{32, 14, 0, 0},
		{33, 15, 1, 1},
		{29, 11, -1, -1},
		{0, 0, -10, -4},
		{330, 312, 100, 100},
	}
	for _, c := range cases {
		x, y := v.consolePixel(c.wx, c.wy)
		if x != c.cx || y != c.cy {
			t.Errorf("window %d,%d became %d,%d, want %d,%d", c.wx, c.wy, x, y, c.cx, c.cy)
		}
	}
}

func TestTheFirstWindowFitsTheMonitor(t *testing.T) {
	cases := []struct {
		name               string
		monitorW, monitorH int
		want               int
	}{
		{"a small laptop", 1440, 900, 2},
		{"a 1080p screen", 1920, 1080, 3},
		{"a big screen", 3840, 2160, 6}, // capped, not eight
		{"something tiny", 640, 480, 1},
		{"nothing known", 0, 0, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := startScale(c.monitorW, c.monitorH, pico.ScreenWidth, pico.ScreenHeight)
			if got != c.want {
				t.Errorf("startScale = %d, want %d", got, c.want)
			}
		})
	}
}

func TestParseArgs(t *testing.T) {
	cases := []struct {
		args   []string
		want   Options
		errors bool
	}{
		{args: nil, want: Options{ArgIdx: 2}},
		{args: []string{"game.lua"}, want: Options{Script: "game.lua", ArgIdx: 2}},
		{
			args: []string{"-scale", "4", "-fullscreen", "game.lua", "hard"},
			// os.Args is [tlua play -scale 4 -fullscreen game.lua hard], so the
			// script the arg table counts from sits at five.
			want: Options{Script: "game.lua", Args: []string{"hard"}, Scale: 4, Fullscreen: true, ArgIdx: 5},
		},
		{
			args: []string{"-scale=3", "-title=Snake", "game.lua"},
			want: Options{Script: "game.lua", Scale: 3, Title: "Snake", ArgIdx: 4},
		},
		{
			// A script whose name starts with a dash is still a script after --.
			args: []string{"--", "-odd.lua"},
			want: Options{Script: "-odd.lua", ArgIdx: 3},
		},
		{args: []string{"-scale", "0"}, errors: true},
		{args: []string{"-scale", "big"}, errors: true},
		{args: []string{"-scale"}, errors: true},
		{args: []string{"-wat"}, errors: true},
	}

	for _, c := range cases {
		got, err := parseArgs(c.args)
		if c.errors {
			if err == nil {
				t.Errorf("%v should have been refused", c.args)
			}
			continue
		}
		if err != nil {
			t.Errorf("%v: %v", c.args, err)
			continue
		}
		if got.Script != c.want.Script || got.Scale != c.want.Scale ||
			got.Fullscreen != c.want.Fullscreen || got.Title != c.want.Title ||
			got.ArgIdx != c.want.ArgIdx || strings.Join(got.Args, ",") != strings.Join(c.want.Args, ",") {
			t.Errorf("%v gave %+v, want %+v", c.args, got, c.want)
		}
	}
}

func TestHelpIsAskedForRatherThanRun(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		opts, err := parseArgs([]string{flag})
		if err != nil || !opts.help {
			t.Errorf("%s should ask for help (err %v)", flag, err)
		}
	}
}

func TestScriptPathFindsMainLuaInADirectory(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, EntryName)
	if err := os.WriteFile(entry, []byte("-- nothing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other.lua")
	if err := os.WriteFile(other, []byte("-- nothing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, err := scriptPath(dir); err != nil || got != entry {
		t.Errorf("scriptPath(dir) = %q, %v; want %q", got, err, entry)
	}
	if got, err := scriptPath(other); err != nil || got != other {
		t.Errorf("scriptPath(file) = %q, %v; want %q", got, err, other)
	}

	empty := t.TempDir()
	if _, err := scriptPath(empty); err == nil {
		t.Error("a directory with no main.lua should be refused")
	} else if !strings.Contains(err.Error(), EntryName) {
		t.Errorf("the error should name %s: %v", EntryName, err)
	}
	if _, err := scriptPath(filepath.Join(dir, "nothing.lua")); err == nil {
		t.Error("a file that is not there should be refused")
	}
}

func TestTheWindowIsNamedAfterTheProgram(t *testing.T) {
	cases := map[string]string{
		filepath.Join("games", "snake", "main.lua"): "snake",
		filepath.Join("games", "snake.lua"):         "snake",
		"main.lua":                                  "main",
	}
	for script, want := range cases {
		if got := titleFor(script); got != want {
			t.Errorf("titleFor(%q) = %q, want %q", script, got, want)
		}
	}
}

func TestWhereASaveGoesAndWhatItMayBeCalled(t *testing.T) {
	// The host is the only part of this that touches the disk, so it is the
	// part that has to refuse a name that would write somewhere else. The
	// names come from a program, which may have taken them from whoever is
	// playing.
	useATempHome(t)

	read, write := savesFor("my game")
	if read == nil || write == nil {
		t.Skip("this machine has nowhere to put application data")
	}

	if err := write("scores.txt", []byte("hello")); err != nil {
		t.Fatalf("saving: %v", err)
	}
	back, err := read("scores.txt")
	if err != nil || string(back) != "hello" {
		t.Errorf("reading back gave %q, %v", back, err)
	}
	if _, err := read("never-written.txt"); err == nil {
		t.Error("a file that was never saved should not read")
	}

	for _, name := range []string{"../escape", "dir/name", "/etc/passwd", "", ".", ".."} {
		if err := write(name, []byte("x")); err == nil {
			t.Errorf("%q was accepted as a name to save under", name)
		}
		if _, err := read(name); err == nil {
			t.Errorf("%q was accepted as a name to read", name)
		}
	}
}

func TestAProgramsNameBecomesAFolderName(t *testing.T) {
	cases := map[string]string{
		"cellar":        "cellar",
		"My Game":       "My-Game",
		"../../escape":  "escape",
		"a/b":           "a-b",
		"":              "program",
		"...":           "program",
		"game.lua":      "game.lua",
		"tlua's finest": "tlua-s-finest",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}
