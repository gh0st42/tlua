//go:build linux || darwin

package lualib

import (
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	lua "github.com/yuin/gopher-lua"
)

// The layer of socket.core underneath the Lua objects: timeouts, the system
// calls on a non-blocking descriptor, and the receive buffer. Each part is
// LuaSocket 3.1's timeout.c, usocket.c and buffer.c in turn, so that a
// program sees the same results, the same error strings and the same partial
// data on a timeout as it would with the C library.

// What an I/O operation came to. Positive values are errnos.
const (
	ioDone    = 0
	ioTimeout = -1
	ioClosed  = -2
	ioUnknown = -3
)

const (
	sockInvalid     = -1
	sockBufSize     = 8192 // buffer.h BUF_SIZE
	udpDatagramSize = 8192 // udp.h UDP_DATAGRAMSIZE
	sendStepSize    = 8192 // buffer.c STEPSIZE
)

// Which way a descriptor is waited on; poll(2)'s own bits, as usocket.c.
const (
	waitR = unix.POLLIN
	waitW = unix.POLLOUT
	waitC = unix.POLLIN | unix.POLLOUT
)

// gettime is seconds since the epoch, with the fraction.
func gettime() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}

// timeout is timeout.c: a limit per blocking call and a limit on the whole
// operation, either of which can be off (negative).
type timeout struct {
	block, total, start float64
}

func newTimeout() timeout { return timeout{block: -1, total: -1} }

func (tm *timeout) markstart() { tm.start = gettime() }

func (tm *timeout) iszero() bool { return tm.block == 0 }

// getretry is how long the next wait may last, -1 for no limit.
func (tm *timeout) getretry() float64 {
	switch {
	case tm.block < 0 && tm.total < 0:
		return -1
	case tm.block < 0:
		return max(tm.total-gettime()+tm.start, 0)
	case tm.total < 0:
		return max(tm.block-gettime()+tm.start, 0)
	default:
		return min(tm.block, max(tm.total-gettime()+tm.start, 0))
	}
}

// pollSlice bounds a single poll(2), so that a program blocked on a socket
// still stops when it is interrupted.
const pollSlice = 200

// checkInterrupt raises the error the interpreter raises for an interrupted
// chunk, if this one has been.
func checkInterrupt(L *lua.LState) {
	if ctx := L.Context(); ctx != nil {
		select {
		case <-ctx.Done():
			L.RaiseError("%s", ctx.Err().Error())
		default:
		}
	}
}

func errnoOf(err error) int {
	if errno, ok := err.(unix.Errno); ok {
		return int(errno)
	}
	return ioUnknown
}

// waitfd waits until fd is ready for sw, or the timeout runs out.
func waitfd(L *lua.LState, fd int, sw int16, tm *timeout) int {
	if tm.iszero() {
		return ioTimeout
	}
	for {
		ms := -1
		if t := tm.getretry(); t >= 0 {
			ms = int(t * 1e3)
		}
		wait := ms
		if wait < 0 || wait > pollSlice {
			wait = pollSlice
		}
		pfd := []unix.PollFd{{Fd: int32(fd), Events: sw}}
		n, err := unix.Poll(pfd, wait)
		if err == unix.EINTR {
			checkInterrupt(L)
			continue
		}
		if err != nil {
			return errnoOf(err)
		}
		if n == 0 {
			checkInterrupt(L)
			if ms >= 0 && ms <= pollSlice {
				return ioTimeout
			}
			continue
		}
		if sw == waitC && pfd[0].Revents&(unix.POLLIN|unix.POLLERR) != 0 {
			return ioClosed
		}
		return ioDone
	}
}

// sockCreate opens a non-blocking socket that is not inherited by commands
// the program runs.
func sockCreate(family, typ int) (int, int) {
	fd, err := unix.Socket(family, typ, 0)
	if err != nil {
		return sockInvalid, errnoOf(err)
	}
	unix.CloseOnExec(fd)
	return fd, ioDone
}

func sockDestroy(fd *int) {
	if *fd != sockInvalid {
		unix.Close(*fd)
		*fd = sockInvalid
	}
}

func sockConnect(L *lua.LState, fd int, sa unix.Sockaddr, tm *timeout) int {
	if fd == sockInvalid {
		return ioClosed
	}
	var err error
	for {
		if err = unix.Connect(fd, sa); err != unix.EINTR {
			break
		}
	}
	if err == nil {
		return ioDone
	}
	if err != unix.EINPROGRESS && err != unix.EAGAIN {
		return errnoOf(err)
	}
	if tm.iszero() {
		return ioTimeout
	}
	e := waitfd(L, fd, waitC, tm)
	if e == ioClosed {
		// Readable or failed: a zero-length read says which.
		if _, _, err := unix.Recvfrom(fd, nil, 0); err != nil {
			return errnoOf(err)
		}
		return ioDone
	}
	return e
}

func sockAccept(L *lua.LState, fd int, tm *timeout) (int, int) {
	if fd == sockInvalid {
		return sockInvalid, ioClosed
	}
	for {
		nfd, _, err := unix.Accept(fd)
		if err == nil {
			unix.CloseOnExec(nfd)
			return nfd, ioDone
		}
		if err == unix.EINTR {
			continue
		}
		if err != unix.EAGAIN && err != unix.ECONNABORTED {
			return sockInvalid, errnoOf(err)
		}
		if e := waitfd(L, fd, waitR, tm); e != ioDone {
			return sockInvalid, e
		}
	}
}

// sockSend writes what it can of data, waiting while the socket is full.
// SIGPIPE needs no handling: the Go runtime ignores it on sockets, and the
// write fails with EPIPE, which is a closed connection.
func sockSend(L *lua.LState, fd int, data []byte, to unix.Sockaddr, tm *timeout) (int, int) {
	if fd == sockInvalid {
		return 0, ioClosed
	}
	for {
		var n int
		var err error
		if to == nil {
			n, err = unix.Write(fd, data)
		} else {
			n, err = unix.SendmsgN(fd, data, nil, to, 0)
		}
		if err == nil {
			return n, ioDone
		}
		switch err {
		case unix.EPIPE:
			return 0, ioClosed
		case unix.EPROTOTYPE, unix.EINTR:
			continue
		case unix.EAGAIN:
		default:
			return 0, errnoOf(err)
		}
		if e := waitfd(L, fd, waitW, tm); e != ioDone {
			return 0, e
		}
	}
}

// sockRecv reads what is there, waiting while there is nothing. A read of
// nothing at all is a closed connection (or, to UDP, an empty datagram).
func sockRecv(L *lua.LState, fd int, buf []byte, from *unix.Sockaddr, tm *timeout) (int, int) {
	if fd == sockInvalid {
		return 0, ioClosed
	}
	for {
		var n int
		var err error
		if from == nil {
			n, err = unix.Read(fd, buf)
		} else {
			n, *from, err = unix.Recvfrom(fd, buf, 0)
		}
		if err == nil {
			if n > 0 {
				return n, ioDone
			}
			return 0, ioClosed
		}
		switch err {
		case unix.EINTR:
			continue
		case unix.EAGAIN:
		default:
			return 0, errnoOf(err)
		}
		if e := waitfd(L, fd, waitR, tm); e != ioDone {
			return 0, e
		}
	}
}

// sockStrerror is socket_strerror: LuaSocket's own short words for the
// errors a program is expected to handle, strerror for the rest. "" stands
// for no error, which C returns as NULL.
func sockStrerror(e int) string {
	switch {
	case e == ioDone:
		return ""
	case e == ioTimeout:
		return "timeout"
	case e == ioClosed:
		return "closed"
	case e < 0:
		return "unknown error"
	}
	switch unix.Errno(e) {
	case unix.EADDRINUSE:
		return "address already in use"
	case unix.EISCONN:
		return "already connected"
	case unix.EACCES:
		return "permission denied"
	case unix.ECONNREFUSED:
		return "connection refused"
	case unix.ECONNABORTED, unix.ECONNRESET:
		return "closed"
	case unix.ETIMEDOUT:
		return "timeout"
	}
	msg, _ := strerror(unix.Errno(e))
	return msg
}

// pushErr returns nil and the message, the usual failure.
func pushErr(L *lua.LState, msg string) int {
	L.Push(lua.LNil)
	L.Push(lua.LString(msg))
	return 2
}

func pushOne(L *lua.LState) int {
	L.Push(lua.LNumber(1))
	return 1
}

// toNumber is lua_isnumber and lua_tonumber together: a number, or a string
// that reads as one.
func toNumber(lv lua.LValue) (float64, bool) {
	switch v := lv.(type) {
	case lua.LNumber:
		return float64(v), true
	case lua.LString:
		s := strings.TrimSpace(string(v))
		if h, ok := strings.CutPrefix(strings.ToLower(s), "0x"); ok {
			n, err := strconv.ParseUint(h, 16, 64)
			return float64(n), err == nil
		}
		n, err := strconv.ParseFloat(s, 64)
		return n, err == nil
	}
	return 0, false
}

// sockBuffer is buffer.c: what has been read from a stream socket and not
// yet handed to the program, and the counts getstats reports.
type sockBuffer struct {
	data           [sockBufSize]byte
	first, last    int
	received, sent float64
	birthday       float64
}

func (b *sockBuffer) isempty() bool { return b.first >= b.last }

func (b *sockBuffer) skip(n int) {
	b.received += float64(n)
	b.first += n
	if b.isempty() {
		b.first, b.last = 0, 0
	}
}

// transport is what a buffer reads from and writes to: a socket's
// descriptor, or a TLS connection over one (ssl.core). Each returns the
// bytes moved and what the operation came to, as sockRecv and sockSend do.
type transport interface {
	recvInto(L *lua.LState, p []byte, tm *timeout) (int, int)
	sendFrom(L *lua.LState, p []byte, tm *timeout) (int, int)
}

func (s *lsock) recvInto(L *lua.LState, p []byte, tm *timeout) (int, int) {
	return sockRecv(L, s.fd, p, nil, tm)
}

func (s *lsock) sendFrom(L *lua.LState, p []byte, tm *timeout) (int, int) {
	return sockSend(L, s.fd, p, nil, tm)
}

// get hands back what is buffered, reading more first if nothing is.
func (b *sockBuffer) get(L *lua.LState, t transport, tm *timeout) ([]byte, int) {
	err := ioDone
	if b.isempty() {
		var n int
		n, err = t.recvInto(L, b.data[:], tm)
		b.first, b.last = 0, n
	}
	return b.data[b.first:b.last], err
}

func (b *sockBuffer) recvRaw(L *lua.LState, t transport, tm *timeout, wanted int, out []byte) ([]byte, int) {
	err, total := ioDone, 0
	for err == ioDone {
		var data []byte
		data, err = b.get(L, t, tm)
		n := min(len(data), wanted-total)
		out = append(out, data[:n]...)
		b.skip(n)
		total += n
		if total >= wanted {
			break
		}
	}
	return out, err
}

func (b *sockBuffer) recvAll(L *lua.LState, t transport, tm *timeout, out []byte) ([]byte, int) {
	err, total := ioDone, 0
	for err == ioDone {
		var data []byte
		data, err = b.get(L, t, tm)
		total += len(data)
		out = append(out, data...)
		b.skip(len(data))
	}
	if err == ioClosed && total > 0 {
		return out, ioDone
	}
	return out, err
}

// recvLine reads up to a LF, dropping every CR on the way.
func (b *sockBuffer) recvLine(L *lua.LState, t transport, tm *timeout, out []byte) ([]byte, int) {
	err := ioDone
	for err == ioDone {
		var data []byte
		data, err = b.get(L, t, tm)
		pos := 0
		for pos < len(data) && data[pos] != '\n' {
			if data[pos] != '\r' {
				out = append(out, data[pos])
			}
			pos++
		}
		if pos < len(data) {
			b.skip(pos + 1)
			break
		}
		b.skip(pos)
	}
	return out, err
}

func (b *sockBuffer) sendRaw(L *lua.LState, t transport, tm *timeout, data []byte) (int, int) {
	err, total := ioDone, 0
	for total < len(data) && err == ioDone {
		step := min(len(data)-total, sendStepSize)
		var n int
		n, err = t.sendFrom(L, data[total:total+step], tm)
		total += n
	}
	b.sent += float64(total)
	return total, err
}

// bufReceive is buffer_meth_receive: obj:receive([pattern [, prefix]]) on
// whatever the buffer reads from, with strerror for its failures.
func bufReceive(L *lua.LState, b *sockBuffer, t transport, tm *timeout, strerror func(int) string) int {
	prefix, _ := optString(L, 3)
	tm.markstart()
	out := []byte(prefix)
	e := ioDone
	if n, ok := toNumber(L.Get(2)); ok {
		if n < 0 {
			L.ArgError(2, "invalid receive pattern")
		}
		wanted := int(n)
		if len(prefix) == 0 || wanted > len(prefix) {
			out, e = b.recvRaw(L, t, tm, wanted-len(prefix), out)
		}
	} else {
		p := optStringDef(L, 2, "*l")
		switch {
		case len(p) >= 2 && p[0] == '*' && p[1] == 'l':
			out, e = b.recvLine(L, t, tm, out)
		case len(p) >= 2 && p[0] == '*' && p[1] == 'a':
			out, e = b.recvAll(L, t, tm, out)
		default:
			L.ArgError(2, "invalid receive pattern")
		}
	}
	if e != ioDone {
		L.Push(lua.LNil)
		L.Push(lua.LString(strerror(e)))
		L.Push(lua.LString(out))
	} else {
		L.Push(lua.LString(out))
		L.Push(lua.LNil)
		L.Push(lua.LNil)
	}
	L.Push(lua.LNumber(gettime() - tm.start))
	return 4
}

// bufSend is buffer_meth_send: obj:send(data [, i [, j]]).
func bufSend(L *lua.LState, b *sockBuffer, t transport, tm *timeout, strerror func(int) string) int {
	data := L.CheckString(2)
	start := int64(L.OptNumber(3, 1))
	end := int64(L.OptNumber(4, -1))
	tm.markstart()
	size := int64(len(data))
	if start < 0 {
		start = size + start + 1
	}
	if end < 0 {
		end = size + end + 1
	}
	if start < 1 {
		start = 1
	}
	if end > size {
		end = size
	}
	sent, e := 0, ioDone
	if start <= end {
		sent, e = b.sendRaw(L, t, tm, []byte(data[start-1:end]))
	}
	last := lua.LNumber(int64(sent) + start - 1)
	if e != ioDone {
		L.Push(lua.LNil)
		L.Push(lua.LString(strerror(e)))
		L.Push(last)
	} else {
		L.Push(last)
		L.Push(lua.LNil)
		L.Push(lua.LNil)
	}
	L.Push(lua.LNumber(gettime() - tm.start))
	return 4
}

// bufGetstats and bufSetstats are getstats and setstats.
func bufGetstats(L *lua.LState, b *sockBuffer) int {
	L.Push(lua.LNumber(b.received))
	L.Push(lua.LNumber(b.sent))
	L.Push(lua.LNumber(gettime() - b.birthday))
	return 3
}

func bufSetstats(L *lua.LState, b *sockBuffer) int {
	b.received = float64(L.OptNumber(2, lua.LNumber(b.received)))
	b.sent = float64(L.OptNumber(3, lua.LNumber(b.sent)))
	if age, ok := toNumber(L.Get(4)); ok {
		b.birthday = gettime() - age
	}
	return pushOne(L)
}
