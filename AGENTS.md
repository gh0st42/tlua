# Working on tlua

Notes for coding agents (and people) picking up work on this repository:
how it is built and tested, how releases are made, and what has been
agreed about how to work. README.md describes what tlua is; docs/ has the
details of each part.

## What it is

tlua is a Lua 5.1 interpreter in Go, on a patched gopher-lua, with:
- a desktop GUI module on FLTK (`require "gui"`), and `tlua design`, a
  VB6-style form designer written in Lua on it, which also designs
  components (groups of controls made with `gui.component`, shared through
  palettes) and edits console programs and games as source projects;
- a fantasy console for games (`tlua play`);
- bundles (`.ztl`), fused executables, and a terminal editor (`tlua edit`);
- Go versions of the C modules LuaRocks libraries are built on (lfs,
  LuaSocket, LuaSec, zlib, zip), pure Lua libraries from LuaRocks
  (inspect, dkjson, argparse), and modules of its own (png, markdown, and
  sys, term and tui from turbolua).

## Layout

| Where | What |
|---|---|
| cmd/tlua | the command: options, subcommands, running scripts and bundles |
| internal/interp | the interpreter around gopher-lua: paths, signals, fused programs |
| internal/lualib | the modules: lfs, socket.core, ssl.*, zlib, png, zip, mime |
| internal/lualib/lua | modules written in Lua: markdown*, sys, term, tui; LuaSocket's and LuaSec's Lua halves; inspect, dkjson, argparse |
| internal/gui | the gui module; `fltk*.go` on FLTK, `nofltk.go` without cgo |
| internal/gui/lua | controls written in Lua that come with tlua (MarkdownView, MarkdownEdit) and `gui.component` |
| internal/design, internal/design/lua | `tlua design`, mostly in Lua |
| internal/game, internal/pico | `tlua play` |
| internal/update | `tlua update` |
| internal/doc | `tlua doc`: the pages in the terminal or a window |
| third_party/gopher-lua | the patched gopher-lua; PATCHES.md says what and why |
| library | lua-language-server declarations for tlua's modules; `tlua lib` exports them and writes a .luarc.json |
| docs | gui.md, design.md, pico.md; modules/, a page per module and on Lua 5.1; plans (`*-plan.md`) and future.md |
| examples | gui/, console/, pico/, net/ and so on; palette/ holds components for the designer's palette |
| scripts, .github/workflows | release builds and CI |

The sibling repository `../turboapps` holds desktop apps built on tlua
(microword, microsheets, pixel, ...). It uses tlua's markdown modules; a
change to them must keep `make test` and `make check` there passing.

## Building and testing

```sh
make build                  # bin/tlua, with the GUI (cgo)
make test                   # go test ./...
make test-gui               # tests that open windows; see below
CGO_ENABLED=0 go test ./... # the build without the GUI must work too
go vet ./... && gofmt -l .
go test -race ./internal/lualib/   # after touching sockets or TLS
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...   # other platforms compile
```

Before saying a change is done, run vet, `make test`, the no-cgo tests,
and `make test-gui` when the gui or the designer changed. CI
(.github/workflows/ci.yml) runs Linux under Xvfb, macOS, no-cgo, and the
musl build in Alpine.

- **GUI tests** run with `TLUA_GUI_TESTS=1` and `-p 1`; they open real
  windows and send synthetic input (internal/gui/fltkinput), so nothing
  else should take the keyboard while they run.
  Two runs at once take each other's focus and keys, and fail as if there
  were bugs; internal/guilock makes a second run wait for the first. Do not
  start a GUI suite while another is going, nor type while one runs.
- **Nothing may wait for a person.** The designer's questions are
  answered in tests by `D.answer`, `D.answerText`, `D.answerColor`,
  `D.answerFile`, `D.answerMenu`, and at startup `answerStart` and
  `answerDir`; the test harness makes `gui.msgbox` and the like raise an
  error rather than wait.
- **Never touch the user's real clipboard** in a test: stub
  `gui.clipboard`.
- **Every backend function needs its `nofltk.go` twin**, or the no-cgo
  build breaks.

## Looking at what the GUI draws

- Prefer pictures made without the screen: `canvas:snapshot(scale)` and
  `gui.paint(w, h, fn)` give a png image to save and look at.
- When a real screenshot is needed, capture **one window, by its window
  ID** (`screencapture -x -o -l <id>` on macOS), never the whole screen or
  a region of it. FLTK's windows can sit at window layer 3; find the ID by
  the process and the window's title (CGWindowListCopyWindowInfo).

## Processes

- Stop only processes you started, by their PID. Never `pkill` by a broad
  name: `pkill lua-language-server` once killed the language servers of
  the user's editor.
- `cmd &` then `kill $!` kills the subshell when `cmd` is a chain; `exec`
  the program, or find its own PID.
- A GUI program must stay stoppable by SIGTERM and Ctrl-C. Anything that
  waits in an event loop of its own (a modal dialog at startup, say) runs
  under `Interp.Protect`, which a signal cancels.
- Keep an eye on free disk space (`df -h`); the disk once filled up
  mid-work.

## How changes are made

- **Commit, push, tag and release only when asked.** Other agents and the
  user commit to main too: look at `git log` and `git status` first, and
  say when commits you did not make are about to go into a release.
- **Commit messages** follow [Conventional Commits](https://www.conventionalcommits.org):
  a first line `type(scope): subject`, with a type of `feat`, `fix`,
  `docs`, `refactor`, `test`, `perf`, `build`, `ci` or `chore`, a scope
  naming the part (`doc`, `gui`, `lualib`, `interp`, `design`, `library`,
  `update`, ...) when one fits, and a subject in plain words and the
  imperative that says what the change does for its user, such as
  `feat(lualib): add TLS with LuaSec's interface`. A breaking change has a
  `!` after the type or a `BREAKING CHANGE:` footer. Then a blank line and
  prose saying what and why. End with the `Co-Authored-By` line the session
  asks for. Commits before 2026-10-17 are in the older plain style.
- **One feature per commit.** When two features touch the same file, stage
  each one's lines with its own commit.
- **Compatibility with reference Lua 5.1 comes first.** A behaviour that
  differs from it is a bug: fix it in third_party/gopher-lua, add an entry
  to PATCHES.md, and a case to internal/interp/interp_test.go.
- **C modules are written in Go, Lua halves run unchanged.** As
  socket.core is LuaSocket's C half and ssl.* LuaSec's, with the real
  Lua files on top, compared against the C build's results and error
  strings. Differences are listed in the README.
- **Modules tlua carries in Lua** (internal/lualib/luamodules.go) are found
  after `package.path`, so a program's own copy wins. tlua's own ones also
  have a `tlua.<name>` preload name, which they use among themselves.
- **Controls written in Lua** that come with tlua live in internal/gui/lua
  and are defined with `gui.define` when the gui module opens.
- **Docs are built in.** `tlua doc` shows docs/modules, gui.md, design.md
  and pico.md from the binary (docs/embed.go); a new page outside
  docs/modules goes into its `//go:embed` line, and internal/doc's tests
  check that links between pages lead to pages that are built in.
- **Docs move with the code.** A user-visible change updates what applies:
  README.md, docs/gui.md, docs/design.md, docs/modules, the `library/*.lua`
  declarations, `tlua -h` (cmd/tlua/main.go), and an example under
  examples/ when one helps.
- **Writing**: docs and comments in plain English, short sentences, saying
  what something is for and why; match the density of the code around.
- **Plans**: a larger piece of work gets a `docs/<topic>-plan.md` with its
  status, decisions (each with a recommendation), phases and open
  questions. Ideas set aside go to docs/future.md.

## Releases

Only when asked. Patch versions for features and fixes so far
(0.6.0 -> 0.6.1 -> 0.6.2).

1. Commit the work.
2. Set `Number` in internal/version/version.go; build and check `tlua -v`.
3. Commit as `chore(release): X.Y.Z`, the body starting "Since X.Y.W:" and
   listing what changed.
4. `git tag vX.Y.Z`, push main, then push the tag.
5. The Release workflow checks the tag matches the version, builds eight
   archives (macOS arm64; Linux amd64 and arm64, each glibc, static and
   musl; Windows) with SHA256SUMS. Watch it with `gh run watch`, and check
   the assets with `gh release view vX.Y.Z`.

Release builds are stamped with the archive they go into
(`-X tlua/internal/version.Build=...`, scripts/build-release.sh); that is
how `tlua update` knows a release build from one built from source.

## Plans waiting

- docs/components-plan.md: components in the designer. Built (gui.component,
  the designer's component editor, interface dialog, palettes); what is left
  is listed in the plan.
- docs/sandbox-plan.md: running bundles and games locked down, LÖVE-style,
  with permissions and trust. Not started.
- docs/modules-plan.md: `tlua doc`, and the sources and docs in `tlua lib
  export`, so they reach a user with only the binary. The declarations'
  export and `tlua lib luarc` are done; the rest is proposed.
- docs/future.md: smaller things set aside.
