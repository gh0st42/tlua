package editor

import (
	"strings"
	"testing"
)

// classesOf runs the scanner over one line and returns a string with one
// letter per rune: k keyword, b builtin, s string, n number, c comment, . text.
func classesOf(t *testing.T, line string, state lineState) (string, lineState) {
	t.Helper()
	classes, next := scanLine([]rune(line), state)
	letters := map[tokenClass]byte{
		classText: '.', classKeyword: 'k', classBuiltin: 'b',
		classString: 's', classNumber: 'n', classComment: 'c',
	}
	var sb strings.Builder
	for _, c := range classes {
		sb.WriteByte(letters[c])
	}
	return sb.String(), next
}

func TestScanLine(t *testing.T) {
	cases := []struct{ line, want string }{
		{`local x = 1`, `kkkkk.....n`},
		{`print("hi")`, `bbbbb.ssss.`},
		{`x = y -- note`, `......ccccccc`},
		{`n = 0xff + 1.5e3`, `....nnnn...nnnnn`},
		{`s = 'it\'s'`, `....sssssss`},
		{`t = {a = 1}`, `.........n.`},
		{`return f(x)`, `kkkkkk.....`},
	}
	for _, c := range cases {
		got, _ := classesOf(t, c.line, lineState{})
		if len(got) != len([]rune(c.line)) {
			t.Errorf("%q: got %d classes for %d runes", c.line, len(got), len([]rune(c.line)))
			continue
		}
		if got != c.want {
			t.Errorf("%q\n got %s\nwant %s", c.line, got, c.want)
		}
	}
}

func TestScanLineKeywordsAndBuiltins(t *testing.T) {
	got, _ := classesOf(t, `function f() return type(x) end`, lineState{})
	if !strings.HasPrefix(got, "kkkkkkkk") {
		t.Errorf("'function' should be a keyword: %s", got)
	}
	if !strings.Contains(got, "bbbb") {
		t.Errorf("'type' should be a builtin: %s", got)
	}
	// A name that merely contains a keyword is not one.
	got, _ = classesOf(t, `enders = 1`, lineState{})
	if strings.Contains(got, "k") {
		t.Errorf("'enders' was coloured as a keyword: %s", got)
	}
}

// Long brackets run past the end of a line, which is what the carried state is
// for.
func TestScanLineCarriesLongBrackets(t *testing.T) {
	first, state := classesOf(t, `s = [[ start`, lineState{})
	if !strings.HasSuffix(first, "ssssssss") {
		t.Errorf("long string not coloured: %s", first)
	}
	if !state.inLong || state.isComment {
		t.Errorf("state after the first line = %+v", state)
	}

	middle, state := classesOf(t, `still inside`, state)
	if middle != strings.Repeat("s", len("still inside")) {
		t.Errorf("continuation line = %s", middle)
	}

	last, state := classesOf(t, `end ]] x = 1`, state)
	if !strings.HasPrefix(last, "ssssss") {
		t.Errorf("closing line = %s", last)
	}
	if !strings.HasSuffix(last, "n") {
		t.Errorf("code after the long string should be coloured again: %s", last)
	}
	if state.inLong {
		t.Error("the long string should be closed")
	}
}

func TestScanLineLongComments(t *testing.T) {
	first, state := classesOf(t, `--[==[ a comment`, lineState{})
	if first != strings.Repeat("c", len("--[==[ a comment")) {
		t.Errorf("long comment = %s", first)
	}
	if !state.inLong || !state.isComment || state.longLevel != 2 {
		t.Errorf("state = %+v", state)
	}
	// A closing bracket of the wrong level does not end it.
	_, state = classesOf(t, `not the end ]=]`, state)
	if !state.inLong {
		t.Error("a level 1 bracket closed a level 2 comment")
	}
	_, state = classesOf(t, `the end ]==]`, state)
	if state.inLong {
		t.Error("the matching bracket did not close the comment")
	}
}

// The highlighter caches line states and only walks as far as it is asked to.
func TestHighlighterScansOnlyAsFarAsNeeded(t *testing.T) {
	var h highlighter
	h.reset(strings.Repeat("local x = 1\n", 1000))

	h.classesFor(3)
	if h.known > 5 {
		t.Errorf("scanned %d line states to reach line 3", h.known)
	}
	h.classesFor(500)
	if h.known > 502 {
		t.Errorf("scanned %d line states to reach line 500", h.known)
	}
	// Asking again costs nothing.
	before := h.known
	h.classesFor(200)
	if h.known != before {
		t.Errorf("re-scanned: %d then %d", before, h.known)
	}
}

func BenchmarkHighlightVisibleLines(b *testing.B) {
	const line = "local function handler(request) return string.format('%d', request.id) end\n"
	var h highlighter
	h.reset(strings.Repeat(line, 5000))
	h.classesFor(4999) // warm the line states, as scrolling there would

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// One screenful, the work a redraw actually does.
		for row := 4950; row < 4990; row++ {
			h.classesFor(row)
		}
	}
}
