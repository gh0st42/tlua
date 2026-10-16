# Lua language basics

tlua runs **Lua 5.1**; the reference manual is
[lua.org/manual/5.1](https://www.lua.org/manual/5.1/). This page is a
quick tour of the language, not a replacement for the manual: enough to
get productive fast. [lua-stdlib.md](lua-stdlib.md) tours the standard
library. Code written for Lua 5.3 or 5.4 may use things 5.1 does not have;
they are listed [at the end](#not-in-lua-51).

## Comments

```lua
-- a single-line comment
--[[
  a multi-line
  comment
]]
```

## Variables and scope

Variables are dynamically typed and **global by default**. Declare them
with `local` unless you mean a global:

```lua
local count = 0    -- local to the current block or file
total = 0          -- global (avoid unless you mean it)
```

Blocks (`do ... end`, function bodies, loop bodies, `if` branches) each
have their own scope for the `local`s declared in them.

## Types

`nil`, `boolean`, `number`, `string`, `table`, `function`, plus `userdata`
and `thread` for host values and coroutines. `type(v)` gives the name as a
string.

Every number is a double-precision float; there is no separate integer
type. Whole numbers are exact up to 2^53. `tostring` and `print` show 14
significant digits:

```lua
print(7 / 2, 2^10, 0.1 + 0.2) --> 3.5  1024  0.3
print(2^53)                   --> 9.007199254741e+15
print(string.format("%d", 2^53)) --> 9007199254740992
```

`nil` and `false` are the only false values: `0` and `""` are true.

## Operators

```lua
-- arithmetic: + - * / % ^ (power); / never truncates
print(7 / 2, math.floor(7 / 2), 7 % 2, 2 ^ 10) --> 3.5  3  1  1024

-- relational: == ~= < > <= >=
-- logical: and, or, not (short-circuiting; and/or give one of their operands)
local name = given_name or "default" -- the "default value" idiom

-- string concatenation: ..
print("a" .. "b" .. 1) --> ab1

-- length: # (a string's bytes, or a table's array part)
print(#"hello", #{1, 2, 3}) --> 5  3
```

## Control structures

```lua
if x > 0 then
    print("positive")
elseif x < 0 then
    print("negative")
else
    print("zero")
end

while count < 10 do count = count + 1 end

repeat count = count - 1 until count == 0 -- runs the body at least once

for i = 1, 10, 2 do print(i) end -- start, stop, step (step defaults to 1)

for index, value in ipairs({ "a", "b", "c" }) do print(index, value) end -- array order
for key, value in pairs({ x = 1, y = 2 }) do print(key, value) end      -- all keys, any order

for i = 1, 100 do
    if i > 5 then break end
end
```

There is no `continue` and no `goto`. Skip the rest of a loop body with an
`if`, or wrap the body in `repeat ... until true` and `break` out of that:

```lua
for i = 1, 3 do
    repeat
        if i == 2 then break end -- acts as "continue"
        print(i)
    until true
end
```

## Functions

Functions are values; they can return several values and take a variable
number of arguments:

```lua
local function add(a, b) return a + b end

local function minmax(...)
    local args = { ... }
    return math.min(unpack(args)), math.max(unpack(args))
end
local lo, hi = minmax(3, 1, 4, 1, 5)

-- select("#", ...) counts the arguments, nils included
local function count(...) return select("#", ...) end
print(count(1, nil, 3)) --> 3

-- closures: a function keeps the locals in scope where it was made
local function counter()
    local n = 0
    return function() n = n + 1; return n end
end
local next_id = counter()
print(next_id(), next_id()) --> 1  2

-- the "default argument" idiom
local function greet(name) name = name or "world"; print("hello, " .. name) end
```

## Tables

The one structured type, used for arrays, dictionaries, records, objects
and modules alike:

```lua
local t = { 10, 20, 30, name = "widget", nested = { ok = true } }
print(t[1], t.name, t.nested.ok) --> 10  widget  true

t[#t + 1] = 40   -- append
t.name = nil     -- remove a key
```

`#t` is only well defined for a table without holes (nil gaps) in its
array part. For anything else, iterate with `pairs` or keep a count.

## Metatables and simple OOP

`setmetatable` and `getmetatable` give a table behaviour. The most common
is `__index`, for both classes and default values:

```lua
local Animal = {}
Animal.__index = Animal

function Animal.new(name)
    return setmetatable({ name = name }, Animal)
end

function Animal:speak() -- ":" is sugar for function Animal.speak(self)
    print(self.name .. " makes a sound")
end

local a = Animal.new("Rex")
a:speak() --> Rex makes a sound
```

Other common metamethods: `__tostring` (what `tostring` and `print`
show), `__add`, `__eq`, `__lt` and the like (operators), `__call` (make a
table callable), `__newindex` (catch assignments, for read-only tables).
In Lua 5.1, `__len` and `__gc` work for userdata only, not tables, and
`__pairs` does not exist.

## Error handling

```lua
local ok, result = pcall(function()
    if not condition then error("something went wrong") end
    return 42
end)
if not ok then
    print("failed: " .. result) -- result is the error message here
else
    print("got: " .. result)
end

assert(type(x) == "number", "x must be a number") -- raises an error when false
```

## Modules

A module is a file that builds a table and returns it:

```lua
-- mymodule.lua
local M = {}

function M.greet(name)
    return "hello, " .. name
end

return M
```

```lua
local mymodule = require("mymodule")
print(mymodule.greet("world"))
```

The README's "How modules are found" says where `require` looks. tlua's
own modules ([sys](sys.md), [term](term.md), [tui](tui.md), ...) are
built in.

## A few idioms worth knowing

```lua
-- multiple assignment, and a swap
local a, b = 1, 2
a, b = b, a

-- a ternary of sorts: only safe when the "true" value is never false or nil
local label = (count == 1) and "item" or "items"

-- ignoring a return value
local _, err = some_function()
```

## Not in Lua 5.1

What code written for later versions may use, and the 5.1 way:

| Later Lua | Lua 5.1 |
|---|---|
| `a // b` (floor division) | `math.floor(a / b)` |
| `&`, `\|`, `~`, `<<`, `>>` (bitwise) | arithmetic, or a bit library in Lua |
| `goto` and labels | `break`, `repeat ... until true`, or a function |
| integers, `math.type`, `math.tointeger` | all numbers are floats |
| `table.unpack`, `table.pack` | `unpack`, and `{ n = select("#", ...), ... }` |
| `load(string)` | `loadstring(string)`; `load` takes a function |
| `utf8` library | patterns such as `"[%z\1-\127\194-\244][\128-\191]*"` |
| `<const>`, `<close>` locals | plain locals |
| `_ENV` | `setfenv` and `getfenv` |
| `rawlen`, `__pairs`, `__len` on tables | `#`, `pairs` without a metamethod |
