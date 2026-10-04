# The console

`tlua play` runs a Lua program against a fantasy console in the spirit of
PICO-8 and Picotron: a fixed screen, a fixed palette, sprites, input, and a
program made of `_update()` and `_draw()`.

```sh
tlua play game.lua            # one file
tlua play .                   # the main.lua in this directory
tlua play -scale 3 game.lua   # three screen pixels to a console pixel
tlua play -fullscreen game.lua
```

There are no cartridges and no editor of sprites or maps: artwork is written
out as text in the program, or drawn elsewhere and loaded from a PNG, and a
level comes out of a map editor.

- [examples/pico](../examples/pico) has sixteen programs written against this,
  and one folder with its artwork and level beside it.
- [library/pico.lua](../library/pico.lua) declares it all for
  lua-language-server, so an editor can complete these names and show what they
  take. The `.luarc.json` at the root of this repository points at it.

## The shape of a program

```lua
function _init()   -- once, before the first frame
end

function _update() -- sixty times a second: move things
end

function _draw()   -- once a frame: draw them
end
```

The file itself is run first, so anything at the top level happens before
`_init`. A program with no callbacks at all is drawn once and left on the
screen, which is enough for something that only wants to show a picture.

An error in any of them stops the program, printing the file and the line.

### Asking for a window

`tlua play` is one way in. The other is for the program to say so itself, on
its first line:

```lua
boot()                  -- this program wants a window and the console

local w, h = screen()
function _draw() cls(1) print("hi", 8, 8, 7) end
```

That file runs four ways without changing: `tlua game.lua`, `tlua play
game.lua`, `tlua fuse -play`, and `tlua fuse` with no flag at all. It also
works with a shebang, which `tlua play` cannot:

```lua
#!/usr/bin/env tlua
boot()
```

`boot()` takes what `window{}` takes, so a program can say both at once:
`boot{ title = "snake", scale = 3 }`.

Two things follow from how it works. **It goes first**, because the console is
not installed until it is called — `screen()` on the line above it is a call to
nothing. And from that line on, `print` draws on the screen rather than writing
to the terminal, as it does in any program run this way; `printh` is the one
that writes to the terminal.

The window opens once the file has finished running, which is why the callbacks
can be written after the call rather than before it. In a program already being
played, `boot()` is simply true.

### Running a loop of your own

`flip()` shows what has been drawn and waits for the next tick. The program
carries on from exactly where it was, with a tick's worth of fresh input — so a
loop can live anywhere, and the whole game can be written as one:

```lua
while true do
	cls(1)
	if btn("left") then x = x - 1 end
	spr(ship, x, y)
	flip()
end
```

The reason to want that is usually a modal dialog: branch out of the game, loop
until the question is answered, and carry on. No state machine, no "am I in a
dialog" flag threaded through everything — the answer is what the function
returns.

```lua
local function ask(question)
	while true do
		draw_the_box(question)
		if btnp("x") then return true end
		if btnp("o") then return false end
		flip()
	end
end

function _update()
	if btnp("x") and standing_at_the_door then
		if ask("go inside?") then enter() end   -- many frames may pass here
	end
end
```

A flip is a tick: `t()` and `frame()` move on, input is fresh afterwards, and
what was drawn before it is what is shown. The two ways of writing a program
mix freely — `_update` and `_draw` are called every tick as ever, and either
may go off and run its own loop for a while.

Because a flip is a tick, it is also what paces the loop: `flip()` waits, so a
loop of nothing but `flip()` runs at the frame rate — sixty times a second by
default, or whatever `setfps` was last given (see [Time, and the
window](#time-and-the-window)). There is no need to sleep, and no way to run
faster by flipping harder. A loop that does more work
in a tick than a tick lasts simply runs slower, the same as an `_update` that
takes too long.

Underneath, the program runs inside a coroutine of its own for its whole life,
which is what lets it be suspended mid-call and picked up again. One
consequence: `flip()` cannot be called from inside a coroutine the program made
itself, because that would hand control to whoever resumed that one rather than
to the loop. It says so if you try.

`examples/pico/dialog.lua` is a worked example.

## The screen

480 by 270 pixels, one byte a pixel, so 256 colours at the very most and 64 to
begin with. The window scales whatever it is given up to fit, by a whole
number, with black around the edges.

`vid(mode)` switches resolution:

| Mode | Size | |
| --- | --- | --- |
| `vid(0)` | 480x270 | the console's own |
| `vid(1)` | 320x180 | listed as planned in Picotron |
| `vid(2)` | 240x180 | listed as planned in Picotron |
| `vid(3)` | 240x135 | half of it each way |
| `vid(4)` | 160x90 | a third each way, and small enough to work out every pixel in Lua |
| `vid(13)` | 320x200 | what a VGA card called mode 13h |

`vid()` on its own says which mode is in use and how big it is; a size asked
for some other way reports as mode -1. `window{width=, height=}` still takes
any size at all, and `screen()` reports it.

The numbering is Picotron's, including the two modes it lists as planned rather
than supported, so that the same call means the same thing in both. Mode 13 is
the one addition: a great deal of pixel art was drawn at 320x200, and it goes
with the VGA palette below.

### Colours

A colour is a number, and what it looks like is up to the palette. Colour
numbers wrap round the size of the palette in use, so in the 64 the console
starts with, colour 70 is colour 6.

The palette to begin with has 64 colours. **0 to 15 are PICO-8's palette and 16
to 31 its extended one**, both exactly as they are there. 32 to 63 are tlua's
own: a grey scale at 32, then ramps of blue at 40, green at 46, warm at 52 and
violet at 58, each running dark to light. Picotron's own are not published as a
list and are not reproduced. `examples/pico/palette.lua` shows whichever
palette is loaded, with the numbers.

| Call | What it does |
| --- | --- |
| `palette()` | The name of the palette in use, and how many colours it has. |
| `palette("vga")` | One of the console's own: `"default"` or `"vga"`. |
| `palette("some.gpl")` | A palette file, as any pixel art tool will export. |
| `palette(table)` | The colours outright: `0xRRGGBB` each, or `{r, g, b}`. |
| `palette(i)` | What colour `i` looks like. |
| `palette(i, 0xRRGGBB)` | Changes it, and reports what it was. |
| `rgb(i)` | The same as `palette(i)`, for reading in the middle of an expression. |

`"vga"` is the 256 colours an IBM VGA card came up in. The first 16 are the EGA
colours and the next 16 the grey scale, both as they were; the 216 after them
are built the way the card's table was laid out — three tiers of brightness,
each with three of saturation, each running 24 hues round the wheel from
blue — rather than copied entry by entry, so a few are a shade off. The last
eight are black, as they were there.

Changing one entry changes every pixel already drawn in that colour, since the
screen holds indices and not colours:

```lua
for i = 0, 15 do          -- fade the whole picture towards black
	palette(i, 0)
end
```

That is a different thing from `pal(from, to)`, which remaps one index onto
another and leaves the palette alone.

## Drawing

Every drawing call takes its colour last, and it is optional: passing one also
makes it the pen colour, so the next call without one follows suit.

| Call | What it does |
| --- | --- |
| `cls([colour])` | Clears to a colour (0 by default), lifts clipping, and puts the text cursor back in the corner. |
| `color(c, [c2])` | Sets the pen, and reports the one it replaced. `c2` is what the set bits of a fill pattern draw in. |
| `pset(x, y, [c])` | One pixel. |
| `pget(x, y)` | Reads one pixel back. |
| `line(x0, y0, x1, y1, [c])` | A line, both ends included. |
| `rect(x0, y0, x1, y1, [c])` | The outline of a rectangle. |
| `rectfill(x0, y0, x1, y1, [c])` | A filled one. |
| `circ(x, y, [r], [c])` | The outline of a circle; a radius of 0 is one pixel. |
| `circfill(x, y, [r], [c])` | A filled one. |
| `oval(x0, y0, x1, y1, [c])` | The ellipse that fits a rectangle. |
| `ovalfill(x0, y0, x1, y1, [c])` | A filled one. |
| `rrect(x, y, w, h, [r], [c])` | The outline of a rounded rectangle. |
| `rrectfill(x, y, w, h, [r], [c])` | A filled one. |
| `tri(x0, y0, x1, y1, x2, y2, [c])` | The outline of a triangle. |
| `trifill(...)` | A filled one. |

**The two rounded rectangles take a width and a height** where the others take
a second corner, because that is how Picotron spells them. The radius defaults
to 4 and is held to half the shorter side, so asking for more rounding than the
shape can take gives a circle rather than something inside out. The corners are
quarters of the circle `circ()` would draw at that radius.

Coordinates are pixels, counting from the top left, and are rounded down; a
number far off the screen costs nothing, because everything is clipped before
it is drawn rather than after.

Triangles are the one addition to what PICO-8 and Picotron have. Circles are
drawn to the widths the midpoint algorithm gives, so a radius of 4 is three
pixels across at the poles and nine across the middle, as it is there.

### Text

| Call | What it does |
| --- | --- |
| `print(text, [x], [y], [c])` | Draws text, and reports the x it ended at. |
| `cursor([x], [y], [c])` | Moves the cursor a bare `print` draws at. |
| `textwidth(text)`, `textheight(text)` | How big it will be, in the font in hand. |
| `font([which], [first])` | Which font to draw with; reports the one it replaced. |

`print` with no position draws at the cursor and moves it down a line, so
several in a row stack up. `\n` inside the text starts a new line under the
first.

#### Fonts

Two come with the console, and a program can bring its own.

| | |
| --- | --- |
| `"small"` | Three pixels by five, on a grid of four by six. What a console starts with. |
| `"unscii"` | Eight by eight, with most of Unicode in it. |

```lua
font("unscii")
print("héllo ▲ ╔═╗ Привет", 8, 8, 7)
```

The small font draws lower case as small capitals, four rows tall, as PICO-8's
does: three pixels is not enough to tell `a` from `A` any other way. Unscii has
room for real lower case, and for accented letters, Greek, Cyrillic, arrows,
box drawing and the block characters old machines drew their graphics with. It
is [Viznut's unscii-8](http://viznut.fi/unscii/), which is in the public
domain, embedded as the bitmaps it is rather than as a TrueType file: the
outlines in that file are squares drawn around these very pixels, and this
screen has no shade of grey to rasterise one into.

**A font of your own** is a sheet of lettering, one cell a character:

```lua
local letters = loadpng("font", 8, 8)   -- or sprite[[ … ]] with a grid
font(letters, 65)                        -- its first cell is "A"
```

The second argument is the character the first cell stands for, a space unless
said otherwise, since that is where a character set usually starts. Every pixel
that is not colour 0 is ink, and it is drawn in the colour `print` is given —
so one sheet of lettering serves every colour, and `palette()` reaches it like
anything else.

`font()` reports what it replaced, in the form it would be given back, so a
routine can borrow a font and put the one it found back afterwards:

```lua
local was = font("unscii")
print("BIG", 8, 8, 7)
font(was)
```

With no arguments it says the name of the font in hand and the size of one
character. `textwidth` and `textheight` follow whichever font that is, so
centring text needs no arithmetic of its own when the font changes.

`examples/pico/fonts.lua` shows all three kinds side by side. `snake.lua` and
`dialog.lua` show the usual arrangement in a game: the bigger font for a
sentence meant to be read, the small one for the numbers around the edge.

`print` draws on the screen. **`printh` is the one that writes to the terminal**
the program was started from.

### Where and through what

| Call | What it does |
| --- | --- |
| `camera([x], [y])` | Shifts everything drawn afterwards by -x, -y. No arguments puts it back. Reports the offset it replaced. |
| `clip([x], [y], [w], [h], [intersect])` | Confines drawing to a rectangle of the screen. No arguments lifts it. Reports the rectangle it replaced. |
| `pal([from], [to], [screen])` | Remaps a colour. |
| `palt([colour], [on])` | Chooses which colour sprites skip. |
| `fillp([pattern], [holes])` | Dithers later fills. Reports the pattern it replaced. |

The camera moves the world; clipping is a window on the view, and does not move
with it.

`pal(from, to)` recolours what is drawn next, which is how one sprite becomes
eight enemies. `pal(from, to, 1)` recolours the finished picture on its way to
the screen instead, which is how everything fades at once without touching what
is drawn. `pal(table)` applies a whole set at once, `pal()` puts both back.

`fillp(pattern)` takes sixteen bits as a 4x4 dither, bit 15 being the top left
pixel. Where a bit is set, the second pen colour is drawn; with
`fillp(pattern, true)` nothing is drawn there at all, so what is behind shows
through. The pattern is anchored to the screen, not to the shape, so a moving
shape does not drag its texture along. It applies to fills, not to sprites or
text. `fillp()` turns it off.

## Sprites and surfaces

A surface is a rectangle of palette indices. Sprites, sheets and anywhere else a
program draws are all the same thing.

A surface with a **grid** on it is a sprite sheet: it is cut into cells of a
size, and drawn a cell at a time by number. One without a grid is a picture, and
is drawn whole. The two are told apart by the surface itself, so there is no
mistaking which a call means:

```lua
local tiles = loadpng("tiles", 16, 16)   -- a sheet of 16x16 cells
local logo  = loadpng("logo")            -- a picture

spr(tiles, 3, 100, 50)      -- sprite 3, at 100,50
spr(logo, 10, 10)           -- the whole picture, at 10,10
```

Cells can be any size — 8x8, 16x16, whatever the artwork was drawn at — and the
same pixels can be read at more than one size, because a grid is only a way of
counting. **Sprites are numbered from zero**, left to right and then down.

| Call | What it does |
| --- | --- |
| `sprite(art, [cell_w], [cell_h])` | Reads a sprite written as text. |
| `surface(w, h, [cell_w], [cell_h])` | A blank one, every pixel transparent. |
| `loadpng(name, [cell_w], [cell_h])` | Reads a PNG and reduces it to the palette. |
| `spr(sheet, n, x, y, [w], [h], [flip_x], [flip_y], [turn])` | Sprite `n` of a sheet, spanning `w` by `h` cells. |
| `spr(picture, x, y, [flip_x], [flip_y], [turn])` | A picture, whole, at a place. |
| `spr(n, x, y, [w], [h], [flip_x], [flip_y], [turn])` | Sprite `n` of the current sheet. |
| `sspr(surface, sx, sy, sw, sh, dx, dy, [dw], [dh], [flip_x], [flip_y], [turn])` | A rectangle of **pixels**, stretched. |
| `sspr(sx, sy, sw, sh, dx, dy, …)` | The same, from the current sheet. |
| `usesheet([s])` | Makes `s` the current sheet; reports the one it replaced. |
| `sget(x, y)`, `sset(x, y, [c])` | Pixels of the current sheet. |
| `fget(n, [bit])`, `fset(n, …)` | Sprite flags of the current sheet. |
| `target([s])` | Sends later drawing to a surface, or back to the screen. |
| `s:grid()` | The cell size and how many cells, or `0, 0, 0` for a picture. |
| `s:grid(w, [h])` | Cuts it into cells, and hands the surface back. |
| `s:sprite(n, [w], [h])` | One sprite as a surface of its own. |
| `s:flags(n, [mask])` | The eight flags of one of its sprites. |
| `s:width()`, `s:height()`, `s:size()` | How big it is. |
| `s:get(x, y)`, `s:set(x, y, [c])` | One pixel, without the camera or the clip. |
| `s:fill([c])`, `s:clone()` | All of it. |

`w` and `h` in `spr` are counted in **cells**, not pixels: `spr(sheet, 3, x, y, 2, 2)`
draws the four cells whose top left is sprite 3. `sspr` is the one that works in
pixels, and it does not care whether the surface has a grid.

`s:sprite(n)` is a **copy** of that cell, so drawing on it leaves the sheet
alone.

`turn` mirrors a sprite across its own diagonal — rows become columns. With the
two flips it makes all eight ways of putting a sprite down, which is what a map
has always been able to ask for: it is a tile's `flipd`, and a tile object's.
`turn` with `flip_x` is a quarter turn clockwise; with `flip_y`, anticlockwise.

### Sprite flags

Every sprite carries eight flags, which mean whatever a game decides they
mean — solid, water, deadly, a thing to pick up.

| Call | What it does |
| --- | --- |
| `fget(n)` | All eight flags of sprite `n`, as a number. |
| `fget(n, bit)` | One of them, as true or false. `bit` is 0 to 7. |
| `fset(n, mask)` | Sets all eight. |
| `fset(n, bit, on)` | Sets or clears one. |
| `s:flags(n)`, `s:flags(n, mask)` | The same, for a sheet that is not the current one. |

```lua
local SOLID = 0
if fget(mget(x, y), SOLID) then ... end
```

**Flags come with the artwork.** A sheet drawn in an editor keeps them in the
PNG itself, in a text chunk the picture's own decoder ignores, together with
the size of its tiles — so `loadpng("tiles")` comes back already cut up and
already flagged, with nothing said in the program at all:

```lua
local tiles = loadpng("tiles")   -- grid and flags both come from the file
usesheet(tiles)
```

A Tiled tileset beside the picture — `tiles.tsj` next to `tiles.png` — is read
as well, where the flags are boolean properties called `flag_0` to `flag_7`.
That is the same information written where other tools can see it. Both are
read: for a sprite they both mention the picture has the last word, since that
is where a sheet editor keeps the truth and the tileset is what it exports, and
a sprite only the tileset knows about keeps what it says. A cell size given to
`loadpng` wins over both.

This is the shape [fz](https://github.com/gh0st42/fz) writes, whose `fz gfx`
editor draws a sheet, sets its flags, and exports the tileset.

### What else a tile carries

Eight flags are enough for solid, water and deadly, and not enough for a word or
a number. A Tiled tileset lets a tile carry properties of any kind, and those
come along with the artwork as the flags do.

| Call | What it does |
| --- | --- |
| `s:prop(n, key, [missing])` | One property of sprite `n`, or `missing` when it has none. |
| `s:props(n)` | All of them, as a table. |
| `s:props()` | What the sheet says for every sprite. |
| `s:setprop(n, key, value)` | Writes one, over whatever the artwork said. |

```lua
local tiles = loadpng("tiles")   -- tiles.tsj beside it is read as well
if tiles:prop(n, "material") == "ice" then slide() end
if tiles:prop(n, "damage", 0) > 0 then hurt() end
```

A sprite's own properties beat the ones the tileset carries for the whole sheet,
which is what makes a sheet-wide default worth setting. `flag_0` to `flag_7` are
properties too, so a flag can be asked for by name as well as by bit — the bits
are the ones a game loop should use.

Properties are for what a tile *is*. Flags are a mask and a bit test; a property
is a table lookup and a string compare, which is fine once for a tile the player
is standing on and wasteful for every tile on the screen.

### The current sheet

`usesheet()` puts one sheet in hand, and then `spr(n, x, y)` is Picotron's own
call, with no sheet to name:

```lua
usesheet(loadpng("tiles", 8, 8))
spr(3, 100, 50)
sspr(0, 0, 16, 16, 40, 40)
sset(4, 9, 12)            -- and the sheet's own pixels
```

A number as the first argument means the current sheet; a surface means that
surface. There is one current sheet, as there is on Picotron, and asking for a
sprite before there is one says so.

The call is `usesheet` rather than `sheet` because `sheet` is what anyone would
call the variable holding one, and a program that did would lose the call at the
moment it needed it.

### Art written out as text

Art is one character a pixel: `0`-`9` and `a`-`f` for colours 0 to 15, `.` or a
space for the transparent parts. The indentation the lines share is ignored, so
it can sit indented in the program:

```lua
local coin = sprite[[
	..aaaa..
	.a9999a.
	a99aa99a
	a9a77a9a
	a9a77a9a
	a99aa99a
	.a9999a.
	..aaaa..
]]
```

Colour 0 is transparent to begin with; `palt` changes that. Drawing is always
nearest-neighbour, so a sprite scaled up stays square-edged.

An animation is frames side by side in one sheet, drawn either as sprite numbers
or with `sspr`; see `examples/pico/sprites.lua` and `examples/pico/sheets.lua`.

### Maps

A level can be written out in the program, or drawn in an editor and loaded.

**Written out**, it is a list of rows, each a table of sprite numbers or a
string of hex digits:

```lua
map(cells, sheet, [x], [y], [tile_w], [tile_h], [draw_zero])
```

Tiles are the size of the sheet's own cells unless told otherwise, and **sprite
0 is not drawn** unless the last argument asks for it — which is what lets a `0`
in a level mean open sky, as long as the first cell of the sheet is blank.

**Drawn in an editor**, it is a Tiled map, and loading it brings everything it
needs with it:

```lua
usemap(loadmap("level1"))     -- the .tmj, its tilesets, and their pictures
map()                          -- the whole thing, at the origin
map(tx, ty, sx, sy, tw, th)    -- a window of it, put where you like
```

| Call | What it does |
| --- | --- |
| `loadmap(name)` | Reads a map and the artwork it draws with. |
| `usemap([m])` | Makes it the current map; reports the one it replaced. |
| `map(…)` | Draws the current map, as above. |
| `map(…, flags)` | Only sprites carrying one of those flags. |
| `mget(x, y, [layer])` | The sprite in a square. |
| `mset(x, y, n, [layer])` | Puts one there. |
| `m:size()`, `m:tile()` | The map in cells, and a cell in pixels. |
| `m:layers()` | Their names, in order. |
| `m:layer(name_or_number)` | One of them; layers are numbered from one. |
| `m:objects(name_or_number)` | What was placed on a layer, as plain tables. |
| `m:props()` | What the map itself was labelled with. |
| `m:sheet([n])`, `m:sheets()` | The artwork it draws with, counted from one. |
| `m:draw(…)` | Draws it without making it current. |
| `layer:draw(…)`, `layer:get/set`, `layer:visible(…)` | One layer on its own. |

The seventh argument to `map()` is a flag mask, as it is on PICO-8: only sprites
carrying one of those flags are drawn. That is how one layer of artwork becomes
two of scenery — everything solid drawn over everything else — without the map
having to be split.

An object is a plain table, holding everything the editor knows about it:

| Field | |
| --- | --- |
| `id` | The number the editor gave it, unique in the map. |
| `name`, `class` | What it is called, and what kind of thing it is. |
| `x`, `y`, `w`, `h` | Where it is and how big. |
| `rotation` | In turns, like every other angle here. Tiled writes degrees. |
| `visible` | Whether it was left switched on. |
| `shape` | `"rect"`, `"ellipse"`, `"point"`, `"polygon"` or `"polyline"`. |
| `points` | The corners of an outline, relative to `x`, `y`. Nothing otherwise. |
| `props` | What it was labelled with, each value the kind the editor gave it. |

```lua
for thing in all(level:objects("things")) do
	if thing.class == "start" then player.x, player.y = thing.x, thing.y end
end
```

An object that was given a **tile** in the editor — a lamp, a crate, scenery
placed by hand rather than drawn into a layer — also carries `sprite`, the
`sheet` it belongs to, and `flipx`, `flipy` and `flipd`. An object that is only
a region has none of those fields, which is how to tell them apart. Tiled
anchors such an object at its bottom left, so `y` is the foot of the sprite:

```lua
for thing in all(level:objects("things")) do
	if thing.sprite then
		local _, th = level:tile()
		spr(level:sheet(thing.sheet), thing.sprite, thing.x, thing.y - th,
			1, 1, thing.flipx, thing.flipy)
	end
end
```

**Collision comes from the artwork**, not from a list of tile numbers:

```lua
local SOLID = 0
if fget(mget(x, y), SOLID) then ... end
```

What is read: CSV and base64 layers, packed with zlib, gzip or nothing; several
tilesets in one map; tile layers and object layers; and the flip bits Tiled
keeps in the top of a tile number, which between them make all eight ways of
putting a tile down — mirrored either way, and turned. `examples/pico/cellar` is
a worked example, folder and all.

What is not: Zstandard compression, a map saved as infinite, a group layer
(which is skipped along with the layers inside it), and a tileset embedded in
the map rather than saved beside it — in Tiled, *Map → Convert Tileset*, or
untick "Embed in map" when the tileset is made. The first three say so; the last
says the map names no tileset it can find.

Inside a map, sprite 0 is nothing: it is what the editor means by an empty
square and what this console means by the blank first cell of a sheet, which is
why that cell is left blank.

## Input

| Call | What it does |
| --- | --- |
| `btn([button], [player])` | Whether a button is held; with nothing, whether anything is. |
| `btnp(button, [player])` | Whether it has just been pressed, or is repeating. |
| `held(button, [player])` | How many ticks it has been held. |
| `key([name])`, `keyp(name)` | The keyboard directly, by name. |
| `mouse()` | x, y, buttons, wheel. |
| `mousebtn([button], [pressed])` | 1 left, 2 right, 3 middle. |
| `btnkey(button, [player])` | What the key that works a button is called on this keyboard. |
| `typed()` | What was typed this tick. |

Buttons are 0 to 5: left, right, up, down, O and X, and each can be named
instead — `btn("left")`. Players are 0 to 3.

A held button repeats a quarter of a second after it goes down and four times a
second after that, which is what a menu written against `btnp` expects.

The keyboard is two pads: player one has the arrow keys with Z, X, C and V;
player two has E, S, D, F with shift, A, Q and tab. Pads plugged in are players
one to four.

**Those are places on the keyboard, not what is printed on the keys.** A German
keyboard has Y where an American one has Z, so the key under a left hand says
something different — and both of those places work the O button for that
reason. A program telling somebody which key to press should ask rather than
guess:

```lua
print("press " .. btnkey("o") .. " to jump")
```

`btnkey` gives the label from the keyboard actually in use: `Z` on one, `Y` on
another. Before the window opens, and where the system will not say, it gives
the name of the place instead.

Key names are the plain ones — `"left"`, `"space"`, `"escape"`, `"enter"`,
`"tab"`, `"f1"`, a single letter or digit — and `"shift"`, `"ctrl"`, `"alt"`
and `"cmd"` mean either of the pair.

`mouse()` reports where the pointer is **in console pixels**, whatever size the
window is, and a pointer off the picture reads as a negative number.

## Time, and the window

| Call | What it does |
| --- | --- |
| `t()`, `time()` | Seconds since the program started. |
| `frame()` | How many ticks have run. |
| `fps()` | What the window is managing. |
| `setfps(n)` | How many times a second to run the program. 60 unless set. |
| `printh(...)` | Writes to the terminal. |
| `exit([status])` | Closes the window. |
| `window{...}` | `title`, `scale`, `fullscreen`, `width`, `height`. |
| `fullscreen([on])` | On or off. |

`t()` counts ticks rather than reading a clock, so motion stays in step with
what is drawn however the machine is behaving. `setfps(n)` changes how often
the program is run — `_update` and `_draw` are called that often, and `t()`
counts seconds by it, so half a second is half a second at any rate.
`setfps(-1)` runs once for every refresh of the screen, whatever that turns out
to be. How often the window itself is refreshed is the screen's business, and
`fps()` is what that turned out to be.

While a program runs, the window keeps three keys for itself:

| Key | |
| --- | --- |
| `alt-enter`, `F11` | fills the screen, and back |
| `ctrl-D` | shows the frame rate, and what it is being drawn at |
| `ctrl-Q` | closes the window |

The program is not shown those keys, so a game that reads enter, or D as a
direction, does not act on them as well. `ctrl-C` in the terminal the program
was started from closes the window too.

The frame rate counter is drawn over the picture rather than into it, so it
cannot be read back by `pget()`, cannot smear into a program that does not
clear the screen, and is not recoloured by `palette()`.

## Saving

A game keeps what it wants to survive being closed: a high score, where the
player had got to, what they chose.

| Call | What it does |
| --- | --- |
| `store(name, value)` | Saves it. `true`, or `nil` and why not. |
| `fetch(name)` | Reads it back — the value that was saved, or the text of a file the game shipped with. |

```lua
store("scores", { 1200, 900, 80 })
local scores = fetch("scores") or {}
```

A table is written out as text, laid out the way it would be typed, so a save
file can be read, fixed and kept in version control. Saving the same thing twice
gives the same bytes. Strings, numbers, truths and tables are what can be
saved; a function or a table that holds itself says so rather than being
silently dropped.

**Where it goes** is the host's business, not the program's: with the person's
other application data — `~/Library/Application Support/tlua/saves/<game>` on a
Mac, `~/.config/tlua/saves/<game>` on Linux, `AppData` on Windows — because a
game may be run from a folder nobody can write to, from a read-only disk, or as
a single fused executable with no folder at all. A name is a name, not a path:
`store("dir/name", …)` is refused.

**`fetch` looks at saves first**, and at what the game shipped with after, so a
game can ship its defaults and have the player's own version take over once
there is one:

```lua
local settings = fetch("settings")   -- theirs, or the one shipped, or nil
```

A saved file is **parsed, not run**. It is a file on somebody's disk that a
program is about to trust, and running it would make a save file a place to put
code. A file that has been edited into nonsense comes back as `nil` and a
complaint naming the line.

## The short helpers

These are the names PICO-8 and Picotron programs are written with. They exist
here so that such a program reads the way it was written.

**Numbers.** `flr` `ceil` `abs` `sqrt` `sgn` `min` `max` `mid` `clamp` `rnd`
`srand` `sin` `cos` `atan2`

- Angles are **turns**: a whole circle is 1.
- `sin` runs the same way round as the screen's y axis, which is downwards, so
  that `x + cos(a)` and `y + sin(a)` move a thing where the numbers say.
  `atan2(dx, dy)` gives back the turn those two came from.
- `sgn(0)` is 1, and `sqrt` of a negative number is 0, both as on PICO-8.
- `min(x)` and `max(x)` compare against 0. `mid(a, b, c)` is the middle of
  three, and `clamp(x, lo, hi)` is the same thing said plainly: it keeps a
  number between two others, whichever way round they are given.
- `rnd()` is below 1, `rnd(n)` below n, `rnd(table)` one of the things in it.

**Lists.** `add` `del` `deli` `all` `foreach` `count`

`add(list, v)` reports `v` back, so a thing can be made and used in one line.
`all(list)` is the usual loop: `for e in all(enemies) do`.

**Strings.** `sub` `split` `tostr` `tonum` `chr` `ord`

`split("1,2,3")` is a table of numbers; a second argument is the separator and
a third of `false` leaves the parts as strings.

**Coroutines.** `cocreate` `coresume` `costatus` `yield`

These are Lua's own under the names this lineage calls them by: `cocreate` is
`coroutine.create`. What they are for is a piece of work that takes many frames
and reads better written straight through — a cutscene, a path being walked, a
level built a little at a time:

```lua
local job = cocreate(function()
	for i = 1, 100 do build_a_bit(i) yield() end
end)

function _update()
	if costatus(job) ~= "dead" then coresume(job) end
end
```

`flip()` is not for these. It suspends the program's own loop, and a coroutine
of a program's own is not that loop; inside one, use `yield`. A program that
tries is told so rather than behaving strangely.

Lua's own libraries are all still there — `math`, `string`, `table`, `io`,
`os`, `require` — since this is the same interpreter as `tlua` itself.
`require` finds modules beside the script, and in `TLUA_INCLUDE` and
`LUA_PATH`, exactly as it does elsewhere.

## What things cost

A frame has a sixtieth of a second in it: 16.6 milliseconds. Measured on an
M1 laptop, at 480x270:

| | |
| --- | --- |
| clearing the screen | 0.04 ms |
| 500 sprites of 16x16, at their own size | 0.14 ms |
| a screen of 8x8 tiles through `map()` | 0.18 ms |
| the frame reaching the window | 0.14 ms |
| a whole frame of the bigger examples | 0.2 to 0.4 ms |

So the drawing is not usually what a program has to watch; the Lua around it
is. Per-pixel work is the thing that costs: `pset` called for every pixel of a
480x270 screen is 130,000 calls a frame, which the language will not do in the
time available. Two ways out, in the order to try them:

- **Ask for a smaller screen.** `vid(4)` is 160x90, a ninth of the pixels, and
  the window scales it back up. `examples/pico/plasma.lua` does this.
- **Hold the calls you use in locals**: `local sin, pset = sin, pset` at the top
  of the file. A global is a lookup by name in a table, and a loop making six of
  them per pixel spends about a tenth of its time doing that.

Everything the console itself does is free of allocation, so what a program
gives the garbage collector to do is its own.

## What is different

- **Sound is files.** `sfx` and `music` play a .wav or an .ogg by name; there
  is no tracker, no sfx editor, and no instruments.
- **No cartridge**, no sprite or map editor, and no `poke`/`peek`: there is no
  memory layout to poke at, and artwork is written as text in the program.
- **Colours 32 to 63 are not Picotron's**, as above. The video modes are, apart
  from `vid(13)`.
- `color(c, c2)` takes the fill pattern's second colour as its own argument
  rather than packing two colours into one number, because 64 colours do not
  fit in a nibble each.
- Triangles, `held()`, `typed()`, `textwidth()`, `clamp()`, `loadpng()`,
  `fetch()`, `volume()`, `usesheet()`, `setfps()`, the whole of `palette()`,
  and looking a resource up by name are additions.
- A sheet's cells are a uniform grid. Picotron's sprites can each have their own
  size, which is a property of its `.gfx` files rather than of a PNG.

## Shipping a game

`tlua fuse -play` attaches a program to a copy of the binary and marks it as one
that wants a window — or the program says `boot()` itself and needs no flag. What comes out is a single executable with nothing beside
it: no interpreter to install, no files to keep together, no Lua on the machine
it runs on.

```sh
tlua fuse -play -o mygame mygame/     # a directory with main.lua in it
./mygame
```

It takes the same three shapes `tlua fuse` always did — one `.lua` file, a
directory, or a zip — and `require` inside the program reads modules out of the
attached archive, so a game split across files ships as one thing.

Whatever the executable is started with is handed to the program, as it is for
a `.love` file, so a fused game reads `arg` the way any other program does. The
window is named after the executable until `window{title=}` says otherwise.

Without `-play` the program is fused as an ordinary script, with no console API
in it: `cls` would be a nil value. Fusing notices a program that defines
`_draw` or `_update` and says so rather than leaving that to be discovered at
the first drawing call.

Building for another machine is what `--base` is for: cross-compile tlua for it
first, then fuse against that binary.

```sh
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o tlua.exe ./cmd/tlua
tlua fuse -play --base tlua.exe -o mygame.exe mygame/
```
