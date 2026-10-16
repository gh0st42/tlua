// Package library holds the declarations of tlua's modules for
// lua-language-server: gui.lua for require "gui", pico.lua for the console,
// socket/http.lua for require "socket.http", and so on. They are built into
// tlua, so that a language server started by tlua knows them on a machine
// with nothing but the binary, and `tlua lib` writes them out for other
// editors.
package library

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

//go:embed *.lua socket/*.lua
var files embed.FS

// declarations are the embedded files by their path in the library folder,
// with slashes.
func declarations() (map[string][]byte, error) {
	contents := map[string][]byte{}
	err := fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".lua") {
			return err
		}
		data, err := files.ReadFile(path)
		contents[path] = data
		return err
	})
	return contents, err
}

// write puts the declarations in dir, leaving the ones already as they are.
func write(dir string, contents map[string][]byte) error {
	for name, data := range contents {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Dir writes the declarations to a folder of their own in the user's cache,
// once for each version of them, and returns it: the folder to give a
// language server as a library.
func Dir() (string, error) {
	contents, err := declarations()
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(contents))
	for name := range contents {
		names = append(names, name)
	}
	sort.Strings(names)
	sum := sha256.New()
	for _, name := range names {
		sum.Write([]byte(name))
		sum.Write(contents[name])
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "tlua", "library", hex.EncodeToString(sum.Sum(nil))[:12])
	return dir, write(dir, contents)
}

// Config is what a language server is told about a tlua program: Lua 5.1,
// the declarations in dir, and the console's callbacks as globals. The
// settings of the "Lua" section, as a .luarc.json has them.
func Config(dir string) map[string]any {
	return map[string]any{
		"runtime":     map[string]any{"version": "Lua 5.1"},
		"workspace":   map[string]any{"library": []string{dir}, "checkThirdParty": false},
		"diagnostics": map[string]any{"globals": []string{"_init", "_update", "_draw"}, "disable": []string{"lowercase-global"}},
	}
}

// Settings are Config for the declarations in Dir, as an editor sends them;
// nil when the declarations cannot be written.
func Settings() map[string]any {
	dir, err := Dir()
	if err != nil {
		return nil
	}
	return map[string]any{"Lua": Config(dir)}
}

// ExportDir is where `tlua lib export` puts the declarations unless told
// otherwise: one folder, whatever the version, so that an editor pointed at
// it once keeps finding them after tlua is updated and they are exported
// again.
func ExportDir() (string, error) {
	switch runtime.GOOS {
	case "darwin", "windows":
		base, err := os.UserConfigDir() // Application Support, %APPDATA%
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "tlua", "library"), nil
	}
	if base := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(base) {
		return filepath.Join(base, "tlua", "library"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "tlua", "library"), nil
}

// Export writes the declarations to dir and removes declarations an earlier
// export left there that this tlua no longer has. Other files in dir are left
// alone. It returns how many it wrote.
func Export(dir string) (int, error) {
	contents, err := declarations()
	if err != nil {
		return 0, err
	}
	if err := write(dir, contents); err != nil {
		return 0, err
	}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".lua") {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if _, ok := contents[filepath.ToSlash(rel)]; ok {
			return nil
		}
		if data, err := os.ReadFile(path); err == nil && bytes.HasPrefix(data, []byte("---@meta")) {
			return os.Remove(path)
		}
		return nil
	})
	return len(contents), err
}
