# gopher-lua, as tlua carries it

This is [gopher-lua](https://github.com/yuin/gopher-lua) v1.1.2, unchanged
except for what is listed here. `go.mod` points at it with a `replace`. The
upstream tests and `cmd/glua` are left out, and so are the dependencies only
those needed.

Once upstream has the same fixes, delete this directory and the `replace`.

## compile.go: the generic `for` explist

`compileGenericForStmt` registered the three hidden locals (generator, state,
control) before compiling the expressions that fill them. While they were
being compiled, their registers therefore looked like live locals, and two
things went wrong:

- A call inside an operand, as in `for l in (f(x) .. "\n"):gmatch(p)`, was
  taken to need no register of its own, so the next operand overwrote its
  result and the loop died with a Go nil pointer dereference.
- The explist was adjusted to the number of loop *variables* rather than to
  three values, so `for k in next, t do` never set the control value and
  started from whatever an earlier statement had left in that register; the
  loop could silently run zero times.

The fix compiles the explist first, adjusted to exactly three values, and
only then activates the hidden locals, as the reference compiler does.

## state.go, _state.go: closing upvalues after an error

When an error was raised with no handler set, `raiseError` and `Error` closed
the open upvalues of every frame on the stack, including the frames below the
`pcall` that caught it, which go on running. A closure over one of their
locals was cut loose from it: after any caught error, the closure saw a
private copy, and the two drifted apart.

```lua
local n = 0
local function inc() n = n + 1 end
pcall(error, "x")
inc()
print(n)        --> 0 before the fix, 1 after
```

The fix closes upvalues in `PCall` instead, only from the protected call's
base upwards and after any handler has run, as `luaD_pcall` does. Both
`state.go` and the `_state.go` it is generated from carry it.

## stringlib.go: string.format

`string.format` handed its format and arguments straight to Go's
`fmt.Sprintf`. Most conversions came out right, but `%q` used Go's escapes
(`\x01`, `\a`), which neither gopher-lua nor Lua can read back, so a value
written with `%q` and loaded again came back different; `%u` was not known;
and a missing or mistyped argument produced `%!d(MISSING)` in the output
instead of an error.

It is now Lua 5.1's `str_format`: each conversion is parsed as C reads it
and checked against its argument, `%q` escapes as Lua 5.1 does, `%c`, `%s`
widths and precisions count bytes, `%g` has C's default precision, and
infinities print as `inf`. Go's `fmt` still renders the numbers. `%s` keeps
gopher-lua's leniency of taking any value, using `__tostring` as Lua 5.2 does.

## utils.go, baselib.go, mathlib.go, value.go: numbers in strings

Strings were read as numbers in two ways, neither of them Lua's. `tonumber`
parsed a string without a `.` as an integer, so `tonumber("1e5")` was nil
(and `"2e-3"`, `"1E5"`, and `tonumber("1e5", 10)`). Arithmetic on strings
and the compiler went through Go's `strconv.ParseInt` with base 0, which
reads Go's literals: `"010" + 0` was 8, and `"0b101"` and `"1_000"` were
numbers.

Both now use `str2number`, which is Lua 5.1's `luaO_str2d`: a decimal with
an optional fraction and exponent, or a hexadecimal integer, with a sign
and C's whitespace around it. Unlike C's `strtod`, it reads no `inf`, `nan`
or hexadecimal fraction, as Lua 5.2 decided. `tonumber(s, base)` for
another base reads an integer in it as `strtoul` does, `0x` allowed in base
16, and a base outside 2 to 36 is an error.

`math.huge` was the largest finite number rather than infinity, and
infinities and NaN printed as Go writes them (`+Inf`); they are `inf`,
`-inf` and `nan` now, as in Lua.

## compile.go: assignment to more than one variable

`compileAssignStmtRight` compiled each value straight into the register of
the local it was for, which saves a move when there is one target. With
more than one, a local was overwritten before the expressions after it had
read it: `a, b = b, a` gave `2 2`, and in `a, b, c = math.floor(a/2), ...`
the function `math.floor`, loaded into `a`'s register to be called, was
then divided ("cannot perform div operation between function and number").
A field's value was likewise read from a local's own register rather than
a copy, so `t.x, a = a, t.x` stored `a` after it had changed.

With more than one target every value now goes to a temporary first, and
the assignments follow, as Lua does; one target keeps the shortcut.


## stringlib.go: string.match with no match

`string.match` pushed nil when nothing matched but said it returned no
values, so `select("#", s:match(p))` was 0 rather than 1, and
`tonumber(s:match(p))` was an error ("value expected") rather than nil, as
was any call that took the result as an argument. It returns its nil now.
`string.byte` of an empty range and a finished `gmatch` iterator still
return nothing, as Lua 5.1's do.

## coroutinelib.go: a coroutine's context

A coroutine was given a child of the context of the state that made it,
for good. tlua runs each chunk under a context of its own and ends it with
the chunk, so a coroutine made by a program's main chunk and resumed later,
from a GUI timer or event (which run under the event loop's context),
died at its first instruction with "context canceled". `coroutine.resume`
now runs the coroutine under the context of the state resuming it.

## value.go: numbers as strings

`tostring`, `..`, `print` and `io.write` wrote a whole number as an integer
and any other in Go's shortest exact form, so `0.1 + 0.2` was
`0.30000000000000004`, `2^53` was `9007199254740992` and `-0` was `0`. Lua
5.1 writes every number with `%.14g` (LUAI_NUMFFORMAT): `0.3`,
`9.007199254741e+15`, `-0`. Now so does tlua, and JSON or other text a
Lua library writes from numbers comes out as it does under Lua 5.1.

## pm/pm.go: frontier patterns

`%f[set]`, Lua 5.1's frontier pattern, was not understood: `%f` matched the
letter f, so `("hello"):find("hello%f[%W]")` found nothing and
`%f[%a]%a+%f[%A]` could not pick out a word. It is now a pattern of its own:
it matches where the character before is not in the set and the one after is,
the start and the end of the string counting as a zero byte, and it is an
error without a `[` after it.

## tablelib.go: table.concat of a long table

`table.concat` pushed every item and every separator onto the Lua stack
before joining them, so a table of half a million items raised "registry
overflow" (the registry holds a million values). Lua 5.1 joins in a buffer
and has no such limit; so does this now.

## state.go, _state.go: the depth of calls

A state's call stack had gopher-lua's default of 256 frames, so a recursion a
few hundred calls deep raised "stack overflow", where Lua 5.1 allows about
20000 (LUAI_MAXCALLS). tlua now asks for 20000 frames, in a stack that grows
as it is used (`MinimizeStackMemory`) so that each coroutine stays small.
That stack's `IsFull` was never true, so the overflow came as a Go panic
worded "lua callstack overflow"; it now says, as Lua does, "stack overflow".

## state.go, _state.go, coroutinelib.go: yielding across a call through Go

A coroutine yields by leaving the virtual machine's loop, which only works
when that loop is the coroutine's own. From inside a call that went through Go
(a `pcall`, a metamethod, a `table.sort` comparison, a `gsub` callback, a
for-in iterator) the loop that was left was a nested one, and the Go caller
carried on as if the call had returned, ending in a nil pointer dereference
reported as "runtime error: invalid memory address". The state now counts
such calls, and a yield from inside one raises Lua 5.1's own error,
"attempt to yield across metamethod/C-call boundary". `pcall` catches it, as
in 5.1, and puts the count right after any error.

## parse/lexer.go: how syntax errors read

A syntax error read `prog.lua line:7(column:57) near '/':   syntax error`, and
`prog.lua at EOF:   syntax error` at the end of the input, with a newline at
the end: nothing that looks for `file:line:` could find the line. It is now
worded as Lua 5.1 does, `prog.lua:7: unexpected symbol near '/'`, with the
last line given for an error at the end of the input (`near '<eof>'`), and
the lexer's messages as Lua says them (`unfinished string`, `unfinished long
string`, `malformed number`). The parser still cannot tell which of Lua's
specific messages fits (`'end' expected`, `'=' expected`), so the others are
all "unexpected symbol". `parse.Error` keeps its fields, and has `EOFLine`.

## cerror.go, iolib.go, oslib.go, baselib.go, loadlib.go, auxlib.go: how file errors read

Errors from the file system came out as Go words them: `io.open` gave `open
/nonexistent/x: no such file or directory` with an errno of 1 whatever had
gone wrong, `os.remove` and `os.rename` gave no errno, `io.lines` of a missing
file returned nothing at all, `loadfile` said `can not open file: x`, `dofile`
and the script on the command line raised the Go error, and `require` listed
`stat x: no such file or directory` for each path. They now read as Lua 5.1
and C give them: `io.open`, `os.remove` and `os.rename` return `nil,
"x: No such file or directory", 2` (the real errno), `io.lines` raises `bad
argument #1 to 'lines' (x: No such file or directory)`, `io.input` and
`io.output` raise `cannot open file 'x' (...)`, `loadfile`, `dofile` and a
script's file say `cannot open x: No such file or directory`, and `require`
says `module 'x' not found:` with `no file 'path'` for each path tried.
Argument errors put the function's name in quotes and say `?` for one with
no name (`bad argument #1 to 'rep' (...)`), as Lua does.

## utils.go, oslib.go: os.date

`os.date` knew a few of strftime's conversions and wrote the others as they
stood, and several of those it knew were wrong: `%a` gave `mon`, `%x` gave
`22/13/20` and `%c` a format of its own. It is now strftime in the C locale:
all the conversions of C99 (`%j`, `%U`, `%W`, `%V`, `%G`, `%e`, `%D`, `%T`,
`%R`, `%r`, `%C`, `%u`, `%h`, `%n`, `%t`, `%s`, ...), the E and O modifiers
ignored, and an unknown conversion left as written. `os.date("*t")` has the
real `yday` and `isdst`.

## iolib.go: read("*n")

`file:read("*n")` used Go's fmt scanner, which does not read `0x10`, and
returned its parse error and an errno of 1 instead of nil. It reads as
`fscanf("%lf")` does: leading space skipped, the longest text that can be a
number taken (decimal or hexadecimal, with a sign and an exponent when digits
follow them), the rest left in the file, and nil when there is no number.
