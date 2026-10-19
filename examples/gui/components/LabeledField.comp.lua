-- A component: a label and a box that work together. A designer writes
-- these; they are plain data, and can be written by hand.
return {
  kind = "Component", name = "LabeledField", width = 240, height = 28,
  -- What a form sees of it. label and text are those of two parts.
  props = {
    label = { type = "string", default = "Name", to = "lbl.caption" },
    text = { type = "string", to = "txt.text" },
  },
  -- onChange is the box's; onSubmit is raised by the code, below.
  events = { onChange = "txt.onChange", "onSubmit" },
  { kind = "Label", name = "lbl", left = 0, top = 4, width = 80 },
  { kind = "TextBox", name = "txt", left = 84, top = 0, width = 156 },
}
