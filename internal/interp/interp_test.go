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

// The two fixes tlua carries in third_party/gopher-lua; PATCHES.md there has
// the whole story. Both broke ordinary programs without a word.
func TestGopherLuaPatches(t *testing.T) {
	r := New(&Options{})
	defer r.Close()
	for name, src := range map[string]string{
		"generic for over a call in an operand": `
			local function lines(text)
				local out = {}
				for line in (tostring(text) .. "\n"):gmatch("(.-)\n") do out[#out + 1] = line end
				return out
			end
			assert(#lines("a\nb") == 2)`,
		"generic for with two expressions": `
			local function f()
				do local a, b, c = 1, 2, "stale" end
				local n = 0
				for k in next, { x = 1, y = 2 } do n = n + 1 end
				return n
			end
			assert(f() == 2)`,
		"closures keep their upvalues after a caught error": `
			local n = 0
			local function inc() n = n + 1 end
			pcall(error, "x")
			inc()
			pcall(function() error({}) end)
			inc()
			assert(n == 2, n)
			local f
			pcall(function() local x = 1; f = function() return x end; x = 2; error("e") end)
			local junk = { 1, 2, 3 }
			assert(f() == 2)`,
		"string.format %q reads back": `
			local s = "quote \" backslash \\ newline \n cr \r nul \0 byte \1 high \200"
			assert(loadstring("return " .. string.format("%q", s))() == s)`,
		"string.format checks its arguments": `
			assert(not pcall(string.format, "%d"))
			assert(not pcall(string.format, "%d", "x"))
			assert(not pcall(string.format, "%y", 1))
			assert(string.format("%u|%5.2s|%c|%g", 42, "abc", 65, math.pi) == "42|   ab|A|3.14159")`,
		"tonumber reads exponents without a point": `
			assert(tonumber("1e5") == 1e5 and tonumber("1E5") == 1e5 and tonumber("2e-3") == 2e-3)
			assert(tonumber("1e+5") == 1e5 and tonumber("1e5", 10) == 1e5 and tonumber(" 1e5\r") == 1e5)
			assert(tonumber("1e") == nil and tonumber("e5") == nil and tonumber("1e5x") == nil)`,
		"strings become numbers as Lua 5.1 reads them": `
			assert("010" + 0 == 10 and "1e5" + 0 == 1e5 and " 0x10 " * 1 == 16 and "-0x10" + 0 == -16)
			assert(tonumber("0b101") == nil and tonumber("1_000") == nil and tonumber("inf") == nil)
			assert(not pcall(function() return "0b101" + 0 end))
			assert(tonumber("ff", 16) == 255 and tonumber("0xff", 16) == 255 and tonumber("8", 8) == nil)
			assert(tonumber("z", 36) == 35 and tonumber("-101", 2) == -5 and not pcall(tonumber, "1", 99))`,
		"string.match gives nil when nothing matches": `
			assert(select("#", ("ab"):match("x")) == 1 and ("ab"):match("x") == nil)
			assert(tonumber(("h2"):match("^h(%d)$")) == 2 and tonumber(("p"):match("^h(%d)$")) == nil)
			local t = { ("ab"):match("x"), "after" }
			assert(t[1] == nil and t[2] == "after")
			assert(select("#", ("ab"):find("x")) == 1)`,
		"multiple assignment computes every value first": `
			local a, b = 1, 2
			a, b = b, a
			assert(a == 2 and b == 1)
			local x, y, z = 1, 2, 3
			x, y, z = z, x, y
			assert(x == 3 and y == 1 and z == 2)
			local p, q, r = 8, 6, 3
			p, q, r = math.floor(p / 2), math.floor(q / 2), r * 2
			assert(p == 4 and q == 3 and r == 6)
			local function f(m, n, o)
				m, n, o = math.floor(m / 2), math.floor(n / 2), o * 2
				return m, n, o
			end
			local m, n, o = f(8, 6, 3)
			assert(m == 4 and n == 3 and o == 6)
			local t, v = { k = 1 }, 2
			t.k, v = v, t.k
			assert(t.k == 2 and v == 1)
			local s = { 1, 2 }
			s[1], s[2] = s[2], s[1]
			assert(s[1] == 2 and s[2] == 1)`,
		"math.huge is infinite": `
			assert(math.huge == 1/0 and tonumber("1e400") == math.huge)
			assert(tostring(math.huge) == "inf" and tostring(-math.huge) == "-inf")`,
	} {
		if err := r.DoString(src, name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
