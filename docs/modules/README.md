# Modules

What a tlua program can use with nothing installed, and how to use it. Each
page says what the thing is for, then goes through its functions with
examples. `tlua doc <page>` shows one from the binary, `tlua doc
<page>.<name>` the part about one function, and `tlua doc -g` all of them in
a window.

## The language

- [Lua language basics](lua-language.md): Lua 5.1 in one page, and what
  code written for later versions has to do differently.
- [The standard library](lua-stdlib.md): `string`, `table`, `math`, `os`,
  `io`, `coroutine`.

## Programs

What tlua can be used to make, beyond scripts:

- [gui](../gui.md): desktop windows, controls, dialogs and drawing, on FLTK.
- [design](../design.md): `tlua design`, the form designer, and how it
  writes projects: GUI applications, components, console programs, games.
- [pico](../pico.md): the game console, `tlua play`: a fixed screen and
  palette, sprites, sound and input, and a program of `_update` and `_draw`.

## Libraries

What a program `require`s. Each is built into tlua (written in Go or in Lua)
or carried unchanged from LuaRocks, and a program's own module of the same
name is found first. Those marked **turbolua** come from turbolua, the
console toolkit tlua grew from, and are in the manner of Turbo Pascal and
QBasic.

### Files and data

- [lfs](lfs.md): LuaFileSystem 1.9: attributes, folders, links, times.
- [zlib](zlib.md): deflate, inflate, gzip and checksums, with lua-zlib's API.
- [zip](zip.md): zip archives to read and write, with AES-256 encryption in
  the form 7-Zip reads.
- [png](png.md): pictures to read (PNG, JPEG, GIF, WebP), make, change pixel
  by pixel and write as PNG.
- [markdown](markdown.md): Markdown read into blocks and written back, with
  an editor behind it.

### Text and options

Pure Lua libraries from LuaRocks, carried unchanged.

- [inspect](inspect.md) 3.1: any value, tables above all, as readable text.
- [dkjson](dkjson.md) 2.11: JSON to Lua values and back.
- [argparse](argparse.md) 0.7: command-line options, arguments,
  subcommands and `--help`.

### The network

LuaSocket 3.1 and LuaSec 1.3, whose C halves tlua carries written in Go
(Linux and macOS).

- [socket](socket.md): TCP, UDP, DNS, `select`.
- [socket.http](socket.http.md), [socket.ftp](socket.ftp.md),
  [socket.smtp](socket.smtp.md): clients for those protocols.
- [socket.url](socket.url.md): taking URLs apart and putting them
  together.
- [socket.tp](socket.tp.md): the line protocol FTP and SMTP are built on.
- [ssl](ssl.md): TLS over a socket, as a client or a server, and
  `ssl.https`.

### Console programs (turbolua)

[examples/console](../../examples/console) has a demo of each.

- [sys](sys.md) (turbolua): files and folders (read, write, copy, move,
  list, mkdir), sleep, run a command, read a line.
- [term](term.md) (turbolua): an ANSI terminal: colours, cursor, clear
  screen, keys one at a time, and `{red}inline{reset}` colour tags.
- [tui](tui.md) (turbolua): Turbo Vision-like windows, buttons, check
  boxes, input lines, lists and message boxes in the terminal, on `term`.

## Finding modules

`require` looks along `package.path`, and tlua's own modules after it, so
a program's own copy wins. The README tells how the path is made up and what
`TLUA_INCLUDE` and `TLUA_PATH` do.
