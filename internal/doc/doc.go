// Package doc is `tlua doc`: the documentation built into tlua, shown in
// the terminal or in a window. The finding and the drawing are in Lua
// (lua/doc.lua, on the markdown module, and lua/window.lua, on the gui's
// MarkdownView); this is the command line, the pager and the pages.
package doc

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
	"golang.org/x/term"

	"tlua/docs"
	"tlua/internal/gui"
	"tlua/internal/interp"
	"tlua/internal/lualib"
)

//go:embed lua/*.lua
var sources embed.FS

const usage = `usage: tlua doc [-g] [-no-pager] [topic]

Shows the documentation built into tlua: a page for each module (sys,
term, tui, inspect, dkjson, argparse, socket, ...), Lua 5.1 itself (lua,
stdlib), the gui, the designer and the console (gui, design, pico).

  tlua doc              the contents, and the names of the pages
  tlua doc tui          a page
  tlua doc sys.copy     the part of a page about one thing
  tlua doc -g [topic]   the same in a window, with links to follow

In a terminal a long page goes through $PAGER (less, by default);
-no-pager prints it.
`

// pageName is what a page is called on the command line: modules/sys.md
// is sys, modules/README.md the index, gui.md gui.
func pageName(file string) string {
	if file == "modules/README.md" {
		return "index"
	}
	return strings.TrimSuffix(path.Base(file), ".md")
}

// pages are the pages' texts by name, and their files by name.
func pages() (texts map[string]string, files map[string]string, err error) {
	texts, files = map[string]string{}, map[string]string{}
	err = fs.WalkDir(docs.Files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := docs.Files.ReadFile(p)
		texts[pageName(p)] = string(data)
		files[pageName(p)] = p
		return err
	})
	return texts, files, err
}

// Command runs tlua doc.
func Command(args []string) int {
	return command(args, os.Stdout, os.Stderr)
}

func command(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("tlua doc", flag.ContinueOnError)
	fl.SetOutput(stderr)
	fl.Usage = func() { fmt.Fprint(stderr, usage) }
	window := fl.Bool("g", false, "show it in a window")
	noPager := fl.Bool("no-pager", false, "print it, without a pager")
	if err := fl.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fl.NArg() > 1 {
		fl.Usage()
		return 2
	}
	topic := fl.Arg(0)
	if *window {
		return openWindow(topic, stderr)
	}
	texts, _, err := pages()
	if err != nil {
		fmt.Fprintf(stderr, "tlua doc: %v\n", err)
		return 1
	}

	tty, width, height := terminal(stdout)
	color := tty && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	text, err := show(texts, topic, width, color)
	if err != nil {
		fmt.Fprintf(stderr, "tlua doc: %v\n", err)
		return 1
	}
	if tty && !*noPager && strings.Count(text, "\n") >= height-1 && page(text) {
		return 0
	}
	io.WriteString(stdout, text)
	return 0
}

// terminal says whether w is a terminal, and its size; 80 columns when it
// is not, and never wider than 100, which is as wide as prose reads well.
func terminal(w io.Writer) (tty bool, width, height int) {
	width, height = 80, 24
	f, ok := w.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return false, width, height
	}
	if w, h, err := term.GetSize(int(f.Fd())); err == nil {
		width, height = min(w, 100), h
	}
	return true, width, height
}

// Preload makes the documentation requirable: tlua.doc finds and draws
// pages, tlua.window is the window, tlua.help opens it on a topic, and
// tlua.docpages is the pages, written out to the user's cache the first
// time it is required. The designer's Help menu uses tlua.help.
func Preload(L *lua.LState) {
	L.PreloadModule("tlua.docpages", func(L *lua.LState) int {
		texts, files, err := pages()
		if err != nil {
			L.RaiseError("%s", err.Error())
		}
		dir, err := docs.Dir()
		if err != nil {
			L.RaiseError("the documentation cannot be written out: %s", err.Error())
		}
		paths := L.NewTable()
		for name, file := range files {
			paths.RawSetString(name, lua.LString(filepath.Join(dir, filepath.FromSlash(file))))
		}
		mod := L.NewTable()
		mod.RawSetString("texts", luaPages(L, texts))
		mod.RawSetString("files", paths)
		L.Push(mod)
		return 1
	})
	for _, name := range []string{"doc", "window", "help"} {
		file := "lua/" + name + ".lua"
		L.PreloadModule("tlua."+name, func(L *lua.LState) int {
			src, _ := sources.ReadFile(file)
			fn, err := L.Load(bytes.NewReader(src), "="+file)
			if err != nil {
				L.RaiseError("%s", err.Error())
			}
			L.Push(fn)
			L.Call(0, 1)
			return 1
		})
	}
}

func luaPages(L *lua.LState, texts map[string]string) *lua.LTable {
	t := L.NewTable()
	for name, text := range texts {
		t.RawSetString(name, lua.LString(text))
	}
	return t
}

// show is the text for a topic.
func show(texts map[string]string, topic string, width int, color bool) (string, error) {
	L := lua.NewState()
	defer L.Close()
	lualib.Open(L)
	Preload(L)
	opts := L.NewTable()
	opts.RawSetString("width", lua.LNumber(width))
	opts.RawSetString("color", lua.LBool(color))
	if err := L.CallByParam(lua.P{Fn: L.GetField(require(L, "tlua.doc"), "show"), NRet: 2, Protect: true},
		luaPages(L, texts), lua.LString(topic), opts); err != nil {
		return "", err
	}
	text, why := L.Get(-2), L.Get(-1)
	if text == lua.LNil {
		return "", fmt.Errorf("%s", lua.LVAsString(why))
	}
	return lua.LVAsString(text), nil
}

func require(L *lua.LState, name string) lua.LValue {
	L.Push(L.GetGlobal("require"))
	L.Push(lua.LString(name))
	L.Call(1, 1)
	mod := L.Get(-1)
	L.Pop(1)
	return mod
}

// page shows text through the user's pager, and says whether it could.
func page(text string) bool {
	cmdline := strings.Fields(os.Getenv("PAGER"))
	if len(cmdline) == 0 {
		cmdline = []string{"less"}
	}
	bin, err := exec.LookPath(cmdline[0])
	if err != nil {
		return false
	}
	cmd := exec.Command(bin, cmdline[1:]...)
	cmd.Stdin = strings.NewReader(text)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	// less shows the colours (R), stays on the screen afterwards (X) and
	// quits at once when it all fits (F), unless the user says otherwise.
	if os.Getenv("LESS") == "" {
		cmd.Env = append(os.Environ(), "LESS=FRX")
	}
	return cmd.Run() == nil
}

// openWindow is tlua doc -g: the pages, written out so that their links
// work as files, in a window.
func openWindow(topic string, stderr io.Writer) int {
	if !gui.Available {
		fmt.Fprintf(stderr, "tlua doc: %v; without -g it shows the pages here\n", gui.ErrUnavailable)
		return 1
	}
	r := interp.New(&interp.Options{})
	defer r.Close()
	gui.Ready(r, nil)
	Preload(r.L)
	L := r.L
	return r.Report(r.Protect(func() error {
		if err := L.CallByParam(lua.P{Fn: L.GetField(require(L, "tlua.help"), "open"), NRet: 2, Protect: true},
			lua.LString(topic), lua.LTrue); err != nil {
			return err
		}
		if ok, why := L.Get(-2), L.Get(-1); ok == lua.LNil {
			return fmt.Errorf("%s", lua.LVAsString(why))
		}
		return nil
	}))
}
