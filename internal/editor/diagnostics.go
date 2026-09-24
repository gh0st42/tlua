package editor

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"tlua/internal/lsp"
)

// syncDelay is how long the editor waits after the last keystroke before handing
// the text to the server. Diagnostics come back unprompted once it has, so this
// is what decides how soon a mistake is marked; it is long enough that a burst
// of typing is one message rather than twenty.
const syncDelay = 300 * time.Millisecond

// How a marked range is drawn. Errors take the strongest colours the palette
// has for the purpose; the milder severities only underline, so that a note
// cannot be mistaken for a mistake.
var diagnosticStyles = map[lsp.Severity]tcell.Style{
	lsp.SeverityError:   textStyle.Foreground(egaWhite).Background(egaRed),
	lsp.SeverityWarning: textStyle.Foreground(egaBlack).Background(egaBrown),
}

// diagnosticTags are the colours of a severity on a grey dialog.
var diagnosticTags = map[lsp.Severity]string{
	lsp.SeverityError:       "[maroon]",
	lsp.SeverityWarning:     "[olive]",
	lsp.SeverityInformation: "[navy]",
	lsp.SeverityHint:        "[gray]",
}

// watchDiagnostics arranges for the server's reports to reach the buffers.
//
// A report arrives on the client's reader goroutine, which must not be held up:
// it is the same goroutine that delivers the answer to a request, and the editor
// may be waiting on one. Putting a report straight onto the editing loop from
// there deadlocks the two against each other. So the reader only rings a bell,
// and a goroutine of the editor's own goes and fetches the current state.
func (e *Editor) watchDiagnostics(client *lsp.Client) {
	client.OnDiagnostics(func(string, []lsp.Diagnostic) {
		select {
		case e.diagnosticsWake <- struct{}{}:
		default: // a fetch is already pending, and it will see this report too
		}
	})
	go func() {
		for range e.diagnosticsWake {
			e.app.QueueUpdateDraw(func() { e.takeDiagnostics(client) })
		}
	}()
}

// takeDiagnostics brings every buffer up to date with what the server last said
// about it. It reads the client's own record rather than one report, so it
// cannot matter in which order reports arrived.
func (e *Editor) takeDiagnostics(client *lsp.Client) {
	for _, b := range e.buffers {
		list := client.Diagnostics(b.lspPath())
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].Range.Start.Line != list[j].Range.Start.Line {
				return list[i].Range.Start.Line < list[j].Range.Start.Line
			}
			return list[i].Range.Start.Character < list[j].Range.Start.Character
		})
		b.diagnostics = list
		b.area.setDiagnostics(list)
	}
	e.refreshTabs()
	e.refreshStatus()
}

// scheduleSync hands the text to the server once the typing pauses, which is
// what gets a report back. It is debounced: a burst of keystrokes is one sync.
func (e *Editor) scheduleSync() {
	if e.lsp == nil || e.syncScheduled {
		return
	}
	e.syncScheduled = true

	go func() {
		time.Sleep(syncDelay)

		var client *lsp.Client
		var path, text string
		e.app.QueueUpdateDraw(func() {
			e.syncScheduled = false
			if b := e.buf(); b != nil {
				client, path, text = e.lsp, b.lspPath(), b.area.GetText()
			}
		})
		if client != nil {
			_ = client.Sync(path, text) // off the editing loop: it writes to a pipe
		}
	}()
}

// diagnosticAt is what the server said about a line, preferring the worst of it.
func diagnosticAt(list []lsp.Diagnostic, line int) (lsp.Diagnostic, bool) {
	var found lsp.Diagnostic
	ok := false
	for _, d := range list {
		if line < d.Range.Start.Line || line > d.Range.End.Line {
			continue
		}
		if !ok || d.Severity < found.Severity {
			found, ok = d, true
		}
	}
	return found, ok
}

// diagnosticStatus is the message the status line shows for the cursor's line,
// or nothing when the line is clean.
func (e *Editor) diagnosticStatus() string {
	b := e.buf()
	if b == nil || len(b.diagnostics) == 0 {
		return ""
	}
	row, _, _, _ := b.area.GetCursor()
	d, ok := diagnosticAt(b.diagnostics, row)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s: %s", strings.Title(d.Severity.String()), oneLine(d.Message))
}

// problemCount counts what is wrong with a buffer, worst first.
func problemCount(list []lsp.Diagnostic) (errors, warnings int) {
	for _, d := range list {
		switch d.Severity {
		case lsp.SeverityError:
			errors++
		case lsp.SeverityWarning:
			warnings++
		}
	}
	return errors, warnings
}

// nextProblem moves to the next thing the server complained about, wrapping
// round, which is what Alt-F8 did in the Borland editors.
func (e *Editor) nextProblem() { e.moveToProblem(true) }

// previousProblem is the same the other way.
func (e *Editor) previousProblem() { e.moveToProblem(false) }

func (e *Editor) moveToProblem(forwards bool) {
	b := e.buf()
	if b == nil {
		return
	}
	if len(b.diagnostics) == 0 {
		e.setStatus(e.noProblemsMessage(b))
		return
	}

	row, _, _, _ := b.area.GetCursor()
	target := -1
	if forwards {
		for i, d := range b.diagnostics {
			if d.Range.Start.Line > row {
				target = i
				break
			}
		}
		if target < 0 {
			target = 0 // round to the first
		}
	} else {
		for i := len(b.diagnostics) - 1; i >= 0; i-- {
			if b.diagnostics[i].Range.Start.Line < row {
				target = i
				break
			}
		}
		if target < 0 {
			target = len(b.diagnostics) - 1
		}
	}

	d := b.diagnostics[target]
	e.gotoDiagnostic(b, d)
	e.setStatus(fmt.Sprintf("%d of %d — %s: %s", target+1, len(b.diagnostics),
		strings.Title(d.Severity.String()), oneLine(d.Message)))
}

// gotoDiagnostic puts the cursor where the trouble is.
func (e *Editor) gotoDiagnostic(b *buffer, d lsp.Diagnostic) {
	text := b.area.GetText()
	offset, err := lsp.Offset(text, d.Range.Start)
	if err != nil {
		e.gotoLine(b, d.Range.Start.Line+1)
		return
	}
	b.area.Select(offset, offset)
}

// noProblemsMessage says why there is nothing to go to.
func (e *Editor) noProblemsMessage(b *buffer) string {
	switch {
	case e.lsp == nil:
		return "No language server; put one on PATH, or set " + lsp.EnvServer
	case b.path == "" && b.area.GetTextLength() == 0:
		return "Nothing to check yet"
	default:
		return "The server has nothing to report about " + b.name
	}
}

// problemsDialog lists everything the server said about the buffer, so the whole
// lot can be seen at once rather than one message at a time.
func (e *Editor) problemsDialog() {
	b := e.buf()
	if b == nil {
		return
	}
	if len(b.diagnostics) == 0 {
		e.message("Problems", e.noProblemsMessage(b))
		return
	}

	const name = "problems"
	list := newClickList()
	list.SetUseStyleTags(true, false)
	dialogColors(list.Box)
	errors, warnings := problemCount(b.diagnostics)
	list.SetTitle(fmt.Sprintf(" %s: %d errors, %d warnings ", b.name, errors, warnings))

	row, _, _, _ := b.area.GetCursor()
	current := 0
	for i, d := range b.diagnostics {
		if d.Range.Start.Line <= row {
			current = i
		}
		diagnostic := d
		label := fmt.Sprintf("[black]%5d  %s%-8s %s", d.Range.Start.Line+1,
			diagnosticTags[d.Severity], d.Severity, tview.Escape(oneLine(d.Message)))
		list.AddItem(label, "", 0, func() {
			e.closeModal(name)
			e.gotoDiagnostic(b, diagnostic)
			e.setStatus(fmt.Sprintf("%s: %s", strings.Title(diagnostic.Severity.String()),
				oneLine(diagnostic.Message)))
		})
	}
	list.SetCurrentItem(current)
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			e.closeModal(name)
			return nil
		}
		return event
	})

	height := len(b.diagnostics) + 2
	if height > 18 {
		height = 18
	}
	e.showModal(name, list, 72, height)
}
