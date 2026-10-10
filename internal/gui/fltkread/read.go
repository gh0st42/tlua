//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

// Package fltkread reads back what FLTK has drawn into the current
// offscreen buffer, which go-fltk does not wrap: how a Canvas's drawing
// becomes a picture.
package fltkread

// #cgo CXXFLAGS: -std=c++11
// #include <stdlib.h>
// #include "read.h"
import "C"

import (
	"image"
	"unsafe"

	_ "github.com/pwiecz/go-fltk" // links the library fl_read_image is in
)

// Read is the w by h pixels at the corner of what is being drawn into, as
// an opaque picture.
func Read(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w <= 0 || h <= 0 {
		return img
	}
	buf := C.malloc(C.size_t(w * h * 3))
	defer C.free(buf)
	C.tlua_fltk_read_image((*C.uchar)(buf), C.int(w), C.int(h))
	rgb := unsafe.Slice((*byte)(buf), w*h*3)
	for i, j := 0, 0; i < len(rgb); i, j = i+3, j+4 {
		img.Pix[j], img.Pix[j+1], img.Pix[j+2], img.Pix[j+3] = rgb[i], rgb[i+1], rgb[i+2], 255
	}
	return img
}
