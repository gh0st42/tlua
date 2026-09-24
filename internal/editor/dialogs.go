package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// showModal floats a dialog over the desktop and gives it the keyboard. A dialog
// never asks for more room than the terminal has: on a small screen it is cut
// down to fit rather than hanging off the edge, and anything with more to say
// than fits can be scrolled.
func (e *Editor) showModal(name string, p tview.Primitive, width, height int) {
	width, height = e.fit(width, height)
	e.modals++
	e.modalStack = append(e.modalStack, p)
	e.pages.AddPage(name, center(p, width, height), true, true)
	e.app.SetFocus(p)
}

// fit trims a dialog's size to what the screen can hold, leaving the bars around
// it visible.
func (e *Editor) fit(width, height int) (int, int) {
	screenWidth, screenHeight := e.screenSize()
	if width > screenWidth-2 {
		width = screenWidth - 2
	}
	if height > screenHeight-2 {
		height = screenHeight - 2
	}
	if width < 10 {
		width = 10
	}
	if height < 3 {
		height = 3
	}
	return width, height
}

// closeModal takes a dialog back down and returns to the text.
func (e *Editor) closeModal(name string) {
	e.pages.RemovePage(name)
	if e.modals > 0 {
		e.modals--
	}
	if n := len(e.modalStack); n > 0 {
		e.modalStack = e.modalStack[:n-1]
	}
	if e.modals == 0 && e.openMenu < 0 {
		if b := e.buf(); b != nil {
			e.app.SetFocus(b.area)
		}
	}
}

// dialog is the shape every question in this editor takes: something to read, a
// row of buttons, and a size the screen can hold. tview has a Modal that does
// much the same, but it grows to fit its text however long that is, which on a
// 25 row terminal means a box drawn off the top of the screen.
func (e *Editor) dialog(name, title, text string, buttons []string, done func(label string)) {
	body := tview.NewTextView().SetWrap(true).SetScrollable(true)
	body.SetBackgroundColor(egaLightGray)
	body.SetTextStyle(dialogTextStyle)
	body.SetText(text)

	form := tview.NewForm()
	form.SetButtonsAlign(tview.AlignCenter)
	for _, label := range buttons {
		chosen := label
		form.AddButton(chosen, func() {
			e.closeModal(name)
			if done != nil {
				done(chosen)
			}
		})
	}
	e.styleForm(form, title)
	form.SetBorder(false)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			e.closeModal(name)
			if done != nil {
				done("") // leaving is the same as choosing nothing
			}
			return nil
		}
		return event
	})

	frame := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(body, 0, 1, false).
		AddItem(form, 3, 0, true)
	frame.SetBackgroundColor(egaLightGray)
	dialogColors(frame.Box)
	frame.SetTitle(" " + title + " ")

	const width = 60
	height := strings.Count(text, "\n") + len(text)/width + 6
	e.showModal(name, frame, width, height)
	e.app.SetFocus(form)
}

// message states something and waits for the user to acknowledge it.
func (e *Editor) message(title, text string) {
	e.dialog("message", title, text, []string{"OK"}, nil)
}

// confirm asks a yes or no question, running onYes only on yes.
func (e *Editor) confirm(title, text string, onYes func()) {
	e.dialog("confirm", title, text, []string{"Yes", "No"}, func(label string) {
		if label == "Yes" {
			onYes()
		}
	})
}

// prompt asks for one line of text.
func (e *Editor) prompt(title, label, initial string, onDone func(string)) {
	const name = "prompt"
	form := tview.NewForm()
	form.AddInputField(label, initial, 40, nil, nil)
	accept := func() {
		value := strings.TrimSpace(form.GetFormItem(0).(*tview.InputField).GetText())
		e.closeModal(name)
		if value != "" {
			onDone(value)
		}
	}
	form.AddButton("OK", accept)
	form.AddButton("Cancel", func() { e.closeModal(name) })
	e.styleForm(form, title)
	form.SetInputCapture(dialogKeys(e, name, form, accept))
	e.showModal(name, form, 60, 7)
}

// gotoDialog jumps the cursor to a line number.
func (e *Editor) gotoDialog() {
	b := e.buf()
	if b == nil {
		return
	}
	e.prompt("Go to line", "Line: ", "", func(value string) {
		n, err := strconv.Atoi(value)
		if err != nil {
			e.message("Go to line", fmt.Sprintf("%q is not a line number", value))
			return
		}
		e.gotoLine(b, n)
	})
}

// saveAsDialog names a buffer and writes it.
func (e *Editor) saveAsDialog() {
	b := e.buf()
	if b == nil {
		return
	}
	initial := b.path
	if initial == "" {
		wd, _ := os.Getwd()
		initial = filepath.Join(wd, b.name)
	}
	e.prompt("Save as", "File: ", initial, func(path string) {
		path = expandHome(path)
		write := func() {
			if err := e.saveAs(b, path); err != nil {
				e.message("Save as", err.Error())
				return
			}
			e.setStatus("Saved " + b.name)
		}
		// Writing over somebody else's file is worth a question.
		if abs, err := filepath.Abs(path); err == nil && abs != b.path {
			if st, err := os.Stat(abs); err == nil && !st.IsDir() {
				e.confirm("Save as", displayName(abs)+" exists. Write over it?", write)
				return
			}
		}
		write()
	})
}

// openDialog is a small file browser: type a path, or walk the listing.
func (e *Editor) openDialog() {
	const name = "open"
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}

	path := tview.NewInputField().SetLabel("File: ").SetText("")
	path.SetLabelStyle(dialogTextStyle)
	path.SetFieldStyle(dialogFieldStyle)

	list := newClickList()

	dir := wd
	var reload func(string)
	reload = func(to string) {
		dir = to
		list.Clear()
		path.SetLabel(fmt.Sprintf("File (%s): ", displayName(dir)))
		for _, entry := range readDir(dir) {
			target := filepath.Join(dir, entry.name)
			if entry.isDir {
				list.AddItem(entry.name+"/", "", 0, func() { reload(target) })
				continue
			}
			list.AddItem(entry.name, "", 0, func() {
				e.closeModal(name)
				if err := e.openFile(target); err != nil {
					e.message("Open", err.Error())
				}
			})
		}
	}
	reload(wd)

	accept := func() {
		typed := strings.TrimSpace(path.GetText())
		if typed == "" {
			return
		}
		typed = expandHome(typed)
		if !filepath.IsAbs(typed) {
			typed = filepath.Join(dir, typed)
		}
		e.closeModal(name)
		if err := e.openFile(typed); err != nil {
			e.message("Open", err.Error())
		}
	}
	path.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			accept()
		case tcell.KeyTab:
			e.app.SetFocus(list)
		}
	})

	frame := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(path, 1, 0, true).
		AddItem(list, 0, 1, false)
	frame.SetBackgroundColor(egaLightGray)
	dialogColors(frame.Box)
	frame.SetTitle(" Open file ")

	frame.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			e.closeModal(name)
			return nil
		case tcell.KeyTab, tcell.KeyBacktab:
			if path.HasFocus() {
				e.app.SetFocus(list)
			} else {
				e.app.SetFocus(path)
			}
			return nil
		}
		return event
	})

	e.showModal(name, frame, 60, 18)
}

// expandHome turns a leading ~ into the home directory, since a path typed by
// hand often starts with one.
func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), "/"))
}

type dirEntry struct {
	name  string
	isDir bool
}

// readDir lists a directory the way a DOS file dialog did: directories first,
// then files, with the parent directory at the top and dot files left out.
func readDir(dir string) []dirEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	dirs, files := []dirEntry{}, []dirEntry{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if entry.IsDir() {
			dirs = append(dirs, dirEntry{entry.Name(), true})
		} else {
			files = append(files, dirEntry{entry.Name(), false})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].name < dirs[j].name })
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })

	out := []dirEntry{{"..", true}}
	return append(append(out, dirs...), files...)
}

// showHelp lists the keys, which is what F1 was for.
func (e *Editor) showHelp() {
	const name = "help"
	text := tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	text.SetBackgroundColor(egaLightGray)
	text.SetTextStyle(dialogTextStyle)
	text.SetText(strings.Join([]string{
		"[black]tlua editor",
		"",
		"  F1          this help              F5   run the » file",
		"  F2, Ctrl-S  save                   F6   next buffer",
		"  F3, Ctrl-O  open a file            F7   find next",
		"  F4          show or hide output    F8   find previous",
		"  Ctrl-N      new file               F9   check syntax",
		"  Ctrl-B      comment or uncomment   F10  menu bar",
		"  Ctrl-Space  complete               F11  what is under the cursor",
		"  Ctrl-P      parameters of a call   F12  format (language server)",
		"  Ctrl-F      find                   Alt-1..9  pick a buffer",
		"  Ctrl-R      replace                Alt-F2    list functions",
		"  Ctrl-G      go to line             Alt-F3    close buffer",
		"  Ctrl-C      stop the program       Alt-X     leave",
		"",
		"  Alt-F7, Alt-F8  the previous and next thing the language",
		"                  server complained about; Alt-F9 lists them.",
		"",
		"  In the text: Ctrl-Z undo, Ctrl-Y redo, Ctrl-Q copy,",
		"  Ctrl-X cut, Ctrl-V paste, Ctrl-L select all. PgUp and",
		"  PgDn page; Ctrl-F and Ctrl-B take those keys.",
		"",
		"  Alt-F, Alt-E, Alt-S, Alt-R, Alt-W, Alt-H open the menus.",
		"  The mouse works: click the bars, the text and the menus.",
		"  The file marked » in the buffer bar is the one F5 runs;",
		"  set it from the Run menu. A ! or ? after a name means the",
		"  server found something wrong with that file.",
	}, "\n"))
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
	e.showModal(name, text, 68, 28)
}

func (e *Editor) styleForm(form *tview.Form, title string) {
	form.SetBackgroundColor(egaLightGray)
	form.SetLabelColor(egaBlack)
	form.SetFieldStyle(dialogFieldStyle)
	form.SetButtonStyle(dialogButtonStyle)
	form.SetButtonActivatedStyle(dialogButtonFocusedStyle)
	dialogColors(form.Box)
	form.SetTitle(" " + title + " ")
	// Form items draw their own labels and fill their own row, and tview only
	// lets the form pass colours down, keeping the theme's blue behind them;
	// restyle each one directly.
	for i := 0; i < form.GetFormItemCount(); i++ {
		switch item := form.GetFormItem(i).(type) {
		case *tview.InputField:
			item.SetBackgroundColor(egaLightGray)
			item.SetLabelStyle(dialogTextStyle)
			item.SetFieldStyle(dialogFieldStyle)
		case *tview.Checkbox:
			item.SetBackgroundColor(egaLightGray)
			item.SetLabelStyle(dialogTextStyle)
			item.SetUncheckedStyle(dialogFieldStyle)
			item.SetCheckedStyle(dialogFieldStyle)
			item.SetActivatedStyle(dialogFieldStyle)
		}
	}
}

// center places a primitive in the middle of the screen at a fixed size, over
// an overlay that keeps the mouse out of the desktop behind it.
func center(p tview.Primitive, width, height int) tview.Primitive {
	return &blocker{Flex: tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 0, true).
			AddItem(nil, 0, 1, false), width, 0, true).
		AddItem(nil, 0, 1, false)}
}

// place puts a primitive at a fixed row and column without blocking anything:
// it is for hints, which take no focus and swallow no clicks.
func place(p tview.Primitive, col, row, width, height int) tview.Primitive {
	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, row, 0, false).
		AddItem(tview.NewFlex().
			AddItem(nil, col, 0, false).
			AddItem(p, width, 0, false).
			AddItem(nil, 0, 1, false), height, 0, false).
		AddItem(nil, 0, 1, false)
}

// at places a primitive at a fixed row and column, which is how a dropdown
// lines up under its menu title and a completion list under the cursor.
func at2(p tview.Primitive, col, row, width, height int) tview.Primitive {
	return &blocker{Flex: tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, row, 0, false).
		AddItem(tview.NewFlex().
			AddItem(nil, col, 0, false).
			AddItem(p, width, 0, true).
			AddItem(nil, 0, 1, false), height, 0, true).
		AddItem(nil, 0, 1, false)}
}

// outlineDialog lists the functions in the current buffer and jumps to the one
// chosen, the way QBasic's F2 listed SUBs and RHIDE's Alt-F2 listed functions.
// The file itself is the first entry and the end of the file the last, so the
// list navigates a file as well as its functions.
func (e *Editor) outlineDialog() {
	b := e.buf()
	if b == nil {
		return
	}
	text := b.area.GetText()
	lastLine := strings.Count(text, "\n") + 1

	type row struct {
		label string
		line  int
	}
	// The root: the file, at its first line.
	rows := []row{{fmt.Sprintf("[black::b]%5d  %s", 1, tview.Escape(b.name)), 1}}
	for _, entry := range outline(text) {
		indent := strings.Repeat("  ", entry.Depth)
		rows = append(rows, row{
			fmt.Sprintf("[black]%5d  %s%s%s", entry.Line, indent,
				outlineTags[entry.Kind], tview.Escape(entry.Name)),
			entry.Line,
		})
	}
	rows = append(rows, row{fmt.Sprintf("[gray]%5d  (end of file)", lastLine), lastLine})

	const name = "outline"
	list := newClickList()
	list.SetUseStyleTags(true, false)

	// Open on whatever the cursor is inside, so the list answers "where am I"
	// as well as "where do I want to go".
	cursorRow, _, _, _ := b.area.GetCursor()
	current := 0

	for i, r := range rows {
		if r.line <= cursorRow+1 && i < len(rows)-1 {
			current = i
		}
		line := r.line
		list.AddItem(r.label, "", 0, func() {
			e.closeModal(name)
			e.gotoLine(b, line)
			e.setStatus(fmt.Sprintf("%s line %d", b.name, line))
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

	legend := tview.NewTextView().SetDynamicColors(true)
	legend.SetBackgroundColor(egaLightGray)
	legend.SetTextStyle(dialogTextStyle)
	legend.SetText(fmt.Sprintf(" %sglobal  %slocal  %snested", outlineTags[kindGlobal],
		outlineTags[kindLocal], outlineTags[kindNested]))

	// The border belongs to the frame, so the legend sits inside the panel with
	// the list rather than floating under it.
	frame := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(legend, 1, 0, false)
	frame.SetBackgroundColor(egaLightGray)
	dialogColors(frame.Box)
	frame.SetTitle(fmt.Sprintf(" Functions in %s ", b.name))

	height := len(rows) + 3 // the border and the legend
	if height > 20 {
		height = 20
	}
	e.showModal(name, frame, 64, height)
}

// point is a place on the screen.
type point struct{ col, row int }

// cursorScreenPosition is where the cursor is drawn, which is where a
// completion list should appear.
func (e *Editor) cursorScreenPosition(b *buffer) point {
	x, y, _, _ := b.area.GetInnerRect()
	row, column, _, _ := b.area.GetCursor()
	rowOffset, columnOffset := b.area.GetOffset()
	return point{col: x + column - columnOffset, row: y + row - rowOffset}
}

// showAt floats a dialog just below a point, nudged back onto the screen when
// it would hang off an edge, the way a completion list has to behave near the
// bottom or the right of a window.
func (e *Editor) showAt(name string, p tview.Primitive, at point, width, height int) {
	screenWidth, screenHeight := e.screenSize()

	col, row := at.col, at.row+1
	if col+width > screenWidth {
		col = screenWidth - width
	}
	if col < 0 {
		col = 0
	}
	if row+height > screenHeight {
		// No room below: put it above the cursor instead.
		row = at.row - height
	}
	if row < 0 {
		row = 0
	}

	e.modals++
	e.modalStack = append(e.modalStack, p)
	e.pages.AddPage(name, at2(p, col, row, width, height), true, true)
	e.app.SetFocus(p)
}

// screenSize is how much room there is to place things in.
func (e *Editor) screenSize() (width, height int) {
	_, _, width, height = e.pages.GetRect()
	if width <= 0 || height <= 0 {
		return 80, 25 // before the first draw
	}
	return width, height
}
