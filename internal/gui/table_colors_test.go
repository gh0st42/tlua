//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// A Table draws its cells in the colours it was given, and otherwise in
// FLTK's, which gui.colors sets; its grid is lighter than a dark ground.
func TestTableColors(t *testing.T) {
	o := &guiObject{props: map[string]lua.LValue{}}
	if got := tableColor(o, "color", white); got != white {
		t.Fatalf("no colour given: want the default, got %v", got)
	}
	o.props["color"] = lua.LString("#102030")
	if got, want := tableColor(o, "color", white), rgbColor(0x10, 0x20, 0x30); got != want {
		t.Fatalf("color: got %v, want %v", got, want)
	}
	o.props["textColor"] = lua.LString("nonsense")
	if got := tableColor(o, "textColor", black); got != black {
		t.Fatalf("a bad colour keeps the default, got %v", got)
	}
	if got := tableGrid(white); got != gridColor {
		t.Fatalf("grid on white: got %v", got)
	}
	dark := rgbColor(0x16, 0x18, 0x1d)
	gr, gg, gb := tableGrid(dark).RGB()
	if gr != 0x16+28 || gg != 0x18+28 || gb != 0x1d+28 {
		t.Fatalf("grid on dark: got %d %d %d", gr, gg, gb)
	}
}
