package gui

import "testing"

const fieldDef = `
gui.component{
  layout = {
    kind = "Component", name = "LabeledField", width = 240, height = 28,
    props = {
      label = { type = "string", default = "Name", to = "lbl.caption" },
      text  = { type = "string", to = "txt.text" },
      note  = { type = "string", default = "n" },
    },
    events = { onChange = "txt.onChange", "onSubmit" },
    { kind = "Label", name = "lbl", left = 0, top = 4, width = 80 },
    { kind = "TextBox", name = "txt", left = 84, width = 156, public = true },
    { kind = "Frame", name = "box", left = 0, top = 30,
      { kind = "Button", name = "ok", caption = "OK" } },
  },
  code = function(self, parts)
    seen = parts
    function self:clear() parts.txt.text = "" end
  end,
}`

func TestComponentIsADefinedControl(t *testing.T) {
	L, _ := newState(t)
	run(t, L, fieldDef)
	run(t, L, `
form = gui.Form{}
a = form:LabeledField{name = "first", label = "First", text = "x", left = 5, top = 6}
b = form:LabeledField{name = "second"}
assert(tostring(a) == "LabeledField")
assert(a.width == 240 and a.height == 28 and a.left == 5)
assert(a.label == "First" and a.text == "x" and b.label == "Name" and b.text == "")
assert(a.lbl == nil and a.box == nil and form.lbl == nil, "parts are private")
assert(a.txt.text == "x" and a.txt ~= b.txt, "a public part is a field, and each component has its own")
assert(seen.lbl.caption == "Name" and seen.ok.caption == "OK")
a.text = "hello"
assert(a.txt.text == "hello" and a.text == "hello")
a.label = "Changed"
a:clear()
assert(a.text == "")
local got
a.onChange = function(self, ...) got = self end
a.txt:fire("onChange")
assert(got == a, "an event of a part is the component's")
assert(a:fire("onSubmit") == nil)
assert(form.first == a and form:find("second") == b and a:find("ok").caption == "OK")
d = gui.dump(a)
assert(d.kind == "LabeledField" and d.label == "Changed" and #d == 0, "an instance is dumped without its parts")
assert(gui.kinds().LabeledField.defined)`)
}

func TestComponentChecksItsInput(t *testing.T) {
	L, _ := newState(t)
	fails(t, L, `gui.component(1)`, "expects a table")
	fails(t, L, `gui.component{layout = 1}`, "layout must be a layout table")
	fails(t, L, `gui.component{layout = {kind = "Form", name = "A"}}`, "top of a component's layout is a Component")
	fails(t, L, `gui.component{layout = {name = "A", props = {p = {to = "nobody.text"}}}}`, "there is no part called nobody")
	fails(t, L, `gui.component{layout = {name = "A", props = {p = {to = "nodot"}}, {kind = "Label", name = "x"}}}`, "not written part.property")
	fails(t, L, `gui.component{layout = {name = "A", events = {onX = "x.onY"}}}`, "there is no part called x")
	fails(t, L, `gui.component{layout = {name = "Button"}}`, "built-in kinds")
	fails(t, L, `gui.component{layout = {name = "A"}, code = 1}`, "code is a function")

	run(t, L, `
gui.component{layout = {name = "Loop", { kind = "Loop", name = "again" }}}
gui.component{layout = {name = "Bad", { kind = "Nonsense", name = "x" }}}`)
	fails(t, L, `gui.Form{}:Loop{}`, "Loop holds itself")
	fails(t, L, `gui.Form{}:Bad{}`, "there is no kind \"Nonsense\"")
	run(t, L, `gui.Form{}:Label{name = "x"}`) // the failed build left no mark
}

func TestComponentsNestAndMove(t *testing.T) {
	L, _ := newState(t)
	run(t, L, fieldDef)
	run(t, L, `
gui.component{layout = { name = "Pair", width = 300, height = 70,
  { kind = "LabeledField", name = "txt", label = "A" },
  { kind = "LabeledField", name = "other", top = 36, label = "B" } }}
form = gui.Form{}
p = form:Pair{}
q = form:Pair{}
other = form:Frame{}
form:add(p)
other:add(q)
assert(p.parent == form and q.parent == other)
form:TextBox{name = "txt"}   -- the form's own txt, not the parts'`)
}
