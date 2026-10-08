//go:build linux || darwin

package lualib

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// Name resolution for socket.core. LuaSocket asks getaddrinfo(3), which
// would need cgo; Go's own resolver reads the same /etc/hosts and
// resolv.conf and works in a static binary, so it answers instead, and the
// failures are reported in getaddrinfo's words.

// What LuaSocket says for each getaddrinfo failure (pierror.h).
const (
	gaiNoName  = "host or service not provided, or not known"
	gaiAgain   = "temporary failure in name resolution"
	gaiFail    = "non-recoverable failure in name resolution"
	gaiFamily  = "ai_family not supported"
	gaiService = "service not supported for socket type"
)

// addrinfo is one answer: where to connect or bind, and as which family.
type addrinfo struct {
	family int
	sa     unix.Sockaddr
}

// lookupPort is the service half of getaddrinfo: a number, or a name from
// /etc/services. A number is read the way glibc reads it, with strtoul and
// cut to 16 bits; one that comes out negative as an int is looked up as a
// name, and fails.
func lookupPort(serv string, socktype int) (int, string) {
	if n, ok := strtoul(serv); ok && int32(uint32(n)) >= 0 {
		return int(uint16(n)), ""
	}
	network := "tcp"
	if socktype == unix.SOCK_DGRAM {
		network = "udp"
	}
	if n, err := net.LookupPort(network, serv); err == nil {
		return n, ""
	}
	return 0, gaiService
}

// strtoul reads a whole string as C's strtoul(s, &end, 10) with *end == 0:
// leading space and a sign allowed, a negative number wrapping around.
func strtoul(s string) (uint64, bool) {
	t := strings.TrimLeft(s, " \t\n\v\f\r")
	neg := false
	if t != "" && (t[0] == '+' || t[0] == '-') {
		neg = t[0] == '-'
		t = t[1:]
	}
	if t == "" {
		// No digits: C's end pointer stays at the start, so only an empty
		// string reads as a number (zero).
		return 0, s == ""
	}
	var n uint64
	for i := 0; i < len(t); i++ {
		if t[i] < '0' || t[i] > '9' {
			return 0, false
		}
		n = n*10 + uint64(t[i]-'0')
	}
	if neg {
		n = -n
	}
	return n, true
}

func sockaddrOf(ip netip.Addr, port int) (int, unix.Sockaddr) {
	if ip.Is4() {
		return unix.AF_INET, &unix.SockaddrInet4{Port: port, Addr: ip.As4()}
	}
	sa := &unix.SockaddrInet6{Port: port, Addr: ip.As16()}
	if zone := ip.Zone(); zone != "" {
		if ifi, err := net.InterfaceByName(zone); err == nil {
			sa.ZoneId = uint32(ifi.Index)
		} else if n, err := strconv.Atoi(zone); err == nil {
			sa.ZoneId = uint32(n)
		}
	}
	return unix.AF_INET6, sa
}

// resolve is getaddrinfo(host, serv) with the hints LuaSocket passes: a
// family (AF_UNSPEC for any), a socket type, passive for bind (where no
// host means any address) and numeric for a host that must be an address
// already. The message is "" on success.
func resolve(host *string, serv string, family, socktype int, passive, numeric bool) ([]addrinfo, string) {
	port, perr := lookupPort(serv, socktype)
	if perr != "" {
		return nil, perr
	}

	var ips []netip.Addr
	switch {
	case host == nil && passive:
		ips = []netip.Addr{netip.IPv4Unspecified(), netip.IPv6Unspecified()}
	case host == nil:
		ips = []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.IPv6Loopback()}
	default:
		if ip, err := netip.ParseAddr(*host); err == nil {
			ips = []netip.Addr{ip}
		} else if numeric {
			return nil, gaiNoName
		} else {
			network := "ip"
			switch family {
			case unix.AF_INET:
				network = "ip4"
			case unix.AF_INET6:
				network = "ip6"
			}
			found, err := net.DefaultResolver.LookupNetIP(context.Background(), network, *host)
			if err != nil {
				return nil, dnsError(err)
			}
			ips = found
		}
	}

	var out []addrinfo
	for _, ip := range ips {
		if ip.Is4In6() && family != unix.AF_INET6 {
			ip = ip.Unmap()
		}
		fam, sa := sockaddrOf(ip, port)
		if family != unix.AF_UNSPEC && fam != family {
			continue
		}
		out = append(out, addrinfo{fam, sa})
	}
	if len(out) == 0 {
		if host != nil && !numeric {
			return nil, gaiNoName
		}
		return nil, gaiFamily
	}
	return out, ""
}

func dnsError(err error) string {
	var dns *net.DNSError
	if errors.As(err, &dns) {
		switch {
		case dns.IsNotFound:
			return gaiNoName
		case dns.IsTemporary, dns.IsTimeout:
			return gaiAgain
		}
		return gaiFail
	}
	return gaiNoName
}

// numericHost is getnameinfo(NI_NUMERICHOST|NI_NUMERICSERV): an address as
// text, and its port.
func numericHost(sa unix.Sockaddr) (string, int, bool) {
	switch a := sa.(type) {
	case *unix.SockaddrInet4:
		return netip.AddrFrom4(a.Addr).String(), a.Port, true
	case *unix.SockaddrInet6:
		ip := netip.AddrFrom16(a.Addr)
		if a.ZoneId != 0 {
			zone := strconv.Itoa(int(a.ZoneId))
			if ifi, err := net.InterfaceByIndex(int(a.ZoneId)); err == nil {
				zone = ifi.Name
			}
			ip = ip.WithZone(zone)
		}
		return ip.String(), a.Port, true
	}
	return "", 0, false
}

// hostName is what gethostbyname and gethostbyaddr would have found: the
// canonical name, and the IPv4 addresses under it.
func hostName(address string) (string, []string, string) {
	if ip, err := netip.ParseAddr(address); err == nil && ip.Is4() {
		names, err := net.LookupAddr(address)
		if err != nil || len(names) == 0 {
			return "", nil, hostError(err)
		}
		return strings.TrimSuffix(names[0], "."), []string{address}, ""
	}
	ips, err := net.DefaultResolver.LookupNetIP(context.Background(), "ip4", address)
	if err != nil {
		return "", nil, hostError(err)
	}
	name := address
	if cname, err := net.LookupCNAME(address); err == nil && cname != "" {
		name = strings.TrimSuffix(cname, ".")
	}
	addrs := make([]string, 0, len(ips))
	for _, ip := range ips {
		addrs = append(addrs, ip.Unmap().String())
	}
	return name, addrs, ""
}

// hostError is socket_hoststrerror over what gethostbyname's h_errno would
// have been.
func hostError(err error) string {
	var dns *net.DNSError
	if errors.As(err, &dns) && (dns.IsTemporary || dns.IsTimeout) {
		return "Host name lookup failure"
	}
	return "host not found"
}

// serviceName is the reverse of lookupPort, for getnameinfo: the name
// /etc/services gives a port, or the number when it gives none.
func serviceName(port int, socktype int) string {
	proto := "tcp"
	if socktype == unix.SOCK_DGRAM {
		proto = "udp"
	}
	want := strconv.Itoa(port) + "/" + proto
	if f, err := os.Open("/etc/services"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line, _, _ := strings.Cut(sc.Text(), "#")
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[1] == want {
				return fields[0]
			}
		}
	}
	return strconv.Itoa(port)
}
