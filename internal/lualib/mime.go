package lualib

import (
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// mime.core, the C half of LuaSocket's mime module, here in Go. mime.lua and
// ltn12.lua stay as they are, in Lua, and run on top of it unchanged.
//
// Every function is a filter step: it is handed the next chunk and gives back
// what can be output so far plus the state to carry into the next call, and a
// nil chunk means the input has ended. That shape, and every edge of the
// encodings, follows LuaSocket 3.1's mime.c line for line.

const (
	crlf   = "\r\n"
	eqcrlf = "=\r\n"
)

const b64base = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// b64unbase maps a character back to its six bits; '=' counts as a valid
// zero and anything above 64 is not base64 at all.
var b64unbase = func() (t [256]byte) {
	for i := range t {
		t[i] = 255
	}
	for i := 0; i < 64; i++ {
		t[b64base[i]] = byte(i)
	}
	t['='] = 0
	return
}()

const (
	qpPlain = iota
	qpQuoted
	qpCR
	qpIfLast
)

var qpclass = func() (t [256]byte) {
	for i := range t {
		t[i] = qpQuoted
	}
	for i := 33; i <= 60; i++ {
		t[i] = qpPlain
	}
	for i := 62; i <= 126; i++ {
		t[i] = qpPlain
	}
	t['\t'] = qpIfLast
	t[' '] = qpIfLast
	t['\r'] = qpCR
	return
}()

var qpunbase = func() (t [256]byte) {
	for i := range t {
		t[i] = 255
	}
	for i := 0; i < 10; i++ {
		t['0'+i] = byte(i)
	}
	for i := 0; i < 6; i++ {
		t['A'+i] = byte(10 + i)
		t['a'+i] = byte(10 + i)
	}
	return
}()

func openMimeCore(L *lua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"dot":   mimeDot,
		"b64":   mimeB64,
		"eol":   mimeEol,
		"qp":    mimeQp,
		"qpwrp": mimeQpwrp,
		"unb64": mimeUnb64,
		"unqp":  mimeUnqp,
		"wrp":   mimeWrp,
	})
	L.SetField(mod, "_VERSION", lua.LString("MIME 1.0.3"))
	L.Push(mod)
	return 1
}

// optString is luaL_optlstring: a string or a number, or nothing at all.
func optString(L *lua.LState, n int) (string, bool) {
	if L.Get(n) == lua.LNil {
		return "", false
	}
	return L.CheckString(n), true
}

// optStringDef is luaL_optstring with its default. gopher-lua's OptString
// turns a number away, which the C one takes as its digits.
func optStringDef(L *lua.LState, n int, def string) string {
	if s, ok := optString(L, n); ok {
		return s
	}
	return def
}

// atomFilter runs one of the four incremental codecs (b64, unb64, qp, unqp):
// the first chunk, then the second if there is one, through step, carrying
// the partial atom between bytes. Without a second chunk the input is over,
// and finish flushes what is left.
func atomFilter(L *lua.LState, step func(c byte, atom []byte, buf *strings.Builder) []byte, finish func(atom []byte, buf *strings.Builder)) int {
	first, ok := optString(L, 1)
	if !ok {
		L.Push(lua.LNil)
		L.Push(lua.LNil)
		return 2
	}
	var buf strings.Builder
	atom := make([]byte, 0, 4)
	for i := 0; i < len(first); i++ {
		atom = step(first[i], atom, &buf)
	}
	second, ok := optString(L, 2)
	if !ok {
		if finish != nil {
			finish(atom, &buf)
		}
		if buf.Len() == 0 {
			L.Push(lua.LNil)
		} else {
			L.Push(lua.LString(buf.String()))
		}
		L.Push(lua.LNil)
		return 2
	}
	for i := 0; i < len(second); i++ {
		atom = step(second[i], atom, &buf)
	}
	L.Push(lua.LString(buf.String()))
	L.Push(lua.LString(atom))
	return 2
}

func b64encode(c byte, atom []byte, buf *strings.Builder) []byte {
	atom = append(atom, c)
	if len(atom) == 3 {
		v := uint(atom[0])<<16 | uint(atom[1])<<8 | uint(atom[2])
		buf.WriteByte(b64base[v>>18&0x3f])
		buf.WriteByte(b64base[v>>12&0x3f])
		buf.WriteByte(b64base[v>>6&0x3f])
		buf.WriteByte(b64base[v&0x3f])
		atom = atom[:0]
	}
	return atom
}

func b64pad(atom []byte, buf *strings.Builder) {
	switch len(atom) {
	case 1:
		v := uint(atom[0]) << 4
		buf.WriteByte(b64base[v>>6&0x3f])
		buf.WriteByte(b64base[v&0x3f])
		buf.WriteString("==")
	case 2:
		v := (uint(atom[0])<<8 | uint(atom[1])) << 2
		buf.WriteByte(b64base[v>>12&0x3f])
		buf.WriteByte(b64base[v>>6&0x3f])
		buf.WriteByte(b64base[v&0x3f])
		buf.WriteByte('=')
	}
}

func b64decode(c byte, atom []byte, buf *strings.Builder) []byte {
	if b64unbase[c] > 64 {
		return atom
	}
	atom = append(atom, c)
	if len(atom) == 4 {
		v := uint(b64unbase[atom[0]])<<18 | uint(b64unbase[atom[1]])<<12 |
			uint(b64unbase[atom[2]])<<6 | uint(b64unbase[atom[3]])
		decoded := []byte{byte(v >> 16), byte(v >> 8), byte(v)}
		valid := 3
		if atom[2] == '=' {
			valid = 1
		} else if atom[3] == '=' {
			valid = 2
		}
		buf.Write(decoded[:valid])
		atom = atom[:0]
	}
	return atom
}

func mimeB64(L *lua.LState) int {
	return atomFilter(L, b64encode, b64pad)
}

func mimeUnb64(L *lua.LState) int {
	return atomFilter(L, b64decode, nil)
}

func qpquote(c byte, buf *strings.Builder) {
	const hex = "0123456789ABCDEF"
	buf.WriteByte('=')
	buf.WriteByte(hex[c>>4])
	buf.WriteByte(hex[c&0x0f])
}

// qpencoder needs to see up to two bytes past the one it is deciding on:
// a space is quoted only at the end of a line, and a CR is a line break only
// before an LF.
func qpencoder(marker string) func(c byte, atom []byte, buf *strings.Builder) []byte {
	return func(c byte, atom []byte, buf *strings.Builder) []byte {
		atom = append(atom, c)
		for len(atom) > 0 {
			switch qpclass[atom[0]] {
			case qpCR:
				if len(atom) < 2 {
					return atom
				}
				if atom[1] == '\n' {
					buf.WriteString(marker)
					return atom[:0]
				}
				qpquote(atom[0], buf)
			case qpIfLast:
				if len(atom) < 3 {
					return atom
				}
				if atom[1] == '\r' && atom[2] == '\n' {
					qpquote(atom[0], buf)
					buf.WriteString(marker)
					return atom[:0]
				}
				buf.WriteByte(atom[0])
			case qpQuoted:
				qpquote(atom[0], buf)
			default:
				buf.WriteByte(atom[0])
			}
			atom = append(atom[:0], atom[1:]...)
		}
		return atom
	}
}

func qppad(atom []byte, buf *strings.Builder) {
	for _, c := range atom {
		if qpclass[c] == qpPlain {
			buf.WriteByte(c)
		} else {
			qpquote(c, buf)
		}
	}
	if len(atom) > 0 {
		buf.WriteString(eqcrlf)
	}
}

func qpdecode(c byte, atom []byte, buf *strings.Builder) []byte {
	atom = append(atom, c)
	switch atom[0] {
	case '=':
		if len(atom) < 3 {
			return atom
		}
		if atom[1] == '\r' && atom[2] == '\n' {
			return atom[:0]
		}
		hi, lo := qpunbase[atom[1]], qpunbase[atom[2]]
		if hi > 15 || lo > 15 {
			buf.Write(atom)
		} else {
			buf.WriteByte(hi<<4 + lo)
		}
	case '\r':
		if len(atom) < 2 {
			return atom
		}
		if atom[1] == '\n' {
			buf.Write(atom)
		}
	default:
		if atom[0] == '\t' || (atom[0] > 31 && atom[0] < 127) {
			buf.WriteByte(atom[0])
		}
	}
	return atom[:0]
}

func mimeQp(L *lua.LState) int {
	marker := optStringDef(L, 3, crlf)
	return atomFilter(L, qpencoder(marker), qppad)
}

func mimeUnqp(L *lua.LState) int {
	return atomFilter(L, qpdecode, nil)
}

// wrapFilter is wrp and qpwrp: break text into lines of at most length
// bytes, given how much room the current line has left.
func wrapFilter(L *lua.LState, last string, wrap func(c byte, left int, length int, buf *strings.Builder) int) int {
	left := int(L.CheckNumber(1))
	input, ok := optString(L, 2)
	length := int(L.OptNumber(3, 76))
	if !ok {
		if left < length {
			L.Push(lua.LString(last))
		} else {
			L.Push(lua.LNil)
		}
		L.Push(lua.LNumber(length))
		return 2
	}
	var buf strings.Builder
	for i := 0; i < len(input); i++ {
		c := input[i]
		switch c {
		case '\r':
		case '\n':
			buf.WriteString(crlf)
			left = length
		default:
			left = wrap(c, left, length, &buf)
		}
	}
	L.Push(lua.LString(buf.String()))
	L.Push(lua.LNumber(left))
	return 2
}

func mimeWrp(L *lua.LState) int {
	return wrapFilter(L, crlf, func(c byte, left, length int, buf *strings.Builder) int {
		if left <= 0 {
			left = length
			buf.WriteString(crlf)
		}
		buf.WriteByte(c)
		return left - 1
	})
}

// mimeQpwrp breaks lines with soft line breaks, and never inside an =XX.
func mimeQpwrp(L *lua.LState) int {
	return wrapFilter(L, eqcrlf, func(c byte, left, length int, buf *strings.Builder) int {
		room := 1
		if c == '=' {
			room = 3
		}
		if left <= room {
			left = length
			buf.WriteString(eqcrlf)
		}
		buf.WriteByte(c)
		return left - 1
	})
}

// mimeEol turns any of CR, LF, CRLF and LFCR into marker; a doubled CR or
// LF is two line breaks. The context is the last candidate seen, if any.
func mimeEol(L *lua.LState) int {
	ctx := int(L.CheckInt(1))
	input, ok := optString(L, 2)
	marker := optStringDef(L, 3, crlf)
	if !ok {
		L.Push(lua.LNil)
		L.Push(lua.LNumber(0))
		return 2
	}
	candidate := func(c int) bool { return c == '\r' || c == '\n' }
	var buf strings.Builder
	for i := 0; i < len(input); i++ {
		c := int(input[i])
		switch {
		case !candidate(c):
			buf.WriteByte(byte(c))
			ctx = 0
		case candidate(ctx):
			if c == ctx {
				buf.WriteString(marker)
			}
			ctx = 0
		default:
			buf.WriteString(marker)
			ctx = c
		}
	}
	L.Push(lua.LString(buf.String()))
	L.Push(lua.LNumber(ctx))
	return 2
}

// mimeDot is SMTP's dot-stuffing: a '.' at the start of a line is doubled.
// The state counts how much of a CRLF has just been seen.
func mimeDot(L *lua.LState) int {
	state := int(L.CheckNumber(1))
	input, ok := optString(L, 2)
	if !ok {
		L.Push(lua.LNil)
		L.Push(lua.LNumber(2))
		return 2
	}
	var buf strings.Builder
	for i := 0; i < len(input); i++ {
		c := input[i]
		buf.WriteByte(c)
		switch c {
		case '\r':
			state = 1
		case '\n':
			if state == 1 {
				state = 2
			} else {
				state = 0
			}
		case '.':
			if state == 2 {
				buf.WriteByte('.')
			}
			state = 0
		default:
			state = 0
		}
	}
	L.Push(lua.LString(buf.String()))
	L.Push(lua.LNumber(state))
	return 2
}
