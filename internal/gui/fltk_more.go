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

	if o.kind == "TextBox" && textBoxEvent(o, e) {
		return true
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

// tableState is what a Table keeps beside its widget: the selected row, as
// the script last heard of it, and the editor laid over a cell while one is
// being edited.
type tableState struct {
	selected int
	// cols is how many columns there are, which FLTK does not say.
	cols int

	// row and col are the cell being edited, from 1; row is 0 when none is.
	row, col int
	// session counts the edits, so that one finished late — the editor
	// losing the focus, after another has started — leaves the new one be.
	session int
	choices []string
	button  bool
	// before is the cell's text when the edit started.
	before string
	// finishing is set while onEdit is asked: what it does to the table
	// (its rows assigned again, the grid shown afresh) does not finish the
	// edit a second time.
	finishing bool

	input *fltk.Input
	pick  *fltk.MenuButton
	dots  *fltk.Button
}

func tableStateOf(o *guiObject) *tableState {
	st, _ := o.state.(*tableState)
	if st == nil {
		st = &tableState{}
		o.state = st
	}
	return st
}

// rowHeight is how high a Table's rows are.
const rowHeight = 22

func buildTable(o *guiObject, x, y, w, h int) {
	t := fltk.NewTableRow(x, y, w, h)
	t.SetType(fltk.SelectSingle)
	t.SetRowHeightAll(rowHeight)
	t.SetColumnHeaderHeight(24)
	t.SetDrawCellCallback(func(ctx fltk.TableContext, r, c, x, y, w, h int) {
		drawCell(o, t, ctx, r, c, x, y, w, h)
	})
	// The editors are made while the table is still taking children, so
	// they are inside it: drawn over its cells, after them.
	st := &tableState{}
	st.input = fltk.NewInput(x, y, 0, 0)
	st.input.Hide()
	st.input.SetEventHandler(func(e fltk.Event) bool { return editorEvent(o, e) })
	st.pick = fltk.NewMenuButton(x, y, 0, 0, "@-22>")
	st.pick.Hide()
	st.pick.ClearVisibleFocus()
	st.dots = fltk.NewButton(x, y, 0, 0, "...")
	st.dots.Hide()
	st.dots.ClearVisibleFocus()
	st.dots.SetCallback(func() { editButton(o) })
	// FLTK keeps the part of a table that holds children hidden until it
	// has some, and go-fltk's End is the group's, which does not show it.
	if p := st.input.Parent(); p != nil {
		p.Show()
	}
	t.End()
	o.widget = t
	o.state = st
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
	case fltk.ContextStartPage:
		// The table may have scrolled, or been made larger: the editor goes
		// with its cell.
		placeEditor(o)
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
		// FLTK draws the cells after the widgets in the table, so the cell
		// under the editor is left for the editor.
		if st, _ := o.state.(*tableState); st != nil && st.row == r+1 && st.col == c+1 && st.input.Visible() {
			return
		}
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
	// Rows FLTK adds are its own height, not the one asked for when there
	// were none.
	t.SetRowHeightAll(rowHeight)
	st := tableStateOf(o)
	st.cols = n
	// An edit of a cell that is no longer there is over, and so is one
	// whose cell the script has just changed under it.
	if st.row > nrows || st.col > n || (st.row > 0 && cell(o, st.row-1, st.col-1) != st.before) {
		finishEdit(o, false, false)
	}
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
	tableStateOf(o).selected = i
	t.Redraw()
}

// noticeSelection tells the script the selection moved, if it did.
func noticeSelection(o *guiObject) {
	st := tableStateOf(o)
	if now := tableSelected(o); now != st.selected {
		st.selected = now
		o.app.fire(o, "onChange")
	}
}

// tableEvent notices the selection changing, and starts editing a cell. The
// table moves the selection while handling the event, after this handler,
// so the look comes a moment later.
func tableEvent(o *guiObject, e fltk.Event) bool {
	t := o.widget.(*fltk.TableRow)
	switch e {
	case fltk.PUSH:
		r, c := t.RowAndColumnFromCursor()
		double := fltk.EventClicks() > 0 && r >= 0
		fltk.AddTimeout(0, func() {
			if o.widget != t {
				return
			}
			noticeSelection(o)
			if r >= 0 && c >= 0 && cellEditable(o, c+1) {
				startEdit(o, r+1, c+1, "")
				return
			}
			if double && tableSelected(o) > 0 {
				o.app.fire(o, "onDoubleClick")
			}
		})
	case fltk.KEY:
		// F2 and Enter edit the selected row's first cell that can be, and
		// typing starts an edit with what was typed.
		if row := tableSelected(o); row > 0 && tableStateOf(o).row == 0 {
			if col := firstEditable(o); col > 0 {
				key, text := fltk.EventKey(), fltk.EventText()
				switch {
				case key == fltk.F2 || key == fltk.ENTER_KEY || key == kpEnter:
					startEdit(o, row, col, "")
					return true
				case len(text) == 1 && text[0] >= ' ' && text[0] != 127 &&
					fltk.EventState()&(fltk.CTRL|fltk.META|fltk.ALT) == 0:
					startEdit(o, row, col, text)
					return true
				}
			}
		}
		fltk.AddTimeout(0, func() {
			if o.widget == t {
				noticeSelection(o)
			}
		})
	case fltk.RELEASE:
		fltk.AddTimeout(0, func() {
			if o.widget == t {
				noticeSelection(o)
			}
		})
	}
	return false
}

// ---------------------------------------------------------------- editing a Table's cells

// kpEnter is the keypad's Enter, which go-fltk has no name for.
const kpEnter = 0xff80 + 'r'

// refusedColor is the editor's when onEdit has said no to what is in it.
var refusedColor = fltk.ColorFromRgb(255, 214, 214)

// cellEditable says whether column col can be edited: editable is true for
// all of them, or a list of the ones that can.
func cellEditable(o *guiObject, col int) bool {
	switch v := o.props["editable"].(type) {
	case lua.LBool:
		return bool(v)
	case *lua.LTable:
		for i := 1; i <= v.Len(); i++ {
			if int(lua.LVAsNumber(v.RawGetInt(i))) == col {
				return true
			}
		}
	}
	return false
}

func firstEditable(o *guiObject) int {
	for c := 1; c <= tableStateOf(o).cols; c++ {
		if cellEditable(o, c) {
			return c
		}
	}
	return 0
}

// startEdit lays the editor over a cell, once the script's onStartEdit has
// agreed to it and said how. Whatever was being edited is finished first.
// typed, when there is some, replaces the cell's text: the key that
// started the edit.
func startEdit(o *guiObject, row, col int, typed string) {
	t, ok := o.widget.(*fltk.TableRow)
	if !ok || row < 1 || row > t.RowCount() || col < 1 || col > tableStateOf(o).cols {
		return
	}
	st := tableStateOf(o)
	if st.row != 0 {
		if st.row == row && st.col == col {
			return
		}
		finishEdit(o, true, false)
	}
	if tableSelected(o) != row {
		setTableSelected(o, row)
		st.selected = 0 // so that the script hears of it
		noticeSelection(o)
	}

	how := o.app.fire(o, "onStartEdit", lua.LNumber(row), lua.LNumber(col))
	if how == lua.LFalse || o.app.err != nil || o.widget != t {
		return
	}
	st.choices, st.button = nil, false
	readOnly := false
	if spec, ok := how.(*lua.LTable); ok {
		if list, ok := spec.RawGetString("choices").(*lua.LTable); ok {
			for i := 1; i <= list.Len(); i++ {
				st.choices = append(st.choices, lua.LVAsString(list.RawGetInt(i)))
			}
		}
		st.button = lua.LVAsBool(spec.RawGetString("button"))
		readOnly = lua.LVAsBool(spec.RawGetString("readOnly"))
	}

	st.session++
	st.row, st.col = row, col
	st.before = cell(o, row-1, col-1)
	st.pick.Clear()
	for _, choice := range st.choices {
		choice := choice
		st.pick.Add(itemText(choice), func() {
			st.input.SetValue(choice)
			finishEdit(o, true, true)
		})
	}

	// Keep the cell in sight.
	if top, _, bottom, _ := t.VisibleCells(); row-1 < top || row-1 > bottom {
		t.SetTopRow(row - 1)
	}
	text := st.before
	if typed != "" && !readOnly {
		text = typed
	}
	st.input.SetValue(text)
	if readOnly {
		st.input.Deactivate()
	} else {
		st.input.Activate()
	}
	st.input.SetColor(white)
	placeEditor(o)
	if readOnly {
		st.dots.TakeFocus()
	} else {
		st.input.TakeFocus()
		if typed != "" {
			st.input.SetInsertPosition(len(text), len(text))
		} else {
			st.input.SetInsertPosition(len(text), 0)
		}
	}
	t.Redraw()
}

// placeEditor puts the editor over its cell, and its button at the cell's
// right; out of sight when the cell has scrolled away.
func placeEditor(o *guiObject) {
	t, ok := o.widget.(*fltk.TableRow)
	st, _ := o.state.(*tableState)
	if !ok || st == nil {
		return
	}
	if st.row == 0 {
		st.input.Hide()
		st.pick.Hide()
		st.dots.Hide()
		return
	}
	top, left, bottom, right := t.VisibleCells()
	x, y, w, h, err := t.FindCell(fltk.ContextCell, st.row-1, st.col-1)
	if err != nil || st.row-1 < top || st.row-1 > bottom || st.col-1 < left || st.col-1 > right {
		st.input.Hide()
		st.pick.Hide()
		st.dots.Hide()
		return
	}
	// FLTK hides the part of the table that holds the editors again when
	// it lays itself out; it has to be showing for them to be drawn.
	if p := st.input.Parent(); p != nil && !p.Visible() {
		p.Show()
	}
	bw := 0
	if st.button || len(st.choices) > 0 {
		bw = h
	}
	st.input.Resize(x, y, w-bw, h)
	st.input.Show()
	var b, other interface {
		Resize(x, y, w, h int)
		Show()
		Hide()
	} = st.pick, st.dots
	if st.button {
		b, other = st.dots, st.pick
	}
	other.Hide()
	if bw > 0 {
		b.Resize(x+w-bw, y, bw, h)
		b.Show()
	} else {
		b.Hide()
	}
}

// finishEdit takes the editor away, with its text as the cell's new value
// when keep is set and the script's onEdit agrees. Refused, the editor stays
// where it is when stay is set, in red, for the user to put right; it is put
// away otherwise, the cell as it was.
func finishEdit(o *guiObject, keep, stay bool) bool {
	t, ok := o.widget.(*fltk.TableRow)
	st, _ := o.state.(*tableState)
	if !ok || st == nil || st.row == 0 || st.finishing {
		return true
	}
	row, col, text := st.row, st.col, st.input.Value()
	if keep && text != st.before {
		st.finishing = true
		answer := o.app.fire(o, "onEdit", lua.LNumber(row), lua.LNumber(col), lua.LString(text))
		st.finishing = false
		if o.widget != t || st.row != row {
			return true // the handler finished it itself, or closed the form
		}
		if answer == lua.LFalse || o.app.err != nil {
			if stay && o.app.err == nil {
				st.input.SetColor(refusedColor)
				st.input.Redraw()
				st.input.TakeFocus()
				return false
			}
		} else {
			if s, ok := answer.(lua.LString); ok {
				text = string(s)
			}
			setCell(o, row, col, text)
		}
	}
	hadFocus := st.input.HasFocus() || st.dots.HasFocus() || st.pick.HasFocus()
	st.row, st.col = 0, 0
	placeEditor(o)
	if hadFocus {
		t.TakeFocus()
	}
	t.Redraw()
	return true
}

// setCell writes an edited cell back into the script's rows.
func setCell(o *guiObject, row, col int, text string) {
	rows, _ := o.props["rows"].(*lua.LTable)
	if rows == nil {
		return
	}
	r, _ := rows.RawGetInt(row).(*lua.LTable)
	if r == nil {
		r = o.app.L.NewTable()
		rows.RawSetInt(row, r)
	}
	r.RawSetInt(col, lua.LString(text))
}

// editorEvent is the cell editor's keys: Enter keeps what was typed, Escape
// puts it back, Up and Down keep it and edit the cell above or below, and
// Tab keeps it and leaves. A double click on a cell with choices takes the
// next one, as Delphi's did. Losing the focus keeps what was typed.
func editorEvent(o *guiObject, e fltk.Event) bool {
	st, _ := o.state.(*tableState)
	if st == nil || st.row == 0 {
		return false
	}
	switch e {
	case fltk.KEY:
		switch fltk.EventKey() {
		case fltk.ENTER_KEY, kpEnter:
			finishEdit(o, true, true)
			return true
		case fltk.ESCAPE:
			finishEdit(o, false, false)
			return true
		case fltk.UP, fltk.DOWN:
			row, col := st.row, st.col
			if fltk.EventKey() == fltk.UP {
				row--
			} else {
				row++
			}
			if finishEdit(o, true, true) && row >= 1 {
				startEdit(o, row, col, "")
			}
			return true
		case fltk.TAB:
			finishEdit(o, true, false)
			return true
		}
	case fltk.PUSH:
		if fltk.EventClicks() > 0 && len(st.choices) > 0 {
			next := st.choices[0]
			for i, c := range st.choices {
				if c == st.input.Value() && i+1 < len(st.choices) {
					next = st.choices[i+1]
				}
			}
			st.input.SetValue(next)
			finishEdit(o, true, true)
			return true
		}
	case fltk.UNFOCUS:
		session := st.session
		fltk.AddTimeout(0, func() {
			// The focus has gone to the editor's own button, or another
			// edit has begun: either way this one is not over.
			if st.session != session || st.row == 0 || st.input.HasFocus() ||
				st.dots.HasFocus() || st.pick.HasFocus() {
				return
			}
			finishEdit(o, true, false)
		})
	}
	return false
}

// editButton is the "..." at a cell's right: the script's onEditButton
// edits the value its own way, in a dialog of its own, mostly.
func editButton(o *guiObject) {
	st, _ := o.state.(*tableState)
	if st == nil || st.row == 0 {
		return
	}
	row, col, session := st.row, st.col, st.session
	o.app.fire(o, "onEditButton", lua.LNumber(row), lua.LNumber(col))
	if st.session != session || st.row == 0 {
		return
	}
	// The handler has changed the cell, if it changed anything; the edit
	// goes on from there.
	st.before = cell(o, row-1, col-1)
	st.input.SetValue(st.before)
	if st.input.IsActive() {
		st.input.TakeFocus()
	}
}

// tableEdit is grid:edit(row, col): an edit started from the script. With
// no row, it finishes the one there is, keeping what was typed.
func tableEdit(o *guiObject, row, col int) {
	if row == 0 {
		finishEdit(o, true, false)
		return
	}
	startEdit(o, row, col, "")
}

// tableEditing is the cell being edited, or 0, 0.
func tableEditing(o *guiObject) (int, int) {
	if st, ok := o.state.(*tableState); ok {
		return st.row, st.col
	}
	return 0, 0
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
