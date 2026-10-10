//go:build !(cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64)))

package gui

import "testing"

func TestWithoutBackendShowFailsClearly(t *testing.T) {
	L, _ := newState(t)
	if err := L.DoString(`local form = gui.Form{caption = "x"}
form:Button{caption = "b"}
form.caption = "y"
for _, f in ipairs{function() form:show() end,
                   function() gui.msgbox("hi") end,
                   function() gui.inputbox("name?") end,
                   function() gui.openfile("pick") end,
                   function() gui.after(1, print) end,
                   function() gui.paint(10, 10, function() end) end} do
  local ok, err = pcall(f)
  assert(not ok and err:match("built without FLTK"), tostring(err))
end`); err != nil {
		t.Fatal(err)
	}
}
