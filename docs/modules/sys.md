# `sys` — filesystem and system utilities

`sys` provides file I/O, filesystem manipulation (copy/move/mkdir/listdir),
process helpers (`execute`, `sleep`), and basic console input. It comes
from turbolua and is built on
[LuaFileSystem](https://lunarmodules.github.io/luafilesystem/) (`lfs`),
which tlua has built in. See
[examples/console/sys_demo.lua](../../examples/console/sys_demo.lua).

```lua
local sys = require("sys")
```

## Functions

### `sys.readfile(path)`

Reads an entire file as binary data.

- `path` (string) — path to the file.
- Returns: `data` (string) on success, or `nil, err` on failure.

```lua
local data, err = sys.readfile("config.txt")
if not data then
    error(err)
end
print(data)
```

### `sys.writefile(path, data)`

Writes `data` to `path`, creating any missing parent directories first.

- `path` (string) — destination file path.
- `data` (string, optional) — content to write; defaults to an empty string.
- Returns: `true` on success, or `nil, err` on failure.

```lua
local ok, err = sys.writefile("out/log.txt", "hello world\n")
if not ok then
    error(err)
end
```

### `sys.exists(path)`

Checks whether a path (file or directory) exists.

- `path` (string)
- Returns: `boolean`

```lua
if sys.exists("data.json") then
    print("found it")
end
```

### `sys.isdir(path)`

Checks whether a path is a directory.

- `path` (string)
- Returns: `true`/`false` on success, or `nil, err` if `path` is invalid.

```lua
if sys.isdir("assets") then
    print("assets is a directory")
end
```

### `sys.listdir(path)`

Lists the entries of a directory (excluding `.` and `..`), sorted
alphabetically.

- `path` (string) — must be an existing directory.
- Returns: array of names on success, or `nil, err` on failure.

```lua
local entries, err = sys.listdir(".")
if not entries then
    error(err)
end
for _, name in ipairs(entries) do
    print(name)
end
```

### `sys.copy(src, dst)`

Copies a file or, recursively, a directory tree. Missing parent directories
of `dst` are created automatically.

- `src` (string) — source path.
- `dst` (string) — destination path.
- Returns: `true` on success, or `nil, err` on failure.

```lua
local ok, err = sys.copy("assets/icon.png", "build/assets/icon.png")
if not ok then
    error(err)
end

-- Recursively copy a directory
sys.copy("assets", "build/assets")
```

### `sys.move(src, dst)`

Moves/renames a file or directory. Tries an atomic `os.rename` first, and
falls back to copy-then-delete when that fails (e.g. across filesystems).

- `src` (string) — source path.
- `dst` (string) — destination path.
- Returns: `true` on success, or `nil, err` on failure.

```lua
local ok, err = sys.move("tmp/report.csv", "archive/report.csv")
if not ok then
    error(err)
end
```

### `sys.mkdir(path, recursive)`

Creates a directory.

- `path` (string) — directory to create.
- `recursive` (boolean, optional) — when not explicitly `false`, missing
  parent directories are created as needed (like `mkdir -p`). Pass `false`
  to require the parent to already exist.
- Returns: `true` on success (also when the directory already exists), or
  `nil, err` on failure.

```lua
local ok, err = sys.mkdir("build/output/logs")
if not ok then
    error(err)
end
```

### `sys.cwd()`

Returns the current working directory.

- Returns: `path` (string) on success, or `nil, err` on failure.

```lua
local dir = sys.cwd()
print("running from " .. dir)
```

### `sys.sleep(seconds)`

Pauses execution, with `socket.sleep`: to the millisecond, without
starting a process. On Windows, which has no `socket.core`, it waits for
`ping` instead.

- `seconds` (number, optional) — defaults to `1` when omitted or negative.
- Returns: `true` (on Windows, whether `ping` succeeded).

```lua
print("waiting...")
sys.sleep(2)
print("done")
```

### `sys.execute(command)`

Runs a shell command, thin wrapper around `os.execute` with argument
validation.

- `command` (string) — non-empty shell command.
- Returns: whatever `os.execute` returns (implementation/version dependent:
  typically `true`/`nil` plus `"exit"`/`"signal"` and a code), or `nil, err`
  if `command` is invalid.

```lua
local ok = sys.execute("echo hi")
```

### `sys.input(prompt)`

Reads a line of text from standard input, optionally printing a prompt
first.

- `prompt` (string, optional) — text to print before reading (no trailing
  newline is added, so include one if needed).
- Returns: the line read (string), or `nil` at EOF.

```lua
local name = sys.input("Enter your name: ")
print("Hello, " .. name .. "!")
```
