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
	"tlua/internal/picolua"
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
	script string

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

	s := &session{opts: opts, interp: in, script: script}
	s.rt = picolua.New(in.L, picolua.Options{
		Title: title,
		Out:   os.Stdout,
		FPS:   ebiten.ActualFPS,
	})
	return s, nil
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
	s.interp.Close()
}

// runMain runs the program's top level, then its _init.
func (s *session) runMain() error {
	if err := s.interp.DoScript(s.script, s.opts.Args); err != nil {
		return err
	}
	return s.rt.Init()
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
	ebiten.SetTPS(TPS)
	if s.opts.Fullscreen || win.Fullscreen {
		ebiten.SetFullscreen(true)
	}

	if err := ebiten.RunGame(a); err != nil {
		return s.report(err)
	}
	return a.status
}

// TPS is how many times a second _update runs, which is what Picotron does and
// what the counted clock behind t() assumes.
const TPS = 60

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
