---@meta socket.http
--- HTTP/1.1 client, `require "socket.http"` (LuaSocket 3.1). With
--- `require "ssl.https"`, https:// addresses work too.

local http = {}

--- Seconds to wait for the server.
http.TIMEOUT = 60
http.USERAGENT = "LuaSocket 3.0.0"
--- A proxy for every request, "http://host:port".
---@type string?
http.PROXY = nil

---@class socket.http.Request
---@field url string
---@field sink? function an LTN12 sink for the body; nothing keeps it otherwise
---@field method? string "GET" by default, "POST" with a source
---@field headers? table<string, string>
---@field source? function an LTN12 source for the request's body
---@field step? function an LTN12 pump step
---@field proxy? string
---@field redirect? boolean follow redirects (the default)
---@field create? function makes the socket
---@field maxredirects? integer

--- The simple form: GET url, or POST body to it, and give the body.
--- The generic form: a table, with the body going to its sink.
---@overload fun(request: socket.http.Request): (1|nil), integer|string, table<string, string>?, string?
---@param url string
---@param body? string sent with POST, as application/x-www-form-urlencoded
---@return string? body
---@return integer|string code_or_why
---@return table<string, string>? headers
---@return string? status "HTTP/1.1 200 OK"
function http.request(url, body) end

return http
