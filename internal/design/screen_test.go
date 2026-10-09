//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package design

// These tests open the designer's window and drive it with synthetic input,
// as the gui module's own do, so they only run when asked:
// TLUA_GUI_TESTS=1 go test ./internal/design (or make test-gui).

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/pwiecz/go-fltk"
	lua "github.com/yuin/gopher-lua"

	in "tlua/internal/gui/fltkinput"
)

var guiTests = os.Getenv("TLUA_GUI_TESTS") != ""

var mainCalls = make(chan func())

func TestMain(m *testing.M) {
	if !guiTests {
		os.Exit(m.Run())
	}
	done := make(chan int)
	go func() { done <- m.Run() }()
	for {
		select {
		case f := <-mainCalls:
			f()
		case code := <-done:
			os.Exit(code)
		}
	}
}

type scene struct {
	t   *testing.T
	L   *lua.LState
	dir string
}

// withDesigner opens the designer on a new project in a fresh directory
// and runs steps against it, on the main thread. Checks use Errorf only.
func withDesigner(t *testing.T, steps func(s *scene)) {
	t.Helper()
	if !guiTests {
		t.Skip("set TLUA_GUI_TESTS=1 to run tests that open windows")
	}
	dir := t.TempDir()
	done := make(chan struct{})
	mainCalls <- func() {
		defer close(done)
		cwd, _ := os.Getwd()
		defer os.Chdir(cwd) // the designer moves into the project's forms/
		r, _ := newLua(t)
		s := &scene{t: t, L: r.L, dir: dir}
		r.L.SetGlobal("dir", lua.LString(dir))
		// Questions answer themselves "no" unless a test says otherwise, and
		// a dialog that would still open fails the test rather than wait for
		// someone to answer it.
		if err := r.L.DoString(`bootgui(); D = require("design.main").start{dir = dir, create = true}; D.answer = "no"
local gui = require "gui"
gui.msgbox = function(m) error("a message box would wait for someone: " .. tostring(m)) end
gui.inputbox = function(m) error("an input box would wait for someone: " .. tostring(m)) end`); err != nil {
			t.Errorf("start: %v", err)
			return
		}
		defer func() {
			s.L.DoString(`D.answer = "no"; D.dirty = false; D.codeDirty = false; D.win:close()`)
			s.pump()
		}()
		s.pump()
		// A dialog nobody answers would wait for whoever is at the screen:
		// after 20 seconds it is sent Escape, and the test fails.
		finished := false
		var watch func()
		watch = func() {
			if finished {
				return
			}
			t.Errorf("something was still waiting after 20 seconds; it was sent Escape")
			in.Key(fltk.ESCAPE, "", 0)
			fltk.AddTimeout(20, watch)
		}
		fltk.AddTimeout(20, watch)
		defer func() { finished = true }()
		steps(s)
	}
	<-done
}

func (s *scene) pump() {
	for i := 0; i < 6; i++ {
		fltk.Wait(0.01)
	}
}

func (s *scene) lua(src string) {
	s.t.Helper()
	if err := s.L.DoString(src); err != nil {
		s.t.Errorf("%s: %v", src, err)
	}
}

func (s *scene) expect(expr string) {
	s.t.Helper()
	s.pump()
	if err := s.L.DoString("__ok = (" + expr + ")"); err != nil {
		s.t.Errorf("%s: %v", expr, err)
		return
	}
	if !lua.LVAsBool(s.L.GetGlobal("__ok")) {
		s.t.Errorf("not so: %s", expr)
	}
}

// at is a point of a control, given as a Lua expression, in the window.
func (s *scene) at(obj string, dx, dy int) (int, int) {
	s.t.Helper()
	s.lua(fmt.Sprintf(`__x, __y = require("design.main").at(%s, %d, %d)`, obj, dx, dy))
	return int(lua.LVAsNumber(s.L.GetGlobal("__x"))), int(lua.LVAsNumber(s.L.GetGlobal("__y")))
}

// pickTool clicks a kind in the toolbox.
func (s *scene) pickTool(kind string) {
	s.lua(fmt.Sprintf(`__tx, __ty = D.toolbox:rowOf(%q)`, kind))
	x, y := s.at("D.toolbox", int(lua.LVAsNumber(s.L.GetGlobal("__tx"))), int(lua.LVAsNumber(s.L.GetGlobal("__ty"))))
	in.Click(x, y)
	s.pump()
}

// drag drags on the design surface, in the form's coordinates.
func (s *scene) drag(x0, y0, x1, y1 int) {
	ox, oy := s.at("D.surface.overlay", 0, 0)
	in.Drag(ox+x0, oy+y0, ox+x1, oy+y1, 4)
	s.pump()
}

func (s *scene) click(x, y int) {
	ox, oy := s.at("D.surface.overlay", 0, 0)
	in.Click(ox+x, oy+y)
	s.pump()
}

func TestDesignerPlacesMovesResizesAndDeletes(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.expect(`D.formName == "Form1" and D.win.caption:find("Form1") and #D.doc == 0`)

		// Draw a button.
		s.pickTool("Button")
		s.expect(`D:tool() == "Button"`)
		s.drag(20, 30, 120, 60)
		s.expect(`#D.doc == 1 and D.doc[1].kind == "Button" and D.doc[1].name == "Button1"`)
		s.expect(`D.doc[1].left == 20 and D.doc[1].top == 30 and D.doc[1].width == 100 and D.doc[1].height == 30`)
		s.expect(`D:tool() == nil and D.selection == D.doc[1] and D.dirty and D.win.caption:find("%*")`)
		s.expect(`D.props:text("caption") == "Button1"`)

		// Move it.
		s.drag(40, 40, 70, 70)
		s.expect(`D.doc[1].left == 50 and D.doc[1].top == 60`)
		s.expect(`D.props:text("left") == "50"`)

		// Size it from its bottom right handle.
		s.drag(150, 90, 170, 100)
		// 120 is a Button's own width, so the layout leaves it unsaid.
		s.expect(`D.doc[1].width == nil and D.doc[1].height == 40 and D.surface.entries[1].obj.width == 120`)

		// A click on the form selects the form; Delete with a control
		// selected deletes it.
		s.click(300, 250)
		s.expect(`D.selection == D.doc`)
		s.click(60, 70)
		s.expect(`D.selection == D.doc[1]`)
		in.Key(fltk.DELETE, "", 0)
		s.expect(`#D.doc == 0 and D.selection == D.doc`)

		// A double-click in the toolbox puts one of its own size in the
		// middle of the form.
		s.lua(`__tx, __ty = D.toolbox:rowOf("Label")`)
		x, y := s.at("D.toolbox", int(lua.LVAsNumber(s.L.GetGlobal("__tx"))), int(lua.LVAsNumber(s.L.GetGlobal("__ty"))))
		in.Click(x, y)
		in.DoubleClick(x, y)
		s.expect(`#D.doc == 1 and D.doc[1].kind == "Label" and D.doc[1].left == 180 and D.doc[1].top == 146`)

		// The form's own corner handle sizes the form.
		s.click(400, 300) // nothing there: the form is selected
		s.drag(480, 320, 520, 340)
		s.expect(`D.doc.width == 520 and D.doc.height == 340`)
	})
}

func TestDesignerPropertyGrid(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.pickTool("Button")
		s.drag(10, 10, 110, 40)

		// A caption typed into the grid is the button's on Enter.
		s.typeProp("caption", "OK")
		s.expect(`D.doc[1].caption == "OK" and D.surface.entries[1].obj.caption == "OK"`)
		s.expect(`D.props:text("caption") == "OK" and D.props.grid:editing() == nil`)

		// A number that is not one is refused, and said so; the cell stays
		// as it was typed until Escape puts it back.
		s.typeProp("left", "x")
		s.expect(`D.doc[1].left == 10 and D.statusLine.caption:find("left must be a number")`)
		s.expect(`D.props.grid:editing() == D.props:row("left")`)
		in.Key(fltk.ESCAPE, "", 0)
		s.expect(`D.props.grid:editing() == nil and D.props:text("left") == "10"`)

		// Up and Down keep what was typed and go on to the next property.
		s.lua(`D.props.grid:edit(D.props:row("top"), 2)`)
		in.Type("12")
		in.Key(fltk.UP, "", 0)
		s.expect(`D.doc[1].top == 12 and D.props.grid:editing() == D.props:row("left")`)
		in.Key(fltk.ESCAPE, "", 0)

		// A name that is taken is refused by the control itself.
		s.lua(`D.surface:place("Label", 10, 60)`)
		s.lua(`D.surface:selectNode(D.doc[1])`)
		s.typeProp("name", "Label1")
		s.expect(`D.doc[1].name ~= "Label1" and D.statusLine.caption:find("already has a control named Label1")`)
		in.Key(fltk.BACKSPACE, "", 0)
		in.Type("X")
		in.Key(fltk.ENTER_KEY, "", 0)
		s.expect(`D.doc[1].name == "LabelX" and D.props:text("name") == "LabelX"`)

		// True or false is picked; a property that picks the widget makes
		// the control again.
		s.lua(`before = D.surface.entries[1].obj`)
		s.typeProp("default", "true")
		s.expect(`D.doc[1].default == true and D.surface.entries[1].obj ~= before`)
		s.expect(`D.surface.entries[1].obj.caption == "OK" and D.surface.entries[2].node.kind == "Label"`)
		s.typeProp("default", "maybe")
		s.expect(`D.doc[1].default == true and D.statusLine.caption:find("default must be true or false")`)
		in.Key(fltk.ESCAPE, "", 0)

		// A list is edited from its "...".
		s.lua(`D.surface:place("ListBox", 200, 60)`)
		s.lua(`D.props.editList = function() return { "one", "two" } end`)
		s.lua(`D.props.grid:edit(D.props:row("items"), 2); D.props:button(D.props:row("items"))`)
		s.expect(`#D.doc[3].items == 2 and D.props:text("items") == "(2)"`)
		s.lua(`D.props.grid:edit()`)

		// Selecting another control keeps an edit begun on the first.
		s.lua(`D.surface:selectNode(D.doc[1])`)
		s.lua(`D.props.grid:edit(D.props:row("caption"), 2)`)
		in.Type("Fine")
		s.lua(`D.surface:selectNode(D.doc[2])`)
		s.expect(`D.doc[1].caption == "Fine" and D.doc[2].caption ~= "Fine"`)

		// Bring to front and send to back reorder the layout.
		s.lua(`D.surface:selectNode(D.doc[1]); D.surface:toFront()`)
		s.expect(`D.doc[3].name == "LabelX"`)
	})
}

// typeProp types a value into the property grid and keeps it, as Enter
// does.
func (s *scene) typeProp(prop, value string) {
	s.t.Helper()
	s.lua(fmt.Sprintf(`D.props.grid:edit(D.props:row(%q), 2)`, prop))
	s.expect(fmt.Sprintf(`D.props.grid:editing() == D.props:row(%q)`, prop))
	in.Type(value)
	in.Key(fltk.ENTER_KEY, "", 0)
	s.pump()
}

func TestDesignerSavesWhatItShows(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.pickTool("TextBox")
		s.drag(16, 16, 216, 44)
		s.typeProp("name", "txtName")
		mod, _ := cmdKey()
		in.Key('s', "s", mod)
		s.expect(`not D.dirty and not D.win.caption:find("%*")`)
		data, err := os.ReadFile(filepath.Join(s.dir, "forms/Form1.form.lua"))
		if err != nil {
			s.t.Fatal(err)
		}
		want := `{ kind = "TextBox", name = "txtName", left = 16, top = 16, width = 200, height = 28 },`
		if !strings.Contains(string(data), want) {
			s.t.Errorf("saved:\n%s\nwant a line %s", data, want)
		}
	})
}

func TestDesignerRunsTheProgram(t *testing.T) {
	if testing.Short() {
		t.Skip("builds tlua")
	}
	exe := filepath.Join(t.TempDir(), "tlua")
	build := exec.Command("go", "build", "-o", exe, "tlua/cmd/tlua")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building tlua: %v\n%s", err, out)
	}
	withDesigner(t, func(s *scene) {
		os.WriteFile(filepath.Join(s.dir, "main.lua"), []byte(`print("hello from the program") os.exit(0)`), 0o644)
		s.L.SetGlobal("exe", lua.LString(exe))
		s.lua(`require("gui").interpreter = exe; D:run()`)
		for i := 0; i < 100; i++ {
			s.pump()
			if err := s.L.DoString(`__done = D.output.items[#D.output.items] == "> finished"`); err == nil && lua.LVAsBool(s.L.GetGlobal("__done")) {
				break
			}
		}
		s.expect(`D.output.items[2] == "hello from the program" and D.output.items[3] == "> finished"`)
	})
}

func cmdKey() (int, string) {
	if goruntime.GOOS == "darwin" {
		return fltk.META, "Cmd"
	}
	return fltk.CTRL, "Ctrl"
}

// TestDesignerRebuildsTheLayoutExample is phase 2's measure: the form of
// examples/gui/layout made in the designer from nothing, saved, wired up the
// way a programmer would, and run.
func TestDesignerRebuildsTheLayoutExample(t *testing.T) {
	if testing.Short() {
		t.Skip("builds tlua")
	}
	exe := filepath.Join(t.TempDir(), "tlua")
	if out, err := exec.Command("go", "build", "-o", exe, "tlua/cmd/tlua").CombinedOutput(); err != nil {
		t.Fatalf("building tlua: %v\n%s", err, out)
	}
	withDesigner(t, func(s *scene) {
		place := func(kind string, x0, y0, x1, y1 int, props map[string]string) {
			s.pickTool(kind)
			s.drag(x0, y0, x1, y1)
			for _, p := range []string{"name", "caption"} {
				if v, ok := props[p]; ok {
					s.typeProp(p, v)
				}
			}
		}
		place("Label", 16, 16, 76, 44, map[string]string{"name": "lblName", "caption": "Name"})
		place("TextBox", 80, 16, 344, 44, map[string]string{"name": "txtName"})
		place("Label", 16, 60, 344, 108, map[string]string{"name": "lblGreeting", "caption": "Type a name."})
		place("Button", 244, 140, 344, 168, map[string]string{"name": "cmdGreet", "caption": "Greet"})
		s.expect(`#D.doc == 4 and D.doc[4].name == "cmdGreet" and D.doc[4].caption == "Greet"`)
		s.lua(`D:save()`)

		code := `local gui = require "gui"
local frm = gui.load "Form1"
function frm.cmdGreet:onClick()
  frm.lblGreeting.caption = "Hello, " .. frm.txtName.text .. "!"
end
gui.after(0.2, function()
  frm.txtName.text = "Ada"
  frm.cmdGreet:fire("click")
  print(frm.lblGreeting.caption)
  frm:close()
end)
return frm
`
		if err := os.WriteFile(filepath.Join(s.dir, "forms/Form1.lua"), []byte(code), 0o644); err != nil {
			s.t.Fatal(err)
		}
		s.L.SetGlobal("exe", lua.LString(exe))
		s.lua(`require("gui").interpreter = exe; D:run()`)
		for i := 0; i < 200; i++ {
			s.pump()
			if err := s.L.DoString(`__done = D.output.items[#D.output.items]:find("^> ") and #D.output.items > 1`); err == nil && lua.LVAsBool(s.L.GetGlobal("__done")) {
				break
			}
		}
		s.expect(`D.output.items[2] == "Hello, Ada!" and D.output.items[3] == "> finished"`)
	})
}

// later answers a dialog: fn runs from inside its event loop.
func later(fn func()) { fltk.AddTimeout(0.3, fn) }

func TestDesignerDoubleClickWritesAHandler(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.pickTool("Button")
		s.drag(20, 20, 120, 50)
		ox, oy := s.at("D.surface.overlay", 60, 30)
		in.Click(ox, oy)
		in.DoubleClick(ox, oy)
		s.pump()
		s.expect(`D.views.selected == 2 and D.codeDirty`)
		s.expect(`D.codeBox.text:find("function frm.Button1:onClick%(%)\n  \nend\n\nreturn frm")`)
		in.Type(`print("hi")`)
		s.expect(`D.codeBox.text:find('function frm.Button1:onClick%(%)\n  print%("hi"%)\nend')`)
		s.expect(`D.objectBox.text == "Button1" and D.eventBox.items[1]:find("onClick") and D.eventBox.items[1]:find("•")`)
		s.expect(`D.eventBox.text:find("onClick")`)

		// Double-clicking again goes to the handler, and writes no second one.
		s.lua(`D:showDesign()`)
		s.pump()
		in.Click(ox, oy)
		in.DoubleClick(ox, oy)
		s.expect(`select(2, D.codeBox.text:gsub("onClick", "")) == 1`)
		s.expect(`D.codeBox.line == require("design.code").findHandler(D.codeBox.text, "frm", "Button1", "onClick") + 1`)

		s.lua(`D:save()`)
		data, _ := os.ReadFile(filepath.Join(s.dir, "forms/Form1.lua"))
		if !strings.Contains(string(data), "function frm.Button1:onClick()\n  print(\"hi\")\nend") {
			s.t.Errorf("code saved:\n%s", data)
		}
		s.expect(`not D.codeDirty and not D.dirty`)
	})
}

func TestDesignerObjectAndEventBoxes(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.pickTool("Canvas")
		s.drag(10, 10, 110, 80)
		s.lua(`D:showCode()`)
		s.expect(`D.objectBox.items[1] == "(Form1)" and D.objectBox.items[2] == "Canvas1" and D.objectBox.selected == 2`)
		s.lua(`for i, e in ipairs(D.eventNames) do if e == "onDraw" then D:pickEvent(i) end end`)
		s.expect(`D.codeBox.text:find("function frm.Canvas1:onDraw%(g%)")`)
		s.lua(`D:pickObject(1); for i, e in ipairs(D.eventNames) do if e == "onClose" then D:pickEvent(i) end end`)
		s.expect(`D.codeBox.text:find("function frm:onClose%(%)")`)
	})
}

func TestDesignerOffersToRename(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.pickTool("Button")
		s.drag(20, 20, 120, 50)
		s.lua(`D.answer = "yes"`)
		s.lua(`D:openHandler(D.doc[1], "onClick"); D.codeBox.text = D.codeBox.text .. "frm.Button1.caption = 'x'\n"; D:showDesign()`)
		s.lua(`D.surface:selectNode(D.doc[1])`)
		s.typeProp("name", "cmdGo")
		s.expect(`D.doc[1].name == "cmdGo"`)
		s.click(400, 280) // selecting the form ends the renaming
		s.expect(`D.asked == "Rename frm.Button1 to frm.cmdGo in the code? It is there 2 times."`)
		s.expect(`D.codeBox.text:find("function frm.cmdGo:onClick") and D.codeBox.text:find("frm.cmdGo.caption") and not D.codeBox.text:find("Button1")`)
	})
}

func TestDesignerFindsAndGoesToLines(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.lua(`D:showCode(); D.codeBox.cursor = 0; D.lastFind = "gui.load"; D:find(true)`)
		s.expect(`D.codeBox.selectedText == "gui.load"`)
		s.lua(`D.lastFind = "no such thing"; D:find(true)`)
		s.expect(`D.statusLine.caption:find("is not in the code")`)
		s.lua(`D.answerText = "2"`)
		s.lua(`D:goToLine()`)
		s.expect(`D.codeBox.line == 2`)
	})
}

func TestDesignerReadsCodeChangedElsewhere(t *testing.T) {
	withDesigner(t, func(s *scene) {
		path := filepath.Join(s.dir, "forms/Form1.lua")
		if err := os.WriteFile(path, []byte("-- written elsewhere\nlocal gui = require \"gui\"\nlocal frm = gui.load \"Form1\"\nreturn frm\n"), 0o644); err != nil {
			s.t.Fatal(err)
		}
		future := time.Now().Add(5 * time.Second)
		os.Chtimes(path, future, future)
		s.lua(`D:showCode()`)
		s.expect(`D.codeBox.text:find("written elsewhere") and not D.codeDirty`)
	})
}

func TestDesignerJumpsToAnError(t *testing.T) {
	if testing.Short() {
		t.Skip("builds tlua")
	}
	exe := filepath.Join(t.TempDir(), "tlua")
	if out, err := exec.Command("go", "build", "-o", exe, "tlua/cmd/tlua").CombinedOutput(); err != nil {
		t.Fatalf("building tlua: %v\n%s", err, out)
	}
	withDesigner(t, func(s *scene) {
		s.lua(`D:showCode()
D.codeBox.text = 'local gui = require "gui"\nlocal frm = gui.load "Form1"\n\nfrm.NoSuchControl.caption = "x"\n\nreturn frm\n'
D:markCodeDirty()
D:showDesign()`)
		s.L.SetGlobal("exe", lua.LString(exe))
		s.lua(`require("gui").interpreter = exe; D:run()`)
		for i := 0; i < 200; i++ {
			s.pump()
			if err := s.L.DoString(`__done = D.output.items[#D.output.items]:find("^> exited")`); err == nil && lua.LVAsBool(s.L.GetGlobal("__done")) {
				break
			}
		}
		s.expect(`D.output.items[#D.output.items]:find("^> exited with")`)
		s.expect(`D.views.selected == 2 and D.codeBox.line == 4`)
	})
}

// place draws a control of a kind on the form, and names it.
func (s *scene) place(kind string, x0, y0, x1, y1 int, name string) {
	s.pickTool(kind)
	if x1 == 0 && y1 == 0 {
		s.click(x0, y0)
	} else {
		s.drag(x0, y0, x1, y1)
	}
	s.typeProp("name", name)
	s.pump()
}

func (s *scene) shiftClick(x, y int) {
	ox, oy := s.at("D.surface.overlay", 0, 0)
	in.ClickWith(ox+x, oy+y, fltk.SHIFT)
	s.pump()
}

func TestDesignerSelectsSeveral(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.place("Button", 20, 20, 120, 50, "a")
		s.place("Button", 20, 80, 120, 110, "b")
		s.place("Button", 300, 200, 400, 230, "c")

		// A rubber band round the first two selects them.
		s.drag(10, 10, 200, 150)
		s.expect(`#D.surface.sels == 2 and D.statusLine.caption:find("2 controls selected")`)
		s.shiftClick(350, 215)
		s.expect(`#D.surface.sels == 3 and D.surface.sel.node.name == "c"`)
		s.shiftClick(350, 215)
		s.expect(`#D.surface.sels == 2`)

		// Dragging one of them drags both.
		s.drag(60, 30, 90, 50)
		s.expect(`D.doc[1].left == 50 and D.doc[1].top == 40 and D.doc[2].left == 50 and D.doc[2].top == 100`)

		// The grid's properties change both; a name is one control's own.
		s.typeProp("caption", "Go")
		// (Each was Button1 when made: a renamed control's name is free again.)
		s.expect(`D.doc[1].caption == "Go" and D.doc[2].caption == "Go" and D.doc[3].caption == "Button1"`)

		in.Key(fltk.DELETE, "", 0)
		// The caption box had the keyboard; the surface takes it back with a click.
		s.click(60, 45)
		s.drag(10, 10, 200, 150)
		in.Key(fltk.DELETE, "", 0)
		s.expect(`#D.doc == 1 and D.doc[1].name == "c"`)
	})
}

func TestDesignerAlignsAndSnaps(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.place("Button", 20, 20, 120, 50, "a")
		s.place("Button", 47, 80, 127, 110, "b")
		s.click(60, 30)      // a ...
		s.shiftClick(70, 90) // ... then b, which the others line up with
		s.lua(`D.surface:arrange("lefts")`)
		s.expect(`D.doc[1].left == 47 and D.doc[2].left == 47`)
		s.lua(`D.surface:arrange("width")`)
		s.expect(`D.doc[1].width == 80`)

		s.lua(`D.snap = true; D.gridShown = true`)
		s.pickTool("Label")
		s.drag(13, 150, 101, 181)
		s.expect(`D.doc[3].left == 16 and D.doc[3].top == 152 and D.doc[3].width == 88 and D.doc[3].height == 32`)
	})
}

func TestDesignerUndoes(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.pickTool("Button")
		s.drag(20, 20, 120, 50)
		s.typeProp("caption", "Click me")
		s.expect(`D.doc[1].caption == "Click me"`)
		s.lua(`D:undo()`)
		s.expect(`D.doc[1].caption == "Button1" and D.surface.entries[1].obj.caption == "Button1"`)
		s.lua(`D:undo()`)
		s.expect(`#D.doc == 0 and #D.surface.entries == 0`)
		s.lua(`D:redo(); D:redo()`)
		s.expect(`#D.doc == 1 and D.doc[1].caption == "Click me" and D.surface.sel.node == D.doc[1]`)

		s.drag(60, 30, 100, 60)
		s.expect(`D.doc[1].left == 60`)
		mod, _ := cmdKey()
		in.Key('z', "z", mod)
		s.expect(`D.doc[1].left == 20`)
		// With Shift held a keyboard types "Z", which is what tells Redo
		// from Undo: FLTK matches a shortcut without Shift on the text too.
		in.Key('z', "Z", mod|fltk.SHIFT)
		s.expect(`D.doc[1].left == 60`)
	})
}

func TestDesignerCopiesAndPastes(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.place("Button", 20, 20, 120, 50, "cmdOK")
		s.click(60, 30)
		mod, _ := cmdKey()
		in.Key('c', "c", mod)
		in.Key('v', "v", mod)
		s.expect(`#D.doc == 2 and D.doc[2].name == "Button1" and D.doc[2].left == 28 and D.doc[2].top == 28`)
		in.Key('v', "v", mod)
		s.expect(`#D.doc == 3 and D.doc[3].name == "Button2" and D.doc[3].left == 36`)
		in.Key('d', "d", mod)
		s.expect(`#D.doc == 4 and D.doc[4].left == 44 and D.surface.sel.node == D.doc[4]`)
		in.Key('x', "x", mod)
		s.expect(`#D.doc == 3`)
		in.Key('a', "a", mod)
		s.expect(`#D.surface.sels == 3`)
	})
}

func TestDesignerTabOrder(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.place("TextBox", 20, 20, 120, 48, "txtA")
		s.place("TextBox", 20, 80, 120, 108, "txtB")
		s.place("Label", 20, 140, 120, 168, "lblC")
		s.lua(`D:setTabOrder(true)`)
		s.click(60, 90)  // txtB first
		s.click(60, 30)  // then txtA
		s.click(60, 150) // a Label takes no keyboard: nothing
		s.expect(`D.doc[2].tabIndex == 1 and D.doc[1].tabIndex == 2 and D.doc[3].tabIndex == nil`)
		in.Key(fltk.ESCAPE, "", 0)
		s.expect(`not D.surface.tabMode`)
		s.lua(`D:save()`)
		data, _ := os.ReadFile(filepath.Join(s.dir, "forms/Form1.form.lua"))
		if !strings.Contains(string(data), `name = "txtB", left = 20, top = 80, width = 100, height = 28, tabIndex = 1`) {
			s.t.Errorf("saved:\n%s", data)
		}
	})
}

func TestDesignerMenuEditor(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.lua(`E = {}; D.menuEditorHooks = E`)
		click := func(obj string) {
			x, y := s.at(obj, 10, 10)
			in.Click(x, y)
			s.pump()
		}
		later(func() {
			s.lua(`E.caption:focus()`)
			in.Type("&File")
			click("E.next")
			in.Type("&Open")
			s.lua(`E.shortcut:focus()`)
			in.Type("Cmd+O")
			s.lua(`E.name:focus()`)
			in.Type("mnuOpen")
			click("E.indent")
			click("E.next")
			in.Type("-")
			click("E.next")
			in.Type("&Quit")
			s.lua(`E.name:focus()`)
			in.Type("mnuQuit")
			click("E.ok")
		})
		s.lua(`D:editMenu()`)
		s.expect(`D.doc[1].kind == "Menu" and D.doc[1].name == "Menu1"`)
		s.expect(`require("design.model").literal(D.doc[1].items) == '{ { "&File", { { "&Open", name = "mnuOpen", shortcut = "Cmd+O" }, "-", { "&Quit", name = "mnuQuit" } } } }'`)
		s.expect(`D.surface.entries[1].obj.width == 480`)

		// Opening it again shows what there is; deleting every item takes the menu away.
		later(func() {
			s.lua(`E.list.selected = 1; E.list.onChange(E.list)`)
			for i := 0; i < 4; i++ {
				click("E.delete")
			}
			click("E.ok")
		})
		s.lua(`D:editMenu()`)
		s.expect(`#D.doc == 0`)
	})
}

func TestDesignerStartupForm(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.expect(`D.startupLine.caption == "Starts with Form1"`)
		s.lua(`require("design.project").addForm(D.dir, "About"); D:openForm("About"); D:setStartup()`)
		s.expect(`D.startupLine.caption == "Starts with About"`)
		data, _ := os.ReadFile(filepath.Join(s.dir, "main.lua"))
		if !strings.Contains(string(data), `require("forms.About"):show()`) {
			s.t.Errorf("main.lua:\n%s", data)
		}
	})
}

func TestDesignerMakesAnExe(t *testing.T) {
	testPacking(t, "makeExeTo", "app", "fuse", func(exe, out string) *exec.Cmd { return exec.Command(out) })
}

func TestDesignerExportsABundle(t *testing.T) {
	testPacking(t, "exportBundleTo", "app.ztl", "bundle", func(exe, out string) *exec.Cmd { return exec.Command(exe, out) })
}

// testPacking packs a project with the designer's method into a file called
// name, and runs what it made.
func testPacking(t *testing.T, method, name, command string, run func(exe, out string) *exec.Cmd) {
	if testing.Short() {
		t.Skip("builds tlua")
	}
	exe := filepath.Join(t.TempDir(), "tlua")
	if out, err := exec.Command("go", "build", "-o", exe, "tlua/cmd/tlua").CombinedOutput(); err != nil {
		t.Fatalf("building tlua: %v\n%s", err, out)
	}
	app := filepath.Join(t.TempDir(), name)
	withDesigner(t, func(s *scene) {
		os.WriteFile(filepath.Join(s.dir, "main.lua"), []byte(`print("made by tlua design")`), 0o644)
		s.L.SetGlobal("exe", lua.LString(exe))
		s.L.SetGlobal("app", lua.LString(app))
		s.lua(`require("gui").interpreter = exe; D:` + method + `(app)`)
		for i := 0; i < 300; i++ {
			s.pump()
			if err := s.L.DoString(`__done = D.output.items[#D.output.items]:find("^> made") or D.output.items[#D.output.items]:find("^> ` + command + ` exited")`); err == nil && lua.LVAsBool(s.L.GetGlobal("__done")) {
				break
			}
		}
		s.expect(`D.output.items[#D.output.items] == "> made " .. app`)
		out, err := run(exe, app).CombinedOutput()
		if err != nil || strings.TrimSpace(string(out)) != "made by tlua design" {
			s.t.Errorf("running what was made: %v %q", err, out)
		}
	})
}

func TestDesignerNestsControls(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.place("Frame", 20, 20, 220, 160, "fraOpts")
		// Drawn inside the frame, a control goes in it, placed from its corner.
		s.place("CheckBox", 40, 60, 160, 88, "chkBold")
		s.expect(`#D.doc == 1 and D.doc[1][1].name == "chkBold" and D.doc[1][1].left == 20 and D.doc[1][1].top == 40`)
		s.expect(`D.surface.entries[2].obj.parent == D.surface.entries[1].obj`)

		// Dragged out onto the form, it stays where it was on the form.
		s.click(100, 74)
		s.drag(100, 74, 340, 214)
		s.expect(`#D.doc == 2 and #D.doc[1] == 0 and D.doc[2].name == "chkBold"`)
		s.expect(`D.doc[2].left == 280 and D.doc[2].top == 200`)

		// And back in.
		s.drag(340, 214, 100, 74)
		s.expect(`#D.doc == 1 and D.doc[1][1].name == "chkBold" and D.doc[1][1].left == 20 and D.doc[1][1].top == 40`)

		// Deleting the frame deletes what is in it; undo brings both back.
		s.click(30, 150)
		s.expect(`D.surface.sel.node.name == "fraOpts"`)
		in.Key(fltk.DELETE, "", 0)
		s.expect(`#D.doc == 0 and D.surface.host:find("chkBold") == nil`)
		s.lua(`D:undo()`)
		s.expect(`#D.doc == 1 and D.doc[1][1].name == "chkBold" and D.surface.host:find("chkBold") ~= nil`)

		// Pasting with the frame selected pastes into it.
		s.click(100, 74)
		mod, _ := cmdKey()
		in.Key('c', "c", mod)
		s.click(30, 150)
		in.Key('v', "v", mod)
		s.expect(`#D.doc[1] == 2 and D.doc[1][2].name == "CheckBox1"`)

		s.lua(`D:save()`)
		s.expect(`D.props.picker.items[3]:find("^   chkBold")`)
	})
}

func TestDesignerTabsPages(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.place("Tabs", 20, 20, 320, 220, "tabMain")
		s.expect(`#D.doc[1] == 2 and D.surface.entries[1].obj.selected == 1`)
		s.place("Label", 40, 70, 160, 98, "lblFirst") // on the first page
		s.expect(`D.doc[1][1][1].name == "lblFirst" and D.doc[1][1][1].left == 20 and D.doc[1][1][1].top == 25`)

		// A click on the tab row of the selected Tabs shows the next page;
		// the first page's label can no longer be clicked.
		s.click(150, 150)
		s.expect(`D.surface.sel.node.name == "tabMain"`)
		s.click(150, 30)
		s.expect(`D.surface.entries[1].obj.selected == 2`)
		s.click(80, 84)
		s.expect(`D.surface.sel.node.name == "tabMain"`)

		s.lua(`D.surface:addPage()`)
		s.expect(`#D.doc[1] == 3 and D.doc[1][3].kind == "Page" and D.surface.entries[1].obj.selected == 3`)
		s.lua(`D.surface:removePage()`)
		s.expect(`#D.doc[1] == 2`)
	})
}

func TestDesignerUsesControlsOfTheProject(t *testing.T) {
	if testing.Short() {
		t.Skip("builds tlua")
	}
	exe := filepath.Join(t.TempDir(), "tlua")
	if out, err := exec.Command("go", "build", "-o", exe, "tlua/cmd/tlua").CombinedOutput(); err != nil {
		t.Fatalf("building tlua: %v\n%s", err, out)
	}
	withDesigner(t, func(s *scene) {
		s.lua(`D.answerText = "Counter"; D:newControl()`)
		s.expect(`D.toolbox:rowOf("Counter") ~= nil`)
		s.place("Counter", 20, 20, 0, 0, "cntClicks") // a click: its own size
		s.expect(`D.doc[1].kind == "Counter" and D.surface.entries[1].obj.width == 120`)
		s.expect(`D.props:row("value") ~= nil`)
		s.lua(`before = D.surface.entries[1].obj; D:setProp(D.doc[1], "value", 7)`)
		s.expect(`D.doc[1].value == 7 and D.surface.entries[1].obj ~= before and D.surface.entries[1].obj.value == 7`)
		s.lua(`D:save()`)

		code := "local gui = require \"gui\"\nlocal frm = gui.load \"Form1\"\ngui.after(0.2, function() print(tostring(frm.cntClicks), frm.cntClicks.value); frm:close() end)\nreturn frm\n"
		os.WriteFile(filepath.Join(s.dir, "forms/Form1.lua"), []byte(code), 0o644)
		run := exec.Command(exe, "main.lua")
		run.Dir = s.dir
		out, err := run.CombinedOutput()
		if err != nil || strings.TrimSpace(string(out)) != "Counter\t7" {
			s.t.Errorf("running it: %v\n%s", err, out)
		}
	})
}

func TestDesignerWritesLanguageServerStubs(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.place("Button", 20, 20, 120, 50, "cmdGo")
		s.place("Frame", 20, 80, 220, 200, "fraBox")
		s.place("TextBox", 40, 120, 180, 148, "txtIn")
		s.lua(`D:save()`)
		stub, err := os.ReadFile(filepath.Join(s.dir, "forms/Form1.d.lua"))
		if err != nil {
			s.t.Fatal(err)
		}
		for _, want := range []string{"---@class forms.Form1: gui.Form", "---@field cmdGo gui.Button", "---@field fraBox gui.Frame", "---@field txtIn gui.TextBox"} {
			if !strings.Contains(string(stub), want) {
				s.t.Errorf("stub lacks %s:\n%s", want, stub)
			}
		}
		code, _ := os.ReadFile(filepath.Join(s.dir, "forms/Form1.lua"))
		if !strings.Contains(string(code), "--[[@as forms.Form1]]") {
			s.t.Errorf("the code does not say what frm is:\n%s", code)
		}
	})
}
