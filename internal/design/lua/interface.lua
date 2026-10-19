-- The Component Interface: what a component shows of itself to the forms
-- that use it. Its props, each its own or standing for a property of one of
-- its parts; its events, each raised by its code or passed on from a part's;
-- and which parts a form can reach by name. Everything else about the
-- parts stays private.
--
-- The dialog edits three lists of rows of text; check and apply, which do
-- not touch the screen, turn them into a layout's `props`, `events` and
-- parts' `public`.

local gui = require "gui"
local model = require "design.model"

local M = {}

local types = { "string", "number", "integer", "boolean", "color" }

-- rows reads the interface out of a component's layout, as rows of text.
function M.rows(doc)
  local props, events, parts = {}, {}, {}
  for name, d in pairs(doc.props or {}) do
    props[#props + 1] = { name, d.type or "string", d.default ~= nil and tostring(d.default) or "", d.to or "" }
  end
  for k, v in pairs(doc.events or {}) do
    if type(k) == "number" then
      events[#events + 1] = { v, "" }
    else
      events[#events + 1] = { k, v ~= true and tostring(v) or "" }
    end
  end
  table.sort(props, function(a, b) return a[1] < b[1] end)
  table.sort(events, function(a, b) return a[1] < b[1] end)
  model.walk(doc, function(node)
    if node.name then parts[#parts + 1] = { node.name, node.kind, node.public and "true" or "false" } end
  end)
  return props, events, parts
end

-- target checks "part.name" against the parts, and what the part has: its
-- properties or its events. It returns why not.
local function target(text, parts, what, member)
  local part, name = text:match("^([%a_][%w_]*)%.([%a_][%w_]*)$")
  if not part then return ("%s is not written part.%s"):format(text, member) end
  for _, p in ipairs(parts) do
    if p[1] == part then
      local info = model.kinds()[p[2]]
      local ok = false
      if member == "property" then
        ok = info.props[name] ~= nil
      else
        for _, e in ipairs(info.events) do ok = ok or e == name end
      end
      if not ok then return ("a %s has no %s %s"):format(p[2], member, name) end
      return nil
    end
  end
  return ("there is no part called %s"):format(part), what
end

-- check says why the rows cannot be an interface, or returns nil.
function M.check(props, events, parts)
  local common = model.kinds().Frame.props
  local seen = {}
  for _, r in ipairs(props) do
    local name = r[1]
    if not name:match("^[%a_][%w_]*$") then return ("%q is not a name for a property"):format(name) end
    if common[name] or name == "kind" or name == "parent" or name:match("^on%u") then
      return name .. " is a property or event every control has; pick another name"
    end
    if seen[name] then return name .. " is there twice" end
    seen[name] = true
    local ok = false
    for _, t in ipairs(types) do ok = ok or t == r[2] end
    if not ok then return ("%s: the type is one of %s"):format(name, table.concat(types, ", ")) end
    if r[3] ~= "" then
      if (r[2] == "number" or r[2] == "integer") and not tonumber(r[3]) then
        return name .. ": the default must be a number"
      end
      if r[2] == "boolean" and r[3] ~= "true" and r[3] ~= "false" then
        return name .. ": the default must be true or false"
      end
    end
    if r[4] ~= "" then
      local why = target(r[4], parts, name, "property")
      if why then return name .. ": " .. why end
    end
  end
  local names = {}
  for _, r in ipairs(events) do
    local name = r[1]
    if not name:match("^on%u[%w_]*$") then return ("%q is not an event name: as onChange"):format(name) end
    if names[name] then return name .. " is there twice" end
    names[name] = true
    for _, e in ipairs(model.kinds().Frame.events) do
      if e == name then return name .. " is an event every control has" end
    end
    if r[2] ~= "" then
      local why = target(r[2], parts, name, "event")
      if why then return name .. ": " .. why end
    end
  end
end

-- apply writes the rows into the layout. A prop's default is read as its
-- type says; nothing typed is no default.
function M.apply(doc, props, events, parts)
  local p, e = {}, {}
  for _, r in ipairs(props) do
    local d = { type = r[2] }
    if r[3] ~= "" then
      if r[2] == "number" or r[2] == "integer" then d.default = tonumber(r[3])
      elseif r[2] == "boolean" then d.default = r[3] == "true"
      else d.default = r[3] end
    end
    if r[4] ~= "" then d.to = r[4] end
    p[r[1]] = d
  end
  for _, r in ipairs(events) do
    if r[2] ~= "" then e[r[1]] = r[2] else e[#e + 1] = r[1] end
  end
  doc.props = next(p) and p or nil
  doc.events = next(e) and e or nil
  local public = {}
  for _, r in ipairs(parts) do public[r[1]] = r[3] == "true" end
  model.walk(doc, function(node)
    if node.name then node.public = public[node.name] or nil end
  end)
end

-- edit shows the dialog on a component's layout and changes the layout if
-- OK is pressed, returning true. ed is filled in with its parts, for tests.
function M.edit(doc, ed)
  ed = ed or {}
  local props, events, parts = M.rows(doc)
  local saved

  local dlg = gui.Form { caption = "Component Interface: " .. tostring(doc.name), width = 600, height = 560 }
  ed.form = dlg
  ed.props, ed.events, ed.parts = props, events, parts

  local function section(caption, top)
    dlg:Label { caption = caption, left = 10, top = top, width = 300 }
  end

  section("Properties: what a form can set on it", 8)
  ed.propGrid = dlg:Table {
    left = 10, top = 30, width = 580, height = 130,
    columns = { "Name", "Type", "Default", "Is a property of a part" },
    columnWidths = { 120, 80, 100, 260 },
    rows = props, editable = true,
    onStartEdit = function(_, _, col)
      if col == 2 then return { choices = types } end
    end,
  }
  ed.addProp = dlg:Button {
    caption = "Add", left = 10, top = 164, width = 80,
    onClick = function()
      props[#props + 1] = { "", "string", "", "" }
      ed.propGrid.rows = props
      ed.propGrid:edit(#props, 1)
    end,
  }
  ed.removeProp = dlg:Button {
    caption = "Remove", left = 96, top = 164, width = 80,
    onClick = function()
      local i = ed.propGrid.selected
      if i > 0 then table.remove(props, i); ed.propGrid.rows = props end
    end,
  }

  section("Events: what a form can handle", 200)
  ed.eventGrid = dlg:Table {
    left = 10, top = 222, width = 580, height = 110,
    columns = { "Name", "Passed on from an event of a part" },
    columnWidths = { 160, 400 },
    rows = events, editable = true,
  }
  ed.addEvent = dlg:Button {
    caption = "Add", left = 10, top = 336, width = 80,
    onClick = function()
      events[#events + 1] = { "", "" }
      ed.eventGrid.rows = events
      ed.eventGrid:edit(#events, 1)
    end,
  }
  ed.removeEvent = dlg:Button {
    caption = "Remove", left = 96, top = 336, width = 80,
    onClick = function()
      local i = ed.eventGrid.selected
      if i > 0 then table.remove(events, i); ed.eventGrid.rows = events end
    end,
  }
  dlg:Label {
    caption = "An event with nothing after it is raised by the code: self:fire(\"onName\").",
    left = 190, top = 340, width = 400, fontSize = 12,
  }

  section("Parts: the ones a form can reach by name are public", 376)
  ed.partGrid = dlg:Table {
    left = 10, top = 398, width = 580, height = 110,
    columns = { "Part", "Kind", "Public" },
    columnWidths = { 200, 160, 200 },
    rows = parts, editable = { 3 },
    onStartEdit = function() return { choices = { "true", "false" } } end,
  }

  dlg:Button { caption = "Cancel", left = 400, top = 518, width = 90, onClick = function() dlg:close() end }
  ed.ok = dlg:Button {
    caption = "OK", left = 500, top = 518, width = 90, default = true,
    onClick = function()
      if ed.propGrid:editing() then ed.propGrid:edit() end
      if ed.eventGrid:editing() then ed.eventGrid:edit() end
      local why = M.check(props, events, parts)
      if why then
        ed.problem = why
        gui.msgbox(why)
        return
      end
      M.apply(doc, props, events, parts)
      saved = true
      dlg:close()
    end,
  }

  if ed.onShow then gui.after(0.1, function() ed.onShow(ed) end) end
  dlg:showModal()
  return saved
end

return M
