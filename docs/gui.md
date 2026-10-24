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
| `Form`        | `resizable`, `fullscreen`                            | `onLoad`, `onClose`, `onUnload`, `onKey`, `onKeyPreview`, `onActivate`, `onDeactivate` |
| `Menu`        | `items` (see below); Form only                       | — |
| `Label`       | `align` (`"left"`, `"center"`, `"right"`), `image`; `text` is `caption`; `pointer` | `onClick`, `onDoubleClick`, `onMouseDown`, `onMouseUp`, `onMouseMove`, `onMouseEnter`, `onMouseLeave` |
| `Button`      | `default` (Enter presses it), `image` beside the caption | `onClick` |
| `TextBox`     | `text`, `multiLine`, `password`, `readOnly`; for code `lineNumbers`, `syntax = "lua"`, `acceptsTab`; `line`, `cursor`, `selectedText`, `select(i, j)`, `insert(text)`, `pointAt(pos)` | `onChange`, `onKey`, `onHover` |
| `CheckBox`    | `checked` (also `value`)                             | `onChange` |
| `RadioButton` | `checked` (also `value`); one per parent is on       | `onChange` |
| `ComboBox`    | `items`, `selected` (1-based, 0 for none), `text`    | `onChange` |
| `ListBox`     | `items`, `selected`, `text`                          | `onChange`, `onDoubleClick` |
| `Tree`        | `items` (see below), `path`, `text`                  | `onChange`, `onDoubleClick`, `onToggle` |
| `Table`       | `columns`, `rows`, `columnWidths`, `selected`, `editable` | `onChange`, `onDoubleClick`, `onStartEdit`, `onEdit`, `onEditButton` |
| `Canvas`      | drawn by its `onDraw` (see below); `color` is the background; `transparent` shows what is under it; `pointer` is the mouse pointer over it | `onDraw`, `onClick`, `onDoubleClick`, `onMouseDown`, `onMouseUp`, `onMouseMove`, `onMouseDrag`, `onMouseWheel`, `onMouseEnter`, `onMouseLeave`, `onKey` |
| `MarkdownView` | formatted text with links (see [below](#formatted-text-markdownview)): `text`, `file`, `textSize`, `face` | `onLink`, `onNavigate`, `onHover` |
| `MarkdownEdit` | Markdown edited as it looks (see [below](#editing-markdown-markdownedit)): `textSize`, `paper` | `onChange`, `onSelect`, `onMenu`, `onLink`, `onHover` |
| `Slider`      | `min`, `max`, `step`, `value`, `vertical`            | `onChange` |
| `Spinner`     | `min`, `max`, `step`, `value`                        | `onChange` |
| `ProgressBar` | `min`, `max`, `value`; `caption` is drawn on the bar | — |
| `Image`       | `file` (PNG, JPEG, BMP, SVG, GIF), `fit`; `pointer`  | the same as a Label's |
| `Frame`       | holds controls inside a captioned border             | — |
| `Panel`       | holds controls, with no border or caption            | — |
| `Scroll`      | holds controls, showing part of them with scrollbars; their positions are measured from its content's corner; `scrollX`, `scrollY` | `onScroll` |
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
[Context menus](#context-menus). Every kind but a Menu also raises `onShow`,
`onHide`, `onResize` and `onPaint`, and the controls that take the keyboard
`onFocus` and `onBlur`: see [Events of the window and its controls](#events-of-the-window-and-its-controls).

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
`onKeyPreview(self, key, text)` sees each key pressed in a control before
the control has it, as VB6's KeyPreview did; returning true keeps it from the
control. It is for the few keys a form means whatever has the focus: Space
to play and stop in a music editor, where a focused checkbox would otherwise
tick. Keys it lets go of go on as before, to the control and then, unused,
to `onKey`.

### Events of the window and its controls

Besides what the user does to a control, a program hears of what happens to
it. The events are listed for each kind in the table above and by
`gui.kinds()`; the designer's event box lists them as it does the others.

**A form's life.**
- `onLoad(self)` is the form's first event: it is built and on screen, and
  the program has finished setting it up. It comes from the event loop, after
  the main chunk has run, so handlers written after `form:show()` hear it. It
  is the place to fill lists and set what the controls start with. It comes
  once each time the form is built, so again when a form that was closed is
  shown anew (`onUnload` is its other end).
- `onShow(self)` follows it, and comes again whenever the form is shown after
  being hidden (`form.visible = false`, then `true`); `onHide(self)` comes
  when it is hidden, or closed, after `onUnload`.
- `onActivate(self)` is the form's window being given the keyboard (clicked,
  or switched to), and `onDeactivate(self)` its losing it to another window.
  The keyboard moving from one of the form's controls to another is neither.
  It is how a game knows to pause, or a program to look again at a file.
- Nothing on a form hears of anything before `onLoad` has finished: the
  events there would have been are told right after it, in the order a user
  would have met them (the form and its visible controls shown, anything that
  changed size, the window activated, the control with the keyboard given it).

**Showing and hiding.** `onShow` and `onHide` are raised for any control that
becomes visible or stops being so: by `visible`, by its form being shown or
closed, or, for a Page, by its tab being chosen or another. A Page's
`onShow` is where its contents can be filled in when first looked at.

**Size.** `onResize(self)` is raised when a control's, or a form's, size
changes, whether the program changed it, or its container did following the
form. It is raised once for a change, with the new size already the object's,
and not for a move. A handler that sizes its own object again is not asked
about that. A form's moves are not heard.

**Keyboard.** `onFocus(self)` and `onBlur(self)` are raised for the controls
that take the keyboard (a Canvas takes it only if it has `onKey`), once each
time: `onBlur` is raised before the next control's `onFocus`, so a handler
that checks what was typed in a box on `onBlur` has done so by the time the
next one starts. A window that loses the keyboard takes it from its control,
so that control hears `onBlur`, and `onFocus` when the window has it back.

**Painting.** `onPaint(self, g)` is raised each time a control is drawn,
after it has drawn itself and what it holds, with a `g` that draws inside it
and nowhere else, as a Canvas's `onDraw` does (see
[Canvas](#canvas): the same calls, in the control's
own coordinates). It is for what goes over a control: a mark on a picture, a
grid over a panel, a border of one's own. To draw under controls, use a Canvas
below them. `g` only works inside the handler; ask for a redraw with
`control:redraw()`. A Canvas has `onDraw` for this, and a Menu has neither.

```lua
function meter:onPaint(g)
  local w, h = g:size()
  g:color("#ffffff")
  for i = 0, 10 do
    local x = math.floor(w * i / 10)
    g:line(x, 0, x, 8)
  end
end
```

**Clicks and the mouse over a Label, an Image or a Canvas.** A Label or an
Image, which a click does nothing to, hears the mouse as a Canvas does:
`onMouseDown(self, x, y, button, double, mods)`, `onMouseUp(self, x, y,
button)`, `onMouseMove(self, x, y)`, `onMouseEnter` and `onMouseLeave`, with
x and y from its corner. They also have `onClick(self, x, y)`, raised when a
press and the release of it are both over the control with the left button,
and `onDoubleClick(self, x, y)` after it when that press was a double click;
a Canvas has these two as well. `pointer = "hand"` (or `"cross"`, `"text"`,
`"move"`, `"wait"`, `"help"`) shows that mouse pointer over a Label or an
Image as it does over a Canvas, which makes a link of a Label. A Label with
none of these does what it did before: nothing, but start a drag when it has
`onDrag`.

**Scrolling.** `onScroll(self, x, y)` is raised when a Scroll's content was
scrolled, by the wheel, by its bars or by setting `scrollX` and `scrollY`, to
x, y; once for however far it went before the program heard.

[`examples/gui/events.lua`](../examples/gui/events.lua) logs all of them as
they happen.

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
- A Form's `fullscreen` makes its window fill the screen when set to true,
  with no frame, and gives it back its size when false. It may be set
  before the form is shown, and reads back what is in effect.
- Inside a Frame or Page, the same rule applies to its children.
- A Scroll's `scrollX` and `scrollY` are how far its content is scrolled, in
  pixels, from 0 at the corner. Set them to scroll, as a hand tool or a
  "scroll to here" does; a value past the end is the end.

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

Wherever a font is named (`font`, `g:font`, `gui.measure`), a font
installed on the system can be named too: `"Menlo"`, `"Perfect DOS VGA 437
Win"`. `gui.fonts()` lists the names there are, the three first, so a
program can pick one that is there and fall back to `"mono"`. Naming one
that is not there is an error. A font with no bold or italic face of its
own is drawn plain.

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
- **Colours.** Cells are drawn in the colours `gui.colors` gave for text
  boxes and text, so a table follows a dark look as a ListBox does; `color`
  and `textColor` on the Table give its cells colours of their own.
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
works inside `onDraw`, or inside `onPaint`, which gives the same `g` to any
other control to draw over it (see [Events of the window and its
controls](#events-of-the-window-and-its-controls)):

| Call | Draws |
|------|-------|
| `g:size()` | returns the canvas's width and height |
| `g:color(c)` | sets the colour for what follows (`"#rrggbb"` or a name) |
| `g:width(n)` | sets the line width |
| `g:font(name, size [, style])` | sets the font: `"sans"`, `"serif"`, `"mono"` or an installed font's name; `style` is `"bold"`, `"italic"` or `"bold italic"` |
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

`canvas.pointer` is the mouse pointer while the mouse is over it:
`"default"`, `"hand"`, `"text"`, `"cross"`, `"move"`, `"wait"` or `"help"`.
Setting it from `onMouseMove` gives a hand over what can be clicked.

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

## Formatted text: MarkdownView

A MarkdownView shows Markdown as formatted text, and follows its links: for
help pages, notes beside a form, or a small wiki of `.md` files.

```lua
local help = form:MarkdownView { left = 8, top = 8, width = 400, height = 300,
                                 file = "help/index.md" }   -- or text = "# Hi\n..."
function help:onLink(url)
  if url:match("^app:") then runCommand(url:sub(5)) return true end
end
help:open("editing.md#undo")   -- another page, at a heading
help:back()
```

- **What it shows.** Headings, paragraphs, **bold**, *italic*, `code`, links,
  lists (bulleted and numbered, nested), quotes, code blocks, rules,
  pictures, and tables as GitHub writes them:

  ```markdown
  | Key    | Does        |
  |:-------|------------:|
  | Ctrl+C | copies      |
  ```

  A table's columns are as wide as their widest cell when they fit, and
  share out the room when they do not, the text in the cells wrapping. The
  header row is bold, and `:---`, `:---:` and `---:` align a column left,
  centred or right. `\|` is a pipe inside a cell.
- **`[[toc]]`**, on a line of its own, is a list of the page's headings,
  each a link to its heading, indented by its level.
- **`[[Page Name]]`** is a link to the page `Page Name.md` beside this one,
  which is how a wiki links its pages. `[[Page Name|other words]]` shows
  other words, and `[[Page Name#heading]]` goes to a heading on it. When
  there is no `Page Name.md`, `Page-Name.md` is shown, as a GitHub wiki
  names its pages.
- **`text` and `file`.** `text` is the Markdown shown. Giving `file` reads
  the page from that file into `text`; a relative path is looked for next to
  the script, as an Image's is, and out of the program's own files first in
  a fused program or a bundle. Links and pictures on the page are read from
  the page's folder.
- **`pictures`**, when the program sets it, holds pictures by the `src` a
  page gives them, a `png` image each (or a table with one in its `image`
  field), as a MarkdownEdit's does: for pages whose pictures are not files,
  such as ones kept in a zip. Those it does not have are read as before.
- **`textSize`** is the size of its text, 15 unless given. Headings, spaces
  and indents grow with it. `color` is the background and `textColor` the
  text's colour.
- **`face`** draws all its text, code too, in one font, by its name: a
  terminal's look with `face = "mono"`, or an installed font's
  (`gui.fonts()` lists them). `""`, as it starts, is sans with code in mono.
- **`colors`**, a table, sets the colours it draws with, by name; those it
  does not give are the usual ones: `background`, `text`, `link`,
  `heading`, `quote`, `quoteBar`, `codeText`, `codeBg`, `codeLine`,
  `inlineCode`, `rule`, `selection`, `tableLine`, `tableHead`,
  `tableStripe`, and the scrollbar's `bar` and `thumb`. Its `background`
  and `text` win over `color` and `textColor`.
- **Links.** A click on a link calls `onLink(self, url)` first. Returning
  true says the program followed it. Otherwise:
  - `#anchor` scrolls to the heading on the page with that anchor, which is
    made as GitHub makes them: `## Getting Started!` is `#getting-started`.
  - A path to a `.md` file, or with no extension, opens that page in the
    view, `#anchor` and all, and can be gone back from. A name with no
    extension is looked for with `.md` after it too.
  - `http:`, `https:`, `mailto:` and `file:` addresses open with
    `gui.openurl`, in the browser or the mail program.
  - Any other file opens with `gui.openurl` too, in its own program.
  - Any other scheme, such as `app:`, is left to `onLink`.
- **Pages.** `view:open(path)` shows a page, as a link to it would; it
  returns false and why when the file cannot be read, and shows that. Then
  `view:back()`, `view:forward()`, `view:canGoBack()` and
  `view:canGoForward()` go through the pages seen. `onNavigate(self, file)`
  says another page is shown, to update buttons or a title.
  `view:follow(url)` does what a click on the link does, for an `onLink`
  that handles some links and leaves the rest.
- **Headings.** `view:headings()` lists the page's headings, each with
  `level`, `text` and `anchor`, for a list of contents.
  `view:scrollTo(anchor)` brings one to the top.
- **Hovering.** Over a link the pointer is a hand, and
  `onHover(self, url)` says which link; it is called with nil when the mouse
  leaves it, so a status line can show where a link goes.
- **Selecting.** Dragging selects text, a double click a word.
  `view:selectedText()` is the text selected, `view:copy()` puts it on the
  clipboard, and `view:selectAll()` selects everything.
  `view:search(text)` selects where text is next found, the case of letters
  aside, and scrolls to it.
- **Keys.** Clicked on, it takes the keyboard. Up, Down, Page Up, Page Down,
  Space, Home and End scroll; Ctrl+C (Cmd+C) copies and Ctrl+A selects all;
  Alt+Left and Alt+Right (Cmd+[ and Cmd+]) go back and forward.

`examples/gui/help.lua` is a help window built on one: a list of the page's
headings beside it, back and forward buttons, and a search box.

A MarkdownView is written in Lua, on a Canvas, as a control of your own
would be, out of three modules any program can use as well:

- `markdown` reads Markdown into blocks, `markdown.parse(text)`, and writes
  them back, `markdown.write(blocks)`.
- `markdown.layout` lays the blocks out on a page of a given width.
- `markdown.render` draws them with a Canvas's `g`.

- `markdown.editor` edits them: typing, Enter and Backspace, styles, the
  kinds of block, lists, links, pictures, the clipboard and undo, with no
  GUI in it. A MarkdownEdit is one drawn on a Canvas.

## Editing Markdown: MarkdownEdit

A MarkdownEdit edits Markdown as it looks, the way a word processor does:
what it holds is Markdown, and what it shows is the formatted text, with the
caret and the selection.

```lua
local edit = form:MarkdownEdit { left = 0, top = 30, width = 600, height = 400, grow = true }
edit:load("# Notes\n\nSee [[Ideas]].\n", pictures)   -- text, and the pictures it shows
function edit:onChange() saveLater(edit:markdown()) end
function edit:onLink(url) openNote(url) end            -- a Cmd- or Ctrl-click on a link
edit.ed:toggle("b") edit:changed()                     -- a Bold button
```

- **The document.** `edit:load(text or blocks [, pictures])` shows a
  document and starts undo afresh; `edit:markdown()` is what it holds now.
- **Editing.** Typing, Enter, Backspace and Delete, Shift+Enter for a line
  break, Tab and Shift+Tab to indent a list item, the arrows (Alt or Ctrl
  for words, Cmd or Home and End for the line), Page Up and Down, Shift to
  select, a double click for a word. `# `, `## `, `- `, `1. ` and `> ` at a
  paragraph's start make it a heading, a list item or a quote, and `---`
  then Enter a rule. `[[toc]]` and pictures are kept whole: the caret
  steps over each as one character.
- **Tables.** A click puts the caret in a cell, and a cell is edited like a
  line of text: typing, Backspace, Delete and the arrows (which go on into
  the next cell and out of the table at its ends) work in it, Tab and
  Shift+Tab go to the next cell or the one before (a Tab past the last
  adds a row), Enter goes down a cell, and Escape leaves the table. The
  program adds `edit.ed:insertTable(rows, columns)`, `insertRow(after)`,
  `insertColumn(after)`, `deleteRow()`, `deleteColumn()` and
  `setAlign("left" | "center" | "right" | false)`, the last four on the
  caret's row or column; each is followed by `edit:changed()`. A cell holds
  one line of styled text; a line break typed or pasted becomes a space.
- **What a program adds.** `edit.ed` is the `markdown.editor` underneath:
  its `toggle("b")`, `setType("h2")`, `setLink(url)`, `insertImage(src)`,
  `undo()`, `copyText()`, `paste(text)` and the rest are a program's menus
  and toolbar. After calling one, `edit:changed()` shows the change and
  raises `onChange`; `edit:moved()` is for the caret alone. Cmd and Ctrl
  keys are left to the menus.
- **Pictures.** `edit.pictures` holds the pictures the document shows, by
  the `src` it gives them: a `png` image, or a table with one in its
  `image` field. Where they come from (a zip, a folder) is the program's.
- **Links.** Over a link the pointer is a hand and `onHover(self, url)`
  says which. A Cmd- or Ctrl-click on one calls `onLink(self, url)`; a wiki
  link `[[Page Name]]` gives `"Page Name.md"`. A link to a heading,
  `#anchor` (a `[[toc]]` entry's), goes there unless `onLink` returns true.
- **Headings.** `edit:headings()` lists them, as a MarkdownView's do, and
  `edit:scrollTo(anchor)` puts the caret at one and brings it to the top.
- **Events.** `onChange` when the document changes, `onSelect` when the
  caret or the selection moves, `onMenu(self, x, y)` for a right click.
- **Looks.** `paper = true` shows a page on a grey desk, as microword
  does; otherwise it fills with its `color`. `textSize` is 15 unless given.
  The text column is at most 700 wide, at size 15, in the middle.

microword and micronotes, in turboapps, are built on it.

## Drag and drop

- **Receiving.** Any control, or the form itself, takes drops once it has
  `onDrop(self, text, lines, x, y, at)`. `text` is what was dropped. Files
  dragged in from the desktop arrive as their paths, one per line, which
  `lines` holds ready split. `x` and `y` are where it landed, from the
  control's corner.
- **Onto a line.** On a Tree, `at` is the path of the node it landed on, and
  on a ListBox the number of the line; nil when it landed below them all.
  While something is dragged over one, the line under it is selected, to
  show where it would land, and what was selected before comes back
  afterwards, without `onChange`. A Tree that is both dragged from and
  dropped on moves its own nodes: a note dragged into a folder, say.
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

### Components

A composite control can be described as data instead of built in code: a
**component** is a layout with a Frame at its top, and `gui.component`
defines it with `gui.define`, so it is made and used like any other:

```lua
-- components/LabeledField.comp.lua
return {
  kind = "Component", name = "LabeledField", width = 240, height = 28,
  props  = { label = { type = "string", default = "Name", to = "lbl.caption" },
             text  = { type = "string", to = "txt.text" } },
  events = { onChange = "txt.onChange", "onSubmit" },
  { kind = "Label",   name = "lbl", left = 0,  top = 4, width = 80 },
  { kind = "TextBox", name = "txt", left = 84, top = 0, width = 156 },
}
```

```lua
gui.component {
  layout = "components/LabeledField",   -- or the table itself
  code = function(self, parts)          -- optional: what the layout cannot say
    function self:clear() parts.txt.text = "" end
  end,
}
local field = form:LabeledField { label = "Surname", left = 10, top = 10 }
```

- **The layout** is read as `gui.load` reads one: a path without `.lua`
  means its `.comp.lua` file, found next to the script that names it. The
  top says `kind = "Component"` (or `"Frame"`), its `width` and `height` are
  the component's defaults, and its parts are in the array part.
- **Props.** `props` is as in `gui.define`; a prop with `to = "part.property"`
  stands for that property of a part: assigning it sets the part, reading it
  reads the part, and a layout keeps it. A prop with no `to` is a field of
  the component, set to its default when it is made.
- **Events.** `events = { onChange = "txt.onChange" }` raises the component's
  `onChange` when that part's does, with the same arguments after the
  component. A name alone, as `"onSubmit"`, is raised by the code with
  `self:fire`.
- **Parts are private.** The names of the parts belong to the component: two
  `LabeledField`s on a form have a `txt` each, a form cannot reach `field.lbl`
  (and `form.lbl` is nothing), and a part cannot clash with a name on the
  form. A part with `public = true` is a field of the component, as
  `field.txt`. `code` gets all the parts as its second argument.
- **Methods.** `methods = { clear = "fun()" }` in the layout lists, with a
  type each, what the component's code adds as methods. The runtime ignores
  it; it is for a designer to put in the declarations a language server
  reads.
- **Code.** `code(self, parts)` runs once for each component made, after the
  parts are built and the props and events wired. It is for methods, and for
  handlers that tie the parts together. It can also be the name of a module
  that returns that function.
- **On a form,** an instance is one control: `gui.dump` and `gui.save` write
  its kind and props, not its parts, so changing a component changes every
  form that uses it. A component can hold components; one that holds itself
  is an error.

- **Again.** Calling `gui.component` again with the name of a component
  replaces it: forms made from then on use the new one, and those made
  before keep their parts. That is how a designer applies a change; a name
  made by `gui.define` or a built-in kind is never replaced.

`examples/gui/components.lua` uses one.

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
form again builds it afresh from those values, and raises `onLoad` again. So
a form made for one question, on each click, does not pile up. Replacing an
Image's `file` frees the picture it showed.

## Looks

`gui.scheme(name)` gives every window one of FLTK's looks: `"base"`, FLTK's
own and the default, `"gtk+"`, softer edges, `"gleam"`, glossy gradients,
`"plastic"`, a striped background, or `"oxy"`, smooth gradients and
borderless text boxes. It can be called before a form is shown or after,
and returns the look in effect; `gui.scheme()` only returns it.

`gui.colors(t)` gives every window colours of its own, for a dark look
say. It takes `background` (what windows and controls are made of),
`background2` (what text boxes and lists are) and `foreground` (their text,
and labels), each `"#rrggbb"` or a colour name, and any of them may be left
out. It can be called before a form is shown or after, and returns what has
been asked for. A control's own `color` and `textColor` still win over it.
FLTK has no call for going back to its own colours.

```lua
gui.colors { background = "#202020", background2 = "#141414", foreground = "#d0d0d0" }
```

The user has the last word: `TLUA_SCHEME=oxy tlua app.lua` runs any program
in that look, whatever it asks for, and `gui.scheme` then returns the
user's. `tlua -E` ignores the variable, as it does the other `TLUA_` ones.

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

`gui.openurl(target)` opens a web address in the browser, a `mailto:`
address in the mail program, or a file or folder in the program the system
opens it with. It returns true once that program has started, or nil and
why.

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
