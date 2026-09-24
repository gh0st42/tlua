package editor

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"tlua/internal/lsp"
)

const (
	lspCompleteTimeout = 2500 * time.Millisecond
	lspHoverTimeout    = 2500 * time.Millisecond

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
func (e *Editor) complete() {
	b := e.buf()
	if b == nil {
		return
	}
	switch {
	case e.lsp == nil:
		e.setStatus("No language server; put one on PATH, or set " + lsp.EnvServer)
		return
	case !e.lsp.CanComplete():
		e.setStatus(e.lsp.Name() + " does not offer completions")
		return
	case b.path == "":
		e.setStatus("Save this buffer to a file first")
		return
	}

	text := b.area.GetText()
	ctx, cancel := context.WithTimeout(context.Background(), lspCompleteTimeout)
	defer cancel()

	items, err := e.lsp.Complete(ctx, b.path, text, e.cursorPosition(b))
	if err != nil {
		e.setStatus("Complete: " + err.Error())
		return
	}
	if len(items) == 0 {
		e.setStatus("No completions here")
		return
	}
	e.showCompletions(b, items)
}

// showCompletions floats the list under the cursor, the way a completion list
// should appear where the typing is.
func (e *Editor) showCompletions(b *buffer, items []lsp.CompletionItem) {
	const name = "complete"

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
		if event.Key() == tcell.KeyEscape {
			e.closeModal(name)
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
	_, cursor, _ := b.area.GetSelection()
	start := wordBeforeCursor(text, cursor)
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
	case b.path == "":
		e.setStatus("Save this buffer to a file first")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), lspHoverTimeout)
	defer cancel()

	help, err := e.lsp.Hover(ctx, b.path, b.area.GetText(), e.cursorPosition(b))
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
