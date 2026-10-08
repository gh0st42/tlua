#!/usr/bin/env tlua

-- A program whose forms are laid out in files, the way a designer writes
-- them: forms/Main.form.lua is the layout, forms/Main.lua the code that
-- wires it up. Each layout can be changed without touching the code.

local gui = bootgui()
local main = require "forms.Main"
main:show()
