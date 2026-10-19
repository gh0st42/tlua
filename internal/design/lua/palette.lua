-- Palettes: folders of components that projects share. TLUA_PALETTE lists
-- them, as PATH does (separated by : or, on Windows, ;), and defaults to
-- ~/.tlua/palette. A component in a palette is two files, Name.comp.lua and
-- Name.lua; adding it to a project copies them into the project's
-- components/ and makes it a control there, so a project stays complete in
-- itself. Which palette a copy came from, and what that held, is kept in
-- components/Name.palette, to say when the palette has a newer one.

local lfs = require "lfs"
local model = require "design.model"
local project = require "design.project"

local M = {}

local windows = package.config:sub(1, 1) == "\\"

-- folders lists the palettes, in the order TLUA_PALETTE gives them.
function M.folders()
  local list = os.getenv("TLUA_PALETTE")
  local out = {}
  if list and list ~= "" then
    for folder in list:gmatch("[^" .. (windows and ";" or ":") .. "]+") do out[#out + 1] = folder end
  else
    local home = os.getenv("HOME") or os.getenv("USERPROFILE")
    if home then out[1] = home .. "/.tlua/palette" end
  end
  return out
end

local function read(path)
  local f = io.open(path, "rb")
  if not f then return nil end
  local text = f:read("*a")
  f:close()
  return text
end

local function write(path, text)
  local f, err = io.open(path, "wb")
  if not f then return nil, err end
  f:write(text)
  f:close()
  return true
end

-- mkdirs makes a folder and those above it.
local function mkdirs(path)
  local so_far = path:sub(1, 1) == "/" and "" or nil
  for part in path:gmatch("[^/\\]+") do
    so_far = so_far and (so_far .. "/" .. part) or part
    if not path:find("^%a:") or #so_far > 2 then lfs.mkdir(so_far) end
  end
  return lfs.attributes(path, "mode") == "directory"
end

-- sum says what a component in a folder holds, as a short text that changes
-- when either of its files does.
local function sum(folder, name)
  local text = (read(folder .. "/" .. name .. ".comp.lua") or "") .. "\0" .. (read(folder .. "/" .. name .. ".lua") or "")
  local a, b = 0, 0
  for i = 1, #text do
    a = (a + text:byte(i)) % 65521
    b = (b + a) % 65521
  end
  return ("%d-%d-%04x%04x"):format(#text, select(2, text:gsub("\n", "")), b, a)
end

-- list is every component in the palettes, as { folder =, name = }.
function M.list()
  local out = {}
  for _, folder in ipairs(M.folders()) do
    if lfs.attributes(folder, "mode") == "directory" then
      local names = {}
      for file in lfs.dir(folder) do
        local name = file:match("^([%a_][%w_]*)%.comp%.lua$")
        if name then names[#names + 1] = name end
      end
      table.sort(names)
      for _, name in ipairs(names) do out[#out + 1] = { folder = folder, name = name } end
    end
  end
  return out
end

-- put copies a project's component into a palette folder.
function M.put(dir, name, folder)
  if not mkdirs(folder) then return nil, "cannot make " .. folder end
  local layout = read(project.componentPath(dir, name))
  if not layout then return nil, name .. " has not been saved" end
  local ok, err = write(folder .. "/" .. name .. ".comp.lua", layout)
  if not ok then return nil, err end
  ok, err = write(folder .. "/" .. name .. ".lua", read(dir .. "/components/" .. name .. ".lua") or "")
  if not ok then return nil, err end
  -- What it was copied from is what it is now.
  write(dir .. "/components/" .. name .. ".palette", folder .. "\n" .. sum(folder, name) .. "\n")
  return true
end

-- take copies a component of a palette into a project: its files, and what
-- the designer writes beside them. It returns the names of the kinds the
-- component uses that the project lacks.
function M.take(dir, folder, name)
  local layout = read(folder .. "/" .. name .. ".comp.lua")
  if not layout then return nil, name .. " is not in " .. folder end
  lfs.mkdir(dir .. "/components")
  local ok, err = write(project.componentPath(dir, name), layout)
  if not ok then return nil, err end
  ok, err = write(dir .. "/components/" .. name .. ".lua", read(folder .. "/" .. name .. ".lua") or "")
  if not ok then return nil, err end
  local doc, why = project.readComponent(dir, name)
  if not doc then return nil, why end
  project.saveComponent(dir, name, doc)
  write(dir .. "/components/" .. name .. ".palette", folder .. "\n" .. sum(folder, name) .. "\n")
  local missing, seen = {}, {}
  model.walk(doc, function(node)
    local kinds = model.kinds()
    if not kinds[node.kind] and not seen[node.kind]
        and not project.exists(project.componentPath(dir, node.kind))
        and not project.exists(dir .. "/controls/" .. node.kind .. ".lua") then
      seen[node.kind] = true
      missing[#missing + 1] = node.kind
    end
  end)
  table.sort(missing)
  return true, missing
end

-- behind says whether the palette a component was taken from has a newer
-- one, and returns the folder.
function M.behind(dir, name)
  local note = read(dir .. "/components/" .. name .. ".palette")
  if not note then return false end
  local folder, was = note:match("^(.-)\n(.-)\n")
  if not folder or not project.exists(folder .. "/" .. name .. ".comp.lua") then return false end
  return sum(folder, name) ~= was, folder
end

return M
