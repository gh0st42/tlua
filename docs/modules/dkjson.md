# `dkjson` — JSON encoding and decoding

[dkjson](http://dkolf.de/dkjson-lua/) is David Kolf's pure-Lua JSON module
(vendored here as `dkjson`). It encodes Lua values to JSON text and parses
JSON text back into Lua values, with optional support for a faster
[LPeg](http://www.inf.puc-rio.br/~roberto/lpeg/)-based decoder.

```lua
local json = require("dkjson")
```

## Fields

### `json.version`

A string identifying the module version, e.g. `"dkjson 2.11"`.

### `json.null`

A unique sentinel table representing JSON `null`. Lua's `nil` cannot be
stored as a table value (it just removes the key), so use `json.null` when
you need an explicit null in encoded output or want to recognize a
decoded `null`.

```lua
local json = require("dkjson")

local encoded = json.encode({ name = "Ada", nickname = json.null })
print(encoded) --> {"name":"Ada","nickname":null}

local decoded = json.decode(encoded, 1, json.null)
if decoded.nickname == json.null then
    print("nickname is explicitly null")
end
```

## Functions

### `json.encode(value, state)`

Encodes a Lua value as a JSON string. Tables are encoded as JSON arrays
when they look like a sequence (consecutive integer keys starting at `1`),
and as JSON objects otherwise.

- `value` — any encodable Lua value: `nil`, boolean, number, string, or a
  table (nested tables are encoded recursively). Functions, userdata, and
  threads are not encodable.
- `state` (table, optional) — encoding options:
  - `indent` (boolean) — pretty-print with newlines/indentation when true.
  - `level` (number) — starting indentation level (used with `indent`).
  - `keyorder` (array of strings) — encode these object keys first, in
    this order, before any remaining keys (encoded in arbitrary order).
  - `exception` (function) — custom handler for unsupported values or
    reference cycles; see [Custom encoding](#custom-encoding-with-tojson)
    below.
  - `buffer` / `bufferlen` — an existing array/length to append encoded
    fragments into, for incremental encoding across multiple calls (when
    given, `encode` returns `true` and leaves the fragments in `buffer`
    instead of returning a string).
- Returns: the encoded JSON string (or `true`, if `state.buffer` was
  passed in). Raises a Lua error (via `error`) if encoding fails and no
  `exception` handler recovers it.

```lua
local json = require("dkjson")

local ok = { name = "widget", price = 9.99, tags = { "new", "sale" } }
print(json.encode(ok))
--> {"name":"widget","price":9.99,"tags":["new","sale"]}

-- Pretty-printed, with a preferred key order:
print(json.encode(ok, { indent = true, keyorder = { "name", "price", "tags" } }))
```

An empty Lua table `{}` is ambiguous — it encodes as a JSON array `[]` by
default. To force it to encode as an empty object `{}` instead, give it
an `__jsontype = "object"` metatable field:

```lua
local empty = setmetatable({}, { __jsontype = "object" })
print(json.encode(empty)) --> {}
```

### `json.decode(str, pos, nullval, objectmeta, arraymeta)`

Parses a JSON string into a Lua value.

- `str` (string) — the JSON text to parse.
- `pos` (number, optional) — 1-based byte offset to start parsing from
  (defaults to `1`). Useful for parsing multiple JSON values back-to-back
  from the same string.
- `nullval` (optional) — the Lua value to substitute for JSON `null`
  (defaults to Lua `nil`; pass `json.null` to keep nulls distinguishable).
- `objectmeta`, `arraymeta` (tables, optional) — metatables applied to
  decoded objects/arrays respectively. Default to tables with a
  `__jsontype` field of `"object"` or `"array"`, which lets you tell an
  empty JSON object `{}` apart from an empty JSON array `[]` even though
  both decode to an empty Lua table.
- Returns: `value, pos` on success (`pos` is the index just past the
  parsed value), or `nil, errpos, errmsg` on failure.

```lua
local json = require("dkjson")

local value, pos, err = json.decode('{"a":1,"b":[2,3]}')
if not value then
    error(err)
end
print(value.a, value.b[1], value.b[2]) --> 1  2  3

-- Distinguish {} from [] after decoding:
local empty_obj = json.decode("{}")
local empty_arr = json.decode("[]")
print(getmetatable(empty_obj).__jsontype) --> object
print(getmetatable(empty_arr).__jsontype) --> array
```

### `json.quotestring(value)`

Escapes and quotes a Lua string as a JSON string literal (used internally
by `json.encode`, exposed for building custom encoders).

- `value` (string)
- Returns: the quoted, escaped string, e.g. `'"a \\"quote\\""'`.

```lua
local json = require("dkjson")
print(json.quotestring('he said "hi"')) --> "he said \"hi\""
```

### `json.addnewline(state)`

Appends a newline and the current indentation to `state.buffer`, if
`state.indent` is set. Intended for use inside a custom `__tojson`
encoder (see below) so custom output matches the surrounding
indentation style.

- `state` (table) — the encoder state passed into a `__tojson` callback.

### `json.encodeexception(reason, value, state, defaultmessage)`

A ready-made exception handler: turns an encoding failure (an
unsupported value type or a reference cycle) into the JSON string
`"<reason>"` instead of raising an error. By default `json.encode` raises
a Lua error on such failures; assign this function to `state.exception`
to make it degrade gracefully instead (or write your own handler with the
same signature).

- Returns: a quoted placeholder string.

```lua
local json = require("dkjson")

local weird = { ok = "value", bad = print } -- functions aren't encodable
local text = json.encode(weird, { exception = json.encodeexception })
print(text) --> {"ok":"value","bad":"<type 'function' is not supported by JSON.>"}
```

### `json.use_lpeg()`

Loads the [LPeg](http://www.inf.puc-rio.br/~roberto/lpeg/) library and
returns a module table with the same API as `json`, but with `decode`
implemented using an LPeg grammar (generally faster for large documents).
Raises an error if `lpeg` cannot be `require`d, which in tlua it cannot:
LPeg is a C module, and tlua loads none. The decoder written in Lua is the
one tlua uses.

- Returns: a table with `.decode`, `.encode`, `.null`, `.quotestring`,
  and other fields mirroring `json`.

```lua
local json = require("dkjson")

local ok, fast = pcall(json.use_lpeg)
if ok then
    json = fast -- use the LPeg-accelerated decoder from now on
end

print(json.decode('{"fast": true}').fast)
```

## Custom encoding with `__tojson`

Any table with a `__tojson` metamethod is responsible for encoding itself:
`json.encode` calls `__tojson(value, state)` and expects back a JSON
fragment string (or `nil, errmsg` on failure).

```lua
local json = require("dkjson")

local Point = {}
Point.__index = Point
Point.__tojson = function(self, state)
    return string.format('{"x":%d,"y":%d}', self.x, self.y)
end

local p = setmetatable({ x = 1, y = 2 }, Point)
print(json.encode({ point = p })) --> {"point":{"x":1,"y":2}}
```

## Example: round-tripping a config file

```lua
local json = require("dkjson")
local sys = require("sys")

local config = {
    name = "myapp",
    version = 3,
    features = { "logging", "metrics" },
}

sys.writefile("config.json", json.encode(config, { indent = true }))

local text = sys.readfile("config.json")
local loaded, _, err = json.decode(text)
if not loaded then
    error("invalid config: " .. err)
end
print(loaded.name, loaded.version, loaded.features[1])
```
