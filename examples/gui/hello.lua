#!/usr/bin/env tlua

bootgui()

local gui = require("gui")

local form = gui.Form {
  caption = "Hello GUI",
  width = 360,
  height = 180,
}

local label = form:Label {
  caption = "Hello from tlua",
  left = 16,
  top = 20,
  width = 220,
  height = 24,
}

local button = gui.Button {
  caption = "Close",
  left = 16,
  top = 64,
  width = 120,
  height = 28,
}

function button:onClick()
  form:close()
end

form:on("close", function()
  return true
end)

form:show()
