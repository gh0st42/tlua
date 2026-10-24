# `zip` — zip archives, with AES encryption

`zip` reads and writes zip archives, from files or from strings in memory. A
file in an archive can be encrypted with AES-256, in WinZip's form, which
7-Zip and other tools read and write. What tlua writes opens in 7-Zip with the
password, and what 7-Zip writes (AES-128, 192 and 256) opens here; tlua's
tests check both against 7-Zip itself. It also reads the way
[LuaZip](https://github.com/mpeterv/luazip) does, so code written for that
rock runs.

```lua
local zip = require "zip"
```

In a fused program or a bundle, `zip.open` reads the program's own files
first.

## Reading

### `zip.open(path [, options])`

The archive in a file. Returns the archive, or `nil, why`.

### `zip.load(data [, options])`

The archive in a string, with the same options.

- `options.password` (string) — the password for the encrypted files in it.
  It can also be given later, with `arc:setpassword`, or for one call.

```lua
local arc = assert(zip.open("data.zip"))
for _, name in ipairs(arc:list()) do
  print(name, arc:info(name).size)
end
local text = arc:read("readme.txt")
```

### `arc:list()`

The names of the files in the archive, in its order, as a table. Folders
are not listed. Names are never encrypted, so this works without a password.

### `arc:exists(name)`

`true` if there is a file of that name.

### `arc:info(name)`

A table, or `nil` if there is no such file:

| Field | |
|---|---|
| `name` | the file's name |
| `size` | its size, unpacked |
| `compressed` | its size in the archive, without what encryption adds |
| `modified` | when it was changed, as `os.time()` counts |
| `method` | `"deflate"` or `"store"` |
| `encrypted` | `true` if it needs a password |
| `filename`, `uncompressed_size`, `compressed_size` | LuaZip's names for `name`, `size` and `compressed` |

### `arc:read(name [, password])`

The file's contents, or `nil, why`. `password` is for this call; without
it the archive's password is used.

The reasons an encrypted file is not given back say what is wrong:

- `a password is needed`: the archive has none.
- `wrong password`: the password does not fit. This is found out before any
  of the file is decrypted.
- `the data has been altered, or is damaged (or the password is wrong)`: the
  file is encrypted with a MAC, and it does not match. Nothing of the file
  is returned. The password check is only two bytes, so about one wrong
  password in 65,000 gets past it and is found out here.
- `... is encrypted the old way (ZipCrypto), which tlua does not read`: see
  below.

### `arc:setpassword(password)`

The password to read the encrypted files with from now on; `nil` takes it
away.

### `arc:files()` and `arc:open(name [, password])`

LuaZip's way. `arc:files()` is an iterator over a table for each file, with
the fields of `info`. `arc:open(name)` is a file to read from, with
`read("*a")`, `read("*l")`, `read(n)`, `lines()`, `seek()` and `close()`;
the file is read and decrypted whole when it is opened.

```lua
for f in arc:files() do print(f.filename, f.uncompressed_size) end
local f = arc:open("notes.txt")
print(f:read("*l"))
f:close()
```

### `arc:close()`

Does nothing; an archive holds no file open.

## Writing

### `zip.create([path] [, options])`

An archive to add files to. With a `path` it is written there when it is
closed; without, it is made in memory. Options may be the only argument for
an archive in memory.

- `options.password` (string) — encrypts every file added, unless the file
  says otherwise.

### `out:add(name, data [, options])`

Adds a file. It is compressed unless `store` says not to.

- `store` (boolean) — keep it as it is.
- `modified` (number) — its date, as `os.time()` counts; now by default.
- `password` (string or `false`) — a password for this file alone, or `false`
  for none when the archive has one.

### `out:close()`

Writes the archive. Returns `true` for one written to a file, the archive's
bytes for one in memory, or `nil, why`. A file is written through a
temporary file, so a failure never leaves half an archive.

```lua
local out = zip.create("report.zip", { password = "correct horse" })
out:add("report.txt", text)
out:add("logo.png", logo, { store = true })           -- already compressed
out:add("README.txt", "Open with the password.", { password = false })
assert(out:close())

-- and back
local arc = assert(zip.open("report.zip", { password = "correct horse" }))
print(arc:read("report.txt"))
```

## What the encryption is, and is not

- **AES-256**, in counter mode, with each file's own random salt, and a
  MAC (HMAC-SHA-1) over each file's encrypted bytes, checked before anything
  is decrypted. A file that was changed is refused, not decrypted into
  something that looks right. The keys come from the password with
  PBKDF2-HMAC-SHA-1. This is WinZip's AE-2 format; tlua reads AE-1 and AE-2,
  with AES-128, 192 and 256.
- **The names, sizes and dates in the archive are not encrypted.** A zip has
  no way to; anyone can see which files there are and how big. To hide that,
  put the archive inside another, or encrypt the whole file.
- **The password is only as strong as it is long.** The format fixes the key
  derivation at 1000 rounds of SHA-1, which is fast to try against. Use a
  long passphrase, not a short word. This is a property of the format, not
  something tlua can raise.
- **Passwords are bytes**, as Lua strings are, and UTF-8 text works between
  tlua and recent 7-Zip (checked with 26.03). The format does not say how a
  password is turned into bytes, so other tools do it differently: p7zip 17
  does not accept an archive made here with a non-ASCII password, and 7-Zip's
  command line refuses to *make* an AES zip with one. An ASCII password is
  safe everywhere.
- **Old ZipCrypto is not read.** What `zip -e` writes by default is broken
  encryption, and tlua will not pretend otherwise: the error says so. Make
  the archive again with 7-Zip's `-mem=AES256`, or read it with the tool that
  made it.
- Files are encrypted and decrypted whole, in memory, as they are read
  and added: this is for files that fit there.

7-Zip, to make or check one by hand:

    7zz a -tzip -mem=AES256 -p'passphrase' secret.zip files...
    7zz t -p'passphrase' report.zip
