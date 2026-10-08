//go:build linux || darwin

package lualib

import "testing"

func TestSocketTCP(t *testing.T) {
	run(t, `
		local socket = require("socket.core")
		local server = assert(socket.tcp())
		assert(tostring(server):find("^tcp{master}"))
		assert(server:bind("127.0.0.1", 0))
		assert(server:listen())
		assert(tostring(server):find("^tcp{server}"))
		local ip, port = server:getsockname()
		assert(ip == "127.0.0.1" and type(port) == "string")

		local c = assert(socket.connect(ip, port))
		local s = assert(server:accept())
		s:settimeout(5)
		assert(c:send("hello\r\nworld\n12345partial") == 25)
		assert(c:send("abcdef", 2, 4) == 4)
		assert(s:receive() == "hello")
		assert(s:receive("*l") == "world")
		assert(s:receive(5) == "12345")
		assert(s:dirty())
		s:settimeout(0.05)
		local data, err, partial, elapsed = s:receive("*a")
		assert(data == nil and err == "timeout" and partial == "partialbcd", partial)
		assert(type(elapsed) == "number")
		c:send("xyz\n")
		s:settimeout(5)
		assert(s:receive("*l", "pre-") == "pre-xyz")

		c:send("a\nb")
		local r = socket.select({ s }, nil, 1)
		assert(r[1] == s and r[s] == 1)
		assert(s:receive() == "a")
		local t0 = socket.gettime()
		r = socket.select({ s }, nil, 5)
		assert(r[1] == s and socket.gettime() - t0 < 1, "buffered data makes a socket readable at once")
		assert(s:receive(1) == "b")
		local rr, ww, e = socket.select({ s }, nil, 0.01)
		assert(#rr == 0 and e == "timeout")

		assert(c:setoption("tcp-nodelay", true) == 1)
		assert(c:getoption("tcp-nodelay") == true)
		assert(not pcall(c.setoption, c, "bogus", true))
		assert(not pcall(server.send, server, "x"))

		c:close()
		assert(select(2, s:receive()) == "closed")
		assert(select(2, c:send("x")) == "closed")
		assert(c:getfd() == -1)

		local probe = assert(socket.tcp())
		probe:bind("127.0.0.1", 0)
		local _, free = probe:getsockname()
		probe:close()
		assert(select(2, socket.connect("127.0.0.1", free)) == "connection refused")
		assert(select(2, socket.tcp():bind("127.0.0.1", port)) == "address already in use")
	`)
}

func TestSocketUDP(t *testing.T) {
	run(t, `
		local socket = require("socket.core")
		local u = socket.udp()
		assert(u:setsockname("127.0.0.1", 0))
		local ip, port = u:getsockname()
		local v = socket.udp()
		assert(v:sendto("ping", ip, port) == 4)
		u:settimeout(2)
		local data, from = u:receivefrom()
		assert(data == "ping" and from == "127.0.0.1")
		assert(v:setpeername(ip, port))
		assert(tostring(v):find("^udp{connected}"))
		v:send("")
		assert(u:receive() == "")
		v:send("abcdef")
		assert(u:receive(2) == "ab")
		u:settimeout(0.01)
		assert(select(2, u:receive()) == "timeout")
		assert(v:setpeername("*"))
		assert(tostring(v):find("^udp{unconnected}"))
	`)
}

func TestSocketExcept(t *testing.T) {
	run(t, `
		local socket = require("socket.core")
		local cleaned = 0
		local try = socket.newtry(function() cleaned = cleaned + 1 end)
		local f = socket.protect(function(x)
			try(x, "failed")
			return "fine", x
		end)
		local a, b = f(1)
		assert(a == "fine" and b == 1)
		a, b = f(nil)
		assert(a == nil and b == "failed" and cleaned == 1)
		local ok, err = pcall(socket.protect(function() error("real") end))
		assert(not ok and err:find("real"))
		assert(select("#", socket.skip(1, "a", "b", "c")) == 2)
	`)
}

func TestSocketModulesLoad(t *testing.T) {
	// socket.lua and the protocol modules need nothing but socket.core and
	// mime.core underneath; check the names they reach for are there.
	run(t, `
		local core = require("socket.core")
		for _, name in ipairs({ "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6", "connect", "select",
				"sleep", "gettime", "newtry", "protect", "skip", "__unload" }) do
			assert(type(core[name]) == "function", name)
		end
		for _, name in ipairs({ "toip", "tohostname", "getaddrinfo", "getnameinfo", "gethostname" }) do
			assert(type(core.dns[name]) == "function", name)
		end
		assert(core._VERSION == "LuaSocket 3.0.0" and core._DEBUG == true)
		local mime = require("mime.core")
		for _, name in ipairs({ "b64", "unb64", "qp", "unqp", "wrp", "qpwrp", "eol", "dot" }) do
			assert(type(mime[name]) == "function", name)
		end
	`)
}

func TestStrtoul(t *testing.T) {
	for s, want := range map[string]int{"80": 80, "": 0, " 8080": 8080, "+21": 21, "70000": 70000 & 0xffff} {
		if n, ok := strtoul(s); !ok || int(uint16(n)) != want {
			t.Errorf("strtoul(%q) = %d, %v; want %d", s, n, ok, want)
		}
	}
	for _, s := range []string{"http", "8x", " ", "+"} {
		if _, ok := strtoul(s); ok {
			t.Errorf("strtoul(%q) read as a number", s)
		}
	}
}
