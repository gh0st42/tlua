//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import "testing"

// Painting draws offscreen, but FLTK still needs the display, so these run
// with the tests that open windows: TLUA_GUI_TESTS=1.

func TestPaint(t *testing.T) {
	onScreen(t, `
local png = require "png"
pic = gui.paint(40, 20, function(g)
  local w, h = g:size()
  sw, sh = w, h
  g:color("#ff0000") g:fill(0, 0, 20, 20)
  g:color("#0000ff") g:fill(20, 0, 20, 20)
  mw = g:measure("hello")
end, 2)
canvas = gui.Form{}:Canvas{width = 30, height = 10, onDraw = function(self, g)
  g:color("#00ff00") g:fill(0, 0, 30, 10)
end}
snap = canvas:snapshot()
ok1, err1 = pcall(gui.paint, 10, 10, function() error("boom") end)
ok2, err2 = pcall(function() return gui.Form{}:snapshot() end)
ok3 = pcall(gui.paint, 10, 10, function() end, 0)
`, func(s *scene) {
		s.expect(`sw == 40 and sh == 20`)
		s.expect(`pic.width == 80 and pic.height == 40`)
		s.expect(`select(1, pic:get(10, 10)) == 255 and select(3, pic:get(10, 10)) == 0`)
		s.expect(`select(3, pic:get(70, 30)) == 255 and select(1, pic:get(70, 30)) == 0`)
		s.expect(`mw > 10 and mw < 60`) // in units, not the picture's pixels
		s.expect(`snap.width == 30 and snap.height == 10 and select(2, snap:get(15, 5)) == 255`)
		s.expect(`not ok1 and err1:find("boom")`)
		s.expect(`not ok2 and err2:find("only a Canvas")`)
		s.expect(`not ok3`)
	})
}

// Fonts are named as the system names them, and a MarkdownView can be
// drawn in one, in colours of the program's.
func TestFontsAndMarkdownViewLook(t *testing.T) {
	onScreen(t, `
fonts = gui.fonts()
first = table.concat({ fonts[1], fonts[2], fonts[3] }, " ")
okBad, errBad = pcall(gui.measure, "x", "No Such Font At All")
installed = fonts[4]
if installed then wi = gui.measure("hello", installed, 14) end
okPaint, errPaint = pcall(gui.paint, 10, 10, function(g) g:font("No Such Font At All", 12) end)
form = gui.Form{width = 200, height = 300}
okProp = pcall(function() local l = form:Label{} l.font = "No Such Font At All" end)
view = form:MarkdownView{width = 200, height = 300, text = "# Hi\n\n[link](x.md)\n"}
view.colors = { background = "#000080", link = "#55ffff", heading = "#ffffff" }
view.face = "mono"
snap = view:snapshot()
px = table.concat({ snap:get(190, 290) }, ",")
`, func(s *scene) {
		s.expect(`first == "sans serif mono"`)
		s.expect(`not okBad and errBad:find("gui.fonts")`)
		s.expect(`not installed or (wi and wi > 0)`)
		s.expect(`not okPaint and errPaint:find("No Such Font")`)
		s.expect(`not okProp`)
		s.expect(`px == "0,0,128,255"`) // the background
	})
}
