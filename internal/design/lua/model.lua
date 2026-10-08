-- The form being designed, as the layout table gui.save writes and gui.load
-- reads: its properties in the hash part, its controls in the array part.
-- Nothing here touches the screen, so all of it can be tested without one.

local gui = require "gui"

local M = {}

-- The controls the toolbox offers, in its order.
M.tools = {
  "Label", "TextBox", "Button", "CheckBox", "RadioButton", "ComboBox",
  "ListBox", "Frame", "Panel", "Image", "Canvas", "Slider", "Spinner",
  "ProgressBar", "Tree", "Table", "Tabs",
}

-- Kinds a new control of which starts with its name as its caption, the way
-- VB6's Command1 said "Command1".
local captioned = {
  Label = true, Button = true, CheckBox = true, RadioButton = true, Frame = true,
}

local kinds -- gui.kinds(), read once: it does not change while designing

function M.kinds()
  kinds = kinds or gui.kinds()
  return kinds
end

-- newForm is the layout of an empty form.
function M.newForm(name)
  return { kind = "Form", name = name, caption = name, width = 480, height = 320 }
end

-- names lists every name in a layout, the form's own included.
function M.names(doc, out)
  out = out or {}
  if doc.name then out[doc.name] = true end
  for _, child in ipairs(doc) do M.names(child, out) end
  return out
end

-- uniqueName is the next free name for a kind: Button1, Button2, ...
function M.uniqueName(doc, kind)
  local taken = M.names(doc)
  local n = 1
  while taken[kind .. n] do n = n + 1 end
  return kind .. n
end

-- newControl is the layout of a control placed at x, y; a size of nothing
-- means the kind's own.
function M.newControl(doc, kind, x, y, w, h)
  local info = M.kinds()[kind]
  local node = {
    kind = kind,
    name = M.uniqueName(doc, kind),
    left = x, top = y,
    width = (w and w > 0) and w or info.width,
    height = (h and h > 0) and h or info.height,
  }
  if captioned[kind] then node.caption = node.name end
  if kind == "Tabs" then
    -- Tabs start with two pages, named like everything else.
    local taken = M.names(doc)
    taken[node.name] = true
    local function fresh()
      local n = 1
      while taken["Page" .. n] do n = n + 1 end
      taken["Page" .. n] = true
      return "Page" .. n
    end
    node[1] = { kind = "Page", name = fresh(), caption = "Page 1" }
    node[2] = { kind = "Page", name = fresh(), caption = "Page 2" }
  end
  return node
end

function M.indexOf(doc, node)
  for i, child in ipairs(doc) do
    if child == node then return i end
  end
end

function M.remove(doc, node)
  local i = M.indexOf(doc, node)
  if i then table.remove(doc, i) end
  return i
end

-- toFront and toBack move a control in the list, which is also the order
-- they are drawn in, last on top.
function M.toFront(doc, node)
  if M.remove(doc, node) then doc[#doc + 1] = node end
end

function M.toBack(doc, node)
  if M.remove(doc, node) then table.insert(doc, 1, node) end
end

-- leading are the properties the grid shows first, in this order; the rest
-- follow alphabetically.
local leading = { "name", "caption", "text", "left", "top", "width", "height" }

-- propNames lists a kind's properties in the order the grid shows them.
function M.propNames(kind)
  local props = M.kinds()[kind].props
  local rank = {}
  for i, p in ipairs(leading) do rank[p] = i end
  local names = {}
  for p in pairs(props) do names[#names + 1] = p end
  table.sort(names, function(a, b)
    local ra, rb = rank[a] or #leading + 1, rank[b] or #leading + 1
    if ra ~= rb then return ra < rb end
    return a < b
  end)
  return names
end

-- value is what a property of a node is: what the layout says, or else the
-- kind's default.
function M.value(node, prop)
  local v = node[prop]
  if v ~= nil then return v end
  local p = M.kinds()[node.kind].props[prop]
  return p and p.default
end

-- set changes a property in the layout. A value that is the default is left
-- out of it, so that saved layouts say only what differs.
function M.set(node, prop, value)
  local p = M.kinds()[node.kind].props[prop]
  local default = p and p.default
  if value == default or (type(value) == "table" and next(value) == nil) then
    node[prop] = nil
  else
    node[prop] = value
  end
end

-- The two ways of writing a list in a text box: one item a line, or (for
-- rows, trees and menus) as a Lua table.
function M.listToText(list)
  local lines = {}
  for i, v in ipairs(list or {}) do lines[i] = tostring(v) end
  return table.concat(lines, "\n")
end

function M.textToList(text, numbers)
  local list = {}
  for line in (text .. "\n"):gmatch("(.-)\r?\n") do
    if line ~= "" then
      list[#list + 1] = numbers and (tonumber(line) or line) or line
    end
  end
  return list
end

-- literal writes a value as Lua source; parse reads one back, with nothing
-- in scope, or returns nil and why.
function M.literal(v, indent)
  indent = indent or ""
  local t = type(v)
  if t == "string" then return string.format("%q", v) end
  if t ~= "table" then return tostring(v) end
  local parts = {}
  for _, x in ipairs(v) do parts[#parts + 1] = M.literal(x, indent .. "  ") end
  local keys = {}
  for k in pairs(v) do
    if type(k) == "string" then keys[#keys + 1] = k end
  end
  table.sort(keys)
  for _, k in ipairs(keys) do parts[#parts + 1] = k .. " = " .. M.literal(v[k], indent .. "  ") end
  if #parts == 0 then return "{}" end
  return "{ " .. table.concat(parts, ", ") .. " }"
end

function M.parse(text)
  local fn, err = loadstring("return " .. text, "=value")
  if not fn then return nil, err end
  setfenv(fn, {})
  local ok, v = pcall(fn)
  if not ok then return nil, v end
  return v
end

return M
