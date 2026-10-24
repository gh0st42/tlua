-- The New Project dialog: what kind of project, and which folder. It
-- returns { dir =, type = }, or nil when cancelled. ed is filled in with the
-- dialog's parts, for tests.

local gui = require "gui"
local project = require "design.project"

local M = {}

-- ask shows the dialog. dir, when given, is the folder to start with; the
-- user can still change it. type is the one chosen to begin with.
function M.ask(dir, kind, ed)
  ed = ed or {}
  kind = kind or "gui"
  local result

  local f = gui.Form { caption = "New Project", width = 520, height = 330 }
  ed.form = f
  f:Label { caption = "What kind of project?", left = 16, top = 12, width = 488, height = 22 }

  ed.radios = {}
  local about = f:Label { left = 40, top = 134, width = 464, height = 40, textColor = "#404040", fontSize = 12 }
  for i, t in ipairs(project.types) do
    local r = f:RadioButton {
      caption = t.title, left = 24, top = 38 + (i - 1) * 30, width = 300, height = 26,
      checked = t.id == kind,
      onChange = function(self)
        if self.checked then
          kind = t.id
          about.caption = t.about
        end
      end,
    }
    ed.radios[t.id] = r
    if t.id == kind then about.caption = t.about end
  end

  f:Label { caption = "In the folder:", left = 16, top = 186, width = 488, height = 22 }
  ed.folder = f:TextBox { left = 16, top = 210, width = 380, height = 28, text = dir or "" }
  ed.browse = f:Button {
    caption = "Browse...", left = 404, top = 210, width = 100, height = 28,
    onClick = function()
      local chosen = gui.choosedir { title = "A folder for the new project" }
      if chosen then ed.folder.text = chosen end
    end,
  }
  f:Label {
    caption = "A folder that has files in it keeps them; a main.lua there is used as it is.",
    left = 16, top = 244, width = 488, height = 20, fontSize = 12, textColor = "#404040",
  }

  f:Button { caption = "Cancel", left = 306, top = 284, width = 96, height = 30, onClick = function() f:close() end }
  ed.ok = f:Button {
    caption = "Create", left = 408, top = 284, width = 96, height = 30, default = true,
    onClick = function()
      local chosen = ed.folder.text:gsub("^%s+", ""):gsub("%s+$", "")
      if chosen == "" then
        ed.problem = "Name a folder."
        gui.msgbox(ed.problem)
        return
      end
      result = { dir = chosen, type = kind }
      f:close()
    end,
  }
  function f:onKey(key)
    if key == "Escape" then self:close() return true end
  end

  if ed.onShow then gui.after(0.1, function() ed.onShow(ed) end) end
  f:showModal()
  return result
end

return M
