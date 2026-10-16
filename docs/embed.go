// Package docs carries the documentation `tlua doc` shows: the pages on
// the gui, the designer and the console, and a page per module in modules/.
// Of the plans beside them, only rad-plan.md, which design.md links to.
package docs

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

//go:embed gui.md design.md pico.md rad-plan.md modules/*.md
var Files embed.FS

// Dir writes the pages to a folder of their own in the user's cache, once
// for each version of them, and returns it: for a viewer that follows the
// links between pages as files.
func Dir() (string, error) {
	var names []string
	err := fs.WalkDir(Files, ".", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, path)
		}
		return err
	})
	if err != nil {
		return "", err
	}
	sort.Strings(names)
	sum := sha256.New()
	contents := map[string][]byte{}
	for _, name := range names {
		data, _ := Files.ReadFile(name)
		contents[name] = data
		sum.Write([]byte(name))
		sum.Write(data)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "tlua", "docs", hex.EncodeToString(sum.Sum(nil))[:12])
	for name, data := range contents {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
}
