package editor

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// menuItem is one line of a dropdown. A separator is drawn as a rule and
// cannot be chosen.
type menuItem struct {
	label     string
	shortcut  string
	action    func()
	separator bool
}

// menu is one title on the menu bar together with what drops down from it.
type menu struct {
	title  string
	hotkey rune // the letter shown in red, reached with Alt
	col    int  // where on the bar the title starts, for lining up the dropdown
	items  []menuItem
}

// buildMenus lays out the menu bar. The arrangement follows the DOS IDEs:
// File, Edit, Run, Window, Help.
func (e *Editor) buildMenus() []*menu {
	menus := []*menu{
		{title: "File", hotkey: 'F', items: []menuItem{
			{label: "New", shortcut: "Ctrl-N", action: e.newFile},
			{label: "Open...", shortcut: "F3", action: e.openDialog},
			{label: "Save", shortcut: "F2", action: e.saveCurrent},
			{label: "Save as...", action: e.saveAsDialog},
			{separator: true},
			{label: "Close", shortcut: "Alt-F3", action: e.closeCurrent},
			{label: "Exit", shortcut: "Alt-X", action: e.quit},
		}},
		{title: "Edit", hotkey: 'E', items: []menuItem{
			{label: "Undo", shortcut: "Ctrl-Z", action: func() { e.sendToText(tcell.KeyCtrlZ) }},
			{label: "Redo", shortcut: "Ctrl-Y", action: func() { e.sendToText(tcell.KeyCtrlY) }},
			{separator: true},
			{label: "Cut", shortcut: "Ctrl-X", action: func() { e.sendToText(tcell.KeyCtrlX) }},
			{label: "Copy", shortcut: "Ctrl-Q", action: func() { e.sendToText(tcell.KeyCtrlQ) }},
			{label: "Paste", shortcut: "Ctrl-V", action: func() { e.sendToText(tcell.KeyCtrlV) }},
			{label: "Select all", shortcut: "Ctrl-L", action: func() { e.sendToText(tcell.KeyCtrlL) }},
			{separator: true},
			{label: "Complete", shortcut: "Ctrl-Space", action: e.complete},
			{label: "Toggle comment", shortcut: "Ctrl-B", action: e.toggleComment},
			{label: "Format document", shortcut: "F12", action: e.formatCurrent},
			{label: e.formatOnSaveLabel(), action: e.toggleFormatOnSave},
			{label: "Language server...", action: e.languageServerDialog},
		}},
		{title: "Search", hotkey: 'S', items: []menuItem{
			{label: "Find...", shortcut: "Ctrl-F", action: e.findDialog},
			{label: "Find next", shortcut: "F7", action: e.findNext},
			{label: "Find previous", shortcut: "F8", action: e.findPrevious},
			{separator: true},
			{label: "Replace...", shortcut: "Ctrl-R", action: e.replaceDialog},
			{label: "Replace all", action: e.replaceAll},
			{separator: true},
			{label: "Go to line...", shortcut: "Ctrl-G", action: e.gotoDialog},
			{label: "Functions...", shortcut: "Alt-F2", action: e.outlineDialog},
		}},
		{title: "Run", hotkey: 'R', items: []menuItem{
			{label: "Run", shortcut: "F5", action: e.runPrimary},
			{label: "Run this buffer", action: e.runCurrent},
			{label: "Stop program", shortcut: "Ctrl-C", action: e.stopProgram},
			{separator: true},
			{label: "Check syntax", shortcut: "F9", action: e.checkCurrent},
			{label: "Set as primary file", action: e.setPrimary},
			{separator: true},
			{label: "Show output", shortcut: "F4", action: e.toggleOutput},
			{label: "Clear output", action: e.clearOutput},
		}},
		{title: "Window", hotkey: 'W', items: []menuItem{
			{label: "Next", shortcut: "F6", action: e.nextBuffer},
			{label: "Previous", action: e.prevBuffer},
			{separator: true},
			{label: "Functions...", shortcut: "Alt-F2", action: e.outlineDialog},
			{label: "List buffers...", action: e.bufferListDialog},
		}},
		{title: "Help", hotkey: 'H', items: []menuItem{
			{label: "Keys", shortcut: "F1", action: e.showHelp},
			{label: "What is this?", shortcut: "F11", action: e.hover},
			{label: "About", action: e.showAbout},
		}},
	}

	// The bar is " File  Edit  Run ...": remember where each title begins so a
	// dropdown can open underneath it.
	col := 1
	for _, m := range menus {
		m.col = col
		col += len(m.title) + 2
	}
	return menus
}

// drawMenuBar paints the titles, with the open one highlighted.
func (e *Editor) drawMenuBar() {
	var sb strings.Builder
	sb.WriteString(tagBar + " ")
	for i, m := range e.menus {
		title := m.title
		hot := strings.IndexRune(title, m.hotkey)
		switch {
		case i == e.openMenu:
			fmt.Fprintf(&sb, "[black:green]%s[black:silver]  ", title)
		case hot >= 0:
			fmt.Fprintf(&sb, "%s%s%s%c%s%s  ", tagBar, title[:hot],
				tagHotkey, m.hotkey, tagBar, title[hot+1:])
		default:
			fmt.Fprintf(&sb, "%s%s  ", tagBar, title)
		}
	}
	e.menubar.SetText(sb.String())
}

// openMenuAt drops down one menu and keeps the keyboard until it closes.
func (e *Editor) openMenuAt(i int) {
	if i < 0 || i >= len(e.menus) {
		return
	}
	e.closeMenu()
	e.openMenu = i
	m := e.menus[i]
	e.drawMenuBar()

	list := newClickList()
	list.SetBorder(true)
	list.SetBorderColor(egaBlack)

	width := 0
	for _, item := range m.items {
		if w := len(item.label) + len(item.shortcut) + 6; w > width {
			width = w
		}
	}

	for _, item := range m.items {
		if item.separator {
			list.AddItem(strings.Repeat("─", width-2), "", 0, nil)
			continue
		}
		label := item.label
		if item.shortcut != "" {
			label = fmt.Sprintf("%-*s%s", width-4-len(item.shortcut), item.label, item.shortcut)
		}
		action := item.action
		list.AddItem(label, "", 0, func() {
			e.closeMenu()
			if action != nil {
				action()
			}
		})
	}
	// Never start on a separator.
	if len(m.items) > 0 && m.items[0].separator {
		list.SetCurrentItem(1)
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			e.closeMenu()
			return nil
		case tcell.KeyLeft:
			e.openMenuAt((e.openMenu - 1 + len(e.menus)) % len(e.menus))
			return nil
		case tcell.KeyRight:
			e.openMenuAt((e.openMenu + 1) % len(e.menus))
			return nil
		case tcell.KeyDown, tcell.KeyUp:
			// Step over the separators.
			cur := list.GetCurrentItem()
			delta := 1
			if event.Key() == tcell.KeyUp {
				delta = -1
			}
			n := list.GetItemCount()
			for i := 1; i <= n; i++ {
				next := (cur + delta*i + n) % n
				if !m.items[next].separator {
					list.SetCurrentItem(next)
					break
				}
			}
			return nil
		}
		return event
	})

	height := len(m.items) + 2
	e.menuList = list
	e.pages.AddPage("menu", at2(list, m.col, 1, width, height), true, true)
	e.app.SetFocus(list)
}

// closeMenu puts the dropdown away.
func (e *Editor) closeMenu() {
	if e.openMenu < 0 {
		return
	}
	e.openMenu = -1
	e.menuList = nil
	e.pages.RemovePage("menu")
	e.drawMenuBar()
	if e.modals == 0 {
		if b := e.buf(); b != nil {
			e.app.SetFocus(b.area)
		}
	}
}

// sendToText forwards an editing key to the text, so the Edit menu drives the
// same undo, clipboard and selection handling as the keyboard.
func (e *Editor) sendToText(key tcell.Key) {
	b := e.buf()
	if b == nil {
		return
	}
	e.app.SetFocus(b.area)
	if handler := b.area.InputHandler(); handler != nil {
		handler(tcell.NewEventKey(key, 0, tcell.ModNone), func(tview.Primitive) {})
	}
}

// bufferListDialog is the Window menu's list of everything open.
func (e *Editor) bufferListDialog() {
	const name = "buffers"
	list := newClickList()
	list.SetBackgroundColor(egaLightGray)
	list.SetMainTextColor(egaBlack)
	list.SetSelectedTextColor(egaBlack)
	list.SetSelectedBackgroundColor(egaGreen)
	dialogColors(list.Box)
	list.SetTitle(" Buffers ")

	for i, b := range e.buffers {
		idx := i
		label := fmt.Sprintf("%d  %s", i+1, e.title(b))
		if b.path != "" {
			label = fmt.Sprintf("%d  %-24s %s", i+1, e.title(b), b.path)
		}
		list.AddItem(label, "", 0, func() {
			e.closeModal(name)
			e.selectBuffer(idx)
		})
	}
	list.SetCurrentItem(e.current)
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			e.closeModal(name)
			return nil
		}
		return event
	})
	e.showModal(name, list, 70, len(e.buffers)+2)
}

func (e *Editor) showAbout() {
	e.message("About", "tlua editor\n\nA Lua editor in the DOS IDE tradition,\nbuilt with tview on a pure Go interpreter.")
}
