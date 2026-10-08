package lualib

import (
	"os"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// installOpen makes io.open create files the way fopen(3) does, readable by
// everyone the umask allows. gopher-lua creates every file 0600, so what a
// script writes is private to whoever ran it, which no Lua program expects.
//
// A file that does not exist yet is made first, at 0666 less the umask, and
// gopher-lua then opens it as usual; an existing file is not touched.
func installOpen(L *lua.LState) {
	iolib, ok := L.GetGlobal("io").(*lua.LTable)
	if !ok {
		return
	}
	open, ok := L.GetField(iolib, "open").(*lua.LFunction)
	if !ok {
		return
	}
	L.SetField(iolib, "open", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		mode := L.OptString(2, "r")
		if strings.ContainsAny(mode, "wa") {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				if f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666); err == nil {
					f.Close()
				}
			}
		}
		top := L.GetTop()
		L.Push(open)
		for i := 1; i <= top; i++ {
			L.Push(L.Get(i))
		}
		L.Call(top, lua.MultRet)
		return L.GetTop() - top
	}))
}
