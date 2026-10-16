# `term` — ANSI terminal helper library

`term` provides a Turbo Pascal / QBasic–style console API (`textcolor`,
`gotoxy`, `clrscr`, ...) implemented on top of ANSI escape sequences, plus
`term.print`/`term.println` helpers that support inline `{color}` tags. It
targets ANSI-capable terminals; `term.raw` is not supported on Windows.
It comes from turbolua; see
[examples/console/term_demo.lua](../../examples/console/term_demo.lua).

```lua
local term = require("term")
```

## Constants

### `term.ESC`

The ASCII escape character (`"\27"`) used to build ANSI sequences.

### `term.color`

A table mapping EGA/QBasic color names to their palette index (`0`-`15`),
for use with [`term.textcolor`](#termtextcolorcolor-out),
[`term.textbackground`](#termtextbackgroundcolor-out), and
[`term.colorize`](#termcolorizefg-bg-out).

| Name(s)                | Index |
|-------------------------|-------|
| `black`                 | 0     |
| `blue`                  | 1     |
| `green`                 | 2     |
| `cyan`                  | 3     |
| `red`                   | 4     |
| `magenta`               | 5     |
| `brown`                 | 6     |
| `lightgray`, `lightgrey`| 7     |
| `darkgray`, `gray`, `grey` | 8  |
| `lightblue`             | 9     |
| `lightgreen`            | 10    |
| `lightcyan`             | 11    |
| `lightred`              | 12    |
| `lightmagenta`          | 13    |
| `yellow`                | 14    |
| `white`                 | 15    |

```lua
term.textcolor(term.color.lightgreen)
```

## Functions

Most drawing functions accept an optional trailing `out` argument — a file
handle (defaults to `io.stdout`) to write the escape sequence to instead of
standard output.

### `term.raw(enabled)`

Enables or disables raw (uncooked) terminal input mode via `stty`, useful
for reading key presses without waiting for Enter. Raw mode also turns off
XON/XOFF flow control, so `Ctrl+Q` and `Ctrl+S` arrive as keys instead of
pausing the terminal. Not supported on Windows.

- `enabled` (boolean) — `true` to enable raw mode, `false` to restore the
  previous mode.
- Returns: `true` on success, or `nil, err` on failure.

```lua
local ok, err = term.raw(true)
if not ok then
    error(err)
end

local key = term.readkey()
print("you pressed: " .. key)

term.raw(false)
```

### `term.size()`

Detects the terminal size, preferring the `COLUMNS`/`LINES` environment
variables and falling back to `stty size`.

- Returns: `cols, rows` (numbers) on success, or `nil, err` on failure.

```lua
local cols, rows = term.size()
if cols then
    print(string.format("terminal is %dx%d", cols, rows))
end
```

### `term.readkey(timeout)`

Reads a single character from standard input. Combine with
`term.raw(true)` to read key presses immediately, without line buffering.

- `timeout` (number, optional) — seconds to wait for a byte. Omitted (the
  default): blocks forever, like a plain `io.read(1)`. Given: waits at
  most `timeout` seconds (only meaningful while raw mode is enabled;
  rounded to tenths of a second, and not supported on Windows, where it
  falls back to blocking). Pass a small timeout (e.g. `0.05`) to "peek"
  for a following byte — the standard way to tell a bare Escape keypress
  apart from the start of an arrow-key escape sequence (`ESC [ A`, etc.)
  without hanging forever if no more bytes follow.
- Returns: a one-character string, or `nil` if nothing arrived (at EOF,
  or the timeout elapsed with no key pressed).

```lua
term.raw(true)
local c = term.readkey()
if c == term.ESC then
    local c2 = term.readkey(0.05) -- don't hang if it was a lone Escape
    if c2 == "[" then
        local c3 = term.readkey(0.05)
        if c3 == "A" then print("up arrow") end
    else
        print("escape key")
    end
end
term.raw(false)
```

```lua
term.raw(true)
print("press any key...")
local key = term.readkey()
term.raw(false)
print("got: " .. key)
```

### `term.sleep(seconds)`

Pauses execution: [`sys.sleep`](sys.md#syssleepseconds) under another
name.

- `seconds` (number, optional) — defaults to `1` when omitted or negative.
- Returns: `true`.

```lua
term.sleep(0.5)
```

### `term.cursor(visible, out)`

Shows or hides the terminal cursor.

- `visible` (boolean) — `true` to show, `false` to hide.
- `out` (file, optional)
- Returns: the output stream written to, or `nil, err` if `visible` is not
  a boolean.

```lua
term.cursor(false) -- hide cursor while drawing
-- ... draw a frame ...
term.cursor(true)
```

### `term.clrscr(color, out)`

Clears the screen and moves the cursor to the home position (`1,1`).

- `color` (number `0`-`15`, string name from `term.color`, or `nil`,
  optional) — when given, sets the background color (via
  [`term.textbackground`](#termtextbackgroundcolor-out)) before clearing,
  so the whole screen is filled with that color. The background remains
  active afterwards, so subsequent output keeps that color until changed.
- `out` (file, optional)
- Returns: the output stream written to, or `nil, err` if `color` is
  invalid.

```lua
term.clrscr()
print("fresh screen")

-- Clear the screen to a blue background
term.clrscr("blue")
print("blue screen")
term.normvideo() -- restore default colors when done
```

### `term.gotoxy(x, y, out)`

Moves the cursor to column `x`, row `y` (both 1-based).

- `x` (number) — column, clamped to a positive integer (defaults to `1`).
- `y` (number) — row, clamped to a positive integer (defaults to `1`).
- `out` (file, optional)

```lua
term.clrscr()
term.gotoxy(10, 5)
io.write("Hello at (10, 5)")
```

### `term.textcolor(color, out)`

Sets the foreground (text) color.

- `color` (number `0`-`15`, or string name from `term.color`)
- `out` (file, optional)
- Returns: the output stream written to, or `nil, err` for an invalid
  color.

```lua
term.textcolor("yellow")
print("warning!")
term.textcolor(term.color.lightgray)
```

### `term.textbackground(color, out)`

Sets the background color.

- `color` (number `0`-`15`, or string name from `term.color`)
- `out` (file, optional)
- Returns: the output stream written to, or `nil, err` for an invalid
  color.

```lua
term.textbackground("blue")
print("on a blue background")
```

### `term.colorize(fg, bg, out)`

Sets foreground and background color together in a single escape sequence.

- `fg` (number `0`-`15`, or string name)
- `bg` (number `0`-`15`, or string name)
- `out` (file, optional)
- Returns: the output stream written to, or `nil, err` for an invalid
  color.

```lua
term.colorize("white", "red")
print(" ERROR ")
term.normvideo()
```

### `term.normvideo(out)`

Resets text attributes and colors to the default (light gray on black).

- `out` (file, optional)

```lua
term.textcolor("red")
print("red text")
term.normvideo()
print("back to normal")
```

### `term.highvideo(out)`

Enables bold/bright (high-intensity) text.

- `out` (file, optional)

```lua
term.highvideo()
print("bright text")
term.normvideo()
```

### `term.lowvideo(out)`

Enables dim (low-intensity) text.

- `out` (file, optional)

```lua
term.lowvideo()
print("dim text")
term.normvideo()
```

### `term.flush(out)`

Flushes the output stream (defaults to `io.stdout`).

- `out` (file, optional)
- Returns: the result of the stream's `flush` method.

```lua
io.write("loading")
for i = 1, 3 do
    io.write(".")
    term.flush()
    term.sleep(0.3)
end
print()
```

### `term.print(str, out)`

Prints `str` to `out` (defaults to `io.stdout`), interpreting inline
`{...}` color tags (see [Inline color tags](#inline-color-tags) below). No
trailing newline is written. Colors are automatically reset to normal video
at the end of the string if any tag was used, so styling never leaks into
later output.

- `str` (string, or any value convertible with `tostring`)
- `out` (file, optional)
- Returns: the output stream written to.

```lua
term.print("{red}Error:{reset} something went wrong\n")
term.print("plain text, no tags needed\n")
```

### `term.println(str, out)`

Identical to [`term.print`](#termprintstr-out), but appends a trailing
newline after the text (and after any final color reset).

- `str` (string, or any value convertible with `tostring`)
- `out` (file, optional)
- Returns: the output stream written to.

```lua
term.println("{green}OK{reset} — build succeeded")
term.println("{yellow,blue} warning banner {reset}")
```

## Inline color tags

`term.print` and `term.println` support inline styling using `{...}` tags
embedded directly in the string, so a single line can mix multiple colors
without manual calls to `textcolor`/`textbackground`/`colorize`.

| Tag              | Effect                                                        |
|------------------|----------------------------------------------------------------|
| `{name}`         | Set foreground color by name (any key in `term.color`)        |
| `{index}`        | Set foreground color by palette index (`0`-`15`)               |
| `{fg:name}` / `{fg:index}` | Explicit foreground color (same as `{name}`/`{index}`) |
| `{bg:name}` / `{bg:index}` | Set background color                                  |
| `{fg,bg}`        | Set foreground and background together, e.g. `{yellow,blue}`  |
| `{reset}` / `{normal}` | Reset to normal video (equivalent to `term.normvideo`)  |
| `{high}` / `{bright}`  | High-intensity/bold text (equivalent to `term.highvideo`) |
| `{low}` / `{dim}`      | Low-intensity/dim text (equivalent to `term.lowvideo`)  |
| `{{` / `}}`      | Literal `{` / `}` character                                    |

Tag names and color names are case-insensitive. A tag that doesn't match a
known color or keyword (e.g. a typo) is left in the output unchanged as
literal text (`{like this}`) instead of raising an error.

```lua
term.println("{lightgreen}Success:{reset} 12 tests passed")
term.println("{4}numeric index also works{reset}")
term.println("{bg:blue}{white} inverted banner {reset}")
term.println("{black,yellow} WARNING {reset} disk space low")
term.println("Use {{curly braces}} literally by doubling them")
```

### Escaping dynamic content

Because any `{...}` in the string is parsed as a tag, be careful when
building a string for `term.print`/`term.println` out of dynamic or
user-supplied data (e.g. a key press, a filename, network input) — a stray
`{` or `}` in that data will be parsed as part of a tag instead of printed
literally, which can swallow a following `{reset}` and leave colors
"leaking" into later output. Double any braces in dynamic content before
concatenating it into a tag string:

```lua
local untrusted = get_user_input() -- may contain "{" or "}"
local safe = untrusted:gsub("[{}]", "%1%1")
term.println("You typed: {lightgreen}" .. safe .. "{reset}")
```

## Example: simple animated status line

```lua
local term = require("term")

term.cursor(false)
term.clrscr()

for i = 1, 5 do
    term.gotoxy(1, 1)
    term.colorize("black", "lightgreen")
    io.write(string.format(" Step %d/5 ", i))
    term.normvideo()
    term.flush()
    term.sleep(0.5)
end

print()
term.cursor(true)
```
