//go:build musl && linux && cgo

// Package muslcompat lets go-fltk's glibc-built FLTK link against musl, for
// a build on Alpine: go build -tags musl. It gives the six glibc names the
// libraries call (compat.c); nothing in Go uses it.
package muslcompat

import "C"
