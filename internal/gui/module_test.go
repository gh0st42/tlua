package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// newState is a Lua state with the gui module in it, as the global gui.
func newState(t *testing.T) (*lua.LState, *app) {
	t.Helper()
	L := lua.NewState()
	t.Cleanup(L.Close)
	a := Open(L)
	if err := L.DoString(`gui = require("gui")`); err != nil {
		t.Fatal(err)
	}
	return L, a
}

func run(t *testing.T, L *lua.LState, src string) {
	t.Helper()
	if err := L.DoString(src); err != nil {
		t.Fatal(err)
	}
}

// fails runs src and checks that it raises an error mentioning want.
func fails(t *testing.T, L *lua.LState, src, want string) {
	t.Helper()
	err := L.DoString(src)
	if err == nil {
		t.Fatalf("%s: expected an error", src)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("%s: error %q does not mention %q", src, err, want)
	}
}

func global(t *testing.T, L *lua.LState, name string) *guiObject {
	t.Helper()
	obj, ok := toObject(L.GetGlobal(name))
	if !ok {
		t.Fatalf("%s is not a gui object", name)
	}
	return obj
}

func TestObjectsAndProperties(t *testing.T) {
	L, _ := newState(t)
	run(t, L, `
form = gui.Form{caption = "Hello", width = 320, height = 200}
assert(form.caption == "Hello" and form.width == 320)
form.caption = "World"
assert(form.caption == "World")
label = form:Label{caption = "x"}
assert(label.text == "x", "a Label's text is its caption")
label.text = "y"
assert(label.caption == "y")
box = form:TextBox{text = "one", multiLine = true}
box.value = "two"
assert(box.text == "two", "a TextBox's value is its text")
check = form:CheckBox{caption = "c", value = true}
assert(check.checked == true)
assert(tostring(form) == 'Form "World"')
form.tag = 42
assert(form.tag == 42, "a script can keep its own fields on an object")`)
	form := global(t, L, "form")
	if got := len(form.children); got != 3 {
		t.Fatalf("form has %d children, want 3", got)
	}
}

func TestDefaultSizes(t *testing.T) {
	L, _ := newState(t)
	run(t, L, `
local form = gui.Form{caption = "sized"}
assert(form.width == 360 and form.height == 240, "form size")
local b = form:Button{caption = "b"}
assert(b.width == 120 and b.height == 28, "button size")
local wide = form:Label{caption = "w", width = 300}
assert(wide.width == 300 and wide.height == 28, "partial size")
local list = form:ListBox{}
assert(list.width == 160 and list.height == 120, "list size")`)
}

func TestFormPlacedOnlyWhenPositioned(t *testing.T) {
	L, _ := newState(t)
	run(t, L, `
free = gui.Form{caption = "free"}
pinned = gui.Form{caption = "pinned", left = 0, top = 0}
moved = gui.Form{caption = "moved"}
moved.top = 40`)
	for name, want := range map[string]bool{"free": false, "pinned": true, "moved": true} {
		if got := global(t, L, name).placed; got != want {
			t.Errorf("%s: placed = %v, want %v", name, got, want)
		}
	}
}

func TestWhereControlsGo(t *testing.T) {
	L, a := newState(t)
	run(t, L, `
loose = gui.Button{caption = "made before any form"}
first = gui.Form{}
joined = gui.Button{caption = "joins the latest form"}
second = gui.Form{}
told = gui.Button{caption = "told where", parent = first}
frame = second:Frame{caption = "group"}
inner = frame:CheckBox{caption = "in the frame"}
tabs = second:Tabs{}
page = tabs:Page{caption = "One"}
onpage = page:Label{caption = "on the page"}
assert(first:add(loose) == loose)
second:add(joined)`)
	if a.lastForm != global(t, L, "second") {
		t.Fatal("lastForm should be the second form")
	}
	first, second := global(t, L, "first"), global(t, L, "second")
	if len(first.children) != 2 || first.children[0] != global(t, L, "told") || first.children[1] != global(t, L, "loose") {
		t.Fatalf("first form holds %v", first.children)
	}
	if got := global(t, L, "joined").parent; got != second {
		t.Fatal("add should move a control from one form to another")
	}
	if global(t, L, "onpage").parent != global(t, L, "page") || global(t, L, "page").parent != global(t, L, "tabs") {
		t.Fatal("pages and their controls")
	}

	fails(t, L, `tabs:Button{}`, "a Tabs cannot hold a Button")
	fails(t, L, `frame:Page{}`, "a Frame cannot hold a Page")
	fails(t, L, `frame:add(second)`, "a Frame cannot hold a Form")
	fails(t, L, `frame:Menu{}`, "a Frame cannot hold a Menu")
	fails(t, L, `frame:add(frame)`, "cannot hold itself")
	fails(t, L, `inner:add(joined)`, "a CheckBox cannot hold a Button")
	fails(t, L, `frame:show()`, "only a Form can be shown")
	fails(t, L, `gui.Button{parent = 5}`, "parent must be a gui object")
	if gui := L.GetGlobal("gui").(*lua.LTable); gui.RawGetString("Page") != lua.LNil {
		t.Fatal("a Page is only made by its Tabs")
	}
}

func TestEvents(t *testing.T) {
	L, a := newState(t)
	run(t, L, `
form = gui.Form{}
button = form:Button{caption = "b", onClick = function(self) clicked = self end}
other = form:Button{}
other:on("click", function() return "via on" end)
assert(other.onClick ~= nil, "on('click') is onClick")
form:on("close", function() return false end)
assert(form.onClose ~= nil, "on('close') is onClose")`)
	button := global(t, L, "button")
	a.fire(button, "onClick")
	if L.GetGlobal("clicked") != button.ud {
		t.Fatal("a handler's self should be the object itself")
	}
	if got := a.fire(global(t, L, "other"), "onClick"); got != lua.LString("via on") {
		t.Fatalf("handler returned %v", got)
	}
	run(t, L, `button.onClick = nil; assert(button.onClick == nil)`)
	if _, ok := button.events["onClick"]; ok {
		t.Fatal("assigning nil should remove the handler")
	}
	fails(t, L, `button.onChange = function() end`, "a Button has no event onChange (it has onClick, onDrop, onDrag)")
	fails(t, L, `form:on("click", function() end)`, "a Form has no event onClick")
	fails(t, L, `button.onClick = 5`, "must be a function or nil")
	fails(t, L, `button.show = 1`, "show is a method")
	fails(t, L, `form:Label{}.onClick = print`, "a Label has no event onClick (it has onDrop, onDrag)")
	run(t, L, `form:Label{}.onDrop = print`)
	run(t, L, `assert(button.focus == form.focus, "methods are the same function every time")`)
}

func TestCloseCanBeRefused(t *testing.T) {
	L, a := newState(t)
	run(t, L, `
form = gui.Form{}
unloaded = false
form.onClose = function() return false end
form.onUnload = function() unloaded = true end`)
	a.requestClose(global(t, L, "form"))
	if L.GetGlobal("unloaded") != lua.LFalse {
		t.Fatal("onUnload ran although onClose refused")
	}
	run(t, L, `form.onClose = nil`)
	a.requestClose(global(t, L, "form"))
	if L.GetGlobal("unloaded") != lua.LTrue {
		t.Fatal("onUnload should run once nothing refuses")
	}
}

func TestHandlerErrorsAreKept(t *testing.T) {
	L, a := newState(t)
	run(t, L, `
form = gui.Form{}
button = form:Button{onClick = function() error("boom") end}
later = form:Button{onClick = function() ran = true end}`)
	a.fire(global(t, L, "button"), "onClick")
	a.fire(global(t, L, "later"), "onClick")
	if L.GetGlobal("ran") != lua.LNil {
		t.Fatal("nothing should run after a handler has failed")
	}
	L.SetGlobal("raise", L.NewFunction(func(L *lua.LState) int { a.raisePending(L); return 0 }))
	fails(t, L, `raise()`, "boom")
	if a.err != nil {
		t.Fatal("the error should be reported once")
	}
	run(t, L, `raise()`) // nothing pending any more
}

func TestRadioButtonsShareAParent(t *testing.T) {
	L, _ := newState(t)
	run(t, L, `
form = gui.Form{}
a = form:RadioButton{caption = "a", checked = true}
b = form:RadioButton{caption = "b"}
frame = form:Frame{}
c = frame:RadioButton{caption = "c", checked = true}
b.checked = true
assert(not a.checked and b.checked, "one radio button among siblings is on")
assert(c.checked, "a radio button in another container is not a sibling")`)
}

func TestCheckedValues(t *testing.T) {
	L, _ := newState(t)
	fails(t, L, `gui.Form{color = "mauve"}`, "colour must be")
	fails(t, L, `gui.Form{}:Label{font = "comic"}`, "font must be")
	fails(t, L, `gui.Form{}:Label{align = "middle"}`, "align must be")
	fails(t, L, `gui.Form{}:ListBox{items = "a,b"}`, "items must be a table")
	fails(t, L, `gui.Form{}:Table{rows = 3}`, "rows must be a table")
	fails(t, L, `gui.msgbox("hi", "maybe")`, "buttons must be one of")
	run(t, L, `gui.Form{}:Label{color = "#fa0", textColor = "#102030"}`)
	for in, want := range map[string][3]uint8{"#ffaa00": {255, 170, 0}, "#fa0": {255, 170, 0}, "white": {255, 255, 255}} {
		r, g, b, err := parseColor(in)
		if err != nil || [3]uint8{r, g, b} != want {
			t.Errorf("parseColor(%q) = %v %v %v, %v", in, r, g, b, err)
		}
	}
}

func TestMenuIsAListOfItems(t *testing.T) {
	L, _ := newState(t)
	run(t, L, `
form = gui.Form{}
menu = form:Menu{ {"&File", { {"&Quit", function() end} }}, {"&Help", {}} }
assert(#menu.items == 2)`)
}

func TestImagePathIsFoundBesideTheScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pic.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "app.lua")
	if err := os.WriteFile(script, []byte(`image = gui.Form{}:Image{file = "pic.png"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	L, _ := newState(t)
	if err := L.DoFile(script); err != nil {
		t.Fatal(err)
	}
	if got := propString(global(t, L, "image"), "file"); got != filepath.Join(dir, "pic.png") {
		t.Fatalf("file = %q", got)
	}
}

func TestEventNames(t *testing.T) {
	for in, want := range map[string]string{"click": "onClick", "onClick": "onClick", "close": "onClose", "doubleClick": "onDoubleClick", "online": "onOnline"} {
		if got := eventName(in); got != want {
			t.Errorf("eventName(%q) = %q, want %q", in, got, want)
		}
	}
}
