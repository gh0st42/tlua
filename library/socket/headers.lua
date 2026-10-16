---@meta socket.headers
--- Header names as they are usually written, `require "socket.headers"`
--- (LuaSocket 3.1).

local headers = {}

--- From the lower-case name to the usual one: canonic["content-type"] is
--- "Content-Type".
---@type table<string, string>
headers.canonic = {}

return headers
