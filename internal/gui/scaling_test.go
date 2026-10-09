//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import (
	"image"
	"image/color"
	"testing"
)

func TestRepeatPixels(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	src.SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 255})
	src.SetNRGBA(1, 1, color.NRGBA{0, 0, 255, 255})
	dst := repeatPixels(src, 8, 6)
	for y := 0; y < 6; y++ {
		for x := 0; x < 8; x++ {
			want := src.NRGBAAt(x/4, y/3)
			if got := dst.NRGBAAt(x, y); got != want {
				t.Fatalf("pixel %d,%d is %v, want %v", x, y, got, want)
			}
		}
	}
}
