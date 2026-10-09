package editor

import (
	"strings"
	"testing"
)

// classesOf runs the scanner over one line and returns a string with one
// letter per rune: k keyword, b builtin, s string, n number, c comment, . text.
// The highlighter caches line states and only walks as far as it is asked to.
func TestHighlighterScansOnlyAsFarAsNeeded(t *testing.T) {
	var h highlighter
	h.update(strings.Repeat("local x = 1\n", 1000))

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
	h.update(strings.Repeat(line, 5000))
	h.classesFor(4999) // warm the line states, as scrolling there would

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// One screenful, the work a redraw actually does.
		for row := 4950; row < 4990; row++ {
			h.classesFor(row)
		}
	}
}

// An edit keeps the line states above it, so a keystroke at the bottom of a long
// file does not re-read the whole file.
func TestHighlighterKeepsWhatAnEditCannotHaveChanged(t *testing.T) {
	lines := 2000
	text := strings.Repeat("local x = 1\n", lines)

	var h highlighter
	h.update(text)
	h.classesFor(lines - 2) // scroll to the bottom: all the states are worked out
	if h.known < lines-2 {
		t.Fatalf("known = %d", h.known)
	}

	// A change on the last line leaves everything above it alone.
	edited := text[:len(text)-len("local x = 1\n")] + "local x = 2\n"
	h.update(edited)
	if h.known < lines-2 {
		t.Errorf("an edit at the end threw away %d line states", lines-2-h.known)
	}

	// A change at the top invalidates what follows it.
	h.update("local y = 1\n" + edited[len("local x = 1\n"):])
	if h.known > 2 {
		t.Errorf("an edit at the top kept %d line states", h.known)
	}
}

// A long bracket opened above a change still colours the lines below it.
func TestHighlighterFollowsALongStringAcrossAnEdit(t *testing.T) {
	var h highlighter
	h.update("local s = [[\nstill inside\n]]\nlocal x = 1\n")

	_, classes := h.classesFor(1)
	if classes[0] != classString {
		t.Errorf("line 2 is %v, want part of the long string", classes[0])
	}

	// Close the bracket on the first line: the line below stops being a string.
	h.update("local s = [[]]\nstill inside\n]]\nlocal x = 1\n")
	_, classes = h.classesFor(1)
	if classes[0] == classString {
		t.Error("line 2 is still a string after the bracket above it closed")
	}
}

func BenchmarkHighlightAfterEditAtTheEnd(b *testing.B) {
	const line = "local function handler(request) return string.format('%d', request.id) end\n"
	text := strings.Repeat(line, 5000)

	var h highlighter
	h.update(text)
	h.classesFor(4999)

	edits := make([]string, 8)
	for i := range edits {
		edits[i] = text[:len(text)-len(line)] + "local x = " + string(rune('0'+i)) + "\n"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.update(edits[i%len(edits)])
		for row := 4960; row < 4999; row++ {
			h.classesFor(row)
		}
	}
}
