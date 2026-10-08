//go:build linux || darwin

package lualib

import (
	"maps"
	"net"
	"net/netip"
	"os"
	"slices"
	"time"

	"golang.org/x/sys/unix"

	lua "github.com/yuin/gopher-lua"
)

// socket.core, the C half of LuaSocket, in Go. socket.lua and the protocol
// modules written on it (socket.http, socket.ftp, socket.smtp, socket.tp,
// socket.url, socket.headers) run unchanged on top, and so does ltn12.
//
// It is LuaSocket 3.1's luasocket.c, except.c, select.c, inet.c, tcp.c and
// udp.c, over the same non-blocking descriptors, so select works on them
// and getfd gives a real descriptor. Names are resolved by Go rather than
// getaddrinfo(3), which keeps tlua free of cgo. There is no socket.unix or
// socket.serial.

func preloadSocket(L *lua.LState) {
	L.PreloadModule("socket.core", openSocketCore)
}

func openSocketCore(L *lua.LState) int {
	registerClasses(L)

	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"skip":     socketSkip,
		"__unload": func(L *lua.LState) int { return 0 },
		"gettime":  socketGettime,
		"sleep":    socketSleep,
		"select":   socketSelect,
		"tcp":      newTCP(unix.AF_UNSPEC),
		"tcp4":     newTCP(unix.AF_INET),
		"tcp6":     newTCP(unix.AF_INET6),
		"connect":  tcpConnectGlobal,
		"udp":      newUDP(unix.AF_UNSPEC),
		"udp4":     newUDP(unix.AF_INET),
		"udp6":     newUDP(unix.AF_INET6),
	})
	installExcept(L, mod)
	L.SetField(mod, "_VERSION", lua.LString("LuaSocket 3.0.0"))
	// The LuaRocks build, which is the one programs meet, is a debug build.
	L.SetField(mod, "_DEBUG", lua.LTrue)
	L.SetField(mod, "_SETSIZE", lua.LNumber(1024))
	L.SetField(mod, "_SOCKETINVALID", lua.LNumber(sockInvalid))
	L.SetField(mod, "_DATAGRAMSIZE", lua.LNumber(udpDatagramSize))
	L.SetField(mod, "dns", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"toip":        dnsToip,
		"getaddrinfo": dnsGetaddrinfo,
		"tohostname":  dnsTohostname,
		"getnameinfo": dnsGetnameinfo,
		"gethostname": dnsGethostname,
	}))
	L.Push(mod)
	return 1
}

// socketSkip is skip(n, ...): the arguments after the first n.
func socketSkip(L *lua.LState) int {
	amount := L.CheckInt(1)
	if ret := L.GetTop() - amount - 1; ret > 0 {
		return ret
	}
	return 0
}

func socketGettime(L *lua.LState) int {
	L.Push(lua.LNumber(gettime()))
	return 1
}

// socketSleep sleeps for n seconds, waking to notice an interrupt.
func socketSleep(L *lua.LState) int {
	n := float64(L.CheckNumber(1))
	if n < 0 {
		n = 0
	}
	if n > 1<<31-1 {
		n = 1<<31 - 1
	}
	deadline := time.Now().Add(time.Duration(n * float64(time.Second)))
	for {
		left := time.Until(deadline)
		if left <= 0 {
			return 0
		}
		time.Sleep(min(left, pollSlice*time.Millisecond))
		checkInterrupt(L)
	}
}

// installExcept is except.c: newtry and protect, LuaSocket's way of
// turning nil, err returns into errors and back. An error raised by a try
// is a table holding the message, with a metatable of its own, so protect
// can tell its own errors from any other.
func installExcept(L *lua.LState, mod *lua.LTable) {
	mt := L.NewTable()
	L.SetField(mt, "__metatable", lua.LFalse)

	L.SetField(mod, "newtry", L.NewFunction(func(L *lua.LState) int {
		finalizer := L.Get(1)
		L.Push(L.NewFunction(func(L *lua.LState) int {
			if L.ToBool(1) {
				return L.GetTop()
			}
			if finalizer != lua.LNil {
				L.Push(finalizer)
				L.Call(0, 0)
			}
			wrapped := L.NewTable()
			wrapped.RawSetInt(1, L.Get(2))
			wrapped.Metatable = mt
			L.Error(wrapped, 0)
			return 0
		}))
		return 1
	}))

	L.SetField(mod, "protect", L.NewFunction(func(L *lua.LState) int {
		fn := L.Get(1)
		L.Push(L.NewFunction(func(L *lua.LState) int {
			top := L.GetTop()
			L.Insert(fn, 1)
			if err := L.PCall(top, lua.MultRet, nil); err != nil {
				obj := lua.LValue(lua.LString(err.Error()))
				if apiErr, ok := err.(*lua.ApiError); ok {
					obj = apiErr.Object
				}
				if t, ok := obj.(*lua.LTable); ok && t.Metatable == mt {
					L.Push(lua.LNil)
					L.Push(t.RawGetInt(1))
					return 2
				}
				L.Error(obj, 0)
			}
			return L.GetTop()
		}))
		return 1
	}))
}

// socketSelect is select(recvt, sendt [, timeout]): which of the sockets
// can be read and written without blocking. A socket with data already
// buffered counts as readable at once. The results are lists that also map
// each socket back to its place in the list.
func socketSelect(L *lua.LState) int {
	t := float64(L.OptNumber(3, -1))
	L.SetTop(3)

	type entry struct {
		fd  int
		obj lua.LValue
	}
	byFd := map[int]lua.LValue{}
	getfd := func(obj lua.LValue) int {
		method := L.GetField(obj, "getfd")
		if method == lua.LNil {
			return sockInvalid
		}
		L.Push(method)
		L.Push(obj)
		L.Call(1, 1)
		fd := sockInvalid
		if n, ok := toNumber(L.Get(-1)); ok && n >= 0 {
			fd = int(n)
		}
		L.Pop(1)
		return fd
	}
	collect := func(n int) []entry {
		if L.Get(n) == lua.LNil {
			return nil
		}
		tab := L.CheckTable(n)
		var out []entry
		for i := 1; ; i++ {
			obj := tab.RawGetInt(i)
			if obj == lua.LNil {
				obj = L.GetTable(tab, lua.LNumber(i))
			}
			if obj == lua.LNil {
				return out
			}
			if fd := getfd(obj); fd != sockInvalid {
				byFd[fd] = obj
				out = append(out, entry{fd, obj})
			}
		}
	}
	readers := collect(1)
	writers := collect(2)

	rtab, wtab := L.NewTable(), L.NewTable()
	dirty := map[int]bool{}
	for _, e := range readers {
		method := L.GetField(e.obj, "dirty")
		if method == lua.LNil {
			continue
		}
		L.Push(method)
		L.Push(e.obj)
		L.Call(1, 1)
		if L.ToBool(-1) {
			rtab.Append(e.obj)
			dirty[e.fd] = true
		}
		L.Pop(1)
	}
	if len(dirty) > 0 {
		t = 0
	}

	events := map[int]int16{}
	for _, e := range readers {
		if !dirty[e.fd] {
			events[e.fd] |= unix.POLLIN
		}
	}
	for _, e := range writers {
		events[e.fd] |= unix.POLLOUT
	}
	fds := make([]unix.PollFd, 0, len(events))
	for _, fd := range slices.Sorted(maps.Keys(events)) {
		fds = append(fds, unix.PollFd{Fd: int32(fd), Events: events[fd]})
	}

	tm := timeout{block: t, total: -1}
	tm.markstart()
	ready := 0
	for {
		ms := -1
		if left := tm.getretry(); left >= 0 {
			ms = int(left * 1e3)
		}
		wait := ms
		if wait < 0 || wait > pollSlice {
			wait = pollSlice
		}
		n, err := unix.Poll(fds, wait)
		if err == unix.EINTR {
			checkInterrupt(L)
			continue
		}
		if err != nil {
			L.RaiseError("select failed")
		}
		if n > 0 || (ms >= 0 && ms <= pollSlice) {
			ready = n
			break
		}
		checkInterrupt(L)
	}

	// Results come in descriptor order, as select(2)'s bit sets give them.
	for _, p := range fds {
		if p.Revents&unix.POLLNVAL != 0 {
			L.RaiseError("select failed")
		}
		fd := int(p.Fd)
		if p.Events&unix.POLLIN != 0 && p.Revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) != 0 {
			rtab.Append(byFd[fd])
		}
		if p.Events&unix.POLLOUT != 0 && p.Revents&(unix.POLLOUT|unix.POLLHUP|unix.POLLERR) != 0 {
			wtab.Append(byFd[fd])
		}
	}
	if ready == 0 && len(dirty) == 0 {
		L.Push(L.NewTable())
		L.Push(L.NewTable())
		L.Push(lua.LString("timeout"))
		return 3
	}
	assoc := func(tab *lua.LTable) *lua.LTable {
		out := L.NewTable()
		for i := 1; i <= tab.Len(); i++ {
			v := tab.RawGetInt(i)
			out.RawSetInt(i, v)
			out.RawSet(v, lua.LNumber(i))
		}
		return out
	}
	L.Push(assoc(rtab))
	L.Push(assoc(wtab))
	return 2
}

// The dns table.

// pushResolved is the table toip and tohostname give back beside the
// answer.
func pushResolved(L *lua.LState, name string, addrs []string) {
	tbl := L.NewTable()
	L.SetField(tbl, "name", lua.LString(name))
	L.SetField(tbl, "alias", L.NewTable())
	ips := L.NewTable()
	for _, a := range addrs {
		ips.Append(lua.LString(a))
	}
	L.SetField(tbl, "ip", ips)
	L.Push(tbl)
}

func dnsToip(L *lua.LState) int {
	name, addrs, err := hostName(L.CheckString(1))
	if err != "" {
		return pushErr(L, err)
	}
	L.Push(lua.LString(addrs[0]))
	pushResolved(L, name, addrs)
	return 2
}

func dnsTohostname(L *lua.LState) int {
	name, addrs, err := hostName(L.CheckString(1))
	if err != "" {
		return pushErr(L, err)
	}
	L.Push(lua.LString(name))
	pushResolved(L, name, addrs)
	return 2
}

func dnsGetaddrinfo(L *lua.LState) int {
	host := L.CheckString(1)
	addrs, err := resolve(&host, "", unix.AF_UNSPEC, unix.SOCK_STREAM, false, false)
	if err != "" {
		return pushErr(L, err)
	}
	out := L.NewTable()
	for _, ai := range addrs {
		h, _, _ := numericHost(ai.sa)
		entry := L.NewTable()
		L.SetField(entry, "family", lua.LString(familyName(ai.family)))
		L.SetField(entry, "addr", lua.LString(h))
		out.Append(entry)
	}
	L.Push(out)
	return 1
}

// dnsGetnameinfo is getnameinfo(host, serv): the names of every address the
// host resolves to, and the service name of the port.
func dnsGetnameinfo(L *lua.LState) int {
	host, haveHost := optString(L, 1)
	serv, haveServ := optString(L, 2)
	if !haveHost && !haveServ {
		L.RaiseError("host and serv cannot be both nil")
	}
	hp := &host
	if !haveHost {
		hp = nil
	}
	addrs, err := resolve(hp, serv, unix.AF_UNSPEC, unix.SOCK_STREAM, false, false)
	if err != "" {
		return pushErr(L, err)
	}
	names := L.NewTable()
	port := 0
	for _, ai := range addrs {
		h, p, _ := numericHost(ai.sa)
		port = p
		if haveHost {
			if found, err := net.LookupAddr(netip.MustParseAddr(h).WithZone("").String()); err == nil && len(found) > 0 {
				h = trimDot(found[0])
			}
			names.Append(lua.LString(h))
		}
	}
	L.Push(names)
	if haveServ {
		L.Push(lua.LString(serviceName(port, unix.SOCK_STREAM)))
		return 2
	}
	return 1
}

func trimDot(s string) string {
	if len(s) > 1 && s[len(s)-1] == '.' {
		return s[:len(s)-1]
	}
	return s
}

func dnsGethostname(L *lua.LState) int {
	name, err := os.Hostname()
	if err != nil {
		return pushErr(L, sockStrerror(errnoOf(err)))
	}
	L.Push(lua.LString(name))
	return 1
}
