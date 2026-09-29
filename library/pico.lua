---@meta
--- The console API that `tlua play` installs, written out for a language server.
---
--- This file is never run: it declares what every call takes and gives back, so
--- that an editor with lua-language-server behind it can complete these names,
--- show their arguments while they are being typed, and stop calling them
--- undefined globals. The .luarc.json beside it points the server here.
---
--- docs/pico.md is the same API in prose, with what each call means.

---@class Surface A rectangle of palette indices: a sprite, a sheet, or somewhere to draw.
local Surface = {}

---@return integer
function Surface:width() end

---@return integer
function Surface:height() end

---@return integer width, integer height
function Surface:size() end

--- Reads one pixel; 0 outside the surface.
---@param x integer
---@param y integer
---@return integer colour
function Surface:get(x, y) end

--- Writes one pixel, ignoring anything off the surface.
---@param x integer
---@param y integer
---@param colour? integer defaults to the pen colour
function Surface:set(x, y, colour) end

--- Paints the whole surface one colour.
---@param colour? integer defaults to 0, which is transparent in a sprite
function Surface:fill(colour) end

---@return Surface
function Surface:clone() end

--- Called once before the first frame.
function _init() end

--- Called sixty times a second, for everything that moves.
function _update() end

--- Called once a frame, for everything that is drawn.
function _draw() end

--- Clears the screen, lifts any clipping and puts the text cursor back.
---@param colour? integer defaults to 0
function cls(colour) end

--- Sets the pen colour, and reports the one it replaced. A second colour is the
--- one the set bits of a fill pattern draw in.
---@param colour? integer
---@param pattern_colour? integer
---@return integer previous
function color(colour, pattern_colour) end

--- Draws one pixel.
---@param x number
---@param y number
---@param colour? integer
function pset(x, y, colour) end

--- Reads one pixel of whatever is being drawn on.
---@param x number
---@param y number
---@return integer colour
function pget(x, y) end

---@param x0 number
---@param y0 number
---@param x1 number
---@param y1 number
---@param colour? integer
function line(x0, y0, x1, y1, colour) end

--- Draws the outline of a rectangle, corners included.
---@param x0 number
---@param y0 number
---@param x1 number
---@param y1 number
---@param colour? integer
function rect(x0, y0, x1, y1, colour) end

--- Fills a rectangle, corners included.
---@param x0 number
---@param y0 number
---@param x1 number
---@param y1 number
---@param colour? integer
function rectfill(x0, y0, x1, y1, colour) end

--- Draws the outline of a circle. A radius of 0 is one pixel.
---@param x number
---@param y number
---@param radius? number
---@param colour? integer
function circ(x, y, radius, colour) end

--- Fills a circle.
---@param x number
---@param y number
---@param radius? number
---@param colour? integer
function circfill(x, y, radius, colour) end

--- Draws the outline of the ellipse that fits a rectangle.
---@param x0 number
---@param y0 number
---@param x1 number
---@param y1 number
---@param colour? integer
function oval(x0, y0, x1, y1, colour) end

--- Fills the ellipse that fits a rectangle.
---@param x0 number
---@param y0 number
---@param x1 number
---@param y1 number
---@param colour? integer
function ovalfill(x0, y0, x1, y1, colour) end

--- The outline of a rounded rectangle. Given a width and a height rather than a
--- second corner, as Picotron spells it.
---@param x number
---@param y number
---@param w number
---@param h number
---@param radius? number defaults to 4, and is held to what fits
---@param colour? integer
function rrect(x, y, w, h, radius, colour) end

--- A filled rounded rectangle.
---@param x number
---@param y number
---@param w number
---@param h number
---@param radius? number defaults to 4, and is held to what fits
---@param colour? integer
function rrectfill(x, y, w, h, radius, colour) end

--- Draws the outline of a triangle.
function tri(x0, y0, x1, y1, x2, y2, colour) end

--- Fills a triangle.
function trifill(x0, y0, x1, y1, x2, y2, colour) end

--- Draws text. Without a position it draws at the cursor and moves it down a
--- line. Reports where the text ended.
---@param text any
---@param x? number
---@param y? number
---@param colour? integer
---@return integer x
function print(text, x, y, colour) end

--- Moves the text cursor, and reports where it was.
---@param x? number
---@param y? number
---@param colour? integer
---@return integer x, integer y
function cursor(x, y, colour) end

---@param text any
---@return integer pixels
function textwidth(text) end

---@param text any
---@return integer pixels
function textheight(text) end

--- Shifts everything drawn afterwards by -x, -y. With no arguments, back to the
--- origin. Reports the offset it replaced.
---@param x? number
---@param y? number
---@return integer x, integer y
function camera(x, y) end

--- Confines drawing to a rectangle of the screen. With no arguments, lifts it.
--- Reports the rectangle it replaced.
---@param x? number
---@param y? number
---@param w? number
---@param h? number
---@param intersect? boolean narrow the rectangle already in force
---@return integer x, integer y, integer w, integer h
function clip(x, y, w, h, intersect) end

--- Remaps a colour: as things are drawn, or, with a third argument of 1, as the
--- finished picture reaches the screen. A table remaps several at once, and no
--- arguments puts both palettes back.
---@param from? integer|table
---@param to? integer
---@param screen_palette? integer
function pal(from, to, screen_palette) end

--- Chooses which colour sprite drawing skips. No arguments puts it back to
--- colour 0 alone.
---@param colour? integer
---@param transparent? boolean
function palt(colour, transparent) end

--- Dithers later fills with a 4x4 pattern, the top left pixel being bit 15.
--- With `holes`, the pattern's set bits are left alone instead of being drawn in
--- the second pen colour. Reports the pattern it replaced.
---@param pattern? integer
---@param holes? boolean
---@return integer previous
function fillp(pattern, holes) end

--- The size of the screen.
---@return integer width, integer height
function screen() end

--- The palette: which colours the numbers stand for.
---
--- With nothing, says which one is in use. With a name, loads one of the
--- console's own ("default", "vga") or a .gpl palette file. With a table, takes
--- the colours outright, each either 0xRRGGBB or {r, g, b}. With a number, reads
--- one entry; with a number and a colour, changes it, which is the cheapest way
--- to fade or flash a whole picture.
---@param what? string|table|integer
---@param colour? integer
---@return string name, integer size
function palette(what, colour) end

--- What a colour looks like, as 0xRRGGBB, in whichever palette is loaded.
---@param colour integer
---@return integer rgb
function rgb(colour) end

--- Switches resolution, by Picotron's numbering: 0 is 480x270, 3 is 240x135 and
--- 4 is 160x90; 1 and 2 are the 320x180 and 240x180 it lists as planned. 13 is
--- 320x200, what a VGA card called mode 13h. With nothing, says which mode is in
--- use (-1 for a size asked for some other way) and how big it is.
---@param mode? integer
---@return integer width, integer height
function vid(mode) end

--- A new blank surface, every pixel transparent.
---@param w integer
---@param h integer
---@return Surface
function surface(w, h) end

--- Reads a sprite written out as text: a hex digit per pixel, '.' or a space for
--- the transparent parts, one row per line.
---@param art string
---@return Surface
function sprite(art) end

--- Reads a PNG file and reduces it to the palette.
---@param path string
---@return Surface|nil surface, string? err
function loadpng(path) end

--- Draws a surface at its own size.
---@param s Surface
---@param x number
---@param y number
---@param flip_x? boolean
---@param flip_y? boolean
function spr(s, x, y, flip_x, flip_y) end

--- Draws part of a surface, stretched to fill the destination.
---@param s Surface
---@param sx number
---@param sy number
---@param sw number
---@param sh number
---@param dx number
---@param dy number
---@param dw? number defaults to sw
---@param dh? number defaults to sh
---@param flip_x? boolean
---@param flip_y? boolean
function sspr(s, sx, sy, sw, sh, dx, dy, dw, dh, flip_x, flip_y) end

--- Sends later drawing to a surface, or back to the screen when given nothing.
--- Reports what it replaced.
---@param s? Surface
---@return Surface|nil previous
function target(s) end

--- Draws a grid of tiles cut from a sheet. A row of cells is a table of tile
--- numbers or a string of hex digits, where 0 is nothing at all.
---@param cells table
---@param sheet Surface
---@param x? number
---@param y? number
---@param tile_w? integer defaults to 8
---@param tile_h? integer defaults to 8
function map(cells, sheet, x, y, tile_w, tile_h) end

--- Whether a button is held. 0 to 5 are left, right, up, down, O and X, which
--- can also be named. With no arguments, whether anything at all is held.
---@param button? integer|string
---@param player? integer 0 to 3
---@return boolean
function btn(button, player) end

--- Whether a button has just been pressed, or is repeating.
---@param button integer|string
---@param player? integer
---@return boolean
function btnp(button, player) end

--- How many ticks a button has been held for.
---@param button integer|string
---@param player? integer
---@return integer
function held(button, player) end

--- Whether a key is held, by name: "left", "space", "shift", "f1", "escape", or
--- a single letter. With no argument, whether any key is held.
---@param name? string
---@return boolean
function key(name) end

--- Whether a key has just been pressed, or is repeating.
---@param name string
---@return boolean
function keyp(name) end

--- Where the pointer is in screen pixels, which buttons are down as a bit each
--- (1 left, 2 right, 4 middle), and how far the wheel turned this tick.
---@return integer x, integer y, integer buttons, number wheel
function mouse() end

--- Whether a mouse button is down; with `pressed`, whether it went down this
--- tick. 1 is left, 2 right, 3 middle.
---@param button? integer
---@param pressed? boolean
---@return boolean
function mousebtn(button, pressed) end

--- What was typed this tick.
---@return string
function typed() end

--- Seconds since the program started.
---@return number
function t() end

--- Seconds since the program started.
---@return number
function time() end

--- How many ticks have run.
---@return integer
function frame() end

--- The frame rate the window is managing.
---@return number
function fps() end

--- Writes to the terminal the program was started from; print() draws on the
--- screen instead.
function printh(...) end

--- Closes the window and ends the program.
---@param status? integer
function exit(status) end

--- Asks for the window the program wants. Width and height change the
--- resolution of the screen itself.
---@param opts { title?: string, scale?: integer, fullscreen?: boolean, width?: integer, height?: integer }
function window(opts) end

---@param on? boolean
function fullscreen(on) end

---@param x number
---@return number
function flr(x) end

---@param x number
---@return number
function ceil(x) end

---@param x number
---@return number
function abs(x) end

--- The square root, or 0 for a negative number.
---@param x number
---@return number
function sqrt(x) end

--- -1 below zero, 1 at zero and above.
---@param x number
---@return number
function sgn(x) end

--- A whole circle is 1, and sin runs the same way round as the screen's y axis.
---@param turns number
---@return number
function sin(turns) end

---@param turns number
---@return number
function cos(turns) end

--- The turn that sin and cos would give dx, dy back for, between 0 and 1.
---@param dx number
---@param dy number
---@return number turns
function atan2(dx, dy) end

---@param a number
---@param b? number defaults to 0
---@return number
function min(a, b) end

---@param a number
---@param b? number defaults to 0
---@return number
function max(a, b) end

--- The middle of three numbers, which is how a value is kept inside a range.
---@param a number
---@param b number
---@param c number
---@return number
function mid(a, b, c) end

--- Keeps a number between two others, whichever way round they are given.
---@param x number
---@param lo number
---@param hi number
---@return number
function clamp(x, lo, hi) end

--- A fraction below 1, a number below n, or one of the things in a table.
---@param n? number|table
---@return any
function rnd(n) end

--- Makes the sequence of random numbers repeat.
---@param seed? number
function srand(seed) end

--- Adds a value to the end of a list, or at an index, and reports it back.
---@generic T
---@param list table
---@param value T
---@param index? integer
---@return T
function add(list, value, index) end

--- Removes the first value equal to this one, closing the gap.
---@param list table
---@param value any
---@return any removed
function del(list, value) end

--- Removes the value at an index, the last by default.
---@param list table
---@param index? integer
---@return any removed
function deli(list, index) end

--- Walks a list; safe to delete from while walking.
---@param list table
---@return function
function all(list) end

---@param list table
---@param fn function
function foreach(list, fn) end

--- How many things are in a list, or how many of them are this value.
---@param list table
---@param value? any
---@return integer
function count(list, value) end

--- Part of a string, counting from 1, with negative positions counting back
--- from the end.
---@param s string
---@param from? integer
---@param to? integer
---@return string
function sub(s, from, to) end

--- Splits a string into a table, turning anything that looks like a number into
--- one unless told not to.
---@param s string
---@param separator? string defaults to ","
---@param convert? boolean defaults to true
---@return table
function split(s, separator, convert) end

--- A value as a string; with `hex`, a number in hexadecimal.
---@param v any
---@param hex? boolean
---@return string
function tostr(v, hex) end

--- A value as a number, or nil when it is not one.
---@param v any
---@return number|nil
function tonum(v) end

--- Characters from their numbers.
---@return string
function chr(...) end

--- The number of one character of a string.
---@param s string
---@param index? integer
---@return integer|nil
function ord(s, index) end
