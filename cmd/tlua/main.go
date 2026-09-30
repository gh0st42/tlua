// Command tlua is a standalone Lua interpreter written in pure Go.
//
// It embeds gopher-lua (Lua 5.1), so the binary is statically linkable and
// needs no cgo, yet it still runs ordinary Lua files from disk together with
// the modules they require(). A program can also be attached to the binary,
// turning it into a standalone executable; see the fuse subcommand.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tlua/internal/editor"
	"tlua/internal/fuse"
	"tlua/internal/game"
	"tlua/internal/interp"
	"tlua/internal/payload"
	"tlua/internal/version"
)

// banner is what -v prints, and what greets an interactive session.
const banner = "tlua " + version.Number + " (Lua 5.1 via gopher-lua, pure Go)"

const usage = `usage: tlua [options] [script [args]]
       tlua edit [file...]
       tlua play [script | directory] [args]
       tlua fuse [-o output] [-play] <main.lua | directory | archive.zip>

Options:
  -e stat    execute string 'stat'
  -l name    require library 'name' into a global of the same name
             (use -l glob=name to pick the global)
  -p path    prepend 'path' to package.path (a directory, or a ?-pattern)
  -i         enter interactive mode after executing 'script'
  -v         print version information
  -E         ignore environment variables (TLUA_*, LUA_PATH, LUA_INIT)
  -h         print this help
  --         stop handling options
  -          execute stdin as a file and stop handling options

Environment:
  TLUA_INCLUDE   site-wide library directories, listed like PATH; each is
                 searched as <dir>/?.lua and <dir>/?/init.lua
  TLUA_PATH      package.path patterns, as LUA_PATH but tlua-only
  TLUA_INIT      chunk to run at startup ("@file" runs a file)
  TLUA_LSP       language server the editor formats, completes and hovers
                 with, or "off"; by default it looks for one on PATH
  TLUA_LOVE      love2d binary for the editor's LOVE run mode

The edit subcommand opens a full-screen Lua editor: a menu bar, several files
at once, F5 to run the primary file, F9 to check its syntax, and, when a
language server is on PATH, formatting on save, completion and hover help.

The play subcommand opens a window and runs a program against a fantasy
console in the spirit of PICO-8 and Picotron: a 480x270 screen, 64 colours,
sprites, input and a program built out of _update() and _draw(). The calls
are listed in docs/pico.md; "tlua play -h" explains the options.

The fuse subcommand attaches a Lua program to a copy of this binary, producing
a standalone executable; with -play the executable opens a window and runs the
program against the console. "tlua fuse -h" explains it. A zip concatenated
onto the binary (cat tlua app.zip > app) works the same way.
`

func main() {
	// A binary with a program attached stops being an interpreter: it runs
	// that program and hands it the entire command line.
	exe, err := os.Executable()
	if err == nil {
		p, perr := payload.Open(exe)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "tlua: %v\n", perr)
			os.Exit(1)
		}
		if p != nil {
			defer p.Close()
			// A program fused with -play wants a window and the console API;
			// anything else is run as an ordinary script.
			if p.Kind.Game() {
				os.Exit(game.RunFused(p, exe))
			}
			os.Exit(runFused(p, exe))
		}
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "fuse":
			os.Exit(fuse.Command(os.Args[2:]))
		case "edit":
			os.Exit(editCommand(os.Args[2:]))
		case "play":
			os.Exit(game.Command(os.Args[2:]))
		}
	}

	c, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "tlua: %v\n%s", err, usage)
		os.Exit(1)
	}
	os.Exit(run(c))
}

// cli is one parsed command line: the options the interpreter answers itself,
// plus everything it passes on to interp.
type cli struct {
	opts        *interp.Options
	showVersion bool
	showHelp    bool
}

// parseArgs reads the interpreter's own options, stopping at the script name.
func parseArgs(args []string) (*cli, error) {
	c := &cli{opts: &interp.Options{ScriptArgIdx: len(args) + 1}}
	opts := c.opts

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

		// Options with an argument accept both "-efoo" and "-e foo".
		takeArg := func(name rune) (string, error) {
			if len(a) > 2 {
				return a[2:], nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("'-%c' needs an argument", name)
			}
			i++
			return args[i], nil
		}

		switch a[1] {
		case 'e', 'l', 'p':
			v, err := takeArg(rune(a[1]))
			if err != nil {
				return nil, err
			}
			opts.Actions = append(opts.Actions, interp.Action{Kind: rune(a[1]), Arg: v})
		case 'i':
			if len(a) != 2 {
				return nil, fmt.Errorf("unrecognized option '%s'", a)
			}
			opts.Interactive = true
		case 'v':
			c.showVersion = true
		case 'E':
			opts.NoEnv = true
		case 'h':
			c.showHelp = true
		default:
			return nil, fmt.Errorf("unrecognized option '%s'", a)
		}
	}

	if i < len(args) {
		opts.ScriptArgIdx = i + 1 // +1: os.Args[0] is the interpreter itself
		opts.Script = args[i]
		opts.ScriptArgs = args[i+1:]
	}
	return c, nil
}

// run carries out the command line in the order the reference interpreter
// does: the startup chunk, then -e/-l in the order given, then the script,
// then interactive mode.
func run(c *cli) int {
	opts := c.opts

	if c.showHelp {
		fmt.Print(usage)
		return 0
	}
	if c.showVersion {
		fmt.Println(banner)
		if opts.Script == "" && len(opts.Actions) == 0 && !opts.Interactive {
			return 0
		}
	}

	r := interp.New(opts)
	defer r.Close()

	// A script may ask for a window while it runs. Nothing of the console is
	// built unless it does: most scripts are not games, and they expect
	// print() to write to the terminal.
	boot := game.Ready(r, game.Options{
		Script: opts.Script,
		Args:   opts.ScriptArgs,
		ArgIdx: opts.ScriptArgIdx,
	}, nil)

	if !opts.NoEnv {
		if err := r.RunInit(interp.InitChunk()); err != nil {
			return r.Report(err)
		}
	}

	for _, act := range opts.Actions {
		var err error
		switch act.Kind {
		case 'e':
			err = r.DoString(act.Arg, "=(command line)")
		case 'l':
			err = r.Require(act.Arg)
		case 'p':
			// already applied while building package.path
		}
		if err != nil {
			return r.Report(err)
		}
	}

	if opts.Script != "" {
		if err := r.DoScript(opts.Script, opts.ScriptArgs); err != nil {
			return r.Report(err)
		}
	}

	// The program has finished saying what it is. If it asked for a window,
	// that is the rest of its life — unless an interactive session was asked
	// for as well, which then picks up where the window left off.
	if boot.Wanted() {
		status := boot.Show()
		if !opts.Interactive {
			return status
		}
	}
	boot.TooLate()

	if opts.Interactive {
		r.REPL()
		return 0
	}

	// No script and no -e: behave like lua and read a program from stdin,
	// or start a REPL when stdin is a terminal.
	if opts.Script == "" && len(opts.Actions) == 0 && !c.showVersion {
		if isTerminal(os.Stdin) {
			fmt.Println(banner)
			r.REPL()
			return 0
		}
		if err := r.DoScript("-", nil); err != nil {
			return r.Report(err)
		}
	}
	return 0
}

// editCommand opens the editor on the named files.
func editCommand(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Print("usage: tlua edit [file...]\n\n" +
				"Opens a full-screen Lua editor. F1 lists the keys.\n" +
				"A language server on PATH adds formatting on save,\n" +
				"completion on Ctrl-Space and help on F11; TLUA_LSP\n" +
				"names another one, or \"off\" for none.\n")
			return 0
		}
	}
	ed, err := editor.New(editor.Config{Files: args})
	if err != nil {
		fmt.Fprintf(os.Stderr, "tlua edit: %v\n", err)
		return 1
	}
	if err := ed.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tlua edit: %v\n", err)
		return 1
	}
	return 0
}

// isTerminal reports whether f is a tty, which is how the interpreter decides
// between a REPL and reading a program from a pipe.
func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

// runFused runs a program attached to this binary that was not built with
// -play, giving it the chance to ask for a window with boot() anyway.
//
// That is what makes -play optional: a program that says boot() in its own text
// does not also have to be told at the moment it is packed.
func runFused(p *payload.Payload, exe string) int {
	f, err := interp.OpenFused(p, exe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", filepath.Base(exe), err)
		return 1
	}
	defer f.Close()

	args := os.Args[1:]
	boot := game.Ready(f.Interp, game.Options{Title: filepath.Base(exe), Args: args},
		game.Attached(p))

	if err := f.Run(args); err != nil {
		return f.Report(err)
	}
	if boot.Wanted() {
		return boot.Show()
	}
	return 0
}
