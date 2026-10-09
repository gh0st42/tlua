//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"unicode"

	"github.com/pwiecz/go-fltk"
	lua "github.com/yuin/gopher-lua"

	"tlua/internal/gui/fltkcolor"
	"tlua/internal/luasyntax"
)

// The build constraint above is the list of platforms go-fltk ships prebuilt
// libraries for; nofltk.go covers everything else.

// FLTK has to be driven from the main thread on macOS, and Go only keeps the
// main goroutine there when asked before main runs.
func init() { goruntime.LockOSThread() }

// tabRow is the height of the row of tabs above a Tabs' pages.
const tabRow = 25

// wait runs the event loop once. Callers loop on their own condition rather
// than using fltk.Run, which waits for every window. And on macOS a loop
// blocked with nothing pending can miss a window going away for as long as no
// other event arrives, so it wakes up every tenth of a second.
func wait() { fltk.Wait(0.1) }

// centered is where a window of the given size sits in the middle of the
// main screen. FLTK otherwise opens unplaced windows in the top-left corner.
func centered(w, h int) (int, int) {
	sx, sy, sw, sh := fltk.ScreenWorkArea(0)
	return sx + (sw-w)/2, sy + (sh-h)/2
}

// label keeps FLTK from reading an @ in a caption as the name of a symbol.
func label(s string) string { return strings.ReplaceAll(s, "@", "@@") }

// menuText also escapes what a menu path would read as structure: a slash
// starts a submenu, a leading underscore draws a divider. An & still marks
// the letter to underline.
func menuText(s string) string {
	s = strings.ReplaceAll(label(s), "\\", "\\\\")
	s = strings.ReplaceAll(s, "/", "\\/")
	if strings.HasPrefix(s, "_") {
		s = "\\" + s
	}
	return s
}

// itemText is a ComboBox entry, where an & is only an &.
func itemText(s string) string { return menuText(strings.ReplaceAll(s, "&", "&&")) }

func window(f *guiObject) *fltk.Window {
	if f == nil {
		return nil
	}
	w, _ := f.widget.(*fltk.Window)
	return w
}

func shown(f *guiObject) bool {
	w := window(f)
	return w != nil && w.IsShown()
}

func showWindow(f *guiObject, modal bool) {
	w := window(f)
	if modal {
		w.SetModal()
	} else {
		w.SetNonModal()
	}
	w.Show()
}

func hideWindow(f *guiObject) {
	if w := window(f); w != nil {
		w.Hide()
	}
}

func focus(o *guiObject) {
	if w, ok := o.widget.(interface{ TakeFocus() int }); ok {
		w.TakeFocus()
	}
}

func addTimeout(secs float64, fn func()) error {
	fltk.AddTimeout(secs, fn)
	return nil
}

type geometry interface {
	X() int
	Y() int
	W() int
	H() int
}

// origin is where a container's children are measured from, in the window's
// coordinates: a Form's corner, or a Frame's or Page's.
func origin(p *guiObject) (int, int) {
	if p == nil || p.kind == "Form" {
		return 0, 0
	}
	if anchor, ok := p.state.(*fltk.Box); ok && p.kind == "Scroll" {
		// What a Scroll holds is measured from its content's corner, which
		// moves as it scrolls; the anchor sits there and moves with it.
		return anchor.X(), anchor.Y()
	}
	if w, ok := p.widget.(geometry); ok {
		return w.X(), w.Y()
	}
	return 0, 0
}

// bounds is where an object goes, in the window's coordinates.
func bounds(o *guiObject) (x, y, w, h int) {
	if o.kind == "Page" {
		t := o.parent.widget.(*fltk.Tabs)
		return t.X(), t.Y() + tabRow, t.W(), t.H() - tabRow
	}
	ox, oy := origin(o.parent)
	x, y = ox+propInt(o, "left", 0), oy+propInt(o, "top", 0)
	w, h = propInt(o, "width", o.spec.w), propInt(o, "height", o.spec.h)
	if o.kind == "Menu" && w <= 0 {
		if o.parent != nil && o.parent.kind == "Form" {
			if win := window(o.parent); win != nil {
				w = win.W()
			}
		} else if g, ok := o.parent.widget.(geometry); ok {
			w = g.W()
		}
	}
	return x, y, w, h
}

// group is the FLTK group a container's children go into.
func group(o *guiObject) *fltk.Group {
	switch w := o.widget.(type) {
	case *fltk.Window:
		return &w.Group
	case *fltk.Tabs:
		return &w.Group
	case *fltk.Scroll:
		return &w.Group
	case *fltk.Tile:
		return &w.Group
	case *fltk.Group:
		return w
	}
	return nil
}

// destroyWidget frees what a removed control had on screen.
func destroyWidget(o *guiObject) {
	d, _ := o.widget.(interface{ Destroy() })
	win := window(o.form())
	canvas := o.kind == "Canvas" // forget destroys a Canvas itself
	forget(o)
	if d != nil && !canvas {
		d.Destroy()
	}
	if win != nil {
		win.Redraw()
	}
}

// restack puts a container's widgets in the order of its children, which is
// the order FLTK draws them in and offers them events, last on top.
func restack(p *guiObject) {
	g := group(p)
	if g == nil {
		return
	}
	for _, c := range p.children {
		if w, ok := c.widget.(fltk.Widget); ok {
			g.Remove(w)
			g.Add(w)
		}
	}
	g.Redraw()
}

// forget drops the widgets of an object and everything in it, freeing what
// FLTK would not free with them: the images, which widgets only borrow, and
// a Canvas's drawing handler, which go-fltk only lets go of when the Canvas
// itself is destroyed. The caller destroys the window.
func forget(o *guiObject) {
	switch st := o.state.(type) {
	case *canvas:
		for _, img := range st.images {
			img.Destroy()
		}
	case scalable:
		st.Destroy()
	}
	if b, ok := o.widget.(*fltk.Box); ok && o.kind == "Canvas" {
		b.Destroy()
	}
	o.widget, o.state = nil, nil
	o.mouse.armed, o.mouse.dropping = false, false
	for _, c := range o.children {
		forget(c)
	}
}

// releaseForm frees a closed form's window from the event loop, outside the
// callback that closed it, unless it has been shown again by then.
func releaseForm(f *guiObject, gone func()) {
	fltk.AddTimeout(0, func() {
		win := window(f)
		if win == nil || win.IsShown() {
			return
		}
		forget(f)
		win.Destroy()
		gone()
	})
}

func buildForm(f *guiObject) error {
	if f.widget != nil {
		return nil
	}
	w, h := propInt(f, "width", 360), propInt(f, "height", 240)
	x, y := centered(w, h)
	if f.placed {
		x, y = propInt(f, "left", 0), propInt(f, "top", 0)
	}
	win := fltk.NewWindowWithPosition(x, y, w, h)
	f.widget = win
	catchDrops(f, w, h)
	win.SetCallback(func() { f.app.requestClose(f) })
	listen(f)
	win.SetResizeHandler(func() { f.app.fire(f, "onResize") })
	err := buildChildren(f, &win.Group)
	win.End()
	if err == nil {
		err = f.applyStoredProps()
	}
	if err != nil {
		win.Destroy()
		forget(f)
		return err
	}
	return nil
}

// buildChildren builds what o holds into g, which is current, and then says
// how g's children follow it when it is resized.
func buildChildren(o *guiObject, g *fltk.Group) error {
	for _, c := range o.children {
		if err := build(c); err != nil {
			return err
		}
	}
	switch o.kind {
	case "Tabs", "Scroll", "Splitter":
		// Pages fill their Tabs; a Scroll scrolls rather than stretching
		// what it holds; a Splitter's panes share it out among themselves.
		return nil
	}
	// FLTK grows a group by growing its "resizable" child and moving the
	// others out of its way. With the controls marked grow, that child is
	// an invisible box over all of them: they stretch, controls in the same
	// columns widen with them and controls in the same rows grow taller,
	// and the rest keep their size and move with the edge beyond them.
	var x0, y0, x1, y1 int
	growing := false
	for _, c := range o.children {
		w, ok := c.widget.(geometry)
		if !ok || !propBool(c, "grow") {
			continue
		}
		if !growing || w.X() < x0 {
			x0 = w.X()
		}
		if !growing || w.Y() < y0 {
			y0 = w.Y()
		}
		if !growing || w.X()+w.W() > x1 {
			x1 = w.X() + w.W()
		}
		if !growing || w.Y()+w.H() > y1 {
			y1 = w.Y() + w.H()
		}
		growing = true
	}
	gx, gy, gw, gh := g.X(), g.Y(), g.W(), g.H()
	if o.kind == "Form" {
		gx, gy = 0, 0 // a window's children are placed relative to it
	}
	switch {
	case growing:
		g.Resizable(fltk.NewBox(fltk.NO_BOX, x0, y0, x1-x0, y1-y0))
	case o.kind == "Form" && propBool(o, "resizable"):
		g.Resizable(g) // nothing marked grow: everything scales
	case o.kind == "Form":
		return nil // a window with no resizable child keeps its size
	default:
		// A Frame or Page whose children do not grow keeps them where they
		// are: a point in its far corner takes all the growing.
		g.Resizable(fltk.NewBox(fltk.NO_BOX, gx+gw-1, gy+gh-1, 1, 1))
	}
	if o.kind == "Form" {
		win := o.widget.(*fltk.Window)
		win.SetSizeRange(gw, gh, 0, 0, 0, 0, false)
	}
	return nil
}

// buildLive puts an object on a form that is already built.
func buildLive(o *guiObject) error {
	g := group(o.parent)
	if g == nil {
		return fmt.Errorf("gui: cannot add to a %s", o.parent.kind)
	}
	var err error
	if w, ok := o.widget.(fltk.Widget); ok {
		g.Add(w) // moving a built object from one parent to another
		err = applyProp(o, "left", o.props["left"])
	} else {
		g.Begin()
		err = build(o)
		g.End()
	}
	// End leaves the group's parent current, and a window made while a
	// group is current would become part of it.
	if win := window(o.form()); win != nil {
		win.End()
		win.Redraw()
	}
	return err
}

func build(o *guiObject) error {
	x, y, w, h := bounds(o)
	a := o.app
	switch o.kind {
	case "Label":
		o.widget = fltk.NewBox(fltk.NO_BOX, x, y, w, h)
	case "Button":
		if propBool(o, "default") {
			b := fltk.NewReturnButton(x, y, w, h)
			b.SetCallback(func() { a.fire(o, "onClick") })
			o.widget = b
		} else {
			b := fltk.NewButton(x, y, w, h)
			b.SetCallback(func() { a.fire(o, "onClick") })
			o.widget = b
		}
	case "TextBox":
		buildTextBox(o, x, y, w, h)
	case "CheckBox":
		b := fltk.NewCheckButton(x, y, w, h)
		b.SetCallback(func() { a.fire(o, "onChange") })
		o.widget = b
	case "RadioButton":
		b := fltk.NewRadioRoundButton(x, y, w, h)
		b.SetCallback(func() { a.fire(o, "onChange") })
		o.widget = b
	case "ComboBox":
		o.widget = fltk.NewChoice(x, y, w, h)
	case "ListBox":
		b := fltk.NewHoldBrowser(x, y, w, h)
		b.SetFormatChar(0)
		b.SetCallbackCondition(fltk.WhenChanged | fltk.WhenNotChanged | fltk.WhenRelease)
		last := 0
		b.SetCallback(func() {
			if v := b.Value(); v != last {
				last = v
				a.fire(o, "onChange")
			} else if v > 0 && fltk.EventType() == fltk.RELEASE && fltk.EventClicks() > 0 {
				a.fire(o, "onDoubleClick")
			}
		})
		o.widget = b
	case "Slider":
		s := fltk.NewValueSlider(x, y, w, h)
		if propBool(o, "vertical") {
			s.SetType(fltk.VERT_NICE_SLIDER)
		} else {
			s.SetType(fltk.HOR_NICE_SLIDER)
		}
		s.SetCallback(func() { a.fire(o, "onChange") })
		o.widget = s
	case "Spinner":
		s := fltk.NewSpinner(x, y, w, h)
		s.SetCallback(func() { a.fire(o, "onChange") })
		o.widget = s
	case "ProgressBar":
		p := fltk.NewProgress(x, y, w, h)
		p.SetSelectionColor(fltk.ColorFromRgb(70, 130, 200))
		o.widget = p
	case "Image":
		o.widget = fltk.NewBox(fltk.NO_BOX, x, y, w, h)
	case "Menu":
		o.widget = fltk.NewMenuBar(x, y, w, h)
	case "Tree":
		buildTree(o, x, y, w, h)
	case "Table":
		buildTable(o, x, y, w, h)
	case "Canvas":
		buildCanvas(o, x, y, w, h)
	case "Scroll":
		sc := fltk.NewScroll(x, y, w, h)
		o.widget = sc
		// FLTK measures a Scroll's position from what it holds; a box of no
		// size at its corner makes that the corner, and marks where it is.
		o.state = fltk.NewBox(fltk.NO_BOX, x, y, 0, 0)
		err := buildChildren(o, &sc.Group)
		sc.End()
		if err != nil {
			return err
		}
	case "Splitter":
		t := fltk.NewTile(x, y, w, h)
		o.widget = t
		err := buildChildren(o, &t.Group)
		t.End()
		if err != nil {
			return err
		}
	case "Frame", "Page", "Panel":
		g := fltk.NewGroup(x, y, w, h)
		switch o.kind {
		case "Frame":
			g.SetBox(fltk.ENGRAVED_FRAME)
			g.SetAlign(fltk.ALIGN_TOP_LEFT | fltk.ALIGN_INSIDE)
		case "Panel":
			g.SetBox(fltk.FLAT_BOX)
		}
		o.widget = g
		err := buildChildren(o, g)
		g.End()
		if err != nil {
			return err
		}
	case "Tabs":
		t := fltk.NewTabs(x, y, w, h)
		t.SetCallback(func() { a.fire(o, "onChange") })
		o.widget = t
		err := buildChildren(o, &t.Group)
		t.End()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("gui: cannot put a %s on screen", o.kind)
	}
	listen(o)
	return o.applyStoredProps()
}

// The colours of Lua, a style a letter: text, keywords, the standard
// library, strings, numbers, comments.
var luaStyles = []fltk.StyleTableEntry{
	{Color: fltk.ColorFromRgb(0, 0, 0), Font: fltk.COURIER, Size: 14},
	{Color: fltk.ColorFromRgb(0, 0, 160), Font: fltk.COURIER_BOLD, Size: 14},
	{Color: fltk.ColorFromRgb(0, 110, 60), Font: fltk.COURIER, Size: 14},
	{Color: fltk.ColorFromRgb(160, 40, 0), Font: fltk.COURIER, Size: 14},
	{Color: fltk.ColorFromRgb(0, 120, 140), Font: fltk.COURIER, Size: 14},
	{Color: fltk.ColorFromRgb(120, 120, 120), Font: fltk.COURIER_ITALIC, Size: 14},
}

// colourLua colours an editor's text as Lua, again after every change. The
// whole text is classified each time: the code of a form is short.
func colourLua(e *fltk.TextEditor, buf *fltk.TextBuffer) {
	style := fltk.NewTextBuffer()
	restyle := func() {
		classes := luasyntax.Classes(buf.Text())
		b := make([]byte, len(classes))
		for i, c := range classes {
			b[i] = 'A' + byte(c)
		}
		style.SetText(string(b))
	}
	restyle()
	e.SetHighlightData(style, luaStyles)
	buf.AddModifyCallback(func(int, int, int, int, string) { restyle() })
}

func buildTextBox(o *guiObject, x, y, w, h int) {
	a := o.app
	readOnly := propBool(o, "readOnly")
	if propBool(o, "multiLine") {
		buf := fltk.NewTextBuffer()
		if readOnly {
			d := fltk.NewTextDisplay(x, y, w, h)
			d.SetBuffer(buf)
			d.SetWrapMode(fltk.WRAP_AT_BOUNDS)
			o.widget = d
			return
		}
		e := fltk.NewTextEditor(x, y, w, h)
		e.SetBuffer(buf)
		code := propString(o, "syntax") != "" || propBool(o, "lineNumbers")
		if code {
			// Code is not wrapped: a long line scrolls.
			e.SetWrapMode(fltk.WRAP_NONE)
			e.SetTextFont(fltk.COURIER)
		} else {
			e.SetWrapMode(fltk.WRAP_AT_BOUNDS)
		}
		if propBool(o, "lineNumbers") {
			e.SetLinenumberWidth(44)
			e.SetLinenumberSize(12)
			e.SetLinenumberFgcolor(fltk.ColorFromRgb(130, 130, 130))
			e.SetLinenumberBgcolor(fltk.ColorFromRgb(238, 238, 238))
		}
		if propString(o, "syntax") == "lua" {
			colourLua(e, buf)
		}
		e.SetCallbackCondition(fltk.WhenChanged)
		e.SetCallback(func() { a.fire(o, "onChange") })
		e.SetDrawHandler(func(base func()) {
			base()
			drawOver(o)
		})
		o.widget = e
		return
	}
	var in *fltk.Input
	switch {
	case readOnly:
		in = &fltk.NewOutput(x, y, w, h).Input
	case propBool(o, "password"):
		in = &fltk.NewSecretInput(x, y, w, h).Input
	default:
		in = fltk.NewInput(x, y, w, h)
	}
	if !readOnly {
		in.SetCallbackCondition(fltk.WhenChanged)
		in.SetCallback(func() { a.fire(o, "onChange") })
	}
	o.widget = in
}

type labelled interface {
	SetLabel(string)
	SetLabelColor(fltk.Color)
	SetLabelSize(int)
	SetLabelFont(fltk.Font)
	SetColor(fltk.Color)
	SetTooltip(string)
	Redraw()
}

// applyProp carries a property over to the widget.
func applyProp(o *guiObject, name string, value lua.LValue) error {
	w, _ := o.widget.(labelled)
	switch name {
	case "caption":
		text := lua.LVAsString(value)
		switch {
		case o.kind == "Form":
			window(o).SetLabel(text)
		case o.kind == "TextBox" || o.kind == "Menu":
			// A text box's caption would be drawn outside it; it has none.
		case w != nil:
			w.SetLabel(label(text))
			if o.parent != nil && o.parent.widget != nil {
				o.parent.widget.(labelled).Redraw()
			}
		}
	case "visible":
		if o.kind == "Form" {
			if lua.LVAsBool(value) {
				window(o).Show()
			} else {
				window(o).Hide()
			}
		} else if v, ok := o.widget.(interface {
			Show()
			Hide()
		}); ok {
			if lua.LVAsBool(value) {
				v.Show()
			} else {
				v.Hide()
			}
		}
	case "enabled":
		if v, ok := o.widget.(interface {
			Activate()
			Deactivate()
		}); ok {
			if lua.LVAsBool(value) {
				v.Activate()
			} else {
				v.Deactivate()
			}
		}
	case "left", "top", "width", "height":
		resize(o)
	case "tooltip":
		if w != nil {
			w.SetTooltip(lua.LVAsString(value))
		}
	case "color", "textColor":
		if value == lua.LNil || w == nil {
			return nil
		}
		r, g, b, err := parseColor(lua.LVAsString(value))
		if err != nil {
			return err
		}
		c := fltk.ColorFromRgb(r, g, b)
		if name == "color" {
			w.SetColor(c)
			// A Label is drawn on whatever is under it until it is given a
			// colour of its own.
			if b, ok := o.widget.(*fltk.Box); ok && o.kind == "Label" {
				b.SetBox(fltk.FLAT_BOX)
			}
		} else {
			w.SetLabelColor(c)
			if t, ok := o.widget.(interface{ SetTextColor(fltk.Color) }); ok {
				t.SetTextColor(c)
			}
		}
		w.Redraw()
	case "fontSize":
		if value == lua.LNil || w == nil {
			return nil
		}
		size := int(lua.LVAsNumber(value))
		w.SetLabelSize(size)
		if t, ok := o.widget.(interface{ SetTextSize(int) }); ok {
			t.SetTextSize(size)
		}
		w.Redraw()
	case "font":
		if value == lua.LNil || w == nil {
			return nil
		}
		f := map[string]fltk.Font{"sans": fltk.HELVETICA, "serif": fltk.TIMES, "mono": fltk.COURIER}[lua.LVAsString(value)]
		w.SetLabelFont(f)
		if t, ok := o.widget.(interface{ SetTextFont(fltk.Font) }); ok {
			t.SetTextFont(f)
		}
		w.Redraw()
	case "align":
		alignLabel(o)
	case "image":
		if o.kind == "Button" || o.kind == "Label" {
			if err := setImage(o, lua.LVAsString(value)); err != nil {
				return err
			}
			alignLabel(o)
		}
	case "text":
		return setText(o, lua.LVAsString(value))
	case "checked":
		if b, ok := o.widget.(interface{ SetValue(bool) }); ok {
			b.SetValue(lua.LVAsBool(value))
		}
	case "items":
		if o.kind == "Tree" {
			refreshTree(o, lua.LVAsString(o.get("path")))
			return nil
		}
		return setItems(o)
	case "path":
		if _, ok := o.widget.(*fltk.HoldBrowser); ok && o.kind == "Tree" {
			selectPath(o, lua.LVAsString(value))
		}
	case "columns", "rows", "columnWidths":
		if _, ok := o.widget.(*fltk.TableRow); ok {
			setTable(o)
			setTableSelected(o, tableSelected(o))
		}
	case "selected":
		setSelected(o, int(lua.LVAsNumber(value)))
	case "min", "max", "step", "value":
		setNumber(o, name, float64(lua.LVAsNumber(value)))
	case "cursor":
		setCursor(o, int(lua.LVAsNumber(value)))
	case "line":
		if e, ok := o.widget.(*fltk.TextEditor); ok {
			n := int(lua.LVAsNumber(value))
			if n < 1 {
				n = 1
			}
			setCursor(o, e.Buffer().SkipLines(0, n-1))
		}
	case "file":
		return setImage(o, lua.LVAsString(value))
	case "fit":
		return setImage(o, propString(o, "file"))
	}
	return nil
}

func resize(o *guiObject) {
	if o.kind == "Page" {
		return // a page is wherever its Tabs puts it
	}
	x, y, w, h := bounds(o)
	switch widget := o.widget.(type) {
	case *fltk.Window:
		if w <= 0 || h <= 0 {
			return
		}
		// A form the script never placed stays where it was put.
		if !o.placed {
			x, y = widget.X(), widget.Y()
		} else {
			x, y = propInt(o, "left", 0), propInt(o, "top", 0)
		}
		widget.Resize(x, y, w, h)
	case interface{ Resize(int, int, int, int) }:
		widget.Resize(x, y, w, h)
		if win := window(o.form()); win != nil {
			win.Redraw()
		}
	}
}

func setText(o *guiObject, text string) error {
	switch w := o.widget.(type) {
	case *fltk.Input:
		w.SetValue(text)
	case *fltk.TextEditor:
		w.Buffer().SetText(text)
	case *fltk.TextDisplay:
		w.Buffer().SetText(text)
	default:
		// A ComboBox or ListBox picks the item with that text.
		if o.kind == "ComboBox" || o.kind == "ListBox" {
			for i, it := range propItems(o) {
				if it == text {
					o.props["selected"] = lua.LNumber(i + 1)
					setSelected(o, i+1)
				}
			}
		}
	}
	return nil
}

// cursorOf is where a TextBox's cursor is: how many bytes come before it.
func cursorOf(o *guiObject) (int, bool) {
	switch w := o.widget.(type) {
	case *fltk.TextEditor:
		return w.GetInsertPosition(), true
	case *fltk.TextDisplay:
		return w.GetInsertPosition(), true
	case *fltk.Input:
		return w.InsertPosition(), true
	}
	return 0, false
}

func setCursor(o *guiObject, pos int) {
	if pos < 0 {
		pos = 0
	}
	switch w := o.widget.(type) {
	case *fltk.TextEditor:
		if n := w.Buffer().Length(); pos > n {
			pos = n
		}
		w.Buffer().UnSelect()
		w.SetInsertPosition(pos)
		w.ShowInsertPosition()
	case *fltk.Input:
		if n := len(w.Value()); pos > n {
			pos = n
		}
		w.SetInsertPosition(pos, pos)
	}
}

// selectText selects bytes i to j of a TextBox, counted as string.sub
// counts them, and leaves the cursor after them.
func selectText(o *guiObject, i, j int) {
	if i < 1 {
		i = 1
	}
	from, to := i-1, j
	switch w := o.widget.(type) {
	case *fltk.TextEditor:
		if n := w.Buffer().Length(); to > n {
			to = n
		}
		if to < from {
			to = from
		}
		w.Buffer().Select(from, to)
		w.SetInsertPosition(to)
		w.ShowInsertPosition()
	case *fltk.Input:
		if n := len(w.Value()); to > n {
			to = n
		}
		if to < from {
			to = from
		}
		w.SetInsertPosition(to, from)
	}
}

func setItems(o *guiObject) error {
	switch w := o.widget.(type) {
	case *fltk.Choice:
		fillChoice(o, w)
		setSelected(o, propInt(o, "selected", 0))
	case *fltk.HoldBrowser:
		w.Clear()
		for _, it := range propItems(o) {
			w.Add(it)
		}
		setSelected(o, propInt(o, "selected", 0))
	case *fltk.MenuBar:
		w.Clear()
		items, _ := o.props["items"].(*lua.LTable)
		if items == nil {
			return nil
		}
		return addMenu(o, w, "", items)
	}
	return nil
}

// fillChoice puts a ComboBox's items in it, with none of them selected.
func fillChoice(o *guiObject, w *fltk.Choice) {
	w.Clear()
	for _, it := range propItems(o) {
		// Each entry has a callback of its own, which FLTK calls instead
		// of the widget's.
		w.Add(itemText(it), func() { o.app.fire(o, "onChange") })
	}
}

func setSelected(o *guiObject, i int) {
	switch w := o.widget.(type) {
	case *fltk.Choice:
		// The FLTK go-fltk carries takes any index, and an index outside the
		// items points FLTK at memory that is not an item, which it draws.
		// Nothing selected is what a ComboBox is just after it is filled.
		if i >= 1 && i <= len(propItems(o)) {
			w.SetValue(i - 1)
		} else if w.Value() >= 0 {
			fillChoice(o, w)
		}
		w.Redraw()
	case *fltk.HoldBrowser:
		w.SetValue(i)
	case *fltk.Tabs:
		w.SetValue(i - 1)
		w.Redraw()
	case *fltk.TableRow:
		setTableSelected(o, i)
	}
}

func setNumber(o *guiObject, name string, v float64) {
	switch w := o.widget.(type) {
	case *fltk.ValueSlider:
		switch name {
		case "min":
			w.SetMinimum(v)
		case "max":
			w.SetMaximum(v)
		case "step":
			w.SetStep(v)
		case "value":
			w.SetValue(v)
		}
	case *fltk.Spinner:
		switch name {
		case "min":
			w.SetMinimum(v)
		case "max":
			w.SetMaximum(v)
		case "step":
			w.SetStep(v)
			if v != float64(int(v)) {
				w.SetType(fltk.SPINNER_FLOAT_INPUT)
			} else {
				w.SetType(fltk.SPINNER_INT_INPUT)
			}
		case "value":
			w.SetValue(v)
		}
	case *fltk.Progress:
		switch name {
		case "min":
			w.SetMinimum(v)
		case "max":
			w.SetMaximum(v)
		case "value":
			w.SetValue(v)
		}
	}
}

// alignLabel places a Label's caption, and a Button's or Label's image
// beside its caption.
func alignLabel(o *guiObject) {
	w, ok := o.widget.(interface {
		SetAlign(fltk.Align)
		Redraw()
	})
	if !ok {
		return
	}
	var a fltk.Align
	switch o.kind {
	case "Label":
		a = map[string]fltk.Align{"left": fltk.ALIGN_LEFT, "center": fltk.ALIGN_CENTER, "right": fltk.ALIGN_RIGHT}[propString(o, "align")]
		a |= fltk.ALIGN_INSIDE | fltk.ALIGN_WRAP
	case "Button":
		a = fltk.ALIGN_CENTER
	default:
		return
	}
	if propString(o, "image") != "" {
		a |= fltk.ALIGN_IMAGE_NEXT_TO_TEXT
	}
	w.SetAlign(a)
	w.Redraw()
}

func setImage(o *guiObject, path string) error {
	b, ok := o.widget.(interface {
		SetImage(fltk.Image)
		W() int
		H() int
		Redraw()
	})
	if !ok {
		return nil
	}
	var img scalable = blank()
	if path != "" {
		var err error
		if img, err = loadImage(o.app, path); err != nil {
			return fmt.Errorf("gui: cannot load image %s: %v", path, err)
		}
	}
	// Shrunk to fit, and with fit set grown to fit too, keeping its shape.
	fit := propBool(o, "fit")
	if fit || img.W() > b.W() || img.H() > b.H() {
		img.Scale(b.W(), b.H(), true, fit)
	}
	b.SetImage(img)
	// The picture it replaces is the Image's own, and nothing else's.
	if old, ok := o.state.(scalable); ok && old != blankImage {
		old.Destroy()
	}
	o.state = img
	if img == blankImage {
		o.state = nil
	}
	b.Redraw()
	return nil
}

// blankImage is what an Image with no file shows: one transparent pixel,
// since a widget cannot be told to show no image at all.
var (
	blankImage  scalable
	blankPixels = []uint8{0, 0, 0, 0}
)

func blank() scalable {
	if blankImage == nil {
		img, err := fltk.NewRgbImage(blankPixels, 1, 1, 4)
		if err != nil {
			panic(err)
		}
		blankImage = img
	}
	return blankImage
}

type scalable interface {
	fltk.Image
	W() int
	H() int
	Scale(w, h int, proportional, canExpand bool)
	Draw(x, y, w, h int)
	Destroy()
}

// loadImage reads an image of its own. FLTK's shared images are one per
// path, so scaling one for a control would scale it for every control
// showing the same file. A fused program's images come out of its archive.
func loadImage(a *app, path string) (scalable, error) {
	if a.read != nil {
		if data, err := a.read(path); err == nil {
			return imageFromData(path, data)
		}
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return fltk.NewPngImageLoad(path)
	case ".jpg", ".jpeg":
		return fltk.NewJpegImageLoad(path)
	case ".bmp":
		return fltk.NewBmpImageLoad(path)
	case ".svg":
		return fltk.NewSvgImageLoad(path)
	}
	return fltk.NewSharedImageLoad(path)
}

func imageFromData(path string, data []byte) (scalable, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return fltk.NewPngImageFromData(data)
	case ".jpg", ".jpeg":
		return fltk.NewJpegImageFromData(data)
	case ".bmp":
		return fltk.NewBmpImageFromData(data)
	case ".svg":
		return fltk.NewSvgImageFromString(string(data))
	}
	// FLTK reads the other formats from files only.
	tmp, err := os.CreateTemp("", "tlua-image-*"+ext)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	return fltk.NewSharedImageLoad(tmp.Name())
}

// readProp reads what may have changed on screen since the script set it.
func readProp(o *guiObject, name string) (lua.LValue, bool) {
	switch name {
	case "left", "top", "width", "height":
		g, ok := o.widget.(geometry)
		if !ok {
			return nil, false
		}
		x, y := g.X(), g.Y()
		if o.kind != "Form" {
			ox, oy := origin(o.parent)
			x, y = x-ox, y-oy
		}
		return lua.LNumber(map[string]int{"left": x, "top": y, "width": g.W(), "height": g.H()}[name]), true
	case "visible":
		if o.kind == "Form" {
			return lua.LBool(shown(o)), true
		}
		if v, ok := o.widget.(interface{ Visible() bool }); ok {
			return lua.LBool(v.Visible()), true
		}
	case "enabled":
		if v, ok := o.widget.(interface{ IsActive() bool }); ok {
			return lua.LBool(v.IsActive()), true
		}
	case "path":
		if o.kind == "Tree" {
			row, _ := treeRowAt(o, o.widget.(*fltk.HoldBrowser).Value())
			return lua.LString(row.path), true
		}
	case "line", "cursor":
		pos, ok := cursorOf(o)
		if !ok {
			return nil, false
		}
		if name == "cursor" {
			return lua.LNumber(pos), true
		}
		if e, ok := o.widget.(*fltk.TextEditor); ok {
			return lua.LNumber(e.Buffer().CountLines(0, pos) + 1), true
		}
		return lua.LNumber(1), true
	case "selectedText":
		switch w := o.widget.(type) {
		case *fltk.TextEditor:
			return lua.LString(w.Buffer().GetSelectionText()), true
		case *fltk.Input:
			a, b := w.Mark(), w.InsertPosition()
			if a > b {
				a, b = b, a
			}
			return lua.LString(w.Value()[a:b]), true
		}
	case "text":
		if o.kind == "Tree" {
			row, _ := treeRowAt(o, o.widget.(*fltk.HoldBrowser).Value())
			return lua.LString(row.label), true
		}
		switch w := o.widget.(type) {
		case *fltk.Input:
			return lua.LString(w.Value()), true
		case *fltk.TextEditor:
			return lua.LString(w.Buffer().Text()), true
		case *fltk.TextDisplay:
			return lua.LString(w.Buffer().Text()), true
		}
		if o.kind == "ComboBox" || o.kind == "ListBox" {
			sel, _ := readProp(o, "selected")
			items := propItems(o)
			if i := int(lua.LVAsNumber(sel)); i >= 1 && i <= len(items) {
				return lua.LString(items[i-1]), true
			}
			return lua.LString(""), true
		}
	case "checked":
		if b, ok := o.widget.(interface{ Value() bool }); ok {
			return lua.LBool(b.Value()), true
		}
	case "selected":
		switch w := o.widget.(type) {
		case *fltk.Choice:
			return lua.LNumber(w.Value() + 1), true
		case *fltk.HoldBrowser:
			return lua.LNumber(w.Value()), true
		case *fltk.Tabs:
			return lua.LNumber(w.Value() + 1), true
		case *fltk.TableRow:
			return lua.LNumber(tableSelected(o)), true
		}
	case "value":
		if v, ok := o.widget.(interface{ Value() float64 }); ok {
			return lua.LNumber(v.Value()), true
		}
	}
	return nil, false
}

// addMenu adds a list of menu items under prefix. An item is {"&Open", fn},
// with shortcut, checked and enabled as named fields, or {"&File", {...}} for
// a submenu, or "-" for a line between two items.
func addMenu(o *guiObject, mb *fltk.MenuBar, prefix string, items *lua.LTable) error {
	n := items.Len()
	for i := 1; i <= n; i++ {
		v := items.RawGetInt(i)
		if v == lua.LString("-") {
			continue
		}
		item, ok := v.(*lua.LTable)
		if !ok {
			return fmt.Errorf("gui: a menu item is a table like {\"&Open\", fn} or \"-\", not a %s", v.Type())
		}
		text := lua.LVAsString(item.RawGetInt(1))
		path := prefix + menuText(text)
		flags := 0
		if i < n && items.RawGetInt(i+1) == lua.LString("-") {
			flags |= fltk.MENU_DIVIDER
		}
		if item.RawGetString("enabled") == lua.LFalse {
			flags |= fltk.MENU_INACTIVE
		}
		if sub, ok := item.RawGetInt(2).(*lua.LTable); ok {
			mb.AddEx(path, 0, func() {}, flags|fltk.SUBMENU)
			if err := addMenu(o, mb, path+"/", sub); err != nil {
				return err
			}
			continue
		}
		fn, _ := item.RawGetInt(2).(*lua.LFunction)
		if fn == nil {
			fn, _ = item.RawGetString("onClick").(*lua.LFunction)
		}
		shortcut, err := parseShortcut(lua.LVAsString(item.RawGetString("shortcut")))
		if err != nil {
			return err
		}
		if c := item.RawGetString("checked"); c != lua.LNil {
			flags |= fltk.MENU_TOGGLE
			if lua.LVAsBool(c) {
				flags |= fltk.MENU_VALUE
			}
		}
		name := item.RawGetString("name")
		mb.AddEx(path, shortcut, func() {
			on := mb.Mode(mb.Value())&fltk.MENU_VALUE != 0
			if flags&fltk.MENU_TOGGLE != 0 {
				// Kept in the script's table, so that assigning the items
				// again leaves it as the user left it.
				item.RawSetString("checked", lua.LBool(on))
			}
			if fn != nil {
				o.app.call(fn, lua.LString(text), lua.LBool(on))
			}
			// A menu written down in a layout has no functions in it; its
			// items are told apart by name in the Menu's onClick.
			o.app.fire(o, "onClick", name, lua.LString(text), lua.LBool(on))
		}, flags)
	}
	return nil
}

var keyNames = map[int]string{
	fltk.ESCAPE: "Escape", fltk.TAB: "Tab", fltk.ENTER_KEY: "Enter",
	fltk.HOME: "Home", fltk.END: "End", fltk.LEFT: "Left", fltk.UP: "Up",
	fltk.RIGHT: "Right", fltk.DOWN: "Down", fltk.PAGE_UP: "PageUp",
	fltk.PAGE_DOWN: "PageDown", fltk.DELETE: "Delete",
	fltk.BACKSPACE: "Backspace", fltk.INSERT: "Insert", ' ': "Space",
	fltk.F1: "F1", fltk.F2: "F2", fltk.F3: "F3", fltk.F4: "F4", fltk.F5: "F5",
	fltk.F6: "F6", fltk.F7: "F7", fltk.F8: "F8", fltk.F9: "F9",
	fltk.F10: "F10", fltk.F11: "F11", fltk.F12: "F12",
}

// keyName writes a key the way a shortcut is written: "Ctrl+s", "F5",
// "Shift+Tab". On a Mac the Command key is "Cmd".
func keyName(key, state int) string {
	base, ok := keyNames[key]
	if !ok {
		if key > ' ' && key < 127 {
			base = string(rune(key))
		} else {
			base = fmt.Sprintf("Key%d", key)
		}
	}
	if mods := modNames(state); mods != "" {
		return mods + "+" + base
	}
	return base
}

// modNames names the modifier keys held, as "Ctrl+Shift"; "" for none.
func modNames(state int) string {
	var mods []string
	if state&fltk.CTRL != 0 {
		mods = append(mods, "Ctrl")
	}
	if state&fltk.ALT != 0 {
		mods = append(mods, "Alt")
	}
	if state&fltk.META != 0 {
		if goruntime.GOOS == "darwin" {
			mods = append(mods, "Cmd")
		} else {
			mods = append(mods, "Meta")
		}
	}
	if state&fltk.SHIFT != 0 {
		mods = append(mods, "Shift")
	}
	return strings.Join(mods, "+")
}

// parseShortcut reads "Cmd+O", "Ctrl+Shift+Z" or "F5". Cmd is Command on a
// Mac and Ctrl elsewhere, which is what a menu shortcut usually wants.
func parseShortcut(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	parts := strings.Split(s, "+")
	sc := 0
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(p) {
		case "ctrl":
			sc |= fltk.CTRL
		case "alt":
			sc |= fltk.ALT
		case "shift":
			sc |= fltk.SHIFT
		case "cmd":
			if goruntime.GOOS == "darwin" {
				sc |= fltk.META
			} else {
				sc |= fltk.CTRL
			}
		case "meta":
			sc |= fltk.META
		default:
			return 0, fmt.Errorf("gui: shortcut %q: no modifier %q (Cmd, Ctrl, Alt, Shift, Meta)", s, p)
		}
	}
	key := parts[len(parts)-1]
	for code, name := range keyNames {
		if strings.EqualFold(name, key) {
			return sc | code, nil
		}
	}
	if r := []rune(key); len(r) == 1 {
		return sc | int(unicode.ToLower(r[0])), nil
	}
	return 0, fmt.Errorf("gui: shortcut %q: no key %q", s, key)
}

func onKey(f *guiObject) bool {
	if f.events["onKey"] == nil {
		return false
	}
	name := keyName(fltk.EventKey(), fltk.EventState())
	return lua.LVAsBool(f.app.fire(f, "onKey", lua.LString(name), lua.LString(fltk.EventText())))
}

// dialog shows a message with a row of buttons, and a line to type into when
// input is set. It returns the index of the button pressed, or -1 when the
// box was closed some other way, and what was typed.
func dialog(title, message string, buttons []string, input bool, deflt string) (int, string, error) {
	const pad, gap, bw, bh = 16, 8, 88, 28
	win := fltk.NewWindow(400, 200, title)
	defer win.Destroy()
	win.SetModal()
	msg := fltk.NewBox(fltk.NO_BOX, pad, pad, 10, 10, label(message))
	msg.SetAlign(fltk.ALIGN_TOP_LEFT | fltk.ALIGN_INSIDE)
	mw, mh := msg.MeasureLabel()

	w := mw
	if rowW := len(buttons)*(bw+gap) - gap; rowW > w {
		w = rowW
	}
	minW := 200
	if input {
		minW = 320
	}
	if minW > w {
		w = minW
	}
	w += 2 * pad
	y := pad
	msg.Resize(pad, y, w-2*pad, mh)
	y += mh + gap + gap/2

	var in *fltk.Input
	if input {
		in = fltk.NewInput(pad, y, w-2*pad, bh)
		in.SetValue(deflt)
		y += bh + 2*gap
	} else {
		y += gap
	}

	pressed := -1
	bx := w - pad - len(buttons)*(bw+gap) + gap
	var first *fltk.ReturnButton
	for i, text := range buttons {
		i := i
		done := func() { pressed = i; win.Hide() }
		if i == 0 {
			first = fltk.NewReturnButton(bx, y, bw, bh, text)
			first.SetCallback(done)
		} else {
			b := fltk.NewButton(bx, y, bw, bh, text)
			b.SetCallback(done)
		}
		bx += bw + gap
	}
	h := y + bh + pad
	win.End()
	x, wy := centered(w, h)
	win.Resize(x, wy, w, h)
	win.SetCallback(func() { pressed = -1; win.Hide() })
	win.Show()
	// The line to type into has the keyboard; otherwise the first button
	// does, so that Tab and Space move along the row from there.
	if in != nil {
		in.TakeFocus()
	} else {
		first.TakeFocus()
	}
	for win.IsShown() {
		wait()
	}
	text := ""
	if in != nil {
		text = in.Value()
	}
	return pressed, text, nil
}

func colorDialog(title string, r, g, b uint8) (string, bool, error) {
	r, g, b, ok := fltkcolor.Choose(title, r, g, b)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b), ok, nil
}

func fileDialog(opts fileOptions) ([]string, error) {
	c := fltk.NewNativeFileChooser()
	defer c.Destroy()
	switch opts.mode {
	case "open":
		c.SetType(fltk.NativeFileChooser_BROWSE_FILE)
	case "openmulti":
		c.SetType(fltk.NativeFileChooser_BROWSE_MULTI_FILE)
	case "save":
		c.SetType(fltk.NativeFileChooser_BROWSE_SAVE_FILE)
		c.SetOptions(fltk.NativeFileChooser_SAVEAS_CONFIRM | fltk.NativeFileChooser_NEW_FOLDER)
	case "dir":
		c.SetType(fltk.NativeFileChooser_BROWSE_DIRECTORY)
	}
	if opts.title != "" {
		c.SetTitle(opts.title)
	}
	if opts.filter != "" {
		c.SetFilter(opts.filter)
	}
	if opts.dir != "" {
		c.SetDirectory(opts.dir)
	}
	if opts.file != "" {
		c.SetPresetFile(opts.file)
	}
	switch c.Show() {
	case 0:
		return c.Filenames(), nil
	case 1:
		return nil, nil
	default:
		return nil, fmt.Errorf("gui: the file dialog could not be shown")
	}
}
