// Package payload reads and writes the program attached to a tlua binary.
//
// A tlua executable can carry a Lua program appended to its own image, the way
// a .love file is appended to the LÖVE runtime. Two shapes are recognised:
//
//  1. a payload written by `tlua fuse`, marked by a trailer at the very end of
//     the file, holding either a single Lua source file or a zip archive;
//  2. a bare zip simply concatenated onto the binary
//     (`cat tlua app.zip > app`), which is how LÖVE does it.
package payload

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	magic = "tlua-fuse-v1"
	// TrailerSize is the fixed footer `tlua fuse` writes: magic, kind and the
	// big-endian payload length.
	TrailerSize = len(magic) + 1 + 8

	// EntryName is the Lua file a fused archive starts from.
	EntryName = "main.lua"
)

// Kind says what shape a payload is, and how it wants to be run.
//
// The capital is an ordinary program, run as a script; the small letter is the
// same shape of program written for the console, which wants a window around
// it. Spelling the pair this way keeps the trailer the size it has always been,
// so a binary made by an older tlua still reads.
type Kind byte

const (
	Lua Kind = 'L' // a single Lua source file
	Zip Kind = 'Z' // a zip archive rooted at main.lua

	LuaGame Kind = 'l' // the same, for a program that wants a window
	ZipGame Kind = 'z'
)

// Shape reports the payload's shape with the question of how to run it set
// aside: Lua for a single file, Zip for an archive.
func (k Kind) Shape() Kind {
	switch k {
	case LuaGame:
		return Lua
	case ZipGame:
		return Zip
	}
	return k
}

// Game reports whether the program wants a window and the console API, which is
// what `tlua fuse -play` marks it as.
func (k Kind) Game() bool { return k == LuaGame || k == ZipGame }

// AsGame reports the kind that means the same shape, run as a game.
func (k Kind) AsGame() Kind {
	if k.Shape() == Zip {
		return ZipGame
	}
	return LuaGame
}

// Payload is the program attached to a binary.
type Payload struct {
	// Kind is the shape of the program and how it asked to be run; Kind.Shape()
	// and Kind.Game() take it apart.
	Kind Kind

	Source  []byte   // Lua
	Archive *Archive // Zip

	// PrefixLen is the size of the interpreter alone, i.e. where the payload
	// starts. `tlua fuse` uses it to reuse an already fused binary as a base.
	PrefixLen int64

	file *os.File // kept open for the lifetime of the process
}

// Open reports the program attached to the named binary, or nil when it is a
// plain interpreter.
func Open(exe string) (*Payload, error) {
	f, err := os.Open(exe)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	size := st.Size()

	if p, err := readTrailer(f, size); err != nil {
		f.Close()
		return nil, err
	} else if p != nil {
		p.file = f
		return p, nil
	}

	// No trailer: maybe a bare appended zip.
	if zr, err := zip.NewReader(f, size); err == nil {
		archive, err := NewArchive(zr)
		if err != nil {
			f.Close()
			return nil, err
		}
		start, err := zipStart(f, size)
		if err != nil {
			f.Close()
			return nil, err
		}
		return &Payload{Kind: Zip, Archive: archive, file: f, PrefixLen: start}, nil
	}

	f.Close()
	return nil, nil
}

func (p *Payload) Close() {
	if p != nil && p.file != nil {
		p.file.Close()
	}
}

// readTrailer looks for a `tlua fuse` marker at the end of the file.
func readTrailer(f *os.File, size int64) (*Payload, error) {
	if size < int64(TrailerSize) {
		return nil, nil
	}
	buf := make([]byte, TrailerSize)
	if _, err := f.ReadAt(buf, size-int64(TrailerSize)); err != nil {
		return nil, err
	}
	if !bytes.Equal(buf[:len(magic)], []byte(magic)) {
		return nil, nil
	}

	kind := Kind(buf[len(magic)])
	n := int64(binary.BigEndian.Uint64(buf[len(magic)+1:]))
	start := size - int64(TrailerSize) - n
	if n < 0 || start < 0 {
		return nil, errors.New("fused payload has a corrupt trailer")
	}

	switch kind.Shape() {
	case Lua:
		src := make([]byte, n)
		if _, err := f.ReadAt(src, start); err != nil {
			return nil, err
		}
		return &Payload{Kind: kind, Source: src, PrefixLen: start}, nil
	case Zip:
		zr, err := zip.NewReader(io.NewSectionReader(f, start, n), n)
		if err != nil {
			return nil, fmt.Errorf("reading fused archive: %w", err)
		}
		archive, err := NewArchive(zr)
		if err != nil {
			return nil, err
		}
		return &Payload{Kind: kind, Archive: archive, PrefixLen: start}, nil
	default:
		return nil, fmt.Errorf("fused payload has an unknown kind %q", rune(kind))
	}
}

// Trailer builds the footer that marks a payload of n bytes.
func Trailer(kind Kind, n int) []byte {
	t := make([]byte, 0, TrailerSize)
	t = append(t, magic...)
	t = append(t, byte(kind))
	return binary.BigEndian.AppendUint64(t, uint64(n))
}

// InterpreterPrefix reads a binary and strips any program already attached to
// it, so fusing on top of an app binary still works.
func InterpreterPrefix(base string) ([]byte, error) {
	data, err := os.ReadFile(base)
	if err != nil {
		return nil, fmt.Errorf("reading base interpreter: %w", err)
	}
	p, err := Open(base)
	if err != nil {
		return nil, fmt.Errorf("inspecting base interpreter %s: %w", base, err)
	}
	if p == nil {
		return data, nil
	}
	defer p.Close()
	if p.PrefixLen <= 0 || p.PrefixLen > int64(len(data)) {
		return nil, fmt.Errorf("%s already carries a program that cannot be stripped", base)
	}
	return data[:p.PrefixLen], nil
}

// zipStart finds where an appended zip begins, which is also the length of the
// interpreter in front of it.
func zipStart(r io.ReaderAt, size int64) (int64, error) {
	// The end-of-central-directory record lives in the last 64KiB + comment.
	const maxTail = 65536 + 22
	tail := size
	if tail > maxTail {
		tail = maxTail
	}
	buf := make([]byte, tail)
	if _, err := r.ReadAt(buf, size-tail); err != nil {
		return 0, err
	}
	idx := bytes.LastIndex(buf, []byte{'P', 'K', 0x05, 0x06})
	if idx < 0 || len(buf)-idx < 22 {
		return 0, errors.New("no end-of-central-directory record found")
	}
	rec := buf[idx:]
	cdSize := int64(binary.LittleEndian.Uint32(rec[12:16]))
	cdOffset := int64(binary.LittleEndian.Uint32(rec[16:20]))
	eocdAt := size - tail + int64(idx)
	// Where the archive claims its directory is, versus where it really is.
	return eocdAt - cdSize - cdOffset, nil
}
