//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

// What a TextBox offers a code editor written in Lua: its keys before it
// acts on them (onKey), the mouse resting on its text (onHover), where a
// position is shown (pointAt), and text put in at the cursor (insert). A
// completion list or a help tip laid over the box is drawn after it each
// time the box draws itself, so that typing does not paint over it.

import (
	"github.com/pwiecz/go-fltk"
	lua "github.com/yuin/gopher-lua"
)

// hoverDelay is how long the mouse rests before it is a hover.
const hoverDelay = 0.6

type textPositions interface {
	XYToPosition(x, y int) int
	PositionToXY(pos int) (int, int)
	Buffer() *fltk.TextBuffer
}

// textBoxEvent is a TextBox's onKey and onHover; true when the script used
// the event.
func textBoxEvent(o *guiObject, e fltk.Event) bool {
	switch e {
	case fltk.KEY:
		endHover(o)
		if o.events["onKey"] == nil {
			return false
		}
		name := keyName(fltk.EventKey(), fltk.EventState())
		return lua.LVAsBool(o.app.fire(o, "onKey", lua.LString(name), lua.LString(fltk.EventText())))
	case fltk.ENTER, fltk.MOVE:
		if o.events["onHover"] == nil {
			return false
		}
		endHover(o)
		n, x, y := o.mouse.hover, fltk.EventX(), fltk.EventY()
		fltk.AddTimeout(hoverDelay, func() {
			if o.mouse.hover != n || o.widget == nil {
				return
			}
			if pos, ok := textAt(o, x, y); ok {
				o.mouse.hovered = true
				o.app.fire(o, "onHover", lua.LNumber(pos))
			}
		})
	case fltk.LEAVE, fltk.PUSH, fltk.MOUSEWHEEL:
		endHover(o)
	}
	return false
}

// endHover stops a hover on its way, and tells the script one is over.
func endHover(o *guiObject) {
	o.mouse.hover++
	if o.mouse.hovered {
		o.mouse.hovered = false
		o.app.fire(o, "onHover", lua.LNil)
	}
}

// textAt is the position of the character under x, y, when there is one:
// not past the end of a line, nor in the line numbers.
func textAt(o *guiObject, x, y int) (int, bool) {
	t, ok := o.widget.(textPositions)
	if !ok {
		return 0, false
	}
	pos := t.XYToPosition(x, y)
	buf := t.Buffer()
	if pos < 0 || pos >= buf.Length() || buf.CharAt(pos) == '\n' {
		return 0, false
	}
	px, py := t.PositionToXY(pos)
	nx, _ := t.PositionToXY(buf.NextChar(pos))
	if x < px || (nx > px && x > nx) || y < py {
		return 0, false
	}
	return pos, true
}

func pointAt(o *guiObject, pos int) (int, int, int, bool) {
	t, ok := o.widget.(textPositions)
	g, gok := o.widget.(geometry)
	if !ok || !gok {
		return 0, 0, 0, false
	}
	if n := t.Buffer().Length(); pos > n {
		pos = n
	}
	if pos < 0 {
		pos = 0
	}
	x, y := t.PositionToXY(pos)
	return x - g.X(), y - g.Y(), lineHeight(t, pos, y), true
}

// lineHeight is how far apart the lines are, measured from the line next
// to pos's: the font's size says too little of it.
func lineHeight(t textPositions, pos, y int) int {
	buf := t.Buffer()
	if end := buf.LineEnd(pos); end < buf.Length() {
		if _, ny := t.PositionToXY(end + 1); ny > y {
			return ny - y
		}
	}
	if start := buf.LineStart(pos); start > 0 {
		if _, py := t.PositionToXY(start - 1); py < y {
			return y - py
		}
	}
	return 16
}

func insertText(o *guiObject, text string) {
	switch w := o.widget.(type) {
	case *fltk.TextEditor:
		buf := w.Buffer()
		at := w.GetInsertPosition()
		if buf.IsSelected() {
			at, _ = buf.GetSelectionPosition()
			buf.ReplaceSelection(text)
			buf.UnSelect()
		} else {
			buf.Insert(at, text)
		}
		w.SetInsertPosition(at + len(text))
		w.ShowInsertPosition()
	case *fltk.Input:
		v := w.Value()
		a, b := w.Mark(), w.InsertPosition()
		if a > b {
			a, b = b, a
		}
		w.SetValue(v[:a] + text + v[b:])
		w.SetInsertPosition(a+len(text), a+len(text))
	}
}

// drawOver has the controls laid over a TextBox drawn after it, whenever it
// draws: FLTK draws only what changed, and a box being typed into would
// otherwise paint over them.
func drawOver(o *guiObject) {
	p := o.parent
	g, ok := o.widget.(geometry)
	if p == nil || !ok {
		return
	}
	after := false
	for _, c := range p.children {
		if c == o {
			after = true
			continue
		}
		if !after || c.widget == nil {
			continue
		}
		cg, ok := c.widget.(interface {
			geometry
			Visible() bool
			Redraw()
		})
		if ok && cg.Visible() && cg.X() < g.X()+g.W() && g.X() < cg.X()+cg.W() &&
			cg.Y() < g.Y()+g.H() && g.Y() < cg.Y()+cg.H() {
			cg.Redraw()
		}
	}
}
