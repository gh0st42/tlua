# `socket.tp` — generic line-based protocol helper

`socket.tp` factors out the request/reply pattern shared by text-based
control protocols like FTP and SMTP (send a command line, read back a
`NNN reply text` line, possibly multi-line). It's what
[`socket.ftp`](socket.ftp.md) and [`socket.smtp`](socket.smtp.md) are
built on; use it directly if you're implementing a similar protocol of
your own.

```lua
local tp = require("socket.tp")
```

## `tp.connect(host, port, timeout, create)`

Connects to `host:port` and returns a "transport" object wrapping the
connection with reply-parsing helpers.

- `host` (string), `port` (number)
- `timeout` (number, optional) — seconds, defaults to `tp.TIMEOUT` (`60`).
- `create` (function, optional) — a socket constructor to use instead of
  `socket.tcp` (e.g. to plug in a TLS socket).
- Returns: the transport object, or `nil, errmsg`.

```lua
local tp = require("socket.tp")

local c = assert(tp.connect("ftp.example.com", 21))
```

## Transport object methods

### `c:command(cmd, arg)`

Sends a command line: `CMD arg\r\n` (the command name is upper-cased
automatically; `arg` is optional).

```lua
c:command("user", "anonymous")
```

### `c:check(ok)`

Reads one server reply (following the `NNN text` / multi-line
`NNN-text ... NNN text` convention shared by FTP and SMTP) and validates
its status code against `ok`:

- `ok` as a **string** or **number** — a Lua pattern the code must match
  (e.g. `"2.."` for any 2xx code, or `250` for exactly that code).
- `ok` as a **table** — a list of patterns; the code must match one of
  them.
- `ok` as a **function** — called as `ok(code, reply)`; its return value
  is returned as-is (for full custom validation).
- Returns: `code, reply` (the numeric code and the full reply text) if
  the code matched, or `nil, reply` if it didn't.

```lua
c:command("user", "anonymous")
local code, reply = c:check({ "2..", 331 })
if code == 331 then
    c:command("pass", "me@example.com")
    c:check("2..")
end
```

### `c:send(data)` / `c:receive(pattern)`

Raw passthrough to the underlying socket's `send`/`receive` (see
[`socket`](socket.md#tcp-specific)) — useful for sending/reading the
message body once a control command has been acknowledged.

### `c:source(source, step)`

Pumps an [LTN12](socket.md#sources-and-sinks-ltn12-helpers) `source`
into the connection (used to stream a message body, e.g. for SMTP
`DATA`).

- `source` — an LTN12 source.
- `step` (optional) — an LTN12 pump step function; defaults to
  `ltn12.pump.step`.
- Returns: `1` on success, or `nil, errmsg`.

### `c:sink(sink, pattern)`

Reads one chunk (via `receive(pattern)`) and feeds it into an LTN12
`sink` in a single step.

### Other methods

- `c:getcontrol()` — returns the underlying raw socket (the control
  connection), for cases where you need lower-level access.
- `c:getpeername()` / `c:getsockname()` — proxy to the control socket.
- `c:getfd()` / `c:dirty()` — proxy to the control socket (for use with
  `socket.select`).
- `c:close()` — closes the control connection.

## Fields

- `tp.TIMEOUT` — default connection timeout in seconds (`60`); assign to
  it to change the default for subsequent `tp.connect` calls that don't
  pass their own `timeout`.

## Example: a minimal custom control-protocol client

```lua
local tp = require("socket.tp")

local c = assert(tp.connect("localhost", 4000))
c:command("HELLO", "world")
local code, reply = c:check("2..")
if not code then
    error("server rejected HELLO: " .. reply)
end
print(reply)
c:close()
```
