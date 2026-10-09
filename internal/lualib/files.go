package lualib

import (
	"os"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// readers are where each state's modules read a program's own files: the
// archive of a fused program or a bundle first, then the disk. A state
// with none reads the disk.
var readers sync.Map // *lua.LState -> func(string) ([]byte, error)

// SetReader says where png.load and zip.open read files from in L.
func SetReader(L *lua.LState, read func(string) ([]byte, error)) {
	readers.Store(L, read)
}

// Forget lets go of what SetReader kept for a state that is closing.
func Forget(L *lua.LState) { readers.Delete(L) }

func readFile(L *lua.LState, name string) ([]byte, error) {
	if r, ok := readers.Load(L); ok {
		return r.(func(string) ([]byte, error))(name)
	}
	return os.ReadFile(name)
}
