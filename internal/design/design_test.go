package design

import (
	"os"
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
	if !strings.Contains(string(code), `gui.load "Form1"`) {
		t.Errorf("Form1.lua:\n%s", code)
	}
	r2, _ := newLua(t)
	r2.L.SetGlobal("dir", lua.LString(dir))
	run(t, r2.L, `local frm = dofile(dir .. "/forms/Form1.lua"); assert(frm.cmdOK.caption == "OK")`)
}

func TestCode(t *testing.T) {
	r, _ := newLua(t)
	run(t, r.L, `
local code = require "design.code"
local text = 'local gui = require "gui"\nlocal frm = gui.load "Form1"\n\nreturn frm\n'
assert(code.formVar(text) == "frm")
assert(code.formVar('local win = gui.load "Main"') == "win")
assert(code.defaultEvent("Button") == "onClick" and code.defaultEvent("Label") == nil)

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
assert(code.findHandler("frm.Button1.onClick = function() end", "frm", "Button1", "onClick") == 1)

-- No return at the end: the handler goes at the end.
local t5, line5 = code.addHandler("local frm = gui.load 'X'", "frm", "B", "onClick")
assert(t5 == "local frm = gui.load 'X'\n\nfunction frm.B:onClick()\n  \nend\n" and line5 == 4, t5 .. line5)

local src = "frm.Button1.caption = 1\nfrm.Button10.caption = 2\nfunction frm.Button1:onClick() end"
assert(code.countRefs(src, "frm", "Button1") == 2)
local renamed, n = code.renameRefs(src, "frm", "Button1", "cmdOK")
assert(n == 2 and renamed == "frm.cmdOK.caption = 1\nfrm.Button10.caption = 2\nfunction frm.cmdOK:onClick() end", renamed)

assert(select(1, code.errorAt("tlua: ./forms/Main.lua:12: attempt to call a nil value")) == "Main")
assert(select(2, code.errorAt("/tmp/p/forms/Main.lua:12: boom")) == 12)
assert(code.errorAt("main.lua:3: boom") == nil and code.errorAt("forms/Main.form.lua:3: x") == nil)`)
}
