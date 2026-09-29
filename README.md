# tlua

A standalone Lua interpreter written in pure Go, with a DOS-style editor, a
fantasy console for writing games, and LÖVE-style standalone executables. It
embeds [gopher-lua](https://github.com/yuin/gopher-lua) (Lua 5.1) and draws
with [Ebitengine](https://ebitengine.org), neither of which needs cgo, so
`CGO_ENABLED=0 go build` still produces a single static binary you can drop on
any machine.

It runs ordinary `.lua` files from disk, including the modules they `require()`.

## Build

```sh
make build                  # -> bin/tlua
make static                 # stripped, CGO_ENABLED=0
make test                   # go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/tlua ./cmd/tlua
```

The module is self-contained: `go.mod` plus a one-line `go.work` (`use .`) that
keeps it out of any workspace defined in a parent directory. Copy the folder
anywhere — the `go.work` is harmless where no parent workspace exists, and can
be deleted there.

## Use

```
usage: tlua [options] [script [args]]
       tlua edit [file...]
       tlua play [script | directory] [args]
       tlua fuse [-o output] [-play] <main.lua | directory | archive.zip>

Options:
  -e stat    execute string 'stat'
  -l name    require library 'name' into a global of the same name
             (use -l glob=name to pick the global)
  -p path    prepend 'path' to package.path (a directory, or a ?-pattern)
  -i         enter interactive mode after executing 'script'
  -v         print version information
  -E         ignore environment variables (TLUA_*, LUA_PATH, LUA_INIT)
  -h         print this help
  --         stop handling options
  -          execute stdin as a file and stop handling options

Environment:
  TLUA_INCLUDE   site-wide library directories, listed like PATH; each is
                 searched as <dir>/?.lua and <dir>/?/init.lua
  TLUA_PATH      package.path patterns, as LUA_PATH but tlua-only
  TLUA_INIT      chunk to run at startup ("@file" runs a file)
  TLUA_LSP       language server the editor formats, completes and hovers
                 with, or "off"; by default it looks for one on PATH
  TLUA_LOVE      love2d binary for the editor's LOVE run mode
```

```sh
tlua examples/hello.lua world     # run a script with arguments
tlua -e 'print(("%d"):format(42))'
tlua -p ./vendor -l inspect app.lua
echo 'print(1+1)' | tlua          # program on stdin
tlua -i                           # REPL
tlua edit app.lua                 # full-screen editor
tlua play examples/pico/snake.lua # a game, in a window
```

## The console

`tlua play` opens a window and runs a Lua program against a fantasy console in
the spirit of PICO-8 and Picotron: a 480x270 screen, 64 colours, sprites, four
players' worth of input, and a program built out of `_update()` and `_draw()`.

```lua
function _draw()
	cls(1)
	print("hello", 8, 8, 7)
	circfill(64, 64, 10 + sin(t()) * 4, 12)
end
```

```sh
tlua play game.lua              # one file
tlua play .                     # the main.lua in this directory
tlua play -scale 3 game.lua     # three screen pixels to a console pixel
tlua play -fullscreen game.lua
```

A level can be drawn in a map editor and loaded whole — Tiled's own format,
with its tilesets and their artwork:

```lua
usemap(loadmap("level1"))
map()
if fget(mget(x, y), SOLID) then end   -- what a tile is comes with the artwork
```

A sheet of sprites is a surface with a cell size on it — 8x8, 16x16, whatever
the artwork was drawn at — and its sprites are drawn by number:

```lua
local tiles = loadpng("tiles", 16, 16)
spr(tiles, 3, 100, 50)        -- sprite 3
usesheet(tiles)
spr(3, 100, 50)               -- the same, Picotron's own spelling
```

Sprites can also be written out as text in the program itself, so a game is one
file with nothing beside it:

```lua
local coin = sprite[[
	.aaa.
	a9a9a
	a9a9a
	.aaa.
]]
```

[docs/pico.md](docs/pico.md) is the whole API, and
[examples/pico](examples/pico) has twelve programs written against it — from
`hello.lua` up to a snake game, a platformer with a tile map, and a painting
program. [library/pico.lua](library/pico.lua) declares it for
lua-language-server, so an editor completes these names and shows what they
take; the `.luarc.json` at the root points at it.

Drawing happens on an indexed framebuffer, one byte a pixel, which is scaled to
the window by a whole number with nearest-neighbour, so pixels stay square.
`vid()` switches resolution with Picotron's own numbering — `vid(0)` 480x270,
`vid(3)` 240x135, `vid(4)` 160x90, and the 320x180 and 240x180 it lists as
planned — plus `vid(13)`, the 320x200 a VGA card called mode 13h.

The palette is whatever a program asks for, up to 256 colours. It starts with
64: 0-15 are PICO-8's palette exactly and 16-31 its extended one, while 32-63
are tlua's own ramps rather than Picotron's, which are not published as a list.
`palette("vga")` swaps in the 256 an IBM VGA card came up in, `palette(path)`
reads a `.gpl` file from any pixel art tool, and `palette(i, 0xRRGGBB)` changes
one colour — which changes every pixel already drawn in it, the cheapest fade
there is.

While a program runs, the window keeps `alt-enter` and `F11` for fullscreen,
`ctrl-D` for a frame rate counter and `ctrl-Q` for closing it; the program is
not shown those keys.

A finished game ships as one executable with nothing beside it:

```sh
tlua fuse -play -o mygame mygame/   # a directory with main.lua in it
./mygame                            # opens its own window
```

Its artwork, sounds and data go in with it. `loadpng`, `fetch`, `sfx`, `music`
and `require` all read what was attached before they read the disk, so the same
game runs from a directory while it is being written and from one file once it
is finished.

Resources are asked for by name rather than by path: `sfx("jump")` finds
`sfx/jump.wav` or `assets/sfx/jump.wav`, `loadpng("player")` finds
`gfx/player.png`, and a full path still means exactly itself. WAV and Ogg
Vorbis play, on eight channels with separate looping music.

In the editor, the Run menu's "Run with" setting decides what `F5` does:
`tlua`, a console window, or love2d.

## The editor

`tlua edit [file...]` opens a full-screen editor in the manner of QBasic and
Turbo Pascal: a menu bar across the top, a blue desktop, several files open at
once, and F5 to run.

```
 File  Edit  Search  Run  Window  Help
 1:»main.lua  2:lib.lua
╔═════════════════════════════════ »main.lua ══════════════════════════════════╗
║local lib = require("lib")                                                    ║
║print("hello", lib.note)                                                      ║
║                                                                              ║
╚══════════════════════════════════════════════════════════════════════════════╝
 F1 Help  F2 Save  F3 Open  F5 Run  F7 Find  F9 Check  F10 Menu           L1 C1
```

### Keys

| Key | |
| --- | --- |
| `F1` | the key list |
| `F2`, `Ctrl-S` | save |
| `F3`, `Ctrl-O` | open a file |
| `F4` | show or hide the output pane |
| `F5` | run the primary file |
| `F6` | next buffer |
| `Ctrl-B` | comment or uncomment the line, or the selection |
| `Ctrl-F` | find |
| `F7`, `F8` | find next, find previous |
| `Ctrl-R` | replace |
| `F9` | check the current buffer's syntax without running it |
| `Alt-F7`, `Alt-F8` | the previous and next problem the server reported |
| `Alt-F9` | list the problems in this buffer |
| `Ctrl-Space` | complete what is being typed (language server) |
| `Ctrl-P` | the parameters of the call being typed (language server) |
| `F11`, `Ctrl-F1` | what is under the cursor (language server) |
| `F12` | format the buffer through the language server |
| `F10` | the menu bar |
| `Ctrl-N` | new file |
| `Ctrl-G` | go to line |
| `Ctrl-C` | stop the running program (it does not leave the editor) |
| `Alt-F2` | list what this buffer defines, and jump to one |
| `Alt-1`…`Alt-9` | pick a buffer |
| `Alt-F3` | close the current buffer |
| `Alt-X` | leave |
| `Alt-F/E/R/W/H` | open a menu directly |

In the text itself: `Ctrl-Z` undo, `Ctrl-Y` redo, `Ctrl-Q` copy, `Ctrl-X` cut,
`Ctrl-V` paste, `Ctrl-L` select all. The clipboard is shared by every buffer.
`PgUp` and `PgDn` page through a file; `Ctrl-F` and `Ctrl-B`, which tview's text
widget would otherwise use for paging, search and comment instead.

`Ctrl-B` comments the line the cursor is on, or every line the selection
touches. It uncomments only when every line with code on it is already
commented, so one key works both ways and a half-commented block comments
first. The `--` goes in at the shallowest indentation in the block, lining up
with the code rather than with the left margin, blank lines are left alone, and
the whole thing is one undo step.

The mouse works throughout: click a menu title to drop it down and an entry to
run it, click a name on the buffer bar to switch to it, click or drag in the
text to move the cursor and select, scroll the wheel in the text and in the
output pane. Clicking away from an open menu closes it, and a dialog keeps the
mouse to itself while it is up. Your terminal's own selection still works with
`Shift` held, as usual when a program reads the mouse.

### Finding and replacing

`Ctrl-F` opens Find, `Ctrl-R` opens Replace, and both are in the Search menu
along with `F7` and `F8` for the next and previous match. Type the term and
press `Enter`: in these dialogs `Enter` does what the first button does, so a
search never needs the `Tab` key. A search that reaches the end of the file
carries on from the top and the status line says it wrapped; a match is left
selected, so `Ctrl-X` or typing over it does the obvious thing.

Find starts from what is selected, if anything, otherwise from the last term,
and remembers whether case mattered. Replace offers *Replace*, which changes the
match at the cursor and moves to the next, and *Replace all*. Replacements go
through the text widget one at a time, so `Ctrl-Z` walks back through them
individually rather than losing the file in one step.

### What a language server adds

If a Lua language server is on PATH, the editor starts it in the background and
uses it for three things. It looks for `lua-language-server`, `emmylua_ls` and
`lua-lsp`, in that order; `TLUA_LSP` names a different one (with arguments, if it
needs them) and `TLUA_LSP=off` does without.

**Formatting.** Each buffer is formatted as it is saved, so what lands on disk is
what the screen shows. `F12` formats without saving. *Edit › Format on save*
turns the pass on and off when a server's idea of tidy is not yours, and
*Edit › Language server...* says what was found.

**Completion.** Typing `.` or `:` asks the server what comes next, and
`Ctrl-Space` asks anywhere. The answers appear in a panel under the cursor, with
the kind of each beside it and the server's description of the selected one
along the bottom:

```
local x = pri
             ╔═════ 5 completions ═════╗
             ║print  function          ║
             ║pairs  function          ║
             ║table_insert  function   ║
             ║for_loop  snippet        ║
             ║replaced_word  variable  ║
             ║function print(...)      ║
             ╚═════════════════════════╝
```

`Enter` or a click inserts, `Esc` dismisses, and typing carries on regardless:
a letter goes into the text and the list comes back narrowed to the word as it
now stands, backspace likewise. A completion is a session that lasts as long as
the word does, so the list follows the whole of it and goes when the word ends.

An item that brings its own edit says exactly what to replace; otherwise the
word being typed makes way for it, and a dotted or colon prefix is left alone so
`table.ins` completes to `table.insert` rather than doubling the prefix.
Snippets go in with their placeholders reduced to defaults, since this editor has
no tab stops to walk.

Only `.` and `:` open the list unasked. Servers may request far more —
lua-language-server asks to be consulted after a space, a tab, `(`, `=`, `-` and
a dozen others — and a list that opens on every space is an obstacle rather than
help, so the server's list is narrowed to member access. A buffer that has never
been saved completes too: the server is told about it under its name in the
working directory, and the file need not exist.

**Parameters.** Typing the `(` that opens a call, or a `,` inside one, shows what
the call takes on the line above it, with the argument being typed picked out:

```
local s = string.format(
                         function string.format(s: string|number, ...any)
```

The hint takes no focus, so typing carries on underneath it; it follows the
cursor from one argument to the next, goes when the call is closed or `Esc` is
pressed, and `Ctrl-P` asks for it at any point inside a call. As with
completions the server's list of trigger characters is narrowed —
lua-language-server asks to be consulted after a space as well — since a hint
that appears on every space is noise. *Edit › Parameters on status line* moves
it to the status bar for anyone who would rather it never covered a line of code.

**Problems.** What the server finds wrong is marked where it is, said on the
status line, and listed on request. Errors are drawn white on maroon, warnings
black on brown, and notes are underlined; a name on the buffer bar gains a `!`
when its file has errors or a `?` when it only has warnings:

```
 1:»main.lua!
╔══════════════════════ »main.lua! ══════════════════════╗
║local x = 1                                             ║
║end                                                     ║
╚════════════════════════════════════════════════════════╝
 Error: Unexpected symbol `end`.                    L2 C1
```

The status line shows the message for the line the cursor is on, so walking
through a file tells you what is wrong with each line as you reach it. A message
about the last thing you did wins for as long as the cursor has not moved, and
after that the line hands itself back to the diagnostics. `Alt-F7` and `Alt-F8`
step from one problem to the next, wrapping round, as they did in the Borland
editors; `Alt-F9` lists them all with their lines and severities, and `Enter`
jumps.

The text goes to the server 300 ms after you stop typing, which is what prompts
a fresh report, so marks appear as you work rather than only on save. F9 still
checks the syntax on its own, without a server.

**Hover help.** `F11`, or `Ctrl-F1` where the terminal sends it — which is where
Turbo Pascal put help on the word under the cursor — shows what the server knows
about it. The answer arrives as markdown and is reduced to text: fences,
emphasis, links and rules go, and what they wrapped stays. Plain `F1` is still
the key list.

Nothing waits on the server while you work. Every request a keystroke provokes
— completions after a `.`, the parameter hint as the cursor moves — is made off
the editing loop, and its answer is dropped if the cursor has moved on before it
arrives. Asking again while an answer is in flight coalesces rather than
queueing, so holding a key cannot pile requests up, and typing stays as quick
with a slow server as with none: two tests pin that, driving the editor against
a server told to think for 400 ms per answer.

Open buffers are handed to the server as soon as it starts, so it reads the
workspace before the first question rather than after it; a server that has only
just started answers "nothing" until it has, and a request made by hand asks
twice before believing it.

None of this is required: with no server the editor behaves exactly as it did
before, each of the three keys says so plainly, and a server that fails or goes
quiet costs a formatting pass, never a save — the write goes ahead with the text
as it stands and the reason is kept for the Language server dialog. A formatting
pass arrives as a single edit, so one `Ctrl-Z` takes it back, and the cursor
keeps its line. A server that answers `null`, as one still reading a workspace
does, simply has nothing to offer yet.

The client itself is [internal/lsp](internal/lsp/): the initialize handshake,
document synchronisation, formatting, completion and hover — the part of the
protocol these three keys need. Its tests run against a fake server in
`testdata`, and against whatever real server is installed when there is one.

### The function list

`Alt-F2` lists what a buffer defines — QBasic's F2 and RHIDE's Alt-F2 — with the
file itself as the first row and the end of the file as the last, so the list
navigates the file as well as its functions. `Enter`, or a click, jumps.

```
╔══════════════════ Functions in main.lua ══════════════════╗
║    1  main.lua                                            ║
║    2  M.global_one                                        ║
║    3    local_callback                                    ║
║    5  local_one                                           ║
║    6  (end of file)                                       ║
║ global  local  nested                                     ║
╚═══════════════════════════════════════════════════════════╝
```

`Enter` puts the definition at the top of the window with the cursor at the
start of its line, so what you jumped to is followed by its body rather than
sitting at the bottom of the screen.

A row is just the line and the name — no "function" or "local" spelled out — and
the colour says how it is reached, with the legend along the bottom:

| | |
| --- | --- |
| black | global: `function f()`, `f = function()`, `M.f`, `M:f` |
| navy | local: `local function f()`, `local f = function()` |
| maroon | nested: defined inside another function, whatever its name |

Nested definitions are indented by how deep they sit, so a callback assigned
inside a function (`local_callback = function() ... end`) reads as belonging to
the function above it. Global is the plain colour because it is the common case
and black is the most legible on a grey panel.

The list comes from the parse tree, so it tells a local apart from a global,
finds `M.f = function()` and functions inside a module table, and gets nesting
right. While the buffer does not parse — which is most of the time while typing
— it falls back to scanning lines, taking indentation as the clue to nesting, so
the list never goes away mid-edit. It opens on the function the cursor is inside.

### Colours and highlighting

The interface uses the sixteen EGA colours and nothing else: a blue desktop,
light grey bars with red hotkey letters, a green selection bar, and program
output on black. Two tests keep it that way: one walks the theme and fails if a
colour outside the palette creeps in, and one opens every menu and dialog and
fails if any cell inside it still carries the blue of the desktop. That second
test exists because tview builds each widget's text style from the theme's
background colour and prints without preserving what is underneath, so setting
only a foreground colour leaves a blue stripe behind every label on a grey
panel; a panel is styled through its full `tcell.Style`, and form items also
need their own background set.

Lua is highlighted in the same palette, in the manner of the Borland IDEs:

| | |
| --- | --- |
| keywords | white, bold |
| standard library names | light green |
| strings, long and short | yellow |
| numbers | light cyan |
| comments | dark grey |
| everything else | light grey |

`tview` draws a text area in a single style, so the colouring is painted over
what the widget has just drawn: the glyphs stay exactly where tview put them
and only the colour of each cell changes, which leaves the cursor, the
selection and undo untouched.

The work is proportional to the visible window rather than to the file. The
scanner writes into a slice the highlighter owns and works in bytes, so a
redraw allocates nothing of its own; the one thing that needs the lines above —
whether a long string or a `--[[ ]]` comment is still open — is remembered per
line and extended only as far down as you have scrolled. An edit keeps every
line state above the line that changed, so a keystroke at the bottom of a long
file re-reads one line instead of all of them: that took a redraw there from
2.5 ms to 86 µs, and what is left is tview's own drawing.

## Standalone executables (fuse)

A tlua binary with a program appended to it stops being an interpreter and
becomes that program, the way a `.love` file appended to the LÖVE runtime
becomes a game. All command line arguments go to the program; tlua parses none
of them.

```sh
tlua fuse -o myapp main.lua      # embed one file
tlua fuse -o myapp mygame/       # zip the folder (needs mygame/main.lua)
tlua fuse -o myapp game.zip      # embed an existing archive
cat tlua game.zip > myapp && chmod +x myapp    # the LÖVE way, also works
./myapp --any --args you --like
```

Build for another platform by fusing onto a cross-compiled interpreter:

```sh
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/tlua.exe ./cmd/tlua
tlua fuse --base bin/tlua.exe -o myapp.exe mygame/
```

`tlua fuse` also accepts an already fused binary as `--base`; the old program is
stripped first, so rebuilding an app does not stack payloads.

### Inside a fused archive

The entry point is `main.lua` at the archive root (a single wrapping folder,
as produced by `zip -r app.zip mygame`, is unwrapped automatically).

- `require("lib.util")` searches the archive first (`lib/util.lua`, then
  `lib/util/init.lua`), then `package.path`, which starts with the directory
  the executable sits in — so an app can still be extended by files shipped
  beside it.
- `dofile` and `loadfile` read from the archive when it holds the path, so an
  app behaves the same from any working directory, and fall through to disk
  otherwise.
- `require("embed")` returns the archive itself:
  `embed.read(name)` → contents (or `nil, err`), `embed.exists(name)`,
  `embed.files()` → list of names, `embed.kind` → `"zip"` or `"lua"`.

Data files stay read-only; `io.open` is untouched and still talks to the real
file system, which is where an app should put anything it writes.

### macOS note

Appending to a Mach-O binary invalidates its code signature. The result runs
fine locally on Apple Silicon, but `codesign -v` reports strict validation
failure, so an app meant for distribution should be re-signed as part of a
proper `.app` bundle rather than shipped as a bare fused binary.

## How modules are found

`require("foo.bar")` looks for `foo/bar.lua` and `foo/bar/init.lua` along
`package.path`, which tlua assembles as, highest priority first:

1. the directory of the script being run — so a script's neighbours resolve
   no matter which directory you invoke it from;
2. every `-p` path, in command line order;
3. `TLUA_PATH`;
4. every directory in `TLUA_INCLUDE`, in list order;
5. `LUA_PATH`;
6. the built-in default `./?.lua;/usr/local/share/lua/5.1/?.lua;/usr/local/share/lua/5.1/?/init.lua`.

Repeated patterns are dropped, keeping the first occurrence, so overlapping
sources only cost one lookup each.

### Environment

| Variable | Meaning |
| --- | --- |
| `TLUA_INCLUDE` | Site-wide library directories, listed the way `PATH` is (`:` on Unix, `;` on Windows). Each entry is searched as `<dir>/?.lua` and `<dir>/?/init.lua`; an entry containing `?` is taken as a literal pattern instead. |
| `TLUA_PATH` | `package.path` patterns in Lua's own notation, for full control. `;;` expands to the built-in default. Takes precedence over `TLUA_INCLUDE` and `LUA_PATH`. |
| `TLUA_INIT` | A chunk to run before anything else; `@file` runs a file. Takes precedence over `LUA_INIT`. |
| `TLUA_LSP` | The language server `tlua edit` formats with, as a command with any arguments. `off` uses none. Unset, the editor looks for one on PATH. |
| `TLUA_LOVE` | The love2d binary the editor runs in LÖVE mode, as a command with any arguments. Unset, it looks for `love` on PATH and in `/Applications/love.app` on macOS. |
| `LUA_PATH`, `LUA_INIT` | The standard Lua variables, honoured as the reference interpreter does. |

So a machine declares its shared Lua libraries once:

```sh
export TLUA_INCLUDE=/usr/local/share/tlua:/opt/company/lua
tlua app.lua        # require("company.logging") just works
```

`-E` ignores all of them, `TLUA_*` included, which is the way to get a
reproducible run. Fused executables are ordinary tlua processes and honour
these variables too, though their own embedded archive always wins.

`package.cpath` is empty and stays that way. Loading C modules (`.so`/`.dll`)
is the one thing a cgo-free interpreter cannot do; pure-Lua libraries work
unchanged.

## Compatibility notes

The language and standard library are Lua 5.1 as gopher-lua implements them:
`string`, `table`, `math`, `io`, `os`, `coroutine`, `debug` (partial),
metatables, varargs, `pcall`/`error`, and `#!` lines are all supported. The
`goto` statement, integer division, and bitwise operators from 5.3+ are not.

The `arg` table follows the reference interpreter: `arg[0]` is the script,
`arg[1..n]` its arguments (also passed to the chunk as `...`), and negative
indices walk back over tlua's own options.

Ctrl-C aborts the running chunk rather than killing the process, so the REPL
survives an interrupted loop; a script interrupted this way exits with
status 1.

## Layout

```
cmd/tlua/          the command: option parsing, usage, order of operations
internal/interp/   the interpreter: state, search path, arg, REPL, fused apps
internal/payload/  the format of a program attached to a binary, and its archive
internal/fuse/     the fuse subcommand: packing a program onto the interpreter
internal/editor/   the full-screen editor
internal/lsp/      a small Language Server Protocol client
internal/pico/     the fantasy console: framebuffer, palette, drawing, font
internal/picolua/  that console's Lua API
internal/game/     the window: Ebitengine, the frame loop, input
docs/pico.md       the console API, written out
library/pico.lua   the same API declared for lua-language-server
examples/          hello.lua, app/ to fuse, and pico/ for the console
bin/               build output (git-ignored)
```

| File | Purpose |
| --- | --- |
| [cmd/tlua/main.go](cmd/tlua/main.go) | command line parsing and the order of operations |
| [internal/interp/interp.go](internal/interp/interp.go) | the Lua state: search path, `arg`, interrupts, error reporting |
| [internal/interp/repl.go](internal/interp/repl.go) | interactive mode, including multi-line continuation |
| [internal/interp/fused.go](internal/interp/fused.go) | running an attached program, and exposing its archive to Lua |
| [internal/payload/payload.go](internal/payload/payload.go) | detecting, reading and writing the attached-program format |
| [internal/payload/archive.go](internal/payload/archive.go) | the read-only file system inside a fused zip |
| [internal/fuse/fuse.go](internal/fuse/fuse.go) | the `fuse` subcommand that builds standalone executables |
| [internal/version/version.go](internal/version/version.go) | the version number, in one place |
| [internal/editor/editor.go](internal/editor/editor.go) | the editor: layout, bars, global keys |
| [internal/editor/buffer.go](internal/editor/buffer.go) | open files and what the editor does to them |
| [internal/editor/menu.go](internal/editor/menu.go) | the menu bar and its dropdowns |
| [internal/editor/run.go](internal/editor/run.go) | F5, F9, and the output pane |
| [internal/editor/dialogs.go](internal/editor/dialogs.go) | the file browser and the other dialogs |
| [internal/editor/theme.go](internal/editor/theme.go) | the EGA palette |
| [internal/editor/highlight.go](internal/editor/highlight.go) | the Lua scanner behind the colouring |
| [internal/editor/codearea.go](internal/editor/codearea.go) | the text widget that paints those colours |
| [internal/editor/outline.go](internal/editor/outline.go) | finding the functions a buffer defines |
| [internal/editor/mouse.go](internal/editor/mouse.go) | clicks on the bars, and keeping menus and dialogs modal |
| [internal/editor/find.go](internal/editor/find.go) | searching, replacing, and their dialogs |
| [internal/editor/format.go](internal/editor/format.go) | starting a language server and formatting with it |
| [internal/lsp/client.go](internal/lsp/client.go) | the Language Server Protocol client |
| [internal/lsp/edits.go](internal/lsp/edits.go) | applying a server's edits to a document |
| [internal/lsp/text.go](internal/lsp/text.go) | markdown and snippets reduced to plain text |
| [internal/editor/complete.go](internal/editor/complete.go) | the completion panel and the hover box |
| [internal/editor/signature.go](internal/editor/signature.go) | the parameter hint |
| [internal/editor/diagnostics.go](internal/editor/diagnostics.go) | what the server finds wrong, and moving between it |
| [internal/editor/runmode.go](internal/editor/runmode.go) | what F5 starts: tlua, a console window, or love2d |
| [internal/pico/pico.go](internal/pico/pico.go) | the console's screen and drawing state |
| [internal/pico/draw.go](internal/pico/draw.go) | lines, rectangles, ellipses, triangles, sprites |
| [internal/pico/font.go](internal/pico/font.go) | the 3x5 font, written out as pictures |
| [internal/pico/palette.go](internal/pico/palette.go) | the 64 colours |
| [internal/pico/input.go](internal/pico/input.go) | buttons, keys and the mouse, held and repeating |
| [internal/picolua/picolua.go](internal/picolua/picolua.go) | the Lua runtime: callbacks, arguments, the window request |
| [internal/picolua/draw.go](internal/picolua/draw.go) | the drawing calls as Lua sees them |
| [internal/picolua/stdlib.go](internal/picolua/stdlib.go) | `flr`, `rnd`, `add`, `all`, `split` and the rest |
| [internal/game/game.go](internal/game/game.go) | the `play` subcommand: loading and starting a program |
| [internal/game/app.go](internal/game/app.go) | the frame loop, and fitting the picture to the window |

Tests sit beside what they cover: unit tests in each `internal` package, and
end-to-end tests in [cmd/tlua](cmd/tlua/) that build the binary and drive it as
a user would.
