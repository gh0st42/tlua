# tlua design

`tlua design [directory]` opens a form designer in the manner of Visual Basic
6. It draws forms for the [gui module](gui.md), and what it makes is ordinary
tlua: a folder that runs with `tlua main.lua`, and packs with `tlua fuse` or
`tlua bundle`.
[rad-plan.md](rad-plan.md) is the plan it is being built to; this is what
there is so far.

## Kinds of project

File > New Project asks what kind of project and which folder:
- **GUI application:** forms, designed with the mouse, as the rest of this
  page describes. It is what a folder with a `forms/` folder in it is.
- **Console application:** Lua files for the terminal. The designer is a code
  window with the project's files on the left; there is no form and no
  toolbox. F5 runs `tlua main.lua` and shows its output in the pane below, so
  a program that wants the keyboard or the whole terminal is run from one
  instead.
- **Game:** Lua files for `tlua play`: `main.lua` starts with `boot()`, and
  `_init`, `_update` and `_draw` (see [pico.md](pico.md)). F5 runs `tlua play .`
  and Make EXE and Export Bundle pack it with `-play`.

The two kinds that are only code keep a file `.tlua-project` that says
which (`return { type = "game" }`). Without it a folder with a `main.lua` is a
game when it defines `_draw` or `_update`, and a console program otherwise,
so any folder of Lua can be opened. A new project does not touch files that
are there already; an existing `main.lua` is used as it is.

A console program can become a game and back with Project > Project Type,
which rewrites `.tlua-project`; a GUI application has no other kind. `tlua
design -new console|game|gui myapp` makes the project in `myapp` without
asking, and refuses a folder that is a project already.

In these projects the right pane is an outline of the open file: its
functions with their lines (`function name`, `local function name`, `name =
function`), kept up to date as you type, and a click goes to one. Project > New File starts a Lua file, in a folder if the
name has one (`lib/util.lua`); a double click in the tree opens a file; the
language server completes and shows help as it does for a form's code; and
double-clicking a line of an error in the output pane opens the file at that
line. Forms, components and controls are for GUI applications, and say so
when asked for in the others.

## A project

```
myapp/
  main.lua              shows the first form
  forms/Form1.form.lua  the layout: written by the designer
  forms/Form1.lua       the code: yours
  forms/Form1.d.lua     what the form holds, for a language server: written by the designer
  controls/Counter.lua  a control of the project's own: yours
  components/Field.comp.lua  a component's layout: written by the designer
  components/Field.lua       its code: yours
  controls/Field.lua        what makes it a control: written by the designer
```

- **The layout** (`Form1.form.lua`) is the designer's. It is rewritten
  whole on every save, as data that `gui.load` builds.
- **The code** (`Form1.lua`) is yours. The designer writes it once, to load
  the layout. After that it only adds an empty handler where you ask for
  one, and renames a control in it when you agree to. Handlers go there, by
  the names the layout gives the controls:

```lua
local gui = require "gui"
local frm = gui.load "Form1" --[[@as forms.Form1]]

function frm.Button1:onClick()
  gui.msgbox("Hello")
end

return frm
```

Opened on a directory with no forms in it, the designer offers to start a
new project there.

`tlua design` with no directory asks first which project to work on. It
shows the current folder and offers to open the project in it (or start
one there, when it has none), to choose another folder, or to start with
no project. A folder chosen that has no project is offered as a new one.
`tlua design myapp` opens `myapp` without asking.

## The window

- **Left:** the project's forms (double-click one to open it), and the
  toolbox.
- **Middle:** the form being designed, built with the real controls, on the
  Design tab; its code on the Code tab.
- **Right:** the properties of what is selected, with the form and its
  controls listed at the top.
- **Below:** the output of the program when it runs.

## Designing

- **Placing a control:** pick it in the toolbox and drag out its rectangle on
  the form, or click for one of its own size. Double-clicking in the toolbox
  puts one in the middle of the form. New controls are named as VB6 named
  them: `Button1`, `TextBox1`, ...
- **Containers:** a control drawn inside a Frame or a Panel, or on the page a
  Tabs is showing, goes in it, placed from its corner. Dragging controls
  into a container, or out of one onto the form, moves them there, and they
  stay where they were on the form (the container they would go into is
  outlined green while they are dragged). Deleting a container deletes what
  is in it, and pasting with a container selected pastes into it.
- **Tabs:** a new Tabs has two pages. Clicking the row of tabs of the
  selected Tabs shows its next page, as does its `selected` property; Format
  > Tabs adds and removes pages. Controls on a page that is not showing
  cannot be clicked.
- **Selecting:** click a control to select it, or the form around the
  controls to select the form.
  - Several controls are selected by dragging a rubber band round them (it
    takes what it touches), or by Shift-clicking each.
  - The one selected last has solid handles: it is the one the others line
    up with.
  - Cmd-A selects them all.
- **Changing several at once:** dragging any selected control drags them all,
  and the arrow keys move them all. A property typed in the grid goes to
  every selected control that has it, except `name`.
- **Arranging (Format menu):**
  - Align: lefts, centres, rights, tops, middles, bottoms, all to the control
    with solid handles.
  - Make Same Size: width, height or both.
  - Center in Form, horizontally or vertically.
  - Make the spacing between three or more controls equal, across or down.
- **The grid:** View > Show Grid draws VB6's dots, and Snap to Grid puts what
  is drawn, moved and sized on them, every 8 pixels.
- **Moving and sizing:** drag a control to move it, and drag its handles to
  size it. The arrow keys move it a pixel at a time; Shift and the arrows
  size it.
- **The form:** the handles on its right and bottom edges size the form
  itself.
- **Deleting and stacking:** Delete removes the selected controls. Format has
  Bring to Front and Send to Back.
- **Undo and redo** (Cmd-Z, Cmd-Shift-Z) go back and forward through what was
  done to the form. A caption typed a letter at a time, or a run of arrow
  keys, is one step. In a text box, Cmd-Z undoes the typing instead.
- **Cut, copy, paste and duplicate** (Cmd-X, C, V, D). Pasted controls come
  in a little below and right of the originals, with new names where theirs
  are taken. The designer keeps what was copied itself, so it can be pasted
  into another form, and the system clipboard is left alone.
- **Tab order:** Format > Tab Order numbers the controls that take the
  keyboard. Click them in the order Tab is to visit them, and Escape when
  done. It is saved as each control's `tabIndex`.
- **Menus:** Tools > Menu Editor (Cmd-E) is VB6's.
  - Type an item's caption, a name for the code to know it by, and a
    shortcut such as `Cmd+O`.
  - `>` makes it part of the submenu of the item above, and `<` takes it out
    again. A caption of `-` is a line between items.
  - In the code, the menu's `onClick(name, caption, checked)` says which item
    was picked: double-click the menu to write it.
- **Properties:** a grid of names and values, as VB6's, Delphi's and
  Lazarus's are; it is a Table with editable cells. A click on a value edits
  it in place. Enter or leaving the cell keeps it, Up and Down keep it and
  go on to the next property, and Escape puts it back. A value the control
  will not take stays in the cell, in red, with the reason in the status
  line. Empty means the default.
  - True or false, and a choice such as `font` or `align`, are picked from
    the ▾ at the cell's right, or taken in turn with a double click.
  - A colour's "..." opens the colour chooser; a colour can also be typed,
    as `#rrggbb` or a name.
  - An image's "..." picks a file. One inside the project is named relative
    to `forms/`, where `gui.load` looks for it, as `../images/logo.png`. One
    outside is copied into the project's `images/` folder first, if you
    agree, so that it is packed with the program.
  - A list shows how many items it has. Its "..." edits `items` and
    `columns` one item a line, rows and trees as a Lua table, and a Menu's
    items in the Menu Editor.
  - A property that picks the widget (`multiLine`, `password`, `default`,
    ...) makes the control again.
- **Saving:** Cmd-S (Ctrl-S elsewhere) saves the form. The title bar shows
  `*` while there is something to save, and closing asks first.
- **Running:** F5 saves and runs `main.lua` in its own process, with what it
  prints below; Shift-F5 stops it.
- **Several forms:** Project > Add Form adds one. Project > Set as Startup
  Form makes `main.lua` show the open form first. That is done only while
  `main.lua` is as the designer wrote it; one changed by hand is left alone,
  with a note of what to change. The form `main.lua` starts with is named
  under the list of forms.
- **Make EXE:** File > Make EXE saves everything and packs the project into
  one executable with `tlua fuse`, images and all.
- **Export Bundle:** File > Export Bundle saves everything and packs the
  project into a `.ztl` bundle with `tlua bundle`: one file that
  `tlua app.ztl` runs on any platform, with no executable made for each.

## Components

A component is a group of controls that work together, designed like a form
and used like a control: [gui.md](gui.md#components) describes what it is.
- **Making one:** Project > Add Component asks for a name (a capital first)
  and opens an empty component beside the forms in the project tree. It is
  designed on the same surface, with the same toolbox, properties and code
  window; it has a size, no caption, and no menu.
- **Its files:** `components/Name.comp.lua` is the layout, written whole
  on every save. `components/Name.lua` is yours: a function of the component
  and its parts, which the designer starts and then only adds handlers to,
  inside it, as `function parts.txt:onChange()`. `Name.d.lua` tells a language
  server what `parts` and the component hold. `controls/Name.lua` is written
  again on every save and only says `gui.component`; it is a control of the
  project's own like any in [Controls of your own](#controls-of-your-own).
- **In the toolbox:** a component is among the project's own controls, on
  every form and in other components, drawn as a small picture of its parts.
  A component is not offered inside itself or inside one that holds it, which
  would hold itself.
- **On a form:** an instance is one control. It is placed, moved and given
  props like any, and the layout keeps its kind and props, not its parts.
- **The interface:** Tools > Component Interface says what the component
  shows of itself, in three lists. *Properties* have a name, a type and a
  default, and stand for a property of a part (`lbl.caption`) or are the
  component's own. *Events* are passed on from a part's (`txt.onChange`), or
  have nothing after them and are raised by its code with `self:fire`.
  *Methods* are what its code adds (`function self:clear()`), each with a
  type as the language server reads it (`fun()`, `fun(rgb: string)`), so that
  `frm.field:clear()` completes on a form. *Parts* are private to the
  component unless marked public, which makes them fields of it. The dialog refuses what a component cannot carry: a
  name every control has, a part or property that is not there.
- **Saving it** writes its files and defines it again, so a form opened
  afterwards uses the new one. A prop renamed or taken away in the interface
  is renamed or taken away in the layouts of the forms and components that
  use it, and an event renamed is renamed in their handlers
  (`frm.field:onOld`, `frm.field.onOld`, `function frm.field:onOld`); the
  output pane says which files changed. An event taken away cannot be put
  right in code; the pane lists the code that mentions it instead.

### Palettes

A palette is a folder of components that projects share: two files for each,
`Name.comp.lua` and `Name.lua`. `TLUA_PALETTE` lists the folders, separated
by `:` (`;` on Windows), and without it there is one, `~/.tlua/palette`.
Nothing is written to a palette unless you ask.
- **In the toolbox,** below the project's controls, each palette is a heading
  with the components it holds that the project lacks. A click on one copies
  it into the project, and it is among the controls to place.
- **Project > Save Component to Palette** copies the component being
  designed into a palette (asking which when there are several, and before
  replacing one).
- **Project > Add Component from Palette** lists what the palettes hold and
  copies the one you pick into the project, makes it a control there and
  opens it. `examples/palette` holds three to try. It is a copy, so the project stays complete in itself and packs
  with Make EXE without the palette. A component that uses others says which
  the project lacks.
- **Newer copies.** `components/Name.palette` records where a copy came from
  and what that held; opening the component says when the palette has a
  newer one, and adding it again brings it in. A copy changed here is
  replaced, after asking.

## Help with the code

With a Lua language server on PATH (lua-language-server, say; `TLUA_LSP`
names another, or `off` for none), the code window helps as an IDE's does:

- **Completion:** Ctrl+Space, or typing `.` or `:`, lists what could go at
  the cursor: the gui module's functions, a form's controls by name, their
  properties and handlers, and the project's own modules. Typing on
  narrows the list. Up and Down pick, Enter or Tab put the pick in, and
  Escape closes it. The status line says what the pick is.
- **Help:** the mouse resting on a name shows what it is and its
  documentation, and F1 does the same at the cursor.
- **Calls:** typing `(` or `,` puts what the call takes in the status line,
  with the argument being typed in brackets.

The server is started for the project when it is opened, and told of
tlua's own modules, so `require "gui"` is known on a machine with nothing
but the tlua binary. Each form's `forms/Name.d.lua` tells it of the form's
controls. Nothing waits for it: the window works while it starts, and
without one.

## The documentation

The Help menu opens tlua's documentation, the pages `tlua doc` shows, in a
window of its own beside the designer: F1 opens the contents (except in
the code window, where F1 is the language server's help), and the menu
goes straight to the gui module, the designer and Lua 5.1. Shift+F1 in
the code window opens the documentation on the name at the cursor:
`sys.copy`, `tui.Button`, `gui`; for a name it does not know, such as
`frm:show`, the last part of it, and failing that the contents, with a
word in the status line. Asked again while it is open, the window goes to
the new page; its links, Back and Forward, and search work as in `tlua doc
-g`.

## Controls of your own

A project's own controls are in `controls/`, one to a file, each a
`gui.define` (see [gui.md](gui.md#controls-of-your-own)):
- **Where they come from:** Project > New Control starts one: a Canvas
  drawn with a `value` property, an `onChange` event, and a click that counts
  up, to change into what you need.
- **The toolbox:** when the project opens, every control in `controls/` is
  loaded and has a place at the end of the toolbox, marked U. One that fails
  to load says why in the output pane.
- **On the form:** they are placed, moved and given properties like the
  built-in ones, built for real, so the form shows them as they will be.
  Their `props` are rows in the properties grid; changing one builds the
  control again, so what it draws follows.
- **When the program runs:** `gui.load` finds them itself. A layout's kind
  it does not know is looked for as `controls.<kind>`, so `main.lua` needs
  nothing more, packed with Make EXE or not.

## The language server

Saving a form also writes `forms/Form1.d.lua`. It declares a class,
`forms.Form1`, with a field for every named control and its kind. The code
the designer starts says its `frm` is one:

```lua
local frm = gui.load "Form1" --[[@as forms.Form1]]
```

So an editor with lua-language-server completes `frm.Button1.` with a
Button's fields, and points out `frm.Button1.onClick = 5`. With
[library/gui.lua](../library/gui.lua) in the language server's library, as
the `.luarc.json` at this repository's root has it, everything in the gui
module is known too.

## Code

- **Double-click to write a handler.** Double-clicking a control opens its
  code at the event VB6 went to: `onClick` for a Button, a Label and an
  Image, `onChange` for a TextBox and the other inputs, `onDraw` for a
  Canvas, `onScroll` for a Scroll, `onLoad` for the form. If the handler
  isn't written yet, it is added, empty, just above the `return frm` at the
  end, with the cursor inside it.
- **The two boxes above the code** are VB6's: pick a control (or the form),
  then one of its events, and the code goes to that handler, writing it if
  needed. Events that have a handler already are marked •. They list every
  event the control has: besides the usual ones, `onLoad`, `onActivate` and
  `onDeactivate` for the form; `onFocus` and `onBlur` for the controls that
  take the keyboard; `onShow`, `onHide`, `onResize` and `onPaint` for all;
  `onClick` and the mouse's for a Label and an Image; `onScroll` for a Scroll.
  `docs/gui.md` says what each is for; a handler is written with the
  arguments it is given, as `function frm.Panel1:onPaint(g)`.
- **Switching views:** F7 shows the code and Shift-F7 the form (View menu).
- **The code box:** line numbers and Lua's colours. Tab indents, and Enter
  keeps the indentation. Cmd-F finds, Cmd-G finds the next, Cmd-L goes to a
  line (Ctrl elsewhere). Cmd-Z undoes.
- **Saving:** Save writes the code with the layout. If the code was changed
  in another editor and there is nothing unsaved here, it is read again; if
  there is, you are asked before it is written over. View > Open Code in
  Editor saves and hands the file to whatever opens `.lua` files.
- **Errors take you there.** When the program stops with an error in a form's
  code, the designer opens that code at the line. Double-clicking an error
  in the output does the same.
- **Renaming a control** in the properties offers, once you move on, to
  rename `frm.OldName` to `frm.NewName` in its code, saying how many places
  that is. It is a plain change of text, and nothing changes if you say no.

## Not yet

- **Layouts that arrange themselves:** containers built on go-fltk's Flex and
  Grid, the plan's "maybe later".
- **Things VB6 had that are not planned at all:** data binding, a debugger,
  and reports.

## Its tests

`make test-gui` drives the designer through all of the above with synthetic
input, as well as the gui module's own tests. Nothing in them waits for
someone at the screen:
- every question the designer would ask is answered by the test;
- a dialog that would open anyway fails the test at once;
- anything still waiting after 20 seconds is sent Escape, and fails it.

Typing elsewhere while they run can still reach their windows, so they are
best left alone for the minute they take.
