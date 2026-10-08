//go:build linux || darwin

package lualib

import (
	"net/netip"

	"golang.org/x/sys/unix"

	lua "github.com/yuin/gopher-lua"
)

// Socket options, options.c: getoption and setoption by name, with the
// names, argument checks and failures LuaSocket has. Which names a socket
// takes depends on whether it is TCP or UDP, and a few exist only on Linux.

type sockOpt func(L *lua.LState, fd int) int

func optBool(level, name int) (get, set sockOpt) {
	get = func(L *lua.LState, fd int) int {
		v, err := unix.GetsockoptInt(fd, level, name)
		if err != nil {
			return pushErr(L, "getsockopt failed")
		}
		L.Push(lua.LBool(v != 0))
		return 1
	}
	set = func(L *lua.LState, fd int) int {
		b, ok := L.Get(3).(lua.LBool)
		if !ok {
			L.ArgError(3, "boolean expected, got "+L.Get(3).Type().String())
		}
		v := 0
		if b {
			v = 1
		}
		return setResult(L, unix.SetsockoptInt(fd, level, name, v))
	}
	return
}

func optInt(level, name int) (get, set sockOpt) {
	get = func(L *lua.LState, fd int) int {
		v, err := unix.GetsockoptInt(fd, level, name)
		if err != nil {
			return pushErr(L, "getsockopt failed")
		}
		L.Push(lua.LNumber(v))
		return 1
	}
	set = func(L *lua.LState, fd int) int {
		n, _ := toNumber(L.Get(3))
		return setResult(L, unix.SetsockoptInt(fd, level, name, int(n)))
	}
	return
}

func setResult(L *lua.LState, err error) int {
	if err != nil {
		return pushErr(L, "setsockopt failed")
	}
	return pushOne(L)
}

func optGetLinger(L *lua.LState, fd int) int {
	l, err := unix.GetsockoptLinger(fd, unix.SOL_SOCKET, unix.SO_LINGER)
	if err != nil {
		return pushErr(L, "getsockopt failed")
	}
	tbl := L.NewTable()
	L.SetField(tbl, "on", lua.LBool(l.Onoff != 0))
	L.SetField(tbl, "timeout", lua.LNumber(l.Linger))
	L.Push(tbl)
	return 1
}

func optSetLinger(L *lua.LState, fd int) int {
	tbl, ok := L.Get(3).(*lua.LTable)
	if !ok {
		L.ArgError(3, "table expected, got "+L.Get(3).Type().String())
	}
	on, ok := L.GetField(tbl, "on").(lua.LBool)
	if !ok {
		L.ArgError(3, "boolean 'on' field expected")
	}
	secs, ok := toNumber(L.GetField(tbl, "timeout"))
	if !ok {
		L.ArgError(3, "number 'timeout' field expected")
	}
	l := &unix.Linger{Linger: int32(uint16(secs))}
	if on {
		l.Onoff = 1
	}
	return setResult(L, unix.SetsockoptLinger(fd, unix.SOL_SOCKET, unix.SO_LINGER, l))
}

func optGetError(L *lua.LState, fd int) int {
	v, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ERROR)
	if err != nil {
		return pushErr(L, "getsockopt failed")
	}
	if msg := sockStrerror(v); msg != "" {
		L.Push(lua.LString(msg))
	} else {
		L.Push(lua.LNil)
	}
	return 1
}

// ipv4Arg is inet_aton, for the option values that are IPv4 addresses.
func ipv4Arg(s string) ([4]byte, bool) {
	ip, err := netip.ParseAddr(s)
	if err != nil || !ip.Is4() {
		return [4]byte{}, false
	}
	return ip.As4(), true
}

func optSetMulticastIf(L *lua.LState, fd int) int {
	address := L.CheckString(3)
	var val [4]byte
	if address != "*" {
		var ok bool
		if val, ok = ipv4Arg(address); !ok {
			L.ArgError(3, "ip expected")
		}
	}
	return setResult(L, unix.SetsockoptInet4Addr(fd, unix.IPPROTO_IP, unix.IP_MULTICAST_IF, val))
}

func optGetMulticastIf(L *lua.LState, fd int) int {
	val, err := unix.GetsockoptInet4Addr(fd, unix.IPPROTO_IP, unix.IP_MULTICAST_IF)
	if err != nil {
		return pushErr(L, "getsockopt failed")
	}
	L.Push(lua.LString(netip.AddrFrom4(val).String()))
	return 1
}

func optMembership(name int) sockOpt {
	return func(L *lua.LState, fd int) int {
		tbl, ok := L.Get(3).(*lua.LTable)
		if !ok {
			L.ArgError(3, "table expected, got "+L.Get(3).Type().String())
		}
		multi, ok := L.GetField(tbl, "multiaddr").(lua.LString)
		if !ok {
			L.ArgError(3, "string 'multiaddr' field expected")
		}
		mreq := &unix.IPMreq{}
		if mreq.Multiaddr, ok = ipv4Arg(string(multi)); !ok {
			L.ArgError(3, "invalid 'multiaddr' ip address")
		}
		iface, ok := L.GetField(tbl, "interface").(lua.LString)
		if !ok {
			L.ArgError(3, "string 'interface' field expected")
		}
		if iface != "*" {
			if mreq.Interface, ok = ipv4Arg(string(iface)); !ok {
				L.ArgError(3, "invalid 'interface' ip address")
			}
		}
		return setResult(L, unix.SetsockoptIPMreq(fd, unix.IPPROTO_IP, name, mreq))
	}
}

func optIP6Membership(name int) sockOpt {
	return func(L *lua.LState, fd int) int {
		tbl, ok := L.Get(3).(*lua.LTable)
		if !ok {
			L.ArgError(3, "table expected, got "+L.Get(3).Type().String())
		}
		multi, ok := L.GetField(tbl, "multiaddr").(lua.LString)
		if !ok {
			L.ArgError(3, "string 'multiaddr' field expected")
		}
		ip, err := netip.ParseAddr(string(multi))
		if err != nil || !ip.Is6() {
			L.ArgError(3, "invalid 'multiaddr' ip address")
		}
		mreq := &unix.IPv6Mreq{Multiaddr: ip.As16()}
		if iface := L.GetField(tbl, "interface"); iface != lua.LNil {
			n, ok := toNumber(iface)
			if !ok {
				L.ArgError(3, "number 'interface' field expected")
			}
			mreq.Interface = uint32(n)
		}
		return setResult(L, unix.SetsockoptIPv6Mreq(fd, unix.IPPROTO_IPV6, name, mreq))
	}
}

// optPair is an option's getter and setter; either may be missing.
type optPair struct{ get, set sockOpt }

func boolOpt(level, name int) optPair { g, s := optBool(level, name); return optPair{g, s} }
func intOpt(level, name int) optPair  { g, s := optInt(level, name); return optPair{g, s} }

// optTables splits options into a getoption and a setoption table.
func optTables(sets ...map[string]optPair) (get, set map[string]sockOpt) {
	get, set = map[string]sockOpt{}, map[string]sockOpt{}
	for _, opts := range sets {
		for name, o := range opts {
			if o.get != nil {
				get[name] = o.get
			}
			if o.set != nil {
				set[name] = o.set
			}
		}
	}
	return get, set
}

var tcpGetOpts, tcpSetOpts = optTables(map[string]optPair{
	"keepalive":        boolOpt(unix.SOL_SOCKET, unix.SO_KEEPALIVE),
	"reuseaddr":        boolOpt(unix.SOL_SOCKET, unix.SO_REUSEADDR),
	"reuseport":        boolOpt(unix.SOL_SOCKET, unix.SO_REUSEPORT),
	"tcp-nodelay":      boolOpt(unix.IPPROTO_TCP, unix.TCP_NODELAY),
	"tcp-keepcnt":      intOpt(unix.IPPROTO_TCP, unix.TCP_KEEPCNT),
	"tcp-keepintvl":    intOpt(unix.IPPROTO_TCP, unix.TCP_KEEPINTVL),
	"linger":           {optGetLinger, optSetLinger},
	"error":            {optGetError, nil},
	"recv-buffer-size": intOpt(unix.SOL_SOCKET, unix.SO_RCVBUF),
	"send-buffer-size": intOpt(unix.SOL_SOCKET, unix.SO_SNDBUF),
	"ipv6-v6only":      {nil, boolOpt(unix.IPPROTO_IPV6, unix.IPV6_V6ONLY).set},
	"tcp-fastopen":     {nil, intOpt(unix.IPPROTO_TCP, unix.TCP_FASTOPEN).set},
}, tcpExtraOpts())

var udpGetOpts, udpSetOpts = optTables(map[string]optPair{
	"dontroute":          boolOpt(unix.SOL_SOCKET, unix.SO_DONTROUTE),
	"broadcast":          boolOpt(unix.SOL_SOCKET, unix.SO_BROADCAST),
	"reuseaddr":          boolOpt(unix.SOL_SOCKET, unix.SO_REUSEADDR),
	"reuseport":          boolOpt(unix.SOL_SOCKET, unix.SO_REUSEPORT),
	"ip-multicast-if":    {optGetMulticastIf, optSetMulticastIf},
	"ip-multicast-ttl":   {nil, intOpt(unix.IPPROTO_IP, unix.IP_MULTICAST_TTL).set},
	"ip-multicast-loop":  boolOpt(unix.IPPROTO_IP, unix.IP_MULTICAST_LOOP),
	"ip-add-membership":  {nil, optMembership(unix.IP_ADD_MEMBERSHIP)},
	"ip-drop-membership": {nil, optMembership(unix.IP_DROP_MEMBERSHIP)},
	"error":              {optGetError, nil},
	"ipv6-unicast-hops":  intOpt(unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS),
	// LuaSocket maps the multicast name to the unicast hops as well.
	"ipv6-multicast-hops":  intOpt(unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS),
	"ipv6-multicast-loop":  boolOpt(unix.IPPROTO_IPV6, unix.IPV6_MULTICAST_LOOP),
	"ipv6-add-membership":  {nil, optIP6Membership(unix.IPV6_JOIN_GROUP)},
	"ipv6-drop-membership": {nil, optIP6Membership(unix.IPV6_LEAVE_GROUP)},
	"ipv6-v6only":          boolOpt(unix.IPPROTO_IPV6, unix.IPV6_V6ONLY),
	"recv-buffer-size":     intOpt(unix.SOL_SOCKET, unix.SO_RCVBUF),
	"send-buffer-size":     intOpt(unix.SOL_SOCKET, unix.SO_SNDBUF),
})

func sockOption(L *lua.LState, set bool) int {
	_, s := anyGroup(L)
	name := L.CheckString(2)
	table := tcpGetOpts
	switch {
	case s.udp && set:
		table = udpSetOpts
	case s.udp:
		table = udpGetOpts
	case set:
		table = tcpSetOpts
	}
	fn, ok := table[name]
	if !ok {
		if len(name) > 35 {
			name = name[:35]
		}
		L.ArgError(2, "unsupported option `"+name+"'")
	}
	return fn(L, s.fd)
}

func sockGetoption(L *lua.LState) int { return sockOption(L, false) }
func sockSetoption(L *lua.LState) int { return sockOption(L, true) }
