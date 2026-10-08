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
