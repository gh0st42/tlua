package lualib

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// The TCP options only Linux has.
func tcpExtraOpts() map[string]optPair {
	return map[string]optPair{
		"tcp-keepidle":         intOpt(unix.IPPROTO_TCP, unix.TCP_KEEPIDLE),
		"tcp-defer-accept":     {nil, intOpt(unix.IPPROTO_TCP, unix.TCP_DEFER_ACCEPT).set},
		"tcp-fastopen-connect": {nil, intOpt(unix.IPPROTO_TCP, unix.TCP_FASTOPEN_CONNECT).set},
	}
}

// disconnect dissolves a UDP socket's association with its peer: connect(2)
// to an address of family AF_UNSPEC, which x/sys/unix has no Sockaddr for.
func disconnect(fd, family int) {
	var sa unix.RawSockaddrInet6
	size := unsafe.Sizeof(sa)
	if family == unix.AF_INET {
		size = unsafe.Sizeof(unix.RawSockaddrInet4{})
	}
	sa.Family = unix.AF_UNSPEC
	unix.Syscall(unix.SYS_CONNECT, uintptr(fd), uintptr(unsafe.Pointer(&sa)), size)
}
