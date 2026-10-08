package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestNames(t *testing.T) {
	L, _ := newState(t)
	run(t, L, `
form = gui.Form{name = "Main"}
ok = form:Button{name = "cmdOK", caption = "OK"}
frame = form:Frame{name = "fraOpts"}
check = frame:CheckBox{name = "chkBold"}
assert(form.cmdOK == ok and form:find("cmdOK") == ok, "by field and by find")
assert(form.chkBold == check, "controls inside a frame are the form's too")
assert(check:find("cmdOK") == ok, "find works from any control on the form")
assert(form:find("nothing") == nil)
assert(form.name == "Main" and ok.name == "cmdOK")

ok.name = "cmdGo"
assert(form.cmdGo == ok and form:find("cmdOK") == nil, "renaming moves it")

other = gui.Form{}
other:add(frame)
assert(other.chkBold == check and form:find("chkBold") == nil, "moving to another form takes the names along")

loose = gui.Button{name = "cmdLoose", parent = other}
assert(other.cmdLoose == loose)`)

	fails(t, L, `form:Button{name = "cmdGo"}`, "already has a control named cmdGo")
	fails(t, L, `other:Label{}.name = "chkBold"`, "already has a control named chkBold")
	fails(t, L, `form:Button{name = "caption"}`, `"caption" cannot be a name: it is a property`)
	fails(t, L, `form:Button{name = "show"}`, "it is a method")
	fails(t, L, `form:Button{name = "Label"}`, "it is a method")
	fails(t, L, `form:Button{name = "onClick"}`, "it reads as an event")
	fails(t, L, `form:Button{name = "end"}`, "a name is a Lua identifier")
	fails(t, L, `form:Button{name = "two words"}`, "a name is a Lua identifier")
	fails(t, L, `form:Button{name = 7}`, "a name is a string")
	fails(t, L, `form.cmdGo = 1`, "cmdGo is a control on this form")
	// A clash found while moving leaves the control where it was.
	run(t, L, `third = gui.Form{}; third:Button{name = "cmdGo"}`)
	fails(t, L, `third:add(ok)`, "already has a control named cmdGo")
	run(t, L, `assert(form.cmdGo == ok)`)
}

const twoControls = `{
  kind = "Form", name = "Main", caption = "Hello", width = 320, height = 160,
  { kind = "Label", name = "lblName", caption = "Name", left = 16, top = 16, width = 60 },
  { kind = "Frame", name = "fra", caption = "Box", left = 16, top = 50, width = 200, height = 80,
    { kind = "TextBox", name = "txtName", left = 8, top = 24, width = 150, text = "Ada" },
  },
}`

func TestLoadFromATable(t *testing.T) {
	L, _ := newState(t)
	run(t, L, `
frm = gui.load(`+twoControls+`)
assert(frm.caption == "Hello" and frm.width == 320 and frm.name == "Main")
assert(frm.lblName.caption == "Name" and frm.lblName.width == 60 and frm.lblName.height == 28)
assert(frm.txtName.text == "Ada" and frm.txtName.left == 8)
function frm.lblName:onDrop() end -- code wires handlers by name`)
	frm := global(t, L, "frm")
	if len(frm.children) != 2 || len(frm.children[1].children) != 1 {
		t.Fatalf("tree: %v", frm.children)
	}
}

func TestLoadErrorsSayWhere(t *testing.T) {
	L, _ := newState(t)
	fails(t, L, `gui.load{ kind = "Form", { kind = "Buton", name = "b" } }`, `gui.load layout, at b: there is no kind "Buton"`)
	fails(t, L, `gui.load{ kind = "Form", { name = "x" } }`, "every part of a layout says its kind")
	fails(t, L, `gui.load{ kind = "Form", { kind = "Button", caption = 3, font = "comic" } }`, `font must be`)
	fails(t, L, `gui.load{ kind = "Form", { kind = "Frame", name = "f", 7 } }`, "item 1 of f is a number, not a control")
	fails(t, L, `gui.load{ kind = "Button" }`, "a layout's top is a Form")
	fails(t, L, `gui.load{ kind = "Form", { kind = "Form" } }`, "a Form cannot be inside anything")
	fails(t, L, `gui.load{ kind = "Form", { kind = "Tabs", { kind = "Button" } } }`, "a Tabs cannot hold a Button")
	fails(t, L, `gui.load(42)`, "a layout table, or the path of a layout file")
}

func TestLoadIntoAContainer(t *testing.T) {
	L, _ := newState(t)
	run(t, L, `
form = gui.Form{}
frame = gui.load({ kind = "Frame", name = "fraGroup", { kind = "Button", name = "cmdIn" } }, form)
assert(form.fraGroup == frame and form.cmdIn ~= nil)`)
}

func TestLoadFromAFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "forms"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("forms/Main.form.lua", "return "+twoControls)
	write("forms/Main.lua", `local frm = gui.load "Main"; frm.loadedBy = "code"; return frm`)
	write("forms/Sneaky.form.lua", `print("layouts are data"); return {}`)
	write("forms/NotATable.form.lua", `return 5`)
	write("main.lua", `
local frm = dofile(dir .. "/forms/Main.lua")
assert(frm.txtName.text == "Ada" and frm.loadedBy == "code")
local again = gui.load(dir .. "/forms/Main.form.lua")
assert(again.lblName.caption == "Name", "a full path, .lua and all")
`)
	L, _ := newState(t)
	L.SetGlobal("dir", lua.LString(dir))
	if err := L.DoFile(filepath.Join(dir, "main.lua")); err != nil {
		t.Fatal(err)
	}
	L.SetGlobal("sneaky", lua.LString(filepath.Join(dir, "forms/Sneaky")))
	fails(t, L, `gui.load(sneaky)`, "attempt to call a non-function object")
	L.SetGlobal("bad", lua.LString(filepath.Join(dir, "forms/NotATable")))
	fails(t, L, `gui.load(bad)`, "returns a number, not a layout table")
	fails(t, L, `gui.load("no/such/form")`, "no such file")
}

func TestDumpWritesWhatDiffers(t *testing.T) {
	L, _ := newState(t)
	run(t, L, `
form = gui.Form{name = "Main", caption = "Hi"}
b = form:Button{name = "cmdOK", caption = "OK", left = 10, top = 20, onClick = print}
b.tag = "a field of the script's own"
d = gui.dump(form)`)
	run(t, L, `
assert(d.kind == "Form" and d.name == "Main" and d.caption == "Hi")
assert(d.width == nil and d.left == nil and d.visible == nil, "defaults, and an unplaced form's position, go unsaid")
local c = d[1]
assert(c.kind == "Button" and c.name == "cmdOK" and c.left == 10 and c.top == 20)
assert(c.width == nil and c.enabled == nil and c.onClick == nil and c.tag == nil)`)
}

// roundTrip is a form with every built-in kind, each with something set.
const roundTrip = `
gui.define{name = "Stars", props = {value = {type = "integer", default = 0}, mood = {type = "choice", choices = {"glad", "sad"}, default = "glad"}},
  build = function(parent, opts) local c = parent:Canvas{width = 100, height = 20}; c.value = opts.value or 0; return c end}
form = gui.Form{name = "All", caption = "All of them", left = 40, top = 50, width = 640, height = 480, resizable = true}
form:Menu{ {"&File", { {"&Open", shortcut = "Cmd+O", function() end}, "-", {"&Quit"} }} }
form:Label{name = "lbl", caption = "Name", align = "right", color = "#ffeecc", font = "mono", fontSize = 16}
form:Button{name = "cmd", caption = "Go", default = true, tooltip = "go on"}
form:TextBox{name = "txt", text = "two\nlines", multiLine = true, grow = true}
form:TextBox{name = "pwd", password = true}
form:CheckBox{name = "chk", caption = "c", checked = true}
form:RadioButton{name = "opt", caption = "r", checked = true}
form:ComboBox{name = "cbo", items = {"a", "b"}, selected = 2}
form:ListBox{name = "lst", items = {"x", "y \"quoted\""}, selected = 1}
form:Tree{name = "tvw", items = {"leaf", {"branch", {"kid"}, open = true}}, path = "branch/kid"}
form:Table{name = "grd", columns = {"A", "B"}, rows = {{"1", 2}, {"3", 4.5}}, columnWidths = {50, 60}, selected = 1}
form:Slider{name = "sld", min = 1, max = 9, step = 0.5, value = 3, vertical = true}
form:Spinner{name = "spn", value = 7}
form:ProgressBar{name = "prg", value = 30, caption = "30%"}
form:Image{name = "img", file = "pic.png", fit = true}
form:Canvas{name = "cnv", color = "#000000"}
form:Stars{name = "stars", value = 4, mood = "sad", left = 5}
local fra = form:Frame{name = "fra", caption = "Frame", visible = false, enabled = false}
fra:Label{name = "inner", caption = "in the frame"}
local tabs = form:Tabs{name = "tabs", selected = 2}
tabs:Page{name = "pg1", caption = "One"}:Button{name = "cmdPage", caption = "p"}
tabs:Page{name = "pg2", caption = "Two"}
`

func TestEveryKindRoundTrips(t *testing.T) {
	L, _ := newState(t)
	run(t, L, roundTrip)
	path := filepath.Join(t.TempDir(), "All.form.lua")
	L.SetGlobal("path", lua.LString(path))
	run(t, L, `gui.save(form, path)`)
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(first)
	for _, want := range []string{
		`kind = "Form", name = "All", caption = "All of them", left = 40, top = 50, width = 640, height = 480, resizable = true,`,
		`{ kind = "Stars", name = "stars", left = 5, width = 100, height = 20, mood = "sad", value = 4 },`,
		`items = { "x", "y \"quoted\"" }`,
		`rows = { { "1", 2 }, { "3", 4.5 } }`,
		`text = "two\nlines"`,
		`items = { "leaf", { "branch", { "kid" }, open = true } }`,
		`{ "&Open", shortcut = "Cmd+O" }, "-", { "&Quit" }`,
		`file = "pic.png"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("saved layout lacks %s\n%s", want, text)
		}
	}
	// Load what was saved and save it again: the same text.
	run(t, L, `again = gui.load(path); gui.save(again, path .. ".2")`)
	second, err := os.ReadFile(path + ".2")
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != text {
		t.Errorf("a loaded layout saves differently:\n%s\n----\n%s", text, second)
	}
	run(t, L, `assert(again.cmdPage.caption == "p" and again.stars.value == 4 and again.stars.mood == "sad")`)
}

func TestKinds(t *testing.T) {
	L, _ := newState(t)
	run(t, L, `gui.define{name = "Stars", events = {"onChange"}, props = {value = {type = "integer", default = 0}},
  build = function(parent) return parent:Canvas{} end}`)
	run(t, L, `
local k = gui.kinds()
assert(k.Button.props.caption.type == "string" and k.Button.props.caption.default == "")
assert(k.Button.props.default.fixed == true and k.Button.props.caption.fixed == nil)
assert(k.Label.props.align.type == "choice" and #k.Label.props.align.choices == 3)
assert(k.Label.props.color.type == "color" and k.Label.props.color.default == nil)
assert(k.TextBox.props.multiLine.fixed and k.TextBox.props.text.default == "")
assert(k.Tree.props.items.type == "tree" and k.Menu.props.items.type == "menu")
assert(k.Table.props.rows.type == "rows" and #k.Table.props.rows.default == 0)
assert(k.Form.props.visible == nil, "whether a form is up is not part of its design")
assert(k.Button.width == 120 and k.ListBox.height == 120)
local has = function(list, x) for _, v in ipairs(list) do if v == x then return true end end end
assert(has(k.Button.events, "onClick") and has(k.Button.events, "onDrop"))
assert(has(k.Frame.holds, "Button") and has(k.Tabs.holds, "Page") and #k.Button.holds == 0)
assert(k.Stars.defined and k.Stars.props.value.type == "integer" and k.Stars.props.left.type == "integer")
assert(has(k.Stars.events, "onChange"))
k.Button.width = 1
assert(gui.kinds().Button.width == 120, "a fresh table each time")`)
	fails(t, L, `gui.define{name = "Bad", props = {caption = {type = "string"}}, build = print}`, "caption is a property every control has")
	fails(t, L, `gui.define{name = "Bad", props = {x = 1}, build = print}`, "props are written name = {type = ..., default = ...}")
}

func TestQuoteReadsBack(t *testing.T) {
	L, _ := newState(t)
	for _, s := range []string{"plain", "with \"quotes\" and \\", "tab\tnew\nline\r", "\x00\x01\x7f", "ünï"} {
		if err := L.DoString("__s = " + quote(s)); err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		if got := lua.LVAsString(L.GetGlobal("__s")); got != s {
			t.Errorf("quote(%q) reads back as %q", s, got)
		}
	}
}
