package gui

import (
	"bufio"
	"errors"
	"io"
	"os/exec"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// spawn is gui.spawn{command, args..., dir = ..., onOutput = fn, onExit = fn}:
// runs a program beside this one and hands its output to the handlers a line
// at a time, on the GUI's own thread, while the event loop runs. It returns
// a process with kill() and running().
//
// Its output is read by goroutines and collected; a timer on the event loop
// passes it on. FLTK is never touched from another thread.
func (a *app) spawn(L *lua.LState) int {
	t := L.CheckTable(1)
	var args []string
	for i := 1; i <= t.Len(); i++ {
		args = append(args, lua.LVAsString(t.RawGetInt(i)))
	}
	if len(args) == 0 {
		L.ArgError(1, "the command to run, as {command, args...}")
	}
	onOutput, _ := t.RawGetString("onOutput").(*lua.LFunction)
	onExit, _ := t.RawGetString("onExit").(*lua.LFunction)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = lua.LVAsString(t.RawGetString("dir"))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		L.RaiseError("gui.spawn: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		L.RaiseError("gui.spawn: %v", err)
	}

	p := &process{}
	// The timer goes first: without an event loop there is no one to tell.
	var poll func()
	poll = func() {
		lines, done, code := p.take()
		for _, l := range lines {
			if onOutput != nil {
				a.call(onOutput, lua.LString(l.text), lua.LString(l.stream))
			}
		}
		if done {
			if onExit != nil {
				a.call(onExit, lua.LNumber(code))
			}
			return
		}
		_ = addTimeout(0.05, poll)
	}
	if err := addTimeout(0.05, poll); err != nil {
		L.RaiseError("%s", err.Error())
	}
	if err := cmd.Start(); err != nil {
		L.RaiseError("gui.spawn: %v", err)
	}

	var readers sync.WaitGroup
	read := func(r io.Reader, stream string) {
		defer readers.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		for sc.Scan() {
			p.add(line{stream: stream, text: sc.Text()})
		}
	}
	readers.Add(2)
	go read(stdout, "stdout")
	go read(stderr, "stderr")
	go func() {
		readers.Wait()
		err := cmd.Wait()
		code := 0
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		} else if err != nil {
			code = -1
		}
		p.finish(code)
	}()

	h := L.NewTable()
	h.RawSetString("kill", L.NewFunction(func(L *lua.LState) int {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return 0
	}))
	h.RawSetString("running", L.NewFunction(func(L *lua.LState) int {
		p.mu.Lock()
		defer p.mu.Unlock()
		L.Push(lua.LBool(!p.done))
		return 1
	}))
	L.Push(h)
	return 1
}

type line struct{ stream, text string }

// process is what the readers have collected and the timer has not yet
// passed on.
type process struct {
	mu    sync.Mutex
	lines []line
	done  bool
	code  int
}

func (p *process) add(l line) {
	p.mu.Lock()
	p.lines = append(p.lines, l)
	p.mu.Unlock()
}

func (p *process) finish(code int) {
	p.mu.Lock()
	p.done, p.code = true, code
	p.mu.Unlock()
}

// take hands over what has been read, and whether that is the end.
func (p *process) take() ([]line, bool, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	lines := p.lines
	p.lines = nil
	return lines, p.done, p.code
}
