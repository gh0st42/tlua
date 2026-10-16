---@meta socket
--- LuaSocket 3.1, `require "socket"`: TCP, UDP, DNS and select. Its C
--- half, socket.core, is built into tlua in Go, on Linux and macOS. Calls
--- that fail give nil and a message ("timeout", "closed",
--- "connection refused", ...).

local socket = {}

socket._VERSION = "LuaSocket 3.0.0"
socket._DEBUG = true
socket.BLOCKSIZE = 2048

---@class socket.Common
local Common = {}

--- Closes it; it can be used no more.
function Common:close() end

--- Seconds to wait in each call (mode "b", the default), or for the whole
--- operation ("t"); nil or negative waits as long as it takes.
---@param seconds? number
---@param mode? "b"|"t"|"r"
---@return number
function Common:settimeout(seconds, mode) end

---@return number? block
---@return number? total
function Common:gettimeout() end

---@return string? address
---@return integer? port
---@return string? family "inet" or "inet6"
function Common:getsockname() end

---@return string? address
---@return integer? port
---@return string? family
function Common:getpeername() end

---@return "inet"|"inet6"
function Common:getfamily() end

---@return integer
function Common:getfd() end

---@param fd integer
function Common:setfd(fd) end

--- Whether there is something to read without waiting.
---@return boolean
function Common:dirty() end

--- "keepalive", "reuseaddr", "reuseport", "tcp-nodelay", "linger",
--- "broadcast", "ipv6-v6only", "ip-multicast-ttl", ...
---@param name string
---@param value any
---@return 1? ok
---@return string? why
function Common:setoption(name, value) end

---@param name string
---@return any
function Common:getoption(name) end

--- A TCP socket: a master as tcp() makes it, a client once connected, a
--- server once listening.
---@class socket.TCP: socket.Common
local TCP = {}

---@param address string
---@param port integer
---@return 1? ok
---@return string? why
function TCP:connect(address, port) end

---@param address string "*" for every address
---@param port integer 0 for any
---@return 1? ok
---@return string? why
function TCP:bind(address, port) end

---@param backlog? integer
---@return 1? ok
---@return string? why
function TCP:listen(backlog) end

--- A client that connected to this server.
---@return socket.TCP? client
---@return string? why
function TCP:accept() end

--- A line ("*l", the default, without its end), everything until the
--- peer closes ("*a"), or n bytes; prefix comes first.
---@param pattern? "*l"|"*a"|integer
---@param prefix? string
---@return string? data
---@return string? why "timeout", "closed"
---@return string? partial what came before it stopped
function TCP:receive(pattern, prefix) end

--- Sends data, or bytes i to j of it.
---@param data string
---@param i? integer
---@param j? integer
---@return integer? last the index of the last byte sent
---@return string? why
---@return integer? partial the last byte sent before it stopped
function TCP:send(data, i, j) end

---@param how? "both"|"send"|"receive"
---@return 1
function TCP:shutdown(how) end

---@return number received
---@return number sent
---@return number age
function TCP:getstats() end

---@return 1
function TCP:setstats(received, sent, age) end

TCP.setpeername = TCP.connect
TCP.setsockname = TCP.bind

---@class socket.UDP: socket.Common
local UDP = {}

--- A datagram, of at most size bytes.
---@param size? integer
---@return string? data
---@return string? why
function UDP:receive(size) end

---@param size? integer
---@return string? data
---@return string|nil address_or_why
---@return integer? port
function UDP:receivefrom(size) end

--- To the peer setpeername chose.
---@param data string
---@return 1? ok
---@return string? why
function UDP:send(data) end

---@param data string
---@param address string
---@param port integer
---@return 1? ok
---@return string? why
function UDP:sendto(data, address, port) end

--- Fixes the peer, or with "*" frees it again.
---@param address string
---@param port? integer
---@return 1? ok
---@return string? why
function UDP:setpeername(address, port) end

---@param address string
---@param port integer
---@return 1? ok
---@return string? why
function UDP:setsockname(address, port) end

--- A TCP master of either family; tcp4 and tcp6 choose one.
---@return socket.TCP? master
---@return string? why
function socket.tcp() end
---@return socket.TCP? master
---@return string? why
function socket.tcp4() end
---@return socket.TCP? master
---@return string? why
function socket.tcp6() end

---@return socket.UDP? sock
---@return string? why
function socket.udp() end
---@return socket.UDP? sock
---@return string? why
function socket.udp4() end
---@return socket.UDP? sock
---@return string? why
function socket.udp6() end

--- A TCP client connected to address and port.
---@param address string
---@param port integer
---@param locaddr? string
---@param locport? integer
---@param family? "inet"|"inet6"
---@return socket.TCP? client
---@return string? why
function socket.connect(address, port, locaddr, locport, family) end
---@return socket.TCP? client
---@return string? why
function socket.connect4(address, port, locaddr, locport) end
---@return socket.TCP? client
---@return string? why
function socket.connect6(address, port, locaddr, locport) end

--- A TCP server listening on host and port.
---@param host string
---@param port integer
---@param backlog? integer
---@return socket.TCP? server
---@return string? why
function socket.bind(host, port, backlog) end

--- Waits until a socket in recvt can be read or one in sendt written, at
--- most timeout seconds.
---@param recvt? socket.Common[]
---@param sendt? socket.Common[]
---@param timeout? number
---@return socket.Common[] readable
---@return socket.Common[] writable
---@return string? why "timeout"
function socket.select(recvt, sendt, timeout) end

--- Seconds since 1970, to the microsecond.
---@return number
function socket.gettime() end

---@param seconds number
function socket.sleep(seconds) end

--- The arguments after the first n.
---@param n integer
---@return any ...
function socket.skip(n, ...) end

--- Raises its second argument as an error when the first is nil or false,
--- in a way protect catches; else gives all its arguments.
---@return any ...
function socket.try(...) end

--- A try that calls finalizer before raising.
---@param finalizer fun()
---@return fun(...): ...
function socket.newtry(finalizer) end

--- fn, with try's errors returned as nil, message.
---@param fn function
---@return function
function socket.protect(fn) end

--- An LTN12 sink writing to sock: "close-when-done" or "keep-open".
---@param mode string
---@param sock socket.TCP
---@return function
function socket.sink(mode, sock) end

--- An LTN12 source reading from sock: "by-length" (with a length),
--- "until-closed", or "default".
---@param mode string
---@param sock socket.TCP
---@param length? integer
---@return function
function socket.source(mode, sock, length) end

socket.sinkt = {}
socket.sourcet = {}

--- A function that calls the entry of table chosen by its first argument.
---@param table table<string, function>
---@return function
function socket.choose(table) end

socket.dns = {}

--- An address of a name, and what is known about it.
---@param name string
---@return string? address
---@return table|string info_or_why { name =, alias = {}, ip = {} }
function socket.dns.toip(name) end

---@param address string
---@return string? name
---@return table|string info_or_why
function socket.dns.tohostname(address) end

---@param name string
---@return { family: string, addr: string }[]? addresses
---@return string? why
function socket.dns.getaddrinfo(name) end

---@param host? string
---@param port? integer|string
---@return string[]? names
---@return string? service
function socket.dns.getnameinfo(host, port) end

---@return string
function socket.dns.gethostname() end

return socket
