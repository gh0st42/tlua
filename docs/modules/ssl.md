# `ssl` — TLS

`ssl` is [LuaSec](https://github.com/brunoos/luasec) 1.3.2: a
[LuaSocket](socket.md) TCP connection wrapped in TLS, as a client or as a
server, with certificates, ALPN and SNI. Its Go half is built on Go's
`crypto/tls`, and LuaSec's own `ssl.lua` and `ssl/https.lua` run on top of
it as their author wrote them, so code written for LuaSec runs as it is. On
Linux and macOS, like `socket.core`.

```lua
local ssl = require "ssl"
```

## A client

```lua
local socket, ssl = require "socket", require "ssl"

local tcp = assert(socket.connect("example.org", 443))
local conn = assert(ssl.wrap(tcp, { mode = "client", protocol = "any", verify = "peer" }))
conn:sni("example.org")
assert(conn:dohandshake())
conn:send("GET / HTTP/1.0\r\nHost: example.org\r\n\r\n")
print(conn:receive("*l"))           --> HTTP/1.1 200 OK
print(conn:info("protocol"))        --> TLSv1.3
conn:close()
```

For plain web requests, `ssl.https` (and `socket.http` given an `https://`
address) does all of that:

```lua
local https = require "ssl.https"
local body, code = https.request("https://example.org/")
```

## Connections

### `ssl.wrap(sock, params)`

Takes a connected TCP socket over, which is the connection's from then on:
use the connection instead. Returns the connection, or `nil, why`. `params`
is a table (below) or a context made by `ssl.newcontext`.

### `ssl.newcontext(params)`

A context made of the parameters, to hand to `wrap` again and again so that
certificates are read once.

### `conn:dohandshake()`

Shakes hands. `true`, or `nil` and why.

### `conn:send(data [, i [, j]])`, `conn:receive([pattern [, prefix]])`

As a socket's, with LuaSocket's patterns (`"*l"`, `"*a"` or a number).

### `conn:settimeout(seconds [, mode])`, `conn:close()`

As a socket's. A read or a write that runs out of time says `"wantread"` or
`"wantwrite"`, as LuaSec does, so non-blocking code (copas and the like)
works. `conn:want()` says which it last waited for.

### `conn:dirty()`, `conn:getfd()`

Whether there is something to read without waiting, and the connection's
file descriptor. `socket.select` takes connections too.

## Parameters

| Field | |
|---|---|
| `mode` | `"client"` or `"server"` |
| `protocol` | `"any"` (TLS 1.2 and 1.3), `"tlsv1_2"`, `"tlsv1_3"`, ... |
| `key`, `certificate` | PEM files, or the PEM itself; `certificates` for several `{ key, certificate, password }` |
| `password` | for an encrypted key, a string or a function |
| `cafile`, `capath` | the certificates to trust; the system's when neither is given |
| `verify` | `"none"` (the default), `"peer"`, `"fail_if_no_peer_cert"` |
| `verifyext` | `"lsec_continue"`, `"lsec_ignore_purpose"` |
| `options` | `"no_tlsv1_2"` and the like |
| `alpn` | protocols to offer, or a server's function to choose |
| `curve`, `curveslist`, `depth` | named curves; the longest chain to accept |

A path in `key`, `certificate` or `cafile` is read out of a fused program or
a bundle first. Keys are PEM: PKCS#1, PKCS#8 or EC, and the old encrypted PEM
with a password, not encrypted PKCS#8.

## Certificates

`conn:getpeercertificate()`, `getpeerchain()`, `getlocalcertificate()`,
`getlocalchain()` and `ssl.loadcertificate(pem)` give certificates, with:

| | |
|---|---|
| `cert:digest("sha256")` | the fingerprint |
| `cert:subject()`, `cert:issuer()` | tables of names |
| `cert:extensions()` | the alternative names |
| `cert:notbefore()`, `cert:notafter()`, `cert:validat(time)` | when it is valid |
| `cert:serial()`, `cert:pubkey()`, `cert:pem()`, `cert:getsignaturename()` | |
| `cert:issued(other)` | whether it issued another |

`conn:getpeerverification()` says whether the peer's chain checked out, and
if not, why, in OpenSSL's words (`"self-signed certificate"`, `"certificate
has expired"`, ...).

## Also

`conn:getalpn()` is the protocol ALPN settled on, `conn:getsniname()` the
server name a client asked for, `conn:info()` the cipher, protocol and bits
(or one of them by name), `conn:getstats()` the bytes received and sent and
the connection's age, and `conn:exportkeyingmaterial(label, n)` keying
material. A server picks a certificate by the name a client asks for with
`conn:sni(map)`.

## Checking certificates

**As in LuaSec, a connection checks nothing unless it is told to.** Without
`verify = "peer"` the handshake succeeds whatever the peer shows, and
`getpeerverification()` says what was wrong. `ssl.https` also asks for no
checking, as LuaSec's does; pass `verify = "peer"` in a request table to
have it. With `verify = "peer"` a bad chain fails the handshake with
`"certificate verify failed"`.

Only the chain is checked, not that the certificate names the host: LuaSec
leaves that to the program, with `cert:extensions()` and `cert:subject()`.

## Gemini and certificates nobody vouches for

Gemini capsules mostly sign their own certificates. A client trusts a
host's certificate the first time and expects the same one after.
`examples/net/gemini.lua` is such a client:

```lua
local conn = assert(ssl.wrap(tcp, { mode = "client", protocol = "tlsv1_2" }))
conn:sni(host)
assert(conn:dohandshake())
local fingerprint = conn:getpeercertificate():digest("sha256")
-- compare with the one this host showed before, then:
conn:send("gemini://" .. host .. "/\r\n")
print(conn:receive("*l"))           --> 20 text/gemini
```

A capsule that asks who is visiting gets a client certificate, given as `key`
and `certificate`, which is sent whatever the server asks for.

## What differs from LuaSec on OpenSSL

- With `verify = "peer"` and no `cafile` or `capath`, the system's trusted
  roots are used; LuaSec loads none, and every chain fails.
- `ciphers`, `ciphersuites` and `dhparam` are accepted and left to Go, which
  chooses its own cipher suites; `protocol = "any"` allows TLS 1.2 and 1.3,
  Go's floor.
- With `verifyext = "lsec_continue"`, `getpeerverification()` gives the
  reason as a string, not LuaSec's table by depth.
- `getfinished()` and `getpeerfinished()` return nothing, and there is no
  PSK or DANE.
- The `ssl` modules exist on Linux and macOS only.
