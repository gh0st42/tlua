# Desktop GUI

`require("gui")` builds windows out of forms and controls, and FLTK puts them
on screen. It needs a tlua built with cgo on macOS, Linux, OpenBSD or
Windows/amd64. Everywhere else, `make static` included, the module still
loads and builds forms, but showing one raises "built without FLTK".

`examples/gui/kitchensink.lua` uses everything described here.
[library/gui.lua](../library/gui.lua) declares it all for lua-language-server,
so an editor completes the controls and their properties, and shows what each
handler is given. The `.luarc.json` at the root of this repository points at
it.

## Two ways to run

A script can use the module like any other one. It builds a form and calls
`form:show()`, which returns once the form is closed. That works from a file,
from `-e` and at the prompt:

```lua
local gui = require("gui")
local form = gui.Form { caption = "Hi" }
form:Button { caption = "Close", left = 16, top = 16, onClick = function() form:close() end }
form:show()
print("closed")
```

A GUI application says `bootgui()` at the top instead, the way a game says
`boot()`. From then on `form:show()` puts the form up and returns at once, so
the rest of the file can go on to write its handlers. Once the file has
finished, the event loop runs for as long as any form is open. With `-i`, the
prompt comes after that. `bootgui()` returns the module:

```lua
local gui = bootgui()
local form = gui.Form { caption = "App" }
form:show()
function form:onClose() print("bye") end -- written after show(), still in time
```

A program says `boot()` or `bootgui()`, not both.

## Forms and controls

Every object is made from a table of its properties, plus any handlers:

```lua
local form = gui.Form { caption = "Title", width = 400, height = 300 }
local ok = form:Button { caption = "OK", left = 16, top = 16, onClick = function(self) ... end }
```

- `form:Button{}` makes a control on that form. The same works on a Frame or
  a Page.
- `gui.Button{}` puts the control on the Form made most recently. A control
  made before any Form belongs to none until a container's `:add(control)`
  adopts it, and `parent = container` names one outright.
- `:add()` also moves a control from one container to another.
- `left` and `top` are measured from the container's corner, so controls in
  a Frame or a Page are placed within it.

Controls added after a form is shown appear straight away. `obj:remove()`
takes a control off its container and frees what it had on screen; it can be
added somewhere again. `obj:raise()` and `obj:lower()` put it in front of or
behind its siblings, which is also the order a layout lists them in.
`obj.parent` is what holds it.

| Kind          | Own properties                                       | Events |
|---------------|------------------------------------------------------|--------|
| `Form`        | `resizable`                                          | `onClose`, `onUnload`, `onKey`, `onResize` |
| `Menu`        | `items` (see below); Form only                       | — |
| `Label`       | `align` (`"left"`, `"center"`, `"right"`), `image`; `text` is `caption` | — |
| `Button`      | `default` (Enter presses it), `image` beside the caption | `onClick` |
| `TextBox`     | `text`, `multiLine`, `password`, `readOnly`; for code `lineNumbers`, `syntax = "lua"`, `acceptsTab`; `line`, `cursor`, `selectedText`, `select(i, j)`, `insert(text)`, `pointAt(pos)` | `onChange`, `onKey`, `onHover` |
| `CheckBox`    | `checked` (also `value`)                             | `onChange` |
| `RadioButton` | `checked` (also `value`); one per parent is on       | `onChange` |
| `ComboBox`    | `items`, `selected` (1-based, 0 for none), `text`    | `onChange` |
| `ListBox`     | `items`, `selected`, `text`                          | `onChange`, `onDoubleClick` |
| `Tree`        | `items` (see below), `path`, `text`                  | `onChange`, `onDoubleClick`, `onToggle` |
| `Table`       | `columns`, `rows`, `columnWidths`, `selected`, `editable` | `onChange`, `onDoubleClick`, `onStartEdit`, `onEdit`, `onEditButton` |
| `Canvas`      | drawn by its `onDraw` (see below); `color` is the background; `transparent` shows what is under it | `onDraw`, `onMouseDown`, `onMouseUp`, `onMouseMove`, `onMouseDrag`, `onMouseWheel`, `onMouseEnter`, `onMouseLeave`, `onKey` |
| `Slider`      | `min`, `max`, `step`, `value`, `vertical`            | `onChange` |
| `Spinner`     | `min`, `max`, `step`, `value`                        | `onChange` |
| `ProgressBar` | `min`, `max`, `value`; `caption` is drawn on the bar | — |
| `Image`       | `file` (PNG, JPEG, BMP, SVG, GIF), `fit`             | — |
| `Frame`       | holds controls inside a captioned border             | — |
| `Panel`       | holds controls, with no border or caption            | — |
| `Scroll`      | holds controls, showing part of them with scrollbars; their positions are measured from its content's corner | — |
| `Splitter`    | holds controls that tile it edge to edge; the user drags the lines between them | — |
| `Tabs`        | holds Pages, made with `tabs:Page{caption = ...}`; `selected` | `onChange` |

The controls that take the keyboard also have `tabIndex`. Once any control
on a form has one, Tab goes in that order: those with an index first, by
it, then the rest as the form holds them. Shift-Tab goes back.

Every object also has `name` (see [Forms in files](#forms-in-files)), `caption`, `left`, `top`, `width`, `height`, `visible`,
`enabled` and `tooltip`, plus `color`, `textColor`, `font` (`"sans"`,
`"serif"`, `"mono"`) and `fontSize`. Colours are `"#rrggbb"`, `"#rgb"`, or one
of black, white, gray, red, green, blue, yellow, orange and purple. A script
can keep fields of its own on an object, too (`button.tag = 3`).

Some details:

- **Defaults.** A Form is 360×240 and most controls are 120×28. A form opens
  in the middle of the screen unless it is given `left` or `top`.
- **Reading properties.** Properties are read from the screen, so `text`,
  `checked`, `selected`, `value` and the size and position are always what
  the user left them as.
- **Fixed when made.** `multiLine`, `password`, `readOnly`, `default`,
  `vertical`, `resizable` and `grow` can only be given when the object is
  made.
- **Lists.** `items`, `rows` and `columns` are read when they are assigned:
  to change a list, change the table and assign it again.
- **Image paths.** A relative image path is looked for next to the script that
  names it.
- **TextBox captions.** A TextBox has no caption of its own; put a Label
  beside it.

Every kind also raises `onDrop` and `onDrag`; see [Drag and drop](#drag-and-drop).
Every kind also has a `contextMenu`, and raises `onContextMenu`; see
[Context menus](#context-menus).

## Handlers

A handler gets the object as `self`, so `self == button` holds. A handler is
assigned like a property, given in the table the object is made from, or set
with `obj:on("click", fn)`, which is the same as `obj.onClick = fn`. Setting a
handler to nil removes it. Naming an event the kind does not have is an error
that lists the ones it has. `obj:fire("click", ...)` calls an object's
handler for that event, if it has one, with `self` and the arguments given,
and returns what the handler returns.

`onClose` runs for the close box, Escape and `form:close()` alike. It can
return false to keep the form open; so can `onUnload`, which runs next.
`onKey(self, key, text)` gets keys the focused control did not use. They are
named the way shortcuts are written: `"a"`, `"Ctrl+s"`, `"Shift+Tab"`, `"F5"`,
`"Escape"`, and `"Cmd+..."` on a Mac. Returning true keeps the key from going
any further, menu shortcuts and Escape included.

An error in a handler stops the event loop and is reported where the loop was
started. Without `bootgui()`, it surfaces from `form:show()`, so `pcall` can
catch it. Under `bootgui()`, it stops the program like any other error. The
forms are closed either way.

## Layout

Forms do not resize unless they say so:

- `grow = true` on a control lets the form be resized. That control
  stretches. Controls in the same columns widen with it, and controls in the
  same rows grow taller.
- Everything else keeps its size and moves with the edge beyond it.
- `resizable = true` with nothing marked `grow` scales everything.
- A form never shrinks below the size it opened at.
- Inside a Frame or Page, the same rule applies to its children.

## Text boxes for code

A multi-line TextBox can be made for editing code. These are given when it
is made:
- `lineNumbers = true` shows line numbers.
- `syntax = "lua"` colours keywords, the standard library's names, strings,
  numbers and comments. The colours are the terminal editor's scanner, so the
  two agree.
- `acceptsTab = true` makes Tab put in two spaces rather than move to the
  next control, and Enter start the new line as far in as the one before.

Any TextBox, code or not, has these:
- `line` is the line the cursor is on, from 1. Setting it moves there.
- `cursor` is how many bytes come before the cursor. Setting it moves there
  and shows it.
- `selectedText` is what is selected.
- `box:select(i, j)` selects bytes `i` to `j`, counted as `string.sub` counts
  them, so `box:select(text:find("needle", 1, true))` finds and selects.
- `box:insert(text)` puts text in place of the selection, or at the cursor,
  leaves the cursor after it, and raises `onChange`.

What a code editor of your own needs, a completion list for one, comes
from these:
- `onKey(self, key, text)` sees each key before the box does, named as a
  Form's `onKey` names them (`"Ctrl+Space"`, `"Down"`, `"a"`). Returning
  true keeps the key from the box.
- `onHover(self, pos)` is the mouse resting on the text, at `pos` as
  `cursor` counts. It is raised again with `nil` when the mouse moves on,
  or a key is pressed.
- `box:pointAt(pos)` is where position `pos` is shown: `x` and `y` of its
  top left, from the box's own, and the height of its line. Add the box's
  `left` and `top` to place a control under it in the same container.
- A control laid over a multi-line TextBox, made after it in the same
  container, stays drawn over it while the box is typed into.

tlua design's code window is built this way: see
`internal/design/lua/assist.lua`.

## Menu

```lua
form:Menu {
  { "&File", {
    { "&Open...", function() ... end, shortcut = "Cmd+O" },
    "-",                                       -- a line between items
    { "&Quit", function() form:close() end, shortcut = "Cmd+Q" },
  } },
  { "&View", {
    { "&Wrap", function(caption, on) ... end, checked = true },  -- a toggle
    { "&Later", function() end, enabled = false },
  } },
}
```

A Menu also raises `onClick(self, name, caption, checked)` for every item
picked, after the item's own function if it has one. `name` is the item's
`name` field. That is how the code tells the items of a menu written down in
a layout apart, since a layout has no functions in it:

```lua
form:Menu { { "&File", { { "&Open...", name = "mnuOpen", shortcut = "Cmd+O" } } } }
function form.Menu1:onClick(name)
  if name == "mnuOpen" then ... end
end
```

A Panel can hold a Menu too, across its own width.

A toggle writes `checked` back into its item as the user changes it. So a
menu changed after it is made (renamed, enabled, checked) is the items
table changed and assigned again, and it keeps what the user left it as.

An `&` marks the letter to underline. `Cmd` is Command on a Mac and Ctrl
elsewhere. The menu sits along the top of the form, so leave it 25 pixels.

## Context menus

Any control, and a form, can have a `contextMenu`: a right click on it (a
Ctrl-click on a Mac) shows it at the mouse. Its items are written as a
Menu's, and a caption alone is an item too.

```lua
list.contextMenu = {
  { "&Open", function() open(list.text) end },
  { "Re&name", function() rename(list.text) end, shortcut = "F2" },
  "-",
  { "&Delete", name = "delete", enabled = canDelete },
}
function list:onContextMenu(name, caption, checked)
  if name == "delete" then ... end
end
```

- **Which menu.** A right click shows the menu of the innermost control
  under the mouse that has one: a button's own, or else the Panel's or the
  form's it is in.
- **Lists first.** A ListBox, Tree or Table selects the line under the
  mouse before its menu comes up, so the menu is about that line.
- **What runs.** The item's own function runs, with `(caption, checked)`
  as a Menu item's does. Then the control's `onContextMenu(self, name,
  caption, checked)` runs, which is how a menu written in a layout, with no
  functions in it, is answered in code. tlua design writes a control's
  context menu in the Menu Editor, from the `contextMenu` property's "...".
- **Shortcuts.** A shortcut is shown beside its item, but works only while
  the menu is up.
- `nil` or an empty list is no menu, and a right click is then just a
  click, which a Canvas's `onMouseDown` reports as button 3.

`gui.popup(items [, near])` shows a menu at the mouse whenever the script
asks, from a handler. It returns the picked item's `name`, or its caption
without the `&`, and whether it is checked. It returns nothing when the menu
was closed without a pick, and the item's own function runs as well. The
menu appears over `near`'s form, or over the form the last click or key was
in:

```lua
function canvas:onMouseDown(x, y, button)
  if button == 3 then
    local choice = gui.popup { "Cut", "Copy", "-", "Paste" }
    if choice == "Cut" then ... end
  end
end
```

`gui.measure(text [, font [, size]])` is the width and height text takes
when drawn in a font (`"sans"`, `"serif"` or `"mono"`, sans unless said) and
size (14 unless said), a line a `"\n"`. With it a Label, a tip or a box
drawn on a Canvas can be made to fit what it shows.

`gui.platform` says what the program runs on, as Go names it (`"darwin"`,
`"linux"`, `"windows"`), for the odd thing that differs, as a Mac's
Ctrl-click does.

## Tree

A Tree's `items` are the script's own tables. A string is a leaf, and
`{"label", {children...}}` is a branch, which shows its children when its
`open` field is true:

```lua
local files = { "README", { "src", { "main.lua", { "lib", { "util.lua" } } }, open = true } }
local tree = form:Tree { items = files, onChange = function(self) print(self.path) end }
tree.path = "src/lib/util.lua"   -- opens src/lib on the way, and selects it
```

- **Selection.** `path` is the selected node's labels joined with `/`, or ""
  when nothing is selected; `text` is its own label.
- **Opening and closing.** Clicking a ▸ or ▾, double-clicking a branch, or
  pressing Right or Left opens or closes it. That writes `open` back into the
  script's table and raises `onToggle(self, path, open)`.
- **Changes.** After changing the tables, assign `items` again.

## Table

```lua
local grid = form:Table {
  columns = { "Name", "Year" },
  columnWidths = { 120, 60 },          -- optional; otherwise they share the width
  rows = { { "Lua", 1993 }, { "Go", 2009 } },
  onChange = function(self) print(self.selected) end,
}
```

- **Rows.** Each row is a table of cells, and the table shows them as text.
- **Selection.** `selected` is the selected row, 1 for the first and 0 for
  none. The user picks rows by clicking or with the arrow keys.
- **Changes.** To change the data, change the tables and assign `rows` again
  (sorting them in place, for instance).

### Editing cells

A Table with `editable` set is a data grid: its cells are edited in place,
in an input laid over the cell. `editable = true` edits every column, and a
list such as `editable = { 2, 3 }` edits only those.

- **Starting.** A click on an editable cell edits it. So do F2 and Enter on
  the selected row, and typing on it, which starts with what was typed.
  `grid:edit(row, col)` starts one from the script, and `grid:editing()`
  says which cell is being edited, if any.
- **Finishing.** Enter keeps what was typed, and so does leaving the cell.
  Up and Down keep it and edit the cell above or below. Escape puts the cell
  back. `grid:edit()` with no arguments keeps it from the script.
- **onStartEdit(self, row, col)** says how a cell is edited before it is.
  Return `false` to refuse, nothing for a plain input, or a table:
  - `choices`: a list to pick from, opened with a ▾ at the cell's right. A
    double click takes the next choice, as Delphi's did.
  - `button = true`: a "..." at the cell's right, which raises
    `onEditButton(self, row, col)`. That is where a value is edited in a
    dialog of the script's own.
  - `readOnly = true`: nothing typed, so only the button changes the value.
- **onEdit(self, row, col, text)** is the value being kept. Return `false`
  to refuse it, and the input stays, in red, for the user to put right. A
  string is kept in its place, so a value can be tidied up. Otherwise the
  text goes into the row's table as it is. onEdit is only raised when the
  text has changed.

```lua
local grid = form:Table {
  columns = { "Name", "Year" }, rows = { { "Lua", "1993" } },
  editable = { 2 },
  onEdit = function(self, row, col, text)
    if not tonumber(text) then return false end
  end,
}
```

## Canvas

A Canvas is drawn by its `onDraw(self, g)`, which runs whenever it needs
drawing. `canvas:redraw()` asks for that, and so does assigning any of its
properties, fields of the script's own included. Coordinates are the canvas's own,
with 0, 0 at its top left, and nothing drawn spills outside it. `g` only
works inside `onDraw`:

| Call | Draws |
|------|-------|
| `g:size()` | returns the canvas's width and height |
| `g:color(c)` | sets the colour for what follows (`"#rrggbb"` or a name) |
| `g:width(n)` | sets the line width |
| `g:font(name, size [, style])` | sets the font: `"sans"`, `"serif"` or `"mono"`; `style` is `"bold"`, `"italic"` or `"bold italic"` |
| `g:point(x, y)`, `g:line(x1, y1, x2, y2)` | a point, a line |
| `g:rect(x, y, w, h)`, `g:fill(x, y, w, h)` | a rectangle, outlined or filled |
| `g:circle(x, y, r)`, `g:disc(x, y, r)` | a circle, outlined or filled |
| `g:arc(x, y, w, h, a1, a2)`, `g:pie(...)` | part of an ellipse in that box, from angle a1 to a2 in degrees |
| `g:polygon(x1, y1, x2, y2, x3, y3, ...)`, `g:loop(...)` | a shape filled (convex shapes), or outlined |
| `g:text(s, x, y)` | text with its top left at x, y |
| `g:text(s, x, y, w, h [, align])` | text in a box, `"left"`, `"center"` or `"right"` |
| `g:measure(s)` | returns the width and height of s in the current font |
| `g:image(file, x, y [, w, h])` | an image file, or a picture from `require "png"`, scaled to w by h if given |
| `g:scaling(how)` | how the images after it are drawn bigger or smaller: `"smooth"` blends their pixels, as at the start of each draw; `"nearest"` repeats them, sharp, as pixel art wants |

Mouse handlers get positions in the same coordinates:
- `onMouseDown(self, x, y, button, double, mods)` and
  `onMouseUp(self, x, y, button)`. Button 1 is left, 2 middle, 3 right.
  `double` is true for the second click of a double click. `mods` names the
  modifier keys held, as `"Shift"` or `"Ctrl+Shift"`, and `""` for none.
- `onMouseDrag(self, x, y)` while a button is held down.
- `onMouseMove(self, x, y)` when no button is held.
- `onMouseWheel(self, dx, dy)`.
- `onMouseEnter(self)` and `onMouseLeave(self)`. Entering also counts as a
  move.
- `onKey(self, key, text)` gives a Canvas the keyboard. Clicking it takes the
  focus, and keys are named as a Form's `onKey` names them. Returning true
  keeps the key from going further.

A Canvas with other controls on top of it is drawn again together with
them, so redrawing it never paints over them.

### Pictures of drawings

What a Canvas draws can be kept as a picture, a `png` image, to save as a
PNG file or draw elsewhere:

```lua
local pic = chart:snapshot(2)          -- the Canvas as its onDraw draws it
pic:save("chart.png")

local badge = gui.paint(120, 40, function(g)   -- any drawing, no Canvas needed
  g:color("#2f6fd8") g:fill(0, 0, 120, 40)
  g:color("white") g:text("tlua", 0, 0, 120, 40, "center")
end)
```

- **`canvas:snapshot([scale])`** runs the Canvas's `onDraw` again, into a
  picture of its size. It need not be on screen.
- **`gui.paint(w, h, fn [, scale])`** calls `fn(g)` with a `g` that draws
  into a picture of `w` by `h` units, and returns it. `g:size()` is `w, h`.
- **Scale.** A picture has `scale` pixels to a unit, 1 unless given. At 2
  every coordinate, line width and font size is doubled, so the picture is
  twice as big and as sharp as a Retina screen shows it; `g:measure` still
  answers in units, so the drawing code is the same at any scale.
- **What it is drawn by.** FLTK draws it, offscreen, with the fonts and
  smoothing the screen has. The ground is white: a picture has no
  transparent pixels. It needs the display, like a window does.
- An error in the drawing is raised from `snapshot` or `paint`.

A `transparent = true` Canvas draws only what `onDraw` draws, over the
controls under it, and takes the mouse before they do. That is how a
designer puts handles over a form.

## Drag and drop

- **Receiving.** Any control, or the form itself, takes drops once it has
  `onDrop(self, text, lines)`. `text` is what was dropped. Files dragged in
  from the desktop arrive as their paths, one per line, which `lines` holds
  ready split.
- **Sending.** A control with `onDrag(self)` can be dragged out of: when the
  user drags from it, `onDrag` returns the text to carry. That text can be
  dropped on another control, or into another program.
- **Defaults.** Text boxes take dropped text by themselves, unless they have
  an `onDrop` of their own.

## Clipboard

`gui.clipboard()` returns the text on the clipboard, and `gui.clipboard(text)`
puts text there.

## Controls of your own

A control of your own is a function that builds it out of the built-in ones.
`gui.define` gives that function a name, and then the new control is made and
used like any other:

```lua
gui.define {
  name = "Rating",                 -- a capital, like the built-in kinds
  events = { "onChange" },         -- events of its own, beside its base's
  build = function(parent, opts)   -- makes it in parent and returns it
    local c = parent:Canvas { width = 120, height = 24 }
    c.value = opts.value or 0
    function c:onDraw(g)
      for i = 1, 5 do
        g:color(i <= self.value and "#f0b400" or "#d0d0d0")
        g:disc(i * 24 - 12, 12, 10)
      end
    end
    function c:onMouseDown(x)
      self.value = math.floor(x / 24) + 1   -- assigning redraws it
      self:fire("onChange", self.value)
    end
    return c
  end,
}

local stars = form:Rating { left = 16, top = 16, value = 3,
  onChange = function(self, n) print(n, "stars") end }
stars.value = 5
```

How it works:

- **Where it goes.** `container:Rating{...}` builds it in that Form, Frame or
  Page. `gui.Rating{...}` builds it in the Form made most recently, or in
  `parent =`.
- **What `build` gets.** `build(parent, opts)` gets the container and the
  table the control was made from. It reads its own options there (`value`,
  above) and returns the control it made: usually a Canvas, or a Frame holding
  other controls.
- **What happens afterwards.** The control is placed by the `left`, `top`,
  `width`, `height` and other common properties in that table. The handlers
  there are set, for its own events and its base's alike. It calls itself
  `Rating` in errors and in `tostring`.
- **Raising its events.** `self:fire("onChange", ...)` calls whatever handler
  the user gave. Naming an event the control was not defined with is an error.
- **Its properties.** `props = { value = { type = "integer", default = 0 } }`
  in the definition declares the properties a designer offers, a layout keeps,
  and `gui.kinds()` reports. They take the types above. Given when the control
  is made, they are set on it after `build`. A prop cannot be one every
  control has already.
- **Its own state.** State lives in fields of its own (`c.value`). A drawn
  control reads them in `onDraw`, so assigning one from outside redraws it.
- **Methods.** A field holding a function serves as a method: write
  `function c:set(v) ... end` inside `build`, and the user calls
  `stars:set(4)`.

There are two common shapes:

- **Drawn controls.** These are a Canvas with `onDraw` and the mouse
  handlers: a rating, a switch, a knob, a chart. `onMouseMove` and
  `onMouseLeave` give hover effects.
- **Composite controls.** These are a Frame holding other controls, which
  `build` wires together: a colour picker of three sliders and a swatch, a
  labelled field, a search box with its button. Their handlers can close over
  the parts and over the Frame itself, which is what the user holds.

`examples/gui/custom.lua` defines one of each.

## Forms in files

A form can be written down as a **layout**: a Lua table with the form's kind
and properties, and its controls, each written the same way, in its array
part. A designer writes these, and they can be written by hand too:

```lua
-- forms/Main.form.lua
return { kind = "Form", name = "Main", caption = "Hello", width = 320, height = 160,
  { kind = "Label", name = "lblName", caption = "Name", left = 16, top = 16, width = 60 },
  { kind = "TextBox", name = "txtName", left = 80, top = 16, width = 224 },
  { kind = "Button", name = "cmdGreet", caption = "Greet", left = 204, top = 112, width = 100 },
}
```

The code that goes with it loads the layout and wires up its controls by
name:

```lua
-- forms/Main.lua
local gui = require "gui"
local frm = gui.load "Main"           -- forms/Main.form.lua, beside this file

function frm.cmdGreet:onClick()
  gui.msgbox("Hello, " .. frm.txtName.text .. "!")
end

return frm
```

`examples/gui/layout` is a program of two forms written this way.

### Names

- **Reaching controls.** A control's `name` makes it a field of its form:
  `frm.cmdGreet`, at any depth (a control in a Frame or on a Tabs page is the
  form's too). `obj:find("cmdGreet")` finds it from anywhere on the form, and
  returns nil when there is none.
- **Unique per form.** Names are unique within a form, and a control moved to
  another form takes its name along.
- **Valid names.** A name is a Lua identifier, and it cannot be something
  `frm.<name>` already means: a property, a method, a kind, an event
  (`onX`), or a Lua keyword. Assigning a field of the script's own to a
  control's name is refused too.

### `gui.load`, `gui.dump` and `gui.save`

- **`gui.load(layout [, parent])`** builds what a layout describes and
  returns its top.
  - `layout` is a table, or the path of a file returning one. A path without
    `.lua` at the end means its `.form.lua` file.
  - A path is looked for next to the script that names it, and in a fused
    program's packed files.
  - A layout file runs with nothing in scope: it is data, and cannot call
    anything.
  - A kind it does not know is looked for as the module `controls.<kind>`,
    which is expected to `gui.define` it. A project keeps its own controls in
    `controls/`, and its layouts find them there.
  - Its top is a Form, unless `parent` is given to build it in.
  - An error says which part of the layout it is about:
    `gui.load forms/Main.form.lua, at fraOpts.chkBold: font must be ...`.
- **`gui.dump(obj)`** is the layout of an object as it stands. Values are
  read from the screen, so what the user typed or ticked is in it.
  - Only what differs from the defaults is written.
  - Handlers and fields of the script's own are left out; they are code.
  - An Image's `file` is written as the script gave it.
- **`gui.save(obj or layout, path)`** writes that layout, or a layout table
  as it is, as a Lua file, in a steady
  order: kind, name, caption, position and size first, then the rest
  alphabetically. Saving what was loaded gives back the same file.

### `gui.kinds()`

`gui.kinds()` describes every kind, built in or defined, as a fresh table:

```lua
local k = gui.kinds().Label
k.props.align   --> { type = "choice", default = "left", choices = { "left", "center", "right" } }
k.props.color   --> { type = "color" }
k.events        --> { "onDrop", "onDrag" }
k.width, k.height, k.holds
```

- **Property types:** `string`, `number`, `integer`, `boolean`, `color`,
  `choice` (with `choices`), `file`, `name`, `list`, `rows` (a Table's),
  `tree` (a Tree's items) and `menu` (a Menu's).
- **Fixed properties:** `fixed = true` marks a property that can only be given
  when the control is made.
- **Defined controls** say `defined = true`. They list the placement
  properties every control has, plus the `props` they were defined with.

## Packing an application

`tlua fuse -o myapp myapp/` packs a GUI program into one executable, as it
packs a game. A program that says `bootgui()` needs no flag. Its images go
in with it: an Image's `file` and `g:image` read from the packed files before
the disk, and a relative path is looked for next to the script that names it,
inside the package too. The executable is the tlua it was packed with, so
pack with a tlua built with the GUI (`make build`).

`tlua bundle myapp/` packs it into `myapp.ztl` instead: one file, read the
same way, that `tlua myapp.ztl` runs on any platform with a GUI-built tlua,
without an executable made for each.

## Closing and showing again

Closing a form frees its window and everything in it. What the user left in
its controls stays readable: `name.text` is still what was typed. Showing the
form again builds it afresh from those values. So a form made for one
question, on each click, does not pile up. Replacing an Image's `file` frees
the picture it showed.

## Running another program

`gui.spawn{command, args..., dir = ..., onOutput = fn, onExit = fn}` runs a
program beside this one:
- `onOutput(line, stream)` gets what it prints, a line at a time, with
  `stream` saying `"stdout"` or `"stderr"`.
- `onExit(code)` gets its exit code.
- Both run on the GUI's thread, like any handler, while a form is up.
- It returns a process with `kill()` and `running()`.

`gui.interpreter` is the tlua that is running, for running another Lua
program with it.

## Dialogs and timers

These are module functions; any of them can be called from a handler.

- `gui.msgbox(message [, buttons [, title]])` returns the name of the button
  pressed. `buttons` is `"ok"`, `"okcancel"`, `"yesno"`, `"yesnocancel"` or
  `"retrycancel"`. Closing the box any other way answers with the last
  button.
- `gui.inputbox(prompt [, title [, default]])` returns the text typed (which
  may be empty), or nil when cancelled.
- `gui.openfile(opts)`, `gui.savefile(opts)` and `gui.choosedir(opts)` take a
  title, or a table of `title`, `filter` (`"Lua\t*.lua\nAll\t*"`), `dir` and
  `file`. They return a path or nil. `openfile{multiple = true}` returns a
  list.
- `gui.choosecolor(opts)` shows FLTK's colour chooser. It takes a title, or a
  table of `title` and `color`, the colour it starts at (`"#rrggbb"` or a
  name). It returns the colour picked as `"#rrggbb"`, or nil when cancelled.
- `form:showModal()` shows a form of your own in front of the others and
  waits until it is closed, with or without `bootgui()`.
- `gui.after(seconds, fn)` and `gui.every(seconds, fn)` run `fn` later, or
  repeatedly until it returns false. Both return a timer with `:stop()`.
  Timers run while a form is up.
