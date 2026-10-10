package gui

import (
	"bytes"
	"embed"
	"os"

	lua "github.com/yuin/gopher-lua"
)

// The controls in lua/ come with tlua but are written in Lua, on the gui
// module, the way a script writes its own with gui.define: MarkdownView.

//go:embed lua/*.lua
var builtinSources embed.FS

// defineBuiltins runs each of them with the module and what they need from
// Go: read(path), which reads the program's own files first, as png.load
// does.
func (a *app) defineBuiltins(L *lua.LState, mod *lua.LTable) {
	host := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"read": func(L *lua.LState) int {
			data, err := a.readFile(L.CheckString(1))
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			L.Push(lua.LString(data))
			return 1
		},
	})
	entries, _ := builtinSources.ReadDir("lua")
	a.definingBuiltins = true
	defer func() { a.definingBuiltins = false }()
	for _, e := range entries {
		src, _ := builtinSources.ReadFile("lua/" + e.Name())
		fn, err := L.Load(bytes.NewReader(src), "=gui/"+e.Name())
		if err != nil {
			L.RaiseError("gui: %v", err)
		}
		L.Push(fn)
		L.Push(mod)
		L.Push(host)
		L.Call(2, 0)
	}
}

// readFile reads a file of the program's: out of its archive when it is
// fused or bundled and has it there, else from the disk.
func (a *app) readFile(path string) ([]byte, error) {
	if a.read != nil {
		if data, err := a.read(path); err == nil {
			return data, nil
		}
	}
	return os.ReadFile(path)
}
