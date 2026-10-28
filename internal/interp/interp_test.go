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
		"recursion as deep as Lua 5.1 allows": `
			local function f(n) if n == 0 then return 0 end return 1 + f(n - 1) end
			assert(f(15000) == 15000)
			local co = coroutine.wrap(function() return f(10000) end)
			assert(co() == 10000)
			local function g() return 1 + g() end
			local ok, err = pcall(g)
			assert(not ok and tostring(err):find("stack overflow", 1, true), tostring(err))
			-- and the state is fine after it
			assert(f(100) == 100)`,
		"a yield across a call through Go is an error, not a crash": `
			local function resumeError(f)
				local co = coroutine.create(f)
				local ok, err = coroutine.resume(co)
				return ok, tostring(err)
			end
			local marker = "attempt to yield across metamethod/C-call boundary"
			-- a for-in iterator, a sort's comparison and a metamethod are calls through Go
			for name, f in pairs {
				iterator = function() for x in function() coroutine.yield(1) end do end end,
				sort = function() table.sort({3, 2, 1}, function(a, b) coroutine.yield() return a < b end) end,
				index = function() return setmetatable({}, {__index = function() coroutine.yield() end}).x end,
				gsub = function() return ("abc"):gsub("%w", function() coroutine.yield() end) end,
			} do
				local ok, err = resumeError(f)
				assert(not ok and err:find(marker, 1, true), name .. ": " .. err)
			end
			-- pcall is such a call: it catches the error, as in 5.1, and the coroutine goes on
			local co = coroutine.wrap(function()
				local ok, err = pcall(coroutine.yield, 1)
				assert(not ok and tostring(err):find(marker, 1, true), tostring(err))
				local ok2, err2 = pcall(function() coroutine.yield(2) end)
				assert(not ok2)
				-- a yield that is not across one still works afterwards
				local back = coroutine.yield("fine")
				return back
			end)
			assert(co() == "fine")
			assert(co("done") == "done")
			-- another coroutine resumed from inside a pcall yields to its resumer as ever
			local outer = coroutine.wrap(function()
				local inner = coroutine.wrap(function() coroutine.yield("inner") return "end" end)
				local ok, v = pcall(inner)
				coroutine.yield(v)
				return inner()
			end)
			assert(outer() == "inner" and outer() == "end")
			-- an error through nested calls leaves the count right
			local co3 = coroutine.wrap(function()
				pcall(function() return setmetatable({}, {__index = function() error("x") end}).y end)
				coroutine.yield("after")
			end)
			assert(co3() == "after")`,
		"syntax errors read as Lua's do": `
			local function err(src, name) return select(2, loadstring(src, name)) end
			assert(err("x = = 1", "=chunk") == "chunk:1: unexpected symbol near '='", err("x = = 1", "=chunk"))
			assert(err("x = 1\ny = = 2", "@f.lua") == "f.lua:2: unexpected symbol near '='")
			assert(err("if x then", "=c") == "c:1: unexpected symbol near '<eof>'", err("if x then", "=c"))
			assert(err("x = 1\nif x then\n  print(1)\n", "=c") == "c:4: unexpected symbol near '<eof>'", err("x = 1\nif x then\n  print(1)\n", "=c"))
			assert(err("local s = 'abc", "=c") == "c:1: unfinished string near '<eof>'")
			assert(err("x = [[abc", "=c") == "c:1: unfinished long string near '<eof>'")
			assert(err("x = 0x", "=c") == "c:1: malformed number near '0x'")
			-- and a tool that reads file:line: finds them
			local file, line = err("a = 1\nb = = 2", "@dir/prog.lua"):match("^([^:]+):(%d+):")
			assert(file == "dir/prog.lua" and line == "2")`,
		"file errors read as C's do": `
			local gone = "/nonexistent/dir/file.txt"
			local f, msg, errno = io.open(gone, "r")
			assert(f == nil and msg == gone .. ": No such file or directory" and errno == 2, tostring(msg) .. " " .. tostring(errno))
			local ok, msg, errno = os.remove(gone)
			assert(ok == nil and msg == gone .. ": No such file or directory" and errno == 2, tostring(msg))
			ok, msg, errno = os.rename(gone, gone .. "2")
			assert(ok == nil and msg == gone .. ": No such file or directory" and errno == 2, tostring(msg))
			local ok2, err = pcall(function() return io.lines(gone) end)
			assert(not ok2 and err:find("bad argument #1 to 'lines' (" .. gone .. ": No such file or directory)", 1, true), tostring(err))
			ok2, err = pcall(function() return io.input(gone) end)
			assert(not ok2 and err:find("cannot open file '" .. gone .. "' (No such file or directory)", 1, true), tostring(err))
			local fn, why = loadfile(gone)
			assert(fn == nil and why == "cannot open " .. gone .. ": No such file or directory", tostring(why))
			ok2, err = pcall(dofile, gone)
			assert(not ok2 and err == "cannot open " .. gone .. ": No such file or directory", tostring(err))
			ok2, err = pcall(require, "no.such.module")
			assert(not ok2 and err:find("module 'no.such.module' not found:\n\tno field package.preload['no.such.module']\n\tno file '", 1, true), tostring(err))`,
		"argument errors name the function in quotes": `
			local ok, err = pcall(function() return string.rep() end)
			assert(not ok and err:find("bad argument #1 to 'rep' (string expected", 1, true), tostring(err))
			ok, err = pcall(function() return ("x"):rep({}) end)
			assert(not ok and err:find("bad argument #2 to 'rep' (number expected, got table)", 1, true), tostring(err))
			ok, err = pcall(string.rep)
			assert(not ok and err:find("bad argument #1 to '?'", 1, true), tostring(err))`,
		"os.date as strftime does": `
			-- 2023-11-14 22:13:20 UTC, a Tuesday
			local t = 1700000000
			local want = {
				a = "Tue", A = "Tuesday", b = "Nov", B = "November", c = "Tue Nov 14 22:13:20 2023",
				C = "20", d = "14", D = "11/14/23", e = "14", F = "2023-11-14", g = "23", G = "2023",
				h = "Nov", H = "22", I = "10", j = "318", m = "11", M = "13", n = "\n", p = "PM",
				r = "10:13:20 PM", R = "22:13", S = "20", t = "\t", T = "22:13:20", u = "2", U = "46",
				V = "46", w = "2", W = "46", x = "11/14/23", X = "22:13:20", y = "23", Y = "2023",
				Z = "UTC", ["%"] = "%",
			}
			for conv, text in pairs(want) do
				assert(os.date("!%" .. conv, t) == text, "%" .. conv .. " = " .. os.date("!%" .. conv, t))
			end
			assert(os.date("!%Ey %Od", t) == "23 14", "E and O modifiers are ignored")
			assert(os.date("!%Q", t) == "%Q", "an unknown conversion stays as written")
			-- the weeks at the turn of a year
			assert(os.date("!%j %U %W %V %G", 1704067200) == "001 00 01 01 2024") -- Monday 2024-01-01
			assert(os.date("!%j %U %W %V %G", 1735689600) == "001 00 00 01 2025") -- Wednesday 2025-01-01
			assert(os.date("!%j %U %W %V %G", 1672531200) == "001 01 00 52 2022") -- Sunday 2023-01-01
			local tt = os.date("!*t", 86400 * 40)
			assert(tt.yday == 41 and tt.isdst == false and tt.month == 2 and tt.day == 10 and tt.wday == 3)`,
		"file:read('*n') reads a number as C's scanf does": `
			local function read(content, ...)
				local name = os.tmpname()
				local f = io.open(name, "w") f:write(content) f:close()
				local g = io.open(name)
				local got = {g:read(...)}
				local rest = g:read("*a")
				g:close() os.remove(name)
				return got, rest
			end
			local got, rest = read("0x10 -5 1e2 abc", "*n", "*n", "*n", "*n")
			assert(got[1] == 16 and got[2] == -5 and got[3] == 100 and got[4] == nil and rest == "abc")
			got = read(".5 5. -.5e-1 +7", "*n", "*n", "*n", "*n")
			assert(got[1] == 0.5 and got[2] == 5 and got[3] == -0.05 and got[4] == 7)
			got = read("0x1p4 0X.8", "*n", "*n")
			assert(got[1] == 16 and got[2] == 0.5)
			got, rest = read("1e", "*n")
			assert(got[1] == 1 and rest == "e", "an exponent without digits is not part of it")
			got, rest = read("-", "*n")
			assert(got[1] == nil and rest == "-")
			got, rest = read("abc", "*n")
			assert(got[1] == nil and rest == "abc")
			got, rest = read("12 34\nline", "*n", "*n", "*l", "*l")
			assert(got[1] == 12 and got[2] == 34 and got[3] == "" and got[4] == "line")`,
		"table.concat of a long table": `
			local t = {}
			for i = 1, 600000 do t[i] = "x" end
			assert(#table.concat(t) == 600000)
			assert(#table.concat(t, ",") == 1199999)
			assert(table.concat({1, 2.5, "x"}, "-") == "1-2.5-x")
			assert(table.concat({}, ",") == "" and table.concat({"a"}, ",") == "a")
			assert(table.concat({"a", "b", "c"}, ",", 2, 3) == "b,c")
			local ok, err = pcall(table.concat, {{}})
			assert(not ok and tostring(err):find("invalid value (table) at index 1 in table for concat", 1, true), tostring(err))`,
		"frontier patterns": `
			assert(select("#", ("THE (quick) fox"):find("%f[%a]%a+%f[%A]")) == 2)
			local i, j = ("THE (quick) fox"):find("%f[%a]%a+%f[%A]")
			assert(i == 1 and j == 3)
			assert(("hello world"):find("hello%f[%W]") == 1)
			assert(("hello"):find("hello%f[%z]") == 1, "the end of the string counts as a zero")
			assert(("xhello"):find("%f[%w]hello") == nil)
			assert(("a.b:onSubmit(x)"):gsub("(a%.b[:%.])onSubmit%f[^%w_]", "%1onEnter") == "a.b:onEnter(x)")
			assert(("onSubmitted"):find("onSubmit%f[^%w_]") == nil)
			assert(not pcall(string.find, "x", "%fx"), "%f needs a set")`,
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
		"numbers become strings with 14 digits": `
			assert(tostring(1/3) == "0.33333333333333" and 0.1 + 0.2 .. "" == "0.3")
			assert(tostring(2^53) == "9.007199254741e+15" and tostring(1e15) == "1e+15")
			assert(tostring(-0) == "-0" and tostring(100) == "100" and tostring(-1.5) == "-1.5")
			assert(tostring(123456789012) == "123456789012" and tostring(1e100) == "1e+100")`,
	} {
		if err := r.DoString(src, name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// Each chunk runs under a context of its own, ended with it. A coroutine
// made by one chunk and resumed by a later one, as a GUI's events resume
// what its main chunk made, runs under the later one's.
func TestCoroutineOutlivesItsChunk(t *testing.T) {
	r := New(&Options{})
	defer r.Close()
	if err := r.DoString(`
		co = coroutine.create(function()
			for i = 1, 3 do coroutine.yield(i) end
			return "done"
		end)
		assert(select(2, coroutine.resume(co)) == 1)`, "first"); err != nil {
		t.Fatal(err)
	}
	if err := r.DoString(`
		local ok, v = coroutine.resume(co)
		assert(ok and v == 2, tostring(v))
		local w = coroutine.wrap(function() coroutine.yield(1) end)
		wrapped = w`, "second"); err != nil {
		t.Fatal(err)
	}
	if err := r.DoString(`
		assert(select(2, coroutine.resume(co)) == 3)
		assert(select(2, coroutine.resume(co)) == "done")
		wrapped()`, "third"); err != nil {
		t.Fatal(err)
	}
}
