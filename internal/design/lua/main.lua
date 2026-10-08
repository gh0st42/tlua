-- tlua design: the window that puts the designer together. The project
-- tree and toolbox are on the left, the form being designed in the middle,
-- its properties on the right, and the output of a run below.

local gui = require "gui"
local lfs = require "lfs"
local model = require "design.model"
local project = require "design.project"
local surface = require "design.surface"
local props = require "design.props"
local toolbox = require "design.toolbox"

local M = {}

local W, H = 1180, 760
local MENU, OUTPUT, STATUS = 25, 110, 24
local LEFT, RIGHT = 210, 300

-- start opens the designer on the project in opts.dir: made there first if
-- it has no forms and opts.create says so, or asked about otherwise. It
-- returns the designer, which is what tests hold on to.
function M.start(opts)
  local d = { dirty = false }
  local splitH = H - MENU - OUTPUT - STATUS

  local win = gui.Form { caption = "tlua design", width = W, height = H, resizable = true }
  d.win = win

  local split = win:Splitter { left = 0, top = MENU, width = W, height = splitH, grow = true }
  local leftPane = split:Panel { left = 0, top = 0, width = LEFT, height = splitH }
  local center = split:Panel { left = LEFT, top = 0, width = W - LEFT - RIGHT, height = splitH }
  local rightPane = split:Panel { left = W - RIGHT, top = 0, width = RIGHT, height = splitH }

  leftPane:Label { caption = "Forms", left = 6, top = 2, width = LEFT - 12, height = 20 }
  d.tree = leftPane:Tree {
    left = 6, top = 22, width = LEFT - 12, height = 130,
    onDoubleClick = function(self) d:openForm(self.text) end,
  }
  leftPane:Label { caption = "Toolbox", left = 6, top = 158, width = LEFT - 12, height = 20 }
  d.toolbox = toolbox.new(leftPane, {
    left = 6, top = 178, width = LEFT - 12, height = splitH - 184, grow = true,
    onPick = function() end,
    onPlace = function(kind) if d.doc then d.surface:place(kind) end end,
  })

  d.area = center:Scroll { left = 0, top = 0, width = W - LEFT - RIGHT, height = splitH, grow = true, color = "#8a8f99" }
  d.surface = surface.new(d.area, d)

  rightPane:Label { caption = "Properties", left = 6, top = 2, width = RIGHT - 12, height = 20 }
  d.props = props.new(rightPane, d, { left = 6, top = 24, width = RIGHT - 12, height = splitH - 30 })

  d.output = win:ListBox { left = 0, top = MENU + splitH, width = W, height = OUTPUT, font = "mono" }
  d.statusLine = win:Label { left = 6, top = H - STATUS, width = W - 12, height = STATUS }

  ---------------------------------------------------------------- the designer's side of things

  function d:tool() return self.toolbox:tool() end
  function d:resetTool() self.toolbox:reset(); self.toolbox:redraw() end
  function d:status(text) self.statusLine.caption = text or "" end

  function d:selected(node)
    self.selection = node
    self.props:show(node)
  end

  function d:selectNode(node)
    self.surface:selectNode(node)
  end

  -- moved is the surface saying a control (or the form) was dragged: the
  -- grid follows, and the layout has changed.
  function d:moved(node)
    self.props:refresh(node)
    self:markDirty()
  end

  function d:changed()
    self.props:list(self.doc)
    for i, n in ipairs(self.props.nodes) do
      if n == self.selection then self.props.picker.selected = i end
    end
    self:markDirty()
  end

  function d:markDirty()
    if not self.dirty then
      self.dirty = true
      self:retitle()
    end
  end

  function d:setProp(node, prop, value)
    return self.surface:setProp(node, prop, value)
  end

  function d:retitle()
    local parts = { "tlua design" }
    if self.dir then parts[#parts + 1] = self.dir:match("([^/\\]+)[/\\]?$") end
    if self.formName then parts[#parts + 1] = self.formName .. (self.dirty and " *" or "") end
    self.win.caption = table.concat(parts, " - ")
  end

  ---------------------------------------------------------------- files

  -- keepOrLose asks what to do with unsaved changes; false means stay put.
  function d:keepOrLose()
    if not self.dirty then return true end
    local answer = gui.msgbox("Save the changes to " .. self.formName .. "?", "yesnocancel", "tlua design")
    if answer == "cancel" then return false end
    if answer == "yes" then self:save() end
    return true
  end

  function d:openProject(dir)
    self.dir = dir
    -- Image paths in a layout are relative to the form's code, which lives
    -- in forms/; the designer looks for them from there too.
    lfs.chdir(dir .. "/forms")
    local forms = project.forms(dir)
    self.tree.items = forms
    self:retitle()
    if forms[1] then self:openForm(forms[1]) end
  end

  function d:openForm(name)
    if name == "" or name == self.formName then return end
    if not self:keepOrLose() then return end
    local doc, err = project.read(self.dir, name)
    if not doc then
      gui.msgbox(tostring(err), "ok", "tlua design")
      return
    end
    self.doc, self.formName, self.dirty = doc, name, false
    self.props:list(doc)
    self.surface:load(doc)
    self.surface:select(nil)
    self.tree.path = name
    self:retitle()
    self:status("")
  end

  function d:save()
    if not self.doc then return end
    project.save(self.dir, self.formName, self.doc)
    self.dirty = false
    self:retitle()
    self:status("Saved " .. self.formName .. ".form.lua")
  end

  function d:addForm()
    local n = 1
    while project.exists(project.layoutPath(self.dir, "Form" .. n)) do n = n + 1 end
    local name = gui.inputbox("A name for the new form:", "Add Form", "Form" .. n)
    if not name or name == "" then return end
    if not name:match("^[%a_][%w_]*$") then
      gui.msgbox(name .. " is not a name: use letters, digits and _.", "ok", "Add Form")
      return
    end
    if project.exists(project.layoutPath(self.dir, name)) then
      gui.msgbox("There is a form called " .. name .. " already.", "ok", "Add Form")
      return
    end
    project.addForm(self.dir, name)
    self.tree.items = project.forms(self.dir)
    self:openForm(name)
  end

  function d:newProject()
    local dir = gui.choosedir { title = "A folder for the new project" }
    if not dir or not self:keepOrLose() then return end
    project.create(dir)
    self.formName = nil
    self:openProject(dir)
  end

  function d:chooseProject()
    local dir = gui.choosedir { title = "Open a project" }
    if not dir or not self:keepOrLose() then return end
    if not project.forms(dir)[1] then
      gui.msgbox("There are no forms in " .. dir .. ".", "ok", "Open Project")
      return
    end
    self.formName = nil
    self:openProject(dir)
  end

  ---------------------------------------------------------------- running

  function d:print(line)
    local items = self.output.items
    items[#items + 1] = line
    while #items > 2000 do table.remove(items, 1) end
    self.output.items = items
    self.output.selected = #items -- which brings it into view
  end

  function d:run()
    if not self.dir then return end
    if self.proc and self.proc.running() then self.proc.kill() end
    self:save()
    self.output.items = {}
    self:print("> tlua main.lua")
    self.proc = gui.spawn {
      gui.interpreter, "main.lua", dir = self.dir,
      onOutput = function(line) d:print(line) end,
      onExit = function(code)
        d:print(code == 0 and "> finished" or ("> exited with " .. code))
        d:status("")
      end,
    }
    self:status("Running")
  end

  function d:stop()
    if self.proc and self.proc.running() then self.proc.kill() end
  end

  ---------------------------------------------------------------- the menu and the keys

  win:Menu {
    { "&File", {
      { "&New Project...", function() d:newProject() end },
      { "&Open Project...", function() d:chooseProject() end, shortcut = "Cmd+O" },
      { "&Save", function() d:save() end, shortcut = "Cmd+S" },
      "-",
      { "&Add Form...", function() d:addForm() end },
      "-",
      { "&Quit", function() win:close() end, shortcut = "Cmd+Q" },
    } },
    { "&Edit", {
      { "&Delete", function() d.surface:deleteSelected() end },
      "-",
      { "Bring to &Front", function() d.surface:toFront() end },
      { "Send to &Back", function() d.surface:toBack() end },
    } },
    { "&Run", {
      { "&Start", function() d:run() end, shortcut = "F5" },
      { "S&top", function() d:stop() end, shortcut = "Shift+F5" },
    } },
  }

  function win:onClose()
    if not d:keepOrLose() then return false end
    d:stop()
  end

  ---------------------------------------------------------------- the project

  local dir = opts.dir
  if dir and not project.forms(dir)[1] then
    if opts.create or gui.msgbox("Start a new project in " .. dir .. "?", "yesno", "tlua design") == "yes" then
      project.create(dir)
    else
      dir = nil
    end
  end
  if dir then d:openProject(dir) end
  d:retitle()

  if opts.show ~= false then win:show() end
  return d
end

-- at is where a point of a control is in the designer's window, for tests.
function M.at(obj, dx, dy)
  local x, y = dx or 0, dy or 0
  local o = obj
  while o.parent do
    x, y = x + o.left, y + o.top
    o = o.parent
  end
  return x, y
end

return M
