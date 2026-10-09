---@meta zip
--- Zip archives, `require "zip"`: read from a file or a string, and
--- written to a file or a string. It also reads as LuaZip does.

local zip = {}

---@class zip.Info
---@field name string
---@field size integer
---@field compressed integer
---@field modified integer as os.time() counts
---@field method "deflate"|"store"
---@field filename string LuaZip's name for name
---@field uncompressed_size integer LuaZip's name for size
---@field compressed_size integer LuaZip's name for compressed

---@class zip.Archive
local Archive = {}

--- The names of the files in it, in its order; folders are not listed.
---@return string[]
function Archive:list() end

--- LuaZip's way: an iterator over each file's info.
---@return fun(): zip.Info?
function Archive:files() end

---@param name string
---@return zip.Info?
function Archive:info(name) end

---@param name string
---@return boolean
function Archive:exists(name) end

--- A file's contents; nil and why.
---@param name string
---@return string? data
---@return string? why
function Archive:read(name) end

--- LuaZip's way: a file to read("*a"), read("*l"), read(n), lines() and close().
---@param name string
---@return table? file
---@return string? why
function Archive:open(name) end

function Archive:close() end

---@class zip.Writer
local Writer = {}

--- Adds a file, compressed unless store says not to, dated now unless
--- modified (as os.time() counts) says otherwise.
---@param name string
---@param data string
---@param opts? {store?: boolean, modified?: integer}
function Writer:add(name, data, opts) end

--- Writes the archive: true for a file, the bytes for one made in
--- memory; nil and why.
---@return (true|string)? result
---@return string? why
function Writer:close() end

--- The archive in a file, read out of a fused program or bundle first.
---@param path string
---@return zip.Archive? archive
---@return string? why
function zip.open(path) end

--- The archive in a string.
---@param data string
---@return zip.Archive? archive
---@return string? why
function zip.load(data) end

--- An archive to add files to, written to path when closed, or in memory.
---@param path? string
---@return zip.Writer
function zip.create(path) end

return zip
