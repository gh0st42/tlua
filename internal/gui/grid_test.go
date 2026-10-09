//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import (
	"testing"

	"github.com/pwiecz/go-fltk"

	in "tlua/internal/gui/fltkinput"
)

// The grid an editable Table makes: two columns, the second edited, as a
// property grid has them.
const editableGrid = `
form = gui.Form{width = 400, height = 300}
rows = {{"name", "Ada"}, {"year", "1815"}, {"poet", "false"}, {"notes", "(2)"}, {"fixed", "x"}}
grid = form:Table{left = 10, top = 10, width = 300, height = 200,
  columns = {"Property", "Value"}, columnWidths = {100, 180}, rows = rows, editable = {2},
  onChange = function(self) row = self.selected end}
edits, pressed = {}, 0
function grid:onStartEdit(r, c)
  started = r .. "," .. c
  if r == 3 then return { choices = {"true", "false"} } end
  if r == 4 then return { button = true, readOnly = true } end
  if r == 5 then return false end
end
function grid:onEdit(r, c, text)
  edits[#edits + 1] = r .. "," .. c .. "=" .. text
  if r == 2 and not tonumber(text) then return false end
  if r == 1 then return text:upper() end
end
function grid:onEditButton(r, c)
  pressed = pressed + 1
  rows[r][c] = "(3)"
end
form:show()`

// cellAt is the middle of a cell of the grid, counting from 1: rows start
// below the 24-pixel header and are 22 high.
func cellAt(s *scene, row, col int) (int, int) {
	x := 50
	if col == 2 {
		x = 100 + 90
	}
	return s.at("grid", x, 24+(row-1)*22+11)
}

func TestEditingATable(t *testing.T) {
	onScreen(t, editableGrid, func(s *scene) {
		// A click on a cell that cannot be edited only selects its row.
		in.Click(cellAt(s, 1, 1))
		s.expect(`row == 1 and started == nil and grid:editing() == nil`)

		// One that can starts editing it; Enter keeps what was typed, as
		// onEdit has it.
		in.Click(cellAt(s, 1, 2))
		s.expect(`started == "1,2" and select(2, grid:editing()) == 2`)
		in.Key('a', "a", fltk.META)
		in.Type("grace")
		in.Key(fltk.ENTER_KEY, "", 0)
		s.expect(`edits[1] == "1,2=grace" and rows[1][2] == "GRACE" and grid:editing() == nil`)

		// Refused, the editor stays, and Escape puts the cell back.
		in.Click(cellAt(s, 2, 2))
		in.Type("soon")
		in.Key(fltk.ENTER_KEY, "", 0)
		s.expect(`edits[2] == "2,2=soon" and rows[2][2] == "1815" and grid:editing() == 2`)
		in.Key(fltk.ESCAPE, "", 0)
		s.expect(`grid:editing() == nil and rows[2][2] == "1815" and #edits == 2`)
		s.expect(`form.visible`) // Escape went to the editor, not the form

		// Typing on a selected row starts an edit with what was typed, and
		// Down keeps it and edits the row below.
		in.Type("1")
		s.expect(`grid:editing() == 2`)
		in.Type("9")
		in.Key(fltk.DOWN, "", 0)
		s.expect(`rows[2][2] == "19" and grid:editing() == 3 and row == 3`)

		// A cell with choices takes the next one on a double click.
		in.DoubleClick(cellAt(s, 3, 2))
		s.expect(`rows[3][2] == "true"`)

		// The "..." of a cell is the script's to answer.
		in.Click(cellAt(s, 4, 2))
		s.expect(`grid:editing() == 4`)
		in.Click(s.at("grid", 100+180-11, 24+3*22+11))
		s.expect(`pressed == 1 and rows[4][2] == "(3)"`)

		// onStartEdit can refuse, and the script can start an edit itself.
		in.Click(cellAt(s, 5, 2))
		s.expect(`started == "5,2" and grid:editing() == nil and row == 5`)
		grid := `grid`
		s.lua(grid + `:edit(1, 2)`)
		s.expect(`grid:editing() == 1 and row == 1`)
		s.lua(`grid:edit()`)
		s.expect(`grid:editing() == nil and #edits == 4`)
	})
}

func TestChoosingAColour(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 200, height = 100}
form:show()`, func(s *scene) {
		// OK keeps the colour it started at; Cancel is nil.
		later(func() { in.Key(fltk.ENTER_KEY, "\r", 0) })
		s.lua(`picked = gui.choosecolor{title = "Pick", color = "#336699"}`)
		s.expect(`picked == "#336699"`)
		later(func() { in.Key(fltk.ESCAPE, "", 0) })
		s.lua(`picked = gui.choosecolor{color = "red"}`)
		s.expect(`picked == nil`)
	})
}

// A Table goes with the container it is in, without a word: the events
// its widget is sent on its way out find nothing to answer.
func TestRemovingAContainerOfATable(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 300, height = 200}
frame = form:Frame{caption = "box", left = 10, top = 10, width = 280, height = 180}
grid = frame:Table{left = 10, top = 20, width = 200, height = 100, columns = {"A"}, rows = {{"1"}}, editable = true}
frame:Button{caption = "b", left = 10, top = 130}
form:show()`, func(s *scene) {
		s.focus("grid")
		in.Click(s.at("grid", 20, 24+11))
		s.lua(`frame:remove()`)
		s.pump()
		in.Click(150, 100)
		in.RightClick(150, 100)
		// remove() raising, or a handler failing, fails the test.
		s.expect(`grid.rows[1][1] == "1"`)
	})
}
