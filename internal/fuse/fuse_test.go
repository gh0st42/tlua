package fuse

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"tlua/internal/payload"
)

func readArchive(t *testing.T, data []byte) *payload.Archive {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	a, err := payload.NewArchive(zr)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestDefaultOutputName(t *testing.T) {
	cases := []struct {
		src, base, want string
	}{
		{"main.lua", "tlua", "main"},
		{filepath.Join("path", "to", "game.zip"), "tlua", "game"},
		{filepath.Join("path", "to", "mygame") + string(filepath.Separator), "tlua", "mygame"},
		{"main.lua", "tlua.exe", "main.exe"},
	}
	for _, c := range cases {
		if got := defaultOutputName(c.src, c.base); got != c.want {
			t.Errorf("defaultOutputName(%q, %q) = %q, want %q", c.src, c.base, got, c.want)
		}
	}
}

func TestBuildPayloadFromDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		payload.EntryName: "print(1)",
		"lib.lua":         "return 1",
		".DS_Store":       "junk",
		".git/config":     "junk",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	kind, data, err := buildPayload(dir)
	if err != nil {
		t.Fatal(err)
	}
	if kind != payload.Zip {
		t.Fatalf("kind = %q", rune(kind))
	}

	archive := readArchive(t, data)
	if !archive.Has(payload.EntryName) || !archive.Has("lib.lua") {
		t.Errorf("missing files: %v", archive.List())
	}
	for _, n := range archive.List() {
		if n == ".DS_Store" || n == ".git/config" {
			t.Errorf("dot file %q was packed", n)
		}
	}
}

func TestBuildPayloadRejectsDirectoryWithoutEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "other.lua"), []byte("return 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := buildPayload(dir); err == nil {
		t.Fatal("accepted a directory with no main.lua")
	}
}

func TestBuildPayloadFromLuaFile(t *testing.T) {
	src := filepath.Join(t.TempDir(), "app.lua")
	if err := os.WriteFile(src, []byte("print(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	kind, data, err := buildPayload(src)
	if err != nil {
		t.Fatal(err)
	}
	if kind != payload.Lua || string(data) != "print(1)" {
		t.Errorf("kind = %q, data = %q", rune(kind), data)
	}
}

func TestNoticingAProgramWrittenForTheConsole(t *testing.T) {
	games := []string{
		"function _draw()\nend\n",
		"function _update()\n end\n",
		"_draw = function() end\n",
		"local x = 1\nfunction _init() end\nfunction _draw() cls(1) end\n",
	}
	for _, src := range games {
		if !looksLikeAGame([]byte(src)) {
			t.Errorf("not noticed as a console program:\n%s", src)
		}
	}

	scripts := []string{
		"print('hello')\n",
		"-- this one mentions _draw in a comment only\n",
		"local drawing = {}\nfunction drawing.update() end\n",
		"function my_draw() end\n",
		"",
	}
	for _, src := range scripts {
		if looksLikeAGame([]byte(src)) {
			t.Errorf("wrongly taken for a console program:\n%s", src)
		}
	}
	if looksLikeAGame(nil) {
		t.Error("nothing at all is not a console program")
	}
}

func TestTheMainChunkIsFoundInsideAnArchive(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.lua"), []byte("function _draw() end\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	kind, data, err := buildPayload(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !looksLikeAGame(mainSource(kind, data)) {
		t.Error("a console program in a directory was not noticed")
	}
	if got := mainSource(payload.Zip, []byte("not a zip")); got != nil {
		t.Errorf("reading a main chunk out of nonsense gave %q", got)
	}
}
