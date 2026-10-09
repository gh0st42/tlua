//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

// These tests put real windows on screen and drive them with synthetic
// input, so they only run when asked: TLUA_GUI_TESTS=1 go test ./internal/gui
// (or make test-gui). Typing elsewhere while they run can get in their way.

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/pwiecz/go-fltk"
	lua "github.com/yuin/gopher-lua"

	in "tlua/internal/gui/fltkinput"
)

var guiTests = os.Getenv("TLUA_GUI_TESTS") != ""

// mainCalls carries work to the main thread, which FLTK needs on macOS and
// which go test otherwise keeps for itself.
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

// scene is one test's form, on the main thread. Checks report with Errorf
// only: Fatal would end the main goroutine.
type scene struct {
	t *testing.T
	L *lua.LState
	a *app
}

// onScreen builds the form a script describes, shows it, and runs steps
// against it on the main thread.
func onScreen(t *testing.T, script string, steps func(s *scene)) {
	t.Helper()
	if !guiTests {
		t.Skip("set TLUA_GUI_TESTS=1 to run tests that open windows")
	}
	done := make(chan struct{})
	mainCalls <- func() {
		defer close(done)
		L := lua.NewState()
		defer L.Close()
		a := Open(L)
		a.booted = true // show() returns at once; the test runs the loop
		s := &scene{t: t, L: L, a: a}
		defer func() {
			for _, f := range a.forms {
				hideWindow(f)
			}
			s.pump()
		}()
		if err := L.DoString("gui = require('gui')\n" + script); err != nil {
			t.Errorf("script: %v", err)
			return
		}
		s.pump()
		steps(s)
		if a.err != nil {
			t.Errorf("a handler failed: %v", a.err)
		}
	}
	<-done
}

// pump lets FLTK catch up: drawing, timers set to 0, window changes.
func (s *scene) pump() {
	for i := 0; i < 5; i++ {
		fltk.Wait(0.01)
	}
}

func (s *scene) obj(name string) *guiObject {
	// "frm.txtName" is a control found by name on a global form.
	if form, control, ok := strings.Cut(name, "."); ok {
		if f, ok := toObject(s.L.GetGlobal(form)); ok {
			if c, ok := f.names[control]; ok {
				return c
			}
		}
	}
	o, ok := toObject(s.L.GetGlobal(name))
	if !ok {
		s.t.Errorf("%s is not a gui object", name)
	}
	return o
}

// at is a point inside an object, dx and dy from its top left, in the
// window's coordinates.
func (s *scene) at(name string, dx, dy int) (int, int) {
	g := s.obj(name).widget.(geometry)
	return g.X() + dx, g.Y() + dy
}

func (s *scene) middle(name string) (int, int) {
	g := s.obj(name).widget.(geometry)
	return g.X() + g.W()/2, g.Y() + g.H()/2
}

// expect checks that a Lua expression is true, after letting FLTK catch up.
func (s *scene) expect(expr string) {
	s.t.Helper()
	s.pump()
	if err := s.L.DoString("__ok = (" + expr + ")"); err != nil {
		s.t.Errorf("%s: %v", expr, err)
		return
	}
	if !lua.LVAsBool(s.L.GetGlobal("__ok")) {
		s.t.Errorf("not so: %s%s", expr, s.explain(expr))
	}
}

// explain shows the globals an expression mentions, to see why it failed.
func (s *scene) explain(expr string) string {
	var parts []string
	for _, word := range strings.FieldsFunc(expr, func(r rune) bool {
		return !(r == '_' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) {
		if word == "" || strings.ContainsAny(word[:1], "0123456789") || word == "and" || word == "or" || word == "not" || word == "nil" || word == "true" || word == "false" {
			continue
		}
		if err := s.L.DoString("__v = tostring(" + word + ")"); err == nil {
			parts = append(parts, fmt.Sprintf("%s=%s", word, lua.LVAsString(s.L.GetGlobal("__v"))))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func (s *scene) lua(src string) {
	s.t.Helper()
	if err := s.L.DoString(src); err != nil {
		s.t.Errorf("%s: %v", src, err)
	}
}

func (s *scene) focus(name string) {
	focus(s.obj(name))
	s.pump()
}

// later runs fn from the event loop once whatever starts next is waiting
// in its own loop: a dialog, say.
func later(fn func()) { fltk.AddTimeout(0.2, fn) }

func TestClickingControls(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 400, height = 300}
clicks = 0
button = form:Button{caption = "Press", left = 10, top = 10, onClick = function() clicks = clicks + 1 end}
check = form:CheckBox{caption = "Tick", left = 10, top = 50, onChange = function(self) ticked = self.checked end}
one = form:RadioButton{caption = "One", left = 10, top = 90, checked = true}
two = form:RadioButton{caption = "Two", left = 10, top = 120, onChange = function(self) picked = self.caption end}
disabled = form:Button{caption = "Off", left = 200, top = 10, enabled = false, onClick = function() offClicked = true end}
form:show()`, func(s *scene) {
		x, y := s.middle("button")
		in.Click(x, y)
		in.Click(x, y)
		s.expect(`clicks == 2`)

		in.Click(s.at("check", 8, 14))
		s.expect(`ticked == true and check.checked`)
		in.Click(s.at("check", 8, 14))
		s.expect(`ticked == false`)

		in.Click(s.at("two", 8, 14))
		s.expect(`picked == "Two" and two.checked and not one.checked`)

		in.Click(s.middle("disabled"))
		s.expect(`offClicked == nil`)
	})
}

func TestTyping(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 400, height = 300}
changes = 0
name = form:TextBox{left = 10, top = 10, width = 200, onChange = function() changes = changes + 1 end}
secret = form:TextBox{left = 10, top = 50, width = 200, password = true}
notes = form:TextBox{left = 10, top = 90, width = 300, height = 100, multiLine = true}
fixed = form:TextBox{left = 10, top = 200, width = 200, readOnly = true, text = "fixed"}
form:show()`, func(s *scene) {
		s.focus("name")
		in.Type("Ada")
		s.expect(`name.text == "Ada" and changes == 3`)
		in.Key(fltk.BACKSPACE, "", 0)
		s.expect(`name.text == "Ad" and changes == 4`)

		s.focus("secret")
		in.Type("pw")
		s.expect(`secret.text == "pw"`)

		s.focus("notes")
		in.Type("one")
		in.Key(fltk.ENTER_KEY, "\n", 0)
		in.Type("two")
		s.expect(`notes.text == "one\ntwo"`)

		s.focus("fixed")
		in.Type("x")
		s.expect(`fixed.text == "fixed"`)
	})
}

// cmdKey is the modifier "Cmd" stands for in shortcuts.
func cmdKey() (int, string) {
	if goruntime.GOOS == "darwin" {
		return fltk.META, "Cmd"
	}
	return fltk.CTRL, "Ctrl"
}

func TestFormKeysAndClosing(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
form:Button{caption = "x", left = 10, top = 10}
function form:onKey(key, text) lastKey = key; if key == "F6" then return true end end
locked = true
function form:onClose() return not locked end
function form:onUnload() unloaded = true end
form:show()`, func(s *scene) {
		in.Key(fltk.F5, "", 0)
		s.expect(`lastKey == "F5"`)
		mod, name := cmdKey()
		in.Key('s', "s", mod)
		s.expect(fmt.Sprintf(`lastKey == "%s+s"`, name))

		in.Key(fltk.ESCAPE, "", 0)
		s.expect(`form.visible and lastKey == "Escape" and not unloaded`)

		// Escape is handled, so it does not close the form.
		s.lua(`locked = false; function form:onKey(key) lastKey = key; return key == "Escape" end`)
		in.Key(fltk.ESCAPE, "", 0)
		s.expect(`form.visible and not unloaded`)

		s.lua(`form.onKey = nil`)
		in.Key(fltk.ESCAPE, "", 0)
		s.expect(`not form.visible and unloaded`)
	})
}

func TestMenuShortcuts(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
form:Menu{
  {"&File", {
    {"&Open", function(caption) opened = caption end, shortcut = "Cmd+O"},
    "-",
    {"&Wrap", function(caption, on) wrap = on end, shortcut = "F7", checked = true},
    {"&Never", function() never = true end, shortcut = "F8", enabled = false},
  }},
}
form:show()`, func(s *scene) {
		mod, _ := cmdKey()
		in.Key('o', "o", mod)
		s.expect(`opened == "&Open"`)
		in.Key(fltk.F7, "", 0)
		s.expect(`wrap == false`)
		in.Key(fltk.F7, "", 0)
		s.expect(`wrap == true`)
		in.Key(fltk.F8, "", 0)
		s.expect(`never == nil`)
	})
}

func TestLists(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 500, height = 300}
list = form:ListBox{left = 10, top = 10, width = 150, height = 150, items = {"apple", "banana", "cherry"},
  onChange = function(self) changed = self.text end,
  onDoubleClick = function(self) opened = self.text end}
items = {{"src", {"main.lua"}}, "README"}
tree = form:Tree{left = 170, top = 10, width = 150, height = 150, items = items,
  onChange = function(self) treeAt = self.path end,
  onToggle = function(self, path, open) toggled = path .. "=" .. tostring(open) end}
grid = form:Table{left = 330, top = 10, width = 160, height = 150,
  columns = {"Name"}, rows = {{"Ada"}, {"Alan"}},
  onChange = function(self) row = self.selected end,
  onDoubleClick = function(self) doubled = self.selected end}
form:show()`, func(s *scene) {
		// The first line of a list is just inside its top.
		in.Click(s.at("list", 30, 10))
		s.expect(`changed == "apple" and list.selected == 1`)
		in.Click(s.at("list", 30, 10))
		in.DoubleClick(s.at("list", 30, 10))
		s.expect(`opened == "apple"`)
		in.Key(fltk.DOWN, "", 0)
		s.expect(`changed == "banana"`)

		// A click on a branch's marker opens it; a click on its name selects.
		in.Click(s.at("tree", 8, 10))
		s.expect(`toggled == "src=true" and items[1].open == true and tree.path == "src"`)
		s.focus("tree")
		in.Key(fltk.LEFT, "", 0)
		s.expect(`toggled == "src=false" and items[1].open == false`)
		in.Key(fltk.RIGHT, "", 0)
		s.expect(`toggled == "src=true"`)
		in.Key(fltk.DOWN, "", 0)
		s.expect(`treeAt == "src/main.lua"`)

		// Rows start below the 24-pixel header and are 22 high.
		in.Click(s.at("grid", 40, 24+22+11))
		s.expect(`row == 2 and grid.selected == 2`)
		in.DoubleClick(s.at("grid", 40, 24+11))
		s.expect(`doubled == 1`)
	})
}

func TestTabsByClicking(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
tabs = form:Tabs{left = 10, top = 10, width = 280, height = 180, onChange = function(self) now = self.selected end}
tabs:Page{caption = "One"}
tabs:Page{caption = "Two"}
form:show()`, func(s *scene) {
		// A tab is as wide as its caption and a little; the second starts
		// where the first ends.
		fltk.SetDrawFont(fltk.HELVETICA, 14)
		w, _ := fltk.MeasureText("One", false)
		in.Click(s.at("tabs", w+30, 12))
		s.expect(`now == 2 and tabs.selected == 2`)
		in.Click(s.at("tabs", 8, 12))
		s.expect(`now == 1 and tabs.selected == 1`)
	})
}

func TestCanvasMouse(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 300}
canvas = form:Canvas{left = 20, top = 30, width = 200, height = 200}
drags = 0
function canvas:onMouseDown(x, y, b) down = string.format("%d,%d,%d", x, y, b) end
function canvas:onMouseDrag(x, y) drags = drags + 1; dragged = string.format("%d,%d", x, y) end
function canvas:onMouseUp(x, y, b) up = string.format("%d,%d,%d", x, y, b) end
function canvas:onMouseMove(x, y) moved = string.format("%d,%d", x, y) end
function canvas:onMouseWheel(dx, dy) wheel = string.format("%d,%d", dx, dy) end
function canvas:onDraw(g) draws = (draws or 0) + 1 end
form:show()`, func(s *scene) {
		x, y := s.at("canvas", 10, 10)
		in.Drag(x, y, x+40, y+20, 4)
		s.expect(`down == "10,10,1" and drags == 4 and dragged == "50,30" and up == "50,30,1"`)
		in.Send(in.Event{Type: fltk.MOVE, X: x + 5, Y: y + 6})
		s.expect(`moved == "15,16"`)
		in.Wheel(x, y, 0, 3)
		s.expect(`wheel == "0,3"`)
		s.expect(`draws >= 1`)
	})
}

func TestDragAndDrop(t *testing.T) {
	var started []string
	saved := startDrag
	startDrag = func() { started = append(started, "drag") }
	defer func() { startDrag = saved }()

	onScreen(t, `
form = gui.Form{width = 300, height = 200}
zone = form:Label{caption = "drop here", left = 10, top = 10, width = 120, height = 60}
function zone:onDrop(text, lines) dropped = text; count = #lines; first = lines[1] end
plain = form:Label{caption = "no drops", left = 150, top = 10, width = 120, height = 60}
source = form:Label{caption = "drag me", left = 10, top = 100, width = 120, height = 40,
  onDrag = function() dragging = true; return "carried" end}
form:show()`, func(s *scene) {
		if !in.Drop(s.middle("zone")) {
			s.t.Error("the zone did not take the drop")
		}
		// FLTK hands the dropped text to the widget as a paste.
		in.PasteText("/tmp/a.txt\n/tmp/b.txt\n")
		handle(s.obj("zone"), fltk.PASTE)
		s.expect(`dropped == "/tmp/a.txt\n/tmp/b.txt\n" and count == 2 and first == "/tmp/a.txt"`)

		if in.Drop(s.middle("plain")) {
			s.t.Error("a drop on a label without onDrop was taken (by the zone, before the fix)")
		}
		if s.obj("zone").mouse.dropping {
			s.t.Error("the zone took a drop beside it")
		}
		// With an onDrop of its own, the form takes what lands elsewhere.
		s.lua(`function form:onDrop(text) formGot = text end`)
		if !in.Drop(s.middle("plain")) {
			s.t.Error("the form did not take a drop beside its controls")
		}
		// The paste goes to the form's catcher, which hands it on.
		in.PasteText("loose")
		dropEvent(s.obj("form"), fltk.PASTE)
		s.expect(`formGot == "loose"`)

		x, y := s.middle("source")
		in.Drag(x, y, x+2, y+2, 2) // not far enough to count as a drag
		s.expect(`dragging == nil`)
		in.Drag(x, y, x+30, y, 3)
		s.expect(`dragging == true`)
		if len(started) != 1 {
			s.t.Errorf("drag sessions started: %d", len(started))
		}
	})
}

func TestDialogsByKeyboard(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 200, height = 100}
form:show()`, func(s *scene) {
		later(func() { in.Key(fltk.ENTER_KEY, "\r", 0) })
		s.lua(`answer = gui.msgbox("Sure?", "yesno")`)
		s.expect(`answer == "yes"`)

		later(func() { in.Key(fltk.ESCAPE, "", 0) })
		s.lua(`answer = gui.msgbox("Sure?", "yesnocancel")`)
		s.expect(`answer == "cancel"`)

		// Tab to the second button, and Space presses it.
		later(func() {
			in.Key(fltk.TAB, "\t", 0)
			in.Key(' ', " ", 0)
		})
		s.lua(`answer = gui.msgbox("Sure?", "yesnocancel")`)
		s.expect(`answer == "no"`)

		later(func() {
			in.Type("Ada")
			in.Key(fltk.ENTER_KEY, "\r", 0)
		})
		s.lua(`answer = gui.inputbox("Name?")`)
		s.expect(`answer == "Ada"`)

		later(func() { in.Key(fltk.ENTER_KEY, "\r", 0) })
		s.lua(`answer = gui.inputbox("Name?", "Input", "kept")`)
		s.expect(`answer == "kept"`)

		later(func() { in.Key(fltk.ESCAPE, "", 0) })
		s.lua(`answer = gui.inputbox("Name?", "Input", "x")`)
		s.expect(`answer == nil`)

		// A dialog asked for from a handler, inside the form's own loop.
		s.lua(`button = form:Button{caption = "ask", left = 10, top = 10,
		  onClick = function() asked = gui.inputbox("From a handler?") end}`)
		s.pump()
		later(func() {
			in.Type("yes")
			in.Key(fltk.ENTER_KEY, "\r", 0)
		})
		in.Click(s.middle("button"))
		s.expect(`asked == "yes"`)
	})
}

func TestModalForm(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 200, height = 100}
form:show()
function ask()
  local dlg = gui.Form{caption = "modal", width = 200, height = 100}
  box = dlg:TextBox{left = 10, top = 10, width = 180}
  dlg:Button{caption = "OK", default = true, left = 10, top = 50, onClick = function() result = box.text; dlg:close() end}
  dlg:showModal()
  return result
end`, func(s *scene) {
		later(func() {
			s.lua(`box:focus()`)
			in.Type("hi")
			in.Key(fltk.ENTER_KEY, "\r", 0)
		})
		s.lua(`got = ask()`)
		s.expect(`got == "hi"`)
	})
}

func TestDefinedControlByMouse(t *testing.T) {
	onScreen(t, ratingDef+`
gui.define{
  name = "Stars",
  events = {"onChange"},
  build = function(parent, opts)
    local c = parent:Canvas{width = 100, height = 20}
    c.value, c.hover = opts.value or 0, 0
    function c:onDraw(g) draws = (draws or 0) + 1 end
    function c:onMouseMove(x) self.hover = math.floor(x / 20) + 1 end
    function c:onMouseLeave() self.hover = 0 end
    function c:onMouseDown(x) self.value = math.floor(x / 20) + 1; self:fire("onChange", self.value) end
    return c
  end,
}
form = gui.Form{width = 300, height = 100}
stars = form:Stars{left = 10, top = 10, onChange = function(self, n) rated = n end}
form:show()`, func(s *scene) {
		in.Click(s.at("stars", 70, 10))
		s.expect(`rated == 4 and stars.value == 4`)
		x, y := s.at("stars", 25, 10)
		in.Send(in.Event{Type: fltk.MOVE, X: x, Y: y})
		s.expect(`stars.hover == 2`)
		in.Send(in.Event{Type: fltk.MOVE, X: x, Y: y + 50}) // off it
		s.expect(`stars.hover == 0`)
		s.lua(`before = draws; stars.value = 1`)
		s.expect(`draws > before`) // assigning a property redraws it
	})
}

func TestClosedFormsAreFreed(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
name = form:TextBox{left = 10, top = 10, width = 150}
check = form:CheckBox{caption = "c", left = 10, top = 50}
canvas = form:Canvas{left = 170, top = 10, width = 100, height = 60, onDraw = function(self, g) g:fill(0, 0, 10, 10) end}
form:show()`, func(s *scene) {
		form := s.obj("form")
		s.focus("name")
		in.Type("Ada")
		in.Click(s.at("check", 8, 14))
		s.lua(`form:close()`)
		s.pump()
		if form.widget != nil || s.obj("name").widget != nil || s.obj("canvas").widget != nil {
			s.t.Error("a closed form should have let go of its widgets")
		}
		if len(s.a.forms) != 0 {
			s.t.Errorf("forms still kept: %d", len(s.a.forms))
		}
		s.expect(`name.text == "Ada" and check.checked == true and not form.visible`)

		s.lua(`form:show()`)
		s.pump()
		if form.widget == nil || len(s.a.forms) != 1 {
			s.t.Error("showing it again should build it again")
		}
		s.expect(`form.visible and name.text == "Ada" and check.checked == true`)
		s.focus("name")
		in.Type("!")
		s.expect(`name.text == "Ada!"`)
	})
}

func TestModalFormsDoNotPileUp(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 200, height = 100}
form:show()
function ask()
  local dlg = gui.Form{caption = "modal", width = 150, height = 80}
  dlg:Button{caption = "OK", default = true, left = 10, top = 10, onClick = function() dlg:close() end}
  dlg:showModal()
end`, func(s *scene) {
		for i := 0; i < 3; i++ {
			later(func() { in.Key(fltk.ENTER_KEY, "\r", 0) })
			s.lua(`ask()`)
			s.pump()
		}
		if len(s.a.forms) != 1 {
			s.t.Errorf("forms kept after three modal forms: %d, want 1", len(s.a.forms))
		}
	})
}

func TestReplacingAnImageFreesTheOldOne(t *testing.T) {
	pic, _ := filepath.Abs("../../examples/pico/cellar/gfx/tiles.png")
	onScreen(t, fmt.Sprintf(`
form = gui.Form{width = 300, height = 200}
pic = form:Image{file = %q, left = 10, top = 10, width = 100, height = 40}
form:show()`, pic), func(s *scene) {
		o := s.obj("pic")
		first, ok := o.state.(scalable)
		if !ok {
			s.t.Fatal("the Image should keep its picture")
		}
		s.lua(fmt.Sprintf(`pic.fit = true; pic.file = %q`, pic))
		second, _ := o.state.(scalable)
		if second == nil || second == first {
			s.t.Error("a new file should give a new picture")
		}
		s.lua(`pic.file = ""`)
		if o.state != nil {
			s.t.Error("an Image with no file holds no picture of its own")
		}
		s.lua(fmt.Sprintf(`pic.file = %q`, pic))
		s.pump()
	})
}

func TestLoadedFormsWorkAndDumpWhatIsOnScreen(t *testing.T) {
	onScreen(t, `
frm = gui.load{ kind = "Form", name = "Main", width = 300, height = 150,
  { kind = "TextBox", name = "txtName", left = 10, top = 10, width = 150 },
  { kind = "Button", name = "cmdGo", caption = "Go", left = 10, top = 50 },
}
function frm.cmdGo:onClick() went = frm.txtName.text end
frm:show()`, func(s *scene) {
		s.focus("frm.txtName")
		in.Type("Ada")
		in.Click(s.middle("frm.cmdGo"))
		s.expect(`went == "Ada"`)
		s.expect(`gui.dump(frm)[1].text == "Ada"`)
	})
}

func TestRemovingAndStackingOnScreen(t *testing.T) {
	pic, _ := filepath.Abs("../../examples/pico/cellar/gfx/tiles.png")
	onScreen(t, fmt.Sprintf(`
form = gui.Form{width = 300, height = 200}
under = form:Button{caption = "under", left = 10, top = 10, width = 100, onClick = function() hit = "under" end}
over = form:Button{caption = "over", left = 10, top = 10, width = 100, onClick = function() hit = "over" end}
icon = form:Button{caption = "icon", image = %q, left = 150, top = 10, width = 120, height = 40}
form:show()`, pic), func(s *scene) {
		in.Click(s.at("over", 20, 10))
		s.expect(`hit == "over"`)
		s.lua(`under:raise()`)
		in.Click(s.at("over", 20, 10))
		s.expect(`hit == "under"`)
		s.lua(`under:remove()`)
		s.pump()
		if s.obj("under").widget != nil {
			s.t.Error("a removed control keeps no widget")
		}
		in.Click(s.at("over", 20, 10))
		s.expect(`hit == "over"`)
		s.lua(`form:add(under); under:raise()`)
		s.pump()
		in.Click(s.at("under", 20, 10))
		s.expect(`hit == "under" and under.caption == "under"`)
		if _, ok := s.obj("icon").state.(scalable); !ok {
			s.t.Error("a Button's image is its own")
		}
	})
}

func TestTransparentOverlayTakesTheMouse(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
button = form:Button{caption = "b", left = 10, top = 10, onClick = function() clicked = true end}
overlay = form:Canvas{transparent = true, left = 0, top = 0, width = 300, height = 200,
  onMouseDown = function(self, x, y) down = x .. "," .. y end,
  onDraw = function(self, g) draws = (draws or 0) + 1; g:rect(5, 5, 20, 20) end}
form:show()`, func(s *scene) {
		in.Click(s.at("button", 10, 10))
		s.expect(`down == "20,20" and clicked == nil`)
		s.lua(`before = draws; overlay:redraw()`)
		s.expect(`draws > before`)
	})
}

func TestScrollCoordinates(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
scroll = form:Scroll{left = 10, top = 10, width = 200, height = 100}
far = scroll:Label{caption = "far", left = 20, top = 300}
form:show()`, func(s *scene) {
		s.expect(`far.left == 20 and far.top == 300`)
		s.obj("scroll").widget.(*fltk.Scroll).ScrollTo(0, 250)
		s.pump()
		s.expect(`far.left == 20 and far.top == 300`) // measured from the content, not the view
	})
}

func TestSpawn(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("uses /bin/sh")
	}
	onScreen(t, `
form = gui.Form{width = 100, height = 50}
form:show()
out, errs = {}, {}
-- Under go test the interpreter is the test binary, so a shell stands in.
proc = gui.spawn{"/bin/sh", "-c", "echo one; echo two; echo oops >&2; exit 3",
  onOutput = function(line, stream) if stream == "stdout" then out[#out + 1] = line else errs[#errs + 1] = line end end,
  onExit = function(code) exited = code end}`, func(s *scene) {
		for i := 0; i < 100 && lua.LVAsBool(s.L.GetGlobal("exited")) == false && s.L.GetGlobal("exited") == lua.LNil; i++ {
			fltk.Wait(0.05)
		}
		s.expect(`exited == 3 and table.concat(out, ",") == "one,two" and errs[1] == "oops" and not proc.running()`)
	})
}

func TestCanvasKeys(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
canvas = form:Canvas{left = 10, top = 10, width = 100, height = 100,
  onKey = function(self, key) last = key; return key == "Delete" end}
form:show()`, func(s *scene) {
		in.Click(s.at("canvas", 10, 10))
		in.Key(fltk.DELETE, "", 0)
		s.expect(`last == "Delete" and canvas.parent == form`)
		in.Key(fltk.LEFT, "", 0)
		s.expect(`last == "Left"`)
	})
}

func TestCodeTextBox(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 400, height = 300}
code = form:TextBox{multiLine = true, lineNumbers = true, syntax = "lua", acceptsTab = true,
  left = 10, top = 10, width = 380, height = 200, text = "local x = 1\nfunction f()\nend",
  onChange = function() changes = (changes or 0) + 1 end}
plain = form:TextBox{left = 10, top = 220, width = 200, text = "hello world"}
form:Menu{ {"&View", { {"&Wrap", function() end, shortcut = "F7", checked = false} }} }
form:show()`, func(s *scene) {
		s.expect(`code.line == 1 and code.cursor == 0`)
		s.lua(`code.line = 2`)
		s.expect(`code.cursor == 12 and code.line == 2`)
		s.lua(`code.cursor = 24`) // the end of "function f()"
		s.focus("code")
		in.Key(fltk.ENTER_KEY, "\r", 0)
		in.Key(fltk.TAB, "\t", 0)
		in.Type("return 1")
		s.expect(`code.text == "local x = 1\nfunction f()\n  return 1\nend" and changes >= 3`)
		s.lua(`code:select(7, 7)`)
		s.expect(`code.selectedText == "x" and code.cursor == 7`)
		s.lua(`plain:select(7, 11)`)
		s.expect(`plain.selectedText == "world"`)
		in.Key(fltk.F7, "", 0)
		s.expect(`gui.dump(form)[3].kind == "Menu" and gui.dump(form)[3].items[1][2][1].checked == true`)
	})
}

// TestComboBoxesWithNothingSelected guards against go-fltk's FLTK, whose
// Fl_Menu_::value(int) does not check its index: a ComboBox with nothing
// selected was handed -1, FLTK then drew from before its items, and enough
// ComboBoxes being filled and freed made that a crash.
func TestComboBoxesWithNothingSelected(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 300}
empty = form:ComboBox{left = 200, top = 0, width = 90}
holder = form:Scroll{left = 0, top = 0, width = 190, height = 300}
rows = {}
function churn(n)
  empty.items = {"x", "y"}
  empty.selected = 0
  for _, r in ipairs(rows) do r:remove() end
  rows = {}
  for i = 1, n do
    rows[#rows + 1] = holder:ComboBox{left = 10, top = i * 30, width = 150, items = {"", "a", "b"}, selected = i % 3}
  end
end
form:show()`, func(s *scene) {
		for i := 0; i < 60; i++ {
			s.lua(`churn(3)`)
			s.pump()
		}
		s.expect(`empty.selected == 0 and empty.text == "" and rows[1].selected == 1 and rows[3].selected == 0`)
		s.lua(`empty.selected = 2`)
		s.expect(`empty.text == "y"`)
		s.lua(`empty.selected = 9`)
		s.expect(`empty.selected == 0`)
	})
}

func TestPhase4Runtime(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 400, height = 300}
canvas = form:Canvas{left = 10, top = 40, width = 100, height = 60,
  onMouseDown = function(self, x, y, b, double, mods) held = mods end}
a = form:TextBox{left = 150, top = 40, width = 100, tabIndex = 3}
b = form:TextBox{left = 150, top = 80, width = 100, tabIndex = 1}
c = form:TextBox{left = 150, top = 120, width = 100}
d = form:Button{caption = "d", left = 150, top = 160, tabIndex = 2}
panel = form:Panel{left = 260, top = 40, width = 130, height = 100}
menu = panel:Menu{ {"&File", { {"&Open", name = "mnuOpen", shortcut = "F6"} }} }
function menu:onClick(name, caption) picked = name .. ":" .. caption end
form:show()`, func(s *scene) {
		x, y := s.at("canvas", 10, 10)
		in.ClickWith(x, y, 0)
		s.expect(`held == ""`)
		in.ClickWith(x, y, fltk.SHIFT)
		s.expect(`held == "Shift"`)
		in.ClickWith(x, y, fltk.SHIFT|fltk.CTRL)
		s.expect(`held == "Ctrl+Shift"`)

		// Tab goes b (1), d (2), a (3), then c, which has none, then round.
		s.focus("b")
		order := ""
		for i := 0; i < 5; i++ {
			in.Key(fltk.TAB, "\t", 0)
			s.pump()
			for _, n := range []string{"a", "b", "c", "d"} {
				if w, ok := s.obj(n).widget.(interface{ HasFocus() bool }); ok && w.HasFocus() {
					order += n
				}
			}
		}
		if order != "dacbd" {
			s.t.Errorf("tab order: %s, want dacbd", order)
		}
		in.Send(in.Event{Type: fltk.KEY, Key: fltk.TAB, Text: "\t", State: fltk.SHIFT})
		s.pump()
		if w := s.obj("b").widget.(interface{ HasFocus() bool }); !w.HasFocus() {
			s.t.Error("Shift-Tab should go back to b")
		}

		in.Key(fltk.F6, "", 0)
		s.expect(`picked == "mnuOpen:&Open"`)
		s.expect(`menu.width == 130`)
	})
}
