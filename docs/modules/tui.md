# `tui` — Turbo Vision-style text UI widgets

`tui` is a small, keyboard-driven widget toolkit built on top of
[`term`](term.md), in the spirit of Borland's Turbo Vision: windows,
labels, buttons, checkboxes, radio buttons, a text input, a list box, a
status bar, and modal dialogs, all using simple single-inheritance OOP.

```lua
local tui = require("tui")
```

It comes from turbolua. See
[examples/console/tui_demo.lua](../../examples/console/tui_demo.lua) for a
complete, runnable demo using every widget below.

## Design at a glance

- **Full-screen redraw.** Every loop iteration clears and redraws the
  whole screen; there's no dirty-rect tracking. Simple, and plenty fast
  for a keyboard-driven form/dialog-style UI.
- **Parent-relative coordinates.** A widget's `x`/`y` are relative to its
  parent's *content origin* — for a plain container that's just the
  parent's own top-left corner, but a [`Window`](#tuiwindow) insets its
  children by one cell on each side for the border.
- **One focus chain.** `Tab`/`Shift-Tab` cycle focus among the
  *innermost* container's focusable children (see [Keys](#keys)) — there
  is no multi-window manager; a modal [`Dialog`](#tuidialog) is drawn on
  top of the normal screen with its own nested event loop instead.
- **ASCII only**, by design: `+`/`-`/`|` borders (no Unicode box-drawing
  glyphs to worry about width/encoding for) and single-byte character
  input (no multi-byte UTF-8 editing in `InputLine`).
- **Keyboard only** — there's no mouse support.

## Keys

| Key(s) | Effect |
|---|---|
| `Tab` / `Shift+Tab` | move focus to the next/previous focusable widget in the current window/dialog |
| `Enter` / `Space` | activate the focused widget (press a button, toggle a checkbox, ...) |
| `Up` / `Down` | move within the focused widget (a `ListBox`'s selection, a `RadioButtons` group) |
| `Left` / `Right`, `Home`, `End`, `Backspace`, `Delete` | edit the focused `InputLine` |
| `PageUp` / `PageDown` | scroll a focused `ListBox` by a page |
| `Esc` | cancel/close the current modal `Dialog` |
| `Ctrl+Q` | quit the `Application` |

Only plain arrow/navigation keys are recognized — modified combinations
like `Shift+Up` or `Ctrl+Right` aren't decoded and are treated as a
stray `Esc`.

## Building blocks

### `tui.class(base)`

The tiny single-inheritance helper every widget (including your own) is
built with: `class(base)` returns a new class table whose instances
inherit from `base`; define an `init(self, ...)` method to run on
`YourClass.new(...)`.

```lua
local tui = require("tui")
local term = require("term")

local Spinner = tui.class(tui.View)

function Spinner:init(x, y)
    tui.View.init(self, x, y, 1, 1)
    self.frame = 1
end

function Spinner:draw()
    local ax, ay = self:abspos()
    term.gotoxy(ax, ay)
    io.write(("|/-\\"):sub(self.frame, self.frame))
end
```

### `tui.View` — base class

Every widget (including containers) is a `View`. Fields: `x`, `y`, `w`,
`h` (relative to the parent), `visible`, `focusable`, `parent`,
`children` (an array, present once you `addchild` something).

- `View.new(x, y, w, h)` / `view:init(x, y, w, h)`
- `view:abspos()` — returns this view's absolute screen `x, y`.
- `view:origin()` — where this view's *own children's* `(1, 1)` lands on
  screen; override to inset content (this is how `Window` makes room for
  its border).
- `view:addchild(child)` — adds `child`, setting `child.parent = view`;
  returns `child`.
- `view:draw()` — draws this view (default: just draws its children).
- `view:handlekey(key)` — handle a decoded key (see [Keys](#keys));
  return `true` if consumed. The default implementation delegates to the
  focused child first, then handles `Tab`/`Shift-Tab` itself.
- `view:focusables()` / `view:focused()` / `view:focusnext()` /
  `view:focusprev()` — focus-cycling among `visible` children that are
  `focusable`, or that contain something focusable (so a `Window` of
  widgets takes focus and passes keys on to them).
- `view:isfocused()` — `true` if this view is its parent's currently
  focused child.

## Widgets

### `tui.Label`

Static, single-line text.

```lua
local label = tui.Label.new(x, y, "Name:")
```

### `tui.Button`

`Button.new(x, y, text, onclick)` — a `[ Text ]` button; `onclick(self)`
fires on `Enter`/`Space` while focused.

```lua
win:addchild(tui.Button.new(3, 10, "OK", function(btn)
    print("clicked")
end))
```

### `tui.CheckBox`

`CheckBox.new(x, y, text, checked)` — a `[x] Text` toggle; `.checked`
holds the current state, and `onchange(self, checked)` (set it after
construction) fires when it's toggled.

```lua
local agree = tui.CheckBox.new(3, 5, "I agree", false)
agree.onchange = function(self, checked) print("agree:", checked) end
```

### `tui.RadioButtons`

`RadioButtons.new(x, y, items, selected)` — a `(•) item` group where
`items` is an array of strings; `Up`/`Down` move the selection, `.selected`
is the 1-based index, and `onchange(self, selected)` fires on
`Enter`/`Space`.

```lua
local size = tui.RadioButtons.new(3, 5, { "Small", "Medium", "Large" }, 2)
```

### `tui.InputLine`

`InputLine.new(x, y, w, text, maxlen)` — a single-line, horizontally
scrolling text field. `.text` holds the current content; the real
terminal cursor is shown and positioned at the edit point while it's
focused.

```lua
local email = tui.InputLine.new(3, 3, 30, "", 254)
```

### `tui.ListBox`

`ListBox.new(x, y, w, h, items)` — a scrollable, single-select list.
`.selected` is the 1-based index; `onselect(self, index, item)` (set it
after construction) fires on `Enter`.

```lua
local list = tui.ListBox.new(3, 3, 20, 6, { "One", "Two", "Three" })
list.onselect = function(self, i, item) print("picked", item) end
```

### `tui.StatusBar`

`StatusBar.new(y, w, text)` — a single non-interactive line, typically
placed at the bottom of the screen for key hints.

```lua
app:addchild(tui.StatusBar.new(app.h, app.w, "Esc: quit"))
```

### `tui.Window`

`Window.new(x, y, w, h, title)` — a bordered, optionally-titled
container. Add children with `win:addchild(...)`; they're positioned
relative to the window's interior (inset by the border).

```lua
local win = tui.Window.new(5, 3, 40, 15, "Settings")
app:addchild(win)
```

### `tui.Dialog`

A `Window` that can run its own nested, modal event loop — the building
block for prompts and forms that should block interaction with the rest
of the screen until dismissed.

- `dialog:close(result)` — ends the modal loop, recording `result`.
- `dialog:runmodal(app)` — draws the dialog on top of `app`'s current
  screen and handles keys (including `Esc` to close with a `nil` result)
  until `close` is called; returns whatever was passed to `close`.

```lua
local dlg = tui.Dialog.new(10, 5, 30, 8, "Confirm")
dlg:addchild(tui.Label.new(2, 2, "Really delete this?"))
dlg:addchild(tui.Button.new(2, 5, "Yes", function() dlg:close(true) end))
dlg:addchild(tui.Button.new(10, 5, "No", function() dlg:close(false) end))

if dlg:runmodal(app) then
    print("deleted")
end
```

### `tui.messagebox(app, text, buttons)`

A ready-made alert/confirm dialog: centers itself over `app`, shows
`text` (a `\n`-separated string becomes multiple lines), and one button
per entry in `buttons` (default `{ "OK" }`). Returns the label of the
button pressed, or `nil` if dismissed with `Esc`.

```lua
if tui.messagebox(app, "Delete this file?", { "Yes", "No" }) == "Yes" then
    os.remove(path)
end
```

### `tui.Application`

The root container and event loop.

- `Application.new()` — sizes itself to the current terminal
  (`term.size()`, falling back to `80x24` if it can't be detected).
- `app:addchild(view)` — add top-level content (typically one `Window`
  filling most of the screen, plus perhaps a `StatusBar`).
- `app:run()` — enables raw input mode and hidden cursor, then loops:
  redraw, read one key, route it, until `app:quit()` is called (or
  `Ctrl+Q` is pressed, which `Application` handles by default). Restores
  normal terminal mode on the way out — even if a widget callback raises
  an error (which is then re-raised after cleanup).
- `app:quit()` — stops the loop after the current iteration.

```lua
local app = tui.Application.new()
local win = tui.Window.new(2, 2, app.w - 4, app.h - 4, "Hello")
app:addchild(win)
app:run()
```

## Theming

`tui.theme` is a plain table of `{ fg = ..., bg = ... }` color pairs
(color names/indices as accepted by [`term.textcolor`](term.md)), keyed
by widget kind (`window`, `normal`, `focus`, `selected`, `button`,
`statusbar`, and `desktop.bg`). Mutate it before calling `app:run()` to
restyle every widget at once:

```lua
tui.theme.focus = { fg = "black", bg = "lightgreen" }
tui.theme.window.background = "black"
```

## Limitations

- No mouse support, and no full pull-down menu bar (a `StatusBar` hint
  line is provided instead) — both are plausible future additions if
  needed.
- Only one focus chain deep at a time — there's no window manager for
  multiple simultaneously visible top-level windows; use modal
  `Dialog`s stacked via `runmodal` instead.
- `InputLine` edits single-byte characters only; typing multi-byte UTF-8
  text will corrupt it byte-by-byte.
- `Ctrl+C` or SIGTERM while `app:run()` waits for a key ends the program
  (a read from the terminal cannot be interrupted), and tlua puts the
  terminal back as it found it first. Only a process killed outright
  (SIGKILL) can leave it in raw/no-echo mode; run `stty sane` to recover.
