//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import (
	"strings"
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

func TestACanvasDrawsAPicture(t *testing.T) {
	onScreen(t, `
png = require "png"
pic = png.new(20, 10, "#ff0000")
form = gui.Form{width = 200, height = 100}
draws = 0
canvas = form:Canvas{left = 10, top = 10, width = 100, height = 50,
  onDraw = function(self, g) draws = draws + 1; g:image(pic, 5, 5); g:image(pic, 40, 5, 40, 20) end}
form:show()`, func(s *scene) {
		s.expect(`draws >= 1`)
		pictures := func() []string {
			var keys []string
			for k := range canvasOf(s.obj("canvas")).images {
				if strings.HasPrefix(k, "png:") {
					keys = append(keys, k)
				}
			}
			return keys
		}
		if n := len(pictures()); n != 2 {
			s.t.Errorf("the picture at two sizes is %d images: %v", n, pictures())
		}
		// A change is drawn, and what was made of the picture before it is
		// let go.
		s.lua(`before = draws; pic:set(0, 0, "#00ff00"); canvas:redraw()`)
		s.expect(`draws > before`)
		for _, k := range pictures() {
			if !strings.Contains(k, ":1@") {
				s.t.Errorf("an image of the picture before it changed is kept: %s", k)
			}
		}
		if n := len(pictures()); n != 2 {
			s.t.Errorf("after the change, %d images: %v", n, pictures())
		}
	})
}

func TestNearestScalingDrawsWholePixels(t *testing.T) {
	onScreen(t, `
png = require "png"
pic = png.new(4, 4, "#ffffff")
pic:set(0, 0, "#000000")
form = gui.Form{width = 200, height = 120}
canvas = form:Canvas{left = 10, top = 10, width = 180, height = 100,
  onDraw = function(self, g)
    g:scaling("nearest"); g:image(pic, 0, 0, 64, 64)
    g:scaling("smooth"); g:image(pic, 80, 0, 64, 64)
    ok, why = pcall(g.scaling, g, "blurry")
  end}
form:show()`, func(s *scene) {
		s.expect(`ok == false and tostring(why):find("nearest")`)
		var nearest, smooth int
		for k := range canvasOf(s.obj("canvas")).images {
			switch {
			case strings.Count(k, "@") == 2:
				nearest++ // made pixel for pixel: its size, and the pixels'
			case strings.HasPrefix(k, "png:"):
				smooth++
			}
		}
		if nearest != 1 || smooth != 1 {
			s.t.Errorf("%d nearest and %d smooth images, want one of each", nearest, smooth)
		}
	})
}
