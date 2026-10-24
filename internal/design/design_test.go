package design

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"tlua/internal/gui"
	"tlua/internal/interp"
)

// newLua is an interpreter with the gui module and the designer's modules.
func newLua(t *testing.T) (*interp.Interp, *gui.Boot) {
	t.Helper()
	r := interp.New(&interp.Options{})
	t.Cleanup(r.Close)
	boot := gui.Ready(r, nil)
	Preload(r.L)
	return r, boot
}

func run(t *testing.T, L *lua.LState, src string) {
	t.Helper()
	if err := L.DoString(src); err != nil {
		t.Fatal(err)
	}
}

func TestModel(t *testing.T) {
	r, _ := newLua(t)
	run(t, r.L, `
local model = require "design.model"
local doc = model.newForm("Main")
assert(model.uniqueName(doc, "Button") == "Button1")
local b = model.newControl(doc, "Button", 10, 20)
doc[#doc + 1] = b
assert(b.name == "Button1" and b.caption == "Button1" and b.width == 120 and b.height == 28)
assert(model.uniqueName(doc, "Button") == "Button2")
local t = model.newControl(doc, "Tabs", 0, 0, 300, 200)
assert(t[1].kind == "Page" and t[1].name == "Page1" and t[2].name == "Page2")
assert(model.newControl(doc, "TextBox", 0, 0).caption == nil, "a TextBox has no caption to start with")

model.set(b, "enabled", true)
assert(b.enabled == nil, "a default is left out")
model.set(b, "enabled", false)
assert(b.enabled == false and model.value(b, "enabled") == false)
model.set(b, "tooltip", "")
assert(b.tooltip == nil and model.value(b, "caption") == "Button1")

local names = model.propNames("Button")
assert(names[1] == "name" and names[2] == "caption" and names[3] == "left" and names[6] == "height", table.concat(names, ","))

local c = model.newControl(doc, "Label", 0, 0); doc[#doc + 1] = c
model.toBack(doc, c)
assert(doc[1] == c and model.indexOf(doc, b) == 2)
model.toFront(doc, c)
assert(doc[2] == c)

assert(table.concat(model.textToList("a\n\nb\r\nc\n"), ",") == "a,b,c")
assert(model.textToList("10\n20", true)[2] == 20)
local v = model.parse(model.literal({ { "x", 1 }, open = true, "y" }))
assert(v[1][1] == "x" and v[1][2] == 1 and v[2] == "y" and v.open == true)
assert(model.parse("os.exit()") == nil, "values are read with nothing in scope")`)
}

func TestProjectFiles(t *testing.T) {
	r, _ := newLua(t)
	dir := t.TempDir()
	r.L.SetGlobal("dir", lua.LString(dir))
	run(t, r.L, `
local project = require "design.project"
project.create(dir)
assert(table.concat(project.forms(dir), ",") == "Form1")
local doc = assert(project.read(dir, "Form1"))
assert(doc.kind == "Form" and doc.name == "Form1" and doc.width == 480)
doc[1] = { kind = "Button", name = "cmdOK", caption = "OK", left = 8, top = 8 }
project.save(dir, "Form1", doc)
project.addForm(dir, "About")
assert(table.concat(project.forms(dir), ",") == "About,Form1")
assert(project.read(dir, "Form1")[1].name == "cmdOK")`)
	for _, f := range []string{"main.lua", "forms/Form1.form.lua", "forms/Form1.lua", "forms/About.lua"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	main, _ := os.ReadFile(filepath.Join(dir, "main.lua"))
	if !strings.Contains(string(main), `require("forms.Form1"):show()`) {
		t.Errorf("main.lua:\n%s", main)
	}
	// What was made runs: the code loads the layout, and finds its button.
	code, _ := os.ReadFile(filepath.Join(dir, "forms/Form1.lua"))
	// The code it starts with names no control: a name there would make
	// renaming that control offer to rename it in the code.
	if strings.Contains(strings.ReplaceAll(string(code), "local frm", ""), "frm.") {
		t.Errorf("the code it starts with mentions a control:\n%s", code)
	}
	if !strings.Contains(string(code), `gui.load "Form1"`) {
		t.Errorf("Form1.lua:\n%s", code)
	}
	r2, _ := newLua(t)
	r2.L.SetGlobal("dir", lua.LString(dir))
	run(t, r2.L, `local frm = dofile(dir .. "/forms/Form1.lua"); assert(frm.cmdOK.caption == "OK")`)
}

func TestImagePaths(t *testing.T) {
	r, _ := newLua(t)
	dir := t.TempDir()
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "logo.png"), []byte("png"), 0o644)
	r.L.SetGlobal("dir", lua.LString(dir))
	r.L.SetGlobal("outside", lua.LString(outside))
	run(t, r.L, `
local project = require "design.project"
project.create(dir)
-- Inside the project, relative to forms/, where gui.load looks.
assert(project.imagePath(dir, dir .. "/images/a.png") == "../images/a.png")
assert(project.imagePath(dir, dir .. "/forms/b.png") == "b.png")
assert(project.imagePath(dir .. "/", dir .. "/c.png") == "../c.png")
assert(project.imagePath(dir, outside .. "/logo.png") == nil)
-- Outside it, copied in under a name of its own.
assert(project.copyIn(dir, outside .. "/logo.png") == "../images/logo.png")
assert(project.copyIn(dir, outside .. "/logo.png") == "../images/logo2.png")
local f = assert(io.open(dir .. "/images/logo2.png", "rb"))
assert(f:read("*a") == "png")
f:close()
-- The chooser starts where the image is, or in the project.
assert(project.imageDir(dir, "../images/logo.png") == dir .. "/forms/../images")
assert(project.imageDir(dir, "") == dir)
assert(project.imageDir(dir, "../nowhere/x.png") == dir)`)
}

func TestCode(t *testing.T) {
	r, _ := newLua(t)
	run(t, r.L, `
local code = require "design.code"
local text = 'local gui = require "gui"\nlocal frm = gui.load "Form1"\n\nreturn frm\n'
assert(code.formVar(text) == "frm")
assert(code.formVar('local win = gui.load "Main"') == "win")
assert(code.defaultEvent("Button") == "onClick" and code.defaultEvent("Label") == "onClick" and code.defaultEvent("Form") == "onLoad")
assert(code.defaultEvent("Panel") == nil and code.params("onClick", "Label") == "x, y" and code.params("onClick", "Button") == "")
assert(code.params("onPaint", "Panel") == "g" and code.params("onScroll", "Scroll") == "x, y")

local t2, line = code.addHandler(text, "frm", "Button1", "onClick")
assert(t2 == 'local gui = require "gui"\nlocal frm = gui.load "Form1"\n\nfunction frm.Button1:onClick()\n  \nend\n\nreturn frm\n', t2)
assert(line == 5, line)
assert(code.findHandler(t2, "frm", "Button1", "onClick") == 4)
assert(code.findHandler(t2, "frm", "Button1", "onChange") == nil)

local t3, line3 = code.addHandler(t2, "frm", "Canvas1", "onMouseDown")
assert(t3:find("function frm.Canvas1:onMouseDown%(x, y, button, double%)"), t3)
assert(select(2, t3:gsub("return frm", "")) == 1 and t3:find("end\n\nfunction frm.Canvas1"), t3)
assert(code.findHandler(t3, "frm", "Canvas1", "onMouseDown") + 1 == line3)

local t4 = code.addHandler(text, "frm", nil, "onClose")
assert(t4:find("function frm:onClose%(%)") and code.findHandler(t4, "frm", nil, "onClose"))
assert(code.addHandler(text, "frm", "Menu1", "onClick", "Menu"):find("function frm.Menu1:onClick%(name, caption, checked%)"))
assert(code.defaultEvent("Menu") == "onClick")
assert(code.findHandler("frm.Button1.onClick = function() end", "frm", "Button1", "onClick") == 1)

-- No return at the end: the handler goes at the end.
local t5, line5 = code.addHandler("local frm = gui.load 'X'", "frm", "B", "onClick")
assert(t5 == "local frm = gui.load 'X'\n\nfunction frm.B:onClick()\n  \nend\n" and line5 == 4, t5 .. line5)

-- The Component Interface turns rows of text into a layout's props, events
-- and public parts, and says why they cannot.
local interface = require "design.interface"
local doc = { kind = "Component", name = "Field", { kind = "Label", name = "lbl" }, { kind = "TextBox", name = "txt" } }
local props, events, parts = interface.rows(doc)
assert(#props == 0 and #events == 0 and #parts == 2 and parts[2][1] == "txt" and parts[2][2] == "TextBox" and parts[2][3] == "false")
props = { { "label", "string", "Name", "lbl.caption" }, { "size", "integer", "3", "" } }
events = { { "onChange", "txt.onChange" }, { "onSubmit", "" } }
parts[2][3] = "true"
assert(interface.check(props, events, parts) == nil)
interface.apply(doc, props, events, parts)
assert(doc.props.label.to == "lbl.caption" and doc.props.label.default == "Name" and doc.props.size.default == 3 and doc.props.size.to == nil)
assert(doc.events.onChange == "txt.onChange" and doc.events[1] == "onSubmit" and doc[2].public == true and doc[1].public == nil)
local again = { interface.rows(doc) }
assert(#again[1] == 2 and again[1][1][1] == "label" and #again[2] == 2 and again[3][2][3] == "true")
-- Methods are listed with a type, for the language server.
local mrows = { { "clear", "fun()" }, { "set", "fun(rgb: string): boolean" } }
assert(interface.check(props, events, parts, mrows) == nil)
interface.apply(doc, props, events, parts, mrows)
assert(doc.methods.clear == "fun()" and doc.methods.set == "fun(rgb: string): boolean")
local _, _, _, back = interface.rows(doc)
assert(#back == 2 and back[1][1] == "clear" and back[2][2] == "fun(rgb: string): boolean")
local function badMethod(rows, want)
  local why = interface.check(props, events, parts, rows)
  assert(why and why:find(want, 1, true), tostring(why))
end
badMethod({ { "a b", "fun()" } }, "not a name")
badMethod({ { "x", "number" } }, "reads like fun()")
badMethod({ { "x", "fun()" }, { "x", "fun()" } }, "twice")
badMethod({ { "size", "fun()" } }, "a property and a method")
badMethod({ { "visible", "fun()" } }, "every control has")
interface.apply(doc, props, events, parts, {})
assert(doc.methods == nil)

local function bad(p, e, want)
  local why = interface.check(p, e or {}, parts)
  assert(why and why:find(want, 1, true), tostring(why))
end
bad({ { "caption", "string", "", "" } }, nil, "every control has")
bad({ { "a b", "string", "", "" } }, nil, "not a name")
bad({ { "x", "string", "", "" }, { "x", "string", "", "" } }, nil, "twice")
bad({ { "x", "length", "", "" } }, nil, "the type is one of")
bad({ { "x", "integer", "abc", "" } }, nil, "must be a number")
bad({ { "x", "string", "", "nobody.text" } }, nil, "there is no part called nobody")
bad({ { "x", "string", "", "lbl" } }, nil, "not written part.property")
bad({ { "x", "string", "", "lbl.nothing" } }, nil, "a Label has no property nothing")
bad({}, { { "change", "" } }, "not an event name")
bad({}, { { "onChange", "txt.onNothing" } }, "a TextBox has no event onNothing")
bad({}, { { "onShow", "" } }, "every control has")

-- What is defined where, for the outline of a file of source.
local outline = code.outline("local function a() end\nfunction M.b(x)\nend\nfunction Obj:c() end\nlocal d = function() end\nM.e = function() end\nlocal f = functional\n  function g() end\nprint('function h')\n")
local names = {}
for _, f in ipairs(outline) do names[#names + 1] = f.name .. ":" .. f.line end
assert(table.concat(names, ",") == "a:1,M.b:2,Obj:c:4,d:5,M.e:6,g:8", table.concat(names, ","))

-- A component's code is a function of the component and its parts: the
-- handlers go inside it, on parts.
local comp = 'local gui = require "gui"\n\nreturn function(self, parts)\nend\n'
assert(code.formVar(comp) == "parts")
local c1, cline, cindent = code.addHandler(comp, "parts", "txt", "onChange")
assert(c1 == 'local gui = require "gui"\n\nreturn function(self, parts)\n  function parts.txt:onChange()\n    \n  end\nend\n', c1)
assert(cline == 5 and cindent == 4 and code.findHandler(c1, "parts", "txt", "onChange") == 4)
local c2 = code.addHandler(c1, "parts", "btn", "onClick")
assert(c2:find("  end\n\n  function parts.btn:onClick%(%)\n    \n  end\nend\n"), c2)
assert(code.countRefs("parts.txt.text = 1", "parts", "txt") == 1)

local src = "frm.Button1.caption = 1\nfrm.Button10.caption = 2\nfunction frm.Button1:onClick() end"
assert(code.countRefs(src, "frm", "Button1") == 2)
local renamed, n = code.renameRefs(src, "frm", "Button1", "cmdOK")
assert(n == 2 and renamed == "frm.cmdOK.caption = 1\nfrm.Button10.caption = 2\nfunction frm.cmdOK:onClick() end", renamed)

assert(select(1, code.errorAt("tlua: ./forms/Main.lua:12: attempt to call a nil value")) == "Main")
assert(select(2, code.errorAt("/tmp/p/forms/Main.lua:12: boom")) == 12)
assert(code.errorAt("main.lua:3: boom") == nil and code.errorAt("forms/Main.form.lua:3: x") == nil)`)
}

func TestArranging(t *testing.T) {
	r, _ := newLua(t)
	run(t, r.L, `
local model = require "design.model"
local a, b, c = {}, {}, {}
local rects = {
  { node = a, l = 10, t = 10, w = 50, h = 20 },
  { node = b, l = 40, t = 50, w = 30, h = 40 },
  { node = c, l = 100, t = 30, w = 20, h = 10 },
}
local ref = rects[1]
local o = model.arrange("lefts", rects, ref, 400, 300)
assert(o[a].l == 10 and o[b].l == 10 and o[c].l == 10 and o[b].t == 50)
o = model.arrange("rights", rects, ref)
assert(o[b].l == 30 and o[c].l == 40)
o = model.arrange("bottoms", rects, ref)
assert(o[b].t == -10 and o[c].t == 20)
o = model.arrange("centers", rects, ref)
assert(o[b].l == 20 and o[c].l == 25)
o = model.arrange("size", rects, ref)
assert(o[c].w == 50 and o[c].h == 20)
o = model.arrange("centerH", rects, ref, 400, 300)
-- the group spans 10..120, 110 wide: it starts at (400 - 110) / 2 = 145
assert(o[a].l == 145 and o[c].l == 235)
o = model.arrange("spaceH", rects, ref)
-- from 10 to 120, with 100 of controls: gaps of 5
assert(o[a].l == 10 and o[b].l == 65 and o[c].l == 100, o[b].l)
assert(model.snap(13) == 16 and model.snap(11) == 8 and model.snap(-3) == 0)`)
}

func TestCopyAndPaste(t *testing.T) {
	r, _ := newLua(t)
	run(t, r.L, `
local model = require "design.model"
local doc = model.newForm("F")
doc[1] = { kind = "Button", name = "Button1", caption = "Button1", left = 10, top = 10 }
doc[2] = { kind = "Tabs", name = "Tabs1", left = 0, top = 50, { kind = "Page", name = "Page1", caption = "One" } }
local text = model.copyText({ doc[1], doc[2] })
local pasted = model.pasteNodes(text, doc, 8)
assert(#pasted == 2 and pasted[1].name == "Button2" and pasted[1].caption == "Button2" and pasted[1].left == 18)
assert(pasted[2].name == "Tabs2" and pasted[2][1].name == "Page2" and pasted[2].top == 58)
assert(doc[1].left == 10, "the copy is a copy")
assert(model.pasteNodes("hello", doc) == nil and model.pasteNodes("-- tlua design: controls\n{ { kind = 'Nope' } }", doc) == nil)
local fresh = model.newForm("G")
assert(model.pasteNodes(text, fresh, 0)[1].name == "Button1", "free names are kept")`)
}

func TestMenuFlattening(t *testing.T) {
	r, _ := newLua(t)
	run(t, r.L, `
local model = require "design.model"
local items = {
  { "&File", { { "&Open", name = "mnuOpen", shortcut = "Cmd+O" }, "-", { "&Quit", name = "mnuQuit" } } },
  { "&View", { { "&Wrap", checked = true }, { "&More", { { "Deep", enabled = false } } } } },
  { "&Help" },
}
local flat = model.flattenMenu(items)
assert(#flat == 9)
assert(flat[1].caption == "&File" and flat[1].level == 0 and flat[2].level == 1 and flat[3].caption == "-")
assert(flat[8].caption == "Deep" and flat[8].level == 2 and flat[8].enabled == false)
local back = model.unflattenMenu(flat)
assert(model.literal(back) == model.literal(items), model.literal(back))`)
}

func TestStartupForm(t *testing.T) {
	r, _ := newLua(t)
	dir := t.TempDir()
	r.L.SetGlobal("dir", lua.LString(dir))
	run(t, r.L, `
local project = require "design.project"
project.create(dir)
project.addForm(dir, "About")
assert(project.startup(dir) == "Form1")
assert(project.setStartup(dir, "About"))
assert(project.startup(dir) == "About")
local f = io.open(dir .. "/main.lua", "a"); f:write("print('mine')\n"); f:close()
local ok, why = project.setStartup(dir, "Form1")
assert(not ok and why:find("changed by hand") and project.startup(dir) == "About")`)
}

// TestStubsTypeTheForm runs lua-language-server, where there is one, on a
// project the designer made: the stub it writes is what tells the server
// that frm.cmdGo is a Button, so giving its onClick a number is an error.
func TestStubsTypeTheForm(t *testing.T) {
	lls, err := exec.LookPath("lua-language-server")
	if err != nil {
		t.Skip("no lua-language-server on PATH")
	}
	library, _ := filepath.Abs("../../library")
	dir := t.TempDir()
	r, _ := newLua(t)
	r.L.SetGlobal("dir", lua.LString(dir))
	run(t, r.L, `
local project = require "design.project"
project.create(dir)
local doc = project.read(dir, "Form1")
doc[1] = { kind = "Button", name = "cmdGo", caption = "Go" }
project.save(dir, "Form1", doc)`)
	code, _ := os.ReadFile(filepath.Join(dir, "forms/Form1.lua"))
	text := strings.Replace(string(code), "return frm", "frm.cmdGo.onClick = 5\nreturn frm", 1)
	os.WriteFile(filepath.Join(dir, "forms/Form1.lua"), []byte(text), 0o644)
	os.WriteFile(filepath.Join(dir, ".luarc.json"), []byte(`{"runtime.version": "Lua 5.1", "workspace.library": ["`+library+`"]}`), 0o644)
	out := filepath.Join(t.TempDir(), "check.json")
	cmd := exec.Command(lls, "--check="+dir, "--checklevel=Warning", "--check_format=json", "--check_out_path="+out, "--logpath="+t.TempDir())
	cmd.Dir = dir
	// It exits with 1 when it finds something, which is the point here.
	b, _ := cmd.CombinedOutput()
	report, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("lua-language-server wrote no report: %v\n%s", err, b)
	}
	// The one warning is about the number given to a Button's onClick: with
	// no stub, cmdGo is an undefined field instead, and a wrongly declared frm
	// would add a warning of its own.
	if !strings.Contains(string(report), "fun(self: gui.Button)") {
		t.Errorf("the language server did not see frm.cmdGo as a Button:\n%s", report)
	}
	if n := strings.Count(string(report), `"code"`); n != 1 {
		t.Errorf("%d warnings, where there should be one:\n%s", n, report)
	}
}

func TestComponentsHoldingComponents(t *testing.T) {
	r, _ := newLua(t)
	run(t, r.L, `dir = ...`) // placeholder so the chunk below can use dir
	r.L.SetGlobal("dir", lua.LString(t.TempDir()))
	run(t, r.L, `
local project = require "design.project"
local model = require "design.model"
gui = require "gui"
project.create(dir)
assert(project.addComponent(dir, "Inner"))
assert(project.addComponent(dir, "Middle"))
assert(project.addComponent(dir, "Outer"))
assert(project.addComponent(dir, "Alone"))
-- Middle holds Inner, Outer holds Middle.
local middle = project.readComponent(dir, "Middle")
middle[1] = { kind = "Inner", name = "i" }
project.saveComponent(dir, "Middle", middle)
local outer = project.readComponent(dir, "Outer")
outer[1] = { kind = "Middle", name = "m" }
project.saveComponent(dir, "Outer", outer)
assert(table.concat(project.holders(dir, "Inner"), ",") == "Middle,Outer")
assert(table.concat(project.holders(dir, "Middle"), ",") == "Outer")
assert(#project.holders(dir, "Outer") == 0 and #project.holders(dir, "Alone") == 0)
assert(table.concat(project.components(dir), ",") == "Alone,Inner,Middle,Outer")`)
}

func TestProjectKinds(t *testing.T) {
	r, _ := newLua(t)
	r.L.SetGlobal("dir", lua.LString(t.TempDir()))
	run(t, r.L, `
local project = require "design.project"
gui = require "gui"
local lfs = require "lfs"
assert(project.kind(dir) == nil, "an empty folder is no project")

local con, game, app = dir .. "/con", dir .. "/game", dir .. "/app"
project.createProject(con, "console")
project.createProject(game, "game")
project.createProject(app, "gui")
assert(project.kind(con) == "console" and project.kind(game) == "game" and project.kind(app) == "gui")
assert(io.open(con .. "/.tlua-project"):read("*a"):find('type = "console"'))
assert(io.open(game .. "/main.lua"):read("*a"):find("function _draw"))
assert(not lfs.attributes(app .. "/.tlua-project"), "a GUI application is told by its forms")

-- Without the marker, a folder with a main.lua is read for what it is.
os.remove(game .. "/.tlua-project")
assert(project.kind(game) == "game", "_draw makes it a game")
os.remove(con .. "/.tlua-project")
assert(project.kind(con) == "console")
local f = io.open(dir .. "/plain.lua", "w"); f:write("print(1)"); f:close()

-- What was there stays.
local f = io.open(con .. "/main.lua", "w"); f:write("print('mine')"); f:close()
project.createProject(con, "console")
assert(io.open(con .. "/main.lua"):read("*a") == "print('mine')")

-- Files: Lua only, nested, without the dotted folders and declarations.
assert(project.addFile(con, "lib/util.lua") and not project.addFile(con, "lib/util.lua"))
assert(project.addFile(con, "a/b/c.lua"))
lfs.mkdir(con .. "/.git"); local g = io.open(con .. "/.git/x.lua", "w"); g:write("x"); g:close()
local d = io.open(con .. "/lib/util.d.lua", "w"); d:write("x"); d:close()
local n = io.open(con .. "/notes.txt", "w"); n:write("x"); n:close()
assert(table.concat(project.files(con), ",") == "a/b/c.lua,lib/util.lua,main.lua", table.concat(project.files(con), ","))`)
}

func TestProjectSetKind(t *testing.T) {
	r, _ := newLua(t)
	r.L.SetGlobal("dir", lua.LString(t.TempDir()))
	run(t, r.L, `
local project = require "design.project"
gui = require "gui"
project.createProject(dir, "console")
assert(project.kind(dir) == "console")
project.setKind(dir, "game")
assert(project.kind(dir) == "game")
project.setKind(dir, "console")
assert(project.kind(dir) == "console")

-- A component's methods are in what a language server reads.
project.create(dir .. "/app")
assert(project.addComponent(dir .. "/app", "Field"))
local comp = project.readComponent(dir .. "/app", "Field")
comp.methods = { clear = "fun()", set = "fun(text: string)" }
project.saveComponent(dir .. "/app", "Field", comp)
local stub = io.open(dir .. "/app/components/Field.d.lua"):read("*a")
assert(stub:find("---@field clear fun()", 1, true) and stub:find("---@field set fun(text: string)", 1, true), stub)
assert(project.readComponent(dir .. "/app", "Field").methods.clear == "fun()")`)
}

func TestDesignCommandOptions(t *testing.T) {
	for _, args := range [][]string{
		{"-new"},                       // needs a kind
		{"-new", "spreadsheet", "dir"}, // no such kind
		{"-new", "game"},               // needs a folder
	} {
		if code := Command(args); code != 1 {
			t.Errorf("tlua design %v: exit %d, want 1", args, code)
		}
	}
	if code := Command([]string{"-h"}); code != 0 {
		t.Errorf("-h: exit %d", code)
	}
}
