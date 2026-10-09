package lualib

// zlib is lua-zlib (brimworks), the zlib binding LuaRocks code expects,
// in Go:
//
//	local deflated = zlib.deflate()(data, "finish")
//	local inflated = zlib.inflate()(deflated)
//	local crc = zlib.crc32()(data)
//
// It also answers to lzlib's zlib.compress(s [, level [, method [,
// windowBits]]]) and zlib.decompress(s [, windowBits]), and, tlua's own,
// zlib.gzip(s [, level]) and zlib.gunzip(s), and zlib.crc32(s) and
// zlib.adler32(s) for a string's checksum at once.
//
// windowBits says the format, as zlib's does: 8 to 15 is zlib's own, -8 to
// -15 raw deflate, 16 more than that gzip, and to inflate, 32 more detects
// zlib or gzip from the header. The window size itself is Go's, always 15.

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"hash/crc32"
	"io"

	lua "github.com/yuin/gopher-lua"
)

const zlibVersion = "1.3.1"

func openZlib(L *lua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"deflate":    zlibDeflate,
		"inflate":    zlibInflate,
		"compress":   zlibCompress,
		"decompress": zlibDecompress,
		"gzip":       zlibGzip,
		"gunzip":     zlibGunzip,
		"crc32": zlibChecksum(0, func(sum uint32, data []byte) uint32 {
			return crc32.Update(sum, crc32.IEEETable, data)
		}),
		"adler32": zlibChecksum(1, adler32Update),
		"version": func(L *lua.LState) int { L.Push(lua.LString(zlibVersion)); return 1 },
	})
	L.SetField(mod, "_VERSION", lua.LString("lua-zlib 1.2 (tlua)"))
	L.Push(mod)
	return 1
}

// zlibFormat is what windowBits says the data is.
type zlibFormat int

const (
	formatZlib zlibFormat = iota
	formatRaw
	formatGzip
	formatAuto
)

func formatOf(L *lua.LState, bits int, inflate bool) zlibFormat {
	switch {
	case bits == 0 && inflate:
		return formatZlib // the window size from the header, which Go reads anyway
	case bits >= 8 && bits <= 15:
		return formatZlib
	case bits >= -15 && bits <= -8:
		return formatRaw
	case bits >= 24 && bits <= 31:
		return formatGzip
	case inflate && bits >= 40 && bits <= 47:
		return formatAuto
	}
	L.RaiseError("zlib: windowBits %d is not one zlib takes", bits)
	return formatZlib
}

// level reads a compression level: -1 is the default, 0 to 9 the rest.
func zlibLevel(L *lua.LState, n int) int {
	level := L.OptInt(n, flate.DefaultCompression)
	if level < -1 || level > 9 {
		L.ArgError(n, "level must be from -1 to 9")
	}
	return level
}

func newCompressor(w io.Writer, format zlibFormat, level int) io.WriteCloser {
	var c io.WriteCloser
	switch format {
	case formatRaw:
		c, _ = flate.NewWriter(w, level)
	case formatGzip:
		c, _ = gzip.NewWriterLevel(w, level)
	default:
		c, _ = zlib.NewWriterLevel(w, level)
	}
	return c
}

func compressAll(data []byte, format zlibFormat, level int) []byte {
	var buf bytes.Buffer
	c := newCompressor(&buf, format, level)
	c.Write(data)
	c.Close()
	return buf.Bytes()
}

// inflateAll reads a whole stream, and says how much of data it took, so
// that what follows the stream can be told apart.
func inflateAll(data []byte, format zlibFormat) (out []byte, used int, err error) {
	src := bytes.NewReader(data)
	if format == formatAuto {
		format = formatZlib
		if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
			format = formatGzip
		}
	}
	var r io.Reader
	switch format {
	case formatRaw:
		r = flate.NewReader(src)
	case formatGzip:
		gz, gerr := gzip.NewReader(src)
		if gerr != nil {
			return nil, 0, gerr
		}
		gz.Multistream(false)
		r = gz
	default:
		zr, zerr := zlib.NewReader(src)
		if zerr != nil {
			return nil, 0, zerr
		}
		r = zr
	}
	out, err = io.ReadAll(r)
	// flate reads ahead through a bufio.Reader only when its source is not
	// a ByteReader; a bytes.Reader is one, so what is left is exactly what
	// the stream did not use.
	return out, len(data) - src.Len(), err
}

// zlibDeflate is zlib.deflate([level [, windowBits]]): a stream that
// compresses what it is given, stream(input [, flush]) returning what came
// out so far, whether the stream is finished, and the bytes in and out.
// flush is "none" (the default), "sync", "full" or "finish".
func zlibDeflate(L *lua.LState) int {
	level := zlibLevel(L, 1)
	format := formatOf(L, L.OptInt(2, 15), false)
	var out bytes.Buffer
	c := newCompressor(&out, format, level)
	var in, total int64
	done := false
	L.Push(L.NewFunction(func(L *lua.LState) int {
		data := L.OptString(1, "")
		mode := L.OptString(2, "none")
		if done {
			L.RaiseError("zlib: the stream is finished")
		}
		c.Write([]byte(data))
		in += int64(len(data))
		switch mode {
		case "none":
		case "sync", "full":
			if f, ok := c.(interface{ Flush() error }); ok {
				f.Flush()
			}
		case "finish":
			c.Close()
			done = true
		default:
			L.ArgError(2, "flush must be none, sync, full or finish")
		}
		chunk := out.String()
		out.Reset()
		total += int64(len(chunk))
		L.Push(lua.LString(chunk))
		L.Push(lua.LBool(done))
		L.Push(lua.LNumber(in))
		L.Push(lua.LNumber(total))
		return 4
	}))
	return 1
}

// zlibInflate is zlib.inflate([windowBits]): a stream that takes compressed
// input in as many pieces as it comes in, stream(input) returning what can
// be decompressed so far, whether the stream has ended, and the bytes in
// and out. Input past the end of the stream is left alone.
func zlibInflate(L *lua.LState) int {
	format := formatOf(L, L.OptInt(1, 15), true)
	var pending []byte
	emitted, consumed := 0, 0
	done := false
	L.Push(L.NewFunction(func(L *lua.LState) int {
		if done {
			L.Push(lua.LString(""))
			L.Push(lua.LTrue)
			L.Push(lua.LNumber(consumed))
			L.Push(lua.LNumber(emitted))
			return 4
		}
		pending = append(pending, L.OptString(1, "")...)
		// Each piece is read from the start of the stream again: simple,
		// and as fast as anyone needs who hands over the data in one go.
		out, used, err := inflateAll(pending, format)
		switch {
		case err == nil:
			done = true
			consumed = used
		case errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF):
			// Not all of it is here yet.
		default:
			L.RaiseError("zlib: %v", err)
		}
		if len(out) < emitted {
			out = out[:emitted]
		}
		chunk := out[emitted:]
		emitted = len(out)
		in := consumed
		if !done {
			in = len(pending)
		}
		L.Push(lua.LString(chunk))
		L.Push(lua.LBool(done))
		L.Push(lua.LNumber(in))
		L.Push(lua.LNumber(emitted))
		return 4
	}))
	return 1
}

// zlibCompress is lzlib's compress(s [, level [, method [, windowBits]]]).
func zlibCompress(L *lua.LState) int {
	data := L.CheckString(1)
	level := zlibLevel(L, 2)
	format := formatOf(L, L.OptInt(4, 15), false)
	L.Push(lua.LString(compressAll([]byte(data), format, level)))
	return 1
}

// zlibDecompress is lzlib's decompress(s [, windowBits]), which reads zlib
// or gzip by default; nil and why for data that is not either.
func zlibDecompress(L *lua.LState) int {
	data := L.CheckString(1)
	bits := L.OptInt(2, 47)
	out, _, err := inflateAll([]byte(data), formatOf(L, bits, true))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LString(out))
	return 1
}

func zlibGzip(L *lua.LState) int {
	data := L.CheckString(1)
	L.Push(lua.LString(compressAll([]byte(data), formatGzip, zlibLevel(L, 2))))
	return 1
}

func zlibGunzip(L *lua.LState) int {
	out, _, err := inflateAll([]byte(L.CheckString(1)), formatGzip)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LString(out))
	return 1
}

// zlibChecksum makes crc32 and adler32, which answer three ways:
//
//	zlib.crc32("data")         -- the checksum of a string (tlua's own)
//	zlib.crc32(crc, "more")    -- a running checksum carried on (lzlib)
//	zlib.crc32([init])(data)   -- a function that keeps a running one (lua-zlib)
func zlibChecksum(empty uint32, carry func(sum uint32, data []byte) uint32) lua.LGFunction {
	return func(L *lua.LState) int {
		if s, ok := L.Get(1).(lua.LString); ok && L.GetTop() == 1 {
			L.Push(lua.LNumber(carry(empty, []byte(s))))
			return 1
		}
		if s, ok := L.Get(2).(lua.LString); ok {
			L.Push(lua.LNumber(carry(uint32(L.CheckNumber(1)), []byte(s))))
			return 1
		}
		sum := empty
		if n, ok := L.Get(1).(lua.LNumber); ok {
			sum = uint32(n)
		}
		L.Push(L.NewFunction(func(L *lua.LState) int {
			if s, ok := L.Get(1).(lua.LString); ok {
				sum = carry(sum, []byte(s))
			}
			L.Push(lua.LNumber(sum))
			return 1
		}))
		return 1
	}
}

// adler32Update carries an Adler-32 checksum on over more data, as zlib's
// adler32(adler, buf, len) does; Go's package starts one only afresh.
func adler32Update(adler uint32, data []byte) uint32 {
	const mod = 65521
	a, b := adler&0xffff, adler>>16
	for _, c := range data {
		a = (a + uint32(c)) % mod
		b = (b + a) % mod
	}
	return b<<16 | a
}
