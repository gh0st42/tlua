return { kind = "Component", name = "LabeledField", width = 240, height = 28,
  events = { onChange = "txt.onChange" },
  props = {
    label = { default = "Name", to = "lbl.caption", type = "string" },
    text = { to = "txt.text", type = "string" },
  },
  { kind = "Label", name = "lbl", left = 0, top = 4, width = 80 },
  { kind = "TextBox", name = "txt", grow = true, left = 84, top = 0, width = 156 },
}
