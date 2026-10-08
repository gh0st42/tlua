package lualib

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// run executes Lua source in a state with this package installed. The
// checks are written in Lua, with assert, so each reads as the program that
// would hit the behaviour.
func run(t *testing.T, src string) {
	t.Helper()
	L := lua.NewState()
	defer L.Close()
	Open(L)
	if err := L.DoString(src); err != nil {
		t.Fatal(err)
	}
}

func TestPopen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("popen is gopher-lua's own here")
	}
	run(t, `
		local p = io.popen("echo one; echo two")
		assert(io.type(p) == "file")
		assert(p:read("*l") == "one")
		assert(p:read("*a") == "two\n")
		assert(p:close() == 0)

		assert(io.popen("exit 3"):close() == 3)
		local w = io.popen("cat > /dev/null", "w")
		assert(w:write("data"))
		assert(io.close(w) == 0)

		local seen = {}
		for line in io.popen("printf 'a\\nb\\n'"):lines() do seen[#seen + 1] = line end
		assert(table.concat(seen, ",") == "a,b")
		assert(not pcall(io.popen, "true", "rw"))
	`)
}

// A command started by popen reads tlua's own stdin, as popen(3) arranges;
// gopher-lua's gave it /dev/null, which is what broke `stty size`.
func TestPopenInheritsStdin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("popen is gopher-lua's own here")
	}
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("from stdin\n")
	f.Seek(0, 0)
	old := os.Stdin
	os.Stdin = f
	defer func() { os.Stdin = old; f.Close() }()
	run(t, `
		local p = io.popen("cat")
		assert(p:read("*a") == "from stdin\n")
		p:close()
	`)
}

func TestOpenPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no umask")
	}
	dir := t.TempDir()
	ref, err := os.Create(filepath.Join(dir, "ref"))
	if err != nil {
		t.Fatal(err)
	}
	ref.Close()
	want, _ := os.Stat(filepath.Join(dir, "ref"))

	path := filepath.Join(dir, "lua.txt")
	run(t, `local f = assert(io.open("`+path+`", "w")); f:write("x"); f:close()`)
	got, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode().Perm() != want.Mode().Perm() {
		t.Errorf("io.open created %v, fopen would have %v", got.Mode().Perm(), want.Mode().Perm())
	}
	// An existing file keeps its mode.
	os.Chmod(path, 0o600)
	run(t, `local f = assert(io.open("`+path+`", "a")); f:write("y"); f:close()`)
	if st, _ := os.Stat(path); st.Mode().Perm() != 0o600 {
		t.Errorf("existing file changed to %v", st.Mode().Perm())
	}
}

func TestLfs(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	run(t, `
		local lfs = require("lfs")
		local dir = "`+dir+`"
		assert(lfs._VERSION == "LuaFileSystem 1.9.0")
		assert(lfs.mkdir(dir .. "/sub"))
		local ok, msg, errno = lfs.mkdir(dir .. "/sub")
		assert(ok == nil and msg == "File exists" and errno > 0, msg)
		assert(select(2, lfs.mkdir(dir .. "/a/b")) == "No such file or directory")

		local f = io.open(dir .. "/f.txt", "w"); f:write("hello"); f:close()
		local a = lfs.attributes(dir .. "/f.txt")
		assert(a.mode == "file" and a.size == 5 and #a.permissions == 9, a.mode)
		assert(lfs.attributes(dir .. "/sub", "mode") == "directory")
		assert(lfs.attributes(dir .. "/f.txt", {}).size == 5)
		local none, why = lfs.attributes(dir .. "/nope")
		assert(none == nil and why:find("^cannot obtain information from file"), why)
		assert(not pcall(lfs.attributes, dir, "colour"))

		local names = {}
		for name in lfs.dir(dir) do names[#names + 1] = name end
		table.sort(names)
		assert(table.concat(names, " ") == ". .. f.txt sub", table.concat(names, " "))
		local iter, d = lfs.dir(dir)
		assert(d:next() == ".")
		d:close()
		assert(not pcall(d.next, d))
		assert(not pcall(lfs.dir, dir .. "/nope"))

		assert(lfs.touch(dir .. "/f.txt", 1000, 2000))
		assert(lfs.attributes(dir .. "/f.txt", "access") == 1000)
		assert(lfs.attributes(dir .. "/f.txt", "modification") == 2000)
		assert(lfs.touch(dir .. "/nope") == nil)

		assert(select(2, lfs.rmdir(dir .. "/f.txt")) == "Not a directory")
		local cwd = lfs.currentdir()
		assert(lfs.chdir(dir .. "/sub"))
		assert(lfs.currentdir():find("sub$"))
		assert(lfs.chdir(cwd))
		assert(lfs.chdir(dir .. "/nope") == nil)

		local lock = assert(lfs.lock_dir(dir))
		assert(lfs.lock_dir(dir) == nil)
		lock:free()
		lfs.lock_dir(dir):free()
		assert(lfs.rmdir(dir .. "/sub"))
	`)
	if runtime.GOOS != "windows" {
		run(t, `
			local lfs = require("lfs")
			local dir = "`+dir+`"
			assert(lfs.link("f.txt", dir .. "/l", true))
			assert(lfs.symlinkattributes(dir .. "/l", "mode") == "link")
			assert(lfs.symlinkattributes(dir .. "/l").target == "f.txt")
			assert(lfs.symlinkattributes(dir .. "/l", "target") == "f.txt")
			assert(lfs.attributes(dir .. "/l", "mode") == "file")
			local none, why = lfs.symlinkattributes(dir .. "/f.txt", "target")
			assert(none == nil and why:find("^could not obtain link target"), why)
		`)
	}
}

// The expected values are what LuaSocket 3.1's C mime.core gives.
func TestMimeCore(t *testing.T) {
	run(t, `
		local m = require("mime.core")
		local function eq(want, ...)
			local got = { n = select("#", ...), ... }
			for i = 1, math.max(#want, got.n) do
				assert(want[i] == got[i], ("value %d: want %s, got %s"):format(i, tostring(want[i]), tostring(got[i])))
			end
		end
		eq({ "aGVsbG8gd29ybGQ=" }, m.b64("hello world"))
		eq({ "", "a" }, m.b64("a", ""))
		eq({ "YXh5", "" }, m.b64("a", "xy"))
		eq({}, m.b64(""))
		eq({ "hello world" }, m.unb64("aGVsbG8gd29ybGQ="))
		eq({ "", "xyz" }, m.unb64("x y \r\n z\t\r\n", ""))
		eq({ "x y=20\r\n z=09\r\n", "" }, m.qp("x y \r\n z\t\r\n", ""))
		eq({ "x y=20\n z=09\n", "" }, m.qp("x y \r\n z\t\r\n", "", "\n"))
		eq({ "A==ZZ" }, m.unqp("=41=3d=ZZ"))
		eq({ "a=\r\n=b", 74 }, m.qpwrp(3, "a=b"))
		eq({ "\r\na=b", 73 }, m.wrp(0, "a=b"))
		eq({ "\r\n", 76 }, m.wrp(5, nil))
		eq({ "x y \n z\t\n", 0 }, m.eol(13, "x y \r\n z\t\r\n", "\n"))
		eq({ "line\r\n..dot", 0 }, m.dot(2, "line\r\n.dot"))
		eq({ nil, 2 }, m.dot(1, nil))
		eq({ "MTIz" }, m.b64(123))
	`)
}
