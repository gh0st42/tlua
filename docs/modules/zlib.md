# `zlib` — compression

`zlib` compresses and decompresses data in the zlib, gzip and raw deflate
formats. It has the API of [lua-zlib](https://github.com/brimworks/lua-zlib)
1.2, which LuaRocks code expects, with lzlib's `compress` and `decompress`,
and `gzip` and `gunzip` for a string at once. tlua carries it written in Go
(the standard library's `compress/flate`), checked against Python's zlib
and gzip both ways round.

```lua
local zlib = require "zlib"
```

## All at once

### `zlib.compress(data [, level [, method [, windowBits]]])`

`data` compressed, in zlib's format unless `windowBits` says otherwise.
`level` is -1 (the default) or 0 to 9; `method` is accepted and ignored.

### `zlib.decompress(data [, windowBits])`

The data back, for zlib or gzip input; `nil` and why if it is not that.

```lua
local text = string.rep("hello ", 100)
local z = zlib.compress(text)
print(#text, #z)                      --> 600   21
print(zlib.decompress(z) == text)     --> true
print(zlib.decompress("garbage"))     --> nil   zlib: invalid header
```

### `zlib.gzip(data [, level])`, `zlib.gunzip(data)`

The gzip format, which is what `.gz` files are:

```lua
local gz = zlib.gzip(text)
print(gz:sub(1, 2) == "\31\139")      --> true
print(zlib.gunzip(gz) == text)        --> true
```

### Window bits

`windowBits` says which format. 8 to 15 is zlib, -8 to -15 is raw deflate
with no header (what zip files hold), 16 more is gzip, and to inflate, 32
more detects zlib or gzip by itself.

```lua
local raw = zlib.compress(text, 9, nil, -15)
print(zlib.decompress(raw, -15) == text)   --> true
```

## In pieces

For data that comes or goes a piece at a time.

### `zlib.deflate([level [, windowBits]])`

A compressing stream: a function you call with the next input.
`stream(input [, flush])` returns what has come out so far, whether the
stream is finished, and the number of bytes in and out. `flush` is `"none"`
(the default), `"sync"`, `"full"` or `"finish"`, which ends it.

```lua
local deflate = zlib.deflate(9)
local out, finished, bytesIn, bytesOut = deflate(text, "finish")
print(#out, finished, bytesIn, bytesOut)   --> 21   true   600   21
```

### `zlib.inflate([windowBits])`

A decompressing stream, given the input in as many pieces as it comes in:
`stream(input)` returns what can be decompressed so far, whether the
compressed stream has ended, and the bytes in and out.

```lua
local inflate = zlib.inflate()
local a = inflate(out:sub(1, 5))
local b, ended = inflate(out:sub(6))
print(#a + #b, ended)                      --> 600   true
```

## Checksums

### `zlib.crc32([data])`, `zlib.adler32([data])`

The checksum of a string. Given a number and a string, they carry a checksum
on: `crc32(crc, more)`. Given nothing, they return a function that keeps a
running one.

```lua
print(zlib.crc32("hello"))                         --> 907060870
print(zlib.crc32(zlib.crc32("hel"), "lo"))         --> 907060870
local running = zlib.crc32()
running("hel")
print(running("lo"))                               --> 907060870
```

### `zlib.version()`

The version of zlib this stands for, `"1.3.1"`.

The `zip` module uses the same compression; [zip](zip.md) reads and writes
archives of files.
