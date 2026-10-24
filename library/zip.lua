---@meta zip
--- Zip archives, `require "zip"`: read from a file or a string, and
--- written to a file or a string. Files can be encrypted with AES-256, in the
--- form 7-Zip reads. It also reads as LuaZip does.

local zip = {}

---@class zip.Info
---@field name string
---@field size integer
---@field compressed integer
---@field modified integer as os.time() counts
---@field method "deflate"|"store"
---@field encrypted boolean needs a password to read
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

--- A file's contents; nil and why ("a password is needed", "wrong password",
--- "the data has been altered, or is damaged ..."). The password is for this
--- call; without one the archive's is used.
---@param name string
---@param password? string
---@return string? data
---@return string? why
function Archive:read(name, password) end

--- The password the encrypted files are read with from now on; nil takes it away.
---@param password string?
function Archive:setpassword(password) end

--- LuaZip's way: a file to read("*a"), read("*l"), read(n), lines() and close().
---@param name string
---@param password? string
---@return table? file
---@return string? why
function Archive:open(name, password) end

function Archive:close() end

---@class zip.Writer
local Writer = {}

--- Adds a file, compressed unless store says not to, dated now unless
--- modified (as os.time() counts) says otherwise. It is encrypted with the
--- archive's password, with its own if password is a string, or not at all if
--- password is false.
---@param name string
---@param data string
---@param opts? {store?: boolean, modified?: integer, password?: string|false}
function Writer:add(name, data, opts) end

--- Writes the archive: true for a file, the bytes for one made in
--- memory; nil and why.
---@return (true|string)? result
---@return string? why
function Writer:close() end

---@class zip.ReadOptions
---@field password? string for the encrypted files in the archive

---@class zip.WriteOptions
---@field password? string encrypts every file added (AES-256)

--- The archive in a file, read out of a fused program or bundle first.
---@param path string
---@param opts? zip.ReadOptions
---@return zip.Archive? archive
---@return string? why
function zip.open(path, opts) end

--- The archive in a string.
---@param data string
---@param opts? zip.ReadOptions
---@return zip.Archive? archive
---@return string? why
function zip.load(data, opts) end

--- An archive to add files to, written to path when closed, or in memory.
--- The options may be given alone for one in memory.
---@overload fun(opts: zip.WriteOptions): zip.Writer
---@param path? string
---@param opts? zip.WriteOptions
---@return zip.Writer
function zip.create(path, opts) end

return zip
