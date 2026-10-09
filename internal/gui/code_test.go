//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import (
	"testing"
	"time"

	"github.com/pwiecz/go-fltk"
	lua "github.com/yuin/gopher-lua"

	in "tlua/internal/gui/fltkinput"
)

func TestWhatATextBoxOffersACodeEditor(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 400, height = 300}
code = form:TextBox{left = 10, top = 10, width = 380, height = 200, multiLine = true,
  syntax = "lua", acceptsTab = true, text = "local gui = require 'gui'\nprint(gui)\n"}
tip = form:Label{left = 40, top = 40, width = 100, height = 20, caption = "tip", color = "#ffffe0"}
keys = {}
function code:onKey(key, text)
  keys[#keys + 1] = key
  return key == "Ctrl+Space" or key == "Down"
end
function code:onHover(pos) hovered = pos or "gone" end
form:show()`, func(s *scene) {
		s.focus("code")
		s.lua(`code.cursor = 0`)

		// onKey sees keys first, and keeps those it says it used.
		in.Key(fltk.DOWN, "", 0)
		s.expect(`keys[1] == "Down" and code.line == 1`)
		in.Key(' ', " ", fltk.CTRL)
		s.expect(`keys[2] == "Ctrl+Space" and not code.text:find("^ ")`)
		in.Type("x")
		s.expect(`keys[3] == "x" and code.text:sub(1, 1) == "x"`)

		// insert replaces the selection, and leaves the cursor after it.
		s.lua(`changes = 0; code.onChange = function() changes = changes + 1 end`)
		s.lua(`code:select(2, 6); code:insert("LOCAL")`)
		s.expect(`code.text:sub(1, 7) == "xLOCAL " and code.cursor == 6 and changes == 1`)

		// pointAt is where a position is, from the box's top left: the
		// second line is a line lower than the first.
		s.lua(`x1, y1, h1 = code:pointAt(0); x2, y2 = code:pointAt(#"xLOCAL gui = require 'gui'\n")`)
		s.expect(`x1 >= 0 and x1 < 60 and y2 >= y1 + h1 - 4 and h1 > 10`)

		// The mouse resting on text is a hover there, and moving on ends it.
		s.lua(`hx, hy = code:pointAt(2)`)
		x, y := s.at("code", int(luaNumber(s, "hx"))+3, int(luaNumber(s, "hy"))+5)
		in.Send(in.Event{Type: fltk.MOVE, X: x, Y: y})
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && s.L.GetGlobal("hovered").String() == "nil" {
			s.pump()
		}
		s.expect(`hovered == 2`)
		in.Send(in.Event{Type: fltk.MOVE, X: x + 40, Y: y})
		s.expect(`hovered == "gone"`)

		// Typing in the box leaves what lies over it drawn: it is drawn
		// after the box each time.
		s.focus("code")
		in.Type("abc")
		s.expect(`tip.visible`)
	})
}

func luaNumber(s *scene, name string) float64 {
	s.t.Helper()
	return float64(lua.LVAsNumber(s.L.GetGlobal(name)))
}
