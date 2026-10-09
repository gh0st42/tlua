package lualib

// zip is tlua's own: zip archives read and written.
//
//	local zip = require "zip"
//	local arc = assert(zip.open("data.zip"))    -- or zip.load(bytes)
//	for _, name in ipairs(arc:list()) do print(name, arc:info(name).size) end
//	local text = arc:read("readme.txt")
//
// It reads as LuaZip does too, so code written for that rock runs:
// arc:files() iterates over tables with filename, uncompressed_size and
// compressed_size, and arc:open(name) is a file with read and close.
//
//	local out = zip.create("new.zip")           -- or zip.create() in memory
//	out:add("hello.txt", "hello\n")
//	out:add("pic.png", img:encode(), { store = true })
//	out:close()                                 -- the bytes, when in memory

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"time"

	lua "github.com/yuin/gopher-lua"
)

const (
	zipReaderType = "zip.archive"
	zipWriterType = "zip.writer"
)

type zipReader struct {
	r     *zip.Reader
	files map[string]*zip.File
	names []string
}

type zipWriter struct {
	buf    bytes.Buffer
	w      *zip.Writer
	path   string // "" for one made in memory
	closed bool
}

func openZip(L *lua.LState) int {
	rmt := L.NewTypeMetatable(zipReaderType)
	L.SetField(rmt, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"list":   zipList,
		"files":  zipIterate,
		"open":   zipOpenFile,
		"info":   zipInfo,
		"exists": zipExists,
		"read":   zipRead,
		"close":  func(L *lua.LState) int { return 0 },
	}))
	wmt := L.NewTypeMetatable(zipWriterType)
	L.SetField(wmt, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"add":   zipAdd,
		"close": zipClose,
	}))
	L.Push(L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"open":   zipOpen,
		"load":   zipLoad,
		"create": zipCreate,
	}))
	return 1
}

func pushZipReader(L *lua.LState, data []byte) int {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	z := &zipReader{r: r, files: map[string]*zip.File{}}
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		z.files[f.Name] = f
		z.names = append(z.names, f.Name)
	}
	ud := L.NewUserData()
	ud.Value = z
	L.SetMetatable(ud, L.GetTypeMetatable(zipReaderType))
	L.Push(ud)
	return 1
}

// zipOpen is zip.open(path): the archive in a file, or nil and why. A fused
// program or a bundle reads its own files first.
func zipOpen(L *lua.LState) int {
	data, err := readFile(L, L.CheckString(1))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	return pushZipReader(L, data)
}

// zipLoad is zip.load(data): the archive in a string.
func zipLoad(L *lua.LState) int {
	return pushZipReader(L, []byte(L.CheckString(1)))
}

func checkZipReader(L *lua.LState) *zipReader {
	ud := L.CheckUserData(1)
	z, ok := ud.Value.(*zipReader)
	if !ok {
		L.ArgError(1, "zip archive expected")
	}
	return z
}

// zipList is arc:list(): the names of the files in it, in its order;
// folders are not listed.
func zipList(L *lua.LState) int {
	z := checkZipReader(L)
	t := L.NewTable()
	for _, n := range z.names {
		t.Append(lua.LString(n))
	}
	L.Push(t)
	return 1
}

// zipIterate is arc:files(), as LuaZip has it: an iterator over a table
// for each file, with LuaZip's filename, uncompressed_size and
// compressed_size, and info's fields besides.
func zipIterate(L *lua.LState) int {
	z := checkZipReader(L)
	i := 0
	L.Push(L.NewFunction(func(L *lua.LState) int {
		if i >= len(z.names) {
			L.Push(lua.LNil)
			return 1
		}
		f := z.files[z.names[i]]
		i++
		t := fileInfo(L, f)
		t.RawSetString("filename", lua.LString(f.Name))
		t.RawSetString("uncompressed_size", lua.LNumber(f.UncompressedSize64))
		t.RawSetString("compressed_size", lua.LNumber(f.CompressedSize64))
		L.Push(t)
		return 1
	}))
	return 1
}

// zipOpenFile is arc:open(name), as LuaZip has it: the file to read, with
// read("*a"), read("*l"), read(n), lines() and close(); nil and why.
func zipOpenFile(L *lua.LState) int {
	z := checkZipReader(L)
	name := L.CheckString(2)
	f, ok := z.files[name]
	if !ok {
		L.Push(lua.LNil)
		L.Push(lua.LString("no file '" + name + "' in the archive"))
		return 2
	}
	rc, err := f.Open()
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	pos := 0
	readLine := func() (string, bool) {
		if pos >= len(data) {
			return "", false
		}
		end := bytes.IndexByte(data[pos:], '\n')
		var line []byte
		if end < 0 {
			line, pos = data[pos:], len(data)
		} else {
			line, pos = data[pos:pos+end], pos+end+1
		}
		return string(line), true
	}
	file := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"read": func(L *lua.LState) int {
			format := L.Get(2)
			if n, ok := format.(lua.LNumber); ok {
				if pos >= len(data) {
					L.Push(lua.LNil)
					return 1
				}
				end := pos + int(n)
				if end > len(data) {
					end = len(data)
				}
				L.Push(lua.LString(data[pos:end]))
				pos = end
				return 1
			}
			switch lua.LVAsString(format) {
			case "*a", "a", "":
				if format == lua.LNil {
					if line, ok := readLine(); ok {
						L.Push(lua.LString(line))
					} else {
						L.Push(lua.LNil)
					}
					return 1
				}
				L.Push(lua.LString(data[pos:]))
				pos = len(data)
			case "*l", "l":
				if line, ok := readLine(); ok {
					L.Push(lua.LString(line))
				} else {
					L.Push(lua.LNil)
				}
			default:
				L.ArgError(2, "format must be *a, *l or a number")
			}
			return 1
		},
		"lines": func(L *lua.LState) int {
			L.Push(L.NewFunction(func(L *lua.LState) int {
				if line, ok := readLine(); ok {
					L.Push(lua.LString(line))
				} else {
					L.Push(lua.LNil)
				}
				return 1
			}))
			return 1
		},
		"seek":  func(L *lua.LState) int { L.Push(lua.LNumber(pos)); return 1 },
		"close": func(L *lua.LState) int { L.Push(lua.LTrue); return 1 },
	})
	L.Push(file)
	return 1
}

func fileInfo(L *lua.LState, f *zip.File) *lua.LTable {
	t := L.NewTable()
	t.RawSetString("name", lua.LString(f.Name))
	t.RawSetString("size", lua.LNumber(f.UncompressedSize64))
	t.RawSetString("compressed", lua.LNumber(f.CompressedSize64))
	t.RawSetString("modified", lua.LNumber(f.Modified.Unix()))
	method := "deflate"
	if f.Method == zip.Store {
		method = "store"
	}
	t.RawSetString("method", lua.LString(method))
	return t
}

// zipInfo is arc:info(name): size, compressed size, modified (as
// os.time() counts) and method ("deflate" or "store"), or nil.
func zipInfo(L *lua.LState) int {
	z := checkZipReader(L)
	f, ok := z.files[L.CheckString(2)]
	if !ok {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(fileInfo(L, f))
	return 1
}

func zipExists(L *lua.LState) int {
	z := checkZipReader(L)
	_, ok := z.files[L.CheckString(2)]
	L.Push(lua.LBool(ok))
	return 1
}

// zipRead is arc:read(name): the file's contents, or nil and why.
func zipRead(L *lua.LState) int {
	z := checkZipReader(L)
	name := L.CheckString(2)
	f, ok := z.files[name]
	if !ok {
		L.Push(lua.LNil)
		L.Push(lua.LString("no file '" + name + "' in the archive"))
		return 2
	}
	rc, err := f.Open()
	if err == nil {
		var data []byte
		data, err = io.ReadAll(rc)
		rc.Close()
		if err == nil {
			L.Push(lua.LString(data))
			return 1
		}
	}
	L.Push(lua.LNil)
	L.Push(lua.LString(err.Error()))
	return 2
}

// zipCreate is zip.create([path]): an archive to add files to, written to
// path when it is closed, or given back as a string with none.
func zipCreate(L *lua.LState) int {
	z := &zipWriter{path: L.OptString(1, "")}
	z.w = zip.NewWriter(&z.buf)
	ud := L.NewUserData()
	ud.Value = z
	L.SetMetatable(ud, L.GetTypeMetatable(zipWriterType))
	L.Push(ud)
	return 1
}

func checkZipWriter(L *lua.LState) *zipWriter {
	ud := L.CheckUserData(1)
	z, ok := ud.Value.(*zipWriter)
	if !ok {
		L.ArgError(1, "zip writer expected")
	}
	if z.closed {
		L.RaiseError("zip: the archive is closed")
	}
	return z
}

// zipAdd is out:add(name, data [, {store = true, modified = time}]): a
// file, compressed unless store says not to, dated now unless modified
// (as os.time() counts) says otherwise.
func zipAdd(L *lua.LState) int {
	z := checkZipWriter(L)
	name, data := L.CheckString(2), L.CheckString(3)
	hdr := &zip.FileHeader{Name: filepath.ToSlash(name), Method: zip.Deflate, Modified: time.Now()}
	if opts, ok := L.Get(4).(*lua.LTable); ok {
		if lua.LVAsBool(opts.RawGetString("store")) {
			hdr.Method = zip.Store
		}
		if t, ok := opts.RawGetString("modified").(lua.LNumber); ok {
			hdr.Modified = time.Unix(int64(t), 0)
		}
	}
	w, err := z.w.CreateHeader(hdr)
	if err == nil {
		_, err = w.Write([]byte(data))
	}
	if err != nil {
		L.RaiseError("zip: %v", err)
	}
	return 0
}

// zipClose is out:close(): true once the file is written, or the archive's
// bytes for one made in memory; nil and why if it could not be written.
func zipClose(L *lua.LState) int {
	z := checkZipWriter(L)
	z.closed = true
	if err := z.w.Close(); err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	if z.path == "" {
		L.Push(lua.LString(z.buf.String()))
		return 1
	}
	// Through a temporary file, so a failure never leaves half an archive.
	tmp := z.path + ".tmp"
	err := os.WriteFile(tmp, z.buf.Bytes(), 0o644)
	if err == nil {
		err = os.Rename(tmp, z.path)
	}
	if err != nil {
		os.Remove(tmp)
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LTrue)
	return 1
}
