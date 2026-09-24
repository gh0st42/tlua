package editor

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"tlua/internal/lsp"
)

func TestWordBeforeCursor(t *testing.T) {
	cases := []struct {
		text   string
		cursor int
		want   string
	}{
		{"local pri", 9, "pri"},
		{"local x = 1", 11, "1"},
		{"table.ins", 9, "ins"}, // the part after the dot is what completes
		{"obj:met", 7, "met"},   // and after a colon
		{"print(", 6, ""},       // nothing to replace
		{"", 0, ""},
		{"  indented", 10, "indented"},
	}
	for _, c := range cases {
		start := wordBeforeCursor(c.text, c.cursor)
		if got := c.text[start:c.cursor]; got != c.want {
			t.Errorf("wordBeforeCursor(%q, %d) covers %q, want %q", c.text, c.cursor, got, c.want)
		}
	}
}

func TestCursorPositionCountsUTF16Units(t *testing.T) {
	dir := t.TempDir()
	e, _ := start(t, write(t, filepath.Join(dir, "main.lua"),
		"-- \U0001F600 x\nsecond line\n"))

	onEditor(t, e, func() {
		b := e.buffers[0]
		// Put the cursor at the end of the first line, past the emoji.
		offset := offsetAt(b.area.GetText(), 0, 99)
		b.area.Select(offset, offset)
		got := e.cursorPosition(b)
		// "-- " is three units, the emoji two, " x" two more.
		if got.Line != 0 || got.Character != 7 {
			t.Errorf("position = %+v, want line 0 character 7", got)
		}

		e.gotoLine(b, 2)
		if got := e.cursorPosition(b); got.Line != 1 || got.Character != 0 {
			t.Errorf("position = %+v, want the start of line 2", got)
		}
	})
}

// completionList digs the list out of the popup, which also holds the hint line.
func completionList(t *testing.T, e *Editor) *clickList {
	t.Helper()
	frame, ok := e.modalStack[len(e.modalStack)-1].(*tview.Flex)
	if !ok {
		t.Fatalf("the completion popup is a %T", e.modalStack[len(e.modalStack)-1])
	}
	list, ok := frame.GetItem(0).(*clickList)
	if !ok {
		t.Fatalf("the first item is a %T", frame.GetItem(0))
	}
	return list
}

func TestCompleteInsertsTheChosenItem(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "local x = 1\npri\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 1, 3) // just after "pri"
		b.area.Select(offset, offset)
	})

	press(screen, tcell.KeyCtrlSpace, 0, tcell.ModNone)
	waitFor(t, e, "the completion list", func() bool { return e.modals == 1 })

	// The first item the fake server offers is "print".
	press(screen, tcell.KeyEnter, 0, tcell.ModNone)
	waitFor(t, e, "the insertion", func() bool {
		return e.modals == 0 && e.buffers[0].area.GetText() == "local x = 1\nprint\n"
	})
	onEditor(t, e, func() {
		if !strings.Contains(e.status, "print") {
			t.Errorf("status = %q", e.status)
		}
	})
}

// An item whose insertText differs from its label inserts the text.
func TestCompleteUsesInsertText(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "tab\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 3)
		b.area.Select(offset, offset)
	})
	press(screen, tcell.KeyCtrlSpace, 0, tcell.ModNone)
	waitFor(t, e, "the completion list", func() bool { return e.modals == 1 })

	onEditor(t, e, func() {
		list := completionList(t, e)
		for i := 0; i < list.GetItemCount(); i++ {
			label, _ := list.GetItemText(i)
			if strings.Contains(label, "table_insert") {
				list.SetCurrentItem(i)
				return
			}
		}
		t.Fatal("table_insert is not in the list")
	})
	press(screen, tcell.KeyEnter, 0, tcell.ModNone)
	waitFor(t, e, "the insertion", func() bool {
		return e.buffers[0].area.GetText() == "table.insert\n"
	})
}

// An item that brings its own edit says exactly what to replace.
func TestCompleteAppliesAnItemsOwnEdit(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "word\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 4)
		b.area.Select(offset, offset)
	})
	press(screen, tcell.KeyCtrlSpace, 0, tcell.ModNone)
	waitFor(t, e, "the completion list", func() bool { return e.modals == 1 })

	onEditor(t, e, func() {
		list := completionList(t, e)
		for i := 0; i < list.GetItemCount(); i++ {
			label, _ := list.GetItemText(i)
			if strings.Contains(label, "replaced_word") {
				list.SetCurrentItem(i)
				return
			}
		}
		t.Fatal("replaced_word is not in the list")
	})
	press(screen, tcell.KeyEnter, 0, tcell.ModNone)
	waitFor(t, e, "the edit to be applied", func() bool {
		return e.buffers[0].area.GetText() == "REPLACED\n"
	})
}

// A snippet goes in with its placeholders reduced to their defaults.
func TestCompleteFlattensASnippet(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "fo\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 2)
		b.area.Select(offset, offset)
	})
	press(screen, tcell.KeyCtrlSpace, 0, tcell.ModNone)
	waitFor(t, e, "the completion list", func() bool { return e.modals == 1 })

	onEditor(t, e, func() {
		list := completionList(t, e)
		for i := 0; i < list.GetItemCount(); i++ {
			label, _ := list.GetItemText(i)
			if strings.Contains(label, "for_loop") {
				list.SetCurrentItem(i)
				return
			}
		}
		t.Fatal("for_loop is not in the list")
	})
	press(screen, tcell.KeyEnter, 0, tcell.ModNone)
	waitFor(t, e, "the snippet", func() bool {
		return strings.HasPrefix(e.buffers[0].area.GetText(), "for i = 1, n do")
	})
	onEditor(t, e, func() {
		if strings.Contains(e.buffers[0].area.GetText(), "$") {
			t.Errorf("placeholders survived: %q", e.buffers[0].area.GetText())
		}
	})
}

func TestCompleteSaysWhenThereIsNoServer(t *testing.T) {
	t.Setenv(lsp.EnvServer, "off")
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "pri\n"))

	press(screen, tcell.KeyCtrlSpace, 0, tcell.ModNone)
	waitFor(t, e, "the explanation", func() bool {
		return strings.Contains(e.status, "No language server")
	})
	if e.modals != 0 {
		t.Error("something was opened anyway")
	}
}

func TestCompleteOnAServerWithoutCompletions(t *testing.T) {
	withLanguageServer(t, "FAKELSP_NOCOMPLETE=1")
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "pri\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil })

	press(screen, tcell.KeyCtrlSpace, 0, tcell.ModNone)
	waitFor(t, e, "the explanation", func() bool {
		return strings.Contains(e.status, "does not offer completions")
	})
}

/* --- hover --- */

func TestHoverShowsWhatTheServerKnows(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "print(1)\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanHover() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 2)
		b.area.Select(offset, offset)
	})
	press(screen, tcell.KeyF11, 0, tcell.ModNone)
	waitFor(t, e, "the help box", func() bool { return e.modals == 1 })

	onEditor(t, e, func() {
		box, ok := e.modalStack[0].(*tview.TextView)
		if !ok {
			t.Fatalf("the help is a %T", e.modalStack[0])
		}
		help := box.GetText(true)
		if !strings.Contains(help, "function print(...)") {
			t.Errorf("help = %q", help)
		}
		if !strings.Contains(help, "character 2") {
			t.Errorf("the cursor position did not reach the server: %q", help)
		}
		if strings.Contains(help, "```") || strings.Contains(help, "**") {
			t.Errorf("markdown survived: %q", help)
		}
	})

	press(screen, tcell.KeyEscape, 0, tcell.ModNone)
	waitFor(t, e, "the help box to close", func() bool { return e.modals == 0 })
}

func TestHoverSaysWhenThereIsNoServer(t *testing.T) {
	t.Setenv(lsp.EnvServer, "off")
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "print(1)\n"))

	press(screen, tcell.KeyF11, 0, tcell.ModNone)
	waitFor(t, e, "the explanation", func() bool {
		return strings.Contains(e.status, "No language server")
	})
}

func TestHoverOnAServerWithoutHover(t *testing.T) {
	withLanguageServer(t, "FAKELSP_NOHOVER=1")
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "print(1)\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil })

	press(screen, tcell.KeyF11, 0, tcell.ModNone)
	waitFor(t, e, "the explanation", func() bool {
		return strings.Contains(e.status, "does not offer hover help")
	})
}

// F1 is still the key list, even though Ctrl-F1 asks the server.
func TestPlainF1IsStillTheKeyList(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "print(1)\n"))

	press(screen, tcell.KeyF1, 0, tcell.ModNone)
	waitFor(t, e, "the key list", func() bool { return e.modals == 1 })
	onEditor(t, e, func() {
		box, ok := e.modalStack[0].(*tview.TextView)
		if !ok {
			t.Fatalf("the help is a %T", e.modalStack[0])
		}
		if !strings.Contains(box.GetText(true), "tlua editor") {
			t.Error("F1 did not open the key list")
		}
	})
}

// Typing a dot asks the server on its own: this is what "autocomplete" means.
func TestTypingADotOpensCompletions(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "local f = io\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 12) // just after "io"
		b.area.Select(offset, offset)
	})

	typeText(screen, ".")
	waitFor(t, e, "the completion list to appear by itself", func() bool { return e.modals == 1 })
	onEditor(t, e, func() {
		if got := e.buffers[0].area.GetText(); got != "local f = io.\n" {
			t.Errorf("the dot did not go in: %q", got)
		}
	})

	// And picking an item completes after the dot.
	press(screen, tcell.KeyEnter, 0, tcell.ModNone)
	waitFor(t, e, "the insertion", func() bool {
		return e.modals == 0 && e.buffers[0].area.GetText() == "local f = io.print\n"
	})
}

func TestTypingAColonOpensCompletions(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "obj\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 3)
		b.area.Select(offset, offset)
	})
	typeText(screen, ":")
	waitFor(t, e, "the completion list", func() bool { return e.modals == 1 })
}

// An ordinary letter is not a trigger, so typing stays quiet.
func TestTypingALetterDoesNotOpenCompletions(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "local x = 1\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	typeText(screen, "abc")
	waitFor(t, e, "the text to be typed", func() bool {
		return strings.HasPrefix(e.buffers[0].area.GetText(), "abclocal")
	})
	if e.modals != 0 {
		t.Error("a letter opened a completion list")
	}
}

// With the list open, typing carries on in the text and the list follows.
func TestTypingThroughTheCompletionList(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "io\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 2)
		b.area.Select(offset, offset)
	})
	typeText(screen, ".")
	waitFor(t, e, "the completion list", func() bool { return e.modals == 1 })

	// Keep typing: the characters land in the buffer, and the list comes back.
	typeText(screen, "pr")
	waitFor(t, e, "the typing to reach the buffer", func() bool {
		return e.buffers[0].area.GetText() == "io.pr\n"
	})
	waitFor(t, e, "the list to follow the typing", func() bool { return e.modals == 1 })

	// Backspace works the same way.
	press(screen, tcell.KeyBackspace2, 0, tcell.ModNone)
	waitFor(t, e, "the backspace to reach the buffer", func() bool {
		return e.buffers[0].area.GetText() == "io.p\n"
	})

	press(screen, tcell.KeyEscape, 0, tcell.ModNone)
	waitFor(t, e, "the list to close", func() bool { return e.modals == 0 })
}

// A buffer that has never been saved can still be completed in.
func TestCompleteInAnUnsavedBuffer(t *testing.T) {
	withLanguageServer(t)
	e, screen := start(t)
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	typeText(screen, "pri")
	waitFor(t, e, "the typing", func() bool { return e.buffers[0].area.GetText() == "pri" })

	press(screen, tcell.KeyCtrlSpace, 0, tcell.ModNone)
	waitFor(t, e, "the completion list", func() bool { return e.modals == 1 })
	press(screen, tcell.KeyEnter, 0, tcell.ModNone)
	waitFor(t, e, "the insertion", func() bool {
		return e.buffers[0].area.GetText() == "print"
	})
}

// A server may ask to be consulted after almost anything; a list that opens on
// every space would be an obstacle rather than help.
func TestOnlyMemberAccessTriggersCompletionsByItself(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, _ := start(t, write(t, filepath.Join(dir, "main.lua"), "local x = 1\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	onEditor(t, e, func() {
		// The fake server asks for all of these, as a real one does.
		asked := strings.Join(e.lsp.TriggerCharacters(), "")
		for _, r := range asked {
			if !strings.ContainsRune(" (=,-", r) {
				continue
			}
			if e.isTriggerCharacter(r) {
				t.Errorf("typing %q would open a completion list", string(r))
			}
		}
		for _, r := range ".:" {
			if !e.isTriggerCharacter(r) {
				t.Errorf("typing %q should open a completion list", string(r))
			}
		}
	})
}

func TestTypingASpaceDoesNotOpenCompletions(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "local x =\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 9)
		b.area.Select(offset, offset)
	})
	typeText(screen, " 1")
	waitFor(t, e, "the typing", func() bool {
		return e.buffers[0].area.GetText() == "local x = 1\n"
	})
	if e.modals != 0 {
		t.Error("a space opened a completion list")
	}
}

// The list narrows as the word is typed, and goes when the word ends, which is
// what makes it a session rather than a one-off.
func TestCompletionSessionFollowsTheWordAndEnds(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "io\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 2)
		b.area.Select(offset, offset)
	})
	typeText(screen, ".")
	waitFor(t, e, "the list", func() bool { return e.modals == 1 })

	// Several letters in a row: the list is there after each one.
	for _, letter := range []string{"p", "r", "i"} {
		typeText(screen, letter)
		waitFor(t, e, "the list to follow "+letter, func() bool { return e.modals == 1 })
	}
	waitFor(t, e, "the letters to land", func() bool {
		return e.buffers[0].area.GetText() == "io.pri\n"
	})

	// A character that cannot be part of a name ends the session.
	typeText(screen, "(")
	waitFor(t, e, "the list to go", func() bool { return !e.completing })
	waitFor(t, e, "the bracket to land", func() bool {
		return strings.HasPrefix(e.buffers[0].area.GetText(), "io.pri(")
	})
}

// Backspacing while completing narrows the list back, rather than losing it.
func TestBackspaceKeepsTheCompletionSession(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "io\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 2)
		b.area.Select(offset, offset)
	})
	typeText(screen, ".pr")
	waitFor(t, e, "the list", func() bool { return e.modals == 1 })

	press(screen, tcell.KeyBackspace2, 0, tcell.ModNone)
	waitFor(t, e, "the backspace to land", func() bool {
		return e.buffers[0].area.GetText() == "io.p\n"
	})
	waitFor(t, e, "the list to still be there", func() bool { return e.modals == 1 })
}
