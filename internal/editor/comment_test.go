package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestToggleCommentBlock(t *testing.T) {
	cases := []struct {
		name, in, want string
		commented      bool
	}{
		{
			name:      "one line",
			in:        "print(x)\n",
			want:      "-- print(x)\n",
			commented: true,
		},
		{
			name:      "back again",
			in:        "-- print(x)\n",
			want:      "print(x)\n",
			commented: false,
		},
		{
			name:      "the prefix goes in at the shallowest indentation",
			in:        "  if x then\n    return 1\n  end\n",
			want:      "  -- if x then\n  --   return 1\n  -- end\n",
			commented: true,
		},
		{
			name:      "blank lines are left alone",
			in:        "a = 1\n\nb = 2\n",
			want:      "-- a = 1\n\n-- b = 2\n",
			commented: true,
		},
		{
			name:      "a part commented block gets commented, so the next press undoes it",
			in:        "-- a = 1\nb = 2\n",
			want:      "-- -- a = 1\n-- b = 2\n",
			commented: true,
		},
		{
			name:      "uncommenting takes one space, not the indentation",
			in:        "--   deep\n",
			want:      "  deep\n",
			commented: false,
		},
		{
			name:      "a comment with no space still uncomments",
			in:        "--x = 1\n",
			want:      "x = 1\n",
			commented: false,
		},
		{
			name:      "tabs count as indentation",
			in:        "\tlocal x = 1\n\tlocal y = 2\n",
			want:      "\t-- local x = 1\n\t-- local y = 2\n",
			commented: true,
		},
		{
			name:      "a last line without a newline stays that way",
			in:        "a = 1\nb = 2",
			want:      "-- a = 1\n-- b = 2",
			commented: true,
		},
		{
			name:      "blank lines alone are nothing to comment",
			in:        "\n\n",
			want:      "\n\n",
			commented: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, commented := toggleCommentBlock(c.in)
			if got != c.want {
				t.Errorf("got  %q\nwant %q", got, c.want)
			}
			if commented != c.commented {
				t.Errorf("commented = %v, want %v", commented, c.commented)
			}
		})
	}
}

// Commenting and uncommenting has to come back to where it started, or the key
// is not a toggle.
func TestToggleCommentRoundTrips(t *testing.T) {
	for _, text := range []string{
		"print(x)\n",
		"  if x then\n    return 1\n  end\n",
		"a = 1\n\nb = 2\n",
		"\tlocal x = 1\n",
		"local s = \"--\"\n",
	} {
		once, _ := toggleCommentBlock(text)
		twice, _ := toggleCommentBlock(once)
		if twice != text {
			t.Errorf("%q became %q and then %q", text, once, twice)
		}
	}
}

func TestLineSpan(t *testing.T) {
	text := "one\ntwo\nthree\n"
	cases := []struct {
		first, last, start, end int
	}{
		{0, 0, 0, 4},
		{1, 1, 4, 8},
		{0, 2, 0, 14},
		{2, 2, 8, 14},
	}
	for _, c := range cases {
		start, end := lineSpan(text, c.first, c.last)
		if start != c.start || end != c.end {
			t.Errorf("lineSpan(%d, %d) = %d..%d, want %d..%d",
				c.first, c.last, start, end, c.start, c.end)
		}
	}

	// A file whose last line has no newline still spans to its end.
	if start, end := lineSpan("a\nb", 1, 1); start != 2 || end != 3 {
		t.Errorf("unterminated last line: %d..%d", start, end)
	}
}

/* --- driving the editor --- */

func TestToggleCommentOnTheCursorLine(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"),
		"local a = 1\nlocal b = 2\nlocal c = 3\n"))

	onEditor(t, e, func() { e.gotoLine(e.buffers[0], 2) })
	press(screen, tcell.KeyCtrlB, 0, tcell.ModNone)

	waitFor(t, e, "the line to be commented", func() bool {
		return e.buffers[0].area.GetText() == "local a = 1\n-- local b = 2\nlocal c = 3\n"
	})
	onEditor(t, e, func() {
		row, _, _, _ := e.buffers[0].area.GetCursor()
		if row != 1 {
			t.Errorf("the cursor moved to line %d", row+1)
		}
		if !strings.Contains(e.status, "Commented line 2") {
			t.Errorf("status = %q", e.status)
		}
	})

	press(screen, tcell.KeyCtrlB, 0, tcell.ModNone)
	waitFor(t, e, "the line to be uncommented again", func() bool {
		return e.buffers[0].area.GetText() == "local a = 1\nlocal b = 2\nlocal c = 3\n"
	})
}

func TestToggleCommentOverASelection(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"),
		"function f()\n  local x = 1\n  return x\nend\n"))

	// Select the two lines in the middle.
	onEditor(t, e, func() {
		text := e.buffers[0].area.GetText()
		start, end := lineSpan(text, 1, 2)
		e.buffers[0].area.Select(start, end)
	})
	press(screen, tcell.KeyCtrlB, 0, tcell.ModNone)

	waitFor(t, e, "both lines to be commented", func() bool {
		return e.buffers[0].area.GetText() ==
			"function f()\n  -- local x = 1\n  -- return x\nend\n"
	})
	onEditor(t, e, func() {
		if !strings.Contains(e.status, "Commented 2 lines") {
			t.Errorf("status = %q", e.status)
		}
	})

	// The same lines stay selected, so pressing again undoes it.
	press(screen, tcell.KeyCtrlB, 0, tcell.ModNone)
	waitFor(t, e, "both lines to come back", func() bool {
		return e.buffers[0].area.GetText() ==
			"function f()\n  local x = 1\n  return x\nend\n"
	})
}

// A selection that ends at the start of a line has not reached into it.
func TestToggleCommentIgnoresALineTheSelectionOnlyTouches(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "a = 1\nb = 2\nc = 3\n"))

	onEditor(t, e, func() {
		text := e.buffers[0].area.GetText()
		e.buffers[0].area.Select(0, offsetAt(text, 1, 0)) // all of line 1, nothing of line 2
	})
	press(screen, tcell.KeyCtrlB, 0, tcell.ModNone)

	waitFor(t, e, "only the first line to be commented", func() bool {
		return e.buffers[0].area.GetText() == "-- a = 1\nb = 2\nc = 3\n"
	})
}

func TestToggleCommentIsOneUndoStep(t *testing.T) {
	dir := t.TempDir()
	original := "a = 1\nb = 2\nc = 3\n"
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), original))

	onEditor(t, e, func() {
		text := e.buffers[0].area.GetText()
		start, end := lineSpan(text, 0, 2)
		e.buffers[0].area.Select(start, end)
	})
	press(screen, tcell.KeyCtrlB, 0, tcell.ModNone)
	waitFor(t, e, "everything to be commented", func() bool {
		return strings.HasPrefix(e.buffers[0].area.GetText(), "-- a = 1")
	})

	press(screen, tcell.KeyCtrlZ, 0, tcell.ModNone)
	waitFor(t, e, "one undo to take it all back", func() bool {
		return e.buffers[0].area.GetText() == original
	})
}

func TestToggleCommentSavesAsCommented(t *testing.T) {
	dir := t.TempDir()
	path := write(t, filepath.Join(dir, "main.lua"), "print('x')\n")
	e, screen := start(t, path)

	press(screen, tcell.KeyCtrlB, 0, tcell.ModNone)
	waitFor(t, e, "the comment", func() bool {
		return e.buffers[0].area.GetText() == "-- print('x')\n"
	})
	press(screen, tcell.KeyF2, 0, tcell.ModNone)
	waitFor(t, e, "the save", func() bool { return strings.HasPrefix(e.status, "Saved ") })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "-- print('x')\n" {
		t.Errorf("file = %q", data)
	}
}

// Commented-out code still parses as a file, so the syntax check agrees.
func TestCommentedCodeStillChecks(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "local x = 1\nprint(x)\n"))

	onEditor(t, e, func() {
		text := e.buffers[0].area.GetText()
		start, end := lineSpan(text, 0, 1)
		e.buffers[0].area.Select(start, end)
	})
	press(screen, tcell.KeyCtrlB, 0, tcell.ModNone)
	waitFor(t, e, "the comments", func() bool {
		return strings.HasPrefix(e.buffers[0].area.GetText(), "-- local x = 1")
	})

	press(screen, tcell.KeyF9, 0, tcell.ModNone)
	waitFor(t, e, "the syntax check", func() bool {
		return strings.Contains(e.status, "no syntax errors")
	})
}
