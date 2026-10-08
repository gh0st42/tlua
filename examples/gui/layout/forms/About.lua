local gui = require "gui"

local frm = gui.load "About"

function frm.cmdOK:onClick()
  frm:close()
end

return frm
