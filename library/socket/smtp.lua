---@meta socket.smtp
--- SMTP client, `require "socket.smtp"` (LuaSocket 3.1).

local smtp = {}

smtp.TIMEOUT = 60
smtp.SERVER = "localhost"
smtp.PORT = 25
smtp.DOMAIN = "localhost"
smtp.ZONE = "-0000"

---@class socket.smtp.Mail
---@field from string "<sender@example.org>"
---@field rcpt string|string[] each "<recipient@example.org>"
---@field source function the message, from smtp.message (an LTN12 source)
---@field user? string
---@field password? string
---@field server? string
---@field port? integer
---@field domain? string
---@field step? function
---@field create? function

--- Sends a message to every recipient.
---@param mail socket.smtp.Mail
---@return 1? ok
---@return string? why
function smtp.send(mail) end

---@class socket.smtp.Message
---@field headers? table<string, string>
---@field body string|function|socket.smtp.Message[]|{ preamble?: string, epilogue?: string, [integer]: socket.smtp.Message }

--- An LTN12 source of the message as text, MIME parts and all.
---@param message socket.smtp.Message
---@return function
function smtp.message(message) end

return smtp
