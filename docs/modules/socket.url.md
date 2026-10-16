# `socket.url` — URL parsing and composition

`socket.url` (LuaSocket's `URL 1.0.3` module) parses and builds URLs
according to RFC 2396, and resolves relative URLs against a base — used
internally by [`socket.http`](socket.http.md) and
[`socket.ftp`](socket.ftp.md), and useful on its own.

```lua
local url = require("socket.url")
```

## Functions

### `url.parse(url, default)`

Parses a URL into its components.

- `url` (string) — the URL to parse.
- `default` (table, optional) — default values merged into the result
  before parsing (e.g. `{ scheme = "http" }` to assume a scheme when the
  URL doesn't specify one).
- Returns: a table with (any subset of, depending on what the URL
  contains) `scheme`, `authority`, `userinfo`, `user`, `password`,
  `host`, `port`, `path`, `params`, `query`, `fragment`; or
  `nil, "invalid url"` for an empty/nil URL.

```lua
local url = require("socket.url")

local parsed = url.parse("https://user:pass@example.com:8080/a/b?x=1#frag")
print(parsed.scheme)   --> https
print(parsed.user)     --> user
print(parsed.password) --> pass
print(parsed.host)     --> example.com
print(parsed.port)     --> 8080
print(parsed.path)     --> /a/b
print(parsed.query)    --> x=1
print(parsed.fragment) --> frag
```

### `url.build(parsed)`

The inverse of `parse`: reassembles a URL string from a components
table (as returned by `parse`, or built by hand).

- `parsed` (table) — same fields as `url.parse`'s result.
- Returns: the URL string.

```lua
local url = require("socket.url")

print(url.build({
    scheme = "http",
    host = "example.com",
    path = "/search",
    query = "q=lua",
}))
--> http://example.com/search?q=lua
```

### `url.absolute(base_url, relative_url)`

Resolves `relative_url` against `base_url` per RFC 2396 (the same rule
browsers use for relative links).

- `base_url` (string or a table already parsed by `url.parse`)
- `relative_url` (string)
- Returns: the resulting absolute URL string.

```lua
local url = require("socket.url")

print(url.absolute("http://example.com/a/b/c", "../d")) --> http://example.com/a/d
print(url.absolute("http://example.com/a/b/", "d"))      --> http://example.com/a/b/d
print(url.absolute("http://example.com/a", "https://other.com/x")) --> https://other.com/x
```

### `url.escape(s)` / `url.unescape(s)`

Percent-encodes/decodes a string for safe use as a URL component (query
string value, etc.) — every byte other than letters, digits, and `_` is
escaped as `%XX` by `escape`.

```lua
local url = require("socket.url")

print(url.escape("a b/c"))        --> a%20b%2fc
print(url.unescape("a%20b%2fc"))  --> a b/c
```

### `url.parse_path(path)` / `url.build_path(parsed, unsafe)`

Split a URL path into its `/`-separated, unescaped segments, and
reassemble them back into a path string.

- `url.parse_path(path)` — returns an array of unescaped segments; the
  array also has `is_absolute` (`1` if the path started with `/`) and
  `is_directory` (`1` if it ended with `/`) fields.
- `url.build_path(parsed, unsafe)` — reassembles the segments into a
  path string, re-adding the leading/trailing slash implied by
  `is_absolute`/`is_directory`. Segments are percent-escaped as needed
  unless `unsafe` is true.

```lua
local url = require("socket.url")

local segments = url.parse_path("/a/b%20c/")
print(segments[1], segments[2])            --> a  b c
print(segments.is_absolute, segments.is_directory) --> 1  1

print(url.build_path(segments)) --> /a/b%20c/
```

## Field reference

| Field       | Meaning                                             |
|-------------|-------------------------------------------------------|
| `scheme`    | e.g. `"http"`, `"ftp"`                                |
| `authority` | the raw `user:pass@host:port` portion                |
| `userinfo`  | the raw `user:pass` portion                          |
| `user`, `password` | split-out credentials                         |
| `host`      | hostname or IP (brackets stripped for IPv6 literals) |
| `port`      | port, as a string                                    |
| `path`      | path component, leading `/` included                 |
| `params`    | the (rarely used) `;params` segment                  |
| `query`     | the `?query` string, without the `?`                 |
| `fragment`  | the `#fragment`, without the `#`                     |

## Example: building an API request URL

```lua
local url = require("socket.url")

local base = { scheme = "https", host = "api.example.com", path = "/v1/search" }
base.query = "q=" .. url.escape("hello world") .. "&limit=10"

print(url.build(base))
--> https://api.example.com/v1/search?q=hello%20world&limit=10
```
