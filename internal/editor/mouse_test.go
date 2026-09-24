package editor

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// click presses and releases the left button, which is what tview turns into
// a click action.
func click(screen tcell.SimulationScreen, x, y int) {
	screen.InjectMouse(x, y, tcell.Button1, tcell.ModNone)
	screen.InjectMouse(x, y, tcell.ButtonNone, tcell.ModNone)
}

func TestMouseOpensAndSwitchesMenus(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "-- main\n"))

	run, file := menuIndex(t, e, "Run"), menuIndex(t, e, "File")
	var runColumn, fileColumn int
	onEditor(t, e, func() {
		fileColumn = e.menus[file].col
		runColumn = e.menus[run].col
	})

	click(screen, runColumn, 0)
	waitFor(t, e, "the Run menu to open", func() bool { return e.openMenu == run })

	click(screen, fileColumn, 0)
	waitFor(t, e, "the File menu to take over", func() bool { return e.openMenu == file })

	// Clicking the open title again puts the menu away.
	click(screen, fileColumn, 0)
	waitFor(t, e, "the menu to close", func() bool { return e.openMenu == -1 })
}

func TestMouseChoosesAMenuItem(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "-- main\n"))

	run := menuIndex(t, e, "Run")
	var runColumn int
	onEditor(t, e, func() { runColumn = e.menus[run].col })
	click(screen, runColumn, 0)
	waitFor(t, e, "the Run menu", func() bool { return e.openMenu == run })

	// Find the row that says "Show output" rather than counting lines, which
	// changes whenever the menu does.
	var itemY int
	onEditor(t, e, func() {
		_, y, _, _ := e.menuList.GetRect()
		for i := 0; i < e.menuList.GetItemCount(); i++ {
			if label, _ := e.menuList.GetItemText(i); strings.Contains(label, "Show output") {
				itemY = y + 1 + i // inside the border, then down to the item
				return
			}
		}
		t.Fatal("the Run menu has no Show output")
	})
	click(screen, runColumn+2, itemY)
	waitFor(t, e, "the output pane to appear", func() bool { return e.outputShown })
	waitFor(t, e, "the menu to close itself", func() bool { return e.openMenu == -1 })
}

func TestClickingOutsideAMenuClosesIt(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "-- main\nlocal x = 1\n"))

	file := menuIndex(t, e, "File")
	click(screen, e.menus[file].col, 0)
	waitFor(t, e, "the File menu", func() bool { return e.openMenu == file })

	click(screen, 60, 20) // far away, in the text
	waitFor(t, e, "the menu to close", func() bool { return e.openMenu == -1 })
}

func TestMouseSwitchesBuffers(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t,
		write(t, filepath.Join(dir, "a.lua"), "-- a\n"),
		write(t, filepath.Join(dir, "b.lua"), "-- b\n"))

	var second int
	onEditor(t, e, func() { second = e.tabSpans[1].from + 1 })
	click(screen, second, 1)
	waitFor(t, e, "the second buffer", func() bool { return e.current == 1 })

	var first int
	onEditor(t, e, func() { first = e.tabSpans[0].from + 1 })
	click(screen, first, 1)
	waitFor(t, e, "the first buffer", func() bool { return e.current == 0 })
}

func TestMousePlacesTheCursorInTheText(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"),
		"line one\nline two\nline three\n"))

	// The text starts below the two bars and inside the window frame.
	x, y, _, _ := e.buffers[0].area.GetInnerRect()
	click(screen, x+4, y+2)
	waitFor(t, e, "the cursor to move to the third line", func() bool {
		row, column, _, _ := e.buffers[0].area.GetCursor()
		return row == 2 && column == 4
	})
}

func TestMouseSelectsInTheOutlineList(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"),
		"local x = 1\n\nfunction alpha()\nend\n\nfunction beta()\nend\n"))

	press(screen, tcell.KeyF2, 0, tcell.ModAlt)
	waitFor(t, e, "the function list", func() bool { return e.modals == 1 })

	var x, y int
	onEditor(t, e, func() {
		rx, ry, _, _ := e.modalStack[0].GetRect()
		// Inside the border: row 1 is the file, row 2 alpha, row 3 beta.
		x, y = rx+2, ry+3
	})
	click(screen, x, y)
	waitFor(t, e, "the jump to beta", func() bool {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		return row == 5 && e.modals == 0
	})
}
