//go:build linux || darwin

package lualib

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/unix"

	lua "github.com/yuin/gopher-lua"
)

// The socket objects of socket.core: tcp.c, udp.c and the parts of inet.c
// they share. An object's class is its state, as in LuaSocket: a TCP socket
// starts as a master and becomes a client by connecting or a server by
// listening, and each class has its own metatable, so that a method called
// on an object in the wrong state fails the way the C one does.

const (
	classTCPMaster      = "tcp{master}"
	classTCPClient      = "tcp{client}"
	classTCPServer      = "tcp{server}"
	classUDPConnected   = "udp{connected}"
	classUDPUnconnected = "udp{unconnected}"
	groupTCP            = "tcp{any}"
	groupUDP            = "udp{any}"
)

type lsock struct {
	class  string
	udp    bool
	fd     int
	family int
	tm     timeout
	buf    sockBuffer // TCP only
}

func (s *lsock) inGroup(name string) bool {
	switch name {
	case groupTCP:
		return !s.udp
	case groupUDP:
		return s.udp
	}
	return s.class == name
}

// newSock wraps a socket in a userdata of the given class. A socket the
// program forgets about is closed once Go collects it; gopher-lua runs no
// __gc, so the finalizer stands in for it.
func newSock(L *lua.LState, class string, udp bool, fd, family int) (*lua.LUserData, *lsock) {
	s := &lsock{class: class, udp: udp, fd: fd, family: family, tm: newTimeout()}
	s.buf.birthday = gettime()
	runtime.SetFinalizer(s, func(s *lsock) { sockDestroy(&s.fd) })
	ud := L.NewUserData()
	ud.Value = s
	L.SetMetatable(ud, L.GetTypeMetatable(class))
	return ud, s
}

func setClass(L *lua.LState, ud *lua.LUserData, s *lsock, class string) {
	s.class = class
	L.SetMetatable(ud, L.GetTypeMetatable(class))
}

// checkSock is auxiliar_checkclass and auxiliar_checkgroup.
func checkSock(L *lua.LState, want string) (*lua.LUserData, *lsock) {
	ud, ok := L.Get(1).(*lua.LUserData)
	if ok {
		if s, ok := ud.Value.(*lsock); ok && s.inGroup(want) {
			return ud, s
		}
	}
	L.ArgError(1, want+" expected")
	return nil, nil
}

// registerClasses makes the metatables, as auxiliar_newclass: the methods
// and the class name in __index, __gc and __tostring on the metatable.
func registerClasses(L *lua.LState) {
	add := func(classes []string, methods map[string]lua.LGFunction) {
		for _, class := range classes {
			mt := L.NewTypeMetatable(class)
			index := L.SetFuncs(L.NewTable(), methods)
			L.SetField(index, "class", lua.LString(class))
			L.SetField(mt, "__index", index)
			L.SetField(mt, "__tostring", L.NewFunction(sockTostring))
			L.SetField(mt, "__gc", L.NewFunction(sockClose))
		}
	}
	add([]string{classTCPMaster, classTCPClient, classTCPServer}, tcpMethods)
	add([]string{classUDPConnected, classUDPUnconnected}, udpMethods)
	for _, c := range []string{classTCPMaster, classTCPClient, classTCPServer} {
		L.SetField(L.GetTypeMetatable(c), groupTCP, lua.LTrue)
	}
	for _, c := range []string{classUDPConnected, classUDPUnconnected} {
		mt := L.GetTypeMetatable(c)
		L.SetField(mt, groupUDP, lua.LTrue)
		L.SetField(mt, "select{able}", lua.LTrue)
	}
}

func sockTostring(L *lua.LState) int {
	ud := L.CheckUserData(1)
	s, ok := ud.Value.(*lsock)
	if !ok {
		L.RaiseError("invalid object passed to 'auxiliar.c:__tostring'")
	}
	L.Push(lua.LString(fmt.Sprintf("%s: %p", s.class, s)))
	return 1
}

var tcpMethods = map[string]lua.LGFunction{
	"accept":      tcpAccept,
	"bind":        tcpBind,
	"close":       sockClose,
	"connect":     tcpConnect,
	"dirty":       sockDirty,
	"getfamily":   sockGetfamily,
	"getfd":       sockGetfd,
	"getoption":   sockGetoption,
	"getpeername": tcpGetpeername,
	"getsockname": sockGetsockname,
	"getstats":    tcpGetstats,
	"setstats":    tcpSetstats,
	"listen":      tcpListen,
	"receive":     tcpReceive,
	"send":        tcpSend,
	"setfd":       sockSetfd,
	"setoption":   sockSetoption,
	"setpeername": tcpConnect,
	"setsockname": tcpBind,
	"settimeout":  sockSettimeout,
	"gettimeout":  sockGettimeout,
	"shutdown":    tcpShutdown,
}

var udpMethods = map[string]lua.LGFunction{
	"close":       sockClose,
	"dirty":       sockDirty,
	"getfamily":   sockGetfamily,
	"getfd":       sockGetfd,
	"getpeername": udpGetpeername,
	"getsockname": sockGetsockname,
	"receive":     udpReceive,
	"receivefrom": udpReceivefrom,
	"send":        udpSend,
	"sendto":      udpSendto,
	"setfd":       sockSetfd,
	"setoption":   sockSetoption,
	"getoption":   sockGetoption,
	"setpeername": udpSetpeername,
	"setsockname": udpSetsockname,
	"settimeout":  sockSettimeout,
	"gettimeout":  sockGettimeout,
}

// anyGroup is the group a method that works in every state checks for.
func anyGroup(L *lua.LState) (*lua.LUserData, *lsock) {
	if ud, ok := L.Get(1).(*lua.LUserData); ok {
		if s, ok := ud.Value.(*lsock); ok && s.udp {
			return checkSock(L, groupUDP)
		}
	}
	return checkSock(L, groupTCP)
}

// The inet.c helpers.

func tryCreate(fd *int, family, typ int) string {
	nfd, e := sockCreate(family, typ)
	*fd = nfd
	if e == ioDone && family == unix.AF_INET6 {
		unix.SetsockoptInt(nfd, unix.IPPROTO_IPV6, unix.IPV6_V6ONLY, 1)
	}
	return sockStrerror(e)
}

func tryConnect(L *lua.LState, s *lsock, address, serv string, socktype int) string {
	addrs, err := resolve(&address, serv, s.family, socktype, false, false)
	if err != "" {
		return err
	}
	current := s.family
	for _, ai := range addrs {
		s.tm.markstart()
		if current != ai.family || s.fd == sockInvalid {
			sockDestroy(&s.fd)
			if err = tryCreate(&s.fd, ai.family, socktype); err != "" {
				continue
			}
			current = ai.family
			unix.SetNonblock(s.fd, true)
		}
		err = sockStrerror(sockConnect(L, s.fd, ai.sa, &s.tm))
		if err == "" || s.tm.iszero() {
			s.family = current
			break
		}
	}
	return err
}

func tryBind(s *lsock, address, serv string, socktype int) string {
	host := &address
	if address == "*" {
		host = nil
	}
	addrs, err := resolve(host, serv, s.family, socktype, true, false)
	if err != "" {
		return err
	}
	current := s.family
	for _, ai := range addrs {
		if current != ai.family || s.fd == sockInvalid {
			sockDestroy(&s.fd)
			if err = tryCreate(&s.fd, ai.family, socktype); err != "" {
				continue
			}
			current = ai.family
		}
		err = ""
		if e := unix.Bind(s.fd, ai.sa); e != nil {
			err = sockStrerror(errnoOf(e))
		}
		if err == "" {
			s.family = current
			unix.SetNonblock(s.fd, true)
			break
		}
	}
	return err
}

func familyName(family int) string {
	switch family {
	case unix.AF_INET:
		return "inet"
	case unix.AF_INET6:
		return "inet6"
	case unix.AF_UNSPEC:
		return "unspec"
	}
	return "unknown"
}

// pushName is getpeername and getsockname: the address, the port and the
// family. The port is a number from one and a string from the other, as
// LuaSocket has it.
func pushName(L *lua.LState, s *lsock, peer bool) int {
	var sa unix.Sockaddr
	var err error
	if peer {
		sa, err = unix.Getpeername(s.fd)
	} else {
		sa, err = unix.Getsockname(s.fd)
	}
	if err != nil {
		return pushErr(L, sockStrerror(errnoOf(err)))
	}
	host, port, ok := numericHost(sa)
	if !ok {
		return pushErr(L, "ai_family not supported")
	}
	L.Push(lua.LString(host))
	if peer {
		L.Push(lua.LNumber(port))
	} else {
		L.Push(lua.LString(fmt.Sprint(port)))
	}
	L.Push(lua.LString(familyName(s.family)))
	return 3
}

// Methods common to TCP and UDP.

func sockClose(L *lua.LState) int {
	_, s := anyGroup(L)
	sockDestroy(&s.fd)
	return pushOne(L)
}

func sockDirty(L *lua.LState) int {
	_, s := anyGroup(L)
	L.Push(lua.LBool(!s.udp && !s.buf.isempty()))
	return 1
}

func sockGetfamily(L *lua.LState) int {
	_, s := anyGroup(L)
	if s.family == unix.AF_INET6 {
		L.Push(lua.LString("inet6"))
	} else {
		L.Push(lua.LString("inet4"))
	}
	return 1
}

func sockGetfd(L *lua.LState) int {
	_, s := anyGroup(L)
	L.Push(lua.LNumber(s.fd))
	return 1
}

func sockSetfd(L *lua.LState) int {
	_, s := anyGroup(L)
	s.fd = int(L.CheckNumber(2))
	return 0
}

func sockGetsockname(L *lua.LState) int {
	_, s := anyGroup(L)
	return pushName(L, s, false)
}

func sockSettimeout(L *lua.LState) int {
	_, s := anyGroup(L)
	t := float64(L.OptNumber(2, -1))
	switch mode := optStringDef(L, 3, "b"); {
	case mode != "" && mode[0] == 'b':
		s.tm.block = t
	case mode != "" && (mode[0] == 'r' || mode[0] == 't'):
		s.tm.total = t
	default:
		L.ArgError(3, "invalid timeout mode")
	}
	return pushOne(L)
}

func sockGettimeout(L *lua.LState) int {
	_, s := anyGroup(L)
	L.Push(lua.LNumber(s.tm.block))
	L.Push(lua.LNumber(s.tm.total))
	return 2
}

// TCP.

func newTCP(family int) func(L *lua.LState) int {
	return func(L *lua.LState) int {
		ud, s := newSock(L, classTCPMaster, false, sockInvalid, family)
		if family != unix.AF_UNSPEC {
			if err := tryCreate(&s.fd, family, unix.SOCK_STREAM); err != "" {
				return pushErr(L, err)
			}
			unix.SetNonblock(s.fd, true)
		}
		L.Push(ud)
		return 1
	}
}

// tcpConnectGlobal is socket.connect(address, port [, locaddr, locport,
// family]): a client in one call, bound first if a local address is given.
func tcpConnectGlobal(L *lua.LState) int {
	remoteaddr := L.CheckString(1)
	remoteserv := L.CheckString(2)
	localaddr, haveLocal := optString(L, 3)
	localserv := optStringDef(L, 4, "0")
	family := optFamily(L, 5)

	ud, s := newSock(L, classTCPMaster, false, sockInvalid, unix.AF_UNSPEC)
	if haveLocal {
		s.family = family
		if err := tryBind(s, localaddr, localserv, unix.SOCK_STREAM); err != "" {
			return pushErr(L, err)
		}
	}
	if err := tryConnect(L, s, remoteaddr, remoteserv, unix.SOCK_STREAM); err != "" {
		sockDestroy(&s.fd)
		return pushErr(L, err)
	}
	setClass(L, ud, s, classTCPClient)
	L.Push(ud)
	return 1
}

func optFamily(L *lua.LState, n int) int {
	switch optStringDef(L, n, "unspec") {
	case "unspec":
		return unix.AF_UNSPEC
	case "inet":
		return unix.AF_INET
	case "inet6":
		return unix.AF_INET6
	}
	L.ArgError(n, "invalid option '"+L.CheckString(n)+"'")
	return 0
}

func tcpAccept(L *lua.LState) int {
	_, server := checkSock(L, classTCPServer)
	server.tm.markstart()
	fd, e := sockAccept(L, server.fd, &server.tm)
	if e != ioDone {
		return pushErr(L, sockStrerror(e))
	}
	unix.SetNonblock(fd, true)
	ud, _ := newSock(L, classTCPClient, false, fd, server.family)
	L.Push(ud)
	return 1
}

func tcpBind(L *lua.LState) int {
	_, s := checkSock(L, classTCPMaster)
	address := L.CheckString(2)
	port := L.CheckString(3)
	if err := tryBind(s, address, port, unix.SOCK_STREAM); err != "" {
		return pushErr(L, err)
	}
	return pushOne(L)
}

func tcpConnect(L *lua.LState) int {
	ud, s := checkSock(L, groupTCP)
	address := L.CheckString(2)
	port := L.CheckString(3)
	s.tm.markstart()
	err := tryConnect(L, s, address, port, unix.SOCK_STREAM)
	// The class changes even on failure, for a non-blocking connect that is
	// still under way.
	setClass(L, ud, s, classTCPClient)
	if err != "" {
		return pushErr(L, err)
	}
	return pushOne(L)
}

func tcpListen(L *lua.LState) int {
	ud, s := checkSock(L, classTCPMaster)
	backlog := int(L.OptNumber(2, 32))
	if err := unix.Listen(s.fd, backlog); err != nil {
		return pushErr(L, sockStrerror(errnoOf(err)))
	}
	setClass(L, ud, s, classTCPServer)
	return pushOne(L)
}

func tcpShutdown(L *lua.LState) int {
	_, s := checkSock(L, classTCPClient)
	how := unix.SHUT_RDWR
	switch optStringDef(L, 2, "both") {
	case "receive":
		how = unix.SHUT_RD
	case "send":
		how = unix.SHUT_WR
	case "both":
	default:
		L.ArgError(2, "invalid option '"+L.CheckString(2)+"'")
	}
	unix.Shutdown(s.fd, how)
	return pushOne(L)
}

func tcpGetpeername(L *lua.LState) int {
	_, s := checkSock(L, groupTCP)
	return pushName(L, s, true)
}

func tcpGetstats(L *lua.LState) int {
	_, s := checkSock(L, classTCPClient)
	return bufGetstats(L, &s.buf)
}

func tcpSetstats(L *lua.LState) int {
	_, s := checkSock(L, classTCPClient)
	return bufSetstats(L, &s.buf)
}

// tcpSend is send(data [, i [, j]]): the bytes i..j of data, and how far it
// got, also when it fails part way. Like receive, it also gives back how
// long it took, as LuaSocket does when built by LuaRocks, which defines
// LUASOCKET_DEBUG.
func tcpSend(L *lua.LState) int {
	_, s := checkSock(L, classTCPClient)
	return bufSend(L, &s.buf, s, &s.tm, sockStrerror)
}

// tcpReceive is receive([pattern [, prefix]]): a line ("*l", the default),
// everything up to the close ("*a"), or a number of bytes. On failure it
// gives back nil, the error and what had arrived.
func tcpReceive(L *lua.LState) int {
	_, s := checkSock(L, classTCPClient)
	return bufReceive(L, &s.buf, s, &s.tm, sockStrerror)
}

// UDP.

// udpStrerror: a closed error on an unconnected socket means the address
// was refused.
func udpStrerror(e int) string {
	if e == ioClosed {
		return "refused"
	}
	return sockStrerror(e)
}

func newUDP(family int) func(L *lua.LState) int {
	return func(L *lua.LState) int {
		ud, s := newSock(L, classUDPUnconnected, true, sockInvalid, family)
		if family != unix.AF_UNSPEC {
			if err := tryCreate(&s.fd, family, unix.SOCK_DGRAM); err != "" {
				return pushErr(L, err)
			}
			unix.SetNonblock(s.fd, true)
		}
		L.Push(ud)
		return 1
	}
}

func udpSend(L *lua.LState) int {
	_, s := checkSock(L, classUDPConnected)
	data := L.CheckString(2)
	s.tm.markstart()
	n, e := sockSend(L, s.fd, []byte(data), nil, &s.tm)
	if e != ioDone {
		return pushErr(L, udpStrerror(e))
	}
	L.Push(lua.LNumber(n))
	return 1
}

func udpSendto(L *lua.LState) int {
	_, s := checkSock(L, classUDPUnconnected)
	data := L.CheckString(2)
	ip := L.CheckString(3)
	port := L.CheckString(4)
	addrs, err := resolve(&ip, port, s.family, unix.SOCK_DGRAM, false, true)
	if err != "" {
		// sendto reports getaddrinfo's own words, not LuaSocket's.
		return pushErr(L, "Name or service not known")
	}
	if s.family == unix.AF_UNSPEC && s.fd == sockInvalid {
		for _, ai := range addrs {
			if err = tryCreate(&s.fd, ai.family, unix.SOCK_DGRAM); err == "" {
				unix.SetNonblock(s.fd, true)
				s.family = ai.family
				break
			}
		}
		if err != "" {
			return pushErr(L, err)
		}
	}
	s.tm.markstart()
	n, e := sockSend(L, s.fd, []byte(data), addrs[0].sa, &s.tm)
	if e != ioDone {
		return pushErr(L, udpStrerror(e))
	}
	L.Push(lua.LNumber(n))
	return 1
}

func udpWanted(L *lua.LState) int {
	wanted := int(L.OptNumber(2, udpDatagramSize))
	if wanted < 0 {
		wanted = 0
	}
	return wanted
}

func udpReceive(L *lua.LState) int {
	_, s := checkSock(L, groupUDP)
	buf := make([]byte, udpWanted(L))
	s.tm.markstart()
	n, e := sockRecv(L, s.fd, buf, nil, &s.tm)
	// Unlike TCP, reading nothing is an empty datagram, not a close.
	if e != ioDone && e != ioClosed {
		return pushErr(L, udpStrerror(e))
	}
	L.Push(lua.LString(buf[:n]))
	return 1
}

func udpReceivefrom(L *lua.LState) int {
	_, s := checkSock(L, classUDPUnconnected)
	buf := make([]byte, udpWanted(L))
	s.tm.markstart()
	var from unix.Sockaddr
	n, e := sockRecv(L, s.fd, buf, &from, &s.tm)
	if e != ioDone && e != ioClosed {
		return pushErr(L, udpStrerror(e))
	}
	host, port, ok := numericHost(from)
	if !ok {
		return pushErr(L, "ai_family not supported")
	}
	L.Push(lua.LString(buf[:n]))
	L.Push(lua.LString(host))
	L.Push(lua.LNumber(port))
	return 3
}

func udpGetpeername(L *lua.LState) int {
	_, s := checkSock(L, classUDPConnected)
	return pushName(L, s, true)
}

// udpSetpeername connects the socket to one peer, or with "*" undoes that.
func udpSetpeername(L *lua.LState) int {
	ud, s := checkSock(L, groupUDP)
	address := L.CheckString(2)
	if address != "*" {
		port := L.CheckString(3)
		if err := tryConnect(L, s, address, port, unix.SOCK_DGRAM); err != "" {
			return pushErr(L, err)
		}
		setClass(L, ud, s, classUDPConnected)
	} else {
		// Errors are ignored, as LuaSocket does: some systems refuse.
		disconnect(s.fd, s.family)
		setClass(L, ud, s, classUDPUnconnected)
	}
	return pushOne(L)
}

func udpSetsockname(L *lua.LState) int {
	_, s := checkSock(L, classUDPUnconnected)
	address := L.CheckString(2)
	port := L.CheckString(3)
	if err := tryBind(s, address, port, unix.SOCK_DGRAM); err != "" {
		return pushErr(L, err)
	}
	return pushOne(L)
}
