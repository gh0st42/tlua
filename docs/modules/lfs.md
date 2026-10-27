# `lfs` — files and directories

`lfs` is [LuaFileSystem](https://lunarmodules.github.io/luafilesystem/) 1.9,
the library most Lua programs use to look at the file system: attributes,
folders, links, times. tlua carries it written in Go, so `require "lfs"`
works with nothing installed, and code written for the C module runs as it
is. It gives the same results as the C build, error messages included.

```lua
local lfs = require "lfs"
```

Every function that can fail returns `nil`, a message and the system's error
number. [sys](sys.md) is the friendlier layer for reading, writing, copying
and listing; `lfs` is what it is built on, and what libraries written for
LuaRocks expect.

## Looking

### `lfs.attributes(path [, name])`

A table about a file or folder, or one field of it by name; `nil, why,
errno` if there is no such file. It follows links.

| Field | |
|---|---|
| `mode` | `"file"`, `"directory"`, `"link"`, `"socket"`, `"named pipe"`, `"char device"`, `"block device"` or `"other"` |
| `size` | bytes |
| `permissions` | as `ls` writes them, `"rwxr-xr-x"` |
| `access`, `modification`, `change` | times, as `os.time()` counts |
| `dev`, `ino`, `nlink`, `uid`, `gid`, `rdev`, `blocks`, `blksize` | the system's own numbers |

```lua
print(lfs.attributes("notes.txt", "size"))         --> 5
print(lfs.attributes("notes.txt", "mode"))         --> file
print(lfs.attributes("src", "mode"))               --> directory
print(lfs.attributes("nope"))
--> nil   cannot obtain information from file 'nope': No such file or directory   2
```

`lfs.attributes(path, table)` fills a table you give instead of making one.

### `lfs.symlinkattributes(path [, name])`

The same for a link itself rather than what it points to. A link's `mode` is
`"link"`, and its `target` is where it points.

### `lfs.dir(path)`

An iterator over the names in a folder, `.` and `..` among them, in no
particular order.

```lua
for name in lfs.dir("src") do
  if name ~= "." and name ~= ".." then print(name) end
end
```

It also returns a directory object, with `dir:next()` and `dir:close()`, for
stopping early. A folder that cannot be opened raises an error.

### `lfs.currentdir()`

The current folder.

## Changing

### `lfs.chdir(path)`

Makes `path` the current folder. `true`, or `nil, why`.

### `lfs.mkdir(path)`, `lfs.rmdir(path)`

Make and remove a folder; `rmdir` needs it empty. `true`, or `nil, why,
errno`:

```lua
print(lfs.mkdir("d"))      --> true
print(lfs.mkdir("d"))      --> nil   File exists   17
```

### `lfs.touch(path [, atime [, mtime]])`

Sets a file's access and modification times, both to now when left out, and
`mtime` to `atime` when only that is given.

### `lfs.link(old, new [, symlink])`

A hard link, or with `symlink` set a symbolic one, from `new` to `old`.

### `lfs.setmode(file, mode)`

`"binary"` or `"text"` for an open file. There is no difference between them
on the systems tlua runs on; it returns `true` and the mode before.

## Locks

### `lfs.lock_dir(path [, seconds_stale])`

Locks a folder by making a file `lockfile.lfs` in it, and returns a lock with
`lock:free()`, or `nil, why` if the folder is locked already. A lock file
older than `seconds_stale` is taken to be left over, and is replaced.

```lua
local lock = assert(lfs.lock_dir("work"))
-- ... nothing else that locks the folder runs now
lock:free()
```

### `lfs.lock(file, mode)`, `lfs.unlock(file)`

Locking a part of an open file is **not supported** in tlua: both return
`nil` and say so. Use `lock_dir`.

## What differs from the C module

Only the two functions above. `lfs._VERSION` is `"LuaFileSystem 1.9.0"`.
