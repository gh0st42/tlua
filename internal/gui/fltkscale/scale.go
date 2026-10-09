//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

// Package fltkscale says how many device pixels make a unit of FLTK's
// drawing while a widget draws: what an image has to be drawn at to be
// drawn pixel for pixel, with nothing smoothed.
package fltkscale

// #cgo darwin LDFLAGS: -framework CoreGraphics
// #include "scale.h"
import "C"

import _ "github.com/pwiecz/go-fltk" // the library fl_gc is in

// DeviceScale is the device pixels to a unit while a widget draws, from the
// drawing itself; 0 where that is not known, and FLTK's screen scale is the
// answer.
func DeviceScale() float64 { return float64(C.tlua_fltk_device_scale()) }
