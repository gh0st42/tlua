package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"tlua/internal/lsp"
)

var (
	buildOnce sync.Once
	tluaPath  string
	buildErr  error
)

// interpreter builds the tlua binary once, so F5 starts the real thing.
func interpreter(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "tlua-editor")
		if err != nil {
			buildErr = err
			return
		}
		tluaPath = filepath.Join(dir, "tlua")
		out, err := exec.Command("go", "build", "-o", tluaPath, "../../cmd/tlua").CombinedOutput()
		if err != nil {
			buildErr = err
			t.Logf("build output: %s", out)
		}
	})
	if buildErr != nil {
		t.Fatalf("building tlua: %v", buildErr)
	}
	return tluaPath
}

// start opens an editor on a simulated screen and runs it, the way a terminal
// would, so that key handling and the event loop are part of the test.
func start(t *testing.T, files ...string) (*Editor, tcell.SimulationScreen) {
	t.Helper()
	// Unless a test asked for a language server, do without one: whatever is
	// installed on the machine running the tests is none of their business.
	if os.Getenv(lsp.EnvServer) == "" {
		t.Setenv(lsp.EnvServer, "off")
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(100, 30)

	e, err := New(Config{Files: files, Interpreter: interpreter(t)})
	if err != nil {
		t.Fatal(err)
	}
	e.SetScreen(screen)

	done := make(chan error, 1)
	go func() { done <- e.Run() }()
	t.Cleanup(func() {
		e.app.Stop()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("editor exited with %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("editor did not stop")
		}
	})

	// Wait for the first draw, so the event loop is ready for keys.
	waitFor(t, e, "the editor to start", func() bool { return len(e.buffers) > 0 })
	return e, screen
}

// onEditor runs fn on the editor's own goroutine, where its state is safe to
// touch, and waits for it.
func onEditor(t *testing.T, e *Editor, fn func()) {
	t.Helper()
	done := make(chan struct{})
	e.app.QueueUpdate(func() {
		fn()
		close(done)
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the editor stopped responding")
	}
}

// redraw asks the editor to paint the screen and waits for it. Application.Draw
// queues the work itself, so it must never be called from inside a queued
// callback; this helper is the safe way in.
func redraw(t *testing.T, e *Editor) {
	t.Helper()
	done := make(chan struct{})
	e.app.QueueUpdateDraw(func() { close(done) })
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the editor did not redraw")
	}
}

// waitFor polls a condition on the editor's goroutine. Injected keys and queued
// updates arrive on different channels, so polling is what makes a test
// independent of which lands first.
func waitFor(t *testing.T, e *Editor, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		ok := false
		onEditor(t, e, func() { ok = cond() })
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// outlineList digs the list out of the function-list dialog, which also holds
// the colour legend.
func outlineList(t *testing.T, e *Editor) *clickList {
	t.Helper()
	frame, ok := e.modalStack[len(e.modalStack)-1].(*tview.Flex)
	if !ok {
		t.Fatalf("the function list is a %T", e.modalStack[len(e.modalStack)-1])
	}
	list, ok := frame.GetItem(0).(*clickList)
	if !ok {
		t.Fatalf("the first item is a %T", frame.GetItem(0))
	}
	return list
}

// menuIndex finds a menu by its title, so inserting a menu cannot break a test.
func menuIndex(t *testing.T, e *Editor, title string) int {
	t.Helper()
	index := -1
	onEditor(t, e, func() {
		for i, m := range e.menus {
			if m.title == title {
				index = i
			}
		}
	})
	if index < 0 {
		t.Fatalf("no %s menu", title)
	}
	return index
}

func press(screen tcell.SimulationScreen, key tcell.Key, r rune, mod tcell.ModMask) {
	screen.InjectKey(key, r, mod)
}

func typeText(screen tcell.SimulationScreen, text string) {
	for _, r := range text {
		if r == '\n' {
			screen.InjectKey(tcell.KeyEnter, '\r', tcell.ModNone)
			continue
		}
		screen.InjectKey(tcell.KeyRune, r, tcell.ModNone)
	}
}

func write(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpensEachFileInItsOwnBuffer(t *testing.T) {
	dir := t.TempDir()
	main := write(t, filepath.Join(dir, "main.lua"), "print('main')\n")
	lib := write(t, filepath.Join(dir, "lib.lua"), "return 1\n")

	e, _ := start(t, main, lib)
	onEditor(t, e, func() {
		if len(e.buffers) != 2 {
			t.Fatalf("got %d buffers, want 2", len(e.buffers))
		}
		if e.primary != e.buffers[0] {
			t.Errorf("primary should be the first file")
		}
		if got := e.buffers[0].area.GetText(); got != "print('main')\n" {
			t.Errorf("buffer text = %q", got)
		}
		if e.current != 0 {
			t.Errorf("current = %d, want the first buffer", e.current)
		}
	})
}

func TestSwitchingBuffers(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t,
		write(t, filepath.Join(dir, "a.lua"), "-- a\n"),
		write(t, filepath.Join(dir, "b.lua"), "-- b\n"),
		write(t, filepath.Join(dir, "c.lua"), "-- c\n"))

	press(screen, tcell.KeyF6, 0, tcell.ModNone)
	waitFor(t, e, "F6 to move to the next buffer", func() bool { return e.current == 1 })

	press(screen, tcell.KeyRune, '3', tcell.ModAlt)
	waitFor(t, e, "Alt-3 to select the third buffer", func() bool { return e.current == 2 })

	press(screen, tcell.KeyF6, 0, tcell.ModNone)
	waitFor(t, e, "F6 to wrap around", func() bool { return e.current == 0 })
}

func TestTypingAndSaving(t *testing.T) {
	dir := t.TempDir()
	path := write(t, filepath.Join(dir, "main.lua"), "")
	e, screen := start(t, path)

	typeText(screen, "print('typed')")
	waitFor(t, e, "the buffer to be modified", func() bool { return e.buffers[0].dirty })

	press(screen, tcell.KeyF2, 0, tcell.ModNone)
	waitFor(t, e, "F2 to save", func() bool { return !e.buffers[0].dirty })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "print('typed')\n" {
		t.Errorf("file = %q", data)
	}
}

func TestNewAndCloseBuffer(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "-- main\n"))

	press(screen, tcell.KeyCtrlN, 0, tcell.ModNone)
	waitFor(t, e, "Ctrl-N to add a buffer", func() bool { return len(e.buffers) == 2 })
	onEditor(t, e, func() {
		if got := e.buffers[1].name; got != "untitled1.lua" {
			t.Errorf("new buffer name = %q", got)
		}
		if e.primary != e.buffers[0] {
			t.Errorf("an unsaved buffer must not become the primary file")
		}
	})

	press(screen, tcell.KeyF3, 0, tcell.ModAlt) // Alt-F3 closes it
	waitFor(t, e, "Alt-F3 to close the buffer", func() bool { return len(e.buffers) == 1 })
}

func TestRunPrimaryFileWithF5(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "lib.lua"), "return { note = 'from the library' }\n")
	main := write(t, filepath.Join(dir, "main.lua"),
		"local lib = require('lib')\nprint('hello from F5', lib.note)\n")

	e, screen := start(t, main)
	press(screen, tcell.KeyF5, 0, tcell.ModNone)

	// Wait for the run to finish, not merely for its first line, or the rest
	// of these checks race the program.
	waitFor(t, e, "the program to finish", func() bool {
		return strings.Contains(e.output.GetText(true), "Program finished")
	})
	onEditor(t, e, func() {
		out := e.output.GetText(true)
		if !strings.Contains(out, "hello from F5") {
			t.Errorf("no program output: %q", out)
		}
		if !strings.Contains(out, "from the library") {
			t.Errorf("require() did not resolve: %q", out)
		}
		if !e.outputShown {
			t.Error("the output pane should be visible after a run")
		}
		if e.running != nil {
			t.Error("the program should have finished")
		}
	})
}

// F5 runs what is on screen: unsaved edits are written out first, including to
// modules the script requires.
func TestRunSavesModifiedBuffersFirst(t *testing.T) {
	dir := t.TempDir()
	lib := write(t, filepath.Join(dir, "lib.lua"), "return { note = 'old' }\n")
	main := write(t, filepath.Join(dir, "main.lua"), "print(require('lib').note)\n")

	e, screen := start(t, main, lib)
	onEditor(t, e, func() {
		e.buffers[1].area.SetText("return { note = 'edited in the editor' }\n", true)
	})
	waitFor(t, e, "the library buffer to be modified", func() bool { return e.buffers[1].dirty })

	press(screen, tcell.KeyF5, 0, tcell.ModNone)
	waitFor(t, e, "the program to print the edited value", func() bool {
		return strings.Contains(e.output.GetText(true), "edited in the editor")
	})
	onEditor(t, e, func() {
		if e.buffers[1].dirty {
			t.Error("the library buffer should have been saved by the run")
		}
	})
	data, err := os.ReadFile(lib)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "edited in the editor") {
		t.Errorf("lib.lua on disk = %q", data)
	}
}

func TestRunErrorPutsTheCursorOnTheLine(t *testing.T) {
	dir := t.TempDir()
	main := write(t, filepath.Join(dir, "main.lua"), "print('fine')\nerror('boom')\n")

	e, screen := start(t, main)
	press(screen, tcell.KeyF5, 0, tcell.ModNone)

	waitFor(t, e, "the failure to be reported", func() bool {
		return strings.Contains(e.output.GetText(true), "Program stopped")
	})
	onEditor(t, e, func() {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		if row != 1 { // zero-based: line 2
			t.Errorf("cursor on row %d, want the failing line 2", row+1)
		}
		if !strings.Contains(e.status, "line 2") || !strings.Contains(e.status, "boom") {
			t.Errorf("status = %q", e.status)
		}
	})
}

func TestCheckSyntaxWithF9(t *testing.T) {
	dir := t.TempDir()
	main := write(t, filepath.Join(dir, "main.lua"), "local x = 1\nprint(x)\n")
	e, screen := start(t, main)

	press(screen, tcell.KeyF9, 0, tcell.ModNone)
	waitFor(t, e, "the all-clear", func() bool { return strings.Contains(e.status, "no syntax errors") })

	onEditor(t, e, func() {
		e.buffers[0].area.SetText("local x = 1\nprint(x)\nif x then\n", true)
	})
	press(screen, tcell.KeyF9, 0, tcell.ModNone)
	waitFor(t, e, "the syntax error", func() bool {
		return strings.Contains(e.status, "main.lua") && !strings.Contains(e.status, "no syntax errors")
	})
}

func TestMenuOpensAndCloses(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "-- main\n"))

	press(screen, tcell.KeyF10, 0, tcell.ModNone)
	waitFor(t, e, "F10 to open the first menu", func() bool { return e.openMenu == 0 })

	press(screen, tcell.KeyRight, 0, tcell.ModNone)
	waitFor(t, e, "the next menu", func() bool { return e.openMenu == 1 })

	press(screen, tcell.KeyEscape, 0, tcell.ModNone)
	waitFor(t, e, "Escape to close the menu", func() bool { return e.openMenu == -1 })

	run := menuIndex(t, e, "Run")
	press(screen, tcell.KeyRune, 'r', tcell.ModAlt)
	waitFor(t, e, "Alt-R to open the Run menu", func() bool { return e.openMenu == run })
}

func TestOutputPaneToggles(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "-- main\n"))

	press(screen, tcell.KeyF4, 0, tcell.ModNone)
	waitFor(t, e, "F4 to show the output pane", func() bool { return e.outputShown })
	press(screen, tcell.KeyF4, 0, tcell.ModNone)
	waitFor(t, e, "F4 to hide it again", func() bool { return !e.outputShown })
}

func TestLeavingWithUnsavedChangesAsksFirst(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "-- main\n"))

	typeText(screen, "x")
	waitFor(t, e, "the buffer to be modified", func() bool { return e.buffers[0].dirty })

	press(screen, tcell.KeyRune, 'x', tcell.ModAlt) // Alt-X
	waitFor(t, e, "the confirmation dialog", func() bool { return e.modals == 1 })
	// The editor is still running: Cleanup would report an early exit.
}

// TestThemeUsesOnlyEgaColors guards the palette: every colour the editor hands
// to tview has to be one of the sixteen.
func TestThemeUsesOnlyEgaColors(t *testing.T) {
	applyTheme()
	palette := map[tcell.Color]string{
		egaBlack: "black", egaBlue: "blue", egaGreen: "green", egaCyan: "cyan",
		egaRed: "red", egaMagenta: "magenta", egaBrown: "brown", egaLightGray: "light gray",
		egaDarkGray: "dark gray", egaLightBlue: "light blue", egaLightGreen: "light green",
		egaLightCyan: "light cyan", egaLightRed: "light red", egaLightMagenta: "light magenta",
		egaYellow: "yellow", egaWhite: "white",
	}

	v := reflect.ValueOf(tview.Styles)
	for i := 0; i < v.NumField(); i++ {
		color, ok := v.Field(i).Interface().(tcell.Color)
		if !ok {
			continue
		}
		if _, inPalette := palette[color]; !inPalette {
			t.Errorf("%s is %v, which is outside the EGA palette",
				v.Type().Field(i).Name, color)
		}
	}
}

func TestStoppingARunawayProgram(t *testing.T) {
	dir := t.TempDir()
	main := write(t, filepath.Join(dir, "main.lua"), "print('started')\nwhile true do end\n")

	e, screen := start(t, main)
	press(screen, tcell.KeyF5, 0, tcell.ModNone)
	waitFor(t, e, "the program to start", func() bool {
		return strings.Contains(e.output.GetText(true), "started")
	})

	press(screen, tcell.KeyCtrlC, 0, tcell.ModNone)
	waitFor(t, e, "Ctrl-C to stop the program", func() bool { return e.running == nil })
	onEditor(t, e, func() {
		if !strings.Contains(e.output.GetText(true), "Program stopped") {
			t.Errorf("output = %q", e.output.GetText(true))
		}
	})
}

func TestRunawayOutputIsCapped(t *testing.T) {
	dir := t.TempDir()
	main := write(t, filepath.Join(dir, "main.lua"),
		"for i = 1, 200000 do print(i, 'a line of output that takes up room') end\n")

	e, screen := start(t, main)
	press(screen, tcell.KeyF5, 0, tcell.ModNone)
	waitFor(t, e, "the output to fill up", func() bool { return e.outputFull })

	onEditor(t, e, func() {
		if size := len(e.output.GetText(true)); size > 2*outputLimit {
			t.Errorf("output pane holds %d bytes, want it capped near %d", size, outputLimit)
		}
		if !strings.Contains(e.output.GetText(true), "output truncated") {
			t.Error("no truncation notice")
		}
	})

	// The editor still answers the keyboard while the program runs on.
	press(screen, tcell.KeyCtrlC, 0, tcell.ModNone)
	waitFor(t, e, "Ctrl-C to stop the program", func() bool { return e.running == nil })
}

func TestFunctionListJumpsToTheDefinition(t *testing.T) {
	dir := t.TempDir()
	main := write(t, filepath.Join(dir, "main.lua"), strings.Join([]string{
		"local M = {}",            // 1
		"",                        // 2
		"function M.first()",      // 3
		"  return 1",              // 4
		"end",                     // 5
		"",                        // 6
		"local function second()", // 7
		"  return 2",              // 8
		"end",                     // 9
		"",                        // 10
		"function M:third()",      // 11
		"  return self",           // 12
		"end",                     // 13
		"",
	}, "\n"))

	e, screen := start(t, main)
	press(screen, tcell.KeyF2, 0, tcell.ModAlt)
	waitFor(t, e, "the function list", func() bool { return e.modals == 1 })

	// The first row is the file itself, so M:third is three rows down.
	press(screen, tcell.KeyDown, 0, tcell.ModNone)
	press(screen, tcell.KeyDown, 0, tcell.ModNone)
	press(screen, tcell.KeyDown, 0, tcell.ModNone)
	press(screen, tcell.KeyEnter, 0, tcell.ModNone)

	waitFor(t, e, "the cursor to reach M:third", func() bool {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		return row == 10 && e.modals == 0
	})
	onEditor(t, e, func() {
		if !strings.Contains(e.status, "line 11") {
			t.Errorf("status = %q", e.status)
		}
	})
}

// The list opens on the function the cursor is inside, so Alt-F2 answers
// "where am I" as well as "where do I want to go".
func TestFunctionListStartsAtTheCurrentFunction(t *testing.T) {
	dir := t.TempDir()
	main := write(t, filepath.Join(dir, "main.lua"),
		"function one()\nend\n\nfunction two()\n  local x = 1\nend\n")

	e, screen := start(t, main)
	onEditor(t, e, func() { e.gotoLine(e.buffers[0], 5) }) // inside "two"

	press(screen, tcell.KeyF2, 0, tcell.ModAlt)
	waitFor(t, e, "the function list", func() bool { return e.modals == 1 })
	onEditor(t, e, func() {
		list := outlineList(t, e)
		// Rows are: the file, one(), two(), end of file. The cursor sits in
		// two(), so that is what opens selected.
		if got := list.GetCurrentItem(); got != 2 {
			t.Errorf("selected entry %d, want two()", got)
		}
	})
}

// The list navigates the file as well as its functions: the first row is the
// file itself and the last the end of it.
func TestFunctionListJumpsToTopAndEndOfFile(t *testing.T) {
	dir := t.TempDir()
	main := write(t, filepath.Join(dir, "main.lua"),
		"local M = {}\n\nfunction M.only()\n  return 1\nend\n\nreturn M\n")

	e, screen := start(t, main)
	onEditor(t, e, func() { e.gotoLine(e.buffers[0], 4) })

	// The last row jumps to the end.
	press(screen, tcell.KeyF2, 0, tcell.ModAlt)
	waitFor(t, e, "the function list", func() bool { return e.modals == 1 })
	onEditor(t, e, func() {
		list := outlineList(t, e)
		list.SetCurrentItem(list.GetItemCount() - 1)
	})
	press(screen, tcell.KeyEnter, 0, tcell.ModNone)
	waitFor(t, e, "the jump to the end of the file", func() bool {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		return e.modals == 0 && row == 7
	})

	// And the first row back to the top.
	press(screen, tcell.KeyF2, 0, tcell.ModAlt)
	waitFor(t, e, "the function list", func() bool { return e.modals == 1 })
	onEditor(t, e, func() { outlineList(t, e).SetCurrentItem(0) })
	press(screen, tcell.KeyEnter, 0, tcell.ModNone)
	waitFor(t, e, "the jump to the top of the file", func() bool {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		return e.modals == 0 && row == 0
	})
}
