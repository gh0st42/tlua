//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

// Package fltkcolor is FLTK's colour chooser, which go-fltk does not wrap:
// a window with a colour wheel, sliders and OK and Cancel.
package fltkcolor

// #cgo CXXFLAGS: -std=c++11
// #include <stdlib.h>
// #include "color.h"
import "C"

import (
	"unsafe"

	_ "github.com/pwiecz/go-fltk" // links the library the chooser is in
)

// Choose shows the chooser, starting at r, g, b, and waits for the user. ok
// is false when the chooser was cancelled.
func Choose(title string, r, g, b uint8) (uint8, uint8, uint8, bool) {
	t := C.CString(title)
	defer C.free(unsafe.Pointer(t))
	cr, cg, cb := C.uchar(r), C.uchar(g), C.uchar(b)
	ok := C.tlua_fltk_choose_color(t, &cr, &cg, &cb) != 0
	return uint8(cr), uint8(cg), uint8(cb), ok
}
