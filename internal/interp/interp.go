// Package interp runs Lua code: it owns the interpreter state, the module
// search path, the arg table, interrupt handling and error reporting.
package interp

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	lua "github.com/yuin/gopher-lua"

	"tlua/internal/lualib"
)

// Action is one -e / -l / -p option, kept in command line order.
type Action struct {
	Kind rune // 'e' execute, 'l' require, 'p' search path
	Arg  string
}

// Options is what the command line asked for.
type Options struct {
	Actions     []Action
	Script      string // "" means none, "-" means stdin
	ScriptArgs  []string
	Interactive bool
	NoEnv       bool // -E: ignore TLUA_* and LUA_* variables

	// ScriptArgIdx is where the script name sits in os.Args, which is what
	// the negative half of the arg table counts back from.
	ScriptArgIdx int
}

// Interp owns the Lua state and everything around it.
type Interp struct {
	L    *lua.LState
	opts *Options
	// name prefixes error messages: "tlua", or the app name when fused.
	name string

	interrupt chan os.Signal
}

// New creates an interpreter with the standard libraries open and the search
// path, arg table and signal handling already set up.
func New(opts *Options) *Interp {
	L := lua.NewState(lua.Options{
		// The standard libraries are what make a plain .lua file run as-is.
		SkipOpenLibs:        false,
		IncludeGoStackTrace: false,
		RegistryMaxSize:     1024 * 1024,
		RegistryGrowStep:    512,
	})

	lualib.Open(L)

	r := &Interp{L: L, opts: opts, name: "tlua"}
	r.setupPath()
	r.setupArg()
	r.setupInterrupt()
	return r
}

func (r *Interp) Close() {
	if r.interrupt != nil {
		signal.Stop(r.interrupt)
	}
	lualib.Forget(r.L)
	r.L.Close()
}

// Environment variables tlua understands, on top of Lua's own LUA_PATH and
// LUA_INIT. These are how a machine declares its site-wide Lua libraries.
const (
	// EnvInclude is a list of directories, separated the way PATH is
	// (":" on Unix, ";" on Windows). Each becomes "<dir>/?.lua" and
	// "<dir>/?/init.lua"; an entry containing "?" is used as a pattern.
	EnvInclude = "TLUA_INCLUDE"
	// EnvPath is package.path patterns in Lua's own notation, for full
	// control; ";;" expands to the built-in default, as in LUA_PATH.
	EnvPath = "TLUA_PATH"
	// EnvInit is a chunk to run before anything else ("@file" runs a file),
	// taking precedence over LUA_INIT.
	EnvInit = "TLUA_INIT"
)

// setupPath builds package.path. Precedence, highest first:
//
//	-p options (in command line order)
//	TLUA_PATH        patterns, ";;" expanding to the default path
//	TLUA_INCLUDE     directories, PATH-style list
//	LUA_PATH         patterns, ";;" expanding to the default path
//	the built-in default ("./?.lua;/usr/local/share/lua/5.1/?.lua;...")
//
// -E drops every environment variable from that list. The directory of the
// script being run is prepended later, in doScript, so that a script's own
// requirements resolve no matter where it was invoked from.
func (r *Interp) setupPath() {
	def := lua.LuaPathDefault

	parts := []string{}
	for _, act := range r.opts.Actions {
		if act.Kind == 'p' {
			parts = append(parts, searchPath(act.Arg))
		}
	}

	if r.opts.NoEnv {
		parts = append(parts, def)
	} else {
		if v := os.Getenv(EnvPath); v != "" {
			parts = append(parts, expandDefault(v, def))
		}
		for _, dir := range splitDirList(os.Getenv(EnvInclude)) {
			parts = append(parts, searchPath(dir))
		}
		// LUA_PATH replaces the default rather than extending it, the way
		// the reference interpreter treats it.
		if v := os.Getenv("LUA_PATH"); v != "" {
			parts = append(parts, expandDefault(v, def))
		} else {
			parts = append(parts, def)
		}
	}

	r.setPackageField("path", dedupePath(strings.Join(parts, lua.LuaPathSep)))
	// gopher-lua has no C module loader (that is the point of avoiding cgo),
	// so cpath stays empty and require() says so in its error message.
	r.setPackageField("cpath", "")
}

// expandDefault applies Lua's ";;" rule: it stands for the default path.
func expandDefault(path, def string) string {
	return strings.ReplaceAll(path, ";;", lua.LuaPathSep+def+lua.LuaPathSep)
}

// splitDirList splits a PATH-style list of directories, ignoring empties.
func splitDirList(list string) []string {
	if list == "" {
		return nil
	}
	out := []string{}
	for _, dir := range strings.Split(list, string(os.PathListSeparator)) {
		if dir = strings.TrimSpace(dir); dir != "" {
			out = append(out, dir)
		}
	}
	return out
}

// dedupePath drops repeated and empty patterns, keeping the first occurrence.
// Several sources feeding one path makes duplicates easy, and they only cost
// failed stat calls on every require.
func dedupePath(path string) string {
	seen := map[string]bool{}
	out := []string{}
	for _, p := range strings.Split(path, lua.LuaPathSep) {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return strings.Join(out, lua.LuaPathSep)
}

func (r *Interp) setPackageField(field, value string) {
	pkg := r.L.GetGlobal("package")
	if tbl, ok := pkg.(*lua.LTable); ok {
		r.L.SetField(tbl, field, lua.LString(value))
	}
}

// prependPath puts dir's patterns at the front of package.path, once.
func (r *Interp) prependPath(dir string) {
	pkg, ok := r.L.GetGlobal("package").(*lua.LTable)
	if !ok {
		return
	}
	cur := lua.LVAsString(r.L.GetField(pkg, "path"))
	r.L.SetField(pkg, "path", lua.LString(dedupePath(searchPath(dir)+lua.LuaPathSep+cur)))
}

// setupArg fills the global arg table the way the reference interpreter does:
// arg[0] is the script, arg[1..n] its arguments, and negative indices walk
// back over the interpreter's own options.
func (r *Interp) setupArg() {
	tbl := r.L.NewTable()
	for i, a := range os.Args {
		r.L.RawSetInt(tbl, i-r.opts.ScriptArgIdx, lua.LString(a))
	}
	r.L.SetGlobal("arg", tbl)
}

// setupInterrupt makes Ctrl-C abort the running chunk instead of killing the
// process outright, so the REPL survives it.
func (r *Interp) setupInterrupt() {
	r.interrupt = make(chan os.Signal, 1)
	signal.Notify(r.interrupt, os.Interrupt, syscall.SIGTERM)
}

// Protect runs fn as a chunk is run: Ctrl-C or SIGTERM cancels the state's
// context while it does. A GUI's event loop runs after the program's main
// chunk this way, and stops when the program is told to.
func (r *Interp) Protect(fn func() error) error { return r.protect(fn) }

// protect runs fn with a context that a signal cancels; a cancelled chunk
// surfaces as an ordinary Lua error.
func (r *Interp) protect(fn func() error) error {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		select {
		case <-r.interrupt:
			cancel()
		case <-done:
		}
	}()
	r.L.SetContext(ctx)
	err := fn()
	close(done)
	cancel()
	r.L.RemoveContext()
	return err
}

func (r *Interp) DoString(src, name string) error {
	return r.protect(func() error {
		fn, err := r.L.Load(strings.NewReader(src), name)
		if err != nil {
			return err
		}
		r.L.Push(fn)
		return r.L.PCall(0, lua.MultRet, nil)
	})
}

// require implements -l. Like Lua 5.4 it accepts "-l g=mod" to choose the
// global the module lands in; "-l mod" uses the module name itself.
func (r *Interp) Require(spec string) error {
	global, mod := spec, spec
	if name, rest, ok := strings.Cut(spec, "="); ok {
		global, mod = name, rest
	}
	return r.protect(func() error {
		if err := r.L.CallByParam(lua.P{
			Fn:      r.L.GetGlobal("require"),
			NRet:    1,
			Protect: true,
		}, lua.LString(mod)); err != nil {
			return err
		}
		v := r.L.Get(-1)
		r.L.Pop(1)
		if v != lua.LNil {
			r.L.SetGlobal(global, v)
		}
		return nil
	})
}

// initChunk reports the startup chunk from the environment, TLUA_INIT first.
func InitChunk() string {
	if v := os.Getenv(EnvInit); v != "" {
		return v
	}
	return os.Getenv("LUA_INIT")
}

// runInit honours TLUA_INIT/LUA_INIT: "@file" runs a file, anything else is
// a chunk.
func (r *Interp) RunInit(init string) error {
	switch {
	case init == "":
		return nil
	case strings.HasPrefix(init, "@"):
		return r.DoScript(init[1:], nil)
	default:
		return r.DoString(init, "=INIT")
	}
}

// doScript loads a file from disk (or stdin for "-") and calls it with args as
// its varargs, exactly like `lua script.lua a b c`.
func (r *Interp) DoScript(path string, args []string) error {
	name := path
	if path == "-" {
		name = "" // LoadFile reads stdin for an empty path
	} else {
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if st, err := os.Stat(abs); err == nil && st.IsDir() {
			return fmt.Errorf("cannot run '%s': is a directory", path)
		}
		// A script's requirements live next to it far more often than in the
		// current directory, so its folder goes on the search path first.
		r.prependPath(filepath.Dir(abs))
		name = path
	}

	return r.protect(func() error {
		fn, err := r.L.LoadFile(name)
		if err != nil {
			return err
		}
		r.L.Push(fn)
		for _, a := range args {
			r.L.Push(lua.LString(a))
		}
		return r.L.PCall(len(args), lua.MultRet, nil)
	})
}

// LoadScript reads a script and hands back the chunk, ready to be called but
// not called yet.
//
// Running it is somebody else's business: a program written for the console is
// run inside a coroutine, so that it can suspend itself, and that is a thing
// only the console knows how to do. The search path is set up here all the
// same, so the modules it requires are found the way they always are.
func (r *Interp) LoadScript(path string) (*lua.LFunction, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if st, err := os.Stat(abs); err == nil && st.IsDir() {
		return nil, fmt.Errorf("cannot run '%s': is a directory", path)
	}
	r.prependPath(filepath.Dir(abs))
	return r.L.LoadFile(path)
}

// report prints a Lua error the way the reference interpreter does and gives
// back the process exit status.
func (r *Interp) Report(err error) int {
	if err == nil {
		return 0
	}
	msg := err.Error()
	if apiErr, ok := err.(*lua.ApiError); ok {
		msg = lua.LVAsString(apiErr.Object)
		if msg == "" {
			msg = apiErr.Error()
		}
		if apiErr.StackTrace != "" {
			msg += "\n" + strings.TrimRight(apiErr.StackTrace, "\n")
		}
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", r.name, msg)
	return 1
}

// searchPath turns a directory into package.path patterns. A plain directory
// becomes "<dir>/?.lua;<dir>/?/init.lua"; anything containing '?' is used
// verbatim so callers can spell out their own pattern.
func searchPath(p string) string {
	if strings.Contains(p, "?") {
		return p
	}
	p = strings.TrimSuffix(p, string(filepath.Separator))
	if p == "" {
		p = "."
	}
	return filepath.Join(p, "?.lua") + lua.LuaPathSep + filepath.Join(p, "?", "init.lua")
}
