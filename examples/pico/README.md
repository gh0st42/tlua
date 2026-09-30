# Console examples

Programs for the fantasy console `tlua play` provides: a 480x270 screen, 64
colours, sprites written out as text, and a program built from `_update()` and
`_draw()`. [docs/pico.md](../../docs/pico.md) is the API these are written
against.

Each one is a single file with nothing alongside it — no images, no data, no
dependencies — so any of them can be copied somewhere else and changed.

```sh
tlua play examples/pico/snake.lua
tlua play -scale 3 examples/pico/starfield.lua
tlua play -fullscreen examples/pico/plasma.lua

tlua fuse -play -o snake examples/pico/snake.lua   # one executable, no tlua
./snake
```

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
| [plasma.lua](plasma.lua) | Per-pixel drawing, and `vid(2)` asking for a smaller screen to afford it. |
| [snake.lua](snake.lua) | A whole game: a grid, a score, a title screen and an ending, with the best score kept between runs by `store` and `fetch`. |
| [platformer.lua](platformer.lua) | A tile map with `map()`, gravity, collision, and a camera that follows. |
| [paint.lua](paint.lua) | Drawing with the mouse onto an offscreen surface with `target()`. |
| [modes.lua](modes.lua) | `vid()`: the same picture at each of the six resolutions. |
| [sound.lua](sound.lua) | `sfx()` and `music()`, and where they look for what you ask for. |
| [dialog.lua](dialog.lua) | `flip()`: a modal dialog that runs its own loop, and the game waiting where it stood. |

And one that is a folder rather than a file, because a level drawn in an editor
is three of them:

| Example | What it is for |
| --- | --- |
| [cellar/](cellar/) | A Tiled map with its tileset and artwork: `loadmap`, `mget`, sprite flags for collision, tile properties for what a tile is made of, and objects placed in the editor — including one that is a sprite. Run it with `tlua play examples/pico/cellar`. |

Two of them are worth reading before writing anything: `hello.lua` for the
shape of a program, and `sprites.lua` for how artwork is written without any
files. `palette.lua` is worth keeping open beside whatever is being written,
for the colour numbers.

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
