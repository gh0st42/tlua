---@meta term
--- An ANSI terminal in the manner of Turbo Pascal and QBasic,
--- `require "term"`, from turbolua. The functions that write take an
--- optional file to write to instead of io.stdout, and return it.

local term = {}

term.ESC = "\27"

--- The 16 EGA colours by name, 0 to 15, in QBasic's order.
---@type table<string, integer>
term.color = {}

---@alias term.Color integer|"black"|"blue"|"green"|"cyan"|"red"|"magenta"|"brown"|"lightgray"|"darkgray"|"lightblue"|"lightgreen"|"lightcyan"|"lightred"|"lightmagenta"|"yellow"|"white"

--- Keys one at a time, without echo (true), or back to lines (false).
--- With stty; not on Windows.
---@param enabled boolean
---@return true? ok
---@return string? why
function term.raw(enabled) end

--- The terminal's columns and rows.
---@return integer? cols
---@return integer|string? rows the rows, or why there are none
function term.size() end

--- One byte of input. With a timeout, in seconds, nil when nothing came
--- in time (in raw mode).
---@param timeout? number
---@return string?
function term.readkey(timeout) end

---@param seconds? number
---@return boolean
function term.sleep(seconds) end

---@param visible boolean
---@param out? file*
function term.cursor(visible, out) end

--- Clears the screen, to a background colour when given, and goes to 1, 1.
---@param color? term.Color
---@param out? file*
function term.clrscr(color, out) end

--- Moves the cursor to column x, row y, from 1.
---@param x integer
---@param y integer
---@param out? file*
function term.gotoxy(x, y, out) end

---@param color term.Color
---@param out? file*
---@return file*? out
---@return string? why
function term.textcolor(color, out) end

---@param color term.Color
---@param out? file*
---@return file*? out
---@return string? why
function term.textbackground(color, out) end

---@param fg term.Color
---@param bg term.Color
---@param out? file*
---@return file*? out
---@return string? why
function term.colorize(fg, bg, out) end

--- Light grey on black, no bold or dim.
---@param out? file*
function term.normvideo(out) end

---@param out? file*
function term.highvideo(out) end

---@param out? file*
function term.lowvideo(out) end

---@param out? file*
function term.flush(out) end

--- Writes str with its colour tags: {red}, {fg:4}, {bg:blue},
--- {yellow,blue}, {reset}, {high}, {low}; {{ and }} for braces.
---@param str any
---@param out? file*
---@return file* out
function term.print(str, out) end

--- term.print, and a new line.
---@param str any
---@param out? file*
---@return file* out
function term.println(str, out) end

return term
