package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestFindIn(t *testing.T) {
	text := "local a = 1\nlocal b = 2\nprint(a + b)\n"

	start, end, wrapped, ok := findIn(text, "local", 0, true)
	if !ok || start != 0 || end != 5 || wrapped {
		t.Errorf("first match: %d..%d wrapped=%v ok=%v", start, end, wrapped, ok)
	}

	start, end, wrapped, ok = findIn(text, "local", end, true)
	if !ok || start != 12 || wrapped {
		t.Errorf("second match: %d..%d wrapped=%v", start, end, wrapped)
	}

	// Past the last match it comes back round to the first.
	start, _, wrapped, ok = findIn(text, "local", len(text), true)
	if !ok || start != 0 || !wrapped {
		t.Errorf("wrap: start=%d wrapped=%v ok=%v", start, wrapped, ok)
	}

	if _, _, _, ok := findIn(text, "missing", 0, true); ok {
		t.Error("found something that is not there")
	}
	if _, _, _, ok := findIn(text, "", 0, true); ok {
		t.Error("an empty search should find nothing")
	}
}

func TestFindInCaseSensitivity(t *testing.T) {
	text := "Print(x)\nprint(y)\n"

	if start, _, _, ok := findIn(text, "print", 0, true); !ok || start != 9 {
		t.Errorf("case sensitive: start=%d ok=%v", start, ok)
	}
	if start, _, _, ok := findIn(text, "print", 0, false); !ok || start != 0 {
		t.Errorf("case insensitive: start=%d ok=%v", start, ok)
	}
	// Folding must not shift offsets: this letter grows when lowercased, so the
	// search stays case sensitive rather than selecting the wrong text.
	grown := "aİb print"
	if start, end, _, ok := findIn(grown, "print", 0, false); !ok || grown[start:end] != "print" {
		t.Errorf("offsets shifted: %d..%d ok=%v", start, end, ok)
	}
}

func TestFindBefore(t *testing.T) {
	text := "x\nfoo\nbar\nfoo\n"

	start, _, wrapped, ok := findBefore(text, "foo", len(text), true)
	if !ok || start != 10 || wrapped {
		t.Errorf("last match: start=%d wrapped=%v ok=%v", start, wrapped, ok)
	}
	start, _, wrapped, ok = findBefore(text, "foo", start, true)
	if !ok || start != 2 || wrapped {
		t.Errorf("previous match: start=%d wrapped=%v", start, wrapped)
	}
	start, _, wrapped, ok = findBefore(text, "foo", start, true)
	if !ok || start != 10 || !wrapped {
		t.Errorf("wrap backwards: start=%d wrapped=%v", start, wrapped)
	}
}

func TestMatchesAll(t *testing.T) {
	got := matchesAll("aXaXa", "a", true)
	if len(got) != 3 || got[0][0] != 0 || got[1][0] != 2 || got[2][0] != 4 {
		t.Errorf("matches = %v", got)
	}
	// Overlapping patterns advance past each match, as a replace has to.
	if got := matchesAll("aaaa", "aa", true); len(got) != 2 {
		t.Errorf("overlaps = %v", got)
	}
	if got := matchesAll("abc", "", true); got != nil {
		t.Errorf("empty needle = %v", got)
	}
}

func TestLineOf(t *testing.T) {
	text := "one\ntwo\nthree"
	for offset, want := range map[int]int{0: 1, 3: 1, 4: 2, 8: 3, len(text): 3} {
		if got := lineOf(text, offset); got != want {
			t.Errorf("lineOf(%d) = %d, want %d", offset, got, want)
		}
	}
}

/* --- driving the editor --- */

func TestFindDialogSelectsTheMatch(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"),
		"local a = 1\nlocal target = 2\nprint(target)\n"))

	press(screen, tcell.KeyCtrlF, 0, tcell.ModNone)
	waitFor(t, e, "the Find dialog", func() bool { return e.modals == 1 })

	typeText(screen, "target")
	press(screen, tcell.KeyEnter, 0, tcell.ModNone) // Enter searches

	waitFor(t, e, "the match to be selected", func() bool {
		selected, start, end := e.buffers[0].area.GetSelection()
		return e.modals == 0 && start != end && selected == "target"
	})
	onEditor(t, e, func() {
		_, start, _ := e.buffers[0].area.GetSelection()
		if line := lineOf(e.buffers[0].area.GetText(), start); line != 2 {
			t.Errorf("selected the match on line %d, want 2", line)
		}
		if !strings.Contains(e.status, "line 2") {
			t.Errorf("status = %q", e.status)
		}
	})
}

func TestFindNextWalksThroughAndWrapsRound(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"),
		"aim\nx\naim\ny\naim\n"))
	onEditor(t, e, func() { e.search = searchState{what: "aim"} })

	lines := []int{1, 3, 5, 1}
	for _, want := range lines {
		press(screen, tcell.KeyF7, 0, tcell.ModNone)
		waitFor(t, e, "the next match", func() bool {
			_, start, end := e.buffers[0].area.GetSelection()
			return start != end && lineOf(e.buffers[0].area.GetText(), start) == want
		})
	}
	onEditor(t, e, func() {
		if !strings.Contains(e.status, "wrapped") {
			t.Errorf("the wrap should be mentioned: %q", e.status)
		}
	})
}

func TestFindPreviousGoesBackwards(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "aim\nx\naim\n"))
	onEditor(t, e, func() {
		e.search = searchState{what: "aim"}
		e.gotoLine(e.buffers[0], 3)
	})

	press(screen, tcell.KeyF8, 0, tcell.ModNone)
	waitFor(t, e, "the previous match", func() bool {
		_, start, end := e.buffers[0].area.GetSelection()
		return start != end && lineOf(e.buffers[0].area.GetText(), start) == 1
	})
}

func TestFindReportsWhatIsNotThere(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "local a = 1\n"))
	onEditor(t, e, func() { e.search = searchState{what: "nowhere"} })

	press(screen, tcell.KeyF7, 0, tcell.ModNone)
	waitFor(t, e, "the not-found message", func() bool {
		return strings.Contains(e.status, "not found")
	})
}

func TestReplaceOneOccurrence(t *testing.T) {
	dir := t.TempDir()
	path := write(t, filepath.Join(dir, "main.lua"), "print(old)\nprint(old)\n")
	e, _ := start(t, path)

	onEditor(t, e, func() {
		e.search = searchState{what: "old", with: "new"}
		e.replaceCurrent()
	})
	waitFor(t, e, "the first occurrence to change", func() bool {
		return e.buffers[0].area.GetText() == "print(new)\nprint(old)\n"
	})
	onEditor(t, e, func() {
		if !e.buffers[0].dirty {
			t.Error("the buffer should be modified")
		}
	})
}

func TestReplaceAllOccurrences(t *testing.T) {
	dir := t.TempDir()
	path := write(t, filepath.Join(dir, "main.lua"),
		"local count = 1\ncount = count + 1\nprint(count)\n")
	e, screen := start(t, path)

	onEditor(t, e, func() {
		e.search = searchState{what: "count", with: "total"}
		e.replaceAll()
	})
	waitFor(t, e, "every occurrence to change", func() bool {
		return !strings.Contains(e.buffers[0].area.GetText(), "count")
	})
	onEditor(t, e, func() {
		want := "local total = 1\ntotal = total + 1\nprint(total)\n"
		if got := e.buffers[0].area.GetText(); got != want {
			t.Errorf("text = %q", got)
		}
		if !strings.Contains(e.status, "Replaced 4") {
			t.Errorf("status = %q", e.status)
		}
	})

	// Undo puts them all back, one step at a time, because each replacement
	// went through the text area.
	for i := 0; i < 4; i++ {
		press(screen, tcell.KeyCtrlZ, 0, tcell.ModNone)
	}
	waitFor(t, e, "undo to restore the text", func() bool {
		return e.buffers[0].area.GetText() == "local count = 1\ncount = count + 1\nprint(count)\n"
	})

	// And saving writes what is on screen.
	press(screen, tcell.KeyF2, 0, tcell.ModNone)
	waitFor(t, e, "the save", func() bool { return !e.buffers[0].dirty })
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "total") {
		t.Errorf("file = %q", data)
	}
}

func TestReplaceDialogReplacesAll(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "a a a\n"))

	press(screen, tcell.KeyCtrlR, 0, tcell.ModNone)
	waitFor(t, e, "the Replace dialog", func() bool { return e.modals == 1 })

	typeText(screen, "a")
	press(screen, tcell.KeyTab, 0, tcell.ModNone)
	typeText(screen, "b")
	// Tab to the "Replace all" button and press it.
	for i := 0; i < 3; i++ {
		press(screen, tcell.KeyTab, 0, tcell.ModNone)
	}
	press(screen, tcell.KeyEnter, 0, tcell.ModNone)

	waitFor(t, e, "every a to become a b", func() bool {
		return e.modals == 0 && e.buffers[0].area.GetText() == "b b b\n"
	})
}

func TestSearchRemembersWhatWasAskedFor(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "Alpha\nalpha\n"))

	press(screen, tcell.KeyCtrlF, 0, tcell.ModNone)
	waitFor(t, e, "the Find dialog", func() bool { return e.modals == 1 })
	typeText(screen, "alpha")
	press(screen, tcell.KeyTab, 0, tcell.ModNone) // to the case checkbox
	typeText(screen, " ")                         // tick it
	press(screen, tcell.KeyEnter, 0, tcell.ModNone)

	waitFor(t, e, "the case sensitive match on line 2", func() bool {
		_, start, end := e.buffers[0].area.GetSelection()
		return e.modals == 0 && start != end &&
			lineOf(e.buffers[0].area.GetText(), start) == 2
	})
	onEditor(t, e, func() {
		if e.search.what != "alpha" || !e.search.matchCase {
			t.Errorf("search state = %+v", e.search)
		}
	})
}
