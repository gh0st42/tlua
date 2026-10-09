//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import (
	goruntime "runtime"
	"testing"

	"github.com/pwiecz/go-fltk"

	in "tlua/internal/gui/fltkinput"
)

// whenMenuUp runs fn once a popup menu is up and has settled, from the
// menu's own loop.
func whenMenuUp(fn func()) {
	var try func()
	try = func() {
		if in.Grabbing() {
			fltk.AddTimeout(0.15, fn)
		} else {
			fltk.AddTimeout(0.05, try)
		}
	}
	fltk.AddTimeout(0.05, try)
}

// pickNth picks the nth item of the menu a right click opens, with keys.
func pickNth(n int) {
	whenMenuUp(func() {
		for i := 0; i < n; i++ {
			in.Key(fltk.DOWN, "", 0)
		}
		in.Key(fltk.ENTER_KEY, "\r", 0)
		wake()
	})
}

// dismiss closes it with Escape, which the menu reads by its text too.
func dismiss() {
	whenMenuUp(func() {
		in.Key(fltk.ESCAPE, "\x1b", 0)
		wake()
	})
}

// wake has the menu's loop look again. Keys sent from a timer, as these
// are, end the menu without waking the loop that waits for it to end, as
// keys from the system do.
func wake() { fltk.AddTimeout(0.05, func() {}) }

func TestContextMenus(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 400, height = 300}
picked = {}
btn = form:Button{caption = "Thing", left = 10, top = 10, width = 120,
  contextMenu = {
    {"&Rename", function(caption) renamed = caption end},
    "-",
    {"Delete", name = "del"},
  },
  onContextMenu = function(self, name, caption) picked[#picked + 1] = tostring(name) .. ":" .. caption end}
panel = form:Panel{left = 10, top = 60, width = 300, height = 100, contextMenu = {"Panel item"},
  onContextMenu = function(self, name, caption) panelPick = caption end}
inner = panel:Button{caption = "inner", left = 10, top = 10, width = 80, contextMenu = {"Inner item"},
  onContextMenu = function(self, name, caption) innerPick = caption end}
label = panel:Label{caption = "a label", left = 150, top = 10, width = 80}
list = form:ListBox{left = 10, top = 170, width = 150, height = 100, items = {"one", "two", "three"},
  contextMenu = {"Open"}, onContextMenu = function(self) listPick = self.selected end}
form:show()`, func(s *scene) {
		// The first item runs its function, and the control hears of it.
		pickNth(1)
		in.RightClick(s.middle("btn"))
		s.expect(`renamed == "&Rename" and picked[1] == "nil:&Rename"`)

		// An item is told apart by name.
		pickNth(2)
		in.RightClick(s.middle("btn"))
		s.expect(`picked[2] == "del:Delete"`)

		// Closed without a pick, nothing happens.
		dismiss()
		in.RightClick(s.middle("btn"))
		s.expect(`#picked == 2`)

		// A left click is a click.
		in.Click(s.middle("btn"))
		s.expect(`#picked == 2`)

		// The innermost control with a menu shows it: a button's own, and
		// the panel's over a label that has none.
		pickNth(1)
		in.RightClick(s.middle("inner"))
		s.expect(`innerPick == "Inner item" and panelPick == nil`)
		pickNth(1)
		in.RightClick(s.middle("label"))
		s.expect(`panelPick == "Panel item"`)

		// A list selects the line under the mouse first.
		pickNth(1)
		x, y := s.at("list", 30, 10+18)
		in.RightClick(x, y)
		s.expect(`listPick == 2 and list.selected == 2`)

		// On a Mac a Ctrl-click is a right click.
		if goruntime.GOOS == "darwin" {
			s.lua(`listPick = nil`)
			pickNth(1)
			x, y := s.at("list", 30, 10)
			in.ClickWith(x, y, fltk.CTRL)
			s.expect(`listPick == 1`)
		}

		// With no menu, a right click is what it was.
		s.lua(`btn.contextMenu = nil`)
		in.RightClick(s.middle("btn"))
		s.expect(`#picked == 2`)
	})
}

func TestPopup(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
canvas = form:Canvas{left = 0, top = 0, width = 300, height = 200}
function canvas:onMouseDown(x, y, button)
  if button == 3 then
    choice, checked = gui.popup({"Cut", {"&Copy", function() copied = true end}, "-", {"Grid", checked = false}})
    answered = true
  end
end
form:show()`, func(s *scene) {
		pickNth(2)
		in.RightClick(s.middle("canvas"))
		s.expect(`answered and choice == "Copy" and copied`)
		s.lua(`answered = false`)
		pickNth(3)
		in.RightClick(s.middle("canvas"))
		s.expect(`answered and choice == "Grid" and checked == true`)
		s.lua(`answered = false; choice = "x"`)
		dismiss()
		in.RightClick(s.middle("canvas"))
		s.expect(`answered and choice == nil`)
	})
}
