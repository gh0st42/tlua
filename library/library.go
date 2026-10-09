// Package library holds the declarations of tlua's own modules for
// lua-language-server: gui.lua for require "gui", pico.lua for the console.
// They are built into tlua, so that a language server started by tlua
// knows them on a machine with nothing but the binary.
package library

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed *.lua
var files embed.FS

// Dir writes the declarations to a folder of their own in the user's cache,
// once for each version of them, and returns it: the folder to give a
// language server as a library.
func Dir() (string, error) {
	names, err := fs.Glob(files, "*.lua")
	if err != nil {
		return "", err
	}
	sum := sha256.New()
	contents := map[string][]byte{}
	for _, name := range names {
		data, err := files.ReadFile(name)
		if err != nil {
			return "", err
		}
		contents[name] = data
		sum.Write([]byte(name))
		sum.Write(data)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "tlua", "library", hex.EncodeToString(sum.Sum(nil))[:12])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for name, data := range contents {
		path := filepath.Join(dir, name)
		if old, err := os.ReadFile(path); err == nil && string(old) == string(data) {
			continue
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// Settings are what a language server is told about a tlua program: Lua
// 5.1, the declarations in Dir, and the console's callbacks as globals.
// They are nil when the declarations cannot be written.
func Settings() map[string]any {
	dir, err := Dir()
	if err != nil {
		return nil
	}
	return map[string]any{"Lua": map[string]any{
		"runtime":     map[string]any{"version": "Lua 5.1"},
		"workspace":   map[string]any{"library": []string{dir}, "checkThirdParty": false},
		"diagnostics": map[string]any{"globals": []string{"_init", "_update", "_draw"}, "disable": []string{"lowercase-global"}},
	}}
}
