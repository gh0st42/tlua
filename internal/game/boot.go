package game

import (
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	lua "github.com/yuin/gopher-lua"

	"tlua/internal/interp"
	"tlua/internal/payload"
	"tlua/internal/picolua"
	"tlua/internal/sound"
)

// A program run as an ordinary script — `tlua game.lua`, a `#!/usr/bin/env
// tlua` line, one fused without -play — has none of the console API, because
// most scripts are not games and print() writing to the terminal rather than to
// a screen is what those expect.
//
// boot() is how such a program says otherwise. It puts the console in place
// there and then, so the rest of the file can use it, and asks for a window,
// which opens once the file has finished running. That last part is what makes
// the call work at the top of the file, where it belongs and where the
// callbacks it needs have not been written yet.
//
// The same file then runs three ways without changing: `tlua game.lua`,
// `tlua play game.lua`, and fused either way. In the two where the console is
// already there, boot() is the one in picolua, which says "yes, quite" and
// applies any window options it was given.

// Boot is a program that may ask for a window while it runs. It is installed
// before the program starts and asked afterwards what happened.
type Boot struct {
	in   *interp.Interp
	opts Options
	read func(string) ([]byte, error)

	s      *session
	wanted bool
	late   bool
}

// Ready installs boot() into an interpreter that is about to run an ordinary
// script. Nothing of the console is built until the program asks for it.
//
// read is where the program's own files come from — beside the script, or out
// of the executable for a fused one. Without it, the disk beside the script.
func Ready(in *interp.Interp, opts Options, read func(string) ([]byte, error)) *Boot {
	b := &Boot{in: in, opts: opts, read: read}

	in.L.SetGlobal("boot", in.L.NewFunction(func(L *lua.LState) int {
		if b.late {
			L.RaiseError("boot: too late to ask for a window; " +
				"a program asks while it is running, not after")
		}
		if b.s == nil {
			b.start()
		}
		// From here the console's own boot() has taken the name over, so this
		// runs once. Window options given to the first call are applied the
		// same way the second one would.
		if opts, ok := L.Get(1).(*lua.LTable); ok {
			b.s.rt.ApplyWindow(L, opts)
		}
		b.wanted = true
		L.Push(lua.LTrue)
		return 1
	}))
	return b
}

// start puts the console API on top of the interpreter, which is everything
// `tlua play` does except opening the window.
func (b *Boot) start() {
	title := b.opts.Title
	if title == "" {
		title = titleFor(b.script())
	}
	read, write := savesFor(title)

	b.s = &session{opts: b.opts, interp: b.in, sound: sound.New()}
	b.s.rt = picolua.New(b.in.L, picolua.Options{
		Title:       title,
		Out:         os.Stdout,
		FPS:         ebiten.ActualFPS,
		SetTPS:      b.s.setRate,
		ButtonLabel: buttonLabel,
		ReadFile:    b.files(),
		Sound:       b.s.sound,
		ReadSave:    read,
		WriteSave:   write,
	})
}

// files reports where the program's artwork and data are read from.
func (b *Boot) files() func(string) ([]byte, error) {
	if b.read != nil {
		return b.read
	}
	if b.opts.Script == "" {
		return os.ReadFile // a program from -e, or from standard input
	}
	return besideProgram(filepath.Dir(b.script()))
}

// script reports the file the program came from, resolved the way `tlua play`
// resolves it so that a folder means the main.lua inside it.
func (b *Boot) script() string {
	if path, err := scriptPath(b.opts.Script); err == nil {
		return path
	}
	return b.opts.Script
}

// Attached is where a fused program's own files are read from: the archive
// inside the executable first, then the disk beside it.
func Attached(p *payload.Payload) func(string) ([]byte, error) { return attachedFirst(p) }

// TooLate says that the moment for asking has passed: the program has finished
// and whatever comes next — an interactive session, another script — is not
// something a window can be opened in front of.
//
// A program that asks anyway is told so, which is better than a call that
// quietly does nothing.
func (b *Boot) TooLate() { b.late = true }

// Wanted reports whether the program asked for a window.
func (b *Boot) Wanted() bool { return b != nil && b.wanted }

// Show opens the window and runs the program's callbacks until it is closed.
//
// The main chunk has already run — that is where boot() was called — so the
// loop starts at _init rather than at the top of the file.
func (b *Boot) Show() int {
	if err := b.s.rt.Start(nil, nil); err != nil {
		return b.s.report(err)
	}
	return b.s.afterStart()
}
