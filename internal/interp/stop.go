package interp

import (
	"os"
	"syscall"
	"time"

	"golang.org/x/term"
)

// stopGrace is how long a cancelled chunk has to finish before tlua ends
// the process. Lua code sees the cancel at its next instruction, a socket
// or a GUI wait at once; only a call blocked in Go stays, such as io.read
// waiting on the terminal, which nothing can interrupt.
var stopGrace = time.Second

// stopStuck waits, after a signal has cancelled the running chunk, for it
// to finish. When it does not in time, or a second signal comes, tlua exits
// as a process killed by the signal would, after putting the terminal back:
// a program reading keys in raw mode (tui's) would otherwise leave it
// without echo.
func (r *Interp) stopStuck(sig os.Signal, done <-chan struct{}) {
	select {
	case <-done:
		return
	case sig = <-r.interrupt:
	case <-time.After(stopGrace):
	}
	if r.terminal != nil {
		r.terminal()
	}
	code := 130 // 128 + SIGINT, as a shell reports it
	if sig == syscall.SIGTERM {
		code = 143
	}
	os.Exit(code)
}

// saveTerminal remembers the terminal's settings, when stdin is one, and
// returns what puts them back, with the cursor shown and colours reset.
func saveTerminal() func() {
	in := int(os.Stdin.Fd())
	if !term.IsTerminal(in) {
		return nil
	}
	state, err := term.GetState(in)
	if err != nil {
		return nil
	}
	return func() {
		term.Restore(in, state)
		if term.IsTerminal(int(os.Stdout.Fd())) {
			os.Stdout.WriteString("\x1b[0m\x1b[?25h\n")
		}
	}
}
