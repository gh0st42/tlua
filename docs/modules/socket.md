# `socket` — TCP/UDP networking (LuaSocket core)

`socket` is [LuaSocket](https://github.com/lunarmodules/luasocket) 3.1.0's
core module: raw TCP and UDP sockets, DNS resolution, `select()`-based
multiplexing, and the small utility layer (`try`/`protect`, and the
[LTN12](#sources-and-sinks-ltn12-helpers) source/sink registry) that the
higher-level [`socket.http`](socket.http.md), [`socket.ftp`](socket.ftp.md),
and [`socket.smtp`](socket.smtp.md) clients are built on. Most of it
(socket creation, I/O, `select`) is LuaSocket's C half, `socket.core`,
which tlua carries written in Go, on Linux and macOS; this document
describes that combined API as seen from Lua. The README's "C modules"
section lists what differs: no `socket.unix` or `socket.serial`, and no
`socket.core` on Windows. For TLS, see `ssl` there.

```lua
local socket = require("socket")
```

## Creating sockets

### `socket.tcp()` / `socket.tcp4()` / `socket.tcp6()`

Creates a TCP "master" object — unconnected and unbound, used to either
connect out (as a client) or bind+listen (as a server). `tcp()` and
`tcp4()` create an IPv4 socket; `tcp6()` creates an IPv6 socket.

- Returns: the socket object, or `nil, errmsg` on failure.

```lua
local socket = require("socket")

local server = assert(socket.tcp())
assert(server:bind("*", 8080))
assert(server:listen(32))
```

### `socket.udp()` / `socket.udp4()` / `socket.udp6()`

Creates an unconnected UDP object (IPv4 or IPv6).

- Returns: the socket object, or `nil, errmsg` on failure.

```lua
local udp = assert(socket.udp())
udp:setsockname("*", 9999)
```

### `socket.connect(address, port, locaddr, locport)`

Convenience function: creates a TCP socket and connects it in one step.

- `address`, `port` — remote host/port to connect to.
- `locaddr`, `locport` (optional) — local address/port to bind to first.
- Returns: a connected TCP client object, or `nil, errmsg`.

```lua
local client = assert(socket.connect("example.com", 80))
```

`socket.connect4(...)` / `socket.connect6(...)` are the IPv4-only /
IPv6-only variants (they force address family resolution instead of
picking whichever the DNS returns first).

### `socket.bind(host, port, backlog)`

Convenience function: resolves `host`, creates a TCP socket of the right
address family, sets `SO_REUSEADDR`, binds, and starts listening — the
common server bootstrap in one call.

- `host` (string) — address to bind to; `"*"` means "any address"
  (`0.0.0.0`).
- `port` (number)
- `backlog` (number, optional) — passed to `listen`.
- Returns: a listening TCP master object ready for `accept()`, or
  `nil, errmsg`.

```lua
local server = assert(socket.bind("*", 8080))
while true do
    local client = server:accept()
    client:send("hello\n")
    client:close()
end
```

## Socket object methods

The following methods apply to the socket objects returned above (a
"master"/unconnected socket becomes a "client" once `connect`ed, or a
"server" once bound+listening — `accept()` on a server returns a new
client object per incoming connection).

### Common to TCP and UDP objects

- `sock:settimeout(value, mode)` — sets a timeout in seconds (or `nil`
  for no timeout / blocking forever); `mode` is `"b"` (per-blocking-call,
  the default) or `"t"` (total, across an entire operation).
- `sock:setoption(name, value)` — sets a low-level socket option, e.g.
  `"keepalive"`, `"reuseaddr"`, `"tcp-nodelay"`, `"linger"` (value
  `{on = true, timeout = n}`), `"broadcast"` (UDP), `"sndbuf"`,
  `"rcvbuf"`, `"ip-multicast-loop"`, `"ip-add-membership"` (UDP
  multicast, value `{multiaddr = ..., interface = ...}`).
- `sock:getsockname()` — returns `address, port` of the local side.
- `sock:getpeername()` — returns `address, port` of the remote side
  (once connected).
- `sock:getstats()` — returns `received, sent, age` (bytes transferred
  and seconds since the object was created).
- `sock:setstats(received, sent, age)` — overrides the counters above.
- `sock:getfd()` / `sock:setfd(fd)` — get/set the underlying OS file
  descriptor (advanced; mainly for use with `select`/custom polling).
- `sock:dirty()` — `true` if the object has data buffered that
  `select()` wouldn't otherwise report as readable.
- `sock:close()` — closes the socket.

### TCP-specific

- `sock:bind(address, port)` — binds a master socket to a local address.
- `sock:listen(backlog)` — puts a bound master socket in listening mode.
- `sock:accept()` — blocks (subject to the timeout) until an incoming
  connection arrives; returns a new connected client socket.
- `sock:connect(address, port)` — connects a master socket, turning it
  into a client.
- `sock:send(data, i, j)` — sends `data:sub(i, j)` (defaults to the
  whole string); returns the number of bytes sent, or `nil, errmsg,
  last_byte_sent` on error/partial send (e.g. `"timeout"` or
  `"closed"`) — check `last_byte_sent` to know how much still needs
  sending.
- `sock:receive(pattern, prefix)` — reads data; `pattern` is a byte
  count, `"*l"`/`"l"` (a line, default), or `"*a"`/`"a"` (everything
  until the connection closes). `prefix` is prepended to the result.
  Returns `data`, or `nil, errmsg, partial_data` on error (e.g. on a
  `"timeout"`, `partial_data` holds whatever was read so far).
- `sock:shutdown(mode)` — shuts down one direction of a connected
  socket: `"both"` (default), `"send"`, or `"receive"`.

```lua
local socket = require("socket")

local client = assert(socket.connect("example.com", 80))
client:settimeout(5)
client:send("GET / HTTP/1.0\r\n\r\n")
local line = client:receive("*l")
print(line) --> HTTP/1.1 200 OK
client:close()
```

### UDP-specific

- `sock:sendto(datagram, ip, port)` — sends a datagram to a specific
  address (unconnected socket).
- `sock:receivefrom(size)` — receives a datagram; returns
  `datagram, ip, port`, or `nil, errmsg`.
- `sock:setpeername(address, port)` — "connects" the UDP socket to a
  fixed remote address, after which you use `send`/`receive` instead of
  `sendto`/`receivefrom`; pass `"*"` as address to disconnect.
- `sock:setsockname(address, port)` — binds the local address before
  first use (e.g. `"*", port` to receive on a fixed port).
- `sock:send(datagram)` / `sock:receive(size)` — usable once
  `setpeername` has "connected" the socket.

```lua
local socket = require("socket")

local udp = assert(socket.udp())
udp:sendto("ping", "127.0.0.1", 9999)

local server = assert(socket.udp())
server:setsockname("*", 9999)
local data, ip, port = server:receivefrom()
print(data, ip, port) --> ping  127.0.0.1  <source port>
```

## Multiplexing: `socket.select(recvt, sendt, timeout)`

Waits for any socket in `recvt` to become readable or in `sendt` to
become writable (like POSIX `select()`).

- `recvt`, `sendt` (arrays of sockets, or `nil`)
- `timeout` (number, optional) — seconds to wait; `nil`/omitted waits
  forever.
- Returns three values: `readable`, `writable` (arrays of the sockets
  that are ready — also indexable by the socket itself, e.g.
  `readable[sock]`), and an error string (`"timeout"` if the call timed
  out with nothing ready).

```lua
local socket = require("socket")

local readable = socket.select({ sock1, sock2 }, nil, 2)
for _, sock in ipairs(readable) do
    print(sock:receive())
end
```

## Timing

- `socket.sleep(seconds)` — blocks the current process for `seconds`.
- `socket.gettime()` — returns a high-resolution timestamp (seconds,
  with sub-second precision), useful for measuring elapsed time.

```lua
local socket = require("socket")

local start = socket.gettime()
socket.sleep(0.5)
print(("elapsed: %.3fs"):format(socket.gettime() - start))
```

## DNS: `socket.dns`

- `socket.dns.getaddrinfo(address)` — resolves a hostname (or IP) to a
  list of `{ family = "inet"|"inet6", addr = "..." }` tables.
- `socket.dns.gethostname()` — returns the local machine's hostname.
- `socket.dns.tohostname(address)` — reverse-resolves an IP to a
  hostname; returns `hostname, resolved_table`.
- `socket.dns.toip(address)` — resolves a hostname to an IP address;
  returns `ip, resolved_table`.

```lua
local socket = require("socket")

local ip = socket.dns.toip("example.com")
print(ip) --> e.g. 93.184.216.34
```

## Error handling helpers

LuaSocket's higher-level modules (`http`, `ftp`, `smtp`, `tp`) are all
written using these two helpers, and they're useful for your own code
too — they turn a series of `ok, err = thing()` checks into ordinary Lua
errors you can `pcall`/wrap once, instead of checking every call.

### `socket.try(ret, ...)` / `socket.newtry(finalizer)`

`socket.try(...)` checks whether its first argument is truthy; if so, it
returns all its arguments unchanged, and if not, it raises a Lua error
with the second argument (or a default message) — designed to be used as
`self.try(sock:send(data))`.

`socket.newtry(finalizer)` returns a fresh `try` function that calls
`finalizer()` before raising the error (used to guarantee a socket gets
closed on failure, as seen throughout `socket.tp`/`http`/`ftp`/`smtp`).

```lua
local socket = require("socket")

local function get_line(sock)
    local try = socket.newtry(function() sock:close() end)
    local line = try(sock:receive("*l"))
    return line
end
```

### `socket.protect(func)`

Wraps a function that raises errors (via `try`) into one that instead
returns `nil, errmsg` — the standard Lua convention — so callers don't
need `pcall`.

```lua
local socket = require("socket")

local safe_divmod = socket.protect(function(a, b)
    socket.try(b ~= 0, "division by zero")
    return math.floor(a / b), a % b
end)

local q, r = safe_divmod(7, 0)
print(q, r) --> nil  division by zero
```

### `socket.skip(n, ...)`

Returns all arguments from position `n + 1` onward — handy for
discarding the first few return values of something like
`string.find`, e.g. `socket.skip(2, string.find(s, pattern))` to get
just the captures.

## Sources and sinks (LTN12 helpers)

`socket.source(name, sock, ...)` / `socket.sink(name, sock, ...)` create
[LTN12](http://w3.impa.br/~diego/software/luasocket/ltn12.html)-compatible
source/sink objects backed by a socket, chosen from a registry
(`socket.sourcet` / `socket.sinkt`) by name:

| Name (sink)         | Behavior                                             |
|----------------------|-------------------------------------------------------|
| `"keep-open"` (default) | sends each chunk; leaves the socket open at the end |
| `"close-when-done"` | sends each chunk; closes the socket when the source is exhausted |

| Name (source)        | Behavior                                             |
|-----------------------|-------------------------------------------------------|
| `"until-closed"` (default) | reads until the connection closes                |
| `"by-length"`        | reads exactly `length` bytes (extra `length` argument) |

`socket.http` adds an additional `"http-chunked"` source/sink for
HTTP/1.1 chunked transfer encoding. These are mostly used internally by
`socket.http`/`socket.ftp`, together with the
[`ltn12`](http://w3.impa.br/~diego/software/luasocket/ltn12.html) module's
`pump.all` to move data between a source and a sink.

```lua
local socket = require("socket")
local ltn12 = require("ltn12")

-- Stream a socket's data into a table until the peer closes the connection:
local chunks = {}
ltn12.pump.all(socket.source("until-closed", client), ltn12.sink.table(chunks))
print(table.concat(chunks))
```

## Other fields

- `socket._VERSION` — version string, e.g. `"LuaSocket 3.1.0"`.
- `socket.BLOCKSIZE` — default chunk size (bytes) used by the built-in
  sources/sinks above (`2048`).
