---@meta lfs
--- LuaFileSystem 1.9, `require "lfs"`, built into tlua in Go. Functions
--- give nil, a message and an error number when they fail.

local lfs = {}

lfs._VERSION = "LuaFileSystem 1.9.0"

---@class lfs.Attributes
---@field dev integer
---@field ino integer
---@field mode "file"|"directory"|"link"|"socket"|"named pipe"|"char device"|"block device"|"other"
---@field nlink integer
---@field uid integer
---@field gid integer
---@field rdev integer
---@field access integer last access, as os.time() counts
---@field modification integer
---@field change integer
---@field size integer
---@field permissions string "rwxr-xr-x"
---@field blocks integer
---@field blksize integer
---@field target? string symlinkattributes: where the link points

--- A file's attributes, or one of them by name.
---@param path string
---@param request_name? string|table a field's name, or a table to fill
---@return lfs.Attributes|any? attributes
---@return string? why
---@return integer? errno
function lfs.attributes(path, request_name) end

--- attributes of a link itself, rather than what it points to.
---@param path string
---@param request_name? string|table
---@return lfs.Attributes|any? attributes
---@return string? why
---@return integer? errno
function lfs.symlinkattributes(path, request_name) end

---@param path string
---@return true? ok
---@return string? why
function lfs.chdir(path) end

---@return string? dir
---@return string? why
function lfs.currentdir() end

---@class lfs.Dir
local Dir = {}
---@return string?
function Dir:next() end
function Dir:close() end

--- An iterator over a folder's entries, . and .. included:
--- `for name in lfs.dir(path) do ... end`.
---@param path string
---@return fun(dir: lfs.Dir): string?
---@return lfs.Dir
function lfs.dir(path) end

--- Makes a hard link, or a symbolic one.
---@param old string
---@param new string
---@param symlink? boolean
---@return true? ok
---@return string? why
function lfs.link(old, new, symlink) end

--- Not supported in tlua: says so.
---@param file file*
---@param mode "r"|"w"
---@return nil
---@return string why
function lfs.lock(file, mode, start, length) end

--- Not supported in tlua: says so.
---@param file file*
---@return nil
---@return string why
function lfs.unlock(file, start, length) end

---@class lfs.Lock
local Lock = {}
function Lock:free() end

--- Locks a folder by making lockfile.lfs in it.
---@param path string
---@param seconds_stale? number
---@return lfs.Lock? lock
---@return string? why
function lfs.lock_dir(path, seconds_stale) end

---@param path string
---@return true? ok
---@return string? why
---@return integer? errno
function lfs.mkdir(path) end

---@param path string
---@return true? ok
---@return string? why
---@return integer? errno
function lfs.rmdir(path) end

--- Text or binary mode; the same on every system tlua runs on.
---@param file file*
---@param mode "binary"|"text"
---@return boolean ok
---@return string previous
function lfs.setmode(file, mode) end

--- Sets a file's access and modification times, to now when left out.
---@param path string
---@param atime? integer
---@param mtime? integer
---@return true? ok
---@return string? why
function lfs.touch(path, atime, mtime) end

return lfs
