package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestRemoveAndReAdd(t *testing.T) {
	L, _ := newState(t)
	run(t, L, `
form = gui.Form{}
frame = form:Frame{name = "fra"}
b = frame:Button{name = "cmdA", caption = "A"}
b:remove()
assert(form.cmdA == nil and form:find("cmdA") == nil, "its name goes with it")
form:add(b)
assert(form.cmdA == b, "and comes back with it")
frame:remove()
assert(form.fra == nil)`)
	fails(t, L, `form:remove()`, "a Form is closed, not removed")
	if n := len(global(t, L, "form").children); n != 1 {
		t.Fatalf("form children: %d", n)
	}
}

func TestRaiseAndLower(t *testing.T) {
	L, _ := newState(t)
	run(t, L, `
form = gui.Form{}
a = form:Button{name = "a"}; b = form:Button{name = "b"}; c = form:Button{name = "c"}
a:raise()
order = {}
for i, part in ipairs(gui.dump(form)) do order[i] = part.name end
assert(table.concat(order, ",") == "b,c,a", table.concat(order, ","))
c:lower()
order = {}
for i, part in ipairs(gui.dump(form)) do order[i] = part.name end
assert(table.concat(order, ",") == "c,b,a", table.concat(order, ","))`)
}

func TestNewContainersAndImages(t *testing.T) {
	L, _ := newState(t)
	run(t, L, `
form = gui.Form{}
p = form:Panel{name = "pnl"}; p:Button{name = "inPanel"}
s = form:Scroll{name = "scr"}; s:Label{name = "inScroll"}
sp = form:Splitter{name = "spl"}; sp:Panel{name = "pnlLeft"}; sp:Panel{name = "pnlRight"}
img = form:Button{name = "cmdIcon", image = "icons/open.png"}
cnv = form:Canvas{transparent = true}
local k = gui.kinds()
assert(k.Panel and k.Scroll and k.Splitter and k.Button.props.image.type == "file")
assert(k.Canvas.props.transparent.fixed)
assert(gui.dump(form)[4].image == "icons/open.png", "an image is kept as written")`)
}

func TestSaveATable(t *testing.T) {
	L, _ := newState(t)
	path := filepath.Join(t.TempDir(), "T.form.lua")
	L.SetGlobal("path", lua.LString(path))
	run(t, L, `gui.save({ kind = "Form", name = "T", caption = "x", { kind = "Button", name = "b" } }, path)`)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `{ kind = "Form", name = "T", caption = "x",`) {
		t.Fatalf("saved:\n%s", data)
	}
	run(t, L, `assert(gui.load(path).b ~= nil)`)
}
