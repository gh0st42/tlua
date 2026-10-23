//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import "testing"

func TestComponentOnScreen(t *testing.T) {
	onScreen(t, fieldDef+`
form = gui.Form{width = 400, height = 200}
a = form:LabeledField{name = "a", label = "One", left = 10, top = 10}
b = form:LabeledField{name = "b", label = "Two", left = 10, top = 50}
log = {}
a.onChange = function(self) log[#log + 1] = self.text end
form:show()`, func(s *scene) {
		s.lua(`a.text = "typed"; b.text = "other"`)
		s.expect(`a.text == "typed" and b.text == "other" and a.txt.text == "typed"`)
		s.expect(`a.label == "One" and b.label == "Two" and a.lbl == nil`)
		s.lua(`a.label = "Changed"; b:clear(); b.text = "later"`)
		s.expect(`a.label == "Changed" and b.text == "later"`)
		s.expect(`gui.dump(a).text == "typed" and gui.dump(b).label == "Two"`)
	})
}

func TestComponentResizesItsParts(t *testing.T) {
	onScreen(t, `
gui.component{layout = { name = "Row", width = 200, height = 30,
  { kind = "Label", name = "lbl", left = 0, top = 4, width = 60 },
  { kind = "TextBox", name = "txt", left = 64, top = 0, width = 136, grow = true } }}
form = gui.Form{width = 400, height = 200, resizable = true}
row = form:Row{left = 10, top = 10, grow = true}
gui.component{layout = { name = "Box", width = 220, height = 40,
  { kind = "Row", name = "inner", left = 10, top = 4, width = 200, grow = true } }}
box = form:Box{left = 10, top = 60, grow = true}
inner = box:find("inner")
form:show()
part = function() return row:find("txt") end`, func(s *scene) {
		s.expect(`row.width == 200 and part().width == 136`)
		s.lua(`row.width = 300`)
		s.expect(`part().width == 236 and part().left == 64`)
		// A component in a component follows too.
		s.lua(`box.width = 320`)
		s.expect(`inner.width == 300 and inner:find("txt").width == 236`)
		s.expect(`inner.width == 300 and inner:find("txt").width == 236`)
	})
}
