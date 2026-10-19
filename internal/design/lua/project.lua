-- A project on disk: a folder with main.lua, which shows the first form,
-- and forms/, where each form is a layout file the designer writes
-- (Name.form.lua) and a code file it only ever starts (Name.lua).

local gui = require "gui"
local lfs = require "lfs"
local model = require "design.model"

local M = {}

local function exists(path)
  return lfs.attributes(path) ~= nil
end

local function write(path, text)
  local f = assert(io.open(path, "w"))
  f:write(text)
  f:close()
end

local function mainLua(startup)
  return ([[#!/usr/bin/env tlua
-- Starts the program by showing its first form. Written by tlua design.

local gui = bootgui()
require("forms.%s"):show()
]]):format(startup)
end

local function codeLua(name)
  return ([=[-- The code behind %s.form.lua: the controls are fields of the form, by
-- the names the layout gives them.

local gui = require "gui"
local frm = gui.load "%s" --[[@as forms.%s]]

return frm
]=]):format(name, name, name)
end

local function controlLua(name)
  return ([[-- A control of this project's own. A form's layout that uses a %s
-- finds it here, as controls.%s, the first time it is needed; the designer
-- shows it in its toolbox. It is yours: draw it, give it props and events.

local gui = require "gui"

gui.define {
  name = "%s",
  events = { "onChange" },
  props = { value = { type = "integer", default = 0 } },
  build = function(parent, opts)
    local c = parent:Canvas { width = 120, height = 30 }
    c.value = opts.value or 0
    function c:onDraw(g)
      local w, h = g:size()
      g:color("#e4ecfa"); g:fill(0, 0, w, h)
      g:color("#1e64d8"); g:rect(0, 0, w, h)
      g:color("black"); g:text("%s " .. tostring(self.value), 0, 0, w, h, "center")
    end
    function c:onMouseDown()
      self.value = self.value + 1
      self:fire("onChange", self.value)
    end
    return c
  end,
}
]]):format(name, name, name, name)
end

-- forms lists the forms of the project in dir, by name.
function M.forms(dir)
  local names = {}
  if not exists(dir .. "/forms") then return names end
  for file in lfs.dir(dir .. "/forms") do
    local name = file:match("^(.+)%.form%.lua$")
    if name then names[#names + 1] = name end
  end
  table.sort(names)
  return names
end

-- create makes a new project in dir, with one form, and leaves anything
-- already there alone.
function M.create(dir, first)
  first = first or "Form1"
  lfs.mkdir(dir)
  lfs.mkdir(dir .. "/forms")
  if not exists(dir .. "/main.lua") then write(dir .. "/main.lua", mainLua(first)) end
  M.addForm(dir, first)
end

-- addForm adds a form: an empty layout, and the code that loads it.
function M.addForm(dir, name)
  lfs.mkdir(dir .. "/forms")
  local layout = M.layoutPath(dir, name)
  if not exists(layout) then gui.save(model.newForm(name), layout) end
  local code = dir .. "/forms/" .. name .. ".lua"
  if not exists(code) then write(code, codeLua(name)) end
end

function M.layoutPath(dir, name)
  return dir .. "/forms/" .. name .. ".form.lua"
end

-- controls lists the project's own controls, by name.
function M.controls(dir)
  local names = {}
  if not exists(dir .. "/controls") then return names end
  for file in lfs.dir(dir .. "/controls") do
    local name = file:match("^([%a_][%w_]*)%.lua$")
    if name then names[#names + 1] = name end
  end
  table.sort(names)
  return names
end

-- addControl starts a control of the project's own.
function M.addControl(dir, name)
  lfs.mkdir(dir .. "/controls")
  local path = dir .. "/controls/" .. name .. ".lua"
  if exists(path) then return false end
  write(path, controlLua(name))
  return true
end

-- classOf is the language server's name for what a layout's kind makes: a
-- component of the project has a class of its own.
local function classOf(kind, dir)
  local info = model.kinds()[kind]
  if info and (not info.defined or info.builtin) then return "gui." .. kind end
  if dir and exists(M.componentPath(dir, kind)) then return "components." .. kind end
  return "gui.Object"
end

-- stub writes Name.d.lua beside a form: its controls, as the fields of a
-- class the code's frm is declared as, so a language server completes
-- frm.cmdGreet. with a Button's fields.
function M.stub(dir, name, doc)
  local lines = {
    "---@meta",
    "-- What " .. name .. ".form.lua has in it, for a language server. Written by tlua",
    "-- design each time the form is saved; changing it here changes nothing.",
    "",
    "---@class forms." .. name .. ": gui.Form",
  }
  local fields = {}
  model.walk(doc, function(node)
    if node.name then fields[#fields + 1] = ("---@field %s %s"):format(node.name, classOf(node.kind, dir)) end
  end)
  table.sort(fields)
  for _, f in ipairs(fields) do lines[#lines + 1] = f end
  write(dir .. "/forms/" .. name .. ".d.lua", table.concat(lines, "\n") .. "\n")
end

-- read loads a form's layout as a table, running the file with nothing in
-- scope, as gui.load does.
function M.read(dir, name)
  local fn, err = loadfile(M.layoutPath(dir, name))
  if not fn then return nil, err end
  setfenv(fn, {})
  local ok, doc = pcall(fn)
  if not ok then return nil, doc end
  if type(doc) ~= "table" or doc.kind ~= "Form" then
    return nil, name .. ".form.lua does not return a form's layout"
  end
  return doc
end

-- save writes a form's layout; its code is the programmer's, and stays as
-- it is.
function M.save(dir, name, doc)
  gui.save(doc, M.layoutPath(dir, name))
  M.stub(dir, name, doc)
  local code = dir .. "/forms/" .. name .. ".lua"
  if not exists(code) then write(code, codeLua(name)) end
end

-- startup is the form main.lua shows first, by name.
function M.startup(dir)
  local f = io.open(dir .. "/main.lua", "r")
  if not f then return nil end
  local text = f:read("*a")
  f:close()
  return text:match('require%s*%(?%s*["\']forms%.([%a_][%w_]*)["\']'), text
end

-- setStartup makes main.lua show another form first, if main.lua is still
-- as the designer wrote it; otherwise it says what to change by hand.
function M.setStartup(dir, name)
  local current, text = M.startup(dir)
  if text and current and text ~= mainLua(current) then
    return false, ("main.lua has been changed by hand, so it is left alone: make it require \"forms.%s\""):format(name)
  end
  write(dir .. "/main.lua", mainLua(name))
  return true
end

M.exists = exists

---------------------------------------------------------------- components

-- A component is a layout of controls that work together, kept as
-- components/Name.comp.lua (the designer's), components/Name.lua (the code,
-- yours: a function of the component and its parts) and Name.d.lua (for a
-- language server). controls/Name.lua, written beside them every time, is
-- the gui.component that makes it a kind of control the toolbox offers.

local function componentCode(name)
  return ([[-- The code behind components/%s.comp.lua. It runs once for each %s
-- that is made: self is the component, and parts are its controls by name.

local gui = require "gui"

---@param self components.%s
---@param parts components.%s.parts
return function(self, parts)
end
]]):format(name, name, name, name)
end

local function componentControl(name)
  return ([[-- Written by tlua design from components/%s.comp.lua; written again on
-- every save, so changes belong there and in components/%s.lua.

require("gui").component {
  layout = "../components/%s",
  code = "components.%s",
}
]]):format(name, name, name, name)
end

function M.componentPath(dir, name)
  return dir .. "/components/" .. name .. ".comp.lua"
end

-- components lists the components of the project in dir, by name.
function M.components(dir)
  local names = {}
  if not exists(dir .. "/components") then return names end
  for file in lfs.dir(dir .. "/components") do
    local name = file:match("^(.+)%.comp%.lua$")
    if name then names[#names + 1] = name end
  end
  table.sort(names)
  return names
end

-- addComponent starts a component: an empty layout, and the code that goes
-- with it. A name a control has already is refused.
function M.addComponent(dir, name)
  if exists(dir .. "/controls/" .. name .. ".lua") then return false end
  lfs.mkdir(dir .. "/components")
  M.saveComponent(dir, name, model.newComponent(name))
  return true
end

local luaTypes = {
  string = "string", color = "string", choice = "string", file = "string", name = "string",
  number = "number", integer = "integer", boolean = "boolean",
}

-- componentStub writes components/Name.d.lua: the parts by name, for the
-- code's `parts`, and the component with its props and public parts, for a
-- form's code.
local function componentStub(dir, name, doc)
  local lines = {
    "---@meta",
    "-- What " .. name .. ".comp.lua has in it, for a language server. Written by tlua",
    "-- design each time the component is saved; changing it here changes nothing.",
    "",
    "---@class components." .. name .. ".parts",
  }
  local parts, public = {}, {}
  model.walk(doc, function(node)
    if node.name then
      local field = ("---@field %s %s"):format(node.name, classOf(node.kind, dir))
      parts[#parts + 1] = field
      if node.public then public[#public + 1] = field end
    end
  end)
  table.sort(parts)
  table.sort(public)
  for _, f in ipairs(parts) do lines[#lines + 1] = f end
  lines[#lines + 1] = ""
  lines[#lines + 1] = "---@class components." .. name .. ": gui.Frame"
  local props = {}
  for prop, info in pairs(doc.props or {}) do
    props[#props + 1] = ("---@field %s? %s"):format(prop, luaTypes[info.type or "string"] or "any")
  end
  table.sort(props)
  for _, f in ipairs(props) do lines[#lines + 1] = f end
  for _, f in ipairs(public) do lines[#lines + 1] = f end
  write(dir .. "/components/" .. name .. ".d.lua", table.concat(lines, "\n") .. "\n")
end

-- readComponent loads a component's layout as a table, with nothing in
-- scope, as gui.component does.
function M.readComponent(dir, name)
  local fn, err = loadfile(M.componentPath(dir, name))
  if not fn then return nil, err end
  setfenv(fn, {})
  local ok, doc = pcall(fn)
  if not ok then return nil, doc end
  if type(doc) ~= "table" or (doc.kind ~= "Component" and doc.kind ~= "Frame") then
    return nil, name .. ".comp.lua does not return a component's layout"
  end
  doc.kind = "Component"
  return doc
end

-- rewriteUses changes the layouts that use a component after its props were
-- renamed or taken away: forms and other components. It returns the names
-- of the layouts it rewrote.
function M.rewriteUses(dir, kind, renamed, removed)
  local changed = {}
  local function fix(doc)
    local touched = false
    model.walk(doc, function(node)
      if node.kind == kind then
        for old, new in pairs(renamed) do
          if node[old] ~= nil then node[new], node[old], touched = node[old], nil, true end
        end
        for old in pairs(removed) do
          if node[old] ~= nil then node[old], touched = nil, true end
        end
      end
    end)
    return touched
  end
  for _, name in ipairs(M.forms(dir)) do
    local doc = M.read(dir, name)
    if doc and fix(doc) then
      gui.save(doc, M.layoutPath(dir, name))
      changed[#changed + 1] = name .. ".form.lua"
    end
  end
  for _, name in ipairs(M.components(dir)) do
    local doc = name ~= kind and M.readComponent(dir, name)
    if doc and fix(doc) then
      gui.save(doc, M.componentPath(dir, name))
      changed[#changed + 1] = name .. ".comp.lua"
    end
  end
  return changed
end

-- mentions lists the code files of the project's forms and components that
-- name a word as a field or a method (.word or :word).
function M.mentions(dir, word)
  local found = {}
  local function look(folder)
    if not exists(dir .. "/" .. folder) then return end
    for file in lfs.dir(dir .. "/" .. folder) do
      local path = dir .. "/" .. folder .. "/" .. file
      if file:match("%.lua$") and not file:match("%.form%.lua$") and not file:match("%.comp%.lua$")
          and not file:match("%.d%.lua$") then
        local f = io.open(path, "r")
        local text = f and f:read("*a") or ""
        if f then f:close() end
        if text:find("[%.:]" .. word .. "%f[^%w_]") then found[#found + 1] = folder .. "/" .. file end
      end
    end
  end
  look("forms")
  look("components")
  table.sort(found)
  return found
end

-- saveComponent writes a component's layout and what goes with it; its code
-- is the programmer's, and stays as it is.
function M.saveComponent(dir, name, doc)
  lfs.mkdir(dir .. "/components")
  lfs.mkdir(dir .. "/controls")
  gui.save(doc, M.componentPath(dir, name))
  componentStub(dir, name, doc)
  write(dir .. "/controls/" .. name .. ".lua", componentControl(name))
  local code = dir .. "/components/" .. name .. ".lua"
  if not exists(code) then write(code, componentCode(name)) end
end

-- A layout names its images relative to forms/, where the form's code is:
-- that is where gui.load looks for them, in a packed program too.

local function slashes(path)
  return (path:gsub("\\", "/"))
end

-- imagePath is how a layout names a file chosen for it, or nil when the
-- file is outside the project and would not go with it.
function M.imagePath(dir, file)
  local root = slashes(dir):gsub("/+$", "") .. "/"
  file = slashes(file)
  if file:sub(1, #root) ~= root then return nil end
  local rel = file:sub(#root + 1)
  if rel:sub(1, 6) == "forms/" then return rel:sub(7) end
  return "../" .. rel
end

-- imageDir is the folder an image's chooser starts in: the one the image
-- named now is in, or the project's.
function M.imageDir(dir, current)
  if current and current ~= "" then
    local path = current
    if not (current:match("^/") or current:match("^%a:[/\\]")) then
      path = dir .. "/forms/" .. current
    end
    local folder = slashes(path):match("^(.*)/[^/]*$")
    if folder and exists(folder) then return folder end
  end
  return dir
end

-- copyIn copies a file into the project's images/ folder, under a name of
-- its own, and returns how a layout names the copy.
function M.copyIn(dir, file)
  local src, err = io.open(file, "rb")
  if not src then return nil, err end
  local data = src:read("*a")
  src:close()
  lfs.mkdir(dir .. "/images")
  local name = slashes(file):match("[^/]+$")
  local base, ext = name:match("^(.-)(%.[^.]*)$")
  if not base or base == "" then base, ext = name, "" end
  local target, n = "images/" .. name, 1
  while exists(dir .. "/" .. target) do
    n = n + 1
    target = ("images/%s%d%s"):format(base, n, ext)
  end
  local out, werr = io.open(dir .. "/" .. target, "wb")
  if not out then return nil, werr end
  out:write(data)
  out:close()
  return "../" .. target
end

return M
