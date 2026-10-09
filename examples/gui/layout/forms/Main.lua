-- The code behind forms/Main.form.lua: its controls are fields of the form,
-- by the names the layout gives them.

local gui = require "gui"
local about = require "forms.About"

local frm = gui.load "Main"

function frm.txtName:onChange()
  frm.cmdGreet.enabled = self.text ~= ""
end

function frm.cmdGreet:onClick()
  frm.lblGreeting.caption = "Hello, " .. frm.txtName.text .. "!"
end

function frm.cmdAbout:onClick()
  about:showModal()
end

return frm
