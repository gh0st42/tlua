// Package design is tlua design: a form designer in the manner of Visual
// Basic 6, for the gui module. The designer itself is written in Lua, on the
// gui module, and lives in this package's lua/ directory, embedded in the
// binary; this file only starts it. docs/rad-plan.md is the plan it follows.
package design

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"tlua/internal/gui"
	"tlua/internal/interp"
)

//go:embed lua/*.lua
var sources embed.FS

const usage = `usage: tlua design [directory]

Opens the form designer on the project in directory, or the current one.
A project is main.lua and a forms/ folder: each form a layout file the
designer writes (Name.form.lua) and the code that goes with it (Name.lua).
A directory with no forms in it can be made into a new project.
`

// Command runs tlua design.
func Command(args []string) int {
	dir := "."
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			fmt.Print(usage)
			return 0
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "tlua design: unknown option %s\n%s", a, usage)
			return 1
		default:
			dir = a
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tlua design: %v\n", err)
		return 1
	}

	r := interp.New(&interp.Options{})
	defer r.Close()
	boot := gui.Ready(r, nil)
	Preload(r.L)

	L := r.L
	err = L.DoString(`bootgui()`)
	if err == nil {
		start := L.GetField(requireModule(L, "design.main"), "start")
		opts := L.NewTable()
		opts.RawSetString("dir", lua.LString(abs))
		err = L.CallByParam(lua.P{Fn: start, NRet: 0, Protect: true}, opts)
	}
	if err != nil {
		return r.Report(err)
	}
	boot.TooLate()
	return r.Report(boot.Show())
}

// Preload makes the designer's Lua files requirable as design.<name>.
func Preload(L *lua.LState) {
	entries, _ := sources.ReadDir("lua")
	for _, e := range entries {
		file := e.Name()
		name := "design." + strings.TrimSuffix(file, ".lua")
		L.PreloadModule(name, func(L *lua.LState) int {
			src, err := sources.ReadFile("lua/" + file)
			if err != nil {
				L.RaiseError("%v", err)
			}
			fn, err := L.Load(bytes.NewReader(src), "@design/"+file)
			if err != nil {
				L.RaiseError("%v", err)
			}
			L.Push(fn)
			L.Call(0, 1)
			return 1
		})
	}
}

func requireModule(L *lua.LState, name string) lua.LValue {
	L.Push(L.GetGlobal("require"))
	L.Push(lua.LString(name))
	L.Call(1, 1)
	ret := L.Get(-1)
	L.Pop(1)
	return ret
}
