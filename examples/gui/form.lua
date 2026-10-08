#!/usr/bin/env tlua

bootgui()

local gui = require("gui")

local form = gui.Form {
  caption = "Form demo",
  width = 520,
  height = 260,
}

local editor = form:TextBox {
  caption = "Type here",
  left = 16,
  top = 16,
  width = 320,
  height = 120,
  multiLine = true,
  grow = true,
}

local status = form:Label {
  caption = "Ready",
  left = 16,
  top = 150,
  width = 360,
  height = 24,
}

function editor:onChange()
  status.caption = string.format("%d characters", #(self.text or ""))
end

function form:onUnload()
  return true
end

form:show()
