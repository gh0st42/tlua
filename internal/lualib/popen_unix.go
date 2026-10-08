//go:build unix

package lualib

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	lua "github.com/yuin/gopher-lua"
)

// installPopen replaces io.popen with one whose command shares tlua's stdin
// and stderr, the way popen(3) does. gopher-lua's own gives the command
// /dev/null for both, so `stty size` cannot see the terminal and anything a
// command complains about is lost, and terminal libraries come apart on it.
//
// The command's end of the pipe is opened with gopher-lua's own io.open, by
// way of /dev/fd, so what the program gets back is an ordinary Lua file:
// io.type, read, lines and the rest work on it as they always did. Closing
// it waits for the command and gives back its exit status, as before.
func installPopen(L *lua.LState) {
	iolib, ok := L.GetGlobal("io").(*lua.LTable)
	if !ok {
		return
	}
	open := L.GetField(iolib, "open")
	procs := map[*lua.LUserData]*exec.Cmd{}

	L.SetField(iolib, "popen", L.NewFunction(func(L *lua.LState) int {
		cmd := L.CheckString(1)
		mode := L.OptString(2, "r")
		if mode != "r" && mode != "w" {
			L.ArgError(2, "invalid mode")
		}

		pr, pw, err := os.Pipe()
		if err != nil {
			return fail(L, err)
		}
		c := exec.Command("/bin/sh", "-c", cmd)
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		ours, theirs := pr, pw
		if mode == "r" {
			c.Stdout = pw
		} else {
			c.Stdin = pr
			ours, theirs = pw, pr
		}
		err = c.Start()
		theirs.Close()
		if err != nil {
			ours.Close()
			return fail(L, err)
		}

		L.Push(open)
		L.Push(lua.LString(fmt.Sprintf("/dev/fd/%d", ours.Fd())))
		L.Push(lua.LString(mode))
		L.Call(2, 2)
		ours.Close()
		ud, ok := L.Get(-2).(*lua.LUserData)
		if !ok {
			go c.Wait()
			return 2
		}
		procs[ud] = c
		L.Pop(1)
		return 1
	}))

	// file:close() and io.close(file) both end up here; for a handle popen
	// made, they wait for the command once the pipe is closed.
	wrap := func(orig lua.LValue) lua.LValue {
		fn, ok := orig.(*lua.LFunction)
		if !ok {
			return orig
		}
		return L.NewFunction(func(L *lua.LState) int {
			ud, _ := L.Get(1).(*lua.LUserData)
			top := L.GetTop()
			L.Push(fn)
			for i := 1; i <= top; i++ {
				L.Push(L.Get(i))
			}
			L.Call(top, lua.MultRet)
			c, ok := procs[ud]
			if !ok {
				return L.GetTop() - top
			}
			delete(procs, ud)
			L.SetTop(top)
			L.Push(lua.LNumber(exitStatus(c.Wait())))
			return 1
		})
	}
	if mt, ok := L.GetTypeMetatable("FILE*").(*lua.LTable); ok {
		mt.RawSetString("close", wrap(mt.RawGetString("close")))
	}
	L.SetField(iolib, "close", wrap(L.GetField(iolib, "close")))
}

func exitStatus(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	if err != nil {
		return 1
	}
	return 0
}
