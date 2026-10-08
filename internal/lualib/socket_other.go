//go:build !(linux || darwin)

package lualib

import lua "github.com/yuin/gopher-lua"

// preloadSocket has nothing to offer here: socket.core is written against
// the system calls of Linux and macOS, and elsewhere require reports it
// missing, as it always did.
func preloadSocket(L *lua.LState) {}
