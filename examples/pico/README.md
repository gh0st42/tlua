# Console examples

Programs for the fantasy console: a 480x270 screen, 64 colours, sprites written
out as text, and a program built from `_update()` and `_draw()`.
[docs/pico.md](../../docs/pico.md) is the API these are written against.

Each one is a single file with nothing alongside it — no images, no data, no
dependencies — so any of them can be copied somewhere else and changed. The one
exception is `cellar/`, which is a folder because a level drawn in an editor is
three files.

## Running one

Every example here starts with `boot()`, which is how a program says it wants a
window and the console. That one line means there is nothing special about
running a game: it is a Lua file, and `tlua` runs Lua files.

```sh
tlua examples/pico/snake.lua         # like any other script
tlua play examples/pico/snake.lua    # the same, and where the options live
```

`tlua play` is still the way to pass the window's own options, since those
belong to whoever is running the program rather than to the program:

```sh
tlua play -scale 3 examples/pico/starfield.lua
tlua play -fullscreen examples/pico/plasma.lua
```

A folder is a program too. `tlua play` takes the folder and finds the `main.lua`
inside it; run on its own, name that file — either way the artwork beside it is
found, from whatever directory you happen to be in:

```sh
tlua play examples/pico/cellar
tlua examples/pico/cellar/main.lua
```

Because the program says `boot()` itself, `fuse` needs no flag to know what it
is making, and a file with a shebang line is a game you can run by name:

```sh
tlua fuse -o snake examples/pico/snake.lua   # one executable, no tlua
./snake
```

A program that does *not* say `boot()` needs `tlua play` or `tlua fuse -play`,
which is what those are for: the console is not installed unless something asks,
because most Lua scripts are not games and expect `print` to write to the
terminal.

| Example | What it is for |
| --- | --- |
| [hello.lua](hello.lua) | The smallest program: `cls`, `print`, `circfill`, and `t()` for motion. |
| [palette.lua](palette.lua) | Whichever palette is loaded, with the numbers; switches to the 256 VGA colours, and fades by changing them. |
| [shapes.lua](shapes.lua) | Every drawing call, rounded rectangles included, and what the dither patterns of `fillp` do. |
| [sprites.lua](sprites.lua) | Sprites written as text: drawing, flipping, stretching, recolouring, transparency. |
| [sheets.lua](sheets.lua) | Sprite sheets of any cell size, drawn by number, and the current sheet. |
| [bounce.lua](bounce.lua) | A list of things that move: `add`, `all`, and the shape of most games here. |
| [input.lua](input.lua) | Buttons, keys, the mouse and the wheel, all shown as they are pressed. |
| [starfield.lua](starfield.lua) | Three hundred stars, one `pset` each, and perspective by division. |
| [plasma.lua](plasma.lua) | Per-pixel drawing, and `vid(4)` asking for a smaller screen to afford it. |
| [fonts.lua](fonts.lua) | Both built-in fonts and one drawn in the program: `font`, and what unscii's box drawing is for. |
| [snake.lua](snake.lua) | A whole game: a grid, a score, a title screen and an ending. Keeps its best score between runs with `store` and `fetch`, and sets its panels in unscii while the score stays in the small font. |
| [platformer.lua](platformer.lua) | A tile map with `map()`, gravity, collision, and a camera that follows. |
| [paint.lua](paint.lua) | Drawing with the mouse onto an offscreen surface with `target()`. |
| [modes.lua](modes.lua) | `vid()`: the same picture at each of the six resolutions. |
| [sound.lua](sound.lua) | `sfx()` and `music()`, and where they look for what you ask for. |
| [dialog.lua](dialog.lua) | `flip()`: a modal dialog that runs its own loop, and the game waiting where it stood. Takes the bigger font for the question and puts back whatever it found. |

And one that is a folder rather than a file, because a level drawn in an editor
is three of them:

| Example | What it is for |
| --- | --- |
| [cellar/](cellar/) | A Tiled map with its tileset and artwork: `loadmap`, `mget`, sprite flags for collision, tile properties for what a tile is made of, and objects placed in the editor — including one that is a sprite. |

Two of them are worth reading before writing anything: `hello.lua` for the
shape of a program, and `sprites.lua` for how artwork is written without any
files. `palette.lua` is worth keeping open beside whatever is being written,
for the colour numbers.

## Text

Most of these draw their text in the small font the console starts with — three
pixels by five, which suits a score in a corner. `fonts.lua` shows the other
one: unscii-8, eight by eight, with real lower case, accented letters, Greek,
Cyrillic, arrows, and the box-drawing and block characters old machines drew
their panels with.

```lua
local was = font("unscii")
print("shall we begin?", x, y, 7)
font(was)                          -- back to whatever the caller was using
```

`snake.lua` and `dialog.lua` use both at once, which is the usual arrangement:
the bigger font for a sentence meant to be read, the small one for the numbers
around the edge. `textwidth()` follows whichever is in hand, so centring needs
no arithmetic of its own.

## Playing

The keyboard is two pads. Player one has the arrow keys with Z, X, C and V
beside them; player two has E, S, D and F with shift, A, Q and tab. Any game
pads plugged in are players one to four.

Those are places rather than printed letters: on a German keyboard the key in
the Z place says Y, and both of those places work the O button because of it.
The examples say which key to press by asking `btnkey()`, so what they print is
what is on the keyboard in front of you.

`alt-enter` or `F11` fills the screen, `ctrl-D` shows the frame rate, `ctrl-Q`
quits, and `ctrl-C` in the terminal closes the window too. Those keys belong to
the window, so a program never sees them.
