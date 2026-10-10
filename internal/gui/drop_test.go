//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import (
	"testing"

	"github.com/pwiecz/go-fltk"

	in "tlua/internal/gui/fltkinput"
)

// A drop on a Tree or a ListBox says where it landed: x and y, and the
// path or the line under it. While it is dragged over one, the line under
// it is selected; the selection is put back afterwards.
func TestDropOnALine(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 400, height = 240}
tree = form:Tree{left = 10, top = 10, width = 180, height = 200,
  items = { "Alpha", { "Folder", { "Inner" }, open = true }, "Beta" }}
tree.path = "Beta"
function tree:onDrop(text, lines, x, y, at) treeGot = { text = text, x = x, y = y, at = at } end
tree.onChange = function() changes = (changes or 0) + 1 end
list = form:ListBox{left = 200, top = 10, width = 180, height = 200, items = { "one", "two", "three" }}
function list:onDrop(text, lines, x, y, at) listGot = { at = at, y = y } end
form:show()`, func(s *scene) {
		fltk.SetDrawFont(fltk.HELVETICA, 14)
		_, lh := fltk.MeasureText("Xg", false)
		// The middle of the n'th line of a list, inside its border.
		line := func(name string, n int) (int, int) { return s.at(name, 30, 3+lh*(n-1)+lh/2) }

		x, y := line("tree", 2)
		in.Send(in.Event{Type: fltk.DND_ENTER, X: x, Y: y})
		in.Send(in.Event{Type: fltk.DND_DRAG, X: x, Y: y})
		in.Send(in.Event{Type: fltk.DND_DRAG, X: x, Y: y + 1})
		s.expect(`tree.path == "Folder"`) // shows where it would land
		in.Send(in.Event{Type: fltk.DND_RELEASE, X: x, Y: y + 1})
		in.PasteText("note.md")
		handle(s.obj("tree"), fltk.PASTE)
		s.expect(`treeGot.text == "note.md" and treeGot.at == "Folder" and treeGot.x == 30`)
		s.expect(`tree.path == "Beta" and changes == nil`) // put back, unheard

		// Away from the lines, it lands on none.
		x, y = s.at("tree", 30, 180)
		in.Drop(x, y)
		in.PasteText("x")
		handle(s.obj("tree"), fltk.PASTE)
		s.expect(`treeGot.at == nil and treeGot.y == 180`)

		// A drag that goes elsewhere puts the selection back too.
		x, y = line("tree", 3)
		in.Send(in.Event{Type: fltk.DND_ENTER, X: x, Y: y})
		in.Send(in.Event{Type: fltk.DND_DRAG, X: x, Y: y})
		in.Send(in.Event{Type: fltk.DND_DRAG, X: x, Y: y + 1})
		in.Send(in.Event{Type: fltk.DND_LEAVE, X: x, Y: y})
		s.expect(`tree.path == "Beta"`)

		x, y = line("list", 3)
		in.Drop(x, y)
		in.PasteText("y")
		handle(s.obj("list"), fltk.PASTE)
		s.expect(`listGot.at == 3`)
	})
}
