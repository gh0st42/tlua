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

// RunFused executes the program attached to the binary. In this mode tlua is
// not an interpreter any more: it parses no options of its own and passes the
// whole command line to the program, like a .love executable does.
func RunFused(p *payload.Payload, exe string) int {
	opts := &Options{ScriptArgIdx: 0} // arg[0] is the executable itself
	r := New(opts)
	defer r.Close()

	r.name = filepath.Base(exe)
	if dir, err := filepath.Abs(filepath.Dir(exe)); err == nil {
		// Files shipped next to the executable are still importable.
		r.prependPath(dir)
	}

	var (
		src   []byte
		chunk string
	)
	switch p.Kind {
	case payload.Zip:
		r.installArchive(p.Archive)
		data, err := p.Archive.Read(payload.EntryName)
		if err != nil {
			return r.Report(err)
		}
		src, chunk = data, payload.EntryName
	default:
		r.installSource(p.Source)
		src, chunk = p.Source, r.name
	}

	err := r.protect(func() error {
		fn, err := r.L.Load(bytes.NewReader(src), chunk)
		if err != nil {
			return err
		}
		r.L.Push(fn)
		args := os.Args[1:]
		for _, a := range args {
			r.L.Push(lua.LString(a))
		}
		return r.L.PCall(len(args), lua.MultRet, nil)
	})
	return r.Report(err)
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
			fn, err := L.Load(bytes.NewReader(src), cand)
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
