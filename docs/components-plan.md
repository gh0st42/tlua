# Designing components: plan

Status: phase 1 is built (2026-10-19): `gui.component`, private part
names, forwarded props and events, in docs/gui.md ("Components") and
`examples/gui/components.lua`. Phase 2 is built: Project > Add Component, a component opens
like a form (`d:openComponent`), saves its files, and is in the toolbox. Phase 3 is built: Tools > Component Interface. Phase 4 is built: saving defines it again, and props renamed or removed are put right in the layouts that use it. Phase 5 is built (palettes, but as dialogs, not a toolbox drawer). Phase 6 is not.

## What it is

Today the designer makes forms. A reusable control has to be written by
hand: `Project > New Control` gives a `controls/Name.lua` with a
`gui.define`, and from there it is code, drawn with `g:` calls or built
with `parent:Frame{...}` by hand. A group of controls that work together
(a labelled field, a colour picker, a search box with its button, an
address block) is the common case, and it is exactly the case the
designer is best at.

The aim: **open a component in the designer like a form, draw it, give it
properties and events, and then find it in the toolbox of every form of
the project**, placed, moved and configured like a built-in control. It
may also be shared between projects, from a user palette.

What it produces is ordinary tlua, as forms are: files that run without
the designer. A component is still a `gui.define`, so everything in
docs/gui.md ("Controls of your own") holds and nothing at runtime changes
for programs that do not use the designer.

## What is there to build on

- `gui.define` with `props`, `events`, `build(parent, opts)`, `fire` and
  methods; `gui.kinds()` reports defined controls with `defined = true`.
- `gui.load(layout, parent)` builds a layout into any parent, and finds
  unknown kinds as `controls.<Kind>`.
- The designer already loads `controls/*.lua`, puts them at the end of
  the toolbox (marked U), builds them for real on the form and shows their
  `props` in the grid.
- A form is two files: a layout the designer owns and code the user owns,
  with `.d.lua` declarations for the language server.

So a component is mostly: a Form-like design surface whose root is a Frame,
saved in the same two-file shape, plus a generated `gui.define` that ties
it to the toolbox.

## Decisions

Each has a recommendation; the alternatives are noted where they are real.

### 1. A component is a layout with a Frame at the root

Same file shape as a form, in its own folder:

```
myapp/
  components/Rating.comp.lua   layout: written by the designer
  components/Rating.lua        code: yours (handlers, methods, state)
  components/Rating.d.lua      declarations: written by the designer
  controls/Rating.lua          the gui.define: written by the designer
```

The layout is a Lua table whose root is `kind = "Component"` (a Frame on
the surface with a designable size and no caption bar), with the parts as
its children, and, beside them, the declaration of its interface:

```lua
return {
  kind = "Component", name = "LabeledField", width = 240, height = 28,
  props  = { label = { type = "string", default = "Name", to = "lbl.caption" },
             text  = { type = "string", to = "txt.text" } },
  events = { onChange = "txt.onChange" },          -- forwarded from a part
  { kind = "Label",   name = "lbl", left = 0, top = 4, width = 80 },
  { kind = "TextBox", name = "txt", left = 84, top = 0, width = 156 },
}
```

Recommended. The alternative, a component as a form flagged `component =
true` in the existing `forms/` folder, saves a folder but mixes things the
toolbox must list and the forms list must not.

### 2. The interface is declared, not guessed

A component shows the outside world its **props**, **events** and
**methods**, not its parts. The designer offers them in a "Component"
dialog (and as rows in the properties grid when the root is selected):
- **Prop mapping (`to`)**: a prop forwards to a part's property
  (`label` → `lbl.caption`), the common case, needing no code.
  A prop without `to` is plain state, read in the code file.
- **Event forwarding**: an event can be wired to a part's event
  (`onChange` ← `txt.onChange`); others are raised by the user's code with
  `self:fire`.
- **Parts are private** by default: `field.txt` is not reachable from a
  form. A part can be marked `public`, which makes it a field of the
  component (`form.field.txt.text`), for the cases where wiring every prop
  would be silly. Recommended default: private; it is what keeps a
  component replaceable.

### 3. The `gui.define` is generated, and the layout is the source

The designer writes `controls/Rating.lua`:

```lua
-- Written by tlua design from components/Rating.comp.lua. Do not edit.
local gui = require "gui"
return require("components.Rating")   -- the user's code file, below
```

and `gui.component` does the work at runtime, in Go/Lua inside the `gui`
module, about forty lines:

```lua
gui.component {
  name = "LabeledField", layout = "components/LabeledField.comp.lua",
  code = function(self) ... end,   -- optional: the user's code file
}
```

It is `gui.define` with a ready-made `build`: `gui.load` the layout into a
Frame, apply the declared prop mappings and event forwards, set `props`
and `events` from the layout's declaration, and call the code file's
function once with the component. Everything else (placement, `fire`,
`gui.kinds()`, the toolbox) is the existing machinery.

Why a function in `gui` and not generated code: the layout stays the one
source of truth, so editing a component in the designer changes every
form that uses it, without regenerating anything in them.

### 4. Editing the component updates the forms that use it

Forms hold `{ kind = "LabeledField", ... }` and nothing of its parts. When a
component is saved, forms open in the designer rebuild their instances (the
designer already rebuilds a control when a prop changes). A rename of a
prop or event in the component is the one change that breaks users; the
designer lists the forms that use the old name, and offers to rewrite their
layouts, as it already offers to rename a control in the code. Recommended;
without it the break is silent.

### 5. Palettes

Project-local first; sharing second. There are two kinds of place a
component lives, and nothing moves between them unless you say so:
- **The project's own** components are in its `components/` and
  `controls/`. They are in that project's toolbox and nowhere else; making
  or editing one never touches a palette.
- **Palettes** are folders of components. `TLUA_PALETTE` lists them, as
  `PATH` does (separated by `:`, or `;` on Windows), and defaults to
  `~/.tlua/palette`. The toolbox has a drawer for each, named by its folder,
  so a team can have a shared checkout beside a personal one.

What moves things:
- **Add to project** copies a palette component's files into `components/`
  and `controls/`. Copy, not reference: a project stays self-contained and
  packs with Make EXE with nothing outside it. The cost is that a copy
  drifts; the answer is the same as for `tlua lib export`: add it again to
  update. The palette entry records the version the copy came from, so the
  designer can say a copy is behind.
- **Save to palette** copies a project's component into a palette, and asks
  which when there is more than one. A palette is never written to
  otherwise.

A project keeps no palette folder of its own: its `components/` already is
that, and a second folder would blur "in use" and "available". A package form
(a `.ztl` of components to hand around) is for later; the bundle machinery is
there when it is wanted.

### 6. Nesting and the drawn kind

- **A component can hold another** (its layout names a component's kind);
  the loader already resolves `controls.<Kind>` recursively. Cycles are an
  error that names the chain.
- **Drawn controls stay code.** A Canvas with `onDraw` is not designed with
  boxes; `Project > New Control` keeps working for them. The designer only
  adds the *composite* shape. Recommended; a drawing designer is a project
  of its own.

### 7. Opening one in the designer

A component is opened like a form: the project tree lists "Components"
beside "Forms", the surface shows a framed area of its size (resizable by
its handles, which sets the default `width` and `height`), and the same
tools apply: toolbox, grid, alignment, the code window with stubs. The
toolbox on a component offers everything but Form and Menu (a Menu belongs
to a window), and the component's own kind is left out so it cannot hold
itself.

How instances look on a form: **built for real**, as defined controls are
today, so the form shows what will run. Selecting an instance selects
the whole component; parts cannot be moved on a form (that is the point).
"Edit component" in the context menu jumps to its design.

## Phases

1. **`gui.component` and the layout's `props`/`events`.** Done. The runtime
   (`internal/gui/lua/component.lua`, `component.go`), with tests, docs/gui.md
   and `library/gui.lua`; hand-written component files work with `gui.load`
   and `gui.kinds` before the designer knows them. Parts' names are kept per
   component (a name scope), props forward through a link on the object.
2. **Components in the designer.** Done. `Component` kind on the surface, the
   Components list in the project tree, New Component, saving the three
   files, the toolbox entry, building on forms. docs/design.md.
3. **The interface dialog.** Done (`interface.lua`). Props with `to`, event forwarding, public
   parts, methods; the grid shows the declared props on instances.
4. **Keeping forms in step.** Done. One document is open at a time, so
   what has to follow a change is the definition (`gui.component` replaces a
   component, and the designer defines it again on save) and the layouts on
   disk: props renamed or removed are rewritten in the forms and components
   that use them. Events renamed or removed are only reported, with the code
   that mentions them. The `.d.lua` declarations let the code window complete
   `form.field.label` (done with phase 2).
5. **Palettes.** Done: `TLUA_PALETTE`; a heading per folder in the toolbox
   with the components not in the project yet, which a click copies in;
   Save to palette and Add from palette in the Project menu; and a record of
   where a copy came from. `examples/palette` has three components.
6. **Finishing.** Done: components named in errors; no component offered
   inside one that holds it; a picture of its parts for each in the toolbox;
   events renamed followed in the handlers of the forms; examples; parts
   follow when an instance is resized (checked, nothing needed). Not done,
   and not planned: a bundle format for components (a palette is a folder, and
   a folder is easy to zip or keep in git), and an icon the author draws.

## Questions, answered

- **Part access.** `public` per part was enough; a whole sub-tree is not
  offered. A component that wants to show more makes it a prop or a method.
- **Layout inside a component.** Parts follow when an instance is resized
  (`grow` and anchors work as on a form), also in a component inside a
  component; `TestComponentResizesItsParts` checks it.
- **Where the code runs.** The function gets the component and its parts by
  name, as a second argument, so the parts stay private to the code.
- **Form-like events of a component** are the ones every control has now;
  nothing of its own was needed.
- **Naming.** "Component" in the designer and `gui.component` at runtime; in
  the toolbox it is one of the project's controls.
