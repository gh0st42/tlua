# `png` — pictures

`png` reads pictures from PNG, JPEG, GIF and WebP files, makes them and
changes them pixel by pixel, and writes them as PNG. It is tlua's own, for
any program: the console and the gui use it, and so can a script that only
makes a thumbnail. A gui Canvas draws a picture with `g:image(img, x, y)`
(see [gui](../gui.md)); `examples/gui/paint.lua` is a small pixel editor
built on it.

```lua
local png = require "png"
```

A picture has a `width` and a `height` in pixels, and each pixel is four
numbers, `r, g, b, a`, from 0 to 255. Coordinates count from 0, from the
top left. A colour can also be written `"#rrggbb"` or `"#rrggbbaa"`.

## Reading and writing

### `png.load(path)`

The picture in a file, or `nil, why`. In a fused program or a bundle the
program's own files are read first. It reads PNG, JPEG, GIF and WebP; of an
animation it gives the first frame (see `png.frames`).

```lua
local img, why = png.load("photo.jpg")
if not img then error(why) end
print(img.width, img.height)
```

### `png.decode(data)`

The same, from the file's bytes as a string.

```lua
print(png.decode("garbage"))        --> nil   image: unknown format
```

### `img:save(path)`

Writes a PNG file: `true`, or `nil, why`.

### `img:encode()`, `png.encode(img)`

The picture as the bytes of a PNG file, as a string: to send over a socket,
or to put in a [zip](zip.md).

## Making

### `png.new(width, height [, color])`

A picture all of one colour, transparent unless one is given.

```lua
local img = png.new(4, 2, "#ff0000")
print(img:get(0, 0))                --> 255   0   0   255
```

### `png.fromPixels(width, height, rgba)`

A picture from `width * height * 4` bytes of `r, g, b, a`, a row at a time
from the top, as `img:pixels()` gives them.

## Looking and changing

### `img:get(x, y)`

The colour at `x, y` as `r, g, b, a`; nothing for a point outside the
picture.

### `img:set(x, y, r, g, b [, a])`, `img:set(x, y, "#rrggbb")`

Sets a pixel; a point outside the picture is left alone.

```lua
img:set(1, 0, 0, 255, 0)            -- green
img:set(2, 0, "#0000ff80")          -- half-transparent blue
print(img:get(2, 0))                --> 0   0   255   128
```

### `img:fill(color)`, `img:fill(color, x, y, w, h)`

Fills all of the picture, or a rectangle of it.

### `img:pixels()`

All the pixels as a string of `width * height * 4` bytes.

### `img:clone()`

A copy that can be changed without changing the first.

## New pictures from old

These make a new picture and leave the first as it was.

### `img:crop(x, y, w, h)`

The part of the picture at `x, y` that is `w` by `h`.

### `img:rotate(turns)`

Turned clockwise by that many quarter turns; a negative number turns it the
other way.

```lua
local r = png.new(4, 2):rotate(1)
print(r.width, r.height)            --> 2   4
```

### `img:flip(axis)`

Mirrored left to right (`"horizontal"`) or top to bottom (`"vertical"`).

## Animations

### `png.frames(path)`, `png.decodeFrames(data)`

The frames of a picture that moves (GIF, APNG or WebP), or of one that does
not, as a list of one. Each frame is a table with:

- `image`: the whole picture as it looks then, so a frame can be shown
  without the ones before it;
- `delay`: how long it stays, in seconds (0 for a picture that does not
  move; a delay of a hundredth of a second or less is given as a tenth, as
  browsers show it).

The list also has the picture's `width` and `height`, `loops` (how many
times it plays in all, 0 for ever) and `truncated`. The frames take 4 bytes
a pixel each, so an animation that would need more than 256 MB gives its
first frame only, and `truncated` is true.

```lua
local frames = png.frames("spinner.gif")
for i, frame in ipairs(frames) do
  print(i, frame.delay, frame.image.width)
end
```
