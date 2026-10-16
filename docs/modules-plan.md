# Shipping the modules and their documentation: plan

Status: the modules are in and work. `tlua doc` is built (decision 2,
phase 1), and for editors `tlua lib export` and `tlua lib luarc` (decision
3, the declarations part). Exporting the sources and documentation is
proposed, not built.
Written 2026-10-16.

## Where things stand

From turbolua came `sys`, `term` and `tui`, and the pure Lua libraries
`inspect` 3.1.3, `dkjson` 2.11 and `argparse` 0.7.2 (LÖVE's parts were
left out). They are embedded in the binary like LuaSocket's Lua half
(internal/lualib/luamodules.go), so `require "tui"` works anywhere, with
nothing installed, and no environment variable. They cost 165 KB of a
30 MB binary.

docs/modules has a page for each, for LuaSocket's modules, and on Lua 5.1
itself (turbolua's pages described Lua 5.4). library/ declares the new
modules for lua-language-server, which tlua already writes to the user's
cache for its editor (library.Dir).

What a binary-only user cannot reach yet:
- the documentation, which is only in the repository;
- the modules' sources, to read or to copy and change;
- the declarations, for an editor other than `tlua edit`;
- the modules from another Lua (LuaJIT, PUC Lua 5.1), for the parts that
  are plain Lua.

## Decisions

### 1. The binary stays the one source: no install step to run programs

Recommendation: keep require finding the embedded copies, as now, and do
not make anything depend on extracted files.

An install subcommand that has to run before `require "tui"` works, or
environment variables that must point at extracted files, would lose what
the single binary gives: a program works on a machine with only tlua, and
the modules always match the interpreter. After `tlua update` a folder
extracted by the old version would be stale, and files on `TLUA_INCLUDE`
win over the embedded ones (those come after `package.path`), so an old
`tui.lua` would quietly replace the new one.

### 2. `tlua doc`: the documentation, embedded and readable offline

Recommendation: do this first. It is what makes the documentation
reachable from the binary, and it costs 35 KB compressed.

- `tlua doc` lists the pages; `tlua doc tui` shows one; `tlua doc
  sys.copy` goes to that heading (headings are already anchors in the
  markdown module).
- In a terminal it is drawn with `term`'s colours (headings, code, tables
  from `markdown.parse`) through `$PAGER`, or `less -R`, when there is
  one.
- `tlua doc -g` opens it in a window: a MarkdownView with the pages as
  wiki links, back and forward, and search. examples/gui/help.lua is most
  of this already.
- The pages embedded: docs/modules, and docs/gui.md, design.md and
  pico.md, so that one command answers questions about every module.

### 3. `tlua lib export [dir]`: the files, for reading and other tools

Recommendation: do this second, as a convenience. Nothing in tlua uses
what it writes.

Built so far: `tlua lib export [dir]` writes the language-server
declarations (library/, every module tlua carries) into one folder,
`<data folder>/tlua/library`, the same for every version, so that a
`.luarc.json` pointing there keeps working after an update and a new
export. `tlua lib luarc [project]` exports and writes or merges the
project's `.luarc.json`. The declarations are written by hand and may
drift from the modules; exporting again brings an editor up to date. What
follows is the rest: the sources and the documentation beside them.

It would write, beside the declarations' folder:

```
<data folder>/tlua/
  library/         the declarations (built)
  share/lua/5.1/   every module tlua carries in Lua, in LuaRocks' layout
                   (tui.lua, socket/http.lua, ...), with their licences
  docs/            the documentation pages
```

and then prints what to do with it:

```sh
# Lua 5.1 or LuaJIT, to use the pure Lua modules:
export LUA_PATH="$HOME/.local/share/tlua/share/lua/5.1/?.lua;;"
```

Its output must say which modules need what only tlua has: `sys` needs
`lfs`, `socket.*` needs `socket.core`, `ssl` needs LuaSec's C half. Under
another Lua they work with those rocks installed.

`--force` overwrites; without it an existing folder is left alone and the
command says so. `tlua lib list` could print what is carried, with
versions, which is also what `tlua doc` lists.

### 4. No environment variables for tlua itself

The only variables export mentions are for other programs (`LUA_PATH`
for another Lua). tlua needs none, and none is added.

## Phases

1. `tlua doc`, terminal and window. The pages embedded with `//go:embed`
   from docs (a small package, docs/embed.go, since a Go package cannot
   embed files from a parent folder). A test that every page parses and
   that every link between the pages resolves.
2. The sources and docs in `tlua lib export` (the declarations and
   `tlua lib luarc` are done), and `tlua lib list`. A test that exports into a
   temporary folder and runs the exported pure Lua modules there with
   tlua `-E` and the folder as `package.path`, so that the files exported
   are the ones carried.
3. Maybe later: `term` on Windows. `term.raw`, `term.size` and
   `term.readkey` shell out to `stty`, which Windows lacks; written in Go
   on golang.org/x/term (which tlua already depends on), raw mode and the
   size would work on Windows too, `tui` with them, and a key read could
   be made interruptible, so that Ctrl-C would cancel a `tui` program's
   chunk rather than end the process.

## Open questions

1. **The name:** decided: `tlua lib export` and `tlua lib luarc`.
   "install" would suggest a program needs it to run, which it does not.
2. **The pager:** use `$PAGER` or `less -R` when there is one, else print
   it all? Recommendation: yes, and `--no-pager` to print.
3. **Which docs:** also the README's sections (lfs, zlib, zip, png,
   markdown, ssl), split into pages of docs/modules? Then `tlua doc` covers
   every module, and the README links there. Recommendation: yes, during
   phase 1.
