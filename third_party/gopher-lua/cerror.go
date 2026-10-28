package lua

import (
	"errors"
	"os"
	"strings"
	"syscall"
)

// cError words an error from the os package as C and Lua 5.1 do: the
// system's message as libc spells it ("No such file or directory"), led by
// the file's name when there is one, and the errno beside it. Go's own
// wording ("open x: no such file or directory") is what a program would have
// to learn to read, and its errno was always 1.
func cError(name string, err error) (string, int) {
	msg, errno := err.Error(), 0
	var en syscall.Errno
	if errors.As(err, &en) {
		msg, errno = en.Error(), int(en)
		if msg != "" {
			msg = strings.ToUpper(msg[:1]) + msg[1:]
		}
	} else {
		var pe *os.PathError
		if errors.As(err, &pe) {
			msg = pe.Err.Error()
		}
	}
	if name != "" {
		msg = name + ": " + msg
	}
	return msg, errno
}
