---@meta argparse
--- argparse 0.7.2, `require "argparse"`: command-line options, arguments,
--- subcommands and --help. argparse(name, description, epilog) makes a
--- parser. Every setter returns its element, to chain; an element can also
--- be called with a table of properties.

---@class argparse.Element
---@overload fun(self, props: table): argparse.Element
local Element = {}

---@return self
function Element:description(text) end
---@return self
function Element:default(value) end
--- A function, or a table of values, that turns what was typed into a value.
---@return self
function Element:convert(fn) end
--- How many values: a number, "?", "*", "+", or "2-3".
---@return self
function Element:args(n) end
--- How many times it may be given, as for args.
---@return self
function Element:count(n) end
---@param values string[]
---@return self
function Element:choices(values) end
--- What to do with the value: "store", "store_true", "count", "append", ... or a function.
---@return self
function Element:action(action) end
--- The field of the result the value goes in.
---@return self
function Element:target(name) end
---@return self
function Element:hidden(hidden) end
---@return self
function Element:argname(name) end

---@class argparse.Parser: argparse.Element
local Parser = {}

--- A positional argument.
---@return argparse.Element
function Parser:argument(name, description, default, convert, args) end

--- An option with a value, such as "-o --output".
---@return argparse.Element
function Parser:option(name, description, default, convert, args, count) end

--- An option without a value.
---@return argparse.Element
function Parser:flag(name, description, default, convert, count) end

--- A subcommand, with options and arguments of its own.
---@return argparse.Parser
function Parser:command(name, description, epilog) end

--- Parses args (arg by default); prints the error and usage and exits on
--- a mistake.
---@param args? string[]
---@return table result
function Parser:parse(args) end

--- parse, giving false and the error instead of exiting.
---@param args? string[]
---@return boolean ok
---@return table|string result_or_error
function Parser:pparse(args) end

--- Prints msg with the usage, and exits.
function Parser:error(msg) end

---@return string
function Parser:get_usage() end

---@return string
function Parser:get_help() end

--- Each subcommand's result goes in result[name] (true) or under name.
---@return self
function Parser:command_target(name) end

---@return self
function Parser:require_command(required) end

--- Options given at most one of.
---@return self
function Parser:mutex(...) end

--- An option, --completion by default, that prints a completion script.
---@return self
function Parser:add_complete(value) end

---@return string
function Parser:get_bash_complete() end
---@return string
function Parser:get_zsh_complete() end
---@return string
function Parser:get_fish_complete() end

---@class argparse
---@overload fun(name?: string, description?: string, epilog?: string): argparse.Parser
local argparse = {}

argparse.version = "0.7.2"

return argparse
