# Components for the designer's palette

Three components made the way `tlua design` makes them: a layout
(`Name.comp.lua`) and the code behind it (`Name.lua`).

- `LabeledField` - a label and a box: `label`, `text`, `onChange`, `clear()`.
- `SearchBox` - a box and a button: `text`, `onSearch(self, text)`.
- `RgbPicker` - three sliders and a swatch: `rgb`, `set("#rrggbb")`,
  `onChange(self, rgb)`.

Point `TLUA_PALETTE` at this folder and the designer offers them under the
toolbox, one click from a project:

    TLUA_PALETTE=examples/palette tlua design myapp

(`:` separates several folders, `;` on Windows.) A click copies the files
into the project's `components/`; they are the project's own from then on.
