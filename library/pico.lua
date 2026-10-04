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

--- How the surface is cut into sprites: the size of a cell and how many there
--- are, or 0, 0, 0 for a picture that was never cut up.
---@return integer cell_w, integer cell_h, integer count
function Surface:grid() end

--- One sprite of the sheet as a surface of its own — a copy, so drawing on it
--- leaves the sheet alone. Nothing, for a number that is not a sprite on it.
---@param n integer counted from zero
---@param w? integer cells wide, 1 by default
---@param h? integer cells tall, 1 by default
---@return Surface|nil
function Surface:sprite(n, w, h) end

--- The eight flags of one of this sheet's sprites, or with a mask, sets them
--- and hands the surface back.
---@param n integer
---@param mask? integer
---@return integer|Surface
function Surface:flags(n, mask) end

--- One property of a sprite: what a Tiled tileset said about that tile beyond
--- its eight flags, or failing that what it said about the whole sheet, or the
--- answer given here for a tile that carries none.
---@param n integer counted from zero
---@param key string
---@param missing? any what to say when nothing does
---@return any
function Surface:prop(n, key, missing) end

--- Everything a sprite carries, as a table: what the sheet says for every
--- sprite, with the sprite's own over the top. With no sprite number, the
--- sheet's own.
---@param n? integer
---@return table
function Surface:props(n) end

--- Writes one property of a sprite, over whatever the artwork said, and hands
--- the surface back.
---@param n integer
---@param key string
---@param value string|number|boolean|nil nothing puts the sheet's answer back
---@return Surface
function Surface:setprop(n, key, value) end

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

--- A new blank surface, every pixel transparent. With a cell size it is a sheet.
---@param w integer
---@param h integer
---@param cell_w? integer
---@param cell_h? integer square if left out
---@return Surface
function surface(w, h, cell_w, cell_h) end

--- Reads a sprite written out as text: a hex digit per pixel, '.' or a space for
--- the transparent parts, one row per line. With a cell size it is a sheet of
--- sprites rather than one picture.
---@param art string
---@param cell_w? integer
---@param cell_h? integer square if left out
---@return Surface
function sprite(art, cell_w, cell_h) end

--- Reads a PNG and reduces it to the palette. Looks inside the executable first
--- when the game was fused with `tlua fuse -play`, then beside the program.
--- With a cell size the picture is a sheet of sprites; without one, a sheet
--- drawn in an editor says its own cell size and sprite flags, out of the PNG
--- or a Tiled tileset of the same name beside it.
---@param path string
---@param cell_w? integer
---@param cell_h? integer square if left out
---@return Surface|nil surface, string? err
function loadpng(path, cell_w, cell_h) end

--- Reads back what store() saved, or failing that a file the game was shipped
--- with. A saved value comes back as the value it was; a plain file comes back
--- as its text.
---@param name string
---@return any|nil value, string? err
function fetch(name) end

--- Which font print() draws with. With nothing, reports the font in hand and
--- the size of one of its characters. With a name, switches to a built-in:
--- "small" (3x5) or "unscii" (8x8, most of Unicode). With a sheet, uses that
--- artwork as lettering, one cell a character, the first standing for `first`
--- (a space by default); every pixel that is not colour 0 is ink, drawn in the
--- colour print() is given.
---
--- Reports what it replaced, in the form it would be given back.
---@param which? string|Surface
---@param first? integer the character the sheet's first cell stands for
---@return string|Surface was, integer? width, integer? height
function font(which, first) end

--- Says that this program wants a window and the console API, for a program run
--- as an ordinary script: `tlua game.lua`, a shebang line, or one fused without
--- -play. Call it first, before anything else of the console: nothing of it
--- exists until this runs. The window opens once the file has finished, so the
--- callbacks may be written after the call.
---
--- In a program already being played it is simply true. Takes what window()
--- takes, so `boot{ title = "snake", scale = 3 }` says both at once.
---@param options? { title?: string, scale?: integer, fullscreen?: boolean, width?: integer, height?: integer }
---@return boolean
function boot(options) end

--- Saves something for the next time the game is run: a score, where the player
--- had got to, what they chose. Strings, numbers, truths and tables of them can
--- be saved. The name is a name, not a path.
---@param name string
---@param value string|number|boolean|table
---@return boolean|nil ok, string? err
function store(name, value) end

--- Draws a sprite, in whichever of three ways it is asked:
---
---   spr(n, x, y, [w], [h], [flip_x], [flip_y])         from the current sheet
---   spr(sheet, n, x, y, [w], [h], [flip_x], [flip_y])  from a sheet by name
---   spr(picture, x, y, [flip_x], [flip_y])             a whole picture
---
--- Sprites are counted from zero, and w and h are counted in cells.
---@param sheet Surface|integer a sheet, a picture, or a sprite number
---@param n integer|number a sprite number, or x for a picture
---@param x number|integer
---@param y? number
---@param w? integer cells wide
---@param h? integer cells tall
---@param flip_x? boolean
---@param flip_y? boolean
---@param turn? boolean mirrored across its own diagonal: a tile's flipd
function spr(sheet, n, x, y, w, h, flip_x, flip_y, turn) end

--- Draws a rectangle of pixels — not of cells — stretched to fill the
--- destination. Without a surface in front, it comes from the current sheet.
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
---@param turn? boolean mirrored across its own diagonal
function sspr(s, sx, sy, sw, sh, dx, dy, dw, dh, flip_x, flip_y, turn) end

--- Sends later drawing to a surface, or back to the screen when given nothing.
--- Reports what it replaced.
---@param s? Surface
---@return Surface|nil previous
function target(s) end

--- Makes a sheet the one that spr(n, ...), sspr(...) and sget() mean, the way
--- Picotron has one spritesheet in hand. Reports the one it replaced; with no
--- argument it asks without changing anything.
---
--- It is not called "sheet" because that is what the variable holding one is
--- usually called.
---@param s? Surface a surface with a grid on it
---@return Surface|nil previous
function usesheet(s) end

--- All eight flags of a sprite as a number, or one of them as a boolean.
--- Flags mean whatever a game decides; they come with the artwork, out of the
--- PNG or the Tiled tileset beside it.
---@param n integer sprite number
---@param bit? integer 0 to 7
---@return integer|boolean
function fget(n, bit) end

--- Sets all eight flags of a sprite, or one of them.
---@param n integer
---@param bit_or_mask integer
---@param on? boolean given, bit_or_mask is the bit to set
function fset(n, bit_or_mask, on) end

--- Reads a pixel of the current sheet.
---@param x integer
---@param y integer
---@return integer colour
function sget(x, y) end

--- Writes a pixel of the current sheet.
---@param x integer
---@param y integer
---@param colour? integer defaults to the pen colour
function sset(x, y, colour) end

---@class Tilemap A level: layers of sprites, with things placed on them.
local Tilemap = {}

---@return integer width, integer height in cells
function Tilemap:size() end

---@return integer width, integer height of one cell, in pixels
function Tilemap:tile() end

--- The names of its layers, in order.
---@return string[]
function Tilemap:layers() end

--- One layer, by name or by its place counted from one.
---@param which string|integer
---@return MapLayer|nil
function Tilemap:layer(which) end

--- What was placed on a layer, as plain MapObject tables.
---@param which string|integer
---@return MapObject[]
function Tilemap:objects(which) end

--- What the map itself was labelled with in the editor.
---@return table
function Tilemap:props() end

--- One of the sheets the map draws with, counted from one, which is what an
--- object's sprite is numbered against.
---@param n? integer 1 by default
---@return Surface|nil
function Tilemap:sheet(n) end

--- How many sheets it draws with.
---@return integer
function Tilemap:sheets() end

---@class MapObject Something placed on a map rather than drawn into it.
---@field id integer the number the editor gave it
---@field name string
---@field class string what Tiled calls the type
---@field x number
---@field y number
---@field w number
---@field h number
---@field rotation number in turns, clockwise; the editor writes degrees
---@field visible boolean
---@field shape "rect"|"ellipse"|"point"|"polygon"|"polyline"
---@field points? {x: number, y: number}[] the corners of an outline, relative to x and y
---@field sprite? integer the tile it was given, for a piece of scenery
---@field sheet? integer which of the map's sheets that sprite is on
---@field flipx? boolean
---@field flipy? boolean
---@field flipd? boolean turned as well as mirrored
---@field props table what it was labelled with
local MapObject = {}

--- Draws a window of it without making it the current map.
function Tilemap:draw(tx, ty, sx, sy, tw, th, flags) end

---@class MapLayer One layer of a map.
local MapLayer = {}

---@return string
function MapLayer:name() end

---@return integer width, integer height in cells
function MapLayer:size() end

--- Whether it is drawn; with an argument, sets that.
---@param on? boolean
---@return boolean
function MapLayer:visible(on) end

---@return MapObject[]
function MapLayer:objects() end

---@param x integer
---@param y integer
---@return integer sprite
function MapLayer:get(x, y) end

---@param x integer
---@param y integer
---@param sprite integer
function MapLayer:set(x, y, sprite) end

--- Draws a window of this layer alone.
function MapLayer:draw(tx, ty, sx, sy, tw, th, flags) end

--- Reads a Tiled map (.tmj) and everything it draws with: the tilesets it
--- names, and the pictures those name in turn.
---@param name string
---@return Tilemap|nil map, string? err
function loadmap(name) end

--- Makes a map the one that map(), mget() and mset() mean. Reports the one it
--- replaced.
---@param m? Tilemap
---@return Tilemap|nil previous
function usemap(m) end

--- The sprite in a square of the current map, or 0 for an empty one. Without a
--- layer it means the first one.
---@param x integer
---@param y integer
---@param layer? string|integer
---@return integer sprite
function mget(x, y, layer) end

--- Puts a sprite in a square of the current map.
---@param x integer
---@param y integer
---@param sprite integer
---@param layer? string|integer
function mset(x, y, sprite, layer) end

--- Draws a grid of sprites. A row of cells is a table of sprite numbers or a
--- string of hex digits.
---
--- Sprite 0 is not drawn unless draw_zero says so, which is what lets a 0 in a
--- level mean open sky. Tiles are the size of the sheet's own cells unless told
--- otherwise.
---@param cells table
---@param sheet Surface
---@param x? number
---@param y? number
---@param tile_w? integer defaults to the sheet's cell width
---@param tile_h? integer defaults to the sheet's cell height
---@param draw_zero? boolean draw sprite 0 as well
function map(cells, sheet, x, y, tile_w, tile_h, draw_zero) end

--- Draws a window of the current map: from cell tx, ty, onto the screen at
--- sx, sy, tw by th cells of it. With flags, only sprites carrying one of them
--- are drawn. Everything may be left out, and then the whole map is drawn at
--- the origin.
---@param tx? integer
---@param ty? integer
---@param sx? integer
---@param sy? integer
---@param tw? integer
---@param th? integer
---@param flags? integer

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

--- What the key that works a button is called on the keyboard in use: "Z" on an
--- American one, "Y" on a German one, since the console binds where a key is
--- rather than what it says. Use it rather than naming a letter outright.
---@param button integer|string
---@param player? integer
---@return string
function btnkey(button, player) end

--- What was typed this tick.
---@return string
function typed() end

--- Plays a sound, and reports the channel it went to. The name is looked for
--- with the extensions and in the folders a game keeps sounds in: sfx("jump")
--- finds sfx/jump.wav or assets/sfx/jump.wav.
---
--- sfx(-1) stops every channel, and sfx(-1, channel) stops one.
---@param name string|integer
---@param channel? integer 0 to 7, or -1 for any free one
---@param volume? number 0 to 1
---@return integer|nil channel, string? err
function sfx(name, channel, volume) end

--- Starts the music, which loops until something else is asked for. music(-1)
--- stops it, with an optional fade in milliseconds, and music() on its own
--- reports what is playing.
---@param name? string|integer
---@param fade_ms? integer
---@param volume? number 0 to 1
---@return string|nil playing, string? err
function music(name, fade_ms, volume) end

--- How loud everything is, from 0 to 1. Reports what it was.
---@param v? number
---@return number previous
function volume(v) end

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

--- Shows what has been drawn and waits for the next tick, carrying on from
--- here with fresh input. It is what lets a loop live somewhere other than
--- _update: a modal dialog, a cutscene, or the whole game written as one loop.
---
--- A flip is a tick, so t() and frame() move on. It cannot be called from
--- inside a coroutine the program made itself.
function flip() end

--- Asks to run n times a second instead of sixty, and reports the rate it
--- replaced. _update and _draw are called that often and t() counts seconds by
--- it. -1 runs once for every refresh of the screen.
---@param n? integer 1 to 1000, or -1
---@return integer previous
function setfps(n) end

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

--- Makes a coroutine: Lua's coroutine.create, under the name this lineage calls
--- it by. For work that takes many frames and reads better written straight
--- through — a cutscene, a path being walked.
---@param fn function
---@return thread
function cocreate(fn) end

--- Carries a coroutine on from where it yielded. coroutine.resume.
---@param co thread
---@param ... any passed to the coroutine
---@return boolean ok, ... any what it yielded or returned
function coresume(co, ...) end

--- "running", "suspended", "normal" or "dead". coroutine.status.
---@param co thread
---@return string
function costatus(co) end

--- Suspends the coroutine this is called in, handing its arguments back to
--- whoever resumed it. coroutine.yield. Not flip(): that suspends the program's
--- own loop, which a coroutine of your own is not.
---@param ... any
---@return ... any what the next resume passes in
function yield(...) end
