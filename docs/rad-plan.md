# A VB6-style form designer: plan

Status: all five phases are built (2026-10-09). Phase 1 added names,
`gui.load`, `gui.dump`, `gui.save` and `gui.kinds`, described in docs/gui.md
under "Forms in files". Phases 2 to 5 are `tlua design`, described in
docs/design.md. Only the "maybe later" of phase 5 remains.

## What it is

`tlua design [project]` opens a window in the manner of Visual Basic 6:
- a toolbox of controls;
- the form being designed, where controls are drawn, moved and resized with
  the mouse;
- a properties window;
- a code window that the designer takes you to when you double-click a
  control.

F5 runs the program and Make EXE packs it with `tlua fuse`.

What it produces is ordinary tlua: a folder of `.lua` files that run with
`tlua main.lua`, without the designer and without anything of it at runtime
beyond the `gui` module. A designed form can be edited by hand, and a
hand-written one can be opened in the designer.

## Decisions this plan rests on

Each has a recommendation; the alternatives are noted where they are real.

### 1. The designer is written in Lua, on the `gui` module

The designer is a Lua program embedded in the binary, the way `tlua edit` is
part of it. `tlua design` runs it.

- **Dogfooding.** Every gap it hits is a gap any GUI program would hit. The
  runtime work it forces (removing controls, scrolling, reflection, a better
  code box) is worth having anyway.
- **Easier to change.** It is easier to change than Go, and users can read it
  to learn the module.
- **Testable.** It can be tested with the synthetic input `make test-gui`
  already uses.

The alternative, Go on go-fltk directly, would be faster to start. But it
would keep two GUI stacks, and leave the module's gaps open.

### 2. A project is a folder; a form is two files

```
myapp/
  main.lua                -- starts the program: shows the startup form
  forms/Main.form.lua     -- the layout: written by the designer
  forms/Main.lua          -- the code: written by you
```

This is VB6's `.frm`, split the way later designers split it:
- **The layout file** belongs to the designer. It is plain data, so it can be
  rewritten whole on every save without touching anyone's code.
- **The code file** belongs to the programmer. The designer only ever appends
  event stubs to it, and never rewrites what is there.

The layout file is a Lua table, so the same loader reads it as reads any
Lua:

```lua
-- forms/Main.form.lua — written by tlua design
return {
  kind = "Form", name = "Main", caption = "Hello", width = 320, height = 160,
  { kind = "Label",   name = "lblName", caption = "Name", left = 16, top = 16, width = 60 },
  { kind = "TextBox", name = "txtName", left = 80, top = 16, width = 224 },
  { kind = "Button",  name = "cmdGreet", caption = "Greet", default = true,
    left = 204, top = 112, width = 100 },
}
```

```lua
-- forms/Main.lua — yours; the designer adds stubs at the end
local gui = require "gui"
local frm = gui.load "Main"   -- forms/Main.form.lua, beside this file

function frm.cmdGreet:onClick()
  gui.msgbox("Hello, " .. frm.txtName.text .. "!")
end

return frm
```

Alternative: a `.frm`-like text format of its own. That means a parser to
write and keep, and nothing gained over a Lua table.

### 3. Controls are found by name

`gui.load` gives back the form, with its controls reachable as fields:
`frm.cmdGreet`. VB6's `Private Sub cmdGreet_Click()` becomes
`function frm.cmdGreet:onClick()`. A name that collides with a property or
method is refused at load time and in the designer, so `frm.caption` can
never be a control.

### 4. The design surface shows real controls

The form is built from its layout file with the real controls, in a group
inside the designer's window. A transparent overlay on top takes all the
mouse input, so nothing underneath reacts to clicks, and it draws the
selection handles. What you see is exactly what runs.

The alternative, drawing look-alike boxes on a Canvas, is easier to make
interactive. But it would never quite look like the program, and it would
need its own drawing for every kind, defined controls included.

### 5. One window, with panes

The toolbox, the surface, and the properties and project panes sit in one
window with draggable dividers (Tile), rather than in VB6's MDI or separate
floating windows. That is simpler to manage across macOS, Windows and Linux.
The code window is a tab beside the surface.

## What the `gui` module needs first

The designer can't be built without these, but they are useful to any
program, so each one ships with docs, a line in `library/gui.lua`, and tests.

| # | Addition | Why the designer needs it | Where |
|---|----------|---------------------------|-------|
| R1 | `name` property; `form.<name>` and `form:find(name)` for the controls of a loaded form | code-behind finds controls by name | module.go |
| R2 | `gui.load(path or table)` builds a form from a layout table; `gui.dump(form)` turns a form back into one | open and save; round trip is the test | new layout.go |
| R3 | `gui.kinds()`: every kind with its properties (type, default, whether fixed at creation), events and what it holds; `gui.define` takes `props` with the same shape | toolbox, property grid, event lists; defined controls in the toolbox | module.go, custom.go |
| R4 | `obj:remove()`, and z-order with `obj:raise()` and `obj:lower()` | deleting and reordering controls | module.go, fltk.go |
| R5 | A transparent Canvas (`transparent = true`, no box of its own) that redraws what is under it | the overlay | fltk_more.go |
| R6 | `Scroll` and `Splitter` containers (go-fltk's Scroll and Tile) | long property lists, panes | fltk.go |
| R7 | `image =` on Button and Label | toolbox icons | fltk.go |
| R8 | A code-editing TextBox: `lineNumbers`, `syntax = "lua"`, `cursor` and `selection`, go-to-line, Tab inserting a tab | the code window | move internal/editor's scanner to a shared internal/luasyntax; fltk.go |
| R9 | Menu items addressable after they are made: enable, check, rename | the designer's own menus; previewing a designed menu | fltk.go |
| R10 | `gui.spawn(command, onOutput, onExit)`, delivered on the GUI thread through `fltk.Awake` | F5: run the program in its own process, output in a pane, errors to their line | new spawn.go |
| R11 | `tabIndex` | tab order, which VB6 lets you set | fltk.go |

R1–R3 are useful on their own, as forms written by hand, and are the first
piece to build.

## The designer, in phases

Each phase ends with something usable and tested.

### Phase 1: forms as files (runtime only: R1–R3) — done

Built as planned, plus `gui.save`, which writes a layout in a steady order,
so that saving what was loaded gives the same file. A designer needs that.
`gui.load` paths resolve beside the calling script (and in a fused
program's archive), so a form's code says `gui.load "Main"`.

- **Build:** `gui.load`, `gui.dump`, names and `gui.kinds`, with
  `examples/gui/hello` rewritten as a layout file plus code.
- **Tests:**
  - load → dump → load gives the same table;
  - every built-in kind round-trips;
  - name collisions are refused.
- **Done when:** a hand-written two-form program runs from its layout files.

### Phase 2: a designer that edits layouts (R4–R7, R11) — done

Built with these changes to the plan:
- **R10 moved into phase 2.** F5 needs `gui.spawn` here, not in phase 3.
- **R11 (tab order) moved out.** Nothing uses it before phase 4's tab-order
  editor, so it waits for that.
- **Additions the designer needed:**
  - `obj.parent`, to find a control's place in the window;
  - `onKey` on a Canvas, so the surface takes Delete and the arrow keys;
  - a `double` argument to `onMouseDown`, for the toolbox's double-click.
- **The panes** are a Splitter of three Panels, and the surface is a Scroll,
  so a form larger than the pane scrolls.
- **Properties grid:** rows of Labels and editors in a Scroll, as the risks
  section suggested.
- **Phase 2 deliberately leaves out** the project tree's adding and
  renaming, apart from File > Add Form.
- **The measure** is a test that builds the layout example's form in the
  designer, saves it, wires it up and runs it: `TestDesignerRebuildsTheLayoutExample`.

The original phase 2 list follows.


- **The window:** toolbox on the left, surface in the middle, properties on
  the right, and a project tree of forms.
- **Placing controls:** pick a kind in the toolbox and drag out its
  rectangle on the surface, or double-click the kind for a default-sized one
  in the middle.
- **Editing controls:** select with a click; move by dragging; resize with
  eight handles; delete with the Delete key; arrow keys nudge.
- **The properties grid:** one row per property, from `gui.kinds()`. Text and
  number boxes, check boxes for booleans, combos for choices (`align`,
  `font`), a swatch with a picker for colours, and a list editor for `items`.
  Fixed properties are editable too: changing one rebuilds that control.
- **Files:** New, Open and Save for projects and forms; a dirty marker; a
  question before losing changes.
- **Run:** F5 saves and runs `main.lua` in its own process (R10), with its
  output in a pane.
- **Tests:** synthetic-input tests for placing, moving, resizing and deleting.
  Each edit must change the saved layout exactly as intended.
- **Done when:** the Phase 1 example can be rebuilt in the designer from
  nothing, saved, and run.

### Phase 3: the code window (R8–R10) — done

Built as planned, with these notes:
- **The highlighter** moved out of the terminal editor into
  internal/luasyntax, which both use.
- **R9 (menus)** turned out to be almost there already: a menu is changed
  by changing its items table and assigning it again. All it lacked was
  toggles writing `checked` back into their items.
- **Renaming** is offered when the change is done with (selecting something
  else, saving, or going to the code) rather than as each letter is typed.
- **"Open in tlua edit"** became Open Code in Editor, which hands the file
  to the system's editor for .lua. `tlua edit` is a terminal program, and
  starting a terminal from a GUI is a different thing on every platform.
- **A crash fixed on the way:** go-fltk's FLTK headers do not check the
  index given to `Fl_Menu_::value(int)`. A ComboBox with nothing selected
  pointed FLTK before its items, and enough ComboBoxes filled and freed (as
  the properties grid does) crashed in drawing. The gui module now never
  gives an index outside the items.

The original phase 3 list follows.


- **Event stubs:** double-click a control and the designer appends
  `function frm.<name>:<defaultEvent>()` to the form's code file, unless it is
  there already, and opens the code window on it. A Button's default event is
  `onClick`, a TextBox's `onChange`, and so on, which is VB6's behaviour.
- **The two combos:** the code window's top bar has VB6's pair: the object,
  then its event. Picking one jumps to that handler, writing the stub if
  needed.
- **Editing:** the code pane is the R8 TextBox, with Lua highlighting from
  the editor's scanner, line numbers, find, and go-to-line. "Open in tlua
  edit" stays available for anyone who prefers the full editor.
- **Errors:** a Run error with a file and line jumps the code window there.
- **Renaming:** renaming a control in the properties grid offers to rename
  `frm.<old>` in the code file. It is a plain text substitution, shown before
  it is applied.
- **Done when:** a form can be designed, wired up and debugged without
  leaving the designer.

### Phase 4: the comforts VB6 had — done

Built with these notes:
- **The runtime** gained modifier keys in a Canvas's `onMouseDown` (for
  Shift-click) and `tabIndex` (R11).
- **Menus in layouts** gained a Menu `onClick(name, caption, checked)`. A
  layout's menu has no functions in it, so its items are told apart by
  `name` in the code. A Panel can hold a Menu, so the designer shows it.
- **Undo** keeps a copy of the layout before each change, not a list of
  edits: forms are small, and undoing is then a matter of building the form
  again.
- **The clipboard** for controls is the designer's own, so copying controls
  leaves the system clipboard alone.
- **The startup form** is read from main.lua, and main.lua is rewritten only
  while it is as the designer wrote it.
- **Make EXE** has a test that packs a project and runs the executable.

The original phase 4 list follows.


- **Selection:** rubber-band and Shift-click multi-select.
- **Arranging:** align (lefts, tops, centres), make the same size, centre in
  form, and space evenly.
- **Grid and snapping:** a grid with snapping, toggled from the menu.
- **Editing:** undo and redo, as a stack of edits to the layout table; cut,
  copy, paste and duplicate.
- **Tab order:** set by clicking controls in order.
- **Menu editor:** VB6's Menu Editor dialog, which edits the form's `Menu`
  items: captions, shortcuts, nesting, separators, checked and enabled.
- **Several forms:** a project with several forms and a startup form.
  `main.lua` is generated, and regenerated only while it is untouched.
- **Make EXE:** runs `tlua fuse`, with the project's images and data
  included.

### Phase 5: extending it — done

Built with these notes:
- **Finding a project's controls** is done by `gui.load` itself: a kind it
  does not know is required as `controls.<kind>`. Programs need no wiring of
  their own, packed or not, and the designer loads them the same way.
- **Changing a declared prop** of such a control builds it again, so a
  control that draws from its props, or builds itself from them, shows the
  change.
- **The stub** is `forms/Name.d.lua`, and the code's `frm` is cast to it with
  `--[[@as forms.Name]]`. A `---@type` annotation would not do: `gui.load` is
  declared to return a Form or any object, which a declared type rejects.
  A test runs lua-language-server on a project the designer made.
- **Containers:** the surface keeps a tree, not a list. Drops go into the
  topmost Frame, Panel or showing page under the mouse, and controls keep
  their place on the form when they move between containers.
- **Clicking the tab row** of the selected Tabs shows its next page. go-fltk
  cannot tell which tab is under a point.
- **The tests** were made to never wait for a person: designer questions go
  through one place the tests answer, an unexpected dialog fails at once, a
  watchdog sends Escape after 20 seconds, and the two GUI packages run one
  after the other so they do not take each other's keyboard focus.

The original phase 5 list follows.


- **Your own controls in the toolbox:** each `gui.define` in the project's
  `controls/` folder gets a toolbox entry, and rows in the properties grid
  from its `props` (R3). It is previewed on the surface by building it for
  real, which is why decision 4 matters.
- **Containers in the designer:** controls dragged into a Frame or onto a
  Tabs page, and pages added and removed.
- **Language-server stubs:** saving a form also writes a `---@class`
  description of it to a `.d.lua` beside it, so the language server
  completes `frm.cmdGreet.` with a Button's fields.
- **Maybe later:** layout containers backed by go-fltk's Flex and Grid, for
  forms that arrange themselves.

## Testing

- **The runtime additions:** model tests without a display, the way the
  module's are now. GUI tests run under `make test-gui`.
- **The designer:** its parts are Lua and can be unit-tested headlessly where
  they don't touch the screen: layout edits, undo, stub insertion, renaming.
  Interaction tests drive the real designer window through `fltkinput`.
- **Golden files:** a set of layout files that every phase must load, save
  unchanged, and run.

## Risks

- **The overlay (R5).** FLTK has no real transparency between sibling
  widgets. A NO_BOX widget on top relies on the widgets under it being redrawn
  first. If that flickers or leaves trails, the fallback is to redraw the
  whole surface group on every change, or to draw the handles in the
  surface's own draw handler.
- **The code editor (R8).** Fl_Text_Editor with highlighting is good enough
  for event handlers. It is not `tlua edit`, and should not try to be. The
  escape hatch to the full editor stays.
- **Properties grid.** Table can't edit cells and Tree can't report clicks
  (both go-fltk limits), so the grid is built from Labels and inputs in a
  Scroll (R6), not from either. That is simpler, and it scales to a few dozen
  rows, which is all a control has.
- **Platforms.** The GUI has only been run on macOS. Linux and Windows
  should be built and run before Phase 2 is called done, since a designer
  shows platform differences (fonts, focus, drag behaviour) at once.
- **Scope.** VB6 had a great deal more: data binding, ActiveX, reports, the
  debugger. None of it is planned. Breakpoints and stepping in particular
  would need debugger support in the interpreter first.

## Open questions

1. **The command:** `tlua design`, or a mode inside `tlua edit`? I recommend
   `tlua design`. The editor is a terminal program and the designer is not.
2. **Files:** two files per form (layout and code), or one file with a
   designer-owned region at the top? I recommend two. Rewriting a file the
   programmer also edits is how designers lose people's work.
3. **Naming:** VB6's prefixes (`cmd`, `txt`, `lbl`) for default names, or
   `Button1`, `TextBox1`? I recommend the kind plus a number by default, which
   is what VB6 itself did (`Command1`), with renaming encouraged.
4. **Where it lives:** the designer's Lua inside the binary (embedded, like
   the editor), or as an example project people copy? I recommend embedded,
   so `tlua design` works anywhere tlua does.

## First step

Phase 1 alone, as a pull request: `name`, `gui.load`, `gui.dump` and
`gui.kinds`, with docs, `library/gui.lua`, tests, and one example rewritten as
layout plus code. It is useful without the designer, and every later phase
builds on it.
