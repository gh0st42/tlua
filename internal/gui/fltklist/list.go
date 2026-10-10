//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

// Package fltklist finds the line of a list under the mouse while
// something is dragged over it, which go-fltk does not wrap: where a drop
// on a ListBox or a Tree lands.
package fltklist

// #cgo CXXFLAGS: -std=c++11
// #include "list.h"
import "C"

import _ "github.com/pwiecz/go-fltk" // links the library the browser is in

// LineUnderMouse is the line, from 1, of the list FLTK has below the mouse
// at the event being handled, or 0 for none. Only call it from a list's
// own drag and drop events, once it has taken the drag: FLTK then holds
// that list as the widget below the mouse.
func LineUnderMouse() int { return int(C.tlua_fltk_line_under_mouse()) }
