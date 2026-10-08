package lualib

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// LuaFileSystem, the module nearly every Lua library that touches a directory
// is written against, here in Go. It answers to the same names with the same
// results as lfs 1.9, errors included: a failure gives back nil, the message
// strerror would have, and the errno.
//
// lock and unlock are the exception. They need the descriptor under a Lua
// file, which gopher-lua keeps to itself, so they report that they cannot.

const lfsVersion = "LuaFileSystem 1.9.0"

const dirClass = "directory metatable"
const lockClass = "lock metatable"

func openLfs(L *lua.LState) int {
	mt := L.NewTypeMetatable(dirClass)
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"next":  lfsDirNext,
		"close": lfsDirClose,
	}))
	L.SetField(mt, "__gc", L.NewFunction(lfsDirClose))
	L.SetField(mt, "__tostring", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString("directory"))
		return 1
	}))

	lock := L.NewTypeMetatable(lockClass)
	L.SetField(lock, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"free": lfsLockFree,
	}))
	L.SetField(lock, "__gc", L.NewFunction(lfsLockFree))

	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"attributes":        func(L *lua.LState) int { return lfsAttributes(L, os.Stat, false) },
		"symlinkattributes": func(L *lua.LState) int { return lfsAttributes(L, os.Lstat, true) },
		"chdir":             lfsChdir,
		"currentdir":        lfsCurrentdir,
		"dir":               lfsDir,
		"link":              lfsLink,
		"lock":              lfsLockFile,
		"unlock":            lfsLockFile,
		"lock_dir":          lfsLockDir,
		"mkdir":             lfsMkdir,
		"rmdir":             lfsRmdir,
		"setmode":           lfsSetmode,
		"touch":             lfsTouch,
	})
	L.SetField(mod, "_VERSION", lua.LString(lfsVersion))
	L.SetField(mod, "_COPYRIGHT", lua.LString("Copyright (C) 2003-2017 Kepler Project"))
	L.SetField(mod, "_DESCRIPTION", lua.LString("LuaFileSystem is a Lua library developed to complement the set of functions related to file systems offered by the standard Lua distribution"))
	L.Push(mod)
	return 1
}

// strerror turns a Go error from the os package back into what C would have
// said: the bare system message, capitalised the way libc spells it, and the
// errno beside it.
func strerror(err error) (string, int) {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		msg := errno.Error()
		if msg != "" {
			msg = strings.ToUpper(msg[:1]) + msg[1:]
		}
		return msg, int(errno)
	}
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error(), 0
	}
	return err.Error(), 0
}

// fail is the usual way a Lua library reports failure: nil, a message, and
// the errno when there is one.
func fail(L *lua.LState, err error) int {
	msg, errno := strerror(err)
	L.Push(lua.LNil)
	L.Push(lua.LString(msg))
	L.Push(lua.LNumber(errno))
	return 3
}

func ok(L *lua.LState) int {
	L.Push(lua.LTrue)
	return 1
}

// attr is one field of an attributes table, in the order lfs lists them.
type attr struct {
	name  string
	value lua.LValue
}

func lfsAttributes(L *lua.LState, stat func(string) (os.FileInfo, error), link bool) int {
	path := L.CheckString(1)
	if link && L.Get(2) == lua.LString("target") {
		// Asked for by name, the target is read first and on its own, and
		// a path that is not a link is an error.
		target, err := os.Readlink(path)
		if err != nil {
			msg, errno := strerror(err)
			L.Push(lua.LNil)
			L.Push(lua.LString("could not obtain link target: " + msg))
			L.Push(lua.LNumber(errno))
			return 3
		}
		L.Push(lua.LString(target))
		return 1
	}
	fi, err := stat(path)
	if err != nil {
		msg, errno := strerror(err)
		L.Push(lua.LNil)
		L.Push(lua.LString("cannot obtain information from file '" + path + "': " + msg))
		L.Push(lua.LNumber(errno))
		return 3
	}
	attrs := fileAttrs(path, fi, link)
	if link {
		target := lua.LValue(lua.LNil)
		if fi.Mode()&os.ModeSymlink != 0 {
			if t, err := os.Readlink(path); err == nil {
				target = lua.LString(t)
			}
		}
		attrs = append(attrs, attr{"target", target})
	}

	switch arg := L.Get(2).(type) {
	case lua.LString:
		for _, a := range attrs {
			if a.name == string(arg) {
				L.Push(a.value)
				return 1
			}
		}
		L.RaiseError("invalid attribute name '%s'", string(arg))
		return 0
	case *lua.LTable:
		for _, a := range attrs {
			L.SetField(arg, a.name, a.value)
		}
		L.Push(arg)
		return 1
	default:
		tbl := L.NewTable()
		for _, a := range attrs {
			L.SetField(tbl, a.name, a.value)
		}
		L.Push(tbl)
		return 1
	}
}

// portableAttrs is the part of an attributes table that os.FileInfo can
// answer for on any system; the rest is zero, as lfs leaves it on Windows.
func portableAttrs(fi os.FileInfo) []attr {
	mtime := lua.LNumber(fi.ModTime().Unix())
	return []attr{
		{"dev", lua.LNumber(0)},
		{"ino", lua.LNumber(0)},
		{"mode", lua.LString(modeName(fi.Mode()))},
		{"nlink", lua.LNumber(1)},
		{"uid", lua.LNumber(0)},
		{"gid", lua.LNumber(0)},
		{"rdev", lua.LNumber(0)},
		{"access", mtime},
		{"modification", mtime},
		{"change", mtime},
		{"size", lua.LNumber(fi.Size())},
		{"permissions", lua.LString(permString(fi.Mode()))},
	}
}

// modeName is lfs's word for the kind of thing a path is.
func modeName(m os.FileMode) string {
	switch {
	case m.IsRegular():
		return "file"
	case m.IsDir():
		return "directory"
	case m&os.ModeSymlink != 0:
		return "link"
	case m&os.ModeSocket != 0:
		return "socket"
	case m&os.ModeNamedPipe != 0:
		return "named pipe"
	case m&os.ModeCharDevice != 0:
		return "char device"
	case m&os.ModeDevice != 0:
		return "block device"
	}
	return "other"
}

// permString is the rwxrwxrwx string ls prints, which is what lfs gives back.
func permString(m os.FileMode) string {
	const rwx = "rwxrwxrwx"
	b := []byte("---------")
	for i := 0; i < 9; i++ {
		if m&(1<<uint(8-i)) != 0 {
			b[i] = rwx[i]
		}
	}
	return string(b)
}

func lfsChdir(L *lua.LState) int {
	path := L.CheckString(1)
	if err := os.Chdir(path); err != nil {
		msg, _ := strerror(err)
		L.Push(lua.LNil)
		L.Push(lua.LString("Unable to change working directory to '" + path + "'\n" + msg + "\n"))
		return 2
	}
	return ok(L)
}

func lfsCurrentdir(L *lua.LState) int {
	dir, err := os.Getwd()
	if err != nil {
		return fail(L, err)
	}
	L.Push(lua.LString(dir))
	return 1
}

// dirState is an open directory being read a few entries at a time; "." and
// ".." come first, as readdir(3) gives them and os.ReadDir does not.
type dirState struct {
	f       *os.File
	pending []string
	closed  bool
}

func (d *dirState) close() {
	if !d.closed {
		d.closed = true
		d.f.Close()
	}
}

func lfsDir(L *lua.LState) int {
	path := L.CheckString(1)
	f, err := os.Open(path)
	if err == nil {
		var fi os.FileInfo
		if fi, err = f.Stat(); err == nil && !fi.IsDir() {
			f.Close()
			err = &os.PathError{Op: "open", Path: path, Err: syscall.ENOTDIR}
		}
	}
	if err != nil {
		msg, _ := strerror(err)
		L.RaiseError("cannot open %s: %s", path, msg)
	}
	ud := L.NewUserData()
	ud.Value = &dirState{f: f, pending: []string{".", ".."}}
	L.SetMetatable(ud, L.GetTypeMetatable(dirClass))
	L.Push(L.NewFunction(lfsDirNext))
	L.Push(ud)
	return 2
}

func checkDir(L *lua.LState) *dirState {
	ud := L.CheckUserData(1)
	d, ok := ud.Value.(*dirState)
	if !ok {
		L.ArgError(1, "directory expected")
	}
	return d
}

func lfsDirNext(L *lua.LState) int {
	d := checkDir(L)
	if d.closed {
		L.ArgError(1, "closed directory")
	}
	for len(d.pending) == 0 {
		entries, err := d.f.ReadDir(64)
		for _, e := range entries {
			d.pending = append(d.pending, e.Name())
		}
		if len(entries) == 0 || err == io.EOF {
			if len(d.pending) > 0 {
				break
			}
			d.close()
			return 0
		}
		if err != nil {
			d.close()
			msg, _ := strerror(err)
			L.RaiseError("%s", msg)
		}
	}
	name := d.pending[0]
	d.pending = d.pending[1:]
	L.Push(lua.LString(name))
	return 1
}

func lfsDirClose(L *lua.LState) int {
	if d, ok := L.CheckUserData(1).Value.(*dirState); ok {
		d.close()
	}
	return 0
}

func lfsLink(L *lua.LState) int {
	oldname, newname := L.CheckString(1), L.CheckString(2)
	var err error
	if L.ToBool(3) {
		err = os.Symlink(oldname, newname)
	} else {
		err = os.Link(oldname, newname)
	}
	if err != nil {
		return fail(L, err)
	}
	return ok(L)
}

func lfsLockFile(L *lua.LState) int {
	L.Push(lua.LNil)
	L.Push(lua.LString("file locking is not supported by tlua"))
	return 2
}

// lfsLockDir takes the lock lfs uses for a directory: a symlink named
// lockfile.lfs inside it, which only one process can create. A lock older
// than seconds_stale is broken and taken over.
func lfsLockDir(L *lua.LState) int {
	path := filepath.Join(L.CheckString(1), "lockfile.lfs")
	stale := float64(L.OptNumber(2, lua.LNumber(1<<30)))
	for {
		err := os.Symlink("lock", path)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return fail(L, err)
		}
		fi, serr := os.Lstat(path)
		if serr != nil || time.Since(fi.ModTime()).Seconds() <= stale {
			L.Push(lua.LNil)
			L.Push(lua.LString("File exists"))
			return 2
		}
		if err := os.Remove(path); err != nil {
			return fail(L, err)
		}
	}
	ud := L.NewUserData()
	ud.Value = &path
	L.SetMetatable(ud, L.GetTypeMetatable(lockClass))
	L.Push(ud)
	return 1
}

func lfsLockFree(L *lua.LState) int {
	if p, ok := L.CheckUserData(1).Value.(*string); ok && *p != "" {
		os.Remove(*p)
		*p = ""
	}
	return 0
}

func lfsMkdir(L *lua.LState) int {
	if err := os.Mkdir(L.CheckString(1), 0o777); err != nil {
		return fail(L, err)
	}
	return ok(L)
}

func lfsRmdir(L *lua.LState) int {
	path := L.CheckString(1)
	// os.Remove would delete a file too; rmdir(2) does not.
	if fi, err := os.Lstat(path); err == nil && !fi.IsDir() {
		return fail(L, &os.PathError{Op: "rmdir", Path: path, Err: syscall.ENOTDIR})
	}
	if err := syscall.Rmdir(path); err != nil {
		return fail(L, err)
	}
	return ok(L)
}

// lfsSetmode has nothing to do outside Windows: every file is binary.
func lfsSetmode(L *lua.LState) int {
	L.Push(lua.LTrue)
	L.Push(lua.LString("binary"))
	return 2
}

func lfsTouch(L *lua.LState) int {
	path := L.CheckString(1)
	now := time.Now()
	atime := now
	if L.Get(2) != lua.LNil {
		atime = time.Unix(int64(L.CheckNumber(2)), 0)
	}
	mtime := atime
	if L.Get(3) != lua.LNil {
		mtime = time.Unix(int64(L.CheckNumber(3)), 0)
	}
	if err := os.Chtimes(path, atime, mtime); err != nil {
		return fail(L, err)
	}
	return ok(L)
}
