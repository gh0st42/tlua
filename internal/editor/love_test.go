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

func TestRunWithLoveRunsTheFolder(t *testing.T) {
	t.Setenv(EnvLove, fakeLove(t))
	dir := loveProject(t)

	e, screen := start(t, filepath.Join(dir, "main.lua"))
	onEditor(t, e, func() {
		e.toggleRunMode()
		if e.runMode != runWithLove {
			t.Fatal("the toggle did not turn on")
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
		e.toggleRunMode()
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
	onEditor(t, e, e.toggleRunMode)

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
		e.toggleRunMode()
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

// The menu says which way F5 will go.
func TestRunMenuShowsTheMode(t *testing.T) {
	t.Setenv(EnvLove, fakeLove(t))
	dir := loveProject(t)
	e, screen := start(t, filepath.Join(dir, "main.lua"))

	run := menuIndex(t, e, "Run")
	press(screen, tcell.KeyRune, 'r', tcell.ModAlt)
	waitFor(t, e, "the Run menu", func() bool { return e.openMenu == run })
	redraw(t, e)
	if words := screenWords(screen); !strings.Contains(words, "Run with LOVE: off") {
		t.Errorf("the menu does not show the mode: %s", words)
	}
	press(screen, tcell.KeyEscape, 0, tcell.ModNone)
	waitFor(t, e, "the menu to close", func() bool { return e.openMenu == -1 })

	onEditor(t, e, e.toggleRunMode)
	press(screen, tcell.KeyRune, 'r', tcell.ModAlt)
	waitFor(t, e, "the Run menu again", func() bool { return e.openMenu == run })
	redraw(t, e)
	if words := screenWords(screen); !strings.Contains(words, "Run with LOVE: on") {
		t.Errorf("the menu does not show the mode: %s", words)
	}
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
		{"Run", "r", "Run with LOVE: off"},
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
