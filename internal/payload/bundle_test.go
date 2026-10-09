package payload

import (
	"os"
	"path/filepath"
	"testing"
)

func writeBundle(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestABundleIsKnownByItsName(t *testing.T) {
	for name, want := range map[string]bool{
		"app.ztl": true, "app.ZIP": true, "app.app": true,
		"app.lua": false, "app": false, "app.ztl.bak": false,
	} {
		if got := IsBundleName(name); got != want {
			t.Errorf("IsBundleName(%q) = %v, want %v", name, got, want)
		}
	}
	// A macOS application is a folder named .app, and not ours to run.
	dir := filepath.Join(t.TempDir(), "Real.app")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if IsBundle(dir) {
		t.Error("a folder named .app is not a bundle")
	}
	if !IsBundle(writeBundle(t, "a.ztl", zipBytes(t, map[string]string{"main.lua": ""}))) {
		t.Error("a .ztl file is a bundle")
	}
}

func TestOpenBundle(t *testing.T) {
	path := writeBundle(t, "app.ztl", zipBytes(t, map[string]string{
		"main.lua": "print(1)", "lib/x.lua": "return 1",
	}))
	p, err := OpenBundle(path)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Kind != Zip {
		t.Errorf("kind %q, want a script archive", rune(p.Kind))
	}
	if !p.Archive.Has("lib/x.lua") {
		t.Error("the bundle's modules should be in its archive")
	}
}

func TestABundleSaysItIsAGame(t *testing.T) {
	var buf = zipBytes(t, map[string]string{"main.lua": "print(1)"})
	// The comment is the last thing in a zip: its length, then itself.
	marked := append(buf[:len(buf)-2:len(buf)-2], byte(len(PlayMarker)), 0)
	marked = append(marked, PlayMarker...)
	p, err := OpenBundle(writeBundle(t, "game.ztl", marked))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if !p.Kind.Game() {
		t.Errorf("kind %q: a bundle marked %q is a game", rune(p.Kind), PlayMarker)
	}
}

func TestABundleWrittenForTheConsoleIsAGame(t *testing.T) {
	for src, game := range map[string]bool{
		"function _draw() cls() end":         true,
		"boot()\nfunction _draw() cls() end": false, // it asks for itself
		"print('hello')":                     false,
	} {
		p, err := OpenBundle(writeBundle(t, "a.zip", zipBytes(t, map[string]string{"main.lua": src})))
		if err != nil {
			t.Fatal(err)
		}
		if p.Kind.Game() != game {
			t.Errorf("%q: game = %v, want %v", src, p.Kind.Game(), game)
		}
		p.Close()
	}
}

func TestOpenBundleRejectsWhatIsNotOne(t *testing.T) {
	if _, err := OpenBundle(writeBundle(t, "a.ztl", []byte("not a zip"))); err == nil {
		t.Error("a file that is not a zip should be refused")
	}
	if _, err := OpenBundle(writeBundle(t, "a.ztl", zipBytes(t, map[string]string{"other.lua": ""}))); err == nil {
		t.Error("a zip without main.lua should be refused")
	}
}
