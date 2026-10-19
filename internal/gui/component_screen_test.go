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
