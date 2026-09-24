package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"

	"tlua/internal/lsp"
)

var (
	fakeOnce sync.Once
	fakeLSP  string
	fakeErr  error
)

// fakeServer builds the test language server that lives with the lsp package.
// It formats by trimming trailing whitespace and turning leading tabs into two
// spaces, which is enough to see that formatting happened.
func fakeServer(t *testing.T) string {
	t.Helper()
	fakeOnce.Do(func() {
		dir, err := os.MkdirTemp("", "editor-fakelsp")
		if err != nil {
			fakeErr = err
			return
		}
		fakeLSP = filepath.Join(dir, "fakelsp")
		out, err := exec.Command("go", "build", "-o", fakeLSP, "../lsp/testdata/fakelsp").CombinedOutput()
		if err != nil {
			fakeErr = err
			t.Logf("build output: %s", out)
		}
	})
	if fakeErr != nil {
		t.Fatalf("building the test language server: %v", fakeErr)
	}
	return fakeLSP
}

// withLanguageServer points the editor at the test server before it starts.
func withLanguageServer(t *testing.T, env ...string) {
	t.Helper()
	t.Setenv(lsp.EnvServer, fakeServer(t))
	for _, pair := range env {
		name, value, _ := strings.Cut(pair, "=")
		t.Setenv(name, value)
	}
}

func waitForServer(t *testing.T, e *Editor) {
	t.Helper()
	waitFor(t, e, "the language server to start", func() bool { return e.canFormat() })
}

// waitForSave waits for the write itself. A buffer that was never modified is
// already clean, so "not dirty" says nothing about whether the save has run.
func waitForSave(t *testing.T, e *Editor) {
	t.Helper()
	waitFor(t, e, "the save", func() bool { return strings.HasPrefix(e.status, "Saved ") })
}

const untidy = "local x = 1   \n\tprint(x)\t\n"
const tidy = "local x = 1\n  print(x)\n"

func TestFormatOnSaveUsesTheLanguageServer(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	path := write(t, filepath.Join(dir, "main.lua"), untidy)

	e, screen := start(t, path)
	waitForServer(t, e)

	press(screen, tcell.KeyF2, 0, tcell.ModNone)
	waitForSave(t, e)

	onEditor(t, e, func() {
		if got := e.buffers[0].area.GetText(); got != tidy {
			t.Errorf("buffer = %q, want it formatted", got)
		}
	})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != tidy {
		t.Errorf("file = %q, want it formatted", data)
	}
}

func TestFormatDocumentWithF12DoesNotSave(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	path := write(t, filepath.Join(dir, "main.lua"), untidy)

	e, screen := start(t, path)
	waitForServer(t, e)

	press(screen, tcell.KeyF12, 0, tcell.ModNone)
	waitFor(t, e, "the formatting", func() bool {
		return e.buffers[0].area.GetText() == tidy
	})
	onEditor(t, e, func() {
		if !e.buffers[0].dirty {
			t.Error("formatting should leave the buffer modified")
		}
		if !strings.Contains(e.status, "fakelsp") {
			t.Errorf("status = %q, want it to name the server", e.status)
		}
	})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != untidy {
		t.Errorf("the file changed without a save: %q", data)
	}
}

// One undo takes the whole formatting pass back.
func TestFormattingIsOneUndoStep(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), untidy))
	waitForServer(t, e)

	press(screen, tcell.KeyF12, 0, tcell.ModNone)
	waitFor(t, e, "the formatting", func() bool { return e.buffers[0].area.GetText() == tidy })

	press(screen, tcell.KeyCtrlZ, 0, tcell.ModNone)
	waitFor(t, e, "undo to put it back", func() bool { return e.buffers[0].area.GetText() == untidy })
}

// The cursor stays on the line it was on, rather than jumping to the top.
func TestFormattingKeepsTheCursorsLine(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"),
		"local a = 1   \nlocal b = 2   \nlocal c = 3   \n"))
	waitForServer(t, e)

	onEditor(t, e, func() { e.gotoLine(e.buffers[0], 3) })
	press(screen, tcell.KeyF12, 0, tcell.ModNone)
	waitFor(t, e, "the formatting", func() bool {
		return !strings.Contains(e.buffers[0].area.GetText(), "   ")
	})
	onEditor(t, e, func() {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		if row != 2 {
			t.Errorf("the cursor is on line %d, want line 3", row+1)
		}
	})
}

func TestFormatOnSaveCanBeTurnedOff(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	path := write(t, filepath.Join(dir, "main.lua"), untidy)

	e, screen := start(t, path)
	waitForServer(t, e)
	onEditor(t, e, func() {
		e.toggleFormatOnSave()
		if e.formatOnSave {
			t.Error("the setting did not turn off")
		}
	})

	press(screen, tcell.KeyF2, 0, tcell.ModNone)
	waitForSave(t, e)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != untidy {
		t.Errorf("file = %q, want it left alone", data)
	}
}

// A server that fails a request must not cost the user their save.
func TestSaveStillWritesWhenFormattingFails(t *testing.T) {
	withLanguageServer(t, "FAKELSP_ERROR=1")
	dir := t.TempDir()
	path := write(t, filepath.Join(dir, "main.lua"), untidy)

	e, screen := start(t, path)
	waitForServer(t, e)

	press(screen, tcell.KeyF2, 0, tcell.ModNone)
	waitForSave(t, e)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != untidy {
		t.Errorf("file = %q, want the text as it was", data)
	}
	onEditor(t, e, func() {
		if !strings.Contains(e.lspNote, "broken today") {
			t.Errorf("the failure was not noted: %q", e.lspNote)
		}
	})
}

// A server that offers no formatting is not an error either.
func TestServerWithoutFormattingLeavesSavesAlone(t *testing.T) {
	withLanguageServer(t, "FAKELSP_NOFORMAT=1")
	dir := t.TempDir()
	path := write(t, filepath.Join(dir, "main.lua"), untidy)

	e, screen := start(t, path)
	waitFor(t, e, "the server to start", func() bool { return e.lsp != nil })
	onEditor(t, e, func() {
		if e.canFormat() {
			t.Error("a server without formatting said it formats")
		}
	})

	press(screen, tcell.KeyF2, 0, tcell.ModNone)
	waitForSave(t, e)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != untidy {
		t.Errorf("file = %q", data)
	}
}

// Without a server the editor behaves as it always did.
func TestEditorWorksWithoutALanguageServer(t *testing.T) {
	t.Setenv(lsp.EnvServer, "off")
	dir := t.TempDir()
	path := write(t, filepath.Join(dir, "main.lua"), untidy)

	e, screen := start(t, path)
	press(screen, tcell.KeyF2, 0, tcell.ModNone)
	waitForSave(t, e)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != untidy {
		t.Errorf("file = %q", data)
	}

	press(screen, tcell.KeyF12, 0, tcell.ModNone)
	waitFor(t, e, "the explanation", func() bool {
		return strings.Contains(e.status, "No language server")
	})
}

// Running the primary file saves the buffers first, so F5 formats them too.
func TestRunFormatsWhatItSaves(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	path := write(t, filepath.Join(dir, "main.lua"), "print('ran')   \n")

	e, screen := start(t, path)
	waitForServer(t, e)
	onEditor(t, e, func() { e.buffers[0].area.Replace(0, 0, "  ") }) // make it dirty

	press(screen, tcell.KeyF5, 0, tcell.ModNone)
	waitFor(t, e, "the program to run", func() bool {
		return strings.Contains(e.output.GetText(true), "ran")
	})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "   \n") {
		t.Errorf("the saved file was not formatted: %q", data)
	}
}

func TestOffsetAt(t *testing.T) {
	text := "local x = 1\n\tprint(x)\nlast\n"
	cases := []struct {
		row, column, want int
		why               string
	}{
		{0, 0, 0, "the start"},
		{0, 6, 6, "part way along the first line"},
		{0, 99, 11, "past the end of a line stops at its end"},
		{1, 0, 12, "the start of the second line"},
		{1, 4, 13, "a tab is one tab stop wide, so column 4 is just after it"},
		{1, 6, 15, "two columns past the tab"},
		{2, 2, 24, "the third line"},
		{9, 0, len(text), "past the last line"},
	}
	for _, c := range cases {
		if got := offsetAt(text, c.row, c.column); got != c.want {
			t.Errorf("offsetAt(%d, %d) = %d, want %d (%s)", c.row, c.column, got, c.want, c.why)
		}
	}
}

// The editor tells the server about open buffers as soon as it starts, so the
// server is reading the workspace before the first question is asked rather
// than after it.
func TestOpenBuffersReachTheServerAtStartup(t *testing.T) {
	log := filepath.Join(t.TempDir(), "fakelsp.log")
	withLanguageServer(t, "FAKELSP_LOG="+log)

	dir := t.TempDir()
	first := write(t, filepath.Join(dir, "main.lua"), "print(1)\n")
	second := write(t, filepath.Join(dir, "lib.lua"), "return 1\n")

	e, _ := start(t, first, second)
	waitForServer(t, e)

	waitFor(t, e, "both buffers to reach the server", func() bool {
		data, err := os.ReadFile(log)
		if err != nil {
			return false
		}
		return strings.Contains(string(data), "main.lua") &&
			strings.Contains(string(data), "lib.lua")
	})
}
