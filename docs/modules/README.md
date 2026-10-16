# Modules

What a tlua program can `require` with nothing installed, beyond the
standard library. Each page says what the module is for, then goes
through its functions with examples. `tlua doc <page>` shows one from
the binary, and `tlua doc -g` all of them in a window.

## The language

- [Lua language basics](lua-language.md): Lua 5.1 in one page, and what
  code written for later versions has to do differently.
- [The standard library](lua-stdlib.md): `string`, `table`, `math`, `os`,
  `io`, `coroutine`.

## Console programs

From turbolua, in the manner of Turbo Pascal and QBasic.
[examples/console](../../examples/console) has a demo of each.

- [sys](sys.md): files and folders (read, write, copy, move, list,
  mkdir), sleep, run a command, read a line.
- [term](term.md): an ANSI terminal: colours, cursor, clear screen, keys
  one at a time, and `{red}inline{reset}` colour tags.
- [tui](tui.md): Turbo Vision-like windows, buttons, check boxes, input
  lines, lists and message boxes in the terminal, on `term`.

## Libraries

Pure Lua libraries from LuaRocks, carried unchanged.

- [inspect](inspect.md) 3.1: any value, tables above all, as readable text.
- [dkjson](dkjson.md) 2.11: JSON to Lua values and back.
- [argparse](argparse.md) 0.7: command-line options, arguments,
  subcommands and `--help`.

## The network

LuaSocket 3.1, whose C half tlua carries written in Go (Linux and macOS).

- [socket](socket.md): TCP, UDP, DNS, `select`.
- [socket.http](socket.http.md), [socket.ftp](socket.ftp.md),
  [socket.smtp](socket.smtp.md): clients for those protocols.
- [socket.url](socket.url.md): taking URLs apart and putting them
  together.
- [socket.tp](socket.tp.md): the line protocol FTP and SMTP are built on.

`ssl` (LuaSec, for TLS) is described in the README's TLS section.

## Elsewhere

The README covers `lfs`, `zlib`, `zip`, `png`, `markdown` and `ssl`, and
how modules are found. [gui.md](../gui.md) is the desktop GUI and
[pico.md](../pico.md) the game console. A program's own module of the
same name as any of these is found first.
