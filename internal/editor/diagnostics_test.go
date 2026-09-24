package editor

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"tlua/internal/lsp"
)

// The test server calls "??" an error and "!!" a warning, wherever it finds them.
const troubled = "local a = 1\nlocal b = ??\nlocal c = 3 !!\nlocal d = 4\n"

func waitForDiagnostics(t *testing.T, e *Editor, want int) {
	t.Helper()
	waitFor(t, e, "the server's report", func() bool {
		return len(e.buffers[e.current].diagnostics) == want
	})
}

func TestDiagnosticsArriveForAnOpenFile(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, _ := start(t, write(t, filepath.Join(dir, "main.lua"), troubled))
	waitForDiagnostics(t, e, 2)

	onEditor(t, e, func() {
		list := e.buffers[0].diagnostics
		if list[0].Severity != lsp.SeverityError || list[0].Range.Start.Line != 1 {
			t.Errorf("first = %+v", list[0])
		}
		if list[1].Severity != lsp.SeverityWarning || list[1].Range.Start.Line != 2 {
			t.Errorf("second = %+v", list[1])
		}
		if errors, warnings := problemCount(list); errors != 1 || warnings != 1 {
			t.Errorf("counted %d errors and %d warnings", errors, warnings)
		}
	})
}

// The message for the line the cursor is on goes on the status line.
func TestDiagnosticOnTheStatusLine(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), troubled))
	waitForDiagnostics(t, e, 2)

	// Moving the cursor hands the status line to the diagnostics, whatever
	// message was on it before.
	onEditor(t, e, func() { e.gotoLine(e.buffers[0], 2) })
	redraw(t, e)

	last := dump(screen)[len(dump(screen))-1]
	if !strings.Contains(last, "Error: unexpected symbol") {
		t.Errorf("status line = %q", last)
	}

	// A clean line says nothing, so the key hints come back.
	onEditor(t, e, func() { e.gotoLine(e.buffers[0], 4) })
	redraw(t, e)
	last = dump(screen)[len(dump(screen))-1]
	if strings.Contains(last, "Error:") {
		t.Errorf("status line on a clean line = %q", last)
	}
	if !strings.Contains(last, "F1 Help") {
		t.Errorf("the key hints did not come back: %q", last)
	}

	// The warning line says so, and says which it is.
	onEditor(t, e, func() { e.gotoLine(e.buffers[0], 3) })
	redraw(t, e)
	last = dump(screen)[len(dump(screen))-1]
	if !strings.Contains(last, "Warning: this looks doubtful") {
		t.Errorf("status line = %q", last)
	}
}

// Alt-F8 and Alt-F7 walk the list, wrapping round.
func TestWalkingThroughProblems(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), troubled))
	waitForDiagnostics(t, e, 2)

	cursorLine := func() int {
		line := 0
		onEditor(t, e, func() {
			row, _, _, _ := e.buffers[0].area.GetCursor()
			line = row + 1
		})
		return line
	}

	press(screen, tcell.KeyF8, 0, tcell.ModAlt)
	waitFor(t, e, "the first problem", func() bool {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		return row == 1
	})
	onEditor(t, e, func() {
		if !strings.Contains(e.status, "1 of 2") {
			t.Errorf("status = %q", e.status)
		}
	})

	press(screen, tcell.KeyF8, 0, tcell.ModAlt)
	waitFor(t, e, "the second problem", func() bool {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		return row == 2
	})

	// Past the last one it comes round to the first.
	press(screen, tcell.KeyF8, 0, tcell.ModAlt)
	waitFor(t, e, "the wrap round", func() bool {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		return row == 1
	})

	// And backwards.
	press(screen, tcell.KeyF7, 0, tcell.ModAlt)
	waitFor(t, e, "the wrap backwards", func() bool {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		return row == 2
	})
	if got := cursorLine(); got != 3 {
		t.Errorf("cursor on line %d", got)
	}
}

func TestProblemsDialogListsAndJumps(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), troubled))
	waitForDiagnostics(t, e, 2)

	press(screen, tcell.KeyF9, 0, tcell.ModAlt)
	waitFor(t, e, "the problems list", func() bool { return e.modals == 1 })
	redraw(t, e)

	rows := dump(screen)
	t.Logf("screen:\n%s", strings.Join(rows, "\n"))
	joined := strings.Join(rows, "\n")
	for _, want := range []string{"1 errors, 1 warnings", "error", "warning", "unexpected symbol", "doubtful"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the list is missing %q", want)
		}
	}

	// The second entry is the warning, on line 3.
	onEditor(t, e, func() {
		list, ok := e.modalStack[0].(*clickList)
		if !ok {
			t.Fatalf("the list is a %T", e.modalStack[0])
		}
		list.SetCurrentItem(1)
	})
	press(screen, tcell.KeyEnter, 0, tcell.ModNone)
	waitFor(t, e, "the jump", func() bool {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		return e.modals == 0 && row == 2
	})
}

// A clean file says so, rather than opening an empty list.
func TestProblemsDialogWithNothingToReport(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "local a = 1\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil })

	press(screen, tcell.KeyF9, 0, tcell.ModAlt)
	waitFor(t, e, "the explanation", func() bool { return e.modals == 1 })
	redraw(t, e)

	// The dialog wraps its text, so read the screen as one run of words.
	if words := screenWords(screen); !strings.Contains(words, "nothing to report about main.lua") {
		t.Errorf("the dialog does not explain itself:\n%s", strings.Join(dump(screen), "\n"))
	}
}

// The buffer bar marks a file that has something wrong with it.
func TestBufferBarMarksProblems(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t,
		write(t, filepath.Join(dir, "bad.lua"), troubled),
		write(t, filepath.Join(dir, "fine.lua"), "local ok = 1\n"))
	waitFor(t, e, "the report", func() bool { return len(e.buffers[0].diagnostics) == 2 })
	redraw(t, e)

	bar := dump(screen)[1]
	if !strings.Contains(bar, "bad.lua!") {
		t.Errorf("buffer bar = %q, want the file with errors marked", bar)
	}
	if strings.Contains(bar, "fine.lua!") {
		t.Errorf("the clean file was marked: %q", bar)
	}
}

// Typing a mistake gets it marked, and putting it right clears the mark.
func TestDiagnosticsFollowTheTyping(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "local a = 1\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 11)
		b.area.Select(offset, offset)
	})
	typeText(screen, " ??")
	waitForDiagnostics(t, e, 1)
	onEditor(t, e, func() {
		if e.buffers[0].diagnostics[0].Range.Start.Line != 0 {
			t.Errorf("diagnostic = %+v", e.buffers[0].diagnostics[0])
		}
	})

	// Take it back out again.
	press(screen, tcell.KeyBackspace2, 0, tcell.ModNone)
	press(screen, tcell.KeyBackspace2, 0, tcell.ModNone)
	waitForDiagnostics(t, e, 0)
}

func TestDiagnosticAt(t *testing.T) {
	list := []lsp.Diagnostic{
		{Range: Range(1, 0, 1, 5), Severity: lsp.SeverityWarning, Message: "mild"},
		{Range: Range(1, 6, 1, 9), Severity: lsp.SeverityError, Message: "worse"},
		{Range: Range(4, 0, 6, 2), Severity: lsp.SeverityError, Message: "spanning"},
	}

	// The worst on a line is the one worth saying.
	if d, ok := diagnosticAt(list, 1); !ok || d.Message != "worse" {
		t.Errorf("line 1 gave %+v", d)
	}
	// A diagnostic that spans lines covers the ones in between.
	for _, line := range []int{4, 5, 6} {
		if d, ok := diagnosticAt(list, line); !ok || d.Message != "spanning" {
			t.Errorf("line %d gave %+v", line, d)
		}
	}
	if _, ok := diagnosticAt(list, 2); ok {
		t.Error("a clean line reported a diagnostic")
	}
	if _, ok := diagnosticAt(nil, 0); ok {
		t.Error("an empty report gave a diagnostic")
	}
}

// Range is a small helper for building diagnostics in tests.
func Range(startLine, startChar, endLine, endChar int) lsp.Range {
	return lsp.Range{
		Start: lsp.Position{Line: startLine, Character: startChar},
		End:   lsp.Position{Line: endLine, Character: endChar},
	}
}
