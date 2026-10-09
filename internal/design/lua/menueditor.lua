-- The Menu Editor, as VB6 had it: the form's menu as an indented list, an
-- item's caption, name and shortcut above it, and buttons to indent an item
-- under the one before (making that one a submenu), to move it, and to add
-- and delete items. A caption of "-" is a line between items.
--
-- A menu in a layout has no functions in it. Its items are told apart by
-- name in the Menu's onClick(name, caption, checked), in the form's code.

local gui = require "gui"
local model = require "design.model"

local M = {}

-- line is how an entry reads in the list.
local function line(e)
  local text = e.caption == "-" and "────────" or (e.caption ~= "" and e.caption or "(new item)")
  if e.shortcut and e.shortcut ~= "" then text = text .. "      " .. e.shortcut end
  return string.rep("····", e.level) .. text
end

-- edit shows the Menu Editor on a menu's items, and returns the new items,
-- or nil if it was cancelled. ed is filled in with the editor's parts, for
-- tests.
function M.edit(items, ed)
  ed = ed or {}
  local list = model.flattenMenu(items)
  -- An empty menu starts with an empty line to type into, as VB6's did.
  if #list == 0 then list[1] = { level = 0, caption = "" } end
  local cur = 1
  local result

  local dlg = gui.Form { caption = "Menu Editor", width = 430, height = 440 }
  ed.form = dlg

  dlg:Label { caption = "Caption", left = 10, top = 10, width = 76 }
  ed.caption = dlg:TextBox { left = 90, top = 10, width = 330 }
  dlg:Label { caption = "Name", left = 10, top = 42, width = 76 }
  ed.name = dlg:TextBox { left = 90, top = 42, width = 330, tooltip = "how the Menu's onClick tells this item apart" }
  dlg:Label { caption = "Shortcut", left = 10, top = 74, width = 76 }
  ed.shortcut = dlg:TextBox { left = 90, top = 74, width = 130, tooltip = "as Cmd+O, Ctrl+Shift+S or F5" }
  ed.toggle = dlg:CheckBox { caption = "Toggle", left = 236, top = 74, width = 80 }
  ed.checked = dlg:CheckBox { caption = "Checked", left = 320, top = 74, width = 100 }
  ed.enabled = dlg:CheckBox { caption = "Enabled", left = 90, top = 104, width = 100 }

  ed.list = dlg:ListBox { left = 10, top = 176, width = 410, height = 210 }

  local function current() return list[cur] end

  local function refresh()
    local lines = {}
    for i, e in ipairs(list) do lines[i] = line(e) end
    ed.list.items = lines
    ed.list.selected = cur
  end

  -- show puts the current entry in the fields; changing them changes it.
  local function show()
    local e = current()
    ed.caption.text = e and e.caption or ""
    ed.name.text = e and e.name or ""
    ed.shortcut.text = e and e.shortcut or ""
    ed.toggle.checked = e ~= nil and e.checked ~= nil
    ed.checked.checked = e ~= nil and e.checked == true
    ed.enabled.checked = not e or e.enabled ~= false
    for _, f in ipairs { ed.caption, ed.name, ed.shortcut, ed.toggle, ed.checked, ed.enabled } do
      f.enabled = e ~= nil
    end
    ed.checked.enabled = e ~= nil and e.checked ~= nil
  end

  local function edited(fn)
    return function(self)
      local e = current()
      if not e then return end
      fn(e, self)
      refresh()
    end
  end
  ed.caption.onChange = edited(function(e, box) e.caption = box.text end)
  ed.name.onChange = edited(function(e, box) e.name = box.text end)
  ed.shortcut.onChange = edited(function(e, box) e.shortcut = box.text end)
  ed.toggle.onChange = edited(function(e, box)
    e.checked = box.checked and (ed.checked.checked or false) or nil
    ed.checked.enabled = box.checked
  end)
  ed.checked.onChange = edited(function(e, box)
    if e.checked ~= nil then e.checked = box.checked end
  end)
  ed.enabled.onChange = edited(function(e, box) e.enabled = box.checked end)

  ed.list.onChange = function(self)
    cur = self.selected
    show()
  end

  local function button(caption, left, width, tip, fn)
    return dlg:Button {
      caption = caption, left = left, top = 138, width = width, tooltip = tip,
      onClick = function() fn(); refresh(); show(); ed.caption:focus() end,
    }
  end
  ed.outdent = button("<", 10, 36, "out a level", function()
    local e = current()
    if e and e.level > 0 then e.level = e.level - 1 end
  end)
  ed.indent = button(">", 50, 36, "into the submenu of the item above", function()
    local e, before = current(), list[cur - 1]
    if e and before and e.level <= before.level then e.level = e.level + 1 end
  end)
  ed.up = button("^", 90, 36, "up", function()
    if cur > 1 then list[cur], list[cur - 1] = list[cur - 1], list[cur]; cur = cur - 1 end
  end)
  ed.down = button("v", 130, 36, "down", function()
    if cur > 0 and cur < #list then list[cur], list[cur + 1] = list[cur + 1], list[cur]; cur = cur + 1 end
  end)
  ed.next = button("Next", 190, 72, "the next item, or a new one", function()
    if cur < #list then
      cur = cur + 1
    else
      local e = current()
      list[#list + 1] = { level = e and e.level or 0, caption = "" }
      cur = #list
    end
  end)
  ed.insert = button("Insert", 268, 72, "a new item before this one", function()
    local e = current()
    table.insert(list, math.max(cur, 1), { level = e and e.level or 0, caption = "" })
    cur = math.max(cur, 1)
  end)
  ed.delete = button("Delete", 346, 74, "this item", function()
    if cur > 0 then
      table.remove(list, cur)
      cur = math.min(cur, #list)
    end
  end)

  dlg:Button {
    caption = "Cancel", left = 230, top = 398, width = 90,
    onClick = function() dlg:close() end,
  }
  ed.ok = dlg:Button {
    caption = "OK", left = 330, top = 398, width = 90,
    onClick = function()
      local keep = {}
      for _, e in ipairs(list) do
        if e.caption ~= "" then keep[#keep + 1] = e end
      end
      result = model.unflattenMenu(keep)
      dlg:close()
    end,
  }

  refresh()
  show()
  if ed.onShow then gui.after(0.1, function() ed.onShow(ed) end) end
  dlg:showModal()
  return result
end

return M
