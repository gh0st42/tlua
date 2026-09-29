package payload

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// fakeInterpreter stands in for a real binary: Open only cares about the tail.
var fakeInterpreter = bytes.Repeat([]byte("go binary\n"), 100)

func writeBinary(t *testing.T, parts ...[]byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app")
	var buf bytes.Buffer
	for _, p := range parts {
		buf.Write(p)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestOpenPlainInterpreter(t *testing.T) {
	p, err := Open(writeBinary(t, fakeInterpreter))
	if err != nil {
		t.Fatal(err)
	}
	if p != nil {
		t.Fatalf("found a payload in a plain binary: %+v", p)
	}
}

func TestOpenSourceTrailer(t *testing.T) {
	src := []byte(`print("hello")`)
	path := writeBinary(t, fakeInterpreter, src, Trailer(Lua, len(src)))

	p, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Kind != Lua {
		t.Errorf("kind = %q, want %q", rune(p.Kind), rune(Lua))
	}
	if string(p.Source) != string(src) {
		t.Errorf("source = %q", p.Source)
	}
	if p.PrefixLen != int64(len(fakeInterpreter)) {
		t.Errorf("prefix = %d, want %d", p.PrefixLen, len(fakeInterpreter))
	}
}

func TestOpenArchiveTrailer(t *testing.T) {
	data := zipBytes(t, map[string]string{EntryName: `print("zip")`, "lib/m.lua": "return 1"})
	path := writeBinary(t, fakeInterpreter, data, Trailer(Zip, len(data)))

	p, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Kind != Zip {
		t.Fatalf("kind = %q", rune(p.Kind))
	}
	body, err := p.Archive.Read(EntryName)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `print("zip")` {
		t.Errorf("entry = %q", body)
	}
	if p.PrefixLen != int64(len(fakeInterpreter)) {
		t.Errorf("prefix = %d, want %d", p.PrefixLen, len(fakeInterpreter))
	}
}

// TestOpenBareAppendedZip covers `cat tlua app.zip > app`, where the only clue
// is the zip's own directory.
func TestOpenBareAppendedZip(t *testing.T) {
	data := zipBytes(t, map[string]string{EntryName: `print("bare")`})
	path := writeBinary(t, fakeInterpreter, data)

	p, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Kind != Zip {
		t.Fatalf("kind = %q", rune(p.Kind))
	}
	if p.PrefixLen != int64(len(fakeInterpreter)) {
		t.Errorf("prefix = %d, want %d", p.PrefixLen, len(fakeInterpreter))
	}
}

func TestInterpreterPrefixStripsPayload(t *testing.T) {
	src := []byte("return 1")
	fused := writeBinary(t, fakeInterpreter, src, Trailer(Lua, len(src)))

	prefix, err := InterpreterPrefix(fused)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(prefix, fakeInterpreter) {
		t.Errorf("prefix is %d bytes, want the %d-byte interpreter", len(prefix), len(fakeInterpreter))
	}

	plain := writeBinary(t, fakeInterpreter)
	prefix, err = InterpreterPrefix(plain)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(prefix, fakeInterpreter) {
		t.Errorf("a plain binary should pass through unchanged")
	}
}

func TestArchiveUnwrapsSingleRootFolder(t *testing.T) {
	data := zipBytes(t, map[string]string{
		"mygame/" + EntryName: "print(1)",
		"mygame/lib/m.lua":    "return 1",
	})
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewArchive(zr)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Has(EntryName) || !a.Has("lib/m.lua") {
		t.Errorf("root folder not stripped: %v", a.List())
	}
	for _, n := range a.List() {
		if n != EntryName && n != "lib/m.lua" {
			t.Errorf("unexpected entry %q", n)
		}
	}
}

func TestArchiveWithoutEntryPointIsRejected(t *testing.T) {
	data := zipBytes(t, map[string]string{"other.lua": "return 1"})
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewArchive(zr); err == nil {
		t.Fatal("accepted an archive with no main.lua")
	}
}

func TestAKindIsAShapeAndHowToRunIt(t *testing.T) {
	cases := []struct {
		kind  Kind
		shape Kind
		game  bool
	}{
		{Lua, Lua, false},
		{Zip, Zip, false},
		{LuaGame, Lua, true},
		{ZipGame, Zip, true},
	}
	for _, c := range cases {
		if got := c.kind.Shape(); got != c.shape {
			t.Errorf("%q is shaped %q, want %q", rune(c.kind), rune(got), rune(c.shape))
		}
		if got := c.kind.Game(); got != c.game {
			t.Errorf("%q.Game() = %v, want %v", rune(c.kind), got, c.game)
		}
		if got := c.kind.AsGame(); !got.Game() || got.Shape() != c.shape {
			t.Errorf("%q.AsGame() = %q", rune(c.kind), rune(got))
		}
	}
}

func TestAGamePayloadKeepsItsMarkThroughTheTrailer(t *testing.T) {
	for _, kind := range []Kind{Lua, Zip, LuaGame, ZipGame} {
		program := []byte("print('x')\n")
		if kind.Shape() == Zip {
			program = zipBytes(t, map[string]string{EntryName: "print('x')\n"})
		}

		exe := filepath.Join(t.TempDir(), "app")
		body := append([]byte("binary"), program...)
		body = append(body, Trailer(kind, len(program))...)
		if err := os.WriteFile(exe, body, 0o755); err != nil {
			t.Fatal(err)
		}

		p, err := Open(exe)
		if err != nil {
			t.Fatalf("%q: %v", rune(kind), err)
		}
		if p == nil {
			t.Fatalf("%q: nothing was found attached", rune(kind))
		}
		if p.Kind != kind {
			t.Errorf("read back %q, want %q", rune(p.Kind), rune(kind))
		}
		if p.Kind.Game() != kind.Game() {
			t.Errorf("%q lost its mark", rune(kind))
		}
		if p.PrefixLen != int64(len("binary")) {
			t.Errorf("%q: the interpreter is %d bytes, want 6", rune(kind), p.PrefixLen)
		}
		p.Close()
	}
}
