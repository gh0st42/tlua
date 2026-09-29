package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// runMode is what F5 starts.
type runMode int

const (
	// runWithTlua runs the file itself through this interpreter.
	runWithTlua runMode = iota
	// runWithLove hands the folder holding the file to love2d, which is how a
	// LÖVE game is run: "love ." from the directory with main.lua in it.
	runWithLove
)

// EnvLove names the environment variable that picks the love2d binary, as a
// command with any arguments.
const EnvLove = "TLUA_LOVE"

// loveEntry is the file love2d insists on finding in the folder it is given.
const loveEntry = "main.lua"

// findLove reports the love2d binary to run: whatever TLUA_LOVE names, else one
// on PATH, else where the macOS application keeps it, since installing LÖVE
// there does not put anything on PATH.
func findLove() (command string, args []string, ok bool) {
	if chosen := strings.TrimSpace(os.Getenv(EnvLove)); chosen != "" {
		fields := strings.Fields(chosen)
		if path, err := exec.LookPath(fields[0]); err == nil {
			return path, fields[1:], true
		}
		return "", nil, false
	}

	if path, err := exec.LookPath("love"); err == nil {
		return path, nil, true
	}

	if runtime.GOOS == "darwin" {
		candidates := []string{"/Applications/love.app/Contents/MacOS/love"}
		if home, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates, filepath.Join(home, "Applications/love.app/Contents/MacOS/love"))
		}
		for _, candidate := range candidates {
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				return candidate, nil, true
			}
		}
	}
	return "", nil, false
}

// runCommand builds what F5 should start for a buffer, and a line saying what
// that is for the output pane.
func (e *Editor) runCommand(b *buffer) (*exec.Cmd, string, error) {
	dir, file := filepath.Dir(b.path), filepath.Base(b.path)

	if e.runMode == runWithLove {
		command, args, ok := findLove()
		if !ok {
			return nil, "", fmt.Errorf("love was not found; put it on PATH or set %s", EnvLove)
		}
		if _, err := os.Stat(filepath.Join(dir, loveEntry)); err != nil {
			return nil, "", fmt.Errorf("%s has no %s, which is what love runs", displayDir(dir), loveEntry)
		}
		cmd := exec.Command(command, append(args, ".")...)
		cmd.Dir = dir
		return cmd, fmt.Sprintf("%s . in %s", filepath.Base(command), displayDir(dir)), nil
	}

	// "--" so that a file whose name begins with a dash is a file, not an option.
	cmd := exec.Command(e.exe, "--", file)
	cmd.Dir = dir
	return cmd, file, nil
}

// toggleRunMode swaps between running a file through the interpreter and handing
// its folder to love2d.
func (e *Editor) toggleRunMode() {
	if e.runMode == runWithLove {
		e.runMode = runWithTlua
		e.setStatus("F5 runs the » file with tlua")
	} else {
		e.runMode = runWithLove
		command, _, ok := findLove()
		switch {
		case !ok:
			e.setStatus("F5 will run love, but love was not found; set " + EnvLove)
		default:
			e.setStatus("F5 runs love in the folder of the » file (" + command + ")")
		}
	}
	e.menus = e.buildMenus() // the menu shows the state
}

// runModeLabel is what the Run menu says about the setting.
func (e *Editor) runModeLabel() string {
	if e.runMode == runWithLove {
		return "Run with LOVE: on"
	}
	return "Run with LOVE: off"
}
