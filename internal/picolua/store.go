package picolua

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// Saving what a program wants to keep: a high score, where the player had got
// to, which options they chose.
//
// store(name, value) writes it and fetch(name) reads it back, which is the pair
// Picotron has. What is written is text — a Lua table as it would be typed —
// because a save file that can be read, diffed and fixed by hand is worth more
// here than a compact one, and there is no version of this that is not going to
// be looked at by whoever is writing the game.

// storeHeader marks a file as one of ours, so that fetch can tell a saved value
// from a text file the game was shipped with, and give back a table for the one
// and a string for the other.
const storeHeader = "-- tlua store 1"

/* --- writing --- */

// encode turns a value into the text that will be saved. Only what an editor
// or a program can sensibly write back out is allowed: a function has nowhere
// to go, and a table inside itself has no end.
func encode(v lua.LValue) (string, error) {
	var b strings.Builder
	b.WriteString(storeHeader)
	b.WriteByte('\n')
	if err := encodeValue(&b, v, 0, map[*lua.LTable]bool{}); err != nil {
		return "", err
	}
	b.WriteByte('\n')
	return b.String(), nil
}

func encodeValue(b *strings.Builder, v lua.LValue, depth int, seen map[*lua.LTable]bool) error {
	switch v := v.(type) {
	case lua.LString:
		b.WriteString(strconv.Quote(string(v)))
	case lua.LNumber:
		b.WriteString(numberText(float64(v)))
	case lua.LBool:
		b.WriteString(fmt.Sprint(bool(v)))
	case *lua.LNilType:
		b.WriteString("nil")
	case *lua.LTable:
		return encodeTable(b, v, depth, seen)
	default:
		return fmt.Errorf("a %s cannot be saved", v.Type())
	}
	return nil
}

// encodeTable writes a table the way it would be typed: the numbered part in
// order, then everything else by name, so that saving the same thing twice
// gives the same file both times.
func encodeTable(b *strings.Builder, t *lua.LTable, depth int, seen map[*lua.LTable]bool) error {
	if seen[t] {
		return fmt.Errorf("this table holds itself, and a file has to end somewhere")
	}
	if depth > 32 {
		return fmt.Errorf("these tables are nested deeper than anything worth saving")
	}
	seen[t] = true
	defer delete(seen, t)

	indent := strings.Repeat("\t", depth+1)
	b.WriteString("{\n")

	n := 0
	for t.RawGetInt(n+1) != lua.LNil {
		n++
	}
	for i := 1; i <= n; i++ {
		b.WriteString(indent)
		if err := encodeValue(b, t.RawGetInt(i), depth+1, seen); err != nil {
			return err
		}
		b.WriteString(",\n")
	}

	// Everything the numbered part did not cover, in an order that does not
	// depend on how Lua happens to be holding the table today.
	var keys []lua.LValue
	t.ForEach(func(k, _ lua.LValue) {
		if num, ok := k.(lua.LNumber); ok && float64(num) == float64(int(num)) &&
			int(num) >= 1 && int(num) <= n {
			return // already written above
		}
		keys = append(keys, k)
	})
	sort.Slice(keys, func(i, j int) bool { return keyText(keys[i]) < keyText(keys[j]) })

	for _, k := range keys {
		switch k.(type) {
		case lua.LString, lua.LNumber:
		default:
			return fmt.Errorf("a table with a %s for a key cannot be saved", k.Type())
		}
		b.WriteString(indent)
		b.WriteString(keyText(k))
		b.WriteString(" = ")
		if err := encodeValue(b, t.RawGet(k), depth+1, seen); err != nil {
			return err
		}
		b.WriteString(",\n")
	}

	b.WriteString(strings.Repeat("\t", depth))
	b.WriteString("}")
	return nil
}

// keyText writes a key the way it would be typed: a plain name where that is
// what it is, and in brackets where it is not.
func keyText(k lua.LValue) string {
	if s, ok := k.(lua.LString); ok && isName(string(s)) {
		return string(s)
	}
	if n, ok := k.(lua.LNumber); ok {
		return "[" + numberText(float64(n)) + "]"
	}
	return "[" + strconv.Quote(lua.LVAsString(k)) + "]"
}

// isName reports whether a string could be written as a bare table key.
func isName(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return !luaKeyword[s]
}

var luaKeyword = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true,
	"end": true, "false": true, "for": true, "function": true, "if": true,
	"in": true, "local": true, "nil": true, "not": true, "or": true,
	"repeat": true, "return": true, "then": true, "true": true, "until": true,
	"while": true,
}

// numberText writes a number without the exponents and trailing zeros a
// general purpose formatter would leave behind.
func numberText(f float64) string {
	if f == float64(int64(f)) && f < 1e15 && f > -1e15 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

/* --- reading --- */

// stored reports whether a file is a saved value rather than something else the
// game shipped with.
func stored(data []byte) bool {
	return strings.HasPrefix(string(data), storeHeader)
}

// decode reads back what encode wrote.
//
// The text is parsed rather than run. It is a file on somebody's disk that a
// program is about to trust, and handing it to the interpreter would make a
// save file a place to put code. It costs a few hundred lines not to.
func decode(L *lua.LState, text string) (lua.LValue, error) {
	p := &parser{src: strings.TrimPrefix(text, storeHeader), L: L}
	p.space()
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	p.space()
	if p.pos < len(p.src) {
		return nil, p.errorf("there is more after the value than there should be")
	}
	return v, nil
}

type parser struct {
	src string
	pos int
	L   *lua.LState
}

func (p *parser) errorf(format string, args ...any) error {
	line := 1 + strings.Count(p.src[:p.pos], "\n")
	return fmt.Errorf("line %d: %s", line, fmt.Sprintf(format, args...))
}

// space steps over whitespace and comments.
func (p *parser) space() {
	for p.pos < len(p.src) {
		switch c := p.src[p.pos]; {
		case c == ' ', c == '\t', c == '\r', c == '\n':
			p.pos++
		case strings.HasPrefix(p.src[p.pos:], "--"):
			if i := strings.IndexByte(p.src[p.pos:], '\n'); i >= 0 {
				p.pos += i + 1
			} else {
				p.pos = len(p.src)
			}
		default:
			return
		}
	}
}

func (p *parser) value() (lua.LValue, error) {
	if p.pos >= len(p.src) {
		return nil, p.errorf("the file stops where a value should be")
	}
	switch c := p.src[p.pos]; {
	case c == '{':
		return p.table()
	case c == '"':
		s, err := p.text()
		if err != nil {
			return nil, err
		}
		return lua.LString(s), nil
	case c == '-' || c == '.' || (c >= '0' && c <= '9'):
		return p.number()
	case strings.HasPrefix(p.src[p.pos:], "true"):
		p.pos += 4
		return lua.LTrue, nil
	case strings.HasPrefix(p.src[p.pos:], "false"):
		p.pos += 5
		return lua.LFalse, nil
	case strings.HasPrefix(p.src[p.pos:], "nil"):
		p.pos += 3
		return lua.LNil, nil
	}
	return nil, p.errorf("%q is not the start of anything that can be saved", p.src[p.pos])
}

func (p *parser) table() (lua.LValue, error) {
	p.pos++ // {
	t := p.L.NewTable()
	for {
		p.space()
		if p.pos >= len(p.src) {
			return nil, p.errorf("this table is never closed")
		}
		if p.src[p.pos] == '}' {
			p.pos++
			return t, nil
		}

		key, err := p.key()
		if err != nil {
			return nil, err
		}
		p.space()
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		if key == nil {
			t.Append(v)
		} else {
			t.RawSet(key, v)
		}

		p.space()
		if p.pos < len(p.src) && (p.src[p.pos] == ',' || p.src[p.pos] == ';') {
			p.pos++
		}
	}
}

// key reads the "name =" or "[k] =" in front of a value, and reports nothing at
// all for a value that is simply the next one in the list.
func (p *parser) key() (lua.LValue, error) {
	start := p.pos
	if p.src[p.pos] == '[' {
		p.pos++
		p.space()
		k, err := p.value()
		if err != nil {
			return nil, err
		}
		p.space()
		if p.pos >= len(p.src) || p.src[p.pos] != ']' {
			return nil, p.errorf("this key is never closed")
		}
		p.pos++
		p.space()
		if p.pos >= len(p.src) || p.src[p.pos] != '=' {
			return nil, p.errorf("a key in brackets wants an = after it")
		}
		p.pos++
		return k, nil
	}

	// A bare name, but only when what follows it is an equals sign: "true" is
	// a value and `true = 1` is not something this ever wrote.
	end := p.pos
	for end < len(p.src) {
		c := p.src[end]
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || (end > p.pos && c >= '0' && c <= '9') {
			end++
			continue
		}
		break
	}
	if end > p.pos {
		rest := p.pos
		p.pos = end
		p.space()
		if p.pos < len(p.src) && p.src[p.pos] == '=' && (p.pos+1 >= len(p.src) || p.src[p.pos+1] != '=') {
			p.pos++
			return lua.LString(p.src[rest:end]), nil
		}
	}
	p.pos = start
	return nil, nil
}

func (p *parser) text() (string, error) {
	// strconv.Unquote wants the whole quoted string, so find where it ends:
	// the first unescaped quote.
	i := p.pos + 1
	for i < len(p.src) {
		switch p.src[i] {
		case '\\':
			i += 2
			continue
		case '"':
			out, err := strconv.Unquote(p.src[p.pos : i+1])
			if err != nil {
				return "", p.errorf("this text cannot be read back: %v", err)
			}
			p.pos = i + 1
			return out, nil
		case '\n':
			return "", p.errorf("this text runs off the end of the line")
		}
		i++
	}
	return "", p.errorf("this text is never closed")
}

func (p *parser) number() (lua.LValue, error) {
	end := p.pos
	for end < len(p.src) {
		c := p.src[end]
		if c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' || (c >= '0' && c <= '9') {
			end++
			continue
		}
		break
	}
	f, err := strconv.ParseFloat(p.src[p.pos:end], 64)
	if err != nil {
		return nil, p.errorf("%q is not a number", p.src[p.pos:end])
	}
	p.pos = end
	return lua.LNumber(f), nil
}

/* --- the calls --- */

// saveName reports the file a name is saved under, and refuses a name that
// would put it somewhere else.
//
// A program picks these names itself and is meant to say "scores", not a path:
// anything with a directory in it, or an extension that is not one a save
// wears, is a mistake worth pointing out rather than a file worth writing.
func saveName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("a save wants a name")
	}
	if strings.ContainsAny(name, `/\`) || name == ".." || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("%q: a save is a name, not a path", name)
	}
	if !strings.Contains(name, ".") {
		name += kindData.exts[0]
	}
	return name, nil
}

// installStore adds store(), and the part of fetch() that reads one back.
func (r *Runtime) installStore() {
	r.register(map[string]lua.LGFunction{
		// store(name, value) keeps something between one run of the game and
		// the next: a score, where the player had got to, what they chose.
		//
		//	store("scores", { 1200, 900, 80 })
		//	local scores = fetch("scores") or {}
		//
		// A table is written out as text that can be read and fixed by hand;
		// a string is written as it is. Where the file lands is the host's
		// business — beside the person's other saved games, not beside the
		// program, which may be somewhere they cannot write to.
		"store": func(L *lua.LState) int {
			name, err := saveName(L.CheckString(1))
			if err == nil {
				var text string
				if s, ok := L.Get(2).(lua.LString); ok {
					text = string(s)
				} else if text, err = encode(L.CheckAny(2)); err != nil {
					err = fmt.Errorf("%s: %w", name, err)
				}
				if err == nil {
					if r.write == nil {
						err = fmt.Errorf("there is nowhere to save: this program is not being run by a host that keeps files")
					} else if err = r.write(name, []byte(text)); err != nil {
						err = fmt.Errorf("saving %s: %w", name, err)
					}
				}
			}
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			L.Push(lua.LTrue)
			return 1
		},
	})
}

// readSaved reports what was stored under a name, if anything was.
func (r *Runtime) readSaved(name string) ([]byte, bool) {
	if r.readSave == nil {
		return nil, false
	}
	saved, err := saveName(name)
	if err != nil {
		return nil, false
	}
	data, err := r.readSave(saved)
	if err != nil {
		return nil, false
	}
	return data, true
}
