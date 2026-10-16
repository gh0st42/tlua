---@meta socket.tp
--- The line protocol FTP and SMTP are built on, `require "socket.tp"`
--- (LuaSocket 3.1): commands, and replies with a three-digit code.

local tp = {}

tp.TIMEOUT = 60

---@class socket.tp.Connection
local Connection = {}

--- Reads a reply and checks its code against ok: a code, a pattern
--- ("2.."), a list of them, or a function.
---@param ok any
---@return integer|nil code
---@return string reply_or_why
function Connection:check(ok) end

--- Sends "cmd arg\r\n".
---@param cmd string
---@param arg? string
---@return integer|nil sent
---@return string? why
function Connection:command(cmd, arg) end

--- A whole reply, lines of a multi-line one included.
---@return integer|nil code
---@return string reply_or_why
function Connection:receive(pattern) end

---@param data string
function Connection:send(data) end

--- Pumps an LTN12 source into the connection.
function Connection:source(source, step) end

--- Pumps from the connection into an LTN12 sink.
function Connection:sink(sink, pattern) end

---@return socket.TCP
function Connection:getcontrol() end

function Connection:getpeername() end
function Connection:getsockname() end
function Connection:getfd() end
function Connection:dirty() end
function Connection:close() end

---@param host string
---@param port integer
---@param timeout? number
---@param create? function
---@return socket.tp.Connection? connection
---@return string? why
function tp.connect(host, port, timeout, create) end

return tp
