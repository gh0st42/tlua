//go:build !(cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64)))

package gui

import (
	"errors"

	lua "github.com/yuin/gopher-lua"
)

// Without cgo, or on a platform go-fltk ships no libraries for, the module
// still loads and builds its object tree; only putting it on screen fails.
// Nothing is ever on screen, so no object has a widget and most of what
// follows is never reached.
var errNoBackend = errors.New("gui: this tlua was built without FLTK (it needs cgo on darwin, linux, openbsd or windows/amd64)")

func buildForm(f *guiObject) error                                  { return errNoBackend }
func buildLive(o *guiObject) error                                  { return errNoBackend }
func showWindow(f *guiObject, modal bool)                           {}
func hideWindow(f *guiObject)                                       {}
func shown(f *guiObject) bool                                       { return false }
func wait()                                                         {}
func focus(o *guiObject)                                            {}
func applyProp(o *guiObject, name string, value lua.LValue) error   { return nil }
func readProp(o *guiObject, name string) (lua.LValue, bool)         { return nil, false }
func addTimeout(secs float64, fn func()) error                      { return errNoBackend }
func fileDialog(opts fileOptions) ([]string, error)                 { return nil, errNoBackend }
func colorDialog(title string, r, g, b uint8) (string, bool, error) { return "", false, errNoBackend }
func redraw(o *guiObject)                                           {}
func releaseForm(f *guiObject, gone func())                         {}
func destroyWidget(o *guiObject)                                    {}
func restack(p *guiObject)                                          {}
func selectText(o *guiObject, i, j int)                             {}
func tableEdit(o *guiObject, row, col int)                          {}
func tableEditing(o *guiObject) (int, int)                          { return 0, 0 }
func insertText(o *guiObject, text string)                          {}
func pointAt(o *guiObject, pos int) (int, int, int, bool)           { return 0, 0, 0, false }
func getClipboard() (string, error)                                 { return "", errNoBackend }
func setClipboard(text string) error                                { return errNoBackend }

func dialog(title, message string, buttons []string, input bool, deflt string) (int, string, error) {
	return -1, "", errNoBackend
}
