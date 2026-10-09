package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirHoldsTheDeclarations(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir()) // macOS keeps its cache under HOME
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gui.lua", "pico.lua"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	again, err := Dir()
	if err != nil || again != dir {
		t.Errorf("a second call gave %q, %v; want the same folder", again, err)
	}
	if Settings() == nil {
		t.Error("no settings")
	}
}
