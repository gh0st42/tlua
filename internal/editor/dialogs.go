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

// showModal floats a dialog over the desktop and gives it the keyboard.
func (e *Editor) showModal(name string, p tview.Primitive, width, height int) {
	e.modals++
	e.modalStack = append(e.modalStack, p)
	e.pages.AddPage(name, center(p, width, height), true, true)
	e.app.SetFocus(p)
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

// message states something and waits for the user to acknowledge it.
func (e *Editor) message(title, text string) {
	const name = "message"
	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(int, string) { e.closeModal(name) })
	modal.SetBackgroundColor(egaLightGray)
	modal.SetTextColor(egaBlack)
	modal.SetButtonBackgroundColor(egaGreen)
	modal.SetButtonTextColor(egaBlack)
	modal.SetBorder(true)
	modal.SetBorderColor(egaBlack)
	modal.SetTitle(" " + title + " ")
	modal.SetTitleColor(egaBlack)

	e.modals++
	e.modalStack = append(e.modalStack, modal)
	e.pages.AddPage(name, modal, true, true)
	e.app.SetFocus(modal)
}

// confirm asks a yes or no question, running onYes only on yes.
func (e *Editor) confirm(title, text string, onYes func()) {
	const name = "confirm"
	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(_ int, label string) {
			e.closeModal(name)
			if label == "Yes" {
				onYes()
			}
		})
	modal.SetBackgroundColor(egaLightGray)
	modal.SetTextColor(egaBlack)
	modal.SetButtonBackgroundColor(egaGreen)
	modal.SetButtonTextColor(egaBlack)
	modal.SetBorder(true)
	modal.SetBorderColor(egaBlack)
	modal.SetTitle(" " + title + " ")
	modal.SetTitleColor(egaBlack)

	e.modals++
	e.modalStack = append(e.modalStack, modal)
	e.pages.AddPage(name, modal, true, true)
	e.app.SetFocus(modal)
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
		if err := e.saveAs(b, path); err != nil {
			e.message("Save as", err.Error())
			return
		}
		e.setStatus("Saved " + b.name)
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
	text := tview.NewTextView().SetDynamicColors(true)
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
		"  Ctrl-F      find                   F10  menu bar",
		"  Ctrl-R      replace                Alt-1..9  pick a buffer",
		"  Ctrl-G      go to line             Alt-F2    list functions",
		"  Ctrl-C      stop the program       Alt-F3    close buffer",
		"                                     Alt-X     leave",
		"",
		"  In the text: Ctrl-Z undo, Ctrl-Y redo, Ctrl-Q copy,",
		"  Ctrl-X cut, Ctrl-V paste, Ctrl-L select all. PgUp and",
		"  PgDn page; Ctrl-F searches instead.",
		"",
		"  Alt-F, Alt-E, Alt-S, Alt-R, Alt-W, Alt-H open the menus.",
		"  The mouse works: click the bars, the text and the menus.",
		"  The file marked » in the buffer bar is the one F5 runs;",
		"  set it from the Run menu.",
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
	e.showModal(name, text, 68, 24)
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

// at places a primitive at a fixed row and column, which is how a dropdown
// lines up under its menu title.
func at(p tview.Primitive, col, row, width, height int) tview.Primitive {
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
