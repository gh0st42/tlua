---@meta socket.ftp
--- FTP client, `require "socket.ftp"` (LuaSocket 3.1).

local ftp = {}

ftp.TIMEOUT = 60
ftp.USER = "ftp"
ftp.PASSWORD = "anonymous@anonymous.org"

---@class socket.ftp.Request
---@field host string
---@field path? string
---@field sink? function where get puts the file (an LTN12 sink)
---@field source? function what put sends (an LTN12 source)
---@field user? string
---@field password? string
---@field port? integer
---@field type? "a"|"i" text or binary
---@field command? string
---@field argument? string
---@field check? any
---@field step? function
---@field create? function

--- The simple form: the file at "ftp://[user[:password]@]host/path[;type=a|i]".
--- The generic form: a table, with the file going to its sink.
---@overload fun(request: socket.ftp.Request): 1?, string?
---@param url string
---@return string? data
---@return string? why
function ftp.get(url) end

--- Sends data to url, or (the generic form) the table's source.
---@overload fun(request: socket.ftp.Request): 1?, string?
---@param url string
---@param data string
---@return 1? ok
---@return string? why
function ftp.put(url, data) end

--- Sends one command, as the table's command and argument say.
---@param request socket.ftp.Request
---@return 1? ok
---@return string? why
function ftp.command(request) end

return ftp
