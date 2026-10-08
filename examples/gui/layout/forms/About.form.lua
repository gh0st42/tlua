-- A layout, written by gui.save: gui.load builds it.
return { kind = "Form", name = "About", caption = "About", width = 280, height = 130,
  { kind = "Label", name = "lblText", caption = "Two forms, laid out in files\nand wired up in code.", left = 16, top = 16, width = 248, height = 48 },
  { kind = "Button", name = "cmdOK", caption = "OK", left = 164, top = 84, width = 100, default = true },
}
