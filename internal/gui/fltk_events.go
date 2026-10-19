//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import (
	"image"

	"github.com/pwiecz/go-fltk"
	lua "github.com/yuin/gopher-lua"
)

// The events that follow what a window and its controls do rather than what
// the user does to them: shown and hidden, resized, painted, given the
// keyboard or losing it. FLTK tells every widget about most of them through
// the one handler listen installs (see observe); two, resizing and drawing,
// cost a handler of their own, so they are put on a widget only once a
// script sets onResize or onPaint for it (hookEvents); onScroll is found
// out when the Scroll is drawn.

// hookEvents puts the handlers that onResize, onPaint and onScroll need on
// an object's widget, if a script has set them and they are not there yet.
func hookEvents(o *guiObject) {
	if o.events["onResize"] != nil {
		hookResize(o)
	}
	if o.events["onPaint"] != nil || o.events["onScroll"] != nil {
		hookDraw(o)
	}
}

// attempt runs fn and says whether it worked: some FLTK widgets cannot take
// the handlers, and say so by panicking.
func attempt(fn func()) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	fn()
	return true
}

// hookResize makes an object raise onResize when its size changes. A Form
// has it from the moment it is built (see buildForm).
func hookResize(o *guiObject) {
	if o.hooked.resize || o.kind == "Form" {
		return
	}
	w, ok := o.widget.(interface {
		SetResizeHandler(func())
		W() int
		H() int
	})
	if !ok || !attempt(func() { w.SetResizeHandler(func() { resized(o) }) }) {
		return
	}
	o.hooked.resize = true
	o.width, o.height = w.W(), w.H()
}

// resized raises onResize when the widget's size is not the one the script
// last heard of. A handler that sizes its own control again is not asked
// about that.
func resized(o *guiObject) {
	g, ok := o.widget.(geometry)
	if !ok || (g.W() == o.width && g.H() == o.height) || quiet(o) {
		return
	}
	o.width, o.height = g.W(), g.H()
	if o.resizing {
		return
	}
	o.resizing = true
	defer func() { o.resizing = false }()
	o.app.fire(o, "onResize")
}

// hookDraw makes an object's widget draw itself and then raise onPaint, for
// the script to draw over it, and notice when a Scroll has been scrolled. A
// widget given a draw handler has to be destroyed by forget rather than
// left to FLTK, which go-fltk does not clean up after.
func hookDraw(o *guiObject) {
	if o.hooked.draw || o.kind == "Canvas" { // a Canvas has onDraw
		return
	}
	w, ok := o.widget.(interface{ SetDrawHandler(func(func())) })
	if !ok || !attempt(func() {
		w.SetDrawHandler(func(base func()) {
			base()
			paintOver(o)
			scrolledTo(o)
		})
	}) {
		return
	}
	o.hooked.draw = true
	if sc, ok := o.widget.(*fltk.Scroll); ok {
		o.scrolled.x, o.scrolled.y = sc.XPosition(), sc.YPosition()
	}
}

// scrolledTo raises onScroll when a Scroll is drawn somewhere other than
// where the script last heard it was. It does so from the event loop, not
// in the middle of drawing, and once for however far it went meanwhile.
func scrolledTo(o *guiObject) {
	sc, ok := o.widget.(*fltk.Scroll)
	if !ok || o.events["onScroll"] == nil || o.scrolled.pending {
		return
	}
	if sc.XPosition() == o.scrolled.x && sc.YPosition() == o.scrolled.y {
		return
	}
	o.scrolled.pending = true
	fltk.AddTimeout(0, func() {
		o.scrolled.pending = false
		sc, ok := o.widget.(*fltk.Scroll)
		if !ok {
			return // gone before the loop got to it
		}
		x, y := sc.XPosition(), sc.YPosition()
		if x == o.scrolled.x && y == o.scrolled.y {
			return
		}
		o.scrolled.x, o.scrolled.y = x, y
		o.app.fire(o, "onScroll", lua.LNumber(x), lua.LNumber(y))
	})
}

// paintBounds is where an object is, in the coordinates it draws in: a
// control's are its window's, and a window draws from its own corner.
func paintBounds(o *guiObject) (x, y, w, h int) {
	g, ok := o.widget.(geometry)
	if !ok {
		return 0, 0, 0, 0
	}
	if o.kind == "Form" {
		return 0, 0, g.W(), g.H()
	}
	return g.X(), g.Y(), g.W(), g.H()
}

// paintOver is onPaint: the widget has been drawn, and the script draws on
// top of it, inside it.
func paintOver(o *guiObject) {
	if o.widget == nil || o.events["onPaint"] == nil {
		return
	}
	c, _ := o.overlay.(*canvas)
	if c == nil {
		c = &canvas{images: map[string]scalable{}, sources: map[string]*image.NRGBA{}}
		c.g = drawingAPI(o, c)
		o.overlay = c
	}
	x, y, w, h := paintBounds(o)
	drawOn(o, c, x, y, w, h, "onPaint")
}

// quiet says whether an object's events are being held back: its form has
// not finished loading, and onLoad is the first thing any of them hears.
// What happens meanwhile is noted, and told when the form is ready
// (announce).
func quiet(o *guiObject) bool {
	f := o.form()
	return f != nil && !f.ready
}

// observe tells an object of the events its widget is sent that are about
// the object rather than something the user did: it is only told, and what
// the widget does about them is left to the widget.
func observe(o *guiObject, e fltk.Event) {
	a := o.app
	switch e {
	case fltk.SHOW:
		if o.kind == "Form" {
			shownForm(o)
		} else if !quiet(o) {
			a.fire(o, "onShow")
		}
	case fltk.HIDE:
		if !quiet(o) {
			a.fire(o, "onHide")
		}
	case fltk.FOCUS:
		switch {
		case o.kind == "Form":
			o.active = true // the window was given the keyboard
			if o.ready {
				told(o, "onActivate", &o.activeTold)
			}
		case takesFocus(o) && !o.focused:
			o.focused = true
			if f := o.form(); f != nil {
				// The control that had the keyboard is told it lost it
				// before this one is told it has it, as VB6 did; FLTK says
				// it the other way round.
				if prev := f.focusOwner; prev != nil && prev != o {
					blur(prev)
				}
				f.focusOwner = o
			}
			if !quiet(o) {
				told(o, "onFocus", &o.told)
			}
		}
	case fltk.UNFOCUS:
		switch {
		case o.kind == "Form":
			deactivateSoon(o)
		case o.focused:
			blur(o)
		}
	}
}

// told raises an event that says something began, once: until its opposite
// has been raised, said is true.
func told(o *guiObject, event string, said *bool) {
	if !*said {
		*said = true
		o.app.fire(o, event)
	}
}

// blur is a control losing the keyboard; it is told only if it was told it
// had it.
func blur(o *guiObject) {
	o.focused = false
	if o.told && o.widget != nil {
		o.told = false
		o.app.fire(o, "onBlur")
	}
}

// takesFocus says whether the object's widget keeps the keyboard when it is
// offered it: a Canvas does only if it listens for keys.
func takesFocus(o *guiObject) bool {
	if !o.hasEvent("onFocus") {
		return false
	}
	return o.kind != "Canvas" || o.events["onKey"] != nil
}

// shownForm is a Form's window being shown. The first time, onLoad waits for
// the event loop, so that handlers a program sets after show() are there to
// hear it, and is followed by everything held back until then; later it is
// just onShow.
func shownForm(f *guiObject) {
	if f.loaded {
		if f.ready {
			f.app.fire(f, "onShow")
		}
		return
	}
	f.loaded = true
	fltk.AddTimeout(0, func() {
		if f.widget == nil || !shown(f) {
			return // closed before the loop got to it
		}
		f.app.fire(f, "onLoad")
		if f.widget == nil {
			return
		}
		f.ready = true
		announce(f)
	})
}

// announce tells a form that has just loaded what happened while it did,
// in the order a user would have met it: it was shown, and so were the
// controls on it (the visible ones); anything that changed size did;
// the window was given the keyboard, and one control had it.
func announce(f *guiObject) {
	a := f.app
	a.fire(f, "onShow")
	var all []*guiObject
	var walk func(o *guiObject)
	walk = func(o *guiObject) {
		for _, c := range o.children {
			all = append(all, c)
			walk(c)
		}
	}
	walk(f)
	for _, c := range all {
		if c.widget != nil && visibleOnScreen(c) {
			a.fire(c, "onShow")
		}
	}
	resized(f)
	for _, c := range all {
		if c.widget != nil && c.hooked.resize {
			resized(c)
		}
	}
	if f.widget != nil && f.active {
		told(f, "onActivate", &f.activeTold)
	}
	for _, c := range all {
		if c.widget != nil && c.focused && takesFocus(c) {
			told(c, "onFocus", &c.told)
		}
	}
}

// visibleOnScreen says whether an object and everything holding it is
// showing: a page that is not the selected one is not.
func visibleOnScreen(o *guiObject) bool {
	for p := o; p != nil; p = p.parent {
		w, ok := p.widget.(interface{ Visible() bool })
		if !ok || !w.Visible() {
			return false
		}
	}
	return true
}

// deactivateSoon is a Form's window being told it lost the keyboard. It is
// told that, too, whenever the keyboard moves from one of its controls to
// another, so it waits for the move to be over and then looks: if none of
// its widgets has the keyboard, it went to another window.
func deactivateSoon(f *guiObject) {
	fltk.AddTimeout(0, func() {
		if f.widget == nil || !f.active || holdsFocus(f) {
			return
		}
		f.active = false
		if f.activeTold {
			f.activeTold = false
			f.app.fire(f, "onDeactivate")
		}
	})
}

// holdsFocus says whether the object or something in it has the keyboard.
func holdsFocus(o *guiObject) bool {
	if w, ok := o.widget.(interface{ HasFocus() bool }); ok && w.HasFocus() {
		return true
	}
	for _, c := range o.children {
		if holdsFocus(c) {
			return true
		}
	}
	return false
}

// pointerEvent is the mouse over a Label or an Image, which have no use for
// it themselves, so that it all goes to the script: onMouseDown, onMouseUp,
// onMouseMove, onMouseEnter, onMouseLeave, and the onClick that a press and
// a release over it make, onDoubleClick as well when the press was a
// double click. A Label with none of them stays as it was: drags only.
func pointerEvent(o *guiObject, e fltk.Event) bool {
	has := func(event string) bool { return o.events[event] != nil }
	wanted := false
	for _, ev := range append(append([]string{}, pointerEvents...), clickEvents...) {
		wanted = wanted || has(ev)
	}
	if !wanted && propString(o, "pointer") == "default" {
		return e == fltk.PUSH && has("onDrag")
	}
	g := o.widget.(geometry)
	a := o.app
	at := func() (lua.LValue, lua.LValue) {
		return lua.LNumber(fltk.EventX() - g.X()), lua.LNumber(fltk.EventY() - g.Y())
	}
	switch e {
	case fltk.PUSH:
		x, y := at()
		o.mouse.pressed, o.mouse.double = true, fltk.EventClicks() > 0
		a.fire(o, "onMouseDown", x, y, lua.LNumber(fltk.EventButton()), lua.LBool(o.mouse.double),
			lua.LString(modNames(fltk.EventState())))
		return true // so that the release comes here too
	case fltk.DRAG:
		return true
	case fltk.RELEASE:
		x, y := at()
		pressed, double := o.mouse.pressed, o.mouse.double
		o.mouse.pressed = false
		a.fire(o, "onMouseUp", x, y, lua.LNumber(fltk.EventButton()))
		px, py := fltk.EventX()-g.X(), fltk.EventY()-g.Y()
		if pressed && fltk.EventButton() == 1 && px >= 0 && py >= 0 && px < g.W() && py < g.H() {
			a.fire(o, "onClick", x, y)
			if double && o.widget != nil {
				a.fire(o, "onDoubleClick", x, y)
			}
		}
		return true
	case fltk.ENTER:
		// FLTK sends the first move over a widget as this, so it is a move
		// as well.
		x, y := at()
		o.mouse.over = true
		a.fire(o, "onMouseEnter")
		a.fire(o, "onMouseMove", x, y)
		if o.widget != nil {
			showPointer(o)
		}
		return true // so that the moves after it come here
	case fltk.LEAVE:
		o.mouse.over = false
		showPointer(o)
		a.fire(o, "onMouseLeave")
		return true
	case fltk.MOVE:
		x, y := at()
		a.fire(o, "onMouseMove", x, y)
		if o.widget != nil {
			showPointer(o)
		}
		return true
	}
	return false
}
