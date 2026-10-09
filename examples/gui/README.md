# GUI examples

Desktop GUI scripts for tlua's `gui` module; docs/gui.md describes it. They
need a tlua built with cgo (`make build`, not `make static`).

- `hello.lua` - a minimal window with a label and a button.
- `form.lua` - an editable text box and a status label.
- `dialog.lua` - an input box before the form, then a form with the answer.
- `kitchensink.lua` - every control, the menu, tabs, a tree and a table, a
  canvas to paint on, dialogs, file choosers, the clipboard, drag and drop,
  timers, keys, resizing, colours and fonts, a modal form, and a close guard.
- `layout/` - a program of two forms laid out in files (`forms/*.form.lua`)
  and wired up by name in code (`forms/*.lua`), as a designer would write it.
- `custom.lua` - controls of your own with `gui.define`: a star rating drawn
  on a Canvas, and a colour picker made of sliders in a Frame.
- `paint.lua` - a pixel editor: a picture from `require "png"` drawn on a
  Canvas, painted with the mouse, opened and saved as PNG, and packed into
  a zip with `require "zip"`.
