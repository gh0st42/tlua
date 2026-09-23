# tlua

A standalone Lua interpreter written in pure Go. It embeds
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
       tlua fuse [-o output] <main.lua | directory | archive.zip>

  -e stat    execute string 'stat'
  -l name    require library 'name' into a global of the same name
             (use -l glob=name to pick the global)
  -p path    prepend 'path' to package.path (a directory, or a ?-pattern)
  -i         enter interactive mode after executing 'script'
  -v         print version information
  -E         ignore environment variables (LUA_PATH, LUA_INIT)
  -h         print this help
  --         stop handling options
  -          execute stdin as a file and stop handling options
```

```sh
tlua examples/hello.lua world     # run a script with arguments
tlua -e 'print(("%d"):format(42))'
tlua -p ./vendor -l inspect app.lua
echo 'print(1+1)' | tlua          # program on stdin
tlua -i                           # REPL
```

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

Tests sit beside what they cover: unit tests in each `internal` package, and
end-to-end tests in [cmd/tlua](cmd/tlua/) that build the binary and drive it as
a user would.
