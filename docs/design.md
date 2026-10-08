# tlua design

`tlua design [directory]` opens a form designer in the manner of Visual Basic
6. It draws forms for the [gui module](gui.md), and what it makes is ordinary
tlua: a folder that runs with `tlua main.lua`, and packs with `tlua fuse`.
[rad-plan.md](rad-plan.md) is the plan it is being built to; this is what
there is so far.

## A project

```
myapp/
  main.lua              shows the first form
  forms/Form1.form.lua  the layout: written by the designer
  forms/Form1.lua       the code: yours
```

- **The layout** (`Form1.form.lua`) is the designer's. It is rewritten
  whole on every save, as data that `gui.load` builds.
- **The code** (`Form1.lua`) is yours. The designer writes it once, to load
  the layout, and never touches it again. Handlers go there, by the names the
  layout gives the controls:

```lua
local gui = require "gui"
local frm = gui.load "Form1"

function frm.Button1:onClick()
  gui.msgbox("Hello")
end

return frm
```

Opened on a directory with no forms in it, the designer offers to start a
new project there.

## The window

- **Left:** the project's forms (double-click one to open it), and the
  toolbox.
- **Middle:** the form being designed, built with the real controls.
- **Right:** the properties of what is selected, with the form and its
  controls listed at the top.
- **Below:** the output of the program when it runs.

## Designing

- **Placing a control:** pick it in the toolbox and drag out its rectangle on
  the form, or click for one of its own size. Double-clicking in the toolbox
  puts one in the middle of the form. New controls are named as VB6 named
  them: `Button1`, `TextBox1`, ...
- **Selecting:** click a control to select it, or the form around the
  controls to select the form.
- **Moving and sizing:** drag a control to move it, and drag its handles to
  size it. The arrow keys move it a pixel at a time; Shift and the arrows
  size it.
- **The form:** the handles on its right and bottom edges size the form
  itself.
- **Deleting and stacking:** Delete removes the selected control. Edit has
  Bring to Front and Send to Back.
- **Properties:** a property changes as it is typed. A value the control will
  not take turns red, with the reason in the status line. Empty means the
  default.
  - Lists (`items`, `columns`) are edited one item a line.
  - Rows, trees and menus are edited as a Lua table.
  - A property that picks the widget (`multiLine`, `password`, `default`,
    ...) makes the control again.
- **Saving:** Cmd-S (Ctrl-S elsewhere) saves the form. The title bar shows
  `*` while there is something to save, and closing asks first.
- **Running:** F5 saves and runs `main.lua` in its own process, with what it
  prints below; Shift-F5 stops it.

## Not yet

These are later phases of the plan:
- the code window, and double-clicking a control to write its event handler;
- placing controls inside a Frame or on a Tabs page;
- multiple selection, alignment and the grid;
- undo;
- the menu editor;
- tab order;
- Make EXE.
