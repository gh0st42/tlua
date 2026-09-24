package editor

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"tlua/internal/lsp"
)

func TestStripTags(t *testing.T) {
	cases := map[string]string{
		"[black]plain":                           "plain",
		"f([black:green:b]x[black:silver:-], y)": "f(x, y)",
		"nothing to strip":                       "nothing to strip",
	}
	for in, want := range cases {
		if got := stripTags(in); got != want {
			t.Errorf("stripTags(%q) = %q, want %q", in, got, want)
		}
	}
}

// Typing the bracket that opens a call shows what it takes.
func TestTypingABracketShowsTheParameters(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "local s = string.format\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanSignature() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 23)
		b.area.Select(offset, offset)
	})

	typeText(screen, "(")
	waitFor(t, e, "the parameter hint", func() bool { return e.signatureShown })
	onEditor(t, e, func() {
		if got := e.buffers[0].area.GetText(); got != "local s = string.format(\n" {
			t.Errorf("the bracket did not go in: %q", got)
		}
		if !e.pages.HasPage(signaturePage) {
			t.Error("the hint is not on screen")
		}
	})

	// The hint takes no focus: typing carries on in the text.
	typeText(screen, "\"%d\"")
	waitFor(t, e, "the typing to reach the buffer", func() bool {
		return strings.Contains(e.buffers[0].area.GetText(), `format("%d"`)
	})

	press(screen, tcell.KeyEscape, 0, tcell.ModNone)
	waitFor(t, e, "the hint to go away", func() bool { return !e.signatureShown })
}

// A comma moves the hint to the next parameter.
func TestCommaMovesToTheNextParameter(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "string.format\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanSignature() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 13)
		b.area.Select(offset, offset)
	})
	typeText(screen, "(")
	waitFor(t, e, "the hint", func() bool { return e.signatureShown })

	// The fake server picks the parameter after the last comma, and the editor
	// shows it in the hint, so the second parameter is picked out after a comma.
	typeText(screen, "x,")
	waitFor(t, e, "the hint to follow the comma", func() bool {
		return e.signatureShown && strings.Contains(e.buffers[0].area.GetText(), "x,")
	})
}

// Leaving the call takes the hint down by itself.
func TestClosingTheCallHidesTheParameters(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "string.format\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanSignature() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 13)
		b.area.Select(offset, offset)
	})
	typeText(screen, "(")
	waitFor(t, e, "the hint", func() bool { return e.signatureShown })

	typeText(screen, "x)")
	waitFor(t, e, "the hint to go once the call is closed", func() bool { return !e.signatureShown })
}

func TestParametersOnTheStatusLine(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "string.format\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanSignature() })

	onEditor(t, e, func() {
		e.toggleSignaturePlace()
		if !e.signatureInStatus {
			t.Fatal("the setting did not turn on")
		}
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 13)
		b.area.Select(offset, offset)
	})

	typeText(screen, "(")
	waitFor(t, e, "the hint on the status line", func() bool {
		return e.signatureShown && strings.Contains(e.taggedStatus, "string.format")
	})
	onEditor(t, e, func() {
		if e.pages.HasPage(signaturePage) {
			t.Error("a panel was drawn as well as the status line")
		}
	})
}

// Ctrl-P asks by hand, and says so when there is nothing to say.
func TestCtrlPAsksForParameters(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "string.format(x\nlocal y = 1\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanSignature() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 15)
		b.area.Select(offset, offset)
	})
	press(screen, tcell.KeyCtrlP, 0, tcell.ModNone)
	waitFor(t, e, "the hint", func() bool { return e.signatureShown })

	// On a line with no call at all, it explains itself.
	onEditor(t, e, func() { e.gotoLine(e.buffers[0], 2) })
	press(screen, tcell.KeyCtrlP, 0, tcell.ModNone)
	waitFor(t, e, "the explanation", func() bool {
		return !e.signatureShown && strings.Contains(e.status, "Nothing is being called")
	})
}

func TestParametersWithoutAServer(t *testing.T) {
	t.Setenv(lsp.EnvServer, "off")
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "print(\n"))

	press(screen, tcell.KeyCtrlP, 0, tcell.ModNone)
	waitFor(t, e, "the explanation", func() bool {
		return strings.Contains(e.status, "No language server")
	})
}

// A server may ask to be consulted after a space; a hint that appears on every
// space is noise, so the list is narrowed as the completion triggers are.
func TestOnlyBracketsAndCommasTriggerTheHint(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, _ := start(t, write(t, filepath.Join(dir, "main.lua"), "print(1)\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanSignature() })

	onEditor(t, e, func() {
		for _, r := range " )" {
			if e.isSignatureTrigger(r) {
				t.Errorf("typing %q would open the hint", string(r))
			}
		}
		for _, r := range "(," {
			if !e.isSignatureTrigger(r) {
				t.Errorf("typing %q should open the hint", string(r))
			}
		}
	})
}

// Cursor movement must not wait on the language server. With a server that
// takes its time, moving about stays as quick as it is with no server at all.
func TestCursorMovementDoesNotWaitOnTheServer(t *testing.T) {
	const thinkMS = 400
	withLanguageServer(t, "FAKELSP_SLOW="+strconv.Itoa(thinkMS))

	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"),
		"string.format(x\nlocal a = 1\nlocal b = 2\nlocal c = 3\nlocal d = 4\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanSignature() })

	// Put the hint up, which does wait: it was asked for by hand.
	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 15)
		b.area.Select(offset, offset)
	})
	press(screen, tcell.KeyCtrlP, 0, tcell.ModNone)
	waitFor(t, e, "the hint", func() bool { return e.signatureShown })

	// Now move about. Each move asks the server again, and each answer takes
	// 400ms, so waiting for them would cost seconds.
	const moves = 8
	start := time.Now()
	for i := 0; i < moves; i++ {
		press(screen, tcell.KeyDown, 0, tcell.ModNone)
		press(screen, tcell.KeyUp, 0, tcell.ModNone)
	}
	// The editor answers the keyboard throughout.
	waitFor(t, e, "the editor to keep up", func() bool {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		return row == 0
	})
	elapsed := time.Since(start)

	if elapsed > moves*thinkMS*time.Millisecond/2 {
		t.Errorf("%d moves took %v; the editor is waiting on the server", moves*2, elapsed)
	}
	t.Logf("%d moves in %v with a server that thinks for %dms", moves*2, elapsed, thinkMS)
}

// Typing does not wait on it either.
func TestTypingDoesNotWaitOnTheServer(t *testing.T) {
	withLanguageServer(t, "FAKELSP_SLOW=400")

	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "local t = {}\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 12)
		b.area.Select(offset, offset)
	})

	start := time.Now()
	typeText(screen, ".insert(1, 2)")
	waitFor(t, e, "the typing to land", func() bool {
		return strings.Contains(e.buffers[0].area.GetText(), ".insert(1, 2)")
	})
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("typing 13 characters took %v; the editor is waiting on the server", elapsed)
	}
	t.Logf("13 characters in %v with a server that thinks for 400ms", elapsed)
}
