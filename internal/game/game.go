// Package game runs a Lua program in a window: it opens one with Ebitengine,
// drives the program's _update and _draw sixty times a second, and puts the
// console's framebuffer on the screen.
//
// Ebitengine needs no cgo on any of the platforms tlua is built for, so adding a
// window costs the project nothing it was not already paying: CGO_ENABLED=0 go
// build still produces one static binary.
package game

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/hajimehoshi/ebiten/v2"

	"tlua/internal/interp"
	"tlua/internal/payload"
	"tlua/internal/picolua"
	"tlua/internal/sound"
)

const usage = `usage: tlua play [options] [script | directory] [args...]

Runs a Lua program in a window, with the console API described in
docs/pico.md: cls, spr, btn, print and the rest. The program draws in
_draw(), moves in _update(), and both are called sixty times a second.

Options:
  -scale n     how many screen pixels to a console pixel
  -fullscreen  start filling the screen
  -title s     what to call the window
  -h           print this help

Given a directory, the file run is its main.lua; given nothing, the
main.lua in the current directory.

While it runs:
  alt-enter, F11   fullscreen
  ctrl-D           show the frame rate
  ctrl-Q           quit

Those keys belong to the window: the program is not shown them.
`

// Options is what the command line asked for.
type Options struct {
	Script     string
	Args       []string
	Scale      int
	Fullscreen bool
	Title      string

	// ArgIdx is where the script sits in os.Args, for the arg table.
	ArgIdx int

	help bool
}

// Command is `tlua play`.
func Command(args []string) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tlua play: %v\n\n%s", err, usage)
		return 1
	}
	if opts.help {
		fmt.Print(usage)
		return 0
	}
	return Run(opts)
}

// parseArgs reads the options, stopping at the script name so that everything
// after it belongs to the program.
func parseArgs(args []string) (Options, error) {
	opts := Options{ArgIdx: 2} // tlua play <script>
	i := 0
	for ; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			break
		}
		if a == "--" {
			i++
			break
		}
		name := strings.TrimLeft(a, "-")
		value := ""
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name, value = name[:eq], name[eq+1:]
		}
		next := func() (string, error) {
			if value != "" {
				return value, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("'%s' needs a value", a)
			}
			i++
			return args[i], nil
		}

		switch name {
		case "h", "help":
			opts.help = true
		case "fullscreen":
			opts.Fullscreen = true
		case "scale":
			v, err := next()
			if err != nil {
				return opts, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return opts, fmt.Errorf("scale must be a whole number of pixels, not %q", v)
			}
			opts.Scale = n
		case "title":
			v, err := next()
			if err != nil {
				return opts, err
			}
			opts.Title = v
		default:
			return opts, fmt.Errorf("unrecognized option '%s'", a)
		}
	}

	if i < len(args) {
		opts.Script = args[i]
		opts.Args = args[i+1:]
		opts.ArgIdx = i + 2
	}
	return opts, nil
}

// EntryName is the file a directory of Lua is run from, as it is for LÖVE and
// for a fused tlua app.
const EntryName = "main.lua"

// scriptPath works out which file to run: the one named, the main.lua inside a
// directory, or the main.lua where the command was run.
func scriptPath(name string) (string, error) {
	if name == "" {
		name = "."
	}
	st, err := os.Stat(name)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return name, nil
	}
	entry := filepath.Join(name, EntryName)
	if _, err := os.Stat(entry); err != nil {
		return "", fmt.Errorf("%s has no %s to run", name, EntryName)
	}
	return entry, nil
}

// session is a loaded program: the interpreter, the console API on top of it,
// and the signal handling that lets Ctrl-C in the terminal close the window.
type session struct {
	opts   Options
	interp *interp.Interp
	rt     *picolua.Runtime

	// start runs the program's main chunk. There are two kinds of program
	// here — a file on disk, and one attached to this binary — and this is
	// the only thing that differs between them.
	start func() error

	sound *sound.Engine

	interrupt chan os.Signal
}

// load builds everything the program needs and runs its main chunk, stopping
// short of opening a window. Everything that can go wrong with a program goes
// wrong here, where there is still a terminal to say so on.
func load(opts Options) (*session, error) {
	script, err := scriptPath(opts.Script)
	if err != nil {
		return nil, err
	}

	title := opts.Title
	if title == "" {
		title = titleFor(script)
	}

	// The interpreter is the ordinary one, so that a game requires its modules
	// the same way any other tlua program does: from beside the script, from
	// TLUA_INCLUDE, from LUA_PATH.
	in := interp.New(&interp.Options{
		Script:       script,
		ScriptArgs:   opts.Args,
		ScriptArgIdx: opts.ArgIdx,
	})

	read, write := savesFor(title)

	s := &session{opts: opts, interp: in, sound: sound.New()}
	s.start = func() error {
		chunk, err := in.LoadScript(script)
		if err != nil {
			return err
		}
		return s.rt.Start(chunk, opts.Args)
	}
	s.rt = picolua.New(in.L, picolua.Options{
		Title:       title,
		Out:         os.Stdout,
		FPS:         ebiten.ActualFPS,
		SetTPS:      s.setRate,
		ButtonLabel: buttonLabel,
		ReadFile:    besideProgram(filepath.Dir(script)),
		Sound:       s.sound,
		ReadSave:    read,
		WriteSave:   write,
	})
	return s, nil
}

// RunFused shows a program attached to this binary, which is what a game built
// with `tlua fuse -play` runs when it is started.
//
// It is the same session as a program read off disk: the difference is only
// where the program and its modules come from, which the interpreter has
// already been told.
func RunFused(p *payload.Payload, exe string) int {
	f, err := interp.OpenFused(p, exe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", filepath.Base(exe), err)
		return 1
	}
	defer f.Close()

	read, write := savesFor(titleFor(exe))

	s := &session{interp: f.Interp, sound: sound.New()}
	// A fused program is handed the whole command line, as a .love executable
	// is, so there are no options of ours to read here.
	args := os.Args[1:]
	s.start = func() error {
		chunk, err := f.Chunk()
		if err != nil {
			return err
		}
		return s.rt.Start(chunk, args)
	}
	s.rt = picolua.New(f.L, picolua.Options{
		Title:       titleFor(exe),
		Out:         os.Stdout,
		FPS:         ebiten.ActualFPS,
		SetTPS:      s.setRate,
		ButtonLabel: buttonLabel,
		ReadFile:    attachedFirst(p),
		Sound:       s.sound,
		ReadSave:    read,
		WriteSave:   write,
	})
	return s.play()
}

// titleFor names the window after the program: the folder for a main.lua, the
// file itself otherwise.
func titleFor(script string) string {
	base := filepath.Base(script)
	if base == EntryName {
		if dir := filepath.Base(filepath.Dir(script)); dir != "." && dir != string(filepath.Separator) {
			return dir
		}
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func (s *session) close() {
	if s.interrupt != nil {
		signal.Stop(s.interrupt)
	}
	if s.sound != nil {
		s.sound.Close()
	}
	s.interp.Close()
}

// runMain runs the program's top level and its _init, which happen inside the
// coroutine the whole program runs in.
func (s *session) runMain() error {
	return s.start()
}

// report prints a Lua error the way the interpreter does and gives back an exit
// status. The message keeps its "file:line:" shape, which is what the editor
// reads to jump to the mistake.
func (s *session) report(err error) int {
	return s.interp.Report(err)
}

// Run loads a program and shows it in a window, reporting the process's exit
// status.
func Run(opts Options) int {
	s, err := load(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tlua play: %v\n", err)
		return 1
	}
	defer s.close()
	return s.play()
}

// play runs a loaded program and then shows it, whichever way it was loaded.
func (s *session) play() int {
	if err := s.runMain(); err != nil {
		return s.report(err)
	}
	// A program can be over before it starts: exit() in _init, or a script that
	// only wanted to print something.
	if quit, code := s.rt.Quitting(); quit {
		return code
	}

	// From here on the terminal is not where the program lives, but Ctrl-C
	// there should still close it. The interpreter has its own handler for
	// interrupting a chunk; this one ends the loop.
	s.interrupt = make(chan os.Signal, 1)
	signal.Notify(s.interrupt, os.Interrupt, syscall.SIGTERM)

	return s.show()
}

// show opens the window and runs the loop until the program, the person or a
// signal stops it.
func (s *session) show() int {
	a := &app{s: s, rt: s.rt}
	screen := s.rt.Screen()

	win, _ := s.rt.Window()
	scale := s.opts.Scale
	if scale <= 0 {
		scale = win.Scale
	}
	if scale <= 0 {
		mw, mh := monitorSize()
		scale = startScale(mw, mh, screen.W, screen.H)
	}

	ebiten.SetWindowTitle(win.Title)
	ebiten.SetWindowSize(screen.W*scale, screen.H*scale)
	ebiten.SetWindowSizeLimits(screen.W/2, screen.H/2, -1, -1)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	s.setRate(s.rt.TPS())
	if s.opts.Fullscreen || win.Fullscreen {
		ebiten.SetFullscreen(true)
	}

	if err := ebiten.RunGame(a); err != nil {
		return s.report(err)
	}
	return a.status
}

// setRate carries out a program's setfps(): how often it is run, and how fast
// a music fade counted in frames therefore goes.
//
// Everything that measures time in frames has to be told, or a program running
// at thirty would find its seconds twice as long as everyone else's.
func (s *session) setRate(rate int) {
	if rate == picolua.SyncWithDisplay {
		ebiten.SetTPS(ebiten.SyncWithFPS)
	} else {
		ebiten.SetTPS(rate)
	}
	if s.sound != nil {
		s.sound.SetTPS(rate)
	}
}

// monitorSize reports the monitor to size the first window against, and zeroes
// when there is no telling.
func monitorSize() (int, int) {
	m := ebiten.Monitor()
	if m == nil {
		return 0, 0
	}
	return m.Size()
}

// startScale picks how big a console pixel should be to begin with: the largest
// whole number that leaves a margin of the monitor around the window, within
// reason. A window that filled the screen exactly would have its title bar off
// the top of it.
func startScale(monitorW, monitorH, w, h int) int {
	const (
		fits    = 80 // per cent of the monitor to stay inside
		biggest = 6
	)
	if monitorW <= 0 || monitorH <= 0 || w <= 0 || h <= 0 {
		return 2
	}
	scale := min(monitorW*fits/100/w, monitorH*fits/100/h)
	return min(max(scale, 1), biggest)
}

// attachedFirst reads a file out of the program attached to the binary, and
// falls back to the disk.
//
// That way a game's artwork ships inside it and loadpng("art.png") finds it,
// while a file the person running it puts beside the executable is still
// readable. The archive comes first for the same reason require() prefers it:
// what was shipped is what the game was tested with.
func attachedFirst(p *payload.Payload) func(string) ([]byte, error) {
	return func(name string) ([]byte, error) {
		if p.Archive != nil {
			if data, err := p.Archive.Read(name); err == nil {
				return data, nil
			}
		}
		return os.ReadFile(name)
	}
}

// besideProgram reads a file named by a program, looking first where the
// program itself is.
//
// A game started from somewhere else — from a menu, from another directory —
// still means the artwork next to its own main.lua when it says
// loadpng("art.png"), which is the same rule require() follows for modules. An
// absolute path, and a relative one that is not there, are left to the working
// directory to answer for.
func besideProgram(dir string) func(string) ([]byte, error) {
	return func(name string) ([]byte, error) {
		if !filepath.IsAbs(name) {
			if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
				return data, nil
			}
		}
		return os.ReadFile(name)
	}
}

// savesFor reports where a program's own saved files go, and how to read and
// write them.
//
// They go where the person's other application data goes — Application Support
// on a Mac, ~/.config on Linux, AppData on Windows — under the program's own
// name, because a game may be run from a folder nobody can write to, from a
// read-only disk, or as a single fused executable with no folder of its own.
// Nothing is created until something is saved.
//
// On a machine with nowhere to put them, both come back nil, and a program that
// tries to save is told so rather than failing quietly.
func savesFor(program string) (read func(string) ([]byte, error), write func(string, []byte) error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, nil
	}
	dir := filepath.Join(base, "tlua", "saves", slug(program))

	read = func(name string) ([]byte, error) {
		if err := plainName(name); err != nil {
			return nil, err
		}
		return os.ReadFile(filepath.Join(dir, name))
	}
	write = func(name string, data []byte) error {
		if err := plainName(name); err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, name), data, 0o644)
	}
	return read, write
}

// plainName refuses anything that is not a file name, since these come from a
// program that may have been given one by whoever is playing.
func plainName(name string) error {
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, `/\`) || filepath.IsAbs(name) {
		return fmt.Errorf("%q is not a name a save can have", name)
	}
	return nil
}

// slug turns a program's name into one a folder can have.
func slug(name string) string {
	out := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_', r == '.':
			return r
		}
		return '-'
	}, name)
	out = strings.Trim(out, "-.")
	if out == "" {
		return "program"
	}
	return out
}
