//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import (
	"fmt"
	"math"
	goruntime "runtime"
	"sort"
	"strings"

	"github.com/pwiecz/go-fltk"
	lua "github.com/yuin/gopher-lua"
)

// listen gives a widget the one event handler every object has: drops,
// drags out, a Canvas's mouse, a Form's keys and a Table's selection. Some
// FLTK widgets cannot take a handler; they go without.
func listen(o *guiObject) {
	w, ok := o.widget.(interface{ SetEventHandler(func(fltk.Event) bool) })
	if !ok {
		return
	}
	defer func() { _ = recover() }() // "this widget does not support event handling"
	w.SetEventHandler(func(e fltk.Event) bool { return handle(o, e) })
}

// startDrag hands the selection to the system's drag and drop. Tests
// replace it: a drag session needs a real mouse behind it.
var startDrag = fltk.DragAndDrop

// dragDistance is how far the mouse moves with the button down before a
// press becomes a drag.
const dragDistance = 5

func handle(o *guiObject, e fltk.Event) bool {
	a := o.app
	has := func(event string) bool { return o.events[event] != nil }

	// A form's drops go to its catcher (see catchDrops), after its controls
	// have had their say; the window itself leaves them to FLTK to route.
	if o.kind != "Form" {
		if used, ok := dropEvent(o, e); ok {
			return used
		}
	}

	switch e {
	case fltk.PUSH:
		o.mouse.armed = has("onDrag")
		o.mouse.x, o.mouse.y = fltk.EventX(), fltk.EventY()
	case fltk.DRAG:
		if o.mouse.armed && (abs(fltk.EventX()-o.mouse.x) > dragDistance || abs(fltk.EventY()-o.mouse.y) > dragDistance) {
			o.mouse.armed = false
			if text, ok := a.fire(o, "onDrag").(lua.LString); ok && text != "" {
				fltk.CopyToSelectionBuffer(string(text))
				startDrag()
				return true
			}
		}
	case fltk.RELEASE:
		o.mouse.armed = false
	}

	if e == fltk.KEY && fltk.EventKey() == fltk.TAB && !propBool(o, "acceptsTab") && tabNavigate(o) {
		return true
	}

	switch o.kind {
	case "Form":
		// Keys come here as shortcuts once the focused control has passed
		// on them, so typing into a TextBox is not a key press of the form's.
		return e == fltk.SHORTCUT && onKey(o)
	case "Canvas":
		return canvasEvent(o, e)
	case "Table":
		return tableEvent(o, e)
	case "Tree":
		return treeKey(o, e)
	case "TextBox":
		return codeKey(o, e)
	case "Label", "Image", "ProgressBar":
		// These take no presses of their own, and a drag starts with one.
		return e == fltk.PUSH && has("onDrag")
	}
	return false
}

// dropEvent handles what drag and drop sends an object; ok is false for
// any other event.
func dropEvent(o *guiObject, e fltk.Event) (used, ok bool) {
	wants := o.events["onDrop"] != nil
	switch e {
	case fltk.DND_LEAVE:
		return wants, true
	case fltk.DND_ENTER, fltk.DND_DRAG:
		return wants && under(o), true
	case fltk.DND_RELEASE:
		// A drop nothing under the mouse took is offered to every control
		// in turn, so each checks that it is the one dropped on.
		if wants && under(o) {
			o.mouse.dropping = true
			return true, true
		}
		return false, true
	case fltk.PASTE:
		// A drop arrives as a paste straight after the release; any other
		// paste is the widget's own business.
		if !o.mouse.dropping {
			return false, true
		}
		o.mouse.dropping = false
		text := fltk.EventText()
		lines := o.app.L.NewTable()
		for _, l := range strings.Split(strings.TrimRight(text, "\r\n"), "\n") {
			lines.Append(lua.LString(strings.TrimRight(l, "\r")))
		}
		o.app.fire(o, "onDrop", lua.LString(text), lines)
		return true, true
	}
	return false, false
}

// catchDrops puts an invisible box behind everything on a form, to take
// the drops its controls do not when the form has an onDrop. FLTK only
// delivers a drop to the widget that said yes to it while it was dragged
// over, and offers it to the controls in front first.
func catchDrops(f *guiObject, w, h int) {
	b := fltk.NewBox(fltk.NO_BOX, 0, 0, w, h)
	b.SetEventHandler(func(e fltk.Event) bool {
		used, _ := dropEvent(f, e)
		return used
	})
}

// under says whether the mouse is over an object; a Form is all under it.
func under(o *guiObject) bool {
	if o.kind == "Form" {
		return true
	}
	g, ok := o.widget.(geometry)
	if !ok {
		return false
	}
	x, y := fltk.EventX(), fltk.EventY()
	return x >= g.X() && x < g.X()+g.W() && y >= g.Y() && y < g.Y()+g.H()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func redraw(o *guiObject) {
	// FLTK draws again only the widget asked for. A transparent Canvas shows
	// what is under it, which has to be drawn first; and a Canvas under other
	// controls would paint over them. Either way it is what holds it that is
	// drawn again, everything in it in its order.
	if o.kind == "Canvas" && o.parent != nil && (propBool(o, "transparent") || covered(o)) {
		o = o.parent
	}
	if w, ok := o.widget.(interface{ Redraw() }); ok {
		w.Redraw()
	}
}

// covered says whether a control drawn after o, in what holds it, overlaps
// it.
func covered(o *guiObject) bool {
	g, ok := o.widget.(geometry)
	if !ok {
		return false
	}
	after := false
	for _, c := range o.parent.children {
		if c == o {
			after = true
			continue
		}
		if !after {
			continue
		}
		if w, ok := c.widget.(geometry); ok &&
			w.X() < g.X()+g.W() && g.X() < w.X()+w.W() && w.Y() < g.Y()+g.H() && g.Y() < w.Y()+w.H() {
			return true
		}
	}
	return false
}

func setClipboard(text string) error {
	fltk.CopyToClipboard(text)
	return nil
}

// pasteInto is a text editor that never shows, kept for reading the
// clipboard: go-fltk has no call for that, but an editor can paste.
var pasteInto *fltk.TextEditor

func getClipboard() (string, error) {
	if pasteInto == nil {
		pasteInto = fltk.NewTextEditor(0, 0, 1, 1)
		pasteInto.SetBuffer(fltk.NewTextBuffer())
		pasteInto.Hide()
	}
	buf := pasteInto.Buffer()
	buf.SetText("")
	pasteInto.Paste()
	// On X11 the text comes back through the event loop.
	if goruntime.GOOS != "darwin" && goruntime.GOOS != "windows" {
		for i := 0; i < 10 && buf.Text() == ""; i++ {
			fltk.Wait(0.05)
		}
	}
	return buf.Text(), nil
}

// ---------------------------------------------------------------- Tree

func buildTree(o *guiObject, x, y, w, h int) {
	b := fltk.NewHoldBrowser(x, y, w, h)
	b.SetFormatChar(0)
	b.SetColumnChar(0)
	b.SetCallbackCondition(fltk.WhenChanged | fltk.WhenNotChanged | fltk.WhenRelease)
	o.widget = b
	o.state = []treeRow(nil)
	last := 0
	b.SetCallback(func() {
		v := b.Value()
		row, ok := treeRowAt(o, v)
		if v != last {
			last = v
			o.app.fire(o, "onChange")
			return
		}
		if !ok || fltk.EventType() != fltk.RELEASE {
			return
		}
		switch {
		case fltk.EventClicks() > 0:
			o.app.fire(o, "onDoubleClick")
			if row.branch {
				toggle(o, row)
			}
		case row.branch && fltk.EventX()-b.X() <= markerRight(row):
			toggle(o, row)
		}
	})
}

func treeRowAt(o *guiObject, line int) (treeRow, bool) {
	rows, _ := o.state.([]treeRow)
	if line < 1 || line > len(rows) {
		return treeRow{}, false
	}
	return rows[line-1], true
}

// markerRight is where a row's ▸ ends, in pixels from the list's left edge.
func markerRight(r treeRow) int {
	fltk.SetDrawFont(fltk.HELVETICA, 14)
	w, _ := fltk.MeasureText(strings.Repeat("     ", r.depth)+"▸ ", false)
	return w + 8
}

// toggle opens or closes a branch, in the script's table as well.
func toggle(o *guiObject, r treeRow) {
	r.node.RawSetString("open", lua.LBool(!r.open))
	refreshTree(o, r.path)
	o.app.fire(o, "onToggle", lua.LString(r.path), lua.LBool(!r.open))
}

// refreshTree lists the tree again, keeping path selected if it shows.
func refreshTree(o *guiObject, path string) {
	b := o.widget.(*fltk.HoldBrowser)
	items, _ := o.props["items"].(*lua.LTable)
	rows := flattenTree(items, 0, "", nil)
	o.state = rows
	top := b.TopLine()
	b.Clear()
	sel := 0
	for i, r := range rows {
		b.Add(treeLine(r))
		if r.path == path {
			sel = i + 1
		}
	}
	_ = b.SetTopLine(top)
	b.SetValue(sel)
}

func selectPath(o *guiObject, path string) {
	items, _ := o.props["items"].(*lua.LTable)
	openTo(items, path)
	refreshTree(o, path)
}

// treeKey opens a branch with Right and closes it with Left.
func treeKey(o *guiObject, e fltk.Event) bool {
	if e != fltk.KEY {
		return false
	}
	row, ok := treeRowAt(o, o.widget.(*fltk.HoldBrowser).Value())
	if !ok || !row.branch {
		return false
	}
	switch fltk.EventKey() {
	case fltk.RIGHT:
		if !row.open {
			toggle(o, row)
		}
		return true
	case fltk.LEFT:
		if row.open {
			toggle(o, row)
		}
		return true
	}
	return false
}

// ---------------------------------------------------------------- Table

var (
	gridColor      = fltk.ColorFromRgb(220, 220, 220)
	selectionColor = fltk.ColorFromRgb(56, 117, 215)
	black          = fltk.ColorFromRgb(0, 0, 0)
	white          = fltk.ColorFromRgb(255, 255, 255)
)

func buildTable(o *guiObject, x, y, w, h int) {
	t := fltk.NewTableRow(x, y, w, h)
	t.SetType(fltk.SelectSingle)
	t.SetRowHeightAll(22)
	t.SetColumnHeaderHeight(24)
	t.SetDrawCellCallback(func(ctx fltk.TableContext, r, c, x, y, w, h int) {
		drawCell(o, t, ctx, r, c, x, y, w, h)
	})
	t.End()
	o.widget = t
}

// cell is what a Table shows at row r and column c, counting from 0.
func cell(o *guiObject, r, c int) string {
	rows, _ := o.props["rows"].(*lua.LTable)
	if rows == nil {
		return ""
	}
	row, _ := rows.RawGetInt(r + 1).(*lua.LTable)
	if row == nil {
		return ""
	}
	if v := row.RawGetInt(c + 1); v != lua.LNil {
		return lua.LVAsString(v)
	}
	return ""
}

func drawCell(o *guiObject, t *fltk.TableRow, ctx fltk.TableContext, r, c, x, y, w, h int) {
	switch ctx {
	case fltk.ContextColHeader:
		fltk.PushClip(x, y, w, h)
		fltk.DrawBox(fltk.THIN_UP_BOX, x, y, w, h, fltk.BACKGROUND_COLOR)
		fltk.SetDrawColor(black)
		fltk.SetDrawFont(fltk.HELVETICA_BOLD, 14)
		cols := propStrings(o, "columns")
		if c < len(cols) {
			fltk.Draw(label(cols[c]), x+4, y, w-8, h, fltk.ALIGN_LEFT)
		}
		fltk.PopClip()
	case fltk.ContextCell:
		fltk.PushClip(x, y, w, h)
		bg, fg := white, black
		if t.IsRowSelected(r) {
			bg, fg = selectionColor, white
		}
		fltk.DrawRectfWithColor(x, y, w, h, bg)
		fltk.SetDrawColor(fg)
		fltk.SetDrawFont(fltk.HELVETICA, 14)
		fltk.Draw(label(cell(o, r, c)), x+4, y, w-8, h, fltk.ALIGN_LEFT)
		fltk.DrawRectWithColor(x, y, w, h, gridColor)
		fltk.PopClip()
	}
}

func propStrings(o *guiObject, name string) []string {
	t, _ := o.props[name].(*lua.LTable)
	if t == nil {
		return nil
	}
	out := make([]string, 0, t.Len())
	for i := 1; i <= t.Len(); i++ {
		out = append(out, lua.LVAsString(t.RawGetInt(i)))
	}
	return out
}

// setTable fits the table to its columns and rows.
func setTable(o *guiObject) {
	t := o.widget.(*fltk.TableRow)
	cols := propStrings(o, "columns")
	n := len(cols)
	rows, _ := o.props["rows"].(*lua.LTable)
	nrows := 0
	if rows != nil {
		nrows = rows.Len()
		for i := 1; i <= nrows; i++ {
			if r, ok := rows.RawGetInt(i).(*lua.LTable); ok && r.Len() > n {
				n = r.Len()
			}
		}
	}
	if len(cols) > 0 {
		t.EnableColumnHeaders()
	} else {
		t.DisableColumnHeaders()
	}
	t.SetColumnCount(n)
	t.SetRowCount(nrows)
	widths, _ := o.props["columnWidths"].(*lua.LTable)
	for c := 0; c < n; c++ {
		w := 0
		if widths != nil {
			w = int(lua.LVAsNumber(widths.RawGetInt(c + 1)))
		}
		if w <= 0 {
			w = (t.W() - t.ScrollbarSize() - 4) / n
		}
		t.SetColumnWidth(c, w)
	}
	t.Redraw()
}

func tableSelected(o *guiObject) int {
	t := o.widget.(*fltk.TableRow)
	for r := 0; r < t.RowCount(); r++ {
		if t.IsRowSelected(r) {
			return r + 1
		}
	}
	return 0
}

func setTableSelected(o *guiObject, i int) {
	t := o.widget.(*fltk.TableRow)
	t.SelectAllRows(fltk.Deselect)
	if i >= 1 && i <= t.RowCount() {
		t.SelectRow(i-1, fltk.Select)
	}
	o.state = i
	t.Redraw()
}

// tableEvent notices the selection changing. The table moves it while
// handling the event, after this handler, so the look comes a moment later.
func tableEvent(o *guiObject, e fltk.Event) bool {
	switch e {
	case fltk.PUSH, fltk.RELEASE, fltk.KEY:
	default:
		return false
	}
	double := false
	if e == fltk.PUSH && fltk.EventClicks() > 0 {
		r, _ := o.widget.(*fltk.TableRow).RowAndColumnFromCursor()
		double = r >= 0
	}
	fltk.AddTimeout(0, func() {
		now := tableSelected(o)
		if last, _ := o.state.(int); now != last {
			o.state = now
			o.app.fire(o, "onChange")
		}
		if double && now > 0 {
			o.app.fire(o, "onDoubleClick")
		}
	})
	return false
}

// ---------------------------------------------------------------- Canvas

func buildCanvas(o *guiObject, x, y, w, h int) {
	box := fltk.FLAT_BOX
	if propBool(o, "transparent") {
		box = fltk.NO_BOX
	}
	b := fltk.NewBox(box, x, y, w, h)
	b.SetDrawHandler(func(base func()) {
		base()
		drawCanvas(o, b)
	})
	o.widget = b
}

// canvas is a Canvas's drawing state, kept between draws.
type canvas struct {
	g       *lua.LTable
	drawing bool
	images  map[string]scalable
}

func drawCanvas(o *guiObject, b *fltk.Box) {
	if o.events["onDraw"] == nil {
		return
	}
	c := canvasOf(o)
	fltk.PushClip(b.X(), b.Y(), b.W(), b.H())
	fltk.SetDrawColor(black)
	fltk.SetDrawFont(fltk.HELVETICA, 14)
	fltk.SetLineStyle(fltk.SOLID, 1)
	c.drawing = true
	o.app.fire(o, "onDraw", c.g)
	c.drawing = false
	fltk.SetLineStyle(fltk.SOLID, 0)
	fltk.PopClip()
}

func canvasOf(o *guiObject) *canvas {
	if c, ok := o.state.(*canvas); ok {
		return c
	}
	c := &canvas{images: map[string]scalable{}}
	c.g = drawingAPI(o, c)
	o.state = c
	return c
}

func canvasEvent(o *guiObject, e fltk.Event) bool {
	a := o.app
	b := o.widget.(*fltk.Box)
	at := func() (lua.LValue, lua.LValue) {
		return lua.LNumber(fltk.EventX() - b.X()), lua.LNumber(fltk.EventY() - b.Y())
	}
	keys := o.events["onKey"] != nil
	switch e {
	case fltk.PUSH:
		if keys {
			b.TakeFocus()
		}
		x, y := at()
		a.fire(o, "onMouseDown", x, y, lua.LNumber(fltk.EventButton()), lua.LBool(fltk.EventClicks() > 0),
			lua.LString(modNames(fltk.EventState())))
		return true // so that the drag and the release come here too
	case fltk.FOCUS, fltk.UNFOCUS:
		// A Canvas with onKey can have the keyboard.
		return keys
	case fltk.KEY:
		if !keys {
			return false
		}
		name := keyName(fltk.EventKey(), fltk.EventState())
		return lua.LVAsBool(a.fire(o, "onKey", lua.LString(name), lua.LString(fltk.EventText())))
	case fltk.DRAG:
		x, y := at()
		a.fire(o, "onMouseDrag", x, y)
		return true
	case fltk.RELEASE:
		x, y := at()
		a.fire(o, "onMouseUp", x, y, lua.LNumber(fltk.EventButton()))
		return true
	case fltk.ENTER:
		// FLTK sends the first move over a widget as this, so it is a move
		// as well.
		x, y := at()
		a.fire(o, "onMouseEnter")
		a.fire(o, "onMouseMove", x, y)
		return true // so that the moves after it come here
	case fltk.LEAVE:
		a.fire(o, "onMouseLeave")
		return true
	case fltk.MOVE:
		x, y := at()
		a.fire(o, "onMouseMove", x, y)
		return true
	case fltk.MOUSEWHEEL:
		if o.events["onMouseWheel"] == nil {
			return false
		}
		a.fire(o, "onMouseWheel", lua.LNumber(fltk.EventDX()), lua.LNumber(fltk.EventDY()))
		return true
	}
	return false
}

// drawingAPI is the g a Canvas's onDraw gets. Its methods draw in the
// canvas's own coordinates, and only while it is being drawn.
func drawingAPI(o *guiObject, c *canvas) *lua.LTable {
	L := o.app.L
	g := L.NewTable()
	origin := func() (int, int) {
		b := o.widget.(*fltk.Box)
		return b.X(), b.Y()
	}
	n := func(L *lua.LState, i int) int { return int(math.Round(float64(L.CheckNumber(i)))) }
	def := func(name string, fn func(L *lua.LState, x0, y0 int) int) {
		g.RawSetString(name, L.NewFunction(func(L *lua.LState) int {
			if !c.drawing {
				L.RaiseError("gui: g:%s only works inside onDraw", name)
			}
			x0, y0 := origin()
			return fn(L, x0, y0)
		}))
	}
	def("size", func(L *lua.LState, _, _ int) int {
		b := o.widget.(*fltk.Box)
		L.Push(lua.LNumber(b.W()))
		L.Push(lua.LNumber(b.H()))
		return 2
	})
	def("color", func(L *lua.LState, _, _ int) int {
		r, gr, b, err := parseColor(L.CheckString(2))
		if err != nil {
			L.RaiseError("%s", err.Error())
		}
		fltk.SetDrawColor(fltk.ColorFromRgb(r, gr, b))
		return 0
	})
	def("width", func(L *lua.LState, _, _ int) int {
		fltk.SetLineStyle(fltk.SOLID, n(L, 2))
		return 0
	})
	def("font", func(L *lua.LState, _, _ int) int {
		f, ok := map[string]fltk.Font{"sans": fltk.HELVETICA, "serif": fltk.TIMES, "mono": fltk.COURIER}[L.CheckString(2)]
		if !ok {
			L.ArgError(2, "font must be \"sans\", \"serif\" or \"mono\"")
		}
		fltk.SetDrawFont(f, int(L.OptNumber(3, 14)))
		return 0
	})
	def("point", func(L *lua.LState, x0, y0 int) int {
		fltk.DrawPoint(x0+n(L, 2), y0+n(L, 3))
		return 0
	})
	def("line", func(L *lua.LState, x0, y0 int) int {
		fltk.DrawLine(x0+n(L, 2), y0+n(L, 3), x0+n(L, 4), y0+n(L, 5))
		return 0
	})
	def("rect", func(L *lua.LState, x0, y0 int) int {
		fltk.DrawRect(x0+n(L, 2), y0+n(L, 3), n(L, 4), n(L, 5))
		return 0
	})
	def("fill", func(L *lua.LState, x0, y0 int) int {
		fltk.DrawRectf(x0+n(L, 2), y0+n(L, 3), n(L, 4), n(L, 5))
		return 0
	})
	def("circle", func(L *lua.LState, x0, y0 int) int {
		x, y, r := n(L, 2), n(L, 3), n(L, 4)
		fltk.DrawArc(x0+x-r, y0+y-r, 2*r, 2*r, 0, 360)
		return 0
	})
	def("disc", func(L *lua.LState, x0, y0 int) int {
		x, y, r := n(L, 2), n(L, 3), n(L, 4)
		fltk.DrawPie(x0+x-r, y0+y-r, 2*r, 2*r, 0, 360)
		return 0
	})
	def("arc", func(L *lua.LState, x0, y0 int) int {
		fltk.DrawArc(x0+n(L, 2), y0+n(L, 3), n(L, 4), n(L, 5), float64(L.CheckNumber(6)), float64(L.CheckNumber(7)))
		return 0
	})
	def("pie", func(L *lua.LState, x0, y0 int) int {
		fltk.DrawPie(x0+n(L, 2), y0+n(L, 3), n(L, 4), n(L, 5), float64(L.CheckNumber(6)), float64(L.CheckNumber(7)))
		return 0
	})
	points := func(L *lua.LState, x0, y0 int) []int {
		top := L.GetTop()
		if top < 7 || (top-1)%2 != 0 {
			L.RaiseError("gui: a polygon takes three or more x, y pairs")
		}
		ps := make([]int, 0, top-1)
		for i := 2; i <= top; i += 2 {
			ps = append(ps, x0+n(L, i), y0+n(L, i+1))
		}
		return ps
	}
	def("polygon", func(L *lua.LState, x0, y0 int) int {
		// FLTK fills three or four corners; more are filled as a fan,
		// which is right for any convex shape.
		ps := points(L, x0, y0)
		for i := 2; i+3 < len(ps); i += 2 {
			fltk.DrawPolygon(ps[0], ps[1], ps[i], ps[i+1], ps[i+2], ps[i+3])
		}
		return 0
	})
	def("loop", func(L *lua.LState, x0, y0 int) int {
		ps := points(L, x0, y0)
		for i := 0; i < len(ps); i += 2 {
			j := (i + 2) % len(ps)
			fltk.DrawLine(ps[i], ps[i+1], ps[j], ps[j+1])
		}
		return 0
	})
	def("text", func(L *lua.LState, x0, y0 int) int {
		s := label(L.CheckString(2))
		x, y := x0+n(L, 3), y0+n(L, 4)
		if L.GetTop() >= 6 {
			align := map[string]fltk.Align{"left": fltk.ALIGN_LEFT, "center": fltk.ALIGN_CENTER, "right": fltk.ALIGN_RIGHT}[L.OptString(7, "left")]
			fltk.Draw(s, x, y, n(L, 5), n(L, 6), align|fltk.ALIGN_INSIDE)
			return 0
		}
		w, h := fltk.MeasureText(s, true)
		fltk.Draw(s, x, y, w, h, fltk.ALIGN_TOP_LEFT|fltk.ALIGN_INSIDE)
		return 0
	})
	def("measure", func(L *lua.LState, _, _ int) int {
		w, h := fltk.MeasureText(label(L.CheckString(2)), true)
		L.Push(lua.LNumber(w))
		L.Push(lua.LNumber(h))
		return 2
	})
	def("image", func(L *lua.LState, x0, y0 int) int {
		path := o.app.besideCaller(L, L.CheckString(2))
		x, y := x0+n(L, 3), y0+n(L, 4)
		w, h := int(L.OptNumber(5, 0)), int(L.OptNumber(6, 0))
		key := fmt.Sprintf("%s@%dx%d", path, w, h)
		img, ok := c.images[key]
		if !ok {
			var err error
			if img, err = loadImage(o.app, path); err != nil {
				L.RaiseError("gui: cannot load image %s: %v", path, err)
			}
			if w > 0 && h > 0 {
				img.Scale(w, h, false, true)
			}
			c.images[key] = img
		}
		img.Draw(x, y, img.W(), img.H())
		return 0
	})
	return g
}

// codeKey makes Tab and Enter indent in a TextBox that acceptsTab: Tab puts
// in two spaces rather than moving to the next control, and Enter starts
// the new line as far in as the one it leaves.
func codeKey(o *guiObject, e fltk.Event) bool {
	ed, ok := o.widget.(*fltk.TextEditor)
	if !ok || e != fltk.KEY || !propBool(o, "acceptsTab") {
		return false
	}
	mods := fltk.EventState() & (fltk.CTRL | fltk.ALT | fltk.META | fltk.SHIFT)
	switch {
	case fltk.EventKey() == fltk.TAB && mods == 0:
		ed.InsertText("  ")
		o.app.fire(o, "onChange")
		return true
	case fltk.EventKey() == fltk.ENTER_KEY && mods == 0:
		buf := ed.Buffer()
		line := buf.LineText(ed.GetInsertPosition())
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		ed.InsertText("\n" + indent)
		ed.ShowInsertPosition()
		o.app.fire(o, "onChange")
		return true
	}
	return false
}

// tabNavigate moves the focus on from o with Tab, or back with Shift-Tab,
// in the order of the form's tabIndex: controls with one first, by it, and
// then the rest as the form holds them. A form whose controls have no
// tabIndex is left to FLTK, which goes in the order they were made.
func tabNavigate(o *guiObject) bool {
	f := o.form()
	if f == nil || fltk.EventState()&(fltk.CTRL|fltk.ALT|fltk.META) != 0 {
		return false
	}
	var order []*guiObject
	ordered := false
	var walk func(p *guiObject)
	walk = func(p *guiObject) {
		for _, c := range p.children {
			if _, ok := c.spec.props["tabIndex"]; ok && c.widget != nil {
				if w, ok := c.widget.(interface {
					Visible() bool
					IsActive() bool
				}); ok && w.Visible() && w.IsActive() {
					order = append(order, c)
					if propInt(c, "tabIndex", 0) > 0 {
						ordered = true
					}
				}
			}
			walk(c)
		}
	}
	walk(f)
	if !ordered || len(order) < 2 {
		return false
	}
	rank := func(c *guiObject) int {
		if i := propInt(c, "tabIndex", 0); i > 0 {
			return i
		}
		return 1 << 30
	}
	sort.SliceStable(order, func(i, j int) bool { return rank(order[i]) < rank(order[j]) })
	at := -1
	for i, c := range order {
		if c == o {
			at = i
		}
	}
	if at < 0 {
		return false
	}
	step := 1
	if fltk.EventState()&fltk.SHIFT != 0 {
		step = len(order) - 1
	}
	for n := 1; n < len(order); n++ {
		next := order[(at+step*n)%len(order)]
		if w, ok := next.widget.(interface{ TakeFocus() int }); ok && w.TakeFocus() != 0 {
			return true
		}
	}
	return true
}
