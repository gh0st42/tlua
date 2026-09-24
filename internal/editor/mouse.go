package editor

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// clickableBar is a one-line text view that reports the column that was
// clicked, which is how the menu bar and the buffer bar answer the mouse.
type clickableBar struct {
	*tview.TextView
	onClick func(column int)
}

func newClickableBar(onClick func(column int)) *clickableBar {
	bar := &clickableBar{TextView: tview.NewTextView(), onClick: onClick}
	bar.SetDynamicColors(true).SetWrap(false)
	return bar
}

func (b *clickableBar) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return b.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		x, y := event.Position()
		if !b.InRect(x, y) {
			return false, nil
		}
		if isClick(action) && b.onClick != nil {
			innerX, _, _, _ := b.GetInnerRect()
			b.onClick(x - innerX)
		}
		return true, nil
	})
}

// clickList is tview's list with double clicks treated as clicks. tview turns
// a second click inside half a second into a double click, which its list
// ignores; in a menu or a function list that reads as the item not reacting.
type clickList struct {
	*tview.List
}

func newClickList() *clickList {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBackgroundColor(egaLightGray)
	// tview builds its text styles from the theme's primitive background, which
	// here is the blue desktop, and prints without keeping what is underneath.
	// Setting only the foreground would leave a blue stripe behind every label
	// on a grey panel, so set the whole style.
	list.SetMainTextStyle(dialogTextStyle)
	list.SetSecondaryTextStyle(dialogTextStyle)
	list.SetShortcutStyle(dialogTextStyle)
	list.SetSelectedStyle(dialogSelectedStyle)
	list.SetHighlightFullLine(true)
	return &clickList{List: list}
}

func (l *clickList) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	handler := l.List.MouseHandler()
	return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		x, y := event.Position()
		if !l.InRect(x, y) {
			return false, nil
		}
		if action == tview.MouseLeftDoubleClick {
			action = tview.MouseLeftClick
		}
		consumed, capture := handler(action, event, setFocus)
		if consumed {
			return consumed, capture
		}
		// tview's list only answers clicks and scrolling. Anything else that
		// lands on it has to stop here anyway: a press falling through to the
		// text behind a dropdown would move the cursor there and, worse, let
		// that text capture the release, so the item never fires.
		return true, nil
	}
}

// isClick covers both the first click and the one that follows it quickly.
func isClick(action tview.MouseAction) bool {
	return action == tview.MouseLeftClick || action == tview.MouseLeftDoubleClick
}

// menuAt reports which menu title covers a column of the menu bar.
func (e *Editor) menuAt(column int) int {
	for i, m := range e.menus {
		if column >= m.col-1 && column < m.col+len(m.title)+1 {
			return i
		}
	}
	return -1
}

// clickMenuBar opens, switches or closes a menu, the way a mouse expects.
func (e *Editor) clickMenuBar(column int) {
	switch i := e.menuAt(column); {
	case i < 0:
		e.closeMenu()
	case i == e.openMenu:
		e.closeMenu()
	default:
		e.openMenuAt(i)
	}
}

// clickTabs switches to whichever buffer's name was clicked.
func (e *Editor) clickTabs(column int) {
	for _, span := range e.tabSpans {
		if column >= span.from && column < span.to {
			e.selectBuffer(span.buffer)
			return
		}
	}
}

// tabSpan records where a buffer's name sits on the buffer bar.
type tabSpan struct {
	from, to int
	buffer   int
}

// captureMouse handles the clicks that belong to the editor as a whole rather
// than to one widget: dismissing an open menu, and keeping a dialog modal.
func (e *Editor) captureMouse(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	// Presses and clicks are what dismiss things; a quick second click arrives
	// as a double click and has to count too.
	if event == nil || !(action == tview.MouseLeftDown || isClick(action)) {
		return event, action
	}
	x, y := event.Position()

	if n := len(e.modalStack); n > 0 {
		// A dialog is up: clicks outside it do nothing at all.
		if !inRect(e.modalStack[n-1], x, y) {
			return nil, action
		}
		return event, action
	}

	if e.openMenu >= 0 && e.menuList != nil {
		switch {
		case inRect(e.menuList, x, y):
			return event, action // the dropdown answers for itself
		case inRect(e.menubar, x, y):
			// The bar is covered by the dropdown's overlay, so route the click
			// here rather than hoping it falls through: it switches menus or
			// closes the open one.
			if isClick(action) {
				innerX, _, _, _ := e.menubar.GetInnerRect()
				e.clickMenuBar(x - innerX)
			}
			return nil, action
		default:
			e.closeMenu()
			return nil, action
		}
	}
	return event, action
}

// blocker is a full-screen overlay holding a dialog. It offers events to the
// dialog and swallows the rest, so nothing reaches the desktop behind it.
type blocker struct {
	*tview.Flex
}

func (b *blocker) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	handler := b.Flex.MouseHandler()
	return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		if consumed, capture := handler(action, event, setFocus); consumed {
			return consumed, capture
		}
		return true, nil
	}
}

func inRect(p tview.Primitive, x, y int) bool {
	px, py, width, height := p.GetRect()
	return x >= px && x < px+width && y >= py && y < py+height
}
