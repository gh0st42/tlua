# GUI implementation progress

Last updated: 2026-10-08

## Locked decisions

- Desktop GUI activation uses boot-style startup via `bootgui()`.
- The existing pico/game runtime stays unchanged in phase 1.
- FLTK will be introduced behind an optional build path so the default pure-Go build stays intact.

## Current work

- [x] Capture the implementation plan and scope.
- [x] Lock the runtime activation policy to `bootgui()`.
- [x] Add the GUI bootstrap hook to the interpreter.
- [x] Scaffold the GUI runtime package boundary.
- [x] Add the first Lua-facing API surface.
- [x] Add Lua example GUI applications.
- [x] Add tests for the bootstrap and API contract.
- [x] Add the backend seam for the future FLTK implementation.
- [x] Make the demo GUI scripts runnable through the current backend.
- [x] Wire an FLTK backend behind build tags: `fltk.go` needs cgo on a platform go-fltk ships libraries for; everywhere else `nofltk.go` builds the same object tree and `show`/`msgbox`/`inputbox` raise "built without FLTK".

## Open points (review 2026-10-08)

Done
- [x] Build-tag split (`fltk.go` / `nofltk.go`), `go mod tidy`, and the main thread locked explicitly in `fltk.go`'s init.
- [x] `bootgui()` makes the program a GUI application: `show()` returns at once and the loop runs after the script, until every form is closed, then `-i` if asked. Without it `show()` blocks until its form closes. Saying both `boot()` and `bootgui()` is an error.
- [x] Event loops wait on their own windows with a bounded `fltk.Wait`, so `inputbox` works from a handler.
- [x] Handler errors stop the loop and are raised where it started (`show()`, or reported by the boot loop); the forms are closed.
- [x] Size defaults per kind; forms centred unless given a position.
- [x] A form's window is shown only once its children exist.
- [x] One userdata per object: `self == button` in handlers.
- [x] Events are `onX` everywhere; `obj:on("click")` is `obj.onClick`; nil removes; unknown names are errors that list the valid ones.
- [x] Per-state app replaces the global `currentForm`; `parent =` and `:add()` (which also moves) for explicit placement.
- [x] `msgbox` buttons: ok, okcancel, yesno, yesnocancel, retrycancel. go-fltk's `ChoiceDialog` panics with three buttons, so the dialogs are our own.
- [x] `inputbox` returns "" when cleared, and Enter presses OK.
- [x] `grow`/`resizable` layout; TextBox captions dropped (they drew outside the box).
- [x] `show()` twice reuses the window; controls added to a shown form appear.
- [x] New kinds: Menu, CheckBox, RadioButton, ComboBox, ListBox, Slider, Spinner, ProgressBar, Image, Frame, Tabs/Page. Password/read-only TextBox, default Button. Module: openfile, savefile, choosedir, after, every, showModal. Properties: tooltip, color, textColor, font, fontSize, align. Form: onKey, onResize.
- [x] docs/gui.md; examples README.
- [x] library/gui.lua declares the module for lua-language-server, as library/pico.lua does the console; the examples check clean against it. Unknown keys in the table a control is made from are not flagged; lua-language-server does not check table literals for those.
- [x] Forms in files (phase 1 of docs/rad-plan.md): control names reached as `frm.<name>` and `find`, unique per form; `gui.load` from a table or a sandboxed layout file (beside the calling script, or in a fused archive); `gui.dump` from what is on screen, defaults left out; `gui.save` in a steady order that round-trips; `gui.kinds()` with property types, defaults, fixed flags and choices; `gui.define` takes `props`. `examples/gui/layout` is two forms written this way.
- [x] Extending the designer, phase 5 of docs/rad-plan.md:
  - controls placed in, dragged into and out of, and pasted into Frames, Panels and Tabs pages; pages added and removed;
  - a project's own controls (`controls/*.lua`) in the toolbox, which `gui.load` finds by itself when a program runs;
  - `forms/Name.d.lua` stubs, so a language server knows each form's controls.
- [x] Redrawing a Canvas painted over the controls on top of it: FLTK draws only the widget asked for. Show Grid in the designer made the form's controls disappear. A Canvas that is transparent or covered now has what holds it drawn again.
- [x] The GUI tests never wait for a person: designer questions are answered through hooks, unexpected dialogs fail, a watchdog sends Escape, and `make test-gui` runs the packages one at a time.
- [x] VB6's comforts, phase 4 of docs/rad-plan.md:
  - rubber-band and Shift-click selection;
  - the Format menu's aligning, sizing, centring and spacing;
  - a grid with snapping;
  - undo and redo;
  - cut, copy, paste and duplicate;
  - tab order (`tabIndex` in the runtime);
  - the Menu Editor (a Menu's `onClick` by item `name`);
  - a startup form;
  - Make EXE, and Export Bundle (`tlua bundle`, a `.ztl` any tlua runs).
- [x] The code window, phase 3 of docs/rad-plan.md:
  - double-clicking a control writes or finds its handler;
  - VB6's object and event boxes;
  - a code TextBox (`lineNumbers`, `syntax = "lua"`, `acceptsTab`, `line`, `cursor`, `selectedText`, `select`), coloured by internal/luasyntax, which the terminal editor now shares;
  - find and go to line;
  - jumping to a run error;
  - renaming a control in the code too;
  - reading code changed elsewhere;
  - menu toggles keep `checked`.
- [x] A crash: go-fltk's FLTK does not bounds-check `Fl_Menu_::value(int)`, so a ComboBox with nothing selected pointed before its items. Out-of-range selections are no longer passed, and a regression test churns ComboBoxes.
- [x] `tlua design`, phase 2 of docs/rad-plan.md: a form designer written in Lua on the gui module, embedded in the binary (internal/design). The runtime gained `remove`, `raise`, `lower`, `parent`, Panel, Scroll and Splitter, `image` on Button and Label, transparent Canvases with `onKey`, and `gui.spawn`. `make test-gui` drives the designer: placing, moving, sizing, deleting, the property grid, saving, and running a program it made.
- [x] Fused GUI programs: `runFused` installs `bootgui()` and runs the GUI loop, and images (Image and `g:image`) are read from the archive before the disk.

Still open
- [x] Real input is tested. `make test-gui` (`TLUA_GUI_TESTS=1`) opens windows and drives them through FLTK's own `Fl::handle()`, with the event fields set as a real event would set them. `internal/gui/fltkinput` reaches those by declaring the few FLTK statics it needs; go-fltk links the library. A `TestMain` serves FLTK on the main thread, which is what kept `go test` out before. The tests cover:
  - clicks on buttons, check boxes, radio buttons and tabs, and a disabled button that ignores them;
  - typing into plain, password, multi-line and read-only text boxes;
  - form keys, Escape and the close guard;
  - menu shortcuts, toggles and disabled items;
  - list, tree and table clicks, double-clicks and arrow keys;
  - canvas mouse and wheel;
  - drops and drags out;
  - every msgbox and inputbox path, including one opened from a handler;
  - a modal form.
  They are opt-in because they take over the screen briefly, and typing elsewhere meanwhile can break them.
- [x] The tests found a drop bug: FLTK offers a drop that nothing under the mouse took to every control in turn, and any control with `onDrop` took it, wherever it was dropped. Controls now take only drops on themselves. A form's own `onDrop` gets the rest through an invisible catcher behind its controls, because FLTK delivers a drop only to the widget that accepted it while it was dragged over.
- [ ] Still driven by nothing: the native file choosers; menus and ComboBoxes opened with the mouse (they run a popup loop of their own); a real drag session out to another program (the test stops at the point where the system takes over).
- [ ] Two Image controls showing one file used to share (and rescale) one FLTK shared image. They now load their own copies, but that fix has not been seen on screen.
- [x] Closed forms are freed: their windows, the images in them, and their Canvases' handlers. What the controls held is copied into their properties first, so it stays readable, and showing the form again rebuilds it. The app keeps only forms that have a window, so a modal form made per click no longer accumulates.
- [x] Replacing an Image's file frees the previous image. Clearing it (`file = ""`) used to call `SetImage(nil)`, which go-fltk does not allow; it now shows a transparent pixel.
- [ ] Inputs have no text size or font of their own in go-fltk, so `font`/`fontSize` change only their captions (multi-line TextBoxes do get them).
- [x] Clipboard (`gui.clipboard`), drag and drop (`onDrop`, `onDrag` on every kind), Tree, Table, and Canvas with a drawing API and mouse events, plus `:redraw()`.
- [ ] go-fltk's own Tree cannot report the selected or clicked item, so Tree is built on the list widget: lines indented with ▸/▾ markers. It has no icons, multiple selection or editing.
- [ ] go-fltk cannot read the clipboard, so `gui.clipboard()` pastes into a hidden text editor and reads that. It is synchronous on macOS and Windows. On X11 it waits up to half a second for the text, and the X11 path is untested.
- [ ] Table cells are drawn as text: no editing, sorting by header click, or per-cell colours.

## Notes

- Initial implementation should stay narrow and reversible.
- If the GUI runtime is not available yet, the bootstrap path should fail explicitly rather than silently doing nothing.
- Validation completed for the first slice: `go test ./internal/gui ./cmd/tlua ./internal/interp`.
- Validation completed for the module/API slice: `go test ./internal/gui ./cmd/tlua ./internal/interp`.
- Validation completed after adding GUI examples: `go test ./internal/gui ./cmd/tlua ./internal/interp`.
- Validation completed after adding the backend seam: `go test ./internal/gui ./cmd/tlua ./internal/interp`.
- Validation completed after making the demos runnable: `go test ./internal/gui ./cmd/tlua ./internal/interp`.