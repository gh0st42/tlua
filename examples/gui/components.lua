#!/usr/bin/env tlua

-- Components: gui.component makes a control out of a layout, here
-- components/LabeledField.comp.lua. Two of them on a form do not clash,
-- though both have parts called lbl and txt: a component's parts are its
-- own, and the form sees only what the component shows (label, text,
-- onChange, onSubmit and the method clear).

local gui = bootgui()

gui.component {
  layout = "components/LabeledField",
  code = function(self, parts)
    -- The code behind it: the parts by name, and the component itself.
    function self:clear() parts.txt.text = "" end
    function parts.txt:onKey(key)
      if key == "Enter" then self.parent:fire("onSubmit", self.text) end
    end
  end,
}

local frm = gui.Form { caption = "Components", width = 320, height = 150 }
local first = frm:LabeledField { name = "first", label = "First name", left = 10, top = 10 }
local last = frm:LabeledField { name = "last", label = "Last name", left = 10, top = 46 }
local says = frm:Label { left = 10, top = 112, width = 300, height = 24 }

function first:onChange() says.caption = ("First: %q, last: %q"):format(first.text, last.text) end
function last:onChange() first:onChange() end
function last:onSubmit(text) says.caption = "Submitted " .. text; self:clear() end

frm:show()
