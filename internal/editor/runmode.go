package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// runMode is what F5 starts. A file can be a plain script, a program for the
// console tlua itself provides, or a LÖVE game, and which one it is belongs to
// the project rather than to the file, so it is a setting rather than a guess.
type runMode int

const (
	// runWithTlua runs the file itself through this interpreter.
	runWithTlua runMode = iota
	// runWithPico runs it in a window against the console API, which is
	// "tlua play" on the file.
	runWithPico
	// runWithLove hands the folder holding the file to love2d, which is how a
	// LÖVE game is run: "love ." from the directory with main.lua in it.
	runWithLove

	runModes // how many there are, for cycling through them
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

	switch e.runMode {
	case runWithPico:
		// The window is this same binary, so there is nothing to look for:
		// "tlua play" on the file, from its own folder so that the modules it
		// requires are found the way they would be anywhere else.
		cmd := exec.Command(e.exe, "play", "--", file)
		cmd.Dir = dir
		return cmd, "tlua play " + file, nil

	case runWithLove:
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

// cycleRunMode moves on to the next way of running a file, and says what that
// means now rather than making anyone open the menu again to find out.
func (e *Editor) cycleRunMode() {
	e.runMode = (e.runMode + 1) % runModes

	switch e.runMode {
	case runWithPico:
		e.setStatus("F5 runs the » file in a window, with the console API")
	case runWithLove:
		command, _, ok := findLove()
		if !ok {
			e.setStatus("F5 will run love, but love was not found; set " + EnvLove)
			break
		}
		e.setStatus("F5 runs love in the folder of the » file (" + command + ")")
	default:
		e.setStatus("F5 runs the » file with tlua")
	}
	e.menus = e.buildMenus() // the menu shows the state
}

// runModeName is what the setting is called.
func (e *Editor) runModeName() string {
	switch e.runMode {
	case runWithPico:
		return "console window"
	case runWithLove:
		return "LOVE"
	default:
		return "tlua"
	}
}

// runModeLabel is what the Run menu says about the setting.
func (e *Editor) runModeLabel() string {
	return "Run with: " + e.runModeName()
}
