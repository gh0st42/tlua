package library

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// The .luarc.json a language server reads from a project's folder:
// lua-language-server's own, which the VS Code extension, Neovim's lua_ls
// and other editors' Lua support all read.
const luarcName = ".luarc.json"

const luarcSchema = "https://raw.githubusercontent.com/LuaLS/vscode-lua/master/setting/schema.json"

// WriteLuaRC makes project's .luarc.json use the declarations in dir, as
// Config describes, or adds that to the one there. Settings it does not
// name are kept; their keys come out sorted. It says whether it made the
// file.
func WriteLuaRC(project, dir string) (made bool, err error) {
	path := filepath.Join(project, luarcName)
	rc := map[string]any{}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		made = true
		rc["$schema"] = luarcSchema
	case err != nil:
		return false, err
	default:
		if err := json.Unmarshal(data, &rc); err != nil {
			return false, fmt.Errorf("%s is not JSON tlua can read (%v)", path, err)
		}
	}
	merge(rc, Config(dir))
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rc); err != nil {
		return false, err
	}
	return made, os.WriteFile(path, out.Bytes(), 0o644)
}

// merge puts config's settings into rc: lists are joined, keeping what is
// there, and other values set, except checkThirdParty, which a project may
// have chosen.
func merge(rc map[string]any, config map[string]any) {
	for _, key := range []string{"runtime.version", "workspace.library", "workspace.checkThirdParty",
		"diagnostics.globals", "diagnostics.disable"} {
		want := lookup(config, key)
		got, set := find(rc, key)
		switch want := want.(type) {
		case []string:
			list, _ := got.([]any)
			if s, ok := got.(string); ok {
				list = []any{s}
			}
			for _, w := range want {
				if !contains(list, w) {
					list = append(list, w)
				}
			}
			set(list)
		default:
			if got == nil || key != "workspace.checkThirdParty" {
				set(want)
			}
		}
	}
}

// lookup reads a dotted key from Config's nested tables.
func lookup(m map[string]any, key string) any {
	first, rest, nested := strings.Cut(key, ".")
	if !nested {
		return m[key]
	}
	sub, _ := m[first].(map[string]any)
	return lookup(sub, rest)
}

// find looks a setting up in a .luarc.json, which may write it dotted
// ("runtime.version") or nested ({"runtime": {"version": ...}}), and gives
// what is there and how to change it in the same place. A setting not there
// is added dotted.
func find(m map[string]any, key string) (any, func(any)) {
	if v, ok := m[key]; ok {
		return v, func(nv any) { m[key] = nv }
	}
	if first, rest, ok := strings.Cut(key, "."); ok {
		if sub, ok := m[first].(map[string]any); ok {
			if v, set := find(sub, rest); v != nil {
				return v, set
			}
		}
	}
	return nil, func(nv any) { m[key] = nv }
}

func contains(list []any, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

const libUsage = `usage: tlua lib export [dir]
       tlua lib luarc [-lib dir] [project]

Writes the declarations of tlua's modules (gui, png, markdown, sys, tui,
socket, lfs, ...) for lua-language-server, so that an editor other than
tlua's own knows them: completion, hover help, and no "undefined" warnings
for what only tlua has.

  export   writes them to dir, by default %s
  luarc    exports them there, then writes the project's .luarc.json (the
           current folder by default) to use them, with Lua 5.1; settings
           already in it are kept

VS Code's Lua extension, Neovim's lua_ls and other editors read .luarc.json.
Run it again after updating tlua: the declarations are written anew.
`

// Command is `tlua lib`.
func Command(args []string) int {
	return command(args, os.Stdout, os.Stderr)
}

func command(args []string, stdout, stderr io.Writer) int {
	def, err := ExportDir()
	if err != nil {
		fmt.Fprintf(stderr, "tlua lib: %v\n", err)
		return 1
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintf(stdout, libUsage, def)
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	fs := flag.NewFlagSet("tlua lib "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintf(stderr, libUsage, def) }
	lib := def
	switch args[0] {
	case "export":
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() > 1 {
			return 2
		}
		if fs.NArg() == 1 {
			lib = fs.Arg(0)
		}
	case "luarc":
		fs.StringVar(&lib, "lib", def, "where the declarations go")
		if err := fs.Parse(args[1:]); err != nil || fs.NArg() > 1 {
			return 2
		}
	default:
		fmt.Fprintf(stderr, "tlua lib: no command %q\n", args[0])
		fs.Usage()
		return 2
	}
	lib, err = filepath.Abs(lib)
	if err != nil {
		fmt.Fprintf(stderr, "tlua lib: %v\n", err)
		return 1
	}
	n, err := Export(lib)
	if err != nil {
		fmt.Fprintf(stderr, "tlua lib: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Wrote %d declarations to %s\n", n, lib)

	if args[0] == "export" {
		fmt.Fprintf(stdout, "\nTo use them in a project, write its .luarc.json with\n\n"+
			"    tlua lib luarc [project]\n\n"+
			"or, for every project, in VS Code's settings.json:\n\n%s\n", indent(userSettings(lib)))
		return 0
	}
	project := "."
	if fs.NArg() == 1 {
		project = fs.Arg(0)
	}
	made, err := WriteLuaRC(project, lib)
	if err != nil {
		fmt.Fprintf(stderr, "tlua lib: %v\nAdd these settings to it by hand:\n\n%s\n", err, indent(userSettings(lib)))
		return 1
	}
	verb := "Updated"
	if made {
		verb = "Wrote"
	}
	fmt.Fprintf(stdout, "%s %s: Lua 5.1, with tlua's modules\n", verb, filepath.Join(project, luarcName))
	return 0
}

// userSettings is Config as VS Code's settings.json writes it.
func userSettings(dir string) string {
	lib, _ := json.Marshal(dir)
	return `"Lua.runtime.version": "Lua 5.1",
"Lua.workspace.library": [` + string(lib) + `],
"Lua.workspace.checkThirdParty": false,
"Lua.diagnostics.globals": ["_init", "_update", "_draw"]`
}

func indent(s string) string {
	return "    " + strings.ReplaceAll(s, "\n", "\n    ")
}
