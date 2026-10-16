# `socket.http` — HTTP/1.1 client

`socket.http` is LuaSocket's HTTP client: a simple string-based API for
quick requests, and a generic table-based API (built on
[LTN12](socket.md#sources-and-sinks-ltn12-helpers) sources/sinks) for
streaming request/response bodies, custom headers, authentication, and
redirects.

```lua
local http = require("socket.http")
```

## `http.request(url_or_reqt, body)`

The single entry point, overloaded two ways:

### Simple form: `http.request(url, body)`

- `url` (string) — the URL to request.
- `body` (string, optional) — if given, sends a `POST` with this string
  as the body and `Content-Type: application/x-www-form-urlencoded`.
  Omit it for a `GET`.
- Returns: `body, code, headers, status_line` — the full response body
  (string), the numeric status code, a table of lower-cased response
  headers, and the raw status line. Returns `nil, errmsg` on a
  connection-level failure (DNS, connect, timeout, etc.).

```lua
local http = require("socket.http")

local body, code = http.request("http://example.com")
print(code, #body)

local resp_body, code2 = http.request("http://example.com/api", "name=lua&x=1")
print(code2, resp_body)
```

### Generic form: `http.request(reqt)`

`reqt` is a table describing the request:

| Field          | Meaning                                                    |
|-----------------|--------------------------------------------------------------|
| `url`          | the target URL (parsed for host/port/path/etc.)              |
| `host`, `port`, `path`, `params`, `query`, `fragment`, `scheme`, `user`, `password` | override individual parts of `url` |
| `method`       | HTTP method, e.g. `"GET"`, `"POST"`, `"PUT"` (default `"GET"`) |
| `headers`      | table of request headers (case-insensitive keys)              |
| `source`       | an [LTN12 source](socket.md#sources-and-sinks-ltn12-helpers) for the request body |
| `sink`         | an LTN12 sink that receives the response body                 |
| `step`         | an LTN12 pump step function (advanced; rarely needed)          |
| `proxy`        | proxy URL to use for this request                              |
| `redirect`     | set to `false` to disable following redirects                 |
| `maxredirects` | maximum redirects to follow (default `5`)                     |
| `create`       | a socket constructor to use instead of `socket.tcp`            |

Returns `1, code, headers, status_line` on success (note: unlike the
simple form there's no body here — it went to your `sink`), or
`nil, errmsg` on failure.

```lua
local http = require("socket.http")
local ltn12 = require("ltn12")

-- Stream a large download straight to a file:
local f = assert(io.open("page.html", "wb"))
local ok, code = http.request({
    url = "http://example.com/",
    sink = ltn12.sink.file(f),
})
print(ok, code)

-- POST a JSON body with custom headers, capturing the response in a table:
local json = require("dkjson")
local response_chunks = {}
local payload = json.encode({ name = "Ada" })
local ok2, code2, headers = http.request({
    url = "http://example.com/api/users",
    method = "POST",
    source = ltn12.source.string(payload),
    sink = ltn12.sink.table(response_chunks),
    headers = {
        ["content-type"] = "application/json",
        ["content-length"] = tostring(#payload),
    },
})
print(code2, table.concat(response_chunks))
```

### Basic authentication

Set `user`/`password` on the request table (or in the URL,
`http://user:pass@host/...`) to send an `Authorization: Basic ...`
header automatically:

```lua
http.request({
    url = "http://example.com/private",
    user = "alice",
    password = "s3cret",
})
```

### Redirects

3xx redirects (301/302/303/307) are followed automatically for `GET`/
`HEAD` requests, up to `maxredirects` (default `5`), as long as you
haven't set `redirect = false` and haven't already supplied a `source`
(since a request body normally can't be safely re-sent to a different
server).

### HTTPS

`socket.http` itself only speaks plain HTTP. Requesting an `https://` URL
requires [LuaSec](https://github.com/brunoos/luasec) to be installed
(`require("ssl.https")`); if it isn't available, an `https://` request
raises an error.

## Fields

- `http.TIMEOUT` — connection timeout in seconds (default `60`);
  assign to change it globally.
- `http.USERAGENT` — the `User-Agent` sent by default (defaults to the
  LuaSocket version string, e.g. `"LuaSocket 3.1.0"`).
- `http.PROXY` — a proxy URL used for every request that doesn't specify
  its own `proxy` field.
- `http.schemes` — table describing the supported URL schemes (`http`,
  `https`) and their default ports/socket constructors.

## Low-level API

For full control over the request/response lifecycle (e.g. pipelining,
or protocols layered on HTTP), `http.open(host, port, create)` returns a
connection object with:

- `h:sendrequestline(method, uri)`
- `h:sendheaders(headers)`
- `h:sendbody(headers, source, step)`
- `h:receivestatusline()` — returns `code, status_line`
- `h:receiveheaders()` — returns a table of lower-cased headers
- `h:receivebody(headers, sink, step)`
- `h:close()`

This is exactly what `http.request` uses internally (see
`socket.http.lua`) — reach for it only when the generic table-based API
doesn't fit.

## Example: fetching and checking a status code

```lua
local http = require("socket.http")

local body, code, headers = http.request("http://example.com/health")
if code ~= 200 then
    error(("health check failed: HTTP %s"):format(tostring(code)))
end
print("content-type:", headers["content-type"])
print(body)
```
