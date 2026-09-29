package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// fakeLove is a stand-in for love2d: it prints the arguments it was given and
// the directory it was run in, which is all a test needs to see.
func fakeLove(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "love")
	script := "#!/bin/sh\necho \"love ran with [$*] in $(pwd)\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// A LÖVE project: main.lua, and a module beside it.
func loveProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "main.lua"), "function love.draw() end\n")
	write(t, filepath.Join(dir, "player.lua"), "return {}\n")
	return dir
}

// chooseRunMode cycles round to the mode a test wants, as a person would from
// the Run menu.
func chooseRunMode(e *Editor, want runMode) {
	for i := 0; i < int(runModes) && e.runMode != want; i++ {
		e.cycleRunMode()
	}
}

func TestRunWithLoveRunsTheFolder(t *testing.T) {
	t.Setenv(EnvLove, fakeLove(t))
	dir := loveProject(t)

	e, screen := start(t, filepath.Join(dir, "main.lua"))
	onEditor(t, e, func() {
		chooseRunMode(e, runWithLove)
		if e.runMode != runWithLove {
			t.Fatal("the setting did not come round to love")
		}
	})

	press(screen, tcell.KeyF5, 0, tcell.ModNone)
	waitFor(t, e, "love to run", func() bool {
		return strings.Contains(e.output.GetText(true), "love ran with")
	})

	onEditor(t, e, func() {
		out := e.output.GetText(true)
		if !strings.Contains(out, "love ran with [.]") {
			t.Errorf("love was given the wrong arguments: %q", out)
		}
		// It ran in the folder holding main.lua.
		if !ranIn(out, t, dir) {
			t.Errorf("love ran somewhere else: %q, want %s", out, dir)
		}
		if !strings.Contains(out, "Running love . in") {
			t.Errorf("the output does not say what it ran: %q", out)
		}
	})
}

// With the toggle off, F5 runs the file through the interpreter as before.
func TestRunWithTluaIsTheDefault(t *testing.T) {
	t.Setenv(EnvLove, fakeLove(t))
	dir := loveProject(t)
	write(t, filepath.Join(dir, "main.lua"), "print('through tlua')\n")

	e, screen := start(t, filepath.Join(dir, "main.lua"))
	onEditor(t, e, func() {
		if e.runMode != runWithTlua {
			t.Fatal("LOVE mode is on to begin with")
		}
	})

	press(screen, tcell.KeyF5, 0, tcell.ModNone)
	waitFor(t, e, "the program to run", func() bool {
		return strings.Contains(e.output.GetText(true), "through tlua")
	})
	onEditor(t, e, func() {
		if strings.Contains(e.output.GetText(true), "love ran") {
			t.Error("love was run with the toggle off")
		}
	})
}

// Running any buffer in the project hands love the same folder, since the file
// being edited need not be main.lua.
func TestRunWithLoveFromAnotherFileInTheProject(t *testing.T) {
	t.Setenv(EnvLove, fakeLove(t))
	dir := loveProject(t)

	e, screen := start(t, filepath.Join(dir, "player.lua"))
	onEditor(t, e, func() {
		chooseRunMode(e, runWithLove)
		e.setPrimary()
	})

	press(screen, tcell.KeyF5, 0, tcell.ModNone)
	waitFor(t, e, "love to run", func() bool {
		return strings.Contains(e.output.GetText(true), "love ran with [.]")
	})
	onEditor(t, e, func() {
		if !ranIn(e.output.GetText(true), t, dir) {
			t.Errorf("love ran in the wrong folder: %q", e.output.GetText(true))
		}
	})
}

// A folder without main.lua is not a LÖVE game, and saying so is more use than
// love's own complaint.
func TestRunWithLoveWithoutAMainFile(t *testing.T) {
	t.Setenv(EnvLove, fakeLove(t))
	dir := t.TempDir()

	e, screen := start(t, write(t, filepath.Join(dir, "script.lua"), "print('x')\n"))
	onEditor(t, e, func() { chooseRunMode(e, runWithLove) })

	press(screen, tcell.KeyF5, 0, tcell.ModNone)
	waitFor(t, e, "the explanation", func() bool { return e.modals == 1 })
	redraw(t, e)
	if words := screenWords(screen); !strings.Contains(words, "has no main.lua") {
		t.Errorf("the dialog does not explain itself: %s", words)
	}
}

func TestRunWithLoveWhenLoveIsNotInstalled(t *testing.T) {
	t.Setenv(EnvLove, "definitely-not-installed-love")
	dir := loveProject(t)

	e, screen := start(t, filepath.Join(dir, "main.lua"))
	onEditor(t, e, func() {
		chooseRunMode(e, runWithLove)
		if !strings.Contains(e.status, "was not found") {
			t.Errorf("the toggle did not say love is missing: %q", e.status)
		}
	})

	press(screen, tcell.KeyF5, 0, tcell.ModNone)
	waitFor(t, e, "the explanation", func() bool { return e.modals == 1 })
	redraw(t, e)
	if words := screenWords(screen); !strings.Contains(words, "love was not found") {
		t.Errorf("the dialog does not explain itself: %s", words)
	}
}

// The menu says which way F5 will go, and choosing it moves on to the next.
func TestRunMenuShowsTheMode(t *testing.T) {
	t.Setenv(EnvLove, fakeLove(t))
	dir := loveProject(t)
	e, screen := start(t, filepath.Join(dir, "main.lua"))
	run := menuIndex(t, e, "Run")

	for _, want := range []string{"Run with: tlua", "Run with: console window", "Run with: LOVE", "Run with: tlua"} {
		press(screen, tcell.KeyRune, 'r', tcell.ModAlt)
		waitFor(t, e, "the Run menu", func() bool { return e.openMenu == run })
		redraw(t, e)
		if words := screenWords(screen); !strings.Contains(words, want) {
			t.Fatalf("the menu does not say %q: %s", want, words)
		}
		press(screen, tcell.KeyEscape, 0, tcell.ModNone)
		waitFor(t, e, "the menu to close", func() bool { return e.openMenu == -1 })
		onEditor(t, e, e.cycleRunMode)
	}
}

// The console window is this same binary, run again with "play".
func TestRunAsAConsoleProgram(t *testing.T) {
	dir := t.TempDir()
	script := write(t, filepath.Join(dir, "game.lua"), "cls(3)\n")

	e, _ := start(t, script)
	onEditor(t, e, func() {
		chooseRunMode(e, runWithPico)
		cmd, what, err := e.runCommand(e.buf())
		if err != nil {
			t.Fatal(err)
		}
		if cmd.Path != e.exe {
			t.Errorf("it runs %q, want this binary %q", cmd.Path, e.exe)
		}
		if len(cmd.Args) < 2 || cmd.Args[1] != "play" {
			t.Errorf("arguments are %q, want play first", cmd.Args)
		}
		if cmd.Args[len(cmd.Args)-1] != "game.lua" {
			t.Errorf("arguments are %q, want the file last", cmd.Args)
		}
		if cmd.Dir != filepath.Dir(script) {
			t.Errorf("it runs in %q, want the file's own folder", cmd.Dir)
		}
		if !strings.Contains(what, "play") {
			t.Errorf("the output would say %q", what)
		}
	})
}

func TestFindLove(t *testing.T) {
	// The environment names it, and it has to be there.
	love := fakeLove(t)
	t.Setenv(EnvLove, love)
	command, args, ok := findLove()
	if !ok || command != love || len(args) != 0 {
		t.Errorf("findLove = %q, %v, %v", command, args, ok)
	}

	// With arguments.
	t.Setenv(EnvLove, love+" --console")
	command, args, ok = findLove()
	if !ok || command != love || len(args) != 1 || args[0] != "--console" {
		t.Errorf("findLove = %q, %v, %v", command, args, ok)
	}

	t.Setenv(EnvLove, "definitely-not-installed-love")
	if _, _, ok := findLove(); ok {
		t.Error("a binary that is not there was found")
	}
}

// ranIn reports whether the output says the program ran in dir. A temporary
// directory on macOS is reached through a symlink, and a shell reports the path
// it was given while Go reports the one it resolves to, so either will do.
func ranIn(output string, t *testing.T, dir string) bool {
	t.Helper()
	if strings.Contains(output, dir) {
		return true
	}
	real, err := filepath.EvalSymlinks(dir)
	return err == nil && strings.Contains(output, real)
}

// Every setting that shows its state in a menu has to show it: tview reads
// square brackets in a list item as a colour tag, which ate "[off]" once.
func TestSettingsShowTheirStateInTheMenus(t *testing.T) {
	withLanguageServer(t)
	dir := loveProject(t)
	e, screen := start(t, filepath.Join(dir, "main.lua"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil })

	menus := []struct {
		title, hotkey, want string
	}{
		{"Run", "r", "Run with: tlua"},
		{"Edit", "e", "Format on save: on"},
		{"Edit", "e", "Parameters on status line: off"},
	}
	for _, m := range menus {
		index := menuIndex(t, e, m.title)
		press(screen, tcell.KeyRune, rune(m.hotkey[0]), tcell.ModAlt)
		waitFor(t, e, "the "+m.title+" menu", func() bool { return e.openMenu == index })
		redraw(t, e)
		if words := screenWords(screen); !strings.Contains(words, m.want) {
			t.Errorf("the %s menu does not say %q: %s", m.title, m.want, words)
		}
		press(screen, tcell.KeyEscape, 0, tcell.ModNone)
		waitFor(t, e, "the menu to close", func() bool { return e.openMenu == -1 })
	}
}
