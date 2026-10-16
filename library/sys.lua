---@meta sys
--- Files, folders and a few system helpers, `require "sys"`, from turbolua.
--- Each function gives nil and why when it fails.

local sys = {}

--- A whole file, as bytes.
---@param path string
---@return string? data
---@return string? why
function sys.readfile(path) end

--- Writes data to path, making the folders it is in first.
---@param path string
---@param data? string
---@return true? ok
---@return string? why
function sys.writefile(path, data) end

--- Whether a file or folder is there.
---@param path string
---@return boolean
function sys.exists(path) end

---@param path string
---@return boolean? isdir
---@return string? why
function sys.isdir(path) end

--- A folder's entries, without . and .., sorted.
---@param path string
---@return string[]? names
---@return string? why
function sys.listdir(path) end

--- Copies a file, or a folder with everything in it.
---@param src string
---@param dst string
---@return true? ok
---@return string? why
function sys.copy(src, dst) end

--- Renames, or copies and deletes when a rename cannot (across disks).
---@param src string
---@param dst string
---@return true? ok
---@return string? why
function sys.move(src, dst) end

--- Makes a folder, and the folders it is in unless recursive is false.
--- A folder already there is fine.
---@param path string
---@param recursive? boolean
---@return true? ok
---@return string? why
function sys.mkdir(path, recursive) end

---@return string? dir
---@return string? why
function sys.cwd() end

--- Waits, to the millisecond.
---@param seconds? number 1 when left out
---@return boolean
function sys.sleep(seconds) end

--- os.execute, with the command checked.
---@param command string
function sys.execute(command) end

--- Writes prompt, if given, and reads a line from stdin.
---@param prompt? string
---@return string? line nil at the end of input
function sys.input(prompt) end

return sys
