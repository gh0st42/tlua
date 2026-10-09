-- The properties pane: a list of the form and its controls at the top, as
-- VB6's object box, and below it a grid of the selected one's properties,
-- as VB6's, Delphi's and Lazarus's have it: names on the left, values on
-- the right, edited in place. A value is kept on Enter, on leaving the cell
-- or on Up and Down, which go on to the next property; Escape puts it back.
-- True or false and the choices of a choice are picked from a list, or
-- taken in turn with a double click; a list is edited in a dialog of its
-- own, from the cell's "...". A value the control will not take stays in
-- the cell, in red, with why in the status line.

local gui = require "gui"
local model = require "design.model"

local M = {}
M.__index = M

local LABEL = 108

function M.new(parent, d, opts)
  local p = setmetatable({ d = d, names = {}, rowOf = {}, width = opts.width }, M)
  p.picker = parent:ComboBox {
    left = opts.left, top = opts.top, width = opts.width,
    onChange = function(self) p:picked(self.selected) end,
  }
  p.grid = parent:Table {
    left = opts.left, top = opts.top + 32, width = opts.width, height = opts.height - 32,
    grow = true, columns = { "Property", "Value" },
    columnWidths = { LABEL, opts.width - LABEL - 18 },
    editable = { 2 },
    onStartEdit = function(_, row) return p:startEdit(row) end,
    onEdit = function(_, row, _, text) return p:edited(row, text) end,
    onEditButton = function(_, row) p:button(row) end,
    onChange = function(self) p:describe(self.selected) end,
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
  if t == "boolean" then
    if text == "true" then return true end
    if text == "false" then return false end
    return nil, "true or false"
  end
  return text
end

local lists = { list = true, rows = true, tree = true, menu = true }

-- display is how a value reads in the grid.
local function display(info, v)
  if lists[info.type] then
    if type(v) ~= "table" or next(v) == nil then return "(none)" end
    return ("(%d)"):format(#v)
  end
  if v == nil then return "" end
  return tostring(v)
end

function M:info(prop)
  return model.kinds()[self.node.kind].props[prop]
end

-- show fills the grid for a node: the form's layout, or a control's. An
-- edit under way is kept first, for the node it was begun on.
function M:show(node)
  if self.grid:editing() then self.grid:edit() end
  self.node = node
  for i, n in ipairs(self.nodes or {}) do
    if n == node then self.picker.selected = i end
  end
  self.names, self.rowOf = model.propNames(node.kind), {}
  local rows = {}
  for i, prop in ipairs(self.names) do
    self.rowOf[prop] = i
    rows[i] = { prop, display(self:info(prop), model.value(node, prop)) }
  end
  self.grid.rows = rows
end

-- describe says what the selected property holds.
function M:describe(row)
  local prop = self.names[row]
  if not prop then return end
  local info = self:info(prop)
  self.d:status(prop .. ": " .. info.type .. (info.fixed and ", rebuilds the control" or ""))
end

-- startEdit is how a property's value is edited: from a list, from a
-- dialog, or typed.
function M:startEdit(row)
  local info = self:info(self.names[row])
  if info.type == "boolean" then return { choices = { "true", "false" } } end
  if info.type == "choice" then return { choices = info.choices } end
  if lists[info.type] then return { button = true, readOnly = true } end
  if info.type == "color" or info.type == "file" then return { button = true } end
end

-- edited is a value typed or picked: the control takes it, or it is
-- refused and said why.
function M:edited(row, text)
  local node, prop = self.node, self.names[row]
  local info = self:info(prop)
  local value, why = convert(info, text)
  if why then
    self.d:status(prop .. " must be " .. why)
    return false
  end
  local ok, err = self.d:setProp(node, prop, value)
  if not ok then
    self.d:status(prop .. ": " .. tostring(err))
    return false
  end
  self.d:status("")
  return display(info, model.value(node, prop))
end

-- button is a cell's "...": a colour from the chooser, a file from the
-- disk, a list edited in a dialog, or the form's menu in the Menu Editor.
function M:button(row)
  local node, prop = self.node, self.names[row]
  if node.kind == "Menu" and prop == "items" then
    self.d:editMenu()
    return
  end
  local info = self:info(prop)
  local value
  if info.type == "menu" then
    -- A control's context menu, in the Menu Editor too; empty is none.
    value = self:editMenuItems(model.value(node, prop) or {})
    if value and #value == 0 then
      local ok, err = self.d:setProp(node, prop, nil)
      self.d:status(ok and "" or (prop .. ": " .. tostring(err)))
      self:refresh(node)
      return
    end
  elseif info.type == "color" then
    value = self.d:chooseColor(prop, model.value(node, prop) or "#ffffff")
  elseif info.type == "file" then
    value = self:chooseFile(prop, model.value(node, prop))
  else
    value = self:editList(prop, info, model.value(node, prop))
  end
  if value ~= nil then
    local ok, err = self.d:setProp(node, prop, value)
    self.d:status(ok and "" or (prop .. ": " .. tostring(err)))
  end
  self:refresh(node)
end

-- editMenuItems edits a menu's items in the Menu Editor: the new items, or
-- nil when it was cancelled.
function M:editMenuItems(items)
  return require("design.menueditor").edit(items)
end

-- chooseFile picks an image for a property. One inside the project is
-- named relative to forms/, as gui.load finds it; one outside is copied into
-- the project's images/ folder first, if wanted, so that it goes with the
-- program when it is packed.
function M:chooseFile(prop, current)
  local project = require "design.project"
  local dir = self.d.dir
  local file = self.d:chooseFile(prop, project.imageDir(dir, current))
  if not file then return nil end
  local path = project.imagePath(dir, file)
  if path then return path end
  local name = file:match("[^/\\]+$")
  if self.d:ask(name .. " is outside the project. Copy it into the project's images folder, "
      .. "so that it goes with the program?", "yesno", prop) ~= "yes" then
    return file
  end
  local copy, err = project.copyIn(dir, file)
  if not copy then
    self.d:ask("Could not copy " .. name .. ": " .. tostring(err), "ok", prop)
    return nil
  end
  return copy
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
-- leaving the grid where it is.
function M:refresh(node)
  if node ~= self.node then return end
  local rows, changed = self.grid.rows, false
  for prop, i in pairs(self.rowOf) do
    local text = display(self:info(prop), model.value(node, prop))
    if rows[i][2] ~= text then
      rows[i][2] = text
      changed = true
    end
  end
  if changed then self.grid.rows = rows end
end

-- row is a property's row in the grid, and text what its value reads as
-- there; for tests, mostly.
function M:row(prop)
  return self.rowOf[prop]
end

function M:text(prop)
  local row = self.rowOf[prop]
  return row and self.grid.rows[row][2]
end

return M
