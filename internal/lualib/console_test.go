package lualib

import (
	"path/filepath"
	"testing"
)

// The modules from turbolua (sys, term, tui) and the LuaRocks libraries
// carried with them (inspect, argparse, dkjson). examples/console has the
// demos, which were also run in a terminal.

func TestSys(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	run(t, `
		local sys = require("sys")
		local dir = "`+dir+`"
		assert(sys.writefile(dir .. "/a/b/c.txt", "hello"))
		assert(sys.readfile(dir .. "/a/b/c.txt") == "hello")
		assert(sys.exists(dir .. "/a/b/c.txt") and not sys.exists(dir .. "/none"))
		assert(sys.isdir(dir .. "/a/b") == true and sys.isdir(dir .. "/a/b/c.txt") == false)
		assert(sys.writefile(dir .. "/a/z.txt", "z"))
		local names = sys.listdir(dir .. "/a")
		assert(#names == 2 and names[1] == "b" and names[2] == "z.txt", table.concat(names, ","))
		assert(select(2, sys.listdir(dir .. "/a/z.txt")) == "not a directory: " .. dir .. "/a/z.txt")
		assert(sys.readfile(dir .. "/none") == nil)

		assert(sys.copy(dir .. "/a", dir .. "/copy"))
		assert(sys.readfile(dir .. "/copy/b/c.txt") == "hello")
		assert(sys.move(dir .. "/copy", dir .. "/moved"))
		assert(not sys.exists(dir .. "/copy") and sys.readfile(dir .. "/moved/z.txt") == "z")

		assert(sys.mkdir(dir .. "/m/n/o") and sys.isdir(dir .. "/m/n/o"))
		assert(sys.mkdir(dir .. "/m/n/o"))
		assert(select(2, sys.mkdir(dir .. "/a/z.txt")) == "path exists but is not a directory: " .. dir .. "/a/z.txt")
		assert(not sys.mkdir(dir .. "/p/q", false))
		assert(type(sys.cwd()) == "string")
		assert(select(2, sys.readfile("")) == "path must be a non-empty string")

		local t0 = require("socket").gettime()
		assert(sys.sleep(0.05) == true)
		local took = require("socket").gettime() - t0
		assert(took >= 0.04 and took < 1, took)
	`)
}

func TestTerm(t *testing.T) {
	file := filepath.ToSlash(filepath.Join(t.TempDir(), "out"))
	run(t, `
		local term = require("term")
		local f = assert(io.open("`+file+`", "wb"))
		term.colorize("yellow", "blue", f)
		term.textcolor(4, f)
		term.gotoxy(10, 5, f)
		term.clrscr(nil, f)
		term.cursor(false, f)
		term.println("{lightgreen}ok{reset} {{x}} {nocolor}", f)
		assert(term.textcolor("purple", f) == nil)
		f:close()
		local got = io.open("`+file+`", "rb"):read("*a")
		local want = "\27[93;44m" .. "\27[31m" .. "\27[5;10H" .. "\27[2J\27[H" .. "\27[?25l"
			.. "\27[92mok\27[0;37;40m {x} {nocolor}\n\27[0;37;40m"
		assert(got == want, ("%q"):format(got))
	`)
}

// A form driven by keys, as a person would: the drawing goes to a file.
func TestTui(t *testing.T) {
	t.Setenv("COLUMNS", "60")
	t.Setenv("LINES", "20")
	file := filepath.ToSlash(filepath.Join(t.TempDir(), "screen"))
	run(t, `
		local tui = require("tui")
		local screen = assert(io.open("`+file+`", "wb"))
		local out, stdout = io.output(), io.stdout
		io.output(screen)
		io.stdout = screen

		local app = tui.Application.new()
		assert(app.w == 60 and app.h == 20)
		local win = app:addchild(tui.Window.new(2, 2, 50, 15, "Form"))
		local name = win:addchild(tui.InputLine.new(2, 1, 20, ""))
		local box = win:addchild(tui.CheckBox.new(2, 3, "Subscribe"))
		local colors = win:addchild(tui.RadioButtons.new(2, 5, { "Red", "Green" }))
		local list = win:addchild(tui.ListBox.new(2, 8, 10, 2, { "a", "b", "c" }))
		local answer
		win:addchild(tui.Button.new(2, 11, "Go", function()
			answer = tui.messagebox(app, "Sure?", { "Yes", "No" })
		end))

		local keys = { "B", "o", "x", "backspace", "b", "tab", "space", "tab", "down", "tab",
			"down", "down", "down", "tab", "enter", "tab", "enter" }
		function app:readkey() return table.remove(keys, 1) end
		for _ = 1, 15 do app:handlekey(app:readkey()) end
		assert(name.text == "Bob", name.text)
		assert(box.checked and colors.selected == 2 and list.selected == 3)
		assert(answer == "No", tostring(answer))

		app:redraw()
		screen:close()
		io.output(out)
		io.stdout = stdout
		local drawn = io.open("`+file+`", "rb"):read("*a"):gsub("\27%[[%d;?]*%a", "")
		assert(drawn:find(" Form ", 1, true) and drawn:find("Bob", 1, true))
		assert(drawn:find("[x] Subscribe", 1, true) and drawn:find("(*) Green", 1, true))
		assert(tui.readkey ~= nil and tui.class(tui.View).new(1, 1, 1, 1).visible)
	`)
}

func TestRocks(t *testing.T) {
	run(t, `
		local inspect = require("inspect")
		assert(inspect({ 1, "a", k = { true } }) == '{ 1, "a",\n  k = { true }\n}')
		local t = {} t.self = t
		assert(inspect(t) == "<1>{\n  self = <table 1>\n}")
		assert(inspect({ a = { b = { c = 1 } } }, { depth = 1, newline = " ", indent = "" }) == "{ a = {...} }")

		local json = require("dkjson")
		assert(json.version == "dkjson 2.11")
		local s = json.encode({ 1, 2.5, 1/3, "é\n" })
		assert(s == '[1,2.5,0.33333333333333,"é\\n"]', s)
		local v = json.decode('{"a":[true,null,{"b":"\\u00e9"}]}', 1, json.null)
		assert(v.a[1] == true and v.a[2] == json.null and v.a[3].b == "é")
		local _, _, why = json.decode("{bad")
		assert(why == "invalid key at line 1, column 2", why)
		assert(json.encode({ b = 1, a = 2 }, { keyorder = { "a", "b" } }) == '{"a":2,"b":1}')

		local argparse = require("argparse")
		local p = argparse("greet", "Greets.")
		p:argument("name")
		p:option("-n --count", "times", "1"):convert(tonumber)
		p:flag("-v --verbose")
		local a = p:parse({ "Bob", "-n", "3", "-v" })
		assert(a.name == "Bob" and a.count == 3 and a.verbose == true)
		local ok, why = p:pparse({ "--count" })
		assert(not ok and why == "option '--count' requires an argument", why)
		assert(p:get_usage() == "Usage: greet [-h] [-n <count>] [-v] <name>")
	`)
}
