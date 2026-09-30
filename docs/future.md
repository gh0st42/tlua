# What is not here yet

Things that have been looked at, decided against for now, and are worth doing
later. Each says what it is, why it matters, and where in the code it would go,
so that picking one up does not start with the investigation again.

Nothing here is a bug. What is missing and *wrong* gets fixed; what is missing
and merely absent is written down here.

## The console API

### A pause menu, and `menuitem()`

Picotron and PICO-8 let a program hang a few entries on the menu the console
opens over a paused game: restart, sound off, back to the title. There is no
pause menu here at all, so there is nowhere to hang them.

It is two pieces: the host pausing the program and drawing a list over the
frozen picture ([internal/game/app.go](../internal/game/app.go), beside the
frame-rate overlay, which already draws over a frame without touching it), and
`menuitem(index, label, fn)` collecting what the program wants on it
([internal/picolua/sys.go](../internal/picolua/sys.go)).

The overlay is the part to copy: it draws after the console's own picture has
been blitted, so a paused menu would not be readable by `pget()` or recoloured
by `palette()`, which is what it should be.

### `tline()`

PICO-8's textured line: draw a line across the screen taking its colours from a
map, stepping through it as the line goes. It is what mode-7 floors, racing
games and small raycasters are built out of, and nothing here replaces it —
`line()` takes one colour and `sspr()` cannot skew.

It belongs beside the other primitives in
[internal/pico/draw.go](../internal/pico/draw.go) and would walk the line in
screen space while stepping a fixed-point position through a surface, which is
the same walk `Blit` already does for the scaling case.

### Working on many pixels at once

Per-pixel work in Lua is bounded by the interpreter, not by the console:
profiling put about 80% of the time and 73% of the allocations in gopher-lua's
own VM for a program that calls `pset` for every pixel. `vid(4)` and holding the
calls in locals are the two ways out today, and both are in the docs, but
neither changes the shape of the problem.

Picotron's answer is `userdata`: a typed array with its own arithmetic, so that
a whole row or a whole picture is one call rather than a hundred thousand. Our
`Surface` is the sprite half of that. The other half would be operations that
run in Go over a whole surface — a row written from a table, two surfaces
combined, a column read out — so that an effect is a few dozen calls a frame
instead of a call a pixel.

This is the one item here that would change what can be written, rather than
adding something that can already be worked around.

## Files other tools write

The Tiled reader ([internal/pico/tmj.go](../internal/pico/tmj.go)) covers what
a map editor writes by default and what [fz](https://github.com/gh0st42/fz)
writes. What it does not:

- **A tileset embedded in the map.** Tiled embeds a new tileset unless told to
  save it beside the map, and only `firstgid` and `source` are read, so an
  embedded one is refused with "the map names no tileset it can find". The
  whole tileset object is right there in the JSON; reading it is a matter of
  parsing the same fields `ReadTSJ` already parses and finding the image
  relative to the map rather than to the tileset. fz has the same hole.
- **Infinite maps.** Their layers hold `chunks` rather than `data`, each with
  its own origin. The reader says "reading the tiles: unexpected end of JSON
  input", which is the right outcome with the wrong words. Reading them means
  deciding what the origin of a map with negative coordinates is.
- **Group layers**, which are dropped along with every layer inside them. They
  nest, so `ReadTMJ` would have to walk them rather than loop once; the
  question worth settling first is whether a group is a layer a program can ask
  for by name or just a folder in the editor.
- **Zstandard compression**, which needs a dependency. It says so rather than
  looking corrupt, which is enough for now.
- **`spacing` and `margin`** on a tileset: a sheet with gaps between its tiles
  is read as though it had none. `Surface.Cell` assumes a packed grid. fz
  ignores them too.
- **Tile animations** in a `.tsj`, and **XML** `.tmx`/`.tsx`.

fz's own sound formats are the other half: `assets/sfx/*.json` is a set of sfxr
parameters and `assets/bgm/*.json` is its song format. A game plays the `.wav`
fz renders beside them, which is enough; reading the sources means porting a
synthesiser and a sequencer, which is a project rather than a gap.

## Smaller things

- **`s:sprite(n)` is a copy.** A window onto the sheet's own pixels would save
  the copying and let `target()` draw into a cell, but it would mean teaching
  every drawing loop about a row length that is not the surface's width. The
  note is in [internal/picolua/surface.go](../internal/picolua/surface.go),
  where the call is.
- **`print` takes no control codes.** PICO-8's P8SCII lets a string change
  colour part way through. Here a colour is an argument.
- **No `vec`.** Picotron has a vector type with arithmetic on it. Tables and
  two numbers are what the examples use.
