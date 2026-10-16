# `inspect` — human-readable table dumps

[inspect.lua](http://github.com/kikito/inspect.lua) turns any Lua value
(especially nested tables) into a readable, Lua-literal-like string —
useful for debugging, logging, and quick `print()`-style introspection.
It handles cycles, metatables, and non-serializable values gracefully.

```lua
local inspect = require("inspect")
```

## Fields

### `inspect._VERSION`, `inspect._URL`, `inspect._DESCRIPTION`, `inspect._LICENSE`

Metadata strings about the module itself.

### `inspect.KEY` / `inspect.METATABLE`

Sentinel values used as path markers when using the `process` option (see
[`inspect.inspect`](#inspectinspectvalue-options) below) — they appear in
the `path` argument passed to your `process` callback to indicate that the
item being processed is a table key or a metatable, respectively.

## Functions

### `inspect(value, options)` / `inspect.inspect(value, options)`

The module table itself is callable and is an alias for
`inspect.inspect`.

Converts `value` into a readable multi-line string representation.

- `value` — any Lua value.
- `options` (table, optional):
  - `depth` (number) — maximum table nesting depth to expand; deeper
    tables are shown as `{...}`. Defaults to unlimited.
  - `newline` (string) — line separator used between entries. Defaults to
    `"\n"` (pass `""` or `" "` for a single-line dump).
  - `indent` (string) — indentation string used per nesting level.
    Defaults to `"  "` (two spaces).
  - `process` (function) — a `function(item, path)` called for every
    value and key as the table is walked (depth-first); its return value
    replaces `item` in the output, and returning `nil` for a key removes
    that key/value pair entirely. `path` is an array describing where
    `item` was found, using [`inspect.KEY`](#inspectkey--inspectmetatable)
    / [`inspect.METATABLE`](#inspectkey--inspectmetatable) markers for
    key/metatable positions.
- Returns: the formatted string.

```lua
local inspect = require("inspect")

print(inspect({ 1, 2, 3, name = "widget", nested = { ok = true } }))
--[[
{ 1, 2, 3,
  name = "widget",
  nested = {
    ok = true
  }
}
]]
```

### Limiting depth

```lua
local inspect = require("inspect")

local deep = { a = { b = { c = { d = 1 } } } }
print(inspect(deep, { depth = 2 }))
--[[
{
  a = {
    b = {...}
  }
}
]]
```

### Compact, single-line output

```lua
local inspect = require("inspect")

print(inspect({ 1, 2, foo = "bar" }, { newline = "", indent = "" }))
--> { 1, 2, foo = "bar" }
```

### Redacting values with `process`

```lua
local inspect = require("inspect")

local user = { name = "Ada", password = "hunter2", roles = { "admin" } }

local redacted = inspect(user, {
    process = function(item, path)
        if path[#path] == "password" then
            return "***"
        end
        return item
    end,
})
print(redacted)
--[[
{
  name = "Ada",
  password = "***",
  roles = { "admin" }
}
]]
```

### Cycles and repeated references

Tables that reference themselves (directly or indirectly) are printed
with a numbered `<id>` tag instead of recursing forever, and non-plain
values (functions, threads, userdata) print as `<type id>`:

```lua
local inspect = require("inspect")

local t = {}
t.self = t
print(inspect(t))
--[[
<1>{
  self = <table 1>
}
]]

print(inspect(print))
--> <function 1>
```

## Example: quick debug logging

```lua
local inspect = require("inspect")

local function debug_dump(label, value)
    io.stderr:write(label, ": ", inspect(value), "\n")
end

debug_dump("response", { status = 200, body = { ok = true, items = {1, 2, 3} } })
```
