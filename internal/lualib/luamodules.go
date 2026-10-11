package lualib

// The modules tlua carries written in Lua. They are found after the
// program's own modules, so a program with a markdown.lua or a socket.lua
// of its own keeps using it.
//
// Two kinds:
//
//   - tlua's own: markdown, markdown.layout, markdown.render and
//     markdown.editor, Markdown read into blocks and written back, laid
//     out, drawn on a gui Canvas, and edited; the gui's MarkdownView and
//     MarkdownEdit are made of them. Each is also tlua.<name>, which is how
//     they require one another and the gui requires them: those names are
//     preloaded, so a program's own markdown.lua never stands in for tlua's
//     inside them.
//   - the Lua halves of LuaSocket 3.1.0 (socket, socket.http, socket.url,
//     socket.headers, socket.tp, socket.ftp, socket.smtp, ltn12, mime) and
//     of LuaSec 1.3.2 (ssl, ssl.https), as their authors wrote them, MIT
//     licensed (lua/luasocket/LICENSE, lua/luasec/LICENSE). Their C halves
//     are socket.core, mime.core and the ssl.* modules, in Go, so that
//     require "socket" and require "ssl" work with nothing installed.

import (
	"bytes"
	"embed"

	lua "github.com/yuin/gopher-lua"
)

//go:embed lua
var luaSources embed.FS

// ownModules are tlua's own, by the name require takes.
var ownModules = map[string]string{
	"markdown":        "lua/markdown.lua",
	"markdown.layout": "lua/markdown_layout.lua",
	"markdown.render": "lua/markdown_render.lua",
	"markdown.editor": "lua/markdown_editor.lua",
}

// libraryModules are the Lua halves of the libraries tlua carries.
var libraryModules = map[string]string{
	"socket":         "lua/luasocket/socket.lua",
	"socket.http":    "lua/luasocket/http.lua",
	"socket.url":     "lua/luasocket/url.lua",
	"socket.headers": "lua/luasocket/headers.lua",
	"socket.tp":      "lua/luasocket/tp.lua",
	"socket.ftp":     "lua/luasocket/ftp.lua",
	"socket.smtp":    "lua/luasocket/smtp.lua",
	"ltn12":          "lua/luasocket/ltn12.lua",
	"mime":           "lua/luasocket/mime.lua",
	"ssl":            "lua/luasec/ssl.lua",
	"ssl.https":      "lua/luasec/https.lua",
}

// loadLuaModule compiles one of them, to be called with its name.
func loadLuaModule(L *lua.LState, file, name string) *lua.LFunction {
	src, _ := luaSources.ReadFile(file)
	fn, err := L.Load(bytes.NewReader(src), "="+name)
	if err != nil {
		L.RaiseError("%s", err.Error())
	}
	return fn
}

func installLuaModules(L *lua.LState) {
	for name, file := range ownModules {
		name, file := "tlua."+name, file
		L.PreloadModule(name, func(L *lua.LState) int {
			L.Push(loadLuaModule(L, file, name))
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
		if file, ok := libraryModules[name]; ok {
			L.Push(loadLuaModule(L, file, name))
			return 1
		}
		if _, ok := ownModules[name]; !ok {
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
