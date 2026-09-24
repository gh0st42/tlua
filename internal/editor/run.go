package editor

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rivo/tview"

	"tlua/internal/interp"
)

// runPrimary is F5: it runs the file marked as primary, whichever buffer is on
// screen at the time.
func (e *Editor) runPrimary() {
	if e.primary == nil {
		e.message("Run", "No primary file yet.\n\nSave a buffer, then pick\nRun > Set as primary file.")
		return
	}
	e.runBuffer(e.primary)
}

// runCurrent runs the buffer being edited without changing which file is
// primary, for trying out a module on its own.
func (e *Editor) runCurrent() {
	if b := e.buf(); b != nil {
		e.runBuffer(b)
	}
}

// runBuffer saves what has changed and starts the interpreter on path. The
// program is a child process, so a runaway loop cannot take the editor with it
// and Ctrl-C can stop it.
func (e *Editor) runBuffer(b *buffer) {
	if e.running != nil {
		e.message("Run", "The program is still running.\nStop it with Ctrl-C first.")
		return
	}
	if b.path == "" {
		e.message("Run", "Save this buffer to a file first.")
		return
	}
	// Everything on disk should match what is on screen, so that a module the
	// script requires is the version just edited.
	for _, other := range e.buffers {
		if other.dirty && other.path != "" {
			if err := e.save(other); err != nil {
				e.message("Save", err.Error())
				return
			}
		}
	}

	e.showOutput()
	e.clearOutput()
	dir, file := filepath.Dir(b.path), filepath.Base(b.path)
	e.appendOutput(tagNote + "Running " + tview.Escape(file) + "\n")

	// "--" so that a file whose name begins with a dash is a file, not an option.
	cmd := exec.Command(e.exe, "--", file)
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		e.message("Run", err.Error())
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		e.message("Run", err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		e.message("Run", err.Error())
		return
	}
	e.running = cmd
	e.setStatus("Running " + file + " — Ctrl-C stops it")

	var (
		wg      sync.WaitGroup
		errText strings.Builder
	)
	wg.Add(2)
	go e.pump(&wg, stdout, tagOutput, nil)
	go e.pump(&wg, stderr, tagErr, &errText)

	go func() {
		wg.Wait()
		waitErr := cmd.Wait()
		// Whatever the program printed last has not been flushed yet.
		remaining := e.takePendingOutput()
		e.app.QueueUpdateDraw(func() {
			if remaining != "" {
				e.appendOutput(remaining)
			}
			e.running = nil
			switch {
			case waitErr == nil:
				e.appendOutput(tagOK + "\nProgram finished.\n")
				e.setStatus(file + " finished")
			default:
				status := waitErr.Error()
				if exitErr, ok := waitErr.(*exec.ExitError); ok {
					status = fmt.Sprintf("exit status %d", exitErr.ExitCode())
				}
				e.appendOutput(tagErr + "\nProgram stopped: " + tview.Escape(status) + "\n")
				e.setStatus(file + ": " + status)
				e.jumpToError(dir, errText.String())
			}
		})
	}()
}

// pump copies one of the program's streams into the output pane, and optionally
// keeps a copy for the error jump.
func (e *Editor) pump(wg *sync.WaitGroup, r io.Reader, tag string, keep *strings.Builder) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if keep != nil {
			keep.WriteString(line)
			keep.WriteByte('\n')
		}
		e.writeOutput(tag + tview.Escape(line) + "\n")
	}
}

// outputFlush is how long lines are allowed to pile up before the pane is
// redrawn.
const outputFlush = 50 * time.Millisecond

// writeOutput queues a line for the output pane. A program that prints without
// pause would otherwise force a redraw of the whole screen for every line, and
// tview's queue waits for each one, which makes the editor crawl just when the
// program is at its busiest. Lines gather instead and go up together.
func (e *Editor) writeOutput(text string) {
	e.outputMu.Lock()
	e.outputPending.WriteString(text)
	already := e.outputFlushing
	e.outputFlushing = true
	e.outputMu.Unlock()
	if already {
		return
	}

	go func() {
		time.Sleep(outputFlush)
		text := e.takePendingOutput()
		if text == "" {
			return
		}
		e.app.QueueUpdateDraw(func() { e.appendOutput(text) })
	}()
}

// takePendingOutput hands over whatever has gathered, and lets the next line
// schedule a fresh flush.
func (e *Editor) takePendingOutput() string {
	e.outputMu.Lock()
	defer e.outputMu.Unlock()
	text := e.outputPending.String()
	e.outputPending.Reset()
	e.outputFlushing = false
	return text
}

// stopProgram is Ctrl-C: it kills the running program, not the editor.
func (e *Editor) stopProgram() {
	if e.running == nil {
		e.setStatus("Nothing is running")
		return
	}
	if err := e.running.Process.Kill(); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			e.setStatus("The program had already finished")
			return
		}
		e.setStatus("Cannot stop the program: " + err.Error())
		return
	}
	e.setStatus("Program stopped")
}

// errorPos matches the "file:line:" that a Lua error starts with.
var errorPos = regexp.MustCompile(`([^\s:]+\.lua):(\d+):`)

// jumpToError puts the cursor where the program failed, opening the file if it
// is not already in a buffer.
func (e *Editor) jumpToError(dir, stderr string) {
	m := errorPos.FindStringSubmatch(stderr)
	if m == nil {
		return
	}
	line, err := strconv.Atoi(m[2])
	if err != nil {
		return
	}
	path := m[1]
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	idx, b := e.bufferFor(path)
	if b == nil {
		if err := e.openFile(path); err != nil {
			return
		}
		idx, b = e.bufferFor(path)
		if b == nil {
			return
		}
	}
	e.selectBuffer(idx)
	e.gotoLine(b, line)
	e.setStatus(fmt.Sprintf("%s line %d: %s", b.name, line, firstErrorMessage(stderr)))
}

// firstErrorMessage pulls the message out of "tlua: file:12: message".
func firstErrorMessage(stderr string) string {
	line := strings.SplitN(strings.TrimSpace(stderr), "\n", 2)[0]
	if loc := errorPos.FindStringIndex(line); loc != nil {
		return strings.TrimSpace(line[loc[1]:])
	}
	return strings.TrimSpace(line)
}

// checkCurrent is F9: it compiles the buffer without running it, the way the
// DOS IDEs compiled to memory.
func (e *Editor) checkCurrent() {
	b := e.buf()
	if b == nil {
		return
	}
	err := interp.CheckSyntax(b.area.GetText(), b.name)
	if err == nil {
		e.setStatus(b.name + ": no syntax errors")
		return
	}
	var syntaxErr *interp.SyntaxError
	if errors.As(err, &syntaxErr) && syntaxErr.Line > 0 {
		e.gotoLine(b, syntaxErr.Line)
		e.setStatus(fmt.Sprintf("%s line %d: %s", b.name, syntaxErr.Line, syntaxErr.Message))
		return
	}
	e.setStatus(b.name + ": " + err.Error())
}

/* --- the output pane --- */

func (e *Editor) showOutput() {
	if !e.outputShown {
		e.outputShown = true
		e.layout.ResizeItem(e.output, outputHeight, 0)
	}
}

func (e *Editor) toggleOutput() {
	if e.outputShown {
		e.outputShown = false
		e.layout.ResizeItem(e.output, 0, 0)
		if b := e.buf(); b != nil {
			e.app.SetFocus(b.area)
		}
		return
	}
	e.showOutput()
	e.app.SetFocus(e.output)
	e.setStatus("Escape goes back to the text")
}

func (e *Editor) clearOutput() {
	e.takePendingOutput() // drop what has not gone up yet
	e.output.SetText("")
	e.outputBytes = 0
	e.outputFull = false
}

// outputLimit caps what the pane keeps. A runaway loop printing for ever is a
// normal thing to do by accident, and it must not take the editor's memory
// with it; the program itself keeps running until Ctrl-C stops it.
const outputLimit = 256 * 1024

func (e *Editor) appendOutput(text string) {
	if e.outputFull {
		return
	}
	if e.outputBytes+len(text) > outputLimit {
		e.outputFull = true
		fmt.Fprint(e.output, tagNote+"\n[output truncated \u2014 Ctrl-C stops the program]\n")
		e.output.ScrollToEnd()
		return
	}
	e.outputBytes += len(text)
	fmt.Fprint(e.output, text)
	e.output.ScrollToEnd()
}
