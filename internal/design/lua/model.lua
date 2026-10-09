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

-- copy is a deep copy of a layout, or of any part of one.
function M.copy(v)
  if type(v) ~= "table" then return v end
  local c = {}
  for k, x in pairs(v) do c[k] = M.copy(x) end
  return c
end

---------------------------------------------------------------- arranging

M.GRID = 8

-- snap rounds a coordinate to the grid.
function M.snap(v, grid)
  grid = grid or M.GRID
  return math.floor(v / grid + 0.5) * grid
end

-- arrange works out where controls go for one of the Format menu's
-- commands. rects are {node =, l =, t =, w =, h =}; ref is the one the
-- others are lined up with (VB6's, with the solid handles); formW and formH
-- size the form. It returns the new rects, by node.
function M.arrange(op, rects, ref, formW, formH)
  local out = {}
  for _, r in ipairs(rects) do out[r.node] = { l = r.l, t = r.t, w = r.w, h = r.h } end
  local function each(fn)
    for _, r in ipairs(rects) do fn(out[r.node], r) end
  end
  if op == "lefts" then each(function(o) o.l = ref.l end)
  elseif op == "rights" then each(function(o) o.l = ref.l + ref.w - o.w end)
  elseif op == "tops" then each(function(o) o.t = ref.t end)
  elseif op == "bottoms" then each(function(o) o.t = ref.t + ref.h - o.h end)
  elseif op == "centers" then each(function(o) o.l = math.floor(ref.l + ref.w / 2 - o.w / 2) end)
  elseif op == "middles" then each(function(o) o.t = math.floor(ref.t + ref.h / 2 - o.h / 2) end)
  elseif op == "width" then each(function(o) o.w = ref.w end)
  elseif op == "height" then each(function(o) o.h = ref.h end)
  elseif op == "size" then each(function(o) o.w, o.h = ref.w, ref.h end)
  elseif op == "centerH" or op == "centerV" then
    -- The selection moves as one, into the middle of the form.
    local l, t, r, b = math.huge, math.huge, -math.huge, -math.huge
    each(function(o) l, t = math.min(l, o.l), math.min(t, o.t); r, b = math.max(r, o.l + o.w), math.max(b, o.t + o.h) end)
    local dx = op == "centerH" and math.floor((formW - (r - l)) / 2) - l or 0
    local dy = op == "centerV" and math.floor((formH - (b - t)) / 2) - t or 0
    each(function(o) o.l, o.t = o.l + dx, o.t + dy end)
  elseif op == "spaceH" or op == "spaceV" then
    -- The first and last stay; those between are spread so that the gaps
    -- between them are the same.
    local pos, size = op == "spaceH" and "l" or "t", op == "spaceH" and "w" or "h"
    local sorted = {}
    for _, r in ipairs(rects) do sorted[#sorted + 1] = out[r.node] end
    table.sort(sorted, function(a, b) return a[pos] < b[pos] end)
    if #sorted > 2 then
      local first, last = sorted[1], sorted[#sorted]
      local total = 0
      for _, o in ipairs(sorted) do total = total + o[size] end
      local gap = ((last[pos] + last[size]) - first[pos] - total) / (#sorted - 1)
      local at = first[pos]
      for _, o in ipairs(sorted) do
        o[pos] = math.floor(at + 0.5)
        at = at + o[size] + gap
      end
    end
  end
  return out
end

---------------------------------------------------------------- copying controls

local CLIP = "-- tlua design: controls\n"

-- copyText writes controls as text to paste: a Lua list of their layouts.
function M.copyText(nodes)
  local list = {}
  for i, n in ipairs(nodes) do list[i] = M.copy(n) end
  return CLIP .. M.literal(list)
end

-- rename gives a pasted control, and anything in it, names that are free.
local function rename(doc, node, taken)
  if node.name then
    if taken[node.name] then
      local n = 1
      while taken[node.kind .. n] do n = n + 1 end
      if node.caption == node.name then node.caption = node.kind .. n end
      node.name = node.kind .. n
    end
    taken[node.name] = true
  end
  for _, child in ipairs(node) do rename(doc, child, taken) end
end

-- pasteNodes reads copied controls back, renamed where their names are
-- taken and moved by offset; nil if the text is not copied controls.
function M.pasteNodes(text, doc, offset)
  if type(text) ~= "string" or text:sub(1, #CLIP) ~= CLIP then return nil end
  local list = M.parse(text:sub(#CLIP + 1))
  if type(list) ~= "table" then return nil end
  local taken = M.names(doc)
  for _, node in ipairs(list) do
    if type(node) ~= "table" or not M.kinds()[node.kind] then return nil end
    rename(doc, node, taken)
    node.left = (node.left or 0) + (offset or 0)
    node.top = (node.top or 0) + (offset or 0)
  end
  return list
end

---------------------------------------------------------------- menus

-- flattenMenu turns a Menu's items into a list the Menu Editor shows a line
-- at a time: each with its level, and "-" for a separator.
function M.flattenMenu(items, level, out)
  out, level = out or {}, level or 0
  for _, item in ipairs(items or {}) do
    if item == "-" then
      out[#out + 1] = { level = level, caption = "-" }
    elseif type(item) == "table" then
      local e = {
        level = level, caption = item[1] or "", name = item.name, shortcut = item.shortcut,
        checked = item.checked, enabled = item.enabled,
      }
      out[#out + 1] = e
      if type(item[2]) == "table" then M.flattenMenu(item[2], level + 1, out) end
    end
  end
  return out
end

-- unflattenMenu turns the Menu Editor's list back into items; an entry with
-- entries a level deeper after it is a submenu.
function M.unflattenMenu(list)
  local root = {}
  local stack = { { level = -1, items = root } }
  for i, e in ipairs(list) do
    while stack[#stack].level >= e.level do stack[#stack] = nil end
    local into = stack[#stack].items
    if e.caption == "-" then
      into[#into + 1] = "-"
    else
      local item = { e.caption, name = e.name, shortcut = e.shortcut, checked = e.checked, enabled = e.enabled }
      if item.name == "" then item.name = nil end
      if item.shortcut == "" then item.shortcut = nil end
      if item.enabled ~= false then item.enabled = nil end
      into[#into + 1] = item
      local nextEntry = list[i + 1]
      if nextEntry and nextEntry.level > e.level then
        item[2] = {}
        stack[#stack + 1] = { level = e.level, items = item[2] }
      end
    end
  end
  return root
end

return M
