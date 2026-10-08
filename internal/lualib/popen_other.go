//go:build !unix

package lualib

import lua "github.com/yuin/gopher-lua"

// installPopen leaves gopher-lua's io.popen alone where there is no /dev/fd
// to hand a pipe to io.open through.
func installPopen(L *lua.LState) {}
