// Package luasyntax classifies Lua source for colouring: keywords, the
// standard library's names, strings, numbers and comments, a line at a time,
// with what a line leaves open (a long string or comment) carried to the
// next. The terminal editor and the gui module's code TextBox both colour
// with it.
package luasyntax

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

// scan classifies one line and reports the state the next line starts in.
//
// It works in bytes rather than runes and writes into a slice the caller owns,
// so a redraw allocates nothing; classes may be nil, which is how the line
// states above the window are worked out without paying for their colours.
// Each byte of a character carries that character's class, which is what the
// drawing loop wants: it walks bytes and looks up the class of the one it is on.
func scan(line string, state lineState, classes []tokenClass) lineState {
	set := func(from, to int, class tokenClass) {
		if classes == nil {
			return
		}
		if to > len(classes) {
			to = len(classes)
		}
		for i := from; i < to; i++ {
			classes[i] = class
		}
	}

	i := 0
	// A long bracket left open above continues here.
	if state.inLong {
		class := classString
		if state.isComment {
			class = classComment
		}
		end := closeLongBracket(line, 0, state.longLevel)
		if end < 0 {
			set(0, len(line), class)
			return state
		}
		set(0, end, class)
		i = end
		state = lineState{}
	}

	for i < len(line) {
		switch c := line[i]; {
		case c == '-' && i+1 < len(line) && line[i+1] == '-':
			// A comment: either long, and possibly running past this line, or
			// to the end of the line.
			if level, ok := openLongBracket(line, i+2); ok {
				end := closeLongBracket(line, i+2+level+2, level)
				if end < 0 {
					set(i, len(line), classComment)
					return lineState{inLong: true, longLevel: level, isComment: true}
				}
				set(i, end, classComment)
				i = end
				continue
			}
			set(i, len(line), classComment)
			return lineState{}

		case c == '[':
			if level, ok := openLongBracket(line, i); ok {
				end := closeLongBracket(line, i+level+2, level)
				if end < 0 {
					set(i, len(line), classString)
					return lineState{inLong: true, longLevel: level}
				}
				set(i, end, classString)
				i = end
				continue
			}
			set(i, i+1, classText)
			i++

		case c == '"' || c == '\'':
			end := closeQuote(line, i)
			set(i, end, classString)
			i = end

		case isDigit(c) || (c == '.' && i+1 < len(line) && isDigit(line[i+1])):
			end := endOfNumber(line, i)
			set(i, end, classNumber)
			i = end

		case isNameStartByte(c):
			end := i + 1
			for end < len(line) && isNameByte(line[end]) {
				end++
			}
			class := classText
			switch word := line[i:end]; {
			case luaKeywords[word]:
				class = classKeyword
			case luaBuiltins[word]:
				class = classBuiltin
			}
			set(i, end, class)
			i = end

		default:
			// Anything else, including a character outside ASCII, is ordinary
			// text; a multi-byte one is coloured byte by byte, which comes to
			// the same thing on screen.
			set(i, i+1, classText)
			i++
		}
	}
	return lineState{}
}

// openLongBracket reports the level of a "[", "[=[" ... opening at i.
func openLongBracket(line string, i int) (int, bool) {
	if i >= len(line) || line[i] != '[' {
		return 0, false
	}
	level := 0
	for i+1+level < len(line) && line[i+1+level] == '=' {
		level++
	}
	if i+1+level < len(line) && line[i+1+level] == '[' {
		return level, true
	}
	return 0, false
}

// closeLongBracket finds the end of a long bracket of the given level, or -1
// when it runs past the end of the line.
func closeLongBracket(line string, from, level int) int {
	for i := from; i < len(line); i++ {
		if line[i] != ']' {
			continue
		}
		found := 0
		for i+1+found < len(line) && line[i+1+found] == '=' {
			found++
		}
		if found == level && i+1+found < len(line) && line[i+1+found] == ']' {
			return i + level + 2
		}
	}
	return -1
}

// closeQuote finds the end of a quoted string, honouring backslash escapes. An
// unterminated string simply colours to the end of the line.
func closeQuote(line string, start int) int {
	quote := line[start]
	for i := start + 1; i < len(line); i++ {
		switch line[i] {
		case '\\':
			i++
		case quote:
			return i + 1
		}
	}
	return len(line)
}

// endOfNumber consumes a Lua numeral, including hex and exponents.
func endOfNumber(line string, start int) int {
	i := start
	for i < len(line) {
		c := line[i]
		switch {
		case isDigit(c) || c == '.' || isHexLetter(c) || c == 'x' || c == 'X':
			i++
		case (c == '+' || c == '-') && i > start && isExponent(line[i-1]):
			i++
		default:
			return i
		}
	}
	return i
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isExponent(c byte) bool { return c == 'e' || c == 'E' || c == 'p' || c == 'P' }

func isHexLetter(c byte) bool {
	return (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || isExponent(c)
}

func isNameStartByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isNameByte(c byte) bool { return isNameStartByte(c) || isDigit(c) }

// Class is what a run of characters is, as far as colour is concerned.
type Class = tokenClass

// The classes.
const (
	Text    = classText
	Keyword = classKeyword
	Builtin = classBuiltin
	String  = classString
	Number  = classNumber
	Comment = classComment
)

// State is what a line inherits from the one above it.
type State = lineState

// Scan classifies one line into classes, which may be nil, and returns the
// state the next line starts in.
func Scan(line string, state State, classes []Class) State {
	return scan(line, state, classes)
}

// Classes returns the class of every byte of a whole text; a newline is
// Text.
func Classes(text string) []Class {
	out := make([]Class, len(text))
	state := State{}
	start := 0
	for start <= len(text) {
		end := start
		for end < len(text) && text[end] != '\n' {
			end++
		}
		state = scan(text[start:end], state, out[start:end])
		start = end + 1
	}
	return out
}
