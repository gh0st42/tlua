-- gui.component: a control made of others, from a layout.
--
-- The layout is a table, or the path of a file returning one, as gui.load
-- takes, with a Component at its top:
--
--   return {
--     kind = "Component", name = "LabeledField", width = 240, height = 28,
--     props  = { label = { type = "string", default = "Name", to = "lbl.caption" } },
--     events = { onChange = "txt.onChange" },
--     { kind = "Label",   name = "lbl", left = 0, top = 4, width = 80 },
--     { kind = "TextBox", name = "txt", left = 84, top = 0, width = 156, public = true },
--   }
--
-- The gui module defines gui.component when it opens; host is what it needs
-- from Go (see component.go).

local gui, host = ...

local function fail(message, ...)
  error("gui.component: " .. message:format(...), 3)
end

-- copy is a layout node made over, without what only a component reads.
local function copy(t)
  local out = {}
  for k, v in pairs(t) do
    if type(v) == "table" then v = copy(v) end
    out[k] = v
  end
  if out.kind ~= nil then out.public = nil end
  return out
end

-- partsOf lists the names of the parts under a node, and which are public.
local function partsOf(node, names, public)
  for _, child in ipairs(node) do
    if type(child) == "table" then
      if child.name and child.name ~= "" then
        names[#names + 1] = child.name
        if child.public then public[child.name] = true end
      end
      partsOf(child, names, public)
    end
  end
end

-- target splits "part.property".
local function target(spec, what, names)
  local part, prop = tostring(spec):match("^([%a_][%w_]*)%.([%a_][%w_]*)$")
  if not part then fail("%s: %q is not written part.property", what, tostring(spec)) end
  if not names[part] then fail("%s: there is no part called %s", what, part) end
  return part, prop
end

local building = {}

function gui.component(spec)
  if type(spec) ~= "table" then fail("expects a table, as gui.define does") end
  local layout = spec.layout
  if type(layout) == "string" then layout = host.layout(layout) end
  if type(layout) ~= "table" then
    fail("layout must be a layout table, or the path of a layout file")
  end
  if layout.kind ~= nil and layout.kind ~= "Component" and layout.kind ~= "Frame" then
    fail("the top of a component's layout is a Component, not a %s", tostring(layout.kind))
  end
  local name = spec.name or layout.name

  -- What the component shows of itself.
  local props, forwards, events, eventNames = {}, {}, {}, {}
  local names, public, list = {}, {}, {}
  partsOf(layout, list, public)
  for _, n in ipairs(list) do names[n] = true end

  for prop, d in pairs(layout.props or {}) do
    if type(d) ~= "table" then fail("props are written name = {type = ..., default = ...}") end
    props[prop] = { type = d.type, default = d.default, choices = d.choices }
    if d.to then
      local part, partProp = target(d.to, "prop " .. prop, names)
      forwards[prop] = { part, partProp }
    end
  end
  for k, v in pairs(layout.events or {}) do
    local event, to
    if type(k) == "number" then event = v else event, to = k, v end
    eventNames[#eventNames + 1] = event
    if to and to ~= true then
      local part, partEvent = target(to, "event " .. event, names)
      events[event] = { part, partEvent }
    end
  end
  table.sort(eventNames)

  local code = spec.code
  if type(code) == "string" then code = require(code) end
  if code ~= nil and type(code) ~= "function" then
    fail("code is a function(self, parts), or the name of a module returning one")
  end

  local frameOpts = {}
  for k, v in pairs(layout) do
    if type(k) == "string" and k ~= "kind" and k ~= "name" and k ~= "props"
        and k ~= "events" and k ~= "left" and k ~= "top" then
      frameOpts[k] = v
    end
  end

  host.undefine(name) -- a component defined before is replaced
  gui.define {
    name = name,
    events = eventNames,
    props = props,
    build = function(parent, opts)
      if building[name] then fail("%s holds itself", name) end
      building[name] = true
      local ok, frame, parts = pcall(function()
        local frame = parent:Frame(copy(frameOpts))
        host.scope(frame)
        for _, node in ipairs(layout) do gui.load(copy(node), frame) end

        local parts = {}
        for _, n in ipairs(list) do parts[n] = frame:find(n) end
        for n in pairs(public) do host.expose(frame, n) end

        for prop, d in pairs(props) do
          local to = forwards[prop]
          if to then host.forward(frame, prop, parts[to[1]], to[2]) end
          if d.default ~= nil then frame[prop] = d.default end
        end
        for event, to in pairs(events) do
          parts[to[1]][to[2]] = function(_, ...) return frame:fire(event, ...) end
        end
        if code then code(frame, parts) end
        return frame, parts
      end)
      building[name] = nil
      if not ok then error(frame, 0) end
      return frame
    end,
  }
  host.mark(name)
end
