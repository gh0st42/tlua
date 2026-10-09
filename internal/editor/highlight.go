package editor

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"tlua/internal/luasyntax"
)

// The scanner is luasyntax's, which the gui module's code TextBox shares;
// these are the names this package knew it by.
type tokenClass = luasyntax.Class

const (
	classText    = luasyntax.Text
	classKeyword = luasyntax.Keyword
	classBuiltin = luasyntax.Builtin
	classString  = luasyntax.String
	classNumber  = luasyntax.Number
	classComment = luasyntax.Comment
)

type lineState = luasyntax.State

func scan(line string, state lineState, classes []tokenClass) lineState {
	return luasyntax.Scan(line, state, classes)
}

// Each class gets one of the sixteen EGA colours on the blue desktop, in the
// spirit of the Borland IDEs: white reserved words, grey comments, yellow
// strings.
var classStyles = [...]tcell.Style{
	classText:    textStyle,
	classKeyword: textStyle.Foreground(egaWhite).Bold(true),
	classBuiltin: textStyle.Foreground(egaLightGreen),
	classString:  textStyle.Foreground(egaYellow),
	classNumber:  textStyle.Foreground(egaLightCyan),
	classComment: textStyle.Foreground(egaDarkGray),
}

// highlighter keeps what a buffer's text costs to work out: the lines, the state
// each one starts in, and a slice to classify a line into.
//
// An edit keeps whatever is still true. The states above the first line that
// changed cannot have changed either, so they stay, and a keystroke at the
// bottom of a long file re-reads one line rather than all of them.
type highlighter struct {
	text    string
	lines   []string
	states  []lineState // states[i] is the state line i starts in
	known   int         // how many entries of states are filled in
	classes []tokenClass
}

// update takes the buffer's text as it now is.
func (h *highlighter) update(text string) {
	if h.lines != nil && text == h.text {
		return
	}
	lines := strings.Split(text, "\n")

	// How far down is the text the same? Everything above the first changed
	// line keeps the state it had.
	same := 0
	for same < len(h.lines) && same < len(lines) && h.lines[same] == lines[same] {
		same++
	}

	h.text, h.lines = text, lines
	if cap(h.states) >= len(lines) {
		h.states = h.states[:len(lines)]
	} else {
		states := make([]lineState, len(lines))
		copy(states, h.states)
		h.states = states
	}
	h.states[0] = lineState{}
	if h.known > same+1 {
		h.known = same + 1
	}
	if h.known < 1 {
		h.known = 1
	}
	if h.known > len(h.lines) {
		h.known = len(h.lines)
	}
}

// classesFor returns a line and the class of each of its bytes, scanning down
// from the last line whose state is already known.
func (h *highlighter) classesFor(line int) (string, []tokenClass) {
	if line < 0 || line >= len(h.lines) {
		return "", nil
	}
	for h.known <= line {
		h.states[h.known] = scan(h.lines[h.known-1], h.states[h.known-1], nil)
		h.known++
	}

	text := h.lines[line]
	if cap(h.classes) < len(text) {
		h.classes = make([]tokenClass, len(text)+64)
	}
	classes := h.classes[:len(text)]
	scan(text, h.states[line], classes)
	return text, classes
}
