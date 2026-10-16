# Lua standard library

A tour of Lua 5.1's standard library: the functions there without a
`require`, and the library tables `string`, `table`, `math`, `os`, `io`
and `coroutine`. For the details see the
[Lua 5.1 Reference Manual](https://www.lua.org/manual/5.1/manual.html#5)
(§5). [lua-language.md](lua-language.md) tours the language itself.

## Basic functions

| Function | What it does |
|---|---|
| `print(...)` | writes its arguments to stdout, tab-separated, with a newline |
| `type(v)` | the type's name as a string: `"nil"`, `"boolean"`, `"number"`, `"string"`, `"table"`, `"function"`, ... |
| `tostring(v)` / `tonumber(v [, base])` | convert to and from a string |
| `pairs(t)` / `ipairs(t)` | iterate over all keys (any order) / the array part `1..n`, in order |
| `next(t, key)` | the iterator `pairs` is built on |
| `select("#", ...)` / `select(n, ...)` | how many arguments / the arguments from the `n`th on |
| `unpack(t [, i [, j]])` | a table's elements `t[i]..t[j]` as separate values |
| `pcall(f, ...)` / `xpcall(f, handler)` | call `f`, catching errors: `ok, results...` (see [error handling](lua-language.md#error-handling)); `xpcall` passes no arguments in 5.1 |
| `error(msg [, level])` / `assert(v [, msg])` | raise an error / raise one when `v` is false or nil |
| `setmetatable(t, mt)` / `getmetatable(t)` | set or get a table's metatable |
| `rawget` / `rawset` / `rawequal` | get, set and compare without metamethods |
| `require(name)` | load a module once, by `package.path` and tlua's own modules |
| `loadstring(s [, name])` / `loadfile(path)` / `dofile(path)` | compile a string or a file into a function (or run a file) |
| `setfenv(f, t)` / `getfenv(f)` | set or get the globals a function sees |

```lua
print(type(42), tostring(42), tonumber("42"), tonumber("ff", 16)) --> number  42  42  255

for i, v in ipairs({ "a", "b" }) do print(i, v) end
for k, v in pairs({ x = 1, y = 2 }) do print(k, v) end

local f = assert(loadstring("return 1 + 1"))
print(f()) --> 2
```

## `string`

Strings cannot be changed; `s:method(...)` is `string.method(s, ...)`.
Positions count from 1, and negative ones from the end. Strings are bytes:
`#"é"` is 2.

| Function | What it does |
|---|---|
| `#s` / `string.len(s)` | the length in bytes |
| `string.sub(s, i [, j])` | the part from `i` to `j`, both included (`j` defaults to the end) |
| `string.upper(s)` / `string.lower(s)` | change case |
| `string.rep(s, n)` | `s` repeated `n` times |
| `string.reverse(s)` | backwards |
| `string.byte(s [, i [, j]])` / `string.char(...)` | characters to byte values and back |
| `string.format(fmt, ...)` | `printf`-style formatting (`%d`, `%s`, `%5.2f`, `%x`, `%q`, ...) |
| `string.find(s, pattern [, init [, plain]])` | where a pattern matches: `start, end [, captures...]` |
| `string.match(s, pattern [, init])` | the first match's captures (the whole match without any) |
| `string.gmatch(s, pattern)` | an iterator over all matches, for a `for` loop |
| `string.gsub(s, pattern, repl [, n])` | replace matches; `repl` is a string, a table or a function |

```lua
print(("Hello"):upper())              --> HELLO
print(string.format("%d-%s", 5, "x")) --> 5-x
print(("a,b,,c"):find(","))           --> 2  2

for word in ("one two three"):gmatch("%a+") do print(word) end

local clean = ("  hi  "):gsub("^%s+", ""):gsub("%s+$", "")
```

### Patterns

Lua's patterns are not full regular expressions, but cover most everyday
needs. Classes: `%a` letters, `%d` digits, `%s` white space, `%w` letters
and digits, `%p` punctuation, `%u` and `%l` upper and lower case (the
capital letter, as in `%D`, is the opposite), `.` any character, `%z` the
zero byte. Repetition: `*` (0 or more, longest), `+` (1 or more), `-` (0
or more, shortest), `?` (0 or 1). Anchors: `^` start, `$` end. Parentheses
capture; `%b()` matches balanced brackets.

```lua
local year, month, day = ("2024-01-15"):match("(%d+)-(%d+)-(%d+)")
```

## `table`

| Function | What it does |
|---|---|
| `table.insert(t, [pos,] value)` | insert at the end, or at `pos`, moving the rest up |
| `table.remove(t [, pos])` | remove and return the element at `pos` (the last by default) |
| `table.concat(t [, sep [, i [, j]]])` | join the elements into a string |
| `table.sort(t [, less])` | sort in place; `less(a, b)` says whether `a` comes before `b` |
| `table.maxn(t)` | the largest positive number key, holes and all |

```lua
local t = { 3, 1, 2 }
table.insert(t, 4)           -- {3, 1, 2, 4}
table.sort(t)                -- {1, 2, 3, 4}
print(table.concat(t, ", ")) --> 1, 2, 3, 4
table.remove(t, 1)           -- {2, 3, 4}
```

## `math`

| Field or function | What it does |
|---|---|
| `math.pi`, `math.huge` | π, and infinity |
| `math.floor(x)` / `math.ceil(x)` | round down / up to a whole number |
| `math.abs(x)` / `math.max(...)` / `math.min(...)` | absolute value, largest, smallest |
| `math.sqrt(x)`, `math.exp(x)`, `math.log(x)`, `math.log10(x)` | roots and logarithms (`x ^ (1/3)` for other roots) |
| `math.sin`, `math.cos`, `math.tan`, `math.atan2(y, x)`, ... | trigonometry, in radians; `math.rad` and `math.deg` convert |
| `math.fmod(x, y)` / `math.modf(x)` | remainder rounding towards zero / whole and fractional parts |
| `math.random([m [, n]])` / `math.randomseed(x)` | a random number in `[0,1)`, `[1,m]` or `[m,n]` |

```lua
print(math.floor(3.7), math.ceil(3.2)) --> 3  4
math.randomseed(os.time())
print(math.random(1, 6)) -- a dice roll
```

## `os`

| Function | What it does |
|---|---|
| `os.time([table])` | now (or a date given as a table) in seconds since 1970 |
| `os.date([format [, time]])` | a time as text, or as a table with `"*t"` |
| `os.clock()` | the processor time the program has used, in seconds |
| `os.difftime(t2, t1)` | the seconds between two times |
| `os.getenv(name)` | an environment variable |
| `os.execute(command)` | run a shell command; gives its exit status (see also [`sys.execute`](sys.md#sysexecutecommand)) |
| `os.exit([code])` | end the program |
| `os.remove(path)` / `os.rename(old, new)` | delete or rename a file (see also [`sys`](sys.md)) |
| `os.tmpname()` | a name for a temporary file |

```lua
print(os.date("%Y-%m-%d %H:%M:%S", os.time()))
```

## `io`

[`sys`](sys.md) wraps the common file operations with simpler error
handling; use `io` itself to read line by line, or to keep a file open.

| Function | What it does |
|---|---|
| `io.open(path, mode)` | open a file (`"r"`, `"w"`, `"a"`, `"rb"`, ...): a file, or `nil, err` |
| `io.write(...)` / `io.read(...)` | write to stdout / read from stdin |
| `io.lines(path)` | an iterator over a file's lines, for a `for` loop |
| `io.popen(command, mode)` | run a command and read its output (or write its input) |
| `file:read(...)` | read: a number of bytes, `"*l"` a line, `"*a"` the rest, `"*n"` a number |
| `file:write(...)` / `file:close()` / `file:seek(...)` | write, close, move |

```lua
for line in io.lines("data.txt") do
    print(line)
end

local f = assert(io.open("out.txt", "w"))
f:write("hello\n")
f:close()
```

## `coroutine`

| Function | What it does |
|---|---|
| `coroutine.create(f)` | make a coroutine of a function (suspended, a `thread` value) |
| `coroutine.resume(co, ...)` | run it until it yields or returns: `ok, values...` |
| `coroutine.yield(...)` | suspend the running coroutine, handing values to `resume` |
| `coroutine.status(co)` | `"suspended"`, `"running"`, `"normal"` or `"dead"` |
| `coroutine.wrap(f)` | like `create`, as a function that resumes it and raises its errors |
| `coroutine.running()` | the running coroutine, or nil in the main program |

```lua
local co = coroutine.create(function(a)
    print("start", a)
    local b = coroutine.yield(a + 1)
    print("resumed with", b)
end)

print(coroutine.resume(co, 10))
-- prints "start  10" from inside the coroutine, then "true  11" from resume
print(coroutine.resume(co, 99))
-- prints "resumed with  99", then "true"
```

## Text in UTF-8

Lua 5.1 has no `utf8` library; strings are bytes. To go over the
characters of UTF-8 text, match one character at a time:

```lua
for ch in ("héllo"):gmatch("[%z\1-\127\194-\244][\128-\191]*") do
    print(ch)
end
-- the number of characters, rather than bytes
local n = select(2, ("héllo"):gsub("[^\128-\191]", ""))
print(n) --> 5
```
