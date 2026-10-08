-- A layout, written by gui.save: gui.load builds it.
return { kind = "Form", name = "Main", caption = "Layouts", width = 360, height = 190,
  { kind = "Label", name = "lblName", caption = "Name", left = 16, top = 16, width = 60 },
  { kind = "TextBox", name = "txtName", left = 80, top = 16, width = 264 },
  { kind = "Label", name = "lblGreeting", caption = "Type a name, then Greet.", left = 16, top = 60, width = 328, height = 48, align = "center", color = "#fff4dc" },
  { kind = "Button", name = "cmdAbout", caption = "About...", left = 16, top = 140, width = 100 },
  { kind = "Button", name = "cmdGreet", caption = "Greet", left = 244, top = 140, width = 100, default = true, enabled = false },
}
