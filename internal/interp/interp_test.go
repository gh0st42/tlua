package interp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchPath(t *testing.T) {
	dir := filepath.Join("opt", "lua")
	want := filepath.Join(dir, "?.lua") + ";" + filepath.Join(dir, "?", "init.lua")
	if got := searchPath(dir); got != want {
		t.Errorf("searchPath(%q) = %q, want %q", dir, got, want)
	}

	// A trailing separator should not produce an empty path element.
	if got := searchPath(dir + string(filepath.Separator)); got != want {
		t.Errorf("trailing separator: %q", got)
	}

	// Patterns are passed through untouched.
	pattern := "/site/?/main.lua"
	if got := searchPath(pattern); got != pattern {
		t.Errorf("searchPath(%q) = %q", pattern, got)
	}
}

func TestSplitDirList(t *testing.T) {
	sep := string(os.PathListSeparator)
	got := splitDirList("/a" + sep + " /b " + sep + sep + "/c")
	want := []string{"/a", "/b", "/c"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := splitDirList(""); got != nil {
		t.Errorf("empty list gave %v", got)
	}
}

func TestDedupePath(t *testing.T) {
	got := dedupePath("./?.lua;/a/?.lua;./?.lua;;/b/?.lua;/a/?.lua")
	want := "./?.lua;/a/?.lua;/b/?.lua"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandDefault(t *testing.T) {
	got := expandDefault("/site/?.lua;;", "./?.lua")
	if !strings.Contains(got, "./?.lua") || !strings.HasPrefix(got, "/site/?.lua") {
		t.Errorf("got %q", got)
	}
}
