package interp

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"tlua/internal/payload"
)

// Fused is the program attached to a binary, ready to run: an interpreter with
// the program's own modules and data loadable out of the payload, and the chunk
// itself waiting to be called.
//
// It is separate from running it because there are two ways to run one. A plain
// program runs and ends, which is RunFused below; a program written for the
// console has an API installed on it and is then driven a frame at a time, and
// package game does that with the same state.
type Fused struct {
	*Interp

	src   []byte
	chunk string
}

// OpenFused prepares the program attached to the binary. In this mode tlua is
// not an interpreter any more: it parses no options of its own and passes the
// whole command line to the program, like a .love executable does.
func OpenFused(p *payload.Payload, exe string) (*Fused, error) {
	r := New(&Options{ScriptArgIdx: 0}) // arg[0] is the executable itself
	r.name = filepath.Base(exe)
	if dir, err := filepath.Abs(filepath.Dir(exe)); err == nil {
		// Files shipped next to the executable are still importable.
		r.prependPath(dir)
	}

	f := &Fused{Interp: r}
	switch p.Kind.Shape() {
	case payload.Zip:
		r.installArchive(p.Archive)
		data, err := p.Archive.Read(payload.EntryName)
		if err != nil {
			r.Close()
			return nil, err
		}
		f.src, f.chunk = data, payload.EntryName
	default:
		r.installSource(p.Source)
		f.src, f.chunk = p.Source, r.name
	}
	return f, nil
}

// Run calls the attached program's main chunk, handing it the arguments the
// executable was started with.
func (f *Fused) Run(args []string) error {
	return f.protect(func() error {
		fn, err := f.L.Load(bytes.NewReader(stripShebang(f.src)), f.chunk)
		if err != nil {
			return err
		}
		f.L.Push(fn)
		for _, a := range args {
			f.L.Push(lua.LString(a))
		}
		return f.L.PCall(len(args), lua.MultRet, nil)
	})
}

// RunFused executes an attached program as a script, and reports the exit
// status for the process.
func RunFused(p *payload.Payload, exe string) int {
	f, err := OpenFused(p, exe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", filepath.Base(exe), err)
		return 1
	}
	defer f.Close()
	return f.Report(f.Run(os.Args[1:]))
}

// installSource gives a single-file fused program the same "embed" module the
// archive form has, so both shapes can be introspected the same way.
func (r *Interp) installSource(src []byte) {
	r.L.PreloadModule("embed", func(L *lua.LState) int {
		mod := L.NewTable()
		L.SetField(mod, "kind", lua.LString("lua"))
		L.SetField(mod, "source", lua.LString(src))
		L.SetFuncs(mod, map[string]lua.LGFunction{
			"read": func(L *lua.LState) int {
				L.Push(lua.LNil)
				L.Push(lua.LString("this executable embeds a single Lua file, not an archive"))
				return 2
			},
			"exists": func(L *lua.LState) int { L.Push(lua.LFalse); return 1 },
			"files":  func(L *lua.LState) int { L.Push(L.NewTable()); return 1 },
		})
		L.Push(mod)
		return 1
	})
}

// installArchive teaches the state to load modules and read data files out of
// the fused archive: a package.loaders entry in front of the disk loaders, an
// "embed" module, and loadfile/dofile that fall back to the archive.
func (r *Interp) installArchive(fs *payload.Archive) {
	r.insertLoader(r.L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		base := strings.ReplaceAll(name, ".", "/")
		for _, cand := range []string{base + ".lua", base + "/init.lua"} {
			src, err := fs.Read(cand)
			if err != nil {
				continue
			}
			fn, err := L.Load(bytes.NewReader(stripShebang(src)), cand)
			if err != nil {
				L.RaiseError("%s", err.Error())
			}
			L.Push(fn)
			return 1
		}
		L.Push(lua.LString(fmt.Sprintf("no file '%s.lua' or '%s/init.lua' in fused archive", base, base)))
		return 1
	}))

	r.L.PreloadModule("embed", func(L *lua.LState) int {
		mod := L.NewTable()
		L.SetField(mod, "kind", lua.LString("zip"))
		L.SetFuncs(mod, map[string]lua.LGFunction{
			"read": func(L *lua.LState) int {
				data, err := fs.Read(L.CheckString(1))
				if err != nil {
					L.Push(lua.LNil)
					L.Push(lua.LString(err.Error()))
					return 2
				}
				L.Push(lua.LString(data))
				return 1
			},
			"exists": func(L *lua.LState) int {
				L.Push(lua.LBool(fs.Has(L.CheckString(1))))
				return 1
			},
			"files": func(L *lua.LState) int {
				tbl := L.NewTable()
				for _, n := range fs.List() {
					tbl.Append(lua.LString(n))
				}
				L.Push(tbl)
				return 1
			},
		})
		L.Push(mod)
		return 1
	})

	r.wrapLoadfile(fs)
}

// insertLoader puts fn into package.loaders right after the preload loader, so
// embedded modules win over anything on disk but package.preload still comes
// first.
func (r *Interp) insertLoader(fn *lua.LFunction) {
	loaders, ok := r.L.GetField(r.L.Get(lua.RegistryIndex), "_LOADERS").(*lua.LTable)
	if !ok {
		return
	}
	n := loaders.Len()
	for i := n; i >= 2; i-- {
		r.L.RawSetInt(loaders, i+1, r.L.RawGetInt(loaders, i))
	}
	r.L.RawSetInt(loaders, 2, fn)

	if pkg, ok := r.L.GetGlobal("package").(*lua.LTable); ok {
		r.L.SetField(pkg, "loaders", loaders)
	}
}

// wrapLoadfile makes loadfile/dofile read from the archive when it holds the
// path, so a fused app can dofile("data/levels.lua") and behave the same from
// any working directory. Paths the archive does not have fall through to disk.
func (r *Interp) wrapLoadfile(fs *payload.Archive) {
	L := r.L
	origLoadfile := L.GetGlobal("loadfile")
	origDofile := L.GetGlobal("dofile")

	load := func(L *lua.LState, name string) (*lua.LFunction, error) {
		src, err := fs.Read(name)
		if err != nil {
			return nil, err
		}
		return L.Load(bytes.NewReader(src), name)
	}

	L.SetGlobal("loadfile", L.NewFunction(func(L *lua.LState) int {
		name := L.OptString(1, "")
		if fs.Has(name) {
			fn, err := load(L, name)
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			L.Push(fn)
			return 1
		}
		return tailcall(L, origLoadfile)
	}))

	L.SetGlobal("dofile", L.NewFunction(func(L *lua.LState) int {
		name := L.OptString(1, "")
		if fs.Has(name) {
			fn, err := load(L, name)
			if err != nil {
				L.RaiseError("%s", err.Error())
			}
			base := L.GetTop()
			L.Push(fn)
			L.Call(0, lua.MultRet)
			return L.GetTop() - base
		}
		return tailcall(L, origDofile)
	}))
}

// tailcall forwards the current call's arguments to fn and returns everything
// it returns.
func tailcall(L *lua.LState, fn lua.LValue) int {
	n := L.GetTop()
	args := make([]lua.LValue, n)
	for i := 1; i <= n; i++ {
		args[i-1] = L.Get(i)
	}
	L.SetTop(0)
	L.Push(fn)
	for _, a := range args {
		L.Push(a)
	}
	L.Call(n, lua.MultRet)
	return L.GetTop()
}

// stripShebang removes the "#!" line a Lua file meant to be run directly starts
// with.
//
// Loading a file from disk skips it, so a fused copy of the same file has to as
// well, or a program that ran perfectly well as a script fails to parse once it
// is attached to a binary. The newline stays behind, so the line numbers in any
// error still match the file the person wrote.
func stripShebang(src []byte) []byte {
	if len(src) == 0 || src[0] != '#' {
		return src
	}
	if i := bytes.IndexByte(src, '\n'); i >= 0 {
		return src[i:]
	}
	return nil
}
