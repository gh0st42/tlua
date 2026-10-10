// Package lualib is what tlua adds to gopher-lua's standard libraries so that
// code written for the reference interpreter runs unchanged: a few repairs to
// the libraries that are there, and Go versions of the C modules that the
// common Lua libraries are built on, since a C module cannot be loaded here.
package lualib

import (
	lua "github.com/yuin/gopher-lua"
)

// Open installs everything into a fresh state. The modules are preloaded
// rather than set as globals, so a program sees them exactly as it would a
// LuaRocks install: through require, and only if it asks.
func Open(L *lua.LState) {
	installOpen(L)
	installPopen(L)
	L.PreloadModule("lfs", openLfs)
	L.PreloadModule("mime.core", openMimeCore)
	L.PreloadModule("zlib", openZlib)
	L.PreloadModule("png", openPng)
	L.PreloadModule("zip", openZip)
	preloadSocket(L)
	installLuaModules(L)
}
