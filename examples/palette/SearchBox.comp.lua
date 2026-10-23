return { kind = "Component", name = "SearchBox", width = 260, height = 28,
  events = { "onSearch" },
  props = { text = { to = "txt.text", type = "string" } },
  { kind = "TextBox", name = "txt", grow = true, left = 0, top = 0, width = 190 },
  { kind = "Button", name = "go", caption = "Search", left = 196, top = 0, width = 64 },
}
