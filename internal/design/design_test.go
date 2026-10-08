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
