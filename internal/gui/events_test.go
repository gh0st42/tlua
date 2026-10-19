//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

// Tests of the events that follow the window and its controls: load, show,
// hide, activate, focus, resize, paint, and the mouse over a Label. Like
// input_test.go's they open real windows, so they run when asked
// (TLUA_GUI_TESTS=1).

import (
	"testing"

	"github.com/pwiecz/go-fltk"

	in "tlua/internal/gui/fltkinput"
)

// onLoad comes once the program has finished setting the form up, whatever
// order it wrote its handlers in; onShow follows it, and again whenever the
// form is shown after being hidden, which is not a second load. Closing a
// form unloads it, and showing it again loads it again.
func TestFormLoadShowHide(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
log = {}
function form:onClose() log[#log + 1] = "close" end
function form:onUnload() log[#log + 1] = "unload" end
form:show()
-- after show(), as a bootgui program writes them
function form:onLoad() log[#log + 1] = "load" end
function form:onShow() log[#log + 1] = "show" end
function form:onHide() log[#log + 1] = "hide" end
logged = function() return table.concat(log, " ") end`, func(s *scene) {
		s.expect(`logged() == "load show"`)

		s.lua(`log = {}; form.visible = false`)
		s.expect(`logged() == "hide"`)
		s.lua(`log = {}; form.visible = true`)
		s.expect(`logged() == "show"`)

		// Closing it: the questions, then it goes.
		s.lua(`log = {}; form:close()`)
		s.expect(`logged() == "close unload hide"`)
		s.pump()

		// Shown again it is built again, and loaded again.
		s.lua(`log = {}; form:show()`)
		s.expect(`logged() == "load show"`)
	})
}

// Nothing on a form hears of anything before its onLoad has finished, and
// what it missed is told right after, in order: the form and its controls
// were shown, and the one with the keyboard was given it.
func TestNothingBeforeLoad(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
log = {}
local function note(what) return function(self) log[#log + 1] = what end end
box = form:TextBox{left = 10, top = 10, width = 100, onShow = note("box show"), onFocus = note("box focus"), onResize = note("box resize")}
tabs = form:Tabs{left = 10, top = 50, width = 200, height = 100}
one = tabs:Page{caption = "One", onShow = note("one show"), onHide = note("one hide")}
two = tabs:Page{caption = "Two", onShow = note("two show"), onHide = note("two hide")}
form.onLoad = function() log[#log + 1] = "load"; box.width = 130 end
form.onShow = note("form show")
form.onActivate = note("form activate")
form:show()
logged = function() return table.concat(log, " ") end`, func(s *scene) {
		s.pump()
		// Wherever the system put the keyboard, the form comes first; the
		// page that is not showing is not shown, and the box's width, which
		// onLoad changed, is the first it hears of.
		s.expect(`logged():find("^load form show") and logged():find("one show") and not logged():find("two")`)
		s.expect(`logged():find("box show") and logged():find("box resize")`)
		s.expect(`not logged():find("hide")`)
	})
}

// A form's onResize is for its size changing, not for it moving or for the
// system settling it into place.
func TestFormResizeIsSize(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200, resizable = true}
sizes = {}
function form:onResize() sizes[#sizes + 1] = self.width .. "x" .. self.height end
form:show()`, func(s *scene) {
		s.pump()
		s.expect(`#sizes == 0`)
		s.lua(`form.left = 120; form.top = 90`)
		s.expect(`#sizes == 0`)
		s.lua(`form.width = 350`)
		s.expect(`#sizes == 1 and sizes[1] == "350x200"`)
		s.lua(`form.width = 350; form.height = 260`)
		s.expect(`#sizes == 2 and sizes[2] == "350x260"`)
	})
}

// A form that is closed before the event loop gets to it is not loaded.
func TestFormClosedBeforeLoad(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
loads = 0
function form:onLoad() loads = loads + 1 end
form:show()
form:close()`, func(s *scene) {
		s.pump()
		s.expect(`loads == 0`)
	})
}

// onLoad can set up the controls, and can close the form.
func TestOnLoadSetsUp(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
list = form:ListBox{left = 10, top = 10, width = 100, height = 100}
box = form:TextBox{left = 120, top = 10, width = 100}
function form:onLoad()
  list.items = {"one", "two", "three"}
  box.text = "loaded"
end
form:show()`, func(s *scene) {
		s.expect(`#list.items == 3 and box.text == "loaded"`)
	})
}

// A form hears that it has the keyboard, and that it lost it to another
// window, but not that it moved from one of its controls to another.
func TestFormActivation(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
one = form:TextBox{left = 10, top = 10, width = 100}
two = form:TextBox{left = 10, top = 50, width = 100}
acts, deacts = 0, 0
function form:onActivate() acts = acts + 1 end
function form:onDeactivate() deacts = deacts + 1 end
form:show()`, func(s *scene) {
		// Whether the system gave the window the keyboard when it was shown
		// is its business: start from the window having just got it.
		s.lua(`acts, deacts = 0, 0`)
		in.Send(in.Event{Type: fltk.UNFOCUS})
		s.pump()
		s.lua(`acts, deacts = 0, 0`)
		in.Send(in.Event{Type: fltk.FOCUS})
		s.expect(`acts == 1 and deacts == 0`)

		s.focus("one")
		s.focus("two")
		s.focus("one")
		s.expect(`acts == 1 and deacts == 0`)

		in.Send(in.Event{Type: fltk.UNFOCUS})
		s.expect(`acts == 1 and deacts == 1`)
		in.Send(in.Event{Type: fltk.FOCUS})
		s.expect(`acts == 2 and deacts == 1`)
	})
}

// Controls hear that they were given the keyboard and that it was taken, once
// each time, however they got it. A window that loses the keyboard takes it
// from the control that had it.
func TestFocusEvents(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 400, height = 300}
log = {}
local function note(what) return function(self) log[#log + 1] = what .. ":" .. self.name end end
button = form:Button{name = "button", caption = "B", left = 10, top = 10, onFocus = note("focus"), onBlur = note("blur")}
box = form:TextBox{name = "box", left = 10, top = 50, width = 100, onFocus = note("focus"), onBlur = note("blur")}
plain = form:Canvas{name = "plain", left = 10, top = 100, width = 50, height = 50, onFocus = note("focus"), onBlur = note("blur")}
keyed = form:Canvas{name = "keyed", left = 70, top = 100, width = 50, height = 50, onKey = function() end,
  onFocus = note("focus"), onBlur = note("blur")}
form:show()
logged = function() return table.concat(log, " ") end`, func(s *scene) {
		s.focus("button")
		s.expect(`logged() == "focus:button"`)
		// Clicking what already has the keyboard is not getting it again.
		x, y := s.middle("button")
		in.Click(x, y)
		s.expect(`logged() == "focus:button"`)

		s.focus("box")
		s.expect(`logged() == "focus:button blur:button focus:box"`)

		// A Canvas that listens for no keys cannot have the keyboard.
		s.lua(`log = {}`)
		s.focus("plain")
		s.expect(`logged() == ""`)
		s.focus("keyed")
		s.expect(`logged() == "blur:box focus:keyed"`)

		// The window losing the keyboard takes it from the control.
		s.lua(`log = {}`)
		in.Send(in.Event{Type: fltk.UNFOCUS})
		s.expect(`logged() == "blur:keyed"`)
		in.Send(in.Event{Type: fltk.FOCUS})
		s.expect(`logged() == "blur:keyed focus:keyed"`)
	})
}

// A control hears when its size changes, once, with the new size already
// its own; not when it only moves, and not for a change its own handler
// makes to it.
func TestResizeEvent(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 400, height = 300}
panel = form:Panel{left = 10, top = 10, width = 100, height = 80}
box = form:TextBox{left = 10, top = 120, width = 100}
canvas = form:Canvas{left = 150, top = 10, width = 50, height = 50, onDraw = function() end}
sizes = {}
function panel:onResize() sizes[#sizes + 1] = self.width .. "x" .. self.height end
function box:onResize() box.width = 200 end -- its own change is not heard again
canvasResizes = 0
function canvas:onResize() canvasResizes = canvasResizes + 1 end
form:show()`, func(s *scene) {
		s.expect(`#sizes == 0`)
		s.lua(`panel.width = 150`)
		s.expect(`#sizes == 1 and sizes[1] == "150x80"`)
		s.lua(`panel.left = 40; panel.width = 150`)
		s.expect(`#sizes == 1`)
		s.lua(`panel.height = 60`)
		s.expect(`#sizes == 2 and sizes[2] == "150x60"`)

		s.lua(`box.width = 150`)
		s.expect(`box.width == 200`)

		s.lua(`canvas.width = 60`)
		s.expect(`canvasResizes == 1`)

		// A handler set after the control is on screen hears from then on.
		s.lua(`late = form:Frame{left = 250, top = 10, width = 60, height = 60}
lateSizes = 0
function late:onResize() lateSizes = lateSizes + 1 end
late.height = 90`)
		s.expect(`lateSizes == 1`)
		s.lua(`late.onResize = nil; late.height = 70`)
		s.expect(`lateSizes == 1`)
	})
}

// onPaint is called each time a control is drawn, with a g that draws inside
// it as a Canvas's onDraw does, and only then.
func TestPaintOverlay(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 400, height = 300}
panel = form:Panel{left = 10, top = 10, width = 120, height = 80}
inner = panel:Button{caption = "inside", left = 10, top = 10, width = 80, height = 30}
frame = form:Frame{caption = "F", left = 150, top = 10, width = 100, height = 80}
box = form:TextBox{left = 10, top = 120, width = 100}
label = form:Label{caption = "L", left = 150, top = 120, width = 60, height = 20}
paints = {}
local function paint(name)
  return function(self, g)
    local w, h = g:size()
    paints[name] = (paints[name] or 0) + 1
    sizes = sizes or {}
    sizes[name] = w .. "x" .. h
    g:color("#ff0000")
    g:rect(0, 0, w, h)
    g:line(0, 0, w, h)
    g:text("x", 2, 2)
    leaked = g
  end
end
panel.onPaint = paint("panel")
frame.onPaint = paint("frame")
box.onPaint = paint("box")
label.onPaint = paint("label")
form.onPaint = paint("form")
form:show()`, func(s *scene) {
		s.lua(`paints = {}; for _, c in ipairs{panel, frame, box, label, form} do c:redraw() end`)
		s.expect(`paints.panel and paints.frame and paints.box and paints.label and paints.form`)
		s.expect(`sizes.panel == "120x80" and sizes.frame == "100x80" and sizes.box == "100x28" and sizes.label == "60x20"`)
		s.expect(`sizes.form == "400x300"`)

		// g only draws while it is being drawn on.
		s.lua(`ok, err = pcall(leaked.rect, leaked, 0, 0, 5, 5)`)
		s.expect(`not ok and err:find("only works inside onDraw or onPaint", 1, true)`)

		// Taking the handler away stops the calls.
		s.lua(`paints = {}; panel.onPaint = nil; panel:redraw()`)
		s.pump()
		s.expect(`paints.panel == nil`)
	})
}

// A control drawn through onPaint can be taken off its form, and the form
// closed, without disturbing the callbacks of controls that live on: go-fltk
// forgets a draw handler by the wrong id unless the widget is destroyed by
// hand, which would drop another widget's callback.
func TestPaintHandlersAreReleased(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 400, height = 300}
clicks = 0
button = form:Button{caption = "alive", left = 10, top = 10, width = 100, height = 30, onClick = function() clicks = clicks + 1 end}
doomed = {}
for i = 1, 6 do
  doomed[i] = form:Panel{left = 10, top = 50 + i * 20, width = 100, height = 15}
  doomed[i]:Button{caption = "in", left = 0, top = 0, width = 40, height = 15}
  doomed[i].onPaint = function(self, g) g:rect(0, 0, 5, 5) end
end
form:show()`, func(s *scene) {
		s.pump()
		for i := 1; i <= 3; i++ {
			s.lua(`doomed[` + string(rune('0'+i)) + `]:remove()`)
		}
		s.pump()
		x, y := s.middle("button")
		in.Click(x, y)
		s.expect(`clicks == 1`)

		// Closing the form frees the rest, and a new form still works.
		s.lua(`form:close()`)
		s.pump()
		s.pump()
		s.lua(`form2 = gui.Form{width = 200, height = 100}
clicks2 = 0
button2 = form2:Button{caption = "two", left = 10, top = 10, width = 100, height = 30, onClick = function() clicks2 = clicks2 + 1 end}
form2:show()`)
		s.pump()
		x, y = s.middle("button2")
		in.Click(x, y)
		s.expect(`clicks2 == 1`)
	})
}

// A Label or an Image can be clicked and hovered over: onClick when a press
// and a release are both over it, onDoubleClick as well for a double click,
// and the rest as a Canvas has them.
func TestLabelMouse(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
label = form:Label{caption = "link", left = 10, top = 10, width = 80, height = 20, pointer = "hand"}
picture = form:Image{left = 10, top = 50, width = 40, height = 40}
other = form:Label{caption = "other", left = 150, top = 10, width = 80, height = 20}
log = {}
local function note(what) return function(self, x, y, button, double)
  log[#log + 1] = what .. (x and ("@" .. x .. "," .. y) or "")
end end
for _, c in ipairs{label, picture} do
  c.onClick = note("click"); c.onDoubleClick = note("double")
  c.onMouseDown = note("down"); c.onMouseUp = note("up")
  c.onMouseEnter = note("enter"); c.onMouseLeave = note("leave")
end
function label:onMouseMove(x, y) log[#log + 1] = "move@" .. x .. "," .. y end
form:show()
logged = function() return table.concat(log, " ") end`, func(s *scene) {
		x, y := s.at("label", 5, 7)
		in.Click(x, y)
		s.expect(`logged() == "down@5,7 up@5,7 click@5,7"`)

		s.lua(`log = {}`)
		in.DoubleClick(x, y)
		s.expect(`logged() == "down@5,7 up@5,7 click@5,7 double@5,7"`)

		// Pressed over it and let go elsewhere is no click.
		s.lua(`log = {}`)
		ox, oy := s.middle("other")
		in.Send(in.Event{Type: fltk.PUSH, X: x, Y: y, Key: 0xfee8 + 1, State: 0x01000000})
		in.Send(in.Event{Type: fltk.RELEASE, X: ox, Y: oy, Key: 0xfee8 + 1, IsClick: true})
		s.expect(`logged():find("^down@5,7") and not logged():find("click")`)

		// Moving in is entering, and a move; out is leaving.
		s.lua(`log = {}`)
		in.Send(in.Event{Type: fltk.MOVE, X: x, Y: y})
		s.expect(`logged() == "enter move@5,7"`)
		in.Send(in.Event{Type: fltk.MOVE, X: x + 3, Y: y})
		s.expect(`logged() == "enter move@5,7 move@8,7"`)
		in.Send(in.Event{Type: fltk.MOVE, X: ox, Y: oy})
		s.expect(`logged() == "enter move@5,7 move@8,7 leave"`)

		// An Image the same.
		s.lua(`log = {}`)
		px, py := s.at("picture", 4, 4)
		in.Click(px, py)
		s.expect(`logged() == "down@4,4 up@4,4 click@4,4"`)

		// A Label with no handlers raises nothing, as before.
		in.Click(ox, oy)
		s.expect(`logged() == "down@4,4 up@4,4 click@4,4"`)
	})
}

// A Canvas hears clicks too.
func TestCanvasClick(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
canvas = form:Canvas{left = 10, top = 10, width = 100, height = 80}
log = {}
function canvas:onDraw(g) end
function canvas:onClick(x, y) log[#log + 1] = "click@" .. x .. "," .. y end
function canvas:onDoubleClick(x, y) log[#log + 1] = "double@" .. x .. "," .. y end
form:show()
logged = function() return table.concat(log, " ") end`, func(s *scene) {
		x, y := s.at("canvas", 20, 30)
		in.Click(x, y)
		s.expect(`logged() == "click@20,30"`)
		s.lua(`log = {}`)
		in.DoubleClick(x, y)
		s.expect(`logged() == "click@20,30 double@20,30"`)
	})
}

// A page of a Tabs is shown when its tab is chosen, and hidden when another
// is.
func TestPageShowAndHide(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
tabs = form:Tabs{left = 10, top = 10, width = 280, height = 180}
first = tabs:Page{caption = "One"}
second = tabs:Page{caption = "Two"}
count = {}
local function note(what) return function() count[what] = (count[what] or 0) + 1 end end
first.onShow, first.onHide = note("show1"), note("hide1")
second.onShow, second.onHide = note("show2"), note("hide2")
form:show()`, func(s *scene) {
		s.lua(`count = {}; tabs.selected = 2`)
		s.expect(`count.hide1 == 1 and count.show2 == 1 and count.show1 == nil and count.hide2 == nil`)
		s.lua(`count = {}; tabs.selected = 1`)
		s.expect(`count.hide2 == 1 and count.show1 == 1 and count.show2 == nil and count.hide1 == nil`)
	})
}

// A Scroll tells when its content was scrolled, by the program or by the
// wheel, once for however far it went, with where it is.
func TestScrollEvent(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
scroll = form:Scroll{left = 10, top = 10, width = 200, height = 100}
tall = scroll:Panel{left = 0, top = 0, width = 180, height = 600}
tall:Button{caption = "far down", left = 10, top = 500, width = 100, height = 30}
seen = {}
function scroll:onScroll(x, y) seen[#seen + 1] = x .. "," .. y end
form:show()`, func(s *scene) {
		s.pump()
		s.expect(`#seen == 0`)
		s.lua(`scroll.scrollY = 40`)
		s.pump()
		s.expect(`#seen == 1 and seen[1] == "0,40"`)
		s.lua(`scroll.scrollY = 40`)
		s.pump()
		s.expect(`#seen == 1`)

		x, y := s.middle("scroll")
		in.Wheel(x, y, 0, 2)
		s.pump()
		s.expect(`#seen == 2 and seen[2] ~= "0,40" and scroll.scrollY ~= 40`)

		// Set after it was built, and taken away again.
		s.lua(`seen = {}; scroll.onScroll = nil; scroll.scrollY = 0`)
		s.pump()
		s.expect(`#seen == 0`)
	})
}
