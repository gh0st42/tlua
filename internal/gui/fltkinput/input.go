//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

// Package fltkinput feeds synthetic input to FLTK, through the same
// Fl::handle() that events from the system go through: focus, shortcuts,
// menus and the widgets' own handling all see them as they would a real
// key or click. It is for tests; nothing in tlua itself imports it.
//
// Events go to the window in front, which is the one shown last. Positions
// are in that window's coordinates.
package fltkinput

// #cgo CXXFLAGS: -std=c++11
// #include <stdlib.h>
// #include "input.h"
import "C"

import (
	"unsafe"

	"github.com/pwiecz/go-fltk"
)

// Event is everything FLTK is told about one event.
type Event struct {
	Type         fltk.Event
	X, Y         int // in the front window
	RootX, RootY int // on the screen
	DX, DY       int // a mouse wheel's turn
	Key          int // keysym; for a mouse button, fltk's Button+n
	State        int // modifiers and buttons held
	Clicks       int // 1 more than the clicks before this one: >0 is a double click
	IsClick      bool
	Text         string
}

// Send hands one event to the front window, and says whether something
// used it. It is false, too, when no window is showing.
func Send(e Event) bool {
	text := C.CString(e.Text)
	defer C.free(unsafe.Pointer(text))
	isClick := 0
	if e.IsClick {
		isClick = 1
	}
	return C.tlua_fltk_send(C.int(e.Type), C.int(e.X), C.int(e.Y), C.int(e.RootX), C.int(e.RootY),
		C.int(e.DX), C.int(e.DY), C.int(e.Key), C.int(e.State), C.int(e.Clicks), C.int(isClick),
		text, C.int(len(e.Text))) > 0
}

// FLTK's numbering of mouse buttons, and the state bits for them held down.
const (
	button      = 0xfee8 // FL_Button
	button1Held = 0x01000000
)

// Click presses and releases mouse button 1 at x, y.
func Click(x, y int) { click(x, y, 0) }

// DoubleClick is the second click of a double click at x, y.
func DoubleClick(x, y int) { click(x, y, 1) }

func click(x, y, clicks int) {
	Send(Event{Type: fltk.PUSH, X: x, Y: y, Key: button + 1, State: button1Held, Clicks: clicks})
	Send(Event{Type: fltk.RELEASE, X: x, Y: y, Key: button + 1, Clicks: clicks, IsClick: true})
}

// Drag presses at from, moves through to, and releases there.
func Drag(fromX, fromY, toX, toY, steps int) {
	Send(Event{Type: fltk.PUSH, X: fromX, Y: fromY, Key: button + 1, State: button1Held})
	for i := 1; i <= steps; i++ {
		x := fromX + (toX-fromX)*i/steps
		y := fromY + (toY-fromY)*i/steps
		Send(Event{Type: fltk.DRAG, X: x, Y: y, Key: button + 1, State: button1Held})
	}
	Send(Event{Type: fltk.RELEASE, X: toX, Y: toY, Key: button + 1})
}

// Key presses one key: a keysym such as fltk.ENTER_KEY or 'a', the text it
// types (if any), and the modifiers held (fltk.CTRL and the like).
func Key(keysym int, text string, state int) bool {
	return Send(Event{Type: fltk.KEY, Key: keysym, Text: text, State: state})
}

// Type presses one key for each character of s.
func Type(s string) {
	for _, r := range s {
		Key(int(r), string(r), 0)
	}
}

// Drop drags text in from outside at x, y and lets it go, as another
// program would; it says whether the widget there took the drop. FLTK then
// delivers the text itself as a paste to that widget, which a test has to
// do on its side (Fl::belowmouse() is not reachable from here).
func Drop(x, y int) bool {
	// Whatever an earlier drag was over, it has left it.
	Send(Event{Type: fltk.DND_LEAVE, X: x, Y: y})
	Send(Event{Type: fltk.DND_ENTER, X: x, Y: y})
	Send(Event{Type: fltk.DND_DRAG, X: x, Y: y})
	return Send(Event{Type: fltk.DND_RELEASE, X: x, Y: y})
}

// PasteText sets the text that the event being handled carries, for a
// paste delivered by hand.
func PasteText(text string) {
	Send(Event{Type: 0, Text: text}) // event 0 (FL_NO_EVENT) only sets the fields
}

// Wheel turns the mouse wheel at x, y.
func Wheel(x, y, dx, dy int) bool {
	return Send(Event{Type: fltk.MOUSEWHEEL, X: x, Y: y, DX: dx, DY: dy})
}
