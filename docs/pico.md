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

It is a window and a framebuffer, nothing more: no sound, no cartridges, no
editor of sprites or maps. Artwork is written out as text in the program
itself, which is why every example ships as a single file.

- [examples/pico](../examples/pico) has twelve programs written against this.
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

## The screen

480 by 270 pixels, one byte a pixel, so 256 colours at the very most and 64 to
begin with. The window scales whatever it is given up to fit, by a whole
number, with black around the edges.

`vid(mode)` switches resolution:

| Mode | Size | |
| --- | --- | --- |
| `vid(0)` | 480x270 | the console's own |
| `vid(1)` | 240x135 | half of it |
| `vid(2)` | 160x90 | a quarter, and enough to work out every pixel in Lua |
| `vid(13)` | 320x200 | what a VGA card called mode 13h |

`vid()` on its own says which mode is in use and how big it is; a size asked
for some other way reports as mode -1. `window{width=, height=}` still takes
any size at all, and `screen()` reports it.

Only the fourth of those is Picotron's; mode 13 is here because a great deal of
pixel art was drawn at 320x200, and because it goes with the VGA palette below.

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
| `textwidth(text)`, `textheight(text)` | How big it will be. |

Characters are three pixels wide and five tall, on a grid of four by six.
Lower case is drawn as small capitals, four rows tall, as PICO-8's font does:
three pixels is not enough to tell `a` from `A` any other way.

`print` with no position draws at the cursor and moves it down a line, so
several in a row stack up. `\n` inside the text starts a new line under the
first.

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

A surface is a rectangle of palette indices. Sprites, sheets and anywhere else
a program draws are all the same thing.

| Call | What it does |
| --- | --- |
| `sprite(art)` | Reads a sprite written as text. |
| `surface(w, h)` | A blank one, every pixel transparent. |
| `loadpng(path)` | Reads a PNG and reduces it to the palette. |
| `spr(s, x, y, [flip_x], [flip_y])` | Draws it at its own size. |
| `sspr(s, sx, sy, sw, sh, dx, dy, [dw], [dh], [flip_x], [flip_y])` | Draws part of it, stretched. |
| `target([s])` | Sends later drawing to a surface, or back to the screen. Reports what it replaced. |
| `s:width()`, `s:height()`, `s:size()` | How big it is. |
| `s:get(x, y)`, `s:set(x, y, [c])` | One pixel, without the camera or the clip. |
| `s:fill([c])`, `s:clone()` | All of it. |

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

An animation is frames side by side in one surface, drawn with `sspr`; see
`examples/pico/sprites.lua`.

### Maps

```lua
map(cells, sheet, [x], [y], [tile_w], [tile_h])
```

`cells` is a list of rows, each one a table of tile numbers or a string of hex
digits. 0 is nothing at all, and 1 is the first tile of the sheet, counting
left to right and then down. Tiles are 8 by 8 unless told otherwise.

`examples/pico/platformer.lua` writes its level out as text, turns it into
numbers once, and then uses that same grid both to draw with and to walk on.

## Input

| Call | What it does |
| --- | --- |
| `btn([button], [player])` | Whether a button is held; with nothing, whether anything is. |
| `btnp(button, [player])` | Whether it has just been pressed, or is repeating. |
| `held(button, [player])` | How many ticks it has been held. |
| `key([name])`, `keyp(name)` | The keyboard directly, by name. |
| `mouse()` | x, y, buttons, wheel. |
| `mousebtn([button], [pressed])` | 1 left, 2 right, 3 middle. |
| `typed()` | What was typed this tick. |

Buttons are 0 to 5: left, right, up, down, O and X, and each can be named
instead — `btn("left")`. Players are 0 to 3.

A held button repeats a quarter of a second after it goes down and four times a
second after that, which is what a menu written against `btnp` expects.

The keyboard is two pads: player one has the arrow keys with Z, X, C and V;
player two has E, S, D, F with shift, A, Q and tab. Pads plugged in are players
one to four.

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
| `printh(...)` | Writes to the terminal. |
| `exit([status])` | Closes the window. |
| `window{...}` | `title`, `scale`, `fullscreen`, `width`, `height`. |
| `fullscreen([on])` | On or off. |

`t()` counts ticks at sixty a second rather than reading a clock, so motion
stays in step with what is drawn, however the machine is behaving.

While a program runs, `F11` or alt-enter fills the screen, `ctrl-Q` closes it,
and `ctrl-C` in the terminal does too.

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

Lua's own libraries are all still there — `math`, `string`, `table`, `io`,
`os`, `require` — since this is the same interpreter as `tlua` itself.
`require` finds modules beside the script, and in `TLUA_INCLUDE` and
`LUA_PATH`, exactly as it does elsewhere.

## What is different

- **No sound.** Neither `sfx` nor `music` exists.
- **No cartridge**, no sprite or map editor, and no `poke`/`peek`: there is no
  memory layout to poke at, and artwork is written as text in the program.
- **No `flip()`.** A program is built from `_update` and `_draw`; it cannot
  draw from inside a loop of its own.
- **Colours 32 to 63 are not Picotron's**, as above, and neither are the video
  modes: Picotron's own numbering is not reproduced, only the idea.
- `color(c, c2)` takes the fill pattern's second colour as its own argument
  rather than packing two colours into one number, because 64 colours do not
  fit in a nibble each.
- Triangles, `held()`, `typed()`, `textwidth()`, `clamp()`, `loadpng()` and the
  whole of `palette()` are additions.

## Shipping a game

`tlua fuse` attaches a program to a copy of the binary, so that a script and
its modules ship as one executable. A fused program runs as an ordinary script
though: the console API is not installed in it, and `cls` would be a nil value.

So a game is shipped as its files plus a tlua binary, and started with
`tlua play`. Making `tlua fuse` able to produce a game that opens its own
window is a small change to the payload format and an obvious next step, but it
is not there yet.
