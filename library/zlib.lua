---@meta zlib
--- Compression, `require "zlib"`: lua-zlib's API, which LuaRocks code
--- expects, with lzlib's compress and decompress, and gzip and gunzip.
---
--- windowBits says the format: 8 to 15 zlib, -8 to -15 raw deflate, 16
--- more gzip, and to inflate 32 more detects zlib or gzip.

local zlib = {}

--- A stream that compresses: stream(input [, flush]) returns what came out
--- so far, whether it is finished, and the bytes in and out. flush is
--- "none", "sync", "full" or "finish".
---@param level? integer -1 (default) or 0 to 9
---@param windowBits? integer
---@return fun(input?: string, flush?: "none"|"sync"|"full"|"finish"): string, boolean, integer, integer
function zlib.deflate(level, windowBits) end

--- A stream that decompresses input given in as many pieces as it comes:
--- stream(input) returns what can be decompressed so far, whether the
--- stream has ended, and the bytes in and out.
---@param windowBits? integer
---@return fun(input?: string): string, boolean, integer, integer
function zlib.inflate(windowBits) end

--- Compresses a string at once, in zlib's format unless windowBits says.
---@param data string
---@param level? integer
---@param method? integer
---@param windowBits? integer
---@return string
function zlib.compress(data, level, method, windowBits) end

--- Decompresses zlib or gzip data at once; nil and why.
---@param data string
---@param windowBits? integer
---@return string? data
---@return string? why
function zlib.decompress(data, windowBits) end

---@param data string
---@param level? integer
---@return string
function zlib.gzip(data, level) end

---@param data string
---@return string? data
---@return string? why
function zlib.gunzip(data) end

--- crc32("data") is a string's checksum, crc32(crc, "more") carries one
--- on, and crc32([init]) a function that keeps a running one.
---@param a? string|integer
---@param b? string
---@return integer|fun(data: string): integer
function zlib.crc32(a, b) end

--- As crc32.
---@param a? string|integer
---@param b? string
---@return integer|fun(data: string): integer
function zlib.adler32(a, b) end

---@return string
function zlib.version() end

return zlib
