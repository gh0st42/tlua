package fuse

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"tlua/internal/payload"
)

func zipComment(t *testing.T, data []byte) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return zr.Comment
}

func TestBundleADirectory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.lua"), []byte("print(1)"), 0o644)
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0o644)
	// A bundle written into the folder it is made of, by an earlier run.
	out := filepath.Join(dir, "app.ztl")
	os.WriteFile(out, []byte("old bundle"), 0o644)

	data, err := Bundle(dir, out, false)
	if err != nil {
		t.Fatal(err)
	}
	a := readArchive(t, data)
	if got := a.List(); len(got) != 1 || got[0] != "main.lua" {
		t.Errorf("bundle holds %v, want only main.lua", got)
	}
	if c := zipComment(t, data); c != "" {
		t.Errorf("a script's bundle has comment %q", c)
	}
}

func TestBundleOneFileAsAGame(t *testing.T) {
	src := filepath.Join(t.TempDir(), "game.lua")
	os.WriteFile(src, []byte("function _draw() end"), 0o644)
	data, err := Bundle(src, "game.ztl", true)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := readArchive(t, data).Read("main.lua"); string(got) != "function _draw() end" {
		t.Errorf("main.lua is %q", got)
	}
	if c := zipComment(t, data); c != payload.PlayMarker {
		t.Errorf("comment %q, want %q", c, payload.PlayMarker)
	}

	// Fusing the bundle keeps it a game.
	path := filepath.Join(t.TempDir(), "game.ztl")
	os.WriteFile(path, data, 0o644)
	kind, _, err := buildPayload(path)
	if err != nil {
		t.Fatal(err)
	}
	if !kind.Game() {
		t.Error("fusing a -play bundle should make a game")
	}
}

func TestBundleName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Notes")
	for src, want := range map[string]string{
		dir:                            "Notes",
		filepath.Join(dir, "main.lua"): "Notes",
		filepath.Join(dir, "pong.lua"): "pong",
	} {
		if got := bundleName(src); got != want {
			t.Errorf("bundleName(%q) = %q, want %q", src, got, want)
		}
	}
}
