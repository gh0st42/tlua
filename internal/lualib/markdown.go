package lualib

// markdown, markdown.layout and markdown.render are tlua's own, written in
// Lua: Markdown read into blocks and written back, laid out on a page, and
// drawn on a gui Canvas. The gui's MarkdownView is made of them, and so can
// a program's own controls be. They are found after the program's own
// modules, so a program with a markdown.lua of its own keeps using it.
//
// Each is also tlua.<name>, which is how they require one another and the
// MarkdownView requires them: those names are preloaded, so a program's own
// markdown.lua never stands in for tlua's inside them.

import (
	"bytes"
	"embed"

	lua "github.com/yuin/gopher-lua"
)

//go:embed lua/*.lua
var luaSources embed.FS

// luaModules are the modules written in Lua, by the name require takes.
var luaModules = map[string]string{
	"markdown":        "lua/markdown.lua",
	"markdown.layout": "lua/markdown_layout.lua",
	"markdown.render": "lua/markdown_render.lua",
}

func installLuaModules(L *lua.LState) {
	for name, file := range luaModules {
		name, file := "tlua."+name, file
		L.PreloadModule(name, func(L *lua.LState) int {
			src, _ := luaSources.ReadFile(file)
			fn, err := L.Load(bytes.NewReader(src), "="+name)
			if err != nil {
				L.RaiseError("%s", err.Error())
			}
			L.Push(fn)
			L.Push(lua.LString(name))
			L.Call(1, 1)
			return 1
		})
	}
	// The plain names come last of all the loaders.
	loaders, ok := L.GetField(L.Get(lua.RegistryIndex), "_LOADERS").(*lua.LTable)
	if !ok {
		return
	}
	loaders.Append(L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		if _, ok := luaModules[name]; !ok {
			L.Push(lua.LString("\n\tno module '" + name + "' built into tlua"))
			return 1
		}
		L.Push(L.NewFunction(func(L *lua.LState) int {
			L.Push(L.GetGlobal("require"))
			L.Push(lua.LString("tlua." + name))
			L.Call(1, 1)
			return 1
		}))
		return 1
	}))
}
