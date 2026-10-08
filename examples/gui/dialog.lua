#!/usr/bin/env tlua

bootgui()

local gui = require("gui")

local answer = gui.inputbox("What is your name?", "GUI example", "Ada")
if not answer then
  gui.msgbox("No answer given", "ok", "GUI example")
  return
end

local form = gui.Form {
  caption = "Dialog result",
  width = 360,
  height = 160,
}

local label = form:Label {
  caption = "Hello, " .. answer,
  left = 16,
  top = 24,
  width = 280,
  height = 24,
}

local close = gui.Button {
  caption = "Close",
  left = 16,
  top = 64,
  width = 100,
  height = 28,
}

function close:onClick()
  form:close()
end

form:show()
