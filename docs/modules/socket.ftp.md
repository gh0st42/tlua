# `socket.ftp` — FTP client

`socket.ftp` implements an FTP client on top of
[`socket.tp`](socket.tp.md), with a high-level `get`/`put`/`command` API
(streaming via [LTN12](socket.md#sources-and-sinks-ltn12-helpers)
sources/sinks) plus the lower-level connection object for finer control.

```lua
local ftp = require("socket.ftp")
```

## High-level API

### `ftp.get(url_or_gett)`

Downloads a file.

**Simple form:** `ftp.get(url)` where `url` looks like
`ftp://[user[:password]@]host[:port]/path[;type=a|i]` — returns the file
contents as a string, or `nil, errmsg`.

```lua
local ftp = require("socket.ftp")

local data, err = ftp.get("ftp://ftp.example.com/pub/readme.txt")
if not data then error(err) end
print(data)
```

**Table form:** `ftp.get(gett)` where `gett` has:

| Field      | Meaning                                                  |
|------------|-------------------------------------------------------------|
| `host`, `port` | server to connect to (required)                          |
| `user`, `password` | credentials (default to anonymous login, see below)  |
| `type`     | `"a"` (ASCII) or `"i"` (binary/image)                       |
| `path` / `argument` | remote file path (or a raw command argument)        |
| `sink`     | an LTN12 sink to receive the downloaded data (required)     |
| `step`     | an LTN12 pump step function (advanced)                      |
| `create`   | a socket constructor to use instead of `socket.tcp`          |

Returns `1` on success, or `nil, errmsg`. Use this form when you want to
stream the download somewhere other than an in-memory string (e.g.
straight to a file):

```lua
local ftp = require("socket.ftp")
local ltn12 = require("ltn12")

local f = assert(io.open("readme.txt", "wb"))
local ok, err = ftp.get({
    host = "ftp.example.com",
    path = "/pub/readme.txt",
    type = "i",
    sink = ltn12.sink.file(f),
})
```

### `ftp.put(url_or_putt, body)`

Uploads a file.

**Simple form:** `ftp.put(url, body)` — uploads the string `body` to
`url` (same URL format as `get`). Returns the number of bytes sent, or
`nil, errmsg`.

```lua
local ftp = require("socket.ftp")

local sent, err = ftp.put("ftp://user:pass@ftp.example.com/incoming/hello.txt", "hello!\n")
```

**Table form:** `ftp.put(putt)` — same fields as `gett` above, but with
`source` (an LTN12 source) instead of `sink`, and `command` (default
`"stor"`) if you need a different storage command (e.g. `"appe"` to
append). Returns bytes sent, or `nil, errmsg`.

```lua
local ftp = require("socket.ftp")
local ltn12 = require("ltn12")

ftp.put({
    host = "ftp.example.com",
    user = "user",
    password = "pass",
    path = "/incoming/data.bin",
    type = "i",
    source = ltn12.source.file(io.open("data.bin", "rb")),
})
```

### `ftp.command(cmdt)`

Sends one or more raw FTP commands after logging in — for operations
`get`/`put` don't cover (e.g. `dele`, `mkd`, `rnfr`/`rnto`).

- `cmdt.host`, `cmdt.port`, `cmdt.user`, `cmdt.password`, `cmdt.create`
  — connection details, as above.
- `cmdt.command` — a command name (string), or an array of command
  names to send in sequence.
- `cmdt.argument` — the argument for `command` (string), or, when
  `command` is an array, a parallel array of arguments.
- `cmdt.check` — an expected reply pattern (string/number/table, same
  rules as [`tp:check`](socket.tp.md#ccheckok)) to validate each
  command's reply against, or a parallel array when `command` is an
  array.
- Returns: `1` on success, or `nil, errmsg`.

```lua
local ftp = require("socket.ftp")

ftp.command({
    host = "ftp.example.com",
    user = "user",
    password = "pass",
    command = { "cwd", "dele" },
    argument = { "/incoming", "old-file.txt" },
    check = { 250, 250 },
})
```

### Defaults

- `ftp.TIMEOUT` — connection timeout in seconds (default `60`).
- `ftp.USER` / `ftp.PASSWORD` — credentials used when a URL/table
  doesn't specify any (default `"ftp"` / `"anonymous@anonymous.org"` —
  the traditional anonymous-FTP convention; consider setting
  `ftp.PASSWORD` to your own email address).

## Low-level API

`ftp.open(server, port, create)` returns a connection object for manual
control of the FTP session — `get`/`put`/`command` are built on exactly
this:

- `f:greet()` — reads the server's welcome banner.
- `f:login(user, password)`
- `f:epsv()` / `f:pasv()` — request extended/classic passive mode
  (returns `address, port` of the data connection the server opened);
  `f:port(address, port)` / `f:eprt(family, address, port)` are the
  active-mode equivalents, opening a local listening socket for the
  server to connect back to.
- `f:type(type)` — sets transfer type (`"a"`/`"i"`).
- `f:cwd(dir)` — changes the remote working directory.
- `f:send(sendt)` — sends data (fields: `command` default `"stor"`,
  `argument`/`path`, `source`, `step`); returns bytes sent.
- `f:receive(recvt)` — receives data (fields: `command` default
  `"retr"`, `argument`/`path`, `sink`, `step`).
- `f:quit()` — sends `QUIT`.
- `f:close()` — closes the control (and any open data) connection.

```lua
local ftp = require("socket.ftp")
local ltn12 = require("ltn12")

local f = assert(ftp.open("ftp.example.com", 21))
f:greet()
f:login("user", "pass")
f:type("i")
f:epsv()
local t = {}
f:receive({ path = "/pub/readme.txt", sink = ltn12.sink.table(t) })
f:quit()
f:close()
print(table.concat(t))
```
