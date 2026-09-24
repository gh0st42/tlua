# tlua

A standalone Lua interpreter written in pure Go, with a DOS-style editor and
LÖVE-style standalone executables. It embeds
[gopher-lua](https://github.com/yuin/gopher-lua) (Lua 5.1), so there is no cgo,
no linking against liblua, and `CGO_ENABLED=0 go build` produces a single
static binary you can drop on any machine.

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
       tlua fuse [-o output] <main.lua | directory | archive.zip>

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
```

```sh
tlua examples/hello.lua world     # run a script with arguments
tlua -e 'print(("%d"):format(42))'
tlua -p ./vendor -l inspect app.lua
echo 'print(1+1)' | tlua          # program on stdin
tlua -i                           # REPL
tlua edit app.lua                 # full-screen editor
```

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

### Formatting, through a language server

If a Lua language server is on PATH, the editor starts it in the background and
uses it to format each buffer as it is saved, so what lands on disk is what the
screen shows. It looks for `lua-language-server`, `emmylua_ls` and `lua-lsp`, in
that order; `TLUA_LSP` names a different one (with arguments, if it needs them)
and `TLUA_LSP=off` does without.

`F12` formats the current buffer without saving. *Edit › Format on save*
turns the pass on and off when a server's idea of tidy is not yours, and
*Edit › Language server...* says what was found.

None of this is required: with no server the editor behaves exactly as it did
before, and a server that fails or goes quiet costs you a formatting pass, never
a save — the write goes ahead with the text as it stands and the reason is kept
for the Language server dialog. A formatting pass arrives as a single edit, so
one `Ctrl-Z` takes it back, and the cursor keeps its line.

The client itself is [internal/lsp](internal/lsp/): the initialize handshake,
document synchronisation and `textDocument/formatting`, which is all that
formatting needs. Its tests run against a fake server in `testdata`, and against
whatever real server is installed when there is one.

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
selection and undo untouched. The work is proportional to the visible window,
not to the file: a redraw scans only the lines on screen (about 33 µs for a
screenful, by the benchmark in `highlight_test.go`), and the one thing that
needs the lines above — whether a long string or `--[[ ]]` comment is still
open — is cached per line and extended only as far down as you have scrolled.

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
examples/          hello.lua, and app/ to fuse
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

Tests sit beside what they cover: unit tests in each `internal` package, and
end-to-end tests in [cmd/tlua](cmd/tlua/) that build the binary and drive it as
a user would.
