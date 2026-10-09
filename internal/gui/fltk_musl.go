//go:build musl && linux && cgo

package gui

// On musl, the glibc names go-fltk's libraries call are made up for.
import _ "tlua/internal/gui/muslcompat"
