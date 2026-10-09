//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

// Popup menus: a control's contextMenu, shown on a right click (a
// Ctrl-click on a Mac), and gui.popup, shown when a script asks.

import (
	"errors"
	goruntime "runtime"

	"github.com/pwiecz/go-fltk"
	lua "github.com/yuin/gopher-lua"
)

// showMenu shows items at the mouse, over o's form, and waits for a pick:
// nil when the menu was closed without one.
func showMenu(o *guiObject, items *lua.LTable) (*menuPick, error) {
	win := window(o.form())
	if win == nil || !win.IsShown() {
		return nil, errors.New("gui: a popup menu is shown over a form on screen")
	}
	// The button the menu hangs from is the form's for as long as it is
	// up, so that the menu knows the window it is in; it is never drawn.
	// Adding it to the window takes it out of whatever group was being
	// built when it was made.
	mb := fltk.NewMenuButton(0, 0, 1, 1)
	defer mb.Destroy()
	mb.SetType(fltk.POPUP3)
	var got *menuPick
	err := addMenu(mb, "", items, func(item *lua.LTable, name lua.LValue, text string, on bool) {
		got = &menuPick{item: item, name: name, text: text, on: on}
	})
	if err != nil {
		return nil, err
	}
	win.Add(mb)
	mb.Resize(fltk.EventX(), fltk.EventY(), 1, 1)
	mb.Popup()
	win.Remove(mb)
	return got, nil
}

// contextClick says whether the mouse press is the one that asks for a
// context menu: the right button, or a Ctrl-click on a Mac.
func contextClick() bool {
	b := fltk.EventButton()
	return b == fltk.RightMouse ||
		goruntime.GOOS == "darwin" && b == fltk.LeftMouse && fltk.EventState()&fltk.CTRL != 0
}

func menuOf(o *guiObject) *lua.LTable {
	if items, ok := o.props["contextMenu"].(*lua.LTable); ok && items.Len() > 0 {
		return items
	}
	return nil
}

// menuTarget is the control under the mouse whose context menu a right
// click on o shows: the innermost one that has one, o itself included.
func menuTarget(o *guiObject) *guiObject {
	x, y := fltk.EventX(), fltk.EventY()
	var best *guiObject
	var walk func(p *guiObject)
	walk = func(p *guiObject) {
		if p.kind != "Form" {
			w, ok := p.widget.(interface {
				geometry
				Visible() bool
			})
			if !ok || !w.Visible() || x < w.X() || y < w.Y() || x >= w.X()+w.W() || y >= w.Y()+w.H() {
				return
			}
		}
		if menuOf(p) != nil {
			best = p
		}
		for _, c := range p.children {
			walk(c)
		}
	}
	walk(o)
	return best
}

// selectsFirst are the kinds a right click picks a line of before its
// menu comes up, as a file manager's list does.
func selectsFirst(kind string) bool {
	return kind == "ListBox" || kind == "Tree" || kind == "Table"
}

// contextPress handles a right click on o: true when it is done with,
// false when the press is to go on to the widget (a list selecting the
// line under the mouse, or a control inside o showing its own menu).
func contextPress(o *guiObject) (used bool) {
	target := menuTarget(o)
	if target == nil {
		return false
	}
	if target != o && target.mouse.listening {
		return false // its own handler shows it
	}
	if target == o && selectsFirst(o.kind) {
		fltk.AddTimeout(0, func() {
			if o.widget != nil {
				runContextMenu(o)
			}
		})
		return false
	}
	runContextMenu(target)
	return true
}

// runContextMenu shows o's context menu, and runs what was picked: the
// item's own function, then o's onContextMenu(name, caption, checked).
func runContextMenu(o *guiObject) {
	items := menuOf(o)
	if items == nil {
		return
	}
	got, err := showMenu(o, items)
	if err != nil {
		if o.app.err == nil {
			o.app.err = err
		}
		return
	}
	if got == nil {
		return
	}
	if fn := itemFunction(got.item); fn != nil {
		o.app.call(fn, lua.LString(got.text), lua.LBool(got.on))
	}
	o.app.fire(o, "onContextMenu", got.name, lua.LString(got.text), lua.LBool(got.on))
}

func measureText(text, font string, size int) (int, int, error) {
	f := map[string]fltk.Font{"sans": fltk.HELVETICA, "serif": fltk.TIMES, "mono": fltk.COURIER}[font]
	fltk.SetDrawFont(f, size)
	w, h := fltk.MeasureText(text, false)
	return w, h, nil
}
