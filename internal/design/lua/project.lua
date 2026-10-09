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

-- classOf is the language server's name for what a layout's kind makes.
local function classOf(kind)
  local info = model.kinds()[kind]
  if info and not info.defined then return "gui." .. kind end
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
    if node.name then fields[#fields + 1] = ("---@field %s %s"):format(node.name, classOf(node.kind)) end
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

return M
