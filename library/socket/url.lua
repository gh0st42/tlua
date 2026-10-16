---@meta socket.url
--- URLs taken apart and put together, `require "socket.url"` (LuaSocket 3.1).

local url = {}

url._VERSION = "URL 1.0.3"

---@class socket.url.Parsed
---@field scheme? string
---@field authority? string
---@field userinfo? string
---@field user? string
---@field password? string
---@field host? string
---@field port? string
---@field path? string
---@field params? string
---@field query? string
---@field fragment? string

--- The %XX escape of every character but letters, digits and "-_.!~*'()".
---@param s string
---@return string
function url.escape(s) end

---@param s string
---@return string
function url.unescape(s) end

--- An URL's parts, on top of default's.
---@param s string
---@param default? table
---@return socket.url.Parsed
function url.parse(s, default) end

---@param parsed socket.url.Parsed
---@return string
function url.build(parsed) end

--- relative resolved against base, as a browser does.
---@param base string|socket.url.Parsed
---@param relative string
---@return string
function url.absolute(base, relative) end

--- A path's segments, unescaped; is_absolute and is_directory say whether
--- it began and ended with a slash.
---@param path string
---@return { is_absolute: integer?, is_directory: integer?, [integer]: string }
function url.parse_path(path) end

---@param segments table
---@param unsafe? boolean leave the segments unescaped
---@return string
function url.build_path(segments, unsafe) end

return url
