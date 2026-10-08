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
  the layout. After that it only adds an empty handler where you ask for
  one, and renames a control in it when you agree to. Handlers go there, by
  the names the layout gives the controls:

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

## Code

- **Double-click to write a handler.** Double-clicking a control opens its
  code at the event VB6 went to: `onClick` for a Button, `onChange` for a
  TextBox and the other inputs, `onDraw` for a Canvas, `onClose` for the
  form. If the handler isn't written yet, it is added, empty, just above the
  `return frm` at the end, with the cursor inside it.
- **The two boxes above the code** are VB6's: pick a control (or the form),
  then one of its events, and the code goes to that handler, writing it if
  needed. Events that have a handler already are marked •.
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

These are later phases of the plan:
- placing controls inside a Frame or on a Tabs page;
- multiple selection, alignment and the grid;
- undo;
- the menu editor;
- tab order;
- Make EXE.
