// Package editor is a full-screen Lua editor in the style of the DOS IDEs:
// a menu bar across the top, a blue editing desktop, a status line of function
// keys along the bottom, several files open at once, and F5 to run.
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"tlua/internal/lsp"
)

// Editor is one editing session.
type Editor struct {
	app     *tview.Application
	pages   *tview.Pages // the desktop, plus menus and dialogs on top
	layout  *tview.Flex
	editors *tview.Pages // one page per buffer

	menubar     *clickableBar
	tabs        *clickableBar
	tabSpans    []tabSpan
	statusKeys  *tview.TextView
	statusRight *tview.TextView
	output      *tview.TextView

	buffers  []*buffer
	current  int
	primary  *buffer // the file F5 runs
	untitled int

	clipboard string
	search    searchState

	menus      []*menu
	openMenu   int        // index into menus, or -1
	menuList   *clickList // the open dropdown, for the mouse
	modals     int        // how many dialogs are stacked on the desktop
	modalStack []tview.Primitive

	// The language server, when one was found on PATH.
	lsp          *lsp.Client
	lspNote      string
	formatOnSave bool
	leaving      bool

	// The parameter hint: whether it is up, and whether it shows at the cursor
	// or on the status line.
	signatureShown    bool
	signatureInStatus bool

	// lspGeneration counts changes of text or cursor, so an answer that arrives
	// after the user has moved on can be recognised and dropped. The asking
	// flags keep one question of each kind in the air at a time.
	lspGeneration   int
	signatureAsking bool
	signatureWanted bool
	completeAsking  bool
	completeWanted  bool
	// completing marks a completion session: while one is in progress every
	// name character asks again, so the list narrows as the word is typed
	// rather than appearing once and going.
	completing bool

	outputShown bool
	outputBytes int  // how much the pane holds, against outputLimit
	outputFull  bool // set once the pane stopped accepting more

	// Program output gathers here between redraws.
	outputMu       sync.Mutex
	outputPending  strings.Builder
	outputFlushing bool
	running        *exec.Cmd // the program started by F5, while it runs
	exe            string    // this binary, used to run scripts
	status         string    // the message shown until something replaces it
	taggedStatus   string    // a message that brings its own colours
}

const outputHeight = 10

// keyHints is the bottom line's standing content: short enough to fit an 80
// column terminal beside the position readout, which is 15 columns. The rest of
// the keys live in the menus and in F1. It never changes, so it is built once
// rather than on every cursor movement.
var keyHints = tagBar + " " + strings.Join([]string{
	"F1 Help", "F2 Save", "F3 Open", "F5 Run", "F7 Find", "F9 Check", "F10 Menu",
}, "  ")

// Config is what an editing session starts from.
type Config struct {
	// Files to open. The first one that has a path becomes the primary file,
	// the one F5 runs.
	Files []string
	// Interpreter is the binary F5 starts. Empty means this executable, which
	// is what the tlua command uses.
	Interpreter string
}

// New builds an editor with the configured files open.
func New(cfg Config) (*Editor, error) {
	applyTheme()

	exe := cfg.Interpreter
	if exe == "" {
		found, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("cannot locate the tlua binary: %w", err)
		}
		exe = found
	}

	e := &Editor{
		app:      tview.NewApplication(),
		pages:    tview.NewPages(),
		editors:  tview.NewPages(),
		openMenu: -1,
		current:  -1,
		exe:      exe,
	}

	e.menubar = newClickableBar(e.clickMenuBar)
	e.menubar.SetBackgroundColor(egaLightGray)

	e.tabs = newClickableBar(e.clickTabs)
	e.tabs.SetBackgroundColor(egaCyan)

	e.statusKeys = tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	e.statusKeys.SetBackgroundColor(egaLightGray)
	e.statusRight = tview.NewTextView().SetDynamicColors(true).SetWrap(false).
		SetTextAlign(tview.AlignRight)
	e.statusRight.SetBackgroundColor(egaLightGray)

	e.output = tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	e.output.SetBackgroundColor(egaBlack)
	e.output.SetBorder(true).SetTitle(" Output ").SetTitleColor(egaYellow)
	e.output.SetBorderColor(egaLightGray)

	status := tview.NewFlex().
		AddItem(e.statusKeys, 0, 1, false).
		AddItem(e.statusRight, 15, 0, false)

	e.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(e.menubar, 1, 0, false).
		AddItem(e.tabs, 1, 0, false).
		AddItem(e.editors, 0, 1, true).
		AddItem(e.output, 0, 0, false). // resized when the pane is shown
		AddItem(status, 1, 0, false)
	e.layout.SetBackgroundColor(egaBlue)

	e.pages.AddPage("desktop", e.layout, true, true)
	e.app.SetRoot(e.pages, true)
	e.app.SetInputCapture(e.handleKey)
	e.app.SetMouseCapture(e.captureMouse)
	e.app.EnableMouse(true)

	e.menus = e.buildMenus()
	e.drawMenuBar()
	e.formatOnSave = true

	for _, f := range cfg.Files {
		if err := e.openFile(f); err != nil {
			return nil, err
		}
	}
	if len(e.buffers) == 0 {
		e.newFile()
	}
	e.selectBuffer(0)
	e.setStatus("F5 runs the file marked ».  F10 opens the menu.")
	// A language server is optional: this looks for one and starts it in the
	// background, and the editor is usable whether or not it finds one.
	e.startLanguageServer()
	return e, nil
}

// Run takes over the terminal until the user leaves.
func (e *Editor) Run() error {
	defer e.stopLanguageServer()
	// A program started with F5 must not outlive the editor that started it.
	defer func() {
		if e.running != nil && e.running.Process != nil {
			_ = e.running.Process.Kill()
		}
	}()
	return e.app.Run()
}

// SetScreen hands the application a screen to draw on, which is what tests use
// to drive the editor without a terminal.
func (e *Editor) SetScreen(screen tcell.Screen) {
	e.app.SetScreen(screen)
}

/* --- the bars --- */

// refreshTabs redraws the buffer list and the frame around the active buffer.
func (e *Editor) refreshTabs() {
	var sb strings.Builder
	e.tabSpans = e.tabSpans[:0]
	column := 0
	for i, b := range e.buffers {
		tag := tagTabIdle
		if i == e.current {
			tag = tagTabActive
		}
		label := fmt.Sprintf(" %d:%s ", i+1, e.title(b))
		fmt.Fprintf(&sb, "%s%s", tag, tview.Escape(label))
		e.tabSpans = append(e.tabSpans, tabSpan{from: column, to: column + len([]rune(label)), buffer: i})
		column += len([]rune(label))
	}
	sb.WriteString(tagTabIdle)
	e.tabs.SetText(sb.String())

	// tview draws a double-line frame around whichever window has the focus,
	// which is how a DOS IDE marked the active one.
	for _, b := range e.buffers {
		b.area.SetBorder(true)
		b.area.SetBorderColor(egaLightGray)
		b.area.SetTitleColor(egaYellow)
		b.area.SetTitle(fmt.Sprintf(" %s ", e.title(b)))
	}
}

// refreshStatus redraws the function key hints and the cursor position.
func (e *Editor) refreshStatus() {
	hint := keyHints
	if e.status != "" {
		hint = tagBar + " " + tview.Escape(e.status)
	}
	if e.taggedStatus != "" {
		hint = tagBar + " " + e.taggedStatus
	}
	e.statusKeys.SetText(hint)

	right := tagBar
	if b := e.buf(); b != nil {
		row, col, _, _ := b.area.GetCursor()
		flag := " "
		if b.dirty {
			flag = "*"
		}
		right += fmt.Sprintf("%s L%d C%d ", flag, row+1, col+1)
	}
	e.statusRight.SetText(right)
}

// setStatus shows a message on the status line until the next keystroke.
func (e *Editor) setStatus(msg string) {
	e.status = msg
	e.taggedStatus = ""
	e.refreshStatus()
}

// setTaggedStatus shows a message that carries its own colour tags, which is
// how the parameter hint picks out the argument being typed.
func (e *Editor) setTaggedStatus(text string) {
	e.taggedStatus = text
	e.refreshStatus()
}

func (e *Editor) clearTaggedStatus() {
	if e.taggedStatus != "" {
		e.taggedStatus = ""
		e.refreshStatus()
	}
}

// clearStatus puts the function key hints back. The parameter hint stays: it
// belongs to the call being typed, not to the last key pressed.
func (e *Editor) clearStatus() {
	if e.status != "" {
		e.status = ""
		e.refreshStatus()
	}
}

/* --- keys --- */

// handleKey is the editor's global key map. Dialogs and menus get first refusal
// so that typing in a dialog cannot trip a function key.
func (e *Editor) handleKey(event *tcell.EventKey) *tcell.EventKey {
	if e.modals > 0 || e.openMenu >= 0 {
		return event
	}
	e.clearStatus()

	if event.Modifiers()&tcell.ModAlt != 0 {
		if k := e.handleAltKey(event); k == nil {
			return nil
		}
	}

	switch event.Key() {
	case tcell.KeyF2:
		e.saveCurrent()
		return nil
	case tcell.KeyF3:
		e.openDialog()
		return nil
	case tcell.KeyF4:
		e.toggleOutput()
		return nil
	case tcell.KeyF5:
		e.runPrimary()
		return nil
	case tcell.KeyF6:
		e.nextBuffer()
		return nil
	case tcell.KeyF9:
		e.checkCurrent()
		return nil
	case tcell.KeyF10:
		e.openMenuAt(0)
		return nil
	case tcell.KeyCtrlN:
		e.newFile()
		return nil
	case tcell.KeyCtrlO:
		e.openDialog()
		return nil
	case tcell.KeyCtrlS:
		e.saveCurrent()
		return nil
	case tcell.KeyCtrlG:
		e.gotoDialog()
		return nil
	case tcell.KeyCtrlF:
		// tview's text area pages down on Ctrl-F; PgDn still does, and Ctrl-F
		// is where a search belongs.
		e.findDialog()
		return nil
	case tcell.KeyCtrlR:
		e.replaceDialog()
		return nil
	case tcell.KeyF7:
		e.findNext()
		return nil
	case tcell.KeyF8:
		e.findPrevious()
		return nil
	case tcell.KeyF12:
		e.formatCurrent()
		return nil
	case tcell.KeyCtrlB:
		// tview's text area pages up on Ctrl-B; PgUp still does.
		e.toggleComment()
		return nil
	case tcell.KeyCtrlSpace:
		// Ctrl-Space arrives as NUL, which is the same key in tcell's book.
		e.complete()
		return nil
	case tcell.KeyF11:
		e.hover()
		return nil
	case tcell.KeyCtrlP:
		e.signatureHelp(false)
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if e.completing && e.textHasFocus() {
			e.sendKeyToText(event)
			e.completeAutomatically()
			return nil
		}
		return event
	case tcell.KeyF1:
		// Turbo Pascal put help on the word under the cursor on Ctrl-F1, and
		// terminals that send it get that here too.
		if event.Modifiers()&tcell.ModCtrl != 0 {
			e.hover()
			return nil
		}
		e.showHelp()
		return nil
	case tcell.KeyCtrlC:
		// Not "quit": in an IDE this is what stops the running program.
		e.stopProgram()
		return nil
	case tcell.KeyEscape:
		// Escape puts the parameter hint away, and otherwise comes back from
		// the output pane to the text.
		if e.signatureShown {
			e.hideSignature()
			return nil
		}
		if b := e.buf(); b != nil {
			e.app.SetFocus(b.area)
		}
		return nil

	case tcell.KeyRune:
		// A "." or a ":" asks the language server what comes next. The character
		// has to be in the text before the question is asked, because the answer
		// depends on the cursor being after it, so it goes in here rather than
		// being left to tview: queueing the request instead would deadlock,
		// since tview's QueueUpdate waits for the work it queues and this is
		// the goroutine that would have to do it.
		if e.textHasFocus() {
			switch {
			case e.isTriggerCharacter(event.Rune()):
				e.sendKeyToText(event)
				e.completeAutomatically()
				return nil
			case e.isSignatureTrigger(event.Rune()):
				e.completing = false // a bracket or comma ends a word
				e.sendKeyToText(event)
				e.signatureHelpAsync()
				return nil
			case e.completing && isNameRune(event.Rune()):
				// The word being completed is still being typed: ask again, so
				// the list follows it.
				e.sendKeyToText(event)
				e.completeAutomatically()
				return nil
			}
			e.completing = false // whatever this is, the word has ended
		}
		return event
	}
	return event
}

// textHasFocus reports whether the keyboard belongs to the buffer being edited,
// rather than to the output pane.
func (e *Editor) textHasFocus() bool {
	b := e.buf()
	return b != nil && b.area.HasFocus()
}

// sendKeyToText hands a key to the buffer being edited, which is how a popup
// lets typing carry on underneath it.
func (e *Editor) sendKeyToText(event *tcell.EventKey) {
	b := e.buf()
	if b == nil {
		return
	}
	e.app.SetFocus(b.area)
	if handler := b.area.InputHandler(); handler != nil {
		handler(event, func(tview.Primitive) {})
	}
}

// handleAltKey covers the Alt combinations: menu hotkeys, Alt-1..9 to pick a
// buffer, Alt-F3 to close one, Alt-X to leave.
func (e *Editor) handleAltKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyF3:
		e.closeCurrent()
		return nil
	case tcell.KeyF2:
		e.outlineDialog()
		return nil
	}
	r := event.Rune()
	if r >= '1' && r <= '9' {
		e.selectBuffer(int(r - '1'))
		return nil
	}
	switch r {
	case 'x', 'X':
		e.quit()
		return nil
	}
	for i, m := range e.menus {
		if r == m.hotkey || r == m.hotkey+32 {
			e.openMenuAt(i)
			return nil
		}
	}
	return event
}

/* --- file commands --- */

func (e *Editor) saveCurrent() {
	b := e.buf()
	if b == nil {
		return
	}
	if b.path == "" {
		e.saveAsDialog()
		return
	}
	if err := e.save(b); err != nil {
		e.message("Save", err.Error())
		return
	}
	e.setStatus("Saved " + b.name)
}

func (e *Editor) closeCurrent() {
	b := e.buf()
	if b == nil {
		return
	}
	if b.dirty {
		e.confirm("Close", fmt.Sprintf("%s has unsaved changes. Close it anyway?", b.name), func() {
			e.closeBuffer(b)
		})
		return
	}
	e.closeBuffer(b)
}

// setPrimary marks the current buffer as the one F5 runs.
func (e *Editor) setPrimary() {
	b := e.buf()
	if b == nil {
		return
	}
	if b.path == "" {
		e.message("Primary file", "Save this buffer first; a file has to exist on disk to be run.")
		return
	}
	e.primary = b
	e.refreshTabs()
	e.setStatus("F5 now runs " + b.name)
}

func (e *Editor) quit() {
	dirty := []string{}
	for _, b := range e.buffers {
		if b.dirty {
			dirty = append(dirty, b.name)
		}
	}
	if len(dirty) > 0 {
		e.confirm("Exit", "Unsaved changes in "+strings.Join(dirty, ", ")+". Leave anyway?", func() {
			e.leaving = true
			e.app.Stop()
		})
		return
	}
	e.leaving = true
	e.app.Stop()
}
