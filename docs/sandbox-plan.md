# Sandboxed apps and games: plan

Status: not started; written down 2026-10-11 to pick up later. Nothing
here is built yet.

## What it is

A program someone else wrote, run as a bundle (`tlua app.ztl`) or a game
(`tlua play cart`), runs locked down by default. It reads its own files,
writes to a save folder of its own, and reaches nothing else: no other
files, no network, no programs, unless the user allows it. A user who
trusts an app can unlock it, for good or once. Everything tlua runs today
keeps running as it does.

### Compared with LÖVE

LÖVE's model is the one to copy, but LÖVE does not enforce it.
`love.filesystem` gives a game read access to its own files and a save
folder (one per game identity), but Lua's `io` and `os` are still there,
and so is LuaJIT's FFI, which calls any C function. The containment is a
convention a game can step around. PICO-8 and TIC-80 lock down for real.

tlua can enforce it:

- **No native code.** `package.cpath` is empty and stays so: no C modules,
  no FFI. Everything a program can do outside itself goes through Go code
  tlua controls.
- **No bytecode.** gopher-lua does not load precompiled bytecode, a common
  way out of Lua sandboxes.
- **Memory safety.** A script cannot break out by corrupting memory, since
  the interpreter and the modules are Go.

## Decisions this plan rests on

Each has a recommendation; the alternatives are noted where they are real.

### 1. Enforce in Go, per Lua state

A policy object belongs to each Lua state, and every Go function that
reaches outside checks it before acting. `lualib.SetReader`
(internal/lualib/files.go) is the seed: it already gives each state a
reader of its own, which bundles and fused programs use.

The policy says:
- which folders can be read, and which written;
- whether the network is allowed, and to where;
- whether programs can be run, and URLs opened;
- whether the clipboard can be read and written;
- which environment variables can be seen.

The alternative, deleting globals and wrapping functions in Lua, is how
Lua sandboxes are usually built. It fails open: a forgotten
`package.loaded.io`, the registry, a metatable reached through `debug`,
and the sandbox is gone. A check made in Go cannot be undone by Lua.

The sandboxed state is also built with only the allowed libraries, not
built whole and then trimmed, and without `debug`.

### 2. Files the way LÖVE has them

A sandboxed app reads its own bundle, and reads and writes one save
folder:

| Platform | Save folder |
|---|---|
| macOS | `~/Library/Application Support/tlua/<app-id>/` |
| Linux | `$XDG_DATA_HOME/tlua/<app-id>/` (`~/.local/share/...`) |
| Windows | `%APPDATA%\tlua\<app-id>\` |

The usual calls go on working, with paths found in those two places:
`io.open`, `io.lines`, `lfs.*`, `png.load`/`save`, `zip.open`/`create`,
`gui.load`/`save`. A relative path is read from the bundle, then the save
folder; writes go to the save folder. `os.remove` and `os.rename` work
inside the save folder only. The save folder has a size limit, so that an
app gone wrong cannot fill the disk.

The program learns the folder as, say, `tlua.savedir` or
`gui.savedir()`; open question below.

### 3. The file dialog is the way out

When the user picks a file in `gui.openfile` or `gui.savefile`, or picks a
folder in `gui.choosedir`, the app may use that file or folder, and only
that. A file dropped on the app (`onDrop`) is the same. This is the
"powerbox": macOS's App Sandbox and Flatpak's portals work this way.

So an editor (microword, the text editor) works sandboxed: it opens and
saves what the user chooses, and nothing else.

### 4. A bundle says what it needs

A manifest in the `.ztl` declares what the app asks for beyond the
sandbox: `tlua.json` beside `main.lua`, or a table `main.lua` returns
first. Open question below.

```json
{ "id": "net.example.pixel",
  "name": "pixel",
  "permissions": {
    "network": ["geminiprotocol.net:1965"],
    "clipboard": true
  } }
```

- No manifest: the strict sandbox, and an id made from the bundle's name.
- On first run, each permission asked for is put to the user once: "pixel
  wants to reach geminiprotocol.net. Allow / Deny".
- The answers are kept per app id and per content hash of the bundle. A
  bundle that changed is asked about again.
- Network permission names hosts. Local and private addresses (127/8,
  10/8, 192.168/16, ::1, link-local) are refused unless named, so that an
  app cannot probe the router or services on the user's machine.

### 5. Trusting an app

- `tlua --trust app.ztl`, or "Always trust" in the first-run dialog, runs
  the app with everything, as tlua runs everything today.
- `tlua --trust-once app.ztl` (or a dialog button) for a single run.
- `tlua trust list` shows the trusted apps; `tlua trust revoke <id>`
  undoes it.
- Trust is tied to the bundle's hash, so an update nobody has looked at
  does not inherit it. Later, publisher signatures could carry trust
  across updates (open question).

### 6. What is sandboxed by default

| How it runs | Default | Why |
|---|---|---|
| `tlua app.ztl` (or .zip, .app) | sandboxed | a downloaded app, LÖVE's case |
| `tlua play cart`, a game bundle | sandboxed | the same, for games |
| `tlua script.lua`, `-e`, the REPL, `tlua edit` | trusted | the user's own code; a sandbox would only be in the way |
| `tlua design`'s Run | trusted, with a "Run sandboxed" choice | so a developer can try the locked-down version |
| a fused executable | the author's choice, when fusing (`tlua fuse -sandbox`) | the user already ran a native program; the sandbox protects from a careless author more than a hostile one |

## What a program can reach today

Every one of these needs the policy's check. The list is the work.

| Reach | Calls | Where |
|---|---|---|
| reading files | `io.open`, `io.lines`, `io.input`, `loadfile`, `dofile`, `require` from disk, `lfs.dir`, `lfs.attributes`, `png.load`, `zip.open`, `gui.load`, an Image's `file`, `g:image(path)`, sound files, cartridges | gopher-lua's iolib and baselib, internal/lualib (lfs, png, zip, files.go), internal/gui, internal/sound, internal/game, internal/pico |
| writing files | `io.open` for writing, `io.output`, `io.tmpfile`, `os.remove`, `os.rename`, `os.tmpname`, `lfs.mkdir`, `lfs.rmdir`, `lfs.touch`, `lfs.link`, `png` `img:save`, `zip.create(path)`, `gui.save` | the same |
| running programs | `os.execute`, `io.popen`, `gui.spawn`, `gui.openurl` | internal/lualib/open.go, popen_*.go, internal/gui/spawn.go, openurl.go |
| the network | `socket.core` (tcp, udp, dns), `ssl` | internal/lualib/socket_*.go |
| the environment | `os.getenv`, `arg`, `gui.interpreter` | gopher-lua oslib, cmd/tlua |
| code | `require` (package.path, loaders, preload), `loadstring`/`load` (fine: same sandbox), `debug.*` | gopher-lua loadlib, interp |
| the desktop | `gui.clipboard` (read and write), file dialogs, dropped paths | internal/gui |
| the process | `os.exit` (fine), `tlua update` (not reachable from Lua) | |

Harmless and left alone: `string`, `table`, `math`, `coroutine`, `os.time`,
`os.clock`, `os.date`, `zlib`, `mime.core`, `markdown`, drawing,
`socket.gettime`, `socket.sleep`.

## Phases

1. **The policy and the strict sandbox.** The policy object; a sandboxed
   state built with the allowed libraries only; no `debug`, no running
   programs, no network, no clipboard reading, no environment; files in the
   bundle and the save folder only, through every call in the table above.
   Bundles and `tlua play` sandboxed by default; `--trust` to turn it off.
2. **The powerbox.** Files and folders chosen in dialogs or dropped are
   allowed, one at a time. Editors work sandboxed.
3. **Manifests, permissions and trust.** `tlua.json` (or the table), the
   first-run dialog, the trust store keyed by app id and hash, `tlua trust
   list` and `revoke`, network allowed to named hosts with private addresses
   refused.
4. **Limits.** A time budget, or an "app not responding: stop it?" dialog,
   on the cancellation the interpreter already has (Ctrl-C ends a script by
   cancelling its context). A size limit on the save folder.
5. **Maybe later: the operating system's sandbox.** Run untrusted apps in a
   child process inside it as a second wall: Landlock and seccomp on Linux,
   `sandbox-exec` or the App Sandbox on macOS, AppContainer on Windows. A
   hole in phase 1's list would then still not reach the user's files. A
   child process also allows a memory limit, which Go's shared heap does
   not. Per platform, and the most work.

## Keeping it complete

The risk is not a clever escape but a forgotten function: a module added
next year that opens a file without asking the policy. So:

- A test builds a sandboxed state, walks every global, every loaded and
  preloadable module and every metatable it can reach, and fails on any
  function not on a list of known, checked functions. A new function
  cannot slip in unreviewed.
- Tests that try each reach in the table above from inside the sandbox and
  expect a refusal, and from a trusted state and expect it to work.
- The policy check is a single Go call each module makes, so that a review
  can find every place by searching for it.

## Open questions

1. **The manifest:** a `tlua.json` file, or a table in `main.lua`? JSON can
   be read without running the app, which a permission prompt needs before
   anything runs. Recommendation: the file.
2. **The save folder's name in Lua:** `tlua.savedir`, a new `app` module
   (`app.savedir`, `app.id`), or something in `gui`? Games without the gui
   need it too, so not `gui`. Recommendation: a small `app` module.
3. **Signing:** trust tied to a hash is safe but asks again on every
   update. Publisher keys (ed25519, the signature in the zip's comment or
   beside the manifest) would let trust carry over. Worth it once there
   are apps people update.
4. **The prompt without a window:** a console program or `tlua play` asks
   in the terminal; a GUI app with a dialog before its first form. What
   when there is neither (piped, no terminal)? Recommendation: deny, and
   say how to trust.
5. **Fused executables:** sandbox them by default too, or only when fused
   with `-sandbox`? Recommendation: the author's choice, default off,
   since a fused program is already a native program the user ran.
6. **Reading the clipboard:** separate from writing it? Writing is mostly
   harmless; reading can leak passwords. Recommendation: writing allowed,
   reading a permission.
