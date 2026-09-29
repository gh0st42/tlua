package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// longProject writes a file of the size real ones come in, with a function every
// twenty lines, so that jumping to one means jumping well out of view.
func longProject(t *testing.T, lines int) (path string, functionLine int) {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= lines; i++ {
		switch {
		case i%20 == 0:
			fmt.Fprintf(&b, "local function handler_%d(value)\n", i)
		case i%20 == 1 && i > 1:
			b.WriteString("end\n")
		default:
			fmt.Fprintf(&b, "  local padding_%d = %d -- filler to push things down\n", i, i)
		}
	}
	path = filepath.Join(t.TempDir(), "main.lua")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path, 600 // "local function handler_600" is on line 600
}

// TestFunctionListJumpsFarIntoAFile is the case that was broken: in a file
// longer than the window, the cursor ended up at the end of the last drawn line
// rather than on the function.
func TestFunctionListJumpsFarIntoAFile(t *testing.T) {
	path, line := longProject(t, 840)
	e, screen := start(t, path)

	press(screen, tcell.KeyF2, 0, tcell.ModAlt)
	waitFor(t, e, "the function list", func() bool { return e.modals == 1 })
	onEditor(t, e, func() {
		list := outlineList(t, e)
		for i := 0; i < list.GetItemCount(); i++ {
			if label, _ := list.GetItemText(i); strings.Contains(label, fmt.Sprintf("handler_%d", line)) {
				list.SetCurrentItem(i)
				return
			}
		}
		t.Fatalf("handler_%d is not in the list", line)
	})
	press(screen, tcell.KeyEnter, 0, tcell.ModNone)

	waitFor(t, e, "the jump", func() bool {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		return e.modals == 0 && row == line-1
	})
	onEditor(t, e, func() {
		b := e.buffers[0]
		row, column, _, _ := b.area.GetCursor()
		if row != line-1 {
			t.Errorf("cursor on line %d, want %d", row+1, line)
		}
		// At the start of the line, not the end of it.
		if column != 0 {
			t.Errorf("cursor at column %d, want the start of the line", column)
		}
		// And the definition is the first line of the window.
		viewRow, viewColumn := b.area.GetOffset()
		if viewRow != line-1 || viewColumn != 0 {
			t.Errorf("the window starts at line %d column %d, want line %d column 0",
				viewRow+1, viewColumn, line)
		}
	})

	redraw(t, e)
	rows := dump(screen)
	if !strings.Contains(rows[3], fmt.Sprintf("local function handler_%d", line)) {
		t.Errorf("the top line of the window is %q", rows[3])
	}
}

// Finding something far down the file lands on it too, with the lines around it
// still in view rather than the match pinned to the top.
func TestFindFarIntoAFile(t *testing.T) {
	path, _ := longProject(t, 840)
	e, screen := start(t, path)

	// Line 703 is a filler line; the twentieth line of each block is a function.
	onEditor(t, e, func() { e.search = searchState{what: "padding_703 ="} })
	press(screen, tcell.KeyF7, 0, tcell.ModNone)

	waitFor(t, e, "the match", func() bool {
		selected, start, end := e.buffers[0].area.GetSelection()
		return start != end && selected == "padding_703 ="
	})
	onEditor(t, e, func() {
		b := e.buffers[0]
		row, _, _, _ := b.area.GetCursor()
		if row != 702 {
			t.Errorf("cursor on line %d, want 703", row+1)
		}
		viewRow, _ := b.area.GetOffset()
		_, _, _, height := b.area.GetInnerRect()
		if row < viewRow || row >= viewRow+height {
			t.Errorf("line %d is outside the window, which shows %d..%d",
				row+1, viewRow+1, viewRow+height)
		}
		if viewRow == row {
			t.Error("the match is pinned to the top; a match reads better with context")
		}
	})
}

// Go to line works at any depth.
func TestGoToLineFarIntoAFile(t *testing.T) {
	path, _ := longProject(t, 840)
	e, _ := start(t, path)

	onEditor(t, e, func() { e.gotoLine(e.buffers[0], 777) })
	waitFor(t, e, "the cursor", func() bool {
		row, column, _, _ := e.buffers[0].area.GetCursor()
		return row == 776 && column == 0
	})
	onEditor(t, e, func() {
		b := e.buffers[0]
		row, _, _, _ := b.area.GetCursor()
		viewRow, _ := b.area.GetOffset()
		_, _, _, height := b.area.GetInnerRect()
		if row < viewRow || row >= viewRow+height {
			t.Errorf("line %d is outside the window %d..%d", row+1, viewRow+1, viewRow+height)
		}
	})
}

// A jump inside the window does not move it.
func TestJumpWithinTheWindowDoesNotScroll(t *testing.T) {
	path, _ := longProject(t, 840)
	e, _ := start(t, path)

	onEditor(t, e, func() { e.gotoLine(e.buffers[0], 5) })
	waitFor(t, e, "the cursor", func() bool {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		return row == 4
	})
	onEditor(t, e, func() {
		if viewRow, _ := e.buffers[0].area.GetOffset(); viewRow != 0 {
			t.Errorf("the window scrolled to %d for a line it was already showing", viewRow+1)
		}
	})
}

// The problems list jumps to a line far down the file as well.
func TestProblemJumpFarIntoAFile(t *testing.T) {
	withLanguageServer(t)
	var b strings.Builder
	for i := 1; i <= 840; i++ {
		if i == 640 {
			b.WriteString("local broken = ??\n")
			continue
		}
		fmt.Fprintf(&b, "local padding_%d = %d\n", i, i)
	}
	dir := t.TempDir()
	path := write(t, filepath.Join(dir, "main.lua"), b.String())

	e, screen := start(t, path)
	waitFor(t, e, "the report", func() bool { return len(e.buffers[0].diagnostics) == 1 })

	press(screen, tcell.KeyF8, 0, tcell.ModAlt)
	waitFor(t, e, "the jump to the problem", func() bool {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		return row == 639
	})
	onEditor(t, e, func() {
		b := e.buffers[0]
		row, _, _, _ := b.area.GetCursor()
		viewRow, _ := b.area.GetOffset()
		_, _, _, height := b.area.GetInnerRect()
		if row < viewRow || row >= viewRow+height {
			t.Errorf("the problem on line %d is outside the window %d..%d",
				row+1, viewRow+1, viewRow+height)
		}
	})
}
