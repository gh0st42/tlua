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
```

| Example | What it is for |
| --- | --- |
| [hello.lua](hello.lua) | The smallest program: `cls`, `print`, `circfill`, and `t()` for motion. |
| [palette.lua](palette.lua) | All 64 colours with their numbers. Worth keeping open while writing anything else. |
| [shapes.lua](shapes.lua) | Every drawing call, and what the dither patterns of `fillp` do. |
| [sprites.lua](sprites.lua) | Sprites written as text: drawing, flipping, stretching, recolouring, transparency. |
| [bounce.lua](bounce.lua) | A list of things that move: `add`, `all`, and the shape of most games here. |
| [input.lua](input.lua) | Buttons, keys, the mouse and the wheel, all shown as they are pressed. |
| [starfield.lua](starfield.lua) | Three hundred stars, one `pset` each, and perspective by division. |
| [plasma.lua](plasma.lua) | Per-pixel drawing, and `window{}` asking for a smaller screen to afford it. |
| [snake.lua](snake.lua) | A whole game: a grid, a score, a title screen and an ending. |
| [platformer.lua](platformer.lua) | A tile map with `map()`, gravity, collision, and a camera that follows. |
| [paint.lua](paint.lua) | Drawing with the mouse onto an offscreen surface with `target()`. |

Two of them are worth reading before writing anything: `hello.lua` for the
shape of a program, and `sprites.lua` for how artwork is written without any
files.

## Playing

The keyboard is two pads. Player one has the arrow keys with Z, X, C and V
beside them; player two has E, S, D and F with shift, A, Q and tab. Any game
pads plugged in are players one to four. `F11` or alt-enter fills the screen,
`ctrl-Q` quits, and `ctrl-C` in the terminal closes the window too.
