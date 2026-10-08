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
  return ([[-- The code behind %s.form.lua: its controls are fields of the form, by
-- the names the layout gives them, as frm.Button1.

local gui = require "gui"
local frm = gui.load "%s"

return frm
]]):format(name, name)
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
  local code = dir .. "/forms/" .. name .. ".lua"
  if not exists(code) then write(code, codeLua(name)) end
end

M.exists = exists

return M
