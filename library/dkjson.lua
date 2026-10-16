---@meta dkjson
--- dkjson 2.11, `require "dkjson"`: JSON to Lua values and back.

local json = {}

json.version = "dkjson 2.11"

--- Stands for JSON's null, which nil cannot in a table.
json.null = setmetatable({}, {})

---@class dkjson.State
---@field indent? boolean pretty-print
---@field level? integer
---@field keyorder? string[] these keys first, in this order
---@field exception? fun(reason: string, value: any, state: dkjson.State, defaultmessage: string): string?, string?
---@field buffer? string[]
---@field bufferlen? integer

--- A value as JSON text. A table is an array when its keys are 1..n.
---@param value any
---@param state? dkjson.State
---@return string|boolean json the text, or true when state.buffer was given
---@return string? why
function json.encode(value, state) end

--- JSON text as a value, the position after it, or nil, the position and
--- why ("invalid key at line 1, column 2").
---@param text string
---@param pos? integer 1 by default
---@param nullval? any what null becomes; nil by default
---@param objectmeta? table metatable for objects
---@param arraymeta? table metatable for arrays
---@return any value
---@return integer pos
---@return string? why
function json.decode(text, pos, nullval, objectmeta, arraymeta) end

--- s as a JSON string, quotes included.
---@param s string
---@return string
function json.quotestring(s) end

--- When state.indent is set, adds a new line and the indentation to
--- state.buffer; for a __tojson method.
---@param state dkjson.State
function json.addnewline(state) end

--- An exception function for encode that writes why a value was left out.
function json.encodeexception(reason, value, state, defaultmessage) end

--- Gives the LPeg decoder; tlua has no LPeg, so this raises an error.
function json.use_lpeg() end

return json
