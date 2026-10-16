---@meta mime
--- MIME encodings, `require "mime"` (LuaSocket 3.1): Base64,
--- quoted-printable, line ends and wrapping, as LTN12 filters and as the
--- low-level functions they are made of (mime.core, in Go in tlua).

local mime = {}

mime._VERSION = "MIME 1.0.3"

--- A filter: "base64" or "quoted-printable" (with "text" or "binary").
---@param name "base64"|"quoted-printable"
---@param mode? "text"|"binary"
---@return ltn12.Filter
function mime.encode(name, mode) end

---@param name "base64"|"quoted-printable"
---@return ltn12.Filter
function mime.decode(name) end

--- A filter breaking lines: "text" at length (76), "base64", or
--- "quoted-printable".
---@param name? "text"|"base64"|"quoted-printable"
---@param length? integer
---@return ltn12.Filter
function mime.wrap(name, length) end

--- A filter making every line end marker (CRLF by default).
---@param marker? string
---@return ltn12.Filter
function mime.normalize(marker) end

--- A filter doubling dots at the start of lines, as SMTP needs.
---@return ltn12.Filter
function mime.stuff() end

mime.encodet = {}
mime.decodet = {}
mime.wrapt = {}

--- Base64 of c .. d: what is done, and what is left for the next call.
---@param c string?
---@param d? string
---@return string? done
---@return string? rest
function mime.b64(c, d) end

---@param c string?
---@param d? string
---@return string? done
---@return string? rest
function mime.unb64(c, d) end

---@param c string?
---@param d? string
---@param marker? string
---@return string? done
---@return string? rest
function mime.qp(c, d, marker) end

---@param c string?
---@param d? string
---@return string? done
---@return string? rest
function mime.unqp(c, d) end

--- Wraps text at length, n characters on the line so far.
---@param n integer
---@param c string?
---@param length? integer
---@return string? done
---@return integer n
function mime.wrp(n, c, length) end

---@param n integer
---@param c string?
---@param length? integer
---@return string? done
---@return integer n
function mime.qpwrp(n, c, length) end

--- Line ends as marker, the last character seen being ctx.
---@param ctx integer
---@param c string?
---@param marker? string
---@return string? done
---@return integer ctx
function mime.eol(ctx, c, marker) end

--- Dot-stuffing, as mime.stuff, at the low level.
---@param m integer
---@param c string?
---@return string? done
---@return integer m
function mime.dot(m, c) end

return mime
