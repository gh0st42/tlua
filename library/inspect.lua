---@meta inspect
--- inspect.lua 3.1, `require "inspect"`: any value as readable text,
--- tables with cycles and metatables included. The module is callable:
--- inspect(v) is inspect.inspect(v).

---@class inspect.Options
---@field depth? integer tables deeper than this are shown as {...}
---@field newline? string "\n" by default
---@field indent? string two spaces by default
---@field process? fun(item: any, path: any[]): any what to show instead of item; nil leaves it out

---@class inspect
---@overload fun(value: any, options?: inspect.Options): string
local inspect = {}

inspect._VERSION = "inspect.lua 3.1.0"

--- Marks a key, and a metatable, in the path process is given.
inspect.KEY = {}
inspect.METATABLE = {}

---@param value any
---@param options? inspect.Options
---@return string
function inspect.inspect(value, options) end

return inspect
