---@meta ltn12
--- Sources, sinks, filters and pumps, `require "ltn12"` (LuaSocket 3.1).
--- A source gives a chunk each call, nil at the end; a sink takes a chunk,
--- nil at the end; a filter turns a chunk into another, "" asking for more.

local ltn12 = {}

ltn12._VERSION = "LTN12 1.0.3"
ltn12.BLOCKSIZE = 2048

---@alias ltn12.Source fun(): string?, string?
---@alias ltn12.Sink fun(chunk: string?, err: string?): 1?, string?
---@alias ltn12.Filter fun(chunk: string?): string?

ltn12.filter = {}

--- A filter calling low(ctx, chunk, extra) and keeping its context.
---@param low function
---@param ctx any
---@param extra any
---@return ltn12.Filter
function ltn12.filter.cycle(low, ctx, extra) end

--- The filters one after another.
---@param ... ltn12.Filter
---@return ltn12.Filter
function ltn12.filter.chain(...) end

ltn12.source = {}

---@return ltn12.Source
function ltn12.source.empty() end
---@param why string
---@return ltn12.Source
function ltn12.source.error(why) end
--- A file's contents, closing it at the end.
---@param file file*?
---@param io_err? string
---@return ltn12.Source
function ltn12.source.file(file, io_err) end
---@param s string
---@return ltn12.Source
function ltn12.source.string(s) end
--- The strings in a table, one by one.
---@param t string[]
---@return ltn12.Source
function ltn12.source.table(t) end
---@param src ltn12.Source
---@return ltn12.Source
function ltn12.source.simplify(src) end
--- A source that can be given chunks back.
---@param src ltn12.Source
---@return ltn12.Source
function ltn12.source.rewind(src) end
--- src with a filter on it (or several, chained).
---@param src ltn12.Source
---@param ... ltn12.Filter
---@return ltn12.Source
function ltn12.source.chain(src, ...) end
--- The sources one after another.
---@param ... ltn12.Source
---@return ltn12.Source
function ltn12.source.cat(...) end

ltn12.sink = {}

--- A sink collecting the chunks in t (a new table if none), and t.
---@param t? string[]
---@return ltn12.Sink
---@return string[]
function ltn12.sink.table(t) end
---@param snk ltn12.Sink
---@return ltn12.Sink
function ltn12.sink.simplify(snk) end
--- A sink writing to a file, closing it at the end.
---@param file file*?
---@param io_err? string
---@return ltn12.Sink
function ltn12.sink.file(file, io_err) end
---@return ltn12.Sink
function ltn12.sink.null() end
---@param why string
---@return ltn12.Sink
function ltn12.sink.error(why) end
--- A filter in front of a sink.
---@param f ltn12.Filter
---@param snk ltn12.Sink
---@return ltn12.Sink
function ltn12.sink.chain(f, snk) end

ltn12.pump = {}

--- Moves one chunk from src to snk.
---@param src ltn12.Source
---@param snk ltn12.Sink
---@return 1? more
---@return string? why
function ltn12.pump.step(src, snk) end

--- Moves everything from src to snk.
---@param src ltn12.Source
---@param snk ltn12.Sink
---@param step? function
---@return 1? ok
---@return string? why
function ltn12.pump.all(src, snk, step) end

return ltn12
