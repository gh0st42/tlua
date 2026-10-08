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
		if err := r.L.DoString(`bootgui(); D = require("design.main").start{dir = dir, create = true}`); err != nil {
			t.Errorf("start: %v", err)
			return
		}
		defer func() {
			s.L.DoString(`D.dirty = false; D.win:close()`)
			s.pump()
		}()
		s.pump()
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
		s.expect(`D.props.editors.caption.obj.text == "Button1"`)

		// Move it.
		s.drag(40, 40, 70, 70)
		s.expect(`D.doc[1].left == 50 and D.doc[1].top == 60`)
		s.expect(`D.props.editors.left.obj.text == "50"`)

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

		// Typing a caption changes the button as it is typed.
		s.lua(`D.props.editors.caption.obj.text = ""; D.props.editors.caption.obj:focus()`)
		in.Type("OK")
		s.expect(`D.doc[1].caption == "OK" and D.surface.entries[1].obj.caption == "OK"`)

		// A number that is not one is refused, and said so.
		s.lua(`D.props.editors.left.obj.text = ""; D.props.editors.left.obj:focus()`)
		in.Type("x")
		s.expect(`D.doc[1].left == 10 and D.statusLine.caption:find("left must be a number")`)

		// A name that is taken is refused by the control itself.
		s.lua(`D.surface:place("Label", 10, 60)`)
		s.lua(`D.surface:selectNode(D.doc[1])`)
		s.lua(`D.props.editors.name.obj.text = ""; D.props.editors.name.obj:focus()`)
		in.Type("Label1")
		s.expect(`D.doc[1].name ~= "Label1" and D.statusLine.caption:find("already has a control named Label1")`)
		in.Key(fltk.BACKSPACE, "", 0)
		in.Type("X")
		s.expect(`D.doc[1].name == "LabelX"`)

		// A property that picks the widget makes the control again.
		s.lua(`before = D.surface.entries[1].obj`)
		s.lua(`D.props.editors.default.obj:focus()`)
		ex, ey := s.at("D.props.editors.default.obj", 8, 10)
		in.Click(ex, ey)
		s.expect(`D.doc[1].default == true and D.surface.entries[1].obj ~= before`)
		s.expect(`D.surface.entries[1].obj.caption == "OK" and D.surface.entries[2].node.kind == "Label"`)

		// Bring to front and send to back reorder the layout.
		s.lua(`D.surface:toFront()`)
		s.expect(`D.doc[2].name == "LabelX"`)
	})
}

func TestDesignerSavesWhatItShows(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.pickTool("TextBox")
		s.drag(16, 16, 216, 44)
		s.lua(`D.props.editors.name.obj.text = ""; D.props.editors.name.obj:focus()`)
		in.Type("txtName")
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
					s.lua(fmt.Sprintf(`D.props.editors.%s.obj.text = ""; D.props.editors.%s.obj:focus()`, p, p))
					in.Type(v)
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
		s.lua(`D:openHandler(D.doc[1], "onClick"); D.codeBox.text = D.codeBox.text .. "frm.Button1.caption = 'x'\n"; D:showDesign()`)
		s.lua(`D.surface:selectNode(D.doc[1])`)
		s.lua(`D.props.editors.name.obj.text = ""; D.props.editors.name.obj:focus()`)
		in.Type("cmdGo")
		s.expect(`D.doc[1].name == "cmdGo"`)
		later(func() { in.Key(fltk.ENTER_KEY, "\r", 0) }) // Yes
		s.click(400, 280)                                 // selecting the form ends the renaming
		s.expect(`D.codeBox.text:find("function frm.cmdGo:onClick") and D.codeBox.text:find("frm.cmdGo.caption") and not D.codeBox.text:find("Button1")`)
	})
}

func TestDesignerFindsAndGoesToLines(t *testing.T) {
	withDesigner(t, func(s *scene) {
		s.lua(`D:showCode(); D.codeBox.cursor = 0; D.lastFind = "gui.load"; D:find(true)`)
		s.expect(`D.codeBox.selectedText == "gui.load"`)
		s.lua(`D.lastFind = "no such thing"; D:find(true)`)
		s.expect(`D.statusLine.caption:find("is not in the code")`)
		later(func() {
			in.Key(fltk.BACKSPACE, "", 0)
			in.Type("2")
			in.Key(fltk.ENTER_KEY, "\r", 0)
		})
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
