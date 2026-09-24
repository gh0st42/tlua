package editor

import (
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// tokenClass is what a run of characters is, as far as colour is concerned.
type tokenClass uint8

const (
	classText tokenClass = iota
	classKeyword
	classBuiltin
	classString
	classNumber
	classComment
)

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

var luaKeywords = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true,
	"end": true, "false": true, "for": true, "function": true, "if": true,
	"in": true, "local": true, "nil": true, "not": true, "or": true,
	"repeat": true, "return": true, "then": true, "true": true, "until": true,
	"while": true,
}

// luaBuiltins are the names the standard library puts in a fresh state, plus
// self, which reads as one in a method.
var luaBuiltins = map[string]bool{
	"assert": true, "collectgarbage": true, "coroutine": true, "debug": true,
	"dofile": true, "error": true, "getmetatable": true, "io": true,
	"ipairs": true, "load": true, "loadfile": true, "loadstring": true,
	"math": true, "next": true, "os": true, "package": true, "pairs": true,
	"pcall": true, "print": true, "rawequal": true, "rawget": true,
	"rawset": true, "require": true, "select": true, "self": true,
	"setmetatable": true, "string": true, "table": true, "tonumber": true,
	"tostring": true, "type": true, "unpack": true, "xpcall": true,
	"_G": true, "_VERSION": true,
}

// lineState is what a line inherits from the one above it: whether it starts
// inside a long bracket, and at which level.
type lineState struct {
	inLong    bool
	longLevel int
	isComment bool
}

// scanLine classifies one line's runes and reports the state the next line
// starts in. It is a plain left-to-right scan with no allocations beyond the
// classes slice, which is what keeps redrawing cheap.
func scanLine(runes []rune, state lineState) ([]tokenClass, lineState) {
	classes := make([]tokenClass, len(runes))
	i := 0

	// A long bracket left open above continues here.
	if state.inLong {
		class := classString
		if state.isComment {
			class = classComment
		}
		end := closeLongBracket(runes, 0, state.longLevel)
		if end < 0 {
			fill(classes, 0, len(runes), class)
			return classes, state
		}
		fill(classes, 0, end, class)
		i = end
		state = lineState{}
	}

	for i < len(runes) {
		r := runes[i]
		switch {
		case r == '-' && i+1 < len(runes) && runes[i+1] == '-':
			// A comment: either long, and possibly running past this line, or
			// to the end of the line.
			if level, ok := openLongBracket(runes, i+2); ok {
				end := closeLongBracket(runes, i+2+level+2, level)
				if end < 0 {
					fill(classes, i, len(runes), classComment)
					return classes, lineState{inLong: true, longLevel: level, isComment: true}
				}
				fill(classes, i, end, classComment)
				i = end
				continue
			}
			fill(classes, i, len(runes), classComment)
			return classes, lineState{}

		case r == '[':
			if level, ok := openLongBracket(runes, i); ok {
				end := closeLongBracket(runes, i+level+2, level)
				if end < 0 {
					fill(classes, i, len(runes), classString)
					return classes, lineState{inLong: true, longLevel: level}
				}
				fill(classes, i, end, classString)
				i = end
				continue
			}
			classes[i] = classText
			i++

		case r == '"' || r == '\'':
			end := closeQuote(runes, i)
			fill(classes, i, end, classString)
			i = end

		case unicode.IsDigit(r) || (r == '.' && i+1 < len(runes) && unicode.IsDigit(runes[i+1])):
			end := endOfNumber(runes, i)
			fill(classes, i, end, classNumber)
			i = end

		case isNameStart(r):
			end := i + 1
			for end < len(runes) && isNameRune(runes[end]) {
				end++
			}
			word := string(runes[i:end])
			class := classText
			switch {
			case luaKeywords[word]:
				class = classKeyword
			case luaBuiltins[word]:
				class = classBuiltin
			}
			fill(classes, i, end, class)
			i = end

		default:
			classes[i] = classText
			i++
		}
	}
	return classes, lineState{}
}

// openLongBracket reports the level of a "[", "[=[" ... opening at i.
func openLongBracket(runes []rune, i int) (int, bool) {
	if i >= len(runes) || runes[i] != '[' {
		return 0, false
	}
	level := 0
	for i+1+level < len(runes) && runes[i+1+level] == '=' {
		level++
	}
	if i+1+level < len(runes) && runes[i+1+level] == '[' {
		return level, true
	}
	return 0, false
}

// closeLongBracket finds the end of a long bracket of the given level, or -1
// when it runs past the end of the line.
func closeLongBracket(runes []rune, from, level int) int {
	for i := from; i < len(runes); i++ {
		if runes[i] != ']' {
			continue
		}
		found := 0
		for i+1+found < len(runes) && runes[i+1+found] == '=' {
			found++
		}
		if found == level && i+1+found < len(runes) && runes[i+1+found] == ']' {
			return i + level + 2
		}
	}
	return -1
}

// closeQuote finds the end of a quoted string, honouring backslash escapes. An
// unterminated string simply colours to the end of the line.
func closeQuote(runes []rune, start int) int {
	quote := runes[start]
	for i := start + 1; i < len(runes); i++ {
		switch runes[i] {
		case '\\':
			i++
		case quote:
			return i + 1
		}
	}
	return len(runes)
}

// endOfNumber consumes a Lua numeral, including hex and exponents.
func endOfNumber(runes []rune, start int) int {
	i := start
	for i < len(runes) {
		r := runes[i]
		switch {
		case unicode.IsDigit(r) || r == '.' || isHexLetter(r) || r == 'x' || r == 'X':
			i++
		case (r == '+' || r == '-') && i > start && isExponent(runes[i-1]):
			i++
		default:
			return i
		}
	}
	return i
}

func isExponent(r rune) bool { return r == 'e' || r == 'E' || r == 'p' || r == 'P' }

func isHexLetter(r rune) bool {
	return (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') || isExponent(r)
}

func isNameStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isNameRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func fill(classes []tokenClass, from, to int, class tokenClass) {
	if to > len(classes) {
		to = len(classes)
	}
	for i := from; i < to; i++ {
		classes[i] = class
	}
}

// highlighter keeps what a buffer's text costs to work out: the lines, and the
// state each one starts in. Both are computed once per edit, and the line
// states only as far down as the screen has scrolled.
type highlighter struct {
	lines  []string
	states []lineState // states[i] is the state line i starts in
	known  int         // how many entries of states are filled in
}

// reset takes new text, throwing away what was derived from the old.
func (h *highlighter) reset(text string) {
	h.lines = strings.Split(text, "\n")
	if cap(h.states) >= len(h.lines) {
		h.states = h.states[:len(h.lines)]
	} else {
		h.states = make([]lineState, len(h.lines))
	}
	h.states[0] = lineState{}
	h.known = 1
}

// classesFor returns the colour of every rune on a line, scanning down from the
// last line whose state is already known.
func (h *highlighter) classesFor(line int) ([]rune, []tokenClass) {
	if line < 0 || line >= len(h.lines) {
		return nil, nil
	}
	for h.known <= line {
		_, next := scanLine([]rune(h.lines[h.known-1]), h.states[h.known-1])
		h.states[h.known] = next
		h.known++
	}
	runes := []rune(h.lines[line])
	classes, _ := scanLine(runes, h.states[line])
	return runes, classes
}
