package editor

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"tlua/internal/lsp"
)

const (
	// Asking by hand can afford to wait; asking because a "." was typed cannot,
	// because the answer is in the way of the next keystroke.
	lspCompleteTimeout = 2500 * time.Millisecond
	lspAutoTimeout     = 700 * time.Millisecond
	lspHoverTimeout    = 2500 * time.Millisecond

	// A server that has only just started is still reading the standard library
	// and answers with nothing; one retry covers that.
	lspCompleteRetry = 250 * time.Millisecond

	// How many completions the popup shows before it scrolls.
	completionRows = 10
)

// cursorPosition turns the cursor into a protocol position: a line, and a
// count of UTF-16 code units along it, which is how the protocol measures a
// column.
func (e *Editor) cursorPosition(b *buffer) lsp.Position {
	text := b.area.GetText()
	row, column, _, _ := b.area.GetCursor()
	offset := offsetAt(text, row, column)

	lineStart := offsetAt(text, row, 0)
	units := 0
	for i := lineStart; i < offset; {
		r, size := utf8.DecodeRuneInString(text[i:])
		units += len(utf16.Encode([]rune{r}))
		i += size
	}
	return lsp.Position{Line: row, Character: units}
}

// isNameRune reports whether a rune can be part of a Lua name, which is what
// decides where the word under the cursor begins and ends.
func isNameRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// wordBeforeCursor is the run of name characters the cursor sits at the end of,
// which is what a completion without an edit of its own replaces.
func wordBeforeCursor(text string, offset int) (start int) {
	start = offset
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:start])
		if !isNameRune(r) && r != '.' && r != ':' {
			break
		}
		start -= size
	}
	// Keep a dotted or colon prefix out of it: the server completes the part
	// after the separator.
	if i := strings.LastIndexAny(text[start:offset], ".:"); i >= 0 {
		start += i + 1
	}
	return start
}

// complete is Ctrl-Space: it asks the language server what could go here and
// offers the answers in a list at the cursor.
func (e *Editor) complete() { e.completeWith(false) }

// completeAutomatically is the same request made because a trigger character was
// typed. It runs off the event loop, so typing never waits on the server, and it
// says nothing when there is nothing to say, since a message on every "." would
// be noise.
func (e *Editor) completeAutomatically() {
	b, client := e.buf(), e.lsp
	if b == nil || client == nil || !client.CanComplete() {
		return
	}
	if e.completeAsking {
		// Typing faster than the server answers: remember that the question
		// needs asking again, rather than losing the latest keystroke.
		e.completeWanted = true
		return
	}
	generation := e.lspGeneration
	path, text, pos := b.lspPath(), b.area.GetText(), e.cursorPosition(b)
	e.completeAsking = true
	// The session starts with the question, not with the answer: the next letter
	// typed has to ask again even if the first answer is still on its way.
	e.completing = true

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), lspAutoTimeout)
		defer cancel()
		items, err := client.Complete(ctx, path, text, pos)

		e.app.QueueUpdateDraw(func() {
			e.completeAsking = false
			if e.completeWanted {
				// The text moved on while this answer was coming; ask about
				// where things are now instead of showing where they were.
				e.completeWanted = false
				e.completeAutomatically()
				return
			}
			if generation != e.lspGeneration || e.buf() != b {
				return // about where the cursor used to be; a newer ask may follow
			}
			if err != nil || len(items) == 0 {
				e.completing = false // there is nothing here to complete
				return
			}
			if e.modals > 0 || e.openMenu >= 0 {
				return // something else has the screen
			}
			e.showCompletions(b, items)
		})
	}()
}

func (e *Editor) completeWith(automatic bool) {
	b := e.buf()
	if b == nil {
		return
	}
	switch {
	case e.lsp == nil:
		if !automatic {
			e.setStatus("No language server; put one on PATH, or set " + lsp.EnvServer)
		}
		return
	case !e.lsp.CanComplete():
		if !automatic {
			e.setStatus(e.lsp.Name() + " does not offer completions")
		}
		return
	}

	timeout := lspCompleteTimeout
	if automatic {
		timeout = lspAutoTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	text := b.area.GetText()
	items, err := e.lsp.Complete(ctx, b.lspPath(), text, e.cursorPosition(b))
	if err == nil && len(items) == 0 && !automatic {
		// A server that has only just started has not read the standard library
		// yet and answers with nothing at all; ask once more.
		time.Sleep(lspCompleteRetry)
		items, err = e.lsp.Complete(ctx, b.lspPath(), text, e.cursorPosition(b))
	}
	if err != nil {
		if !automatic {
			e.setStatus("Complete: " + err.Error())
		}
		return
	}
	if len(items) == 0 {
		if !automatic {
			e.setStatus("No completions here")
		}
		return
	}
	e.showCompletions(b, items)
}

// autoTriggers are the characters after which this editor asks for completions
// without being told to.
//
// Servers are free to ask for more, and lua-language-server asks for a great
// deal: space, tab, newline, "(", "=", "-", "," and more besides. Opening a
// completion list on every space is not help, it is an obstacle, so the
// server's list is narrowed to member access, which is where an unprompted
// list earns its place. Ctrl-Space still asks anywhere.
const autoTriggers = ".:"

// isTriggerCharacter reports whether typing r is reason enough to ask for
// completions: the server has to want it, and it has to be one of the few
// characters where a list is welcome.
func (e *Editor) isTriggerCharacter(r rune) bool {
	if e.lsp == nil || !e.lsp.CanComplete() || !strings.ContainsRune(autoTriggers, r) {
		return false
	}
	for _, trigger := range e.lsp.TriggerCharacters() {
		if trigger == string(r) {
			return true
		}
	}
	return false
}

// showCompletions floats the list under the cursor, the way a completion list
// should appear where the typing is.
func (e *Editor) showCompletions(b *buffer, items []lsp.CompletionItem) {
	const name = "complete"
	e.completing = true // picking from the list is part of the session

	// The frame carries the border; the list sits inside it with the hint line.
	list := newClickList()
	list.SetUseStyleTags(true, false)

	// The line under the list is secondary, so it is drawn dimmer than the
	// items themselves.
	help := tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	help.SetBackgroundColor(egaLightGray)
	help.SetTextStyle(dialogTextStyle.Foreground(egaDarkGray))

	width := 24
	for _, item := range items {
		if w := len(item.Label) + len(item.Kind.String()) + 6; w > width {
			width = w
		}
	}
	if width > 60 {
		width = 60
	}

	for _, item := range items {
		chosen := item
		label := fmt.Sprintf("[black]%s", tview.Escape(item.Label))
		if kind := item.Kind.String(); kind != "" {
			label += fmt.Sprintf("  [gray]%s", kind)
		}
		list.AddItem(label, "", 0, func() {
			e.completing = false
			e.closeModal(name)
			e.insertCompletion(b, chosen)
		})
	}
	// The line under the list explains whatever is selected.
	list.SetChangedFunc(func(index int, _, _ string, _ rune) {
		if index >= 0 && index < len(items) {
			help.SetText(tview.Escape(oneLine(items[index].Help())))
		}
	})
	help.SetText(tview.Escape(oneLine(items[0].Help())))

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			e.completing = false
			e.closeModal(name)
			return nil

		case tcell.KeyRune, tcell.KeyBackspace, tcell.KeyBackspace2:
			// A completion list must not stand in the way of typing: the key
			// goes into the text, and the list comes back narrowed to whatever
			// the word now is.
			e.closeModal(name)
			e.sendKeyToText(event)
			if event.Key() != tcell.KeyRune || isNameRune(event.Rune()) ||
				e.isTriggerCharacter(event.Rune()) {
				e.completeAutomatically()
			} else {
				e.completing = false
			}
			return nil
		}
		return event
	})

	frame := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(help, 1, 0, false)
	frame.SetBackgroundColor(egaLightGray)
	dialogColors(frame.Box)
	frame.SetTitle(fmt.Sprintf(" %d completions ", len(items)))

	rows := len(items)
	if rows > completionRows {
		rows = completionRows
	}
	e.showAt(name, frame, e.cursorScreenPosition(b), width, rows+3)
}

// insertCompletion puts a chosen completion into the buffer. An item that
// brought its own edit says exactly what to replace; otherwise the word being
// typed makes way for it.
func (e *Editor) insertCompletion(b *buffer, item lsp.CompletionItem) {
	text := b.area.GetText()

	if edit := item.TextEdit; edit != nil {
		start, err1 := lsp.Offset(text, edit.Range.Start)
		end, err2 := lsp.Offset(text, edit.Range.End)
		if err1 == nil && err2 == nil {
			if end < start {
				start, end = end, start
			}
			b.area.Replace(start, end, edit.NewText)
			at := start + len(edit.NewText)
			b.area.Select(at, at)
			e.setStatus("Inserted " + item.Label)
			return
		}
	}

	insert := item.Text()
	_, selStart, selEnd := b.area.GetSelection()
	start, cursor := selStart, selEnd
	if selStart == selEnd {
		// Nothing selected: the word being typed makes way.
		start = wordBeforeCursor(text, selEnd)
	}
	b.area.Replace(start, cursor, insert)
	at := start + len(insert)
	b.area.Select(at, at)
	e.setStatus("Inserted " + item.Label)
}

// hover is Ctrl-F1, or F11: it shows what the language server knows about the
// thing under the cursor, which is what Turbo Pascal put on Ctrl-F1.
func (e *Editor) hover() {
	b := e.buf()
	if b == nil {
		return
	}
	switch {
	case e.lsp == nil:
		e.setStatus("No language server; put one on PATH, or set " + lsp.EnvServer)
		return
	case !e.lsp.CanHover():
		e.setStatus(e.lsp.Name() + " does not offer hover help")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), lspHoverTimeout)
	defer cancel()

	help, err := e.lsp.Hover(ctx, b.lspPath(), b.area.GetText(), e.cursorPosition(b))
	if err != nil {
		e.setStatus("Help: " + err.Error())
		return
	}
	if strings.TrimSpace(help) == "" {
		e.setStatus("Nothing known about what is under the cursor")
		return
	}
	e.showHover(help)
}

// showHover puts the server's answer in a scrollable box.
func (e *Editor) showHover(help string) {
	const name = "hover"
	text := tview.NewTextView().SetDynamicColors(false).SetWrap(true).SetScrollable(true)
	text.SetBackgroundColor(egaLightGray)
	text.SetTextStyle(dialogTextStyle)
	text.SetText(help)
	dialogColors(text.Box)
	text.SetTitle(" Help ")
	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyEnter:
			e.closeModal(name)
			return nil
		}
		return event
	})

	height := strings.Count(help, "\n") + 3
	if height > 14 {
		height = 14
	}
	e.showModal(name, text, 64, height)
}

// oneLine squashes a server's paragraphs into the single line a hint bar has.
func oneLine(text string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(text, "\n", " ")), " ")
}
