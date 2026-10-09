-- The properties pane: a list of the form and its controls at the top, as
-- VB6's object box, and below it one row for each property of the one
-- selected, with an editor that suits its type. A change is made as it is
-- typed; a value the control will not take is shown in red, with why in
-- the status line.

local gui = require "gui"
local model = require "design.model"

local M = {}
M.__index = M

local ROW = 26
local LABEL = 108

function M.new(parent, d, opts)
  local p = setmetatable({ d = d, rows = {}, editors = {}, width = opts.width }, M)
  p.picker = parent:ComboBox {
    left = opts.left, top = opts.top, width = opts.width,
    onChange = function(self) p:picked(self.selected) end,
  }
  p.scroll = parent:Scroll {
    left = opts.left, top = opts.top + 32, width = opts.width, height = opts.height - 32,
    grow = true,
  }
  return p
end

-- list fills the object box from the layout: the form, then its controls.
function M:list(doc)
  self.doc = doc
  self.nodes = { doc }
  local items = { (doc.name or "?") .. "  (Form)" }
  local depth = { [doc] = 0 }
  model.walk(doc, function(node, parent)
    depth[node] = depth[parent] + 1
    if node.name then
      self.nodes[#self.nodes + 1] = node
      items[#items + 1] = string.rep("   ", depth[node] - 1) .. node.name .. "  (" .. node.kind .. ")"
    end
  end)
  self.picker.items = items
end

function M:picked(i)
  local node = self.nodes and self.nodes[i]
  if node then self.d:selectNode(node) end
end

-- convert turns what was typed into a value of a property's type, or says
-- why it cannot. Nothing typed is no value: the default.
local function convert(info, text)
  if text == "" then return nil end
  local t = info.type
  if t == "integer" or t == "number" then
    local n = tonumber(text)
    if not n then return nil, "a number" end
    if t == "integer" and n ~= math.floor(n) then return nil, "a whole number" end
    return n
  end
  return text
end

local function summary(v)
  if type(v) ~= "table" or next(v) == nil then return "(none)" end
  return ("(%d)"):format(#v)
end

-- show fills the rows for a node: the form's layout, or a control's.
function M:show(node)
  for _, obj in ipairs(self.rows) do obj:remove() end
  self.rows, self.editors, self.node = {}, {}, node
  for i, n in ipairs(self.nodes or {}) do
    if n == node then self.picker.selected = i end
  end

  local kinds = model.kinds()
  local width = self.width - LABEL - 24
  for i, prop in ipairs(model.propNames(node.kind)) do
    local info = kinds[node.kind].props[prop]
    local y = (i - 1) * ROW + 2
    local label = self.scroll:Label {
      caption = prop, left = 4, top = y, width = LABEL - 8, height = ROW - 4,
      tooltip = info.type .. (info.fixed and ", rebuilds the control" or ""),
    }
    self.rows[#self.rows + 1] = label
    local editor = self:editor(node, prop, info, LABEL, y, width)
    self.rows[#self.rows + 1] = editor
    self.editors[prop] = { obj = editor, info = info }
  end
end

function M:editor(node, prop, info, x, y, w)
  local v = model.value(node, prop)
  local s = self.scroll
  local apply = function(obj, value)
    local ok, err = self.d:setProp(node, prop, value)
    obj.textColor = ok and "black" or "#c00000"
    self.d:status(ok and "" or (prop .. ": " .. tostring(err)))
    return ok
  end

  if info.type == "boolean" then
    return s:CheckBox {
      left = x, top = y, width = w, height = ROW - 4, checked = v == true,
      onChange = function(obj) apply(obj, obj.checked) end,
    }
  end

  if info.type == "choice" then
    local items, selected = {}, 0
    if info.default == nil then items[1] = "" end
    for _, c in ipairs(info.choices) do
      items[#items + 1] = c
      if c == v then selected = #items end
    end
    if selected == 0 and v == nil then selected = 1 end
    return s:ComboBox {
      left = x, top = y, width = w, height = ROW - 4, items = items, selected = selected,
      onChange = function(obj) apply(obj, obj.text ~= "" and obj.text or nil) end,
    }
  end

  if info.type == "list" or info.type == "rows" or info.type == "tree" or info.type == "menu" then
    local button
    button = s:Button {
      left = x, top = y, width = w, height = ROW - 4, caption = summary(v),
      onClick = function()
        local value = self:editList(prop, info, model.value(node, prop))
        if value ~= nil and apply(button, value) then button.caption = summary(value) end
      end,
    }
    return button
  end

  local box = s:TextBox {
    left = x, top = y, width = w, height = ROW - 4,
    text = v ~= nil and tostring(v) or "",
  }
  if info.type == "color" and v then box.color = v end
  box.onChange = function(obj)
    local value, why = convert(info, obj.text)
    if why then
      obj.textColor = "#c00000"
      self.d:status(prop .. " must be " .. why)
      return
    end
    if apply(obj, value) and info.type == "color" then
      obj.color = value or "#ffffff"
    end
  end
  return box
end

-- editList edits a list in a form of its own: one item a line for a list,
-- a Lua table for rows, trees and menus. It returns the new value, or nil
-- if nothing was changed.
function M:editList(prop, info, value)
  local plain = info.type == "list"
  local numbers = prop == "columnWidths"
  local dlg = gui.Form { caption = "Edit " .. prop, width = 420, height = 320 }
  dlg:Label {
    caption = plain and "One item a line:" or "As a Lua table:",
    left = 12, top = 8, width = 396,
  }
  local box = dlg:TextBox {
    multiLine = true, left = 12, top = 36, width = 396, height = 230,
    text = plain and model.listToText(value) or model.literal(value or {}),
  }
  local result
  dlg:Button {
    caption = "Cancel", left = 220, top = 280, width = 90,
    onClick = function() dlg:close() end,
  }
  dlg:Button {
    caption = "OK", left = 318, top = 280, width = 90,
    onClick = function()
      if plain then
        result = model.textToList(box.text, numbers)
      else
        local v, err = model.parse(box.text)
        if type(v) ~= "table" then
          gui.msgbox("That is not a Lua table: " .. tostring(err or v), "ok", "Edit " .. prop)
          return
        end
        result = v
      end
      dlg:close()
    end,
  }
  dlg:showModal()
  return result
end

-- refresh shows a node's values again after the surface changed them,
-- leaving the rows where they are.
function M:refresh(node)
  if node ~= self.node then return end
  for prop, ed in pairs(self.editors) do
    local v = model.value(node, prop)
    local t = ed.info.type
    if t == "integer" or t == "number" or t == "string" or t == "name" then
      local text = v ~= nil and tostring(v) or ""
      if ed.obj.text ~= text then ed.obj.text = text end
    end
  end
end

return M
