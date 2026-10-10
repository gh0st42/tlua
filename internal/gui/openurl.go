package gui

import (
	"os/exec"
	goruntime "runtime"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// openCommand is how the system opens a web address or a file in the
// program it belongs to. Tests replace it.
var openCommand = func(target string) *exec.Cmd {
	switch goruntime.GOOS {
	case "darwin":
		return exec.Command("open", target)
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	}
	return exec.Command("xdg-open", target)
}

// openURL is gui.openurl(target): a web address opened in the browser, a
// mail address in the mail program, a file or folder in the program the
// system opens it with. It returns true once that program is started, not
// when it is done, or nil and why.
func (a *app) openURL(L *lua.LState) int {
	target := L.CheckString(1)
	if target == "" || strings.HasPrefix(target, "-") {
		L.ArgError(1, "a web address or the path of a file")
	}
	cmd := openCommand(target)
	if err := cmd.Start(); err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	go cmd.Wait() // nothing to wait for but its exit, which must be collected
	L.Push(lua.LTrue)
	return 1
}
