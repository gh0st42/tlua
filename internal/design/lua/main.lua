-- tlua design: the window that puts the designer together. The project
-- tree and toolbox are on the left; the form being designed in the middle,
-- with its code on a second tab; its properties on the right; and the output
-- of a run below.

local gui = require "gui"
local lfs = require "lfs"
local model = require "design.model"
local project = require "design.project"
local surface = require "design.surface"
local props = require "design.props"
local toolbox = require "design.toolbox"
local code = require "design.code"
local menueditor = require "design.menueditor"

local M = {}

local W, H = 1180, 760
local MENU, OUTPUT, STATUS = 25, 110, 24
local LEFT, RIGHT = 210, 300

-- start opens the designer on the project in opts.dir: made there first if
-- it has no forms and opts.create says so, or asked about otherwise. It
-- returns the designer, which is what tests hold on to.
function M.start(opts)
  local d = { dirty = false, undoStack = {}, redoStack = {}, gridShown = false, snap = false }
  local splitH = H - MENU - OUTPUT - STATUS

  local win = gui.Form { caption = "tlua design", width = W, height = H, resizable = true }
  d.win = win

  local split = win:Splitter { left = 0, top = MENU, width = W, height = splitH, grow = true }
  local leftPane = split:Panel { left = 0, top = 0, width = LEFT, height = splitH }
  local center = split:Panel { left = LEFT, top = 0, width = W - LEFT - RIGHT, height = splitH }
  local rightPane = split:Panel { left = W - RIGHT, top = 0, width = RIGHT, height = splitH }

  leftPane:Label { caption = "Forms", left = 6, top = 2, width = LEFT - 12, height = 20 }
  d.tree = leftPane:Tree {
    left = 6, top = 22, width = LEFT - 12, height = 110,
    onDoubleClick = function(self) d:openForm(self.text) end,
  }
  d.startupLine = leftPane:Label { left = 6, top = 134, width = LEFT - 12, height = 20, fontSize = 12 }
  leftPane:Label { caption = "Toolbox", left = 6, top = 158, width = LEFT - 12, height = 20 }
  d.toolbox = toolbox.new(leftPane, {
    left = 6, top = 178, width = LEFT - 12, height = splitH - 184, grow = true,
    onPick = function() end,
    onPlace = function(kind) if d.doc then d.surface:place(kind) end end,
  })

  -- The middle: the form on one tab, its code on the other.
  local CW = W - LEFT - RIGHT
  d.views = center:Tabs {
    left = 0, top = 0, width = CW, height = splitH, grow = true,
    onChange = function(self) if self.selected == 2 then d:showCode() end end,
  }
  local designPage = d.views:Page { caption = "Design" }
  local codePage = d.views:Page { caption = "Code" }
  local pageH = splitH - 25
  d.area = designPage:Scroll { left = 0, top = 0, width = CW, height = pageH, grow = true, color = "#8a8f99" }
  d.surface = surface.new(d.area, d)

  -- VB6's two boxes over the code: an object, then one of its events.
  d.objectBox = codePage:ComboBox {
    left = 4, top = 4, width = CW / 2 - 6,
    onChange = function(self) d:pickObject(self.selected) end,
  }
  d.eventBox = codePage:ComboBox {
    left = CW / 2 + 2, top = 4, width = CW / 2 - 6,
    onChange = function(self) d:pickEvent(self.selected) end,
  }
  d.codeBox = codePage:TextBox {
    multiLine = true, lineNumbers = true, syntax = "lua", acceptsTab = true,
    left = 0, top = 34, width = CW, height = pageH - 34, grow = true,
    onChange = function() d:markCodeDirty() end,
  }

  rightPane:Label { caption = "Properties", left = 6, top = 2, width = RIGHT - 12, height = 20 }
  d.props = props.new(rightPane, d, { left = 6, top = 24, width = RIGHT - 12, height = splitH - 30 })

  d.output = win:ListBox {
    left = 0, top = MENU + splitH, width = W, height = OUTPUT, font = "mono",
    onDoubleClick = function(self) d:jumpToError(self.text) end,
  }
  d.statusLine = win:Label { left = 6, top = H - STATUS, width = W - 12, height = STATUS }

  ---------------------------------------------------------------- the designer's side of things

  function d:tool() return self.toolbox:tool() end
  function d:resetTool() self.toolbox:reset(); self.toolbox:redraw() end
  function d:status(text) self.statusLine.caption = text or "" end

  -- ask is every question the designer puts. answer, when set, answers
  -- them all without asking, which is how tests keep a question from
  -- waiting for someone to click.
  function d:ask(message, buttons, title)
    if self.answer then
      self.asked = message
      return self.answer
    end
    return gui.msgbox(message, buttons, title)
  end

  function d:selected(node, count)
    self:offerRename()
    self.selection = node
    self.namedAs = node.name
    self.lastTag = nil -- typing into another control's properties is another edit
    self.props:show(node)
    if count and count > 1 then
      self:status(("%d controls selected: %s, with the solid handles, is the one the others line up with"):format(count, node.name or node.kind))
    end
  end

  -- doubleClicked is a double-click on the form or a control: VB6 opened
  -- the code at its default event, writing the handler if there was none.
  function d:doubleClicked(node)
    local event = code.defaultEvent(node.kind)
    if event then
      self:openHandler(node, event)
    else
      self:showCode()
    end
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

  function d:markCodeDirty()
    if not self.codeDirty then
      self.codeDirty = true
      self:retitle()
    end
  end

  -- setProp changes a property from the grid: of the control shown there,
  -- and of the others selected with it that have the property, as VB6 did.
  -- A name is the one control's own.
  function d:setProp(node, prop, value)
    self:checkpoint("prop:" .. tostring(node) .. ":" .. prop)
    local ok, err = self.surface:setProp(node, prop, value)
    if ok and prop ~= "name" and node ~= self.doc then
      for _, other in ipairs(self.surface:selectedNodes()) do
        if other ~= node and model.kinds()[other.kind].props[prop] then
          self.surface:setProp(other, prop, value)
        end
      end
    end
    return ok, err
  end

  ---------------------------------------------------------------- undo, and the clipboard

  local function selectedNames()
    local names = {}
    for _, n in ipairs(d.surface:selectedNodes()) do names[#names + 1] = n.name end
    return names
  end

  -- checkpoint keeps the layout as it is before a change, to undo to. A
  -- tag joins changes into one: typing a caption a letter at a time is one
  -- edit, as is a run of arrow keys.
  function d:checkpoint(tag)
    if not self.doc or (tag and tag == self.lastTag) then return end
    self.lastTag = tag
    self.undoStack[#self.undoStack + 1] = { doc = model.copy(self.doc), keep = selectedNames() }
    if #self.undoStack > 200 then table.remove(self.undoStack, 1) end
    self.redoStack = {}
  end

  function d:restore(from, to)
    local snap = table.remove(from)
    if not snap then
      self:status(from == self.undoStack and "Nothing to undo" or "Nothing to redo")
      return
    end
    to[#to + 1] = { doc = model.copy(self.doc), keep = selectedNames() }
    self.doc = snap.doc
    self.lastTag = nil
    self.props:list(self.doc)
    self.surface:load(self.doc, snap.keep)
    self:markDirty()
  end

  function d:undo() self:restore(self.undoStack, self.redoStack) end
  function d:redo() self:restore(self.redoStack, self.undoStack) end

  -- The clipboard is the designer's own: what is copied from one form can
  -- be pasted into another, and the system's clipboard is left alone.
  function d:copy()
    local nodes = self.surface:selectedNodes()
    if #nodes == 0 then return end
    self.clip = model.copyText(nodes)
    self.pastes = 0
    self:status(("Copied %d control%s"):format(#nodes, #nodes == 1 and "" or "s"))
  end

  function d:cut()
    self:copy()
    self.surface:deleteSelected()
  end

  function d:paste()
    if not self.clip or not self.doc then return end
    self.pastes = (self.pastes or 0) + 1
    local nodes = model.pasteNodes(self.clip, self.doc, 8 * self.pastes)
    if not nodes then return end
    self:checkpoint()
    self.surface:add(nodes)
  end

  function d:duplicate()
    local nodes = self.surface:selectedNodes()
    if #nodes == 0 then return end
    self:checkpoint()
    self.surface:add(model.pasteNodes(model.copyText(nodes), self.doc, 8))
  end

  ---------------------------------------------------------------- arranging, tab order, menus

  function d:setTabOrder(on)
    if on == nil then on = not self.surface.tabMode end
    self.surface:tabOrder(on)
  end

  -- editMenu opens the Menu Editor on the form's menu, which it makes, or
  -- takes away when its last item goes.
  function d:editMenu()
    if not self.doc then return end
    local node
    for _, n in ipairs(self.doc) do
      if n.kind == "Menu" then node = n end
    end
    local items = menueditor.edit(node and node.items or {}, self.menuEditorHooks)
    if not items then return end
    self:checkpoint()
    if node and #items == 0 then
      self.surface:selectNode(node)
      self.surface:deleteSelected()
    elseif node then
      self.surface:setProp(node, "items", items)
    elseif #items > 0 then
      self.surface:add({ { kind = "Menu", name = model.uniqueName(self.doc, "Menu"), items = items } })
    end
  end

  ---------------------------------------------------------------- the project

  function d:showStartup()
    local name = self.dir and project.startup(self.dir)
    self.startupLine.caption = name and ("Starts with " .. name) or ""
  end

  function d:setStartup()
    if not self.formName then return end
    local ok, why = project.setStartup(self.dir, self.formName)
    if ok then
      self:showStartup()
      self:status(self.formName .. " is shown first now")
    else
      self:ask(why, "ok", "Set as Startup Form")
    end
  end

  -- makeExe packs the project into one executable, with tlua fuse.
  function d:makeExe()
    if not self.dir then return end
    local name = self.dir:match("([^/\\]+)[/\\]?$") or "app"
    local out = gui.savefile { title = "Make EXE", file = name, dir = self.dir:match("^(.*)[/\\]") }
    if out then self:makeExeTo(out) end
  end

  function d:makeExeTo(out)
    self:save()
    self.output.items = {}
    self:print("> tlua fuse -o " .. out .. " .")
    gui.spawn {
      gui.interpreter, "fuse", "-o", out, self.dir,
      onOutput = function(line) d:print(line) end,
      onExit = function(status)
        d:print(status == 0 and ("> made " .. out) or ("> fuse exited with " .. status))
        d:status(status == 0 and ("Made " .. out) or "Make EXE failed")
      end,
    }
  end

  function d:retitle()
    local parts = { "tlua design" }
    if self.dir then parts[#parts + 1] = self.dir:match("([^/\\]+)[/\\]?$") end
    if self.formName then
      parts[#parts + 1] = self.formName .. ((self.dirty or self.codeDirty) and " *" or "")
    end
    self.win.caption = table.concat(parts, " - ")
  end

  ---------------------------------------------------------------- files

  -- keepOrLose asks what to do with unsaved changes; false means stay put.
  function d:keepOrLose()
    if not (self.dirty or self.codeDirty) then return true end
    local answer = self:ask("Save the changes to " .. self.formName .. "?", "yesnocancel", "tlua design")
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
    self:showStartup()
    self:retitle()
    if forms[1] then self:openForm(forms[1]) end
  end

  function d:openForm(name)
    if name == "" or name == self.formName then return end
    if not self:keepOrLose() then return end
    local doc, err = project.read(self.dir, name)
    if not doc then
      self:ask(tostring(err), "ok", "tlua design")
      return
    end
    self.doc, self.formName, self.dirty = doc, name, false
    self.selection, self.namedAs = nil, nil
    self.undoStack, self.redoStack, self.lastTag = {}, {}, nil
    self.props:list(doc)
    self.surface:load(doc)
    self:loadCode()
    self.surface:select(nil)
    self.tree.path = name
    self:retitle()
    self:status("")
  end

  function d:save()
    if not self.doc then return end
    self:offerRename()
    project.save(self.dir, self.formName, self.doc)
    self.dirty = false
    if self.codeDirty then self:saveCode() end
    self:retitle()
    self:status("Saved " .. self.formName)
  end

  ---------------------------------------------------------------- code

  function d:codePath()
    return self.dir .. "/forms/" .. self.formName .. ".lua"
  end

  local function stamp(path)
    return lfs.attributes(path, "modification")
  end

  -- loadCode reads the form's code into the code box.
  function d:loadCode()
    local path = self:codePath()
    if not project.exists(path) then project.addForm(self.dir, self.formName) end
    local f = io.open(path, "r")
    self.codeBox.text = f and f:read("*a") or ""
    if f then f:close() end
    self.codeStamp = stamp(path)
    self.codeDirty = false
    self:retitle()
  end

  function d:saveCode()
    local path = self:codePath()
    if stamp(path) ~= self.codeStamp then
      local answer = self:ask(self.formName .. ".lua has changed on disk since it was opened here. Write over it?", "yesno", "tlua design")
      if answer ~= "yes" then return end
    end
    local f = assert(io.open(path, "w"))
    f:write(self.codeBox.text)
    f:close()
    self.codeStamp = stamp(path)
    self.codeDirty = false
  end

  -- syncCode reads the code again if another editor changed it and there
  -- is nothing here to lose.
  function d:syncCode()
    if self.formName and not self.codeDirty and stamp(self:codePath()) ~= self.codeStamp then
      self:loadCode()
      self:status(self.formName .. ".lua changed on disk, and was read again")
    end
  end

  -- showCode switches to the code, with the object box on the selection.
  function d:showCode()
    if not self.doc then return end
    self:offerRename()
    self:syncCode()
    self.views.selected = 2
    self:fillObjects()
    self.codeBox:focus()
  end

  function d:showDesign()
    self.views.selected = 1
  end

  -- fillObjects lists the form and its controls in the object box, and the
  -- events of the one selected in the event box.
  function d:fillObjects()
    self.codeNodes = { self.doc }
    local items = { "(" .. self.formName .. ")" }
    for _, node in ipairs(self.doc) do
      self.codeNodes[#self.codeNodes + 1] = node
      items[#items + 1] = node.name
    end
    self.objectBox.items = items
    local at = 1
    for i, n in ipairs(self.codeNodes) do
      if n == self.selection then at = i end
    end
    self.objectBox.selected = at
    self:fillEvents(self.codeNodes[at])
  end

  function d:fillEvents(node)
    self.eventNode = node
    local events = model.kinds()[node.kind].events
    local text, var = self.codeBox.text, code.formVar(self.codeBox.text)
    local items = {}
    self.eventNames = {}
    for _, e in ipairs(events) do
      self.eventNames[#self.eventNames + 1] = e
      -- A handler already written is marked, as VB6 showed it in bold.
      local written = code.findHandler(text, var, node ~= self.doc and node.name or nil, e)
      items[#items + 1] = (written and "• " or "  ") .. e
    end
    self.eventBox.items = items
    self.eventBox.selected = 0
  end

  function d:pickObject(i)
    local node = self.codeNodes and self.codeNodes[i]
    if node then self:fillEvents(node) end
  end

  function d:pickEvent(i)
    local event = self.eventNames and self.eventNames[i]
    if event and self.eventNode then self:openHandler(self.eventNode, event) end
  end

  -- openHandler shows a handler in the code, writing an empty one first if
  -- there is none.
  function d:openHandler(node, event)
    self:offerRename()
    self:syncCode()
    local text = self.codeBox.text
    local var = code.formVar(text)
    local name = node ~= self.doc and node.name or nil
    local line = code.findHandler(text, var, name, event)
    local indent = 0
    if line then
      line = line + 1
    else
      text, line = code.addHandler(text, var, name, event, node.kind)
      self.codeBox.text = text
      self:markCodeDirty()
      indent = 2
    end
    self.views.selected = 2
    self:fillObjects()
    for i, e in ipairs(self.eventNames) do
      if e == event then self.eventBox.selected = i end
    end
    self.codeBox.line = line
    self.codeBox.cursor = self.codeBox.cursor + indent
    self.codeBox:focus()
  end

  -- offerRename asks to rename a control in the code too, after its name
  -- changed in the properties: once, when the change is done with.
  function d:offerRename()
    local node, old = self.selection, self.namedAs
    if not node or node == self.doc or not old or node.name == old then return end
    self.namedAs = node.name
    local text = self.codeBox.text
    local var = code.formVar(text)
    local n = code.countRefs(text, var, old)
    if n == 0 then return end
    local question = ("Rename %s.%s to %s.%s in the code? It is there %d time%s."):format(
      var, old, var, node.name, n, n == 1 and "" or "s")
    if self:ask(question, "yesno", "Rename") == "yes" then
      self.codeBox.text = code.renameRefs(text, var, old, node.name)
      self:markCodeDirty()
    end
  end

  -- find looks for text in the code, from the cursor on and round again.
  function d:find(again)
    self:showCode()
    if not again or not self.lastFind then
      local what = gui.inputbox("Find:", "Find", self.lastFind or "")
      if not what or what == "" then return end
      self.lastFind = what
    end
    local text = self.codeBox.text
    local from = self.codeBox.cursor + 1
    local i, j = text:find(self.lastFind, from, true)
    if not i then i, j = text:find(self.lastFind, 1, true) end
    if i then
      self.codeBox:select(i, j)
      self:status("")
    else
      self:status(self.lastFind .. " is not in the code")
    end
    self.codeBox:focus()
  end

  function d:goToLine()
    self:showCode()
    local n = tonumber(gui.inputbox("Line:", "Go to Line", tostring(self.codeBox.line)) or "")
    if n then self.codeBox.line = n end
    self.codeBox:focus()
  end

  -- jumpToError opens the code where an error message says it happened.
  function d:jumpToError(message)
    local form, line = code.errorAt(message or "")
    if not form then return false end
    if form ~= self.formName then
      self:openForm(form)
      if form ~= self.formName then return false end
    end
    self:showCode()
    self.codeBox.line = line
    self.codeBox:focus()
    return true
  end

  -- openInEditor hands the code to whatever opens .lua files here.
  function d:openInEditor()
    if not self.formName then return end
    self:save()
    local path = self:codePath()
    local opener
    if package.config:sub(1, 1) == "\\" then
      opener = { "cmd", "/c", "start", "", path }
    else
      local uname = io.popen("uname -s")
      local os_name = uname and uname:read("*l") or ""
      if uname then uname:close() end
      opener = { os_name == "Darwin" and "open" or "xdg-open", path }
    end
    gui.spawn(opener)
    self:status("Opened " .. self.formName .. ".lua elsewhere; it is read again here when it changes")
  end

  function d:addForm()
    local n = 1
    while project.exists(project.layoutPath(self.dir, "Form" .. n)) do n = n + 1 end
    local name = gui.inputbox("A name for the new form:", "Add Form", "Form" .. n)
    if not name or name == "" then return end
    if not name:match("^[%a_][%w_]*$") then
      self:ask(name .. " is not a name: use letters, digits and _.", "ok", "Add Form")
      return
    end
    if project.exists(project.layoutPath(self.dir, name)) then
      self:ask("There is a form called " .. name .. " already.", "ok", "Add Form")
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
      self:ask("There are no forms in " .. dir .. ".", "ok", "Open Project")
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
      onExit = function(status)
        d:print(status == 0 and "> finished" or ("> exited with " .. status))
        d:status("")
        if status ~= 0 then
          -- To the first error in the program's own forms, if there is one.
          for _, line in ipairs(d.output.items) do
            if d:jumpToError(line) then break end
          end
        end
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
      { "&Make EXE...", function() d:makeExe() end },
      "-",
      { "&Quit", function() win:close() end, shortcut = "Cmd+Q" },
    } },
    { "&Edit", {
      { "&Undo", function() d:undo() end, shortcut = "Cmd+Z" },
      { "&Redo", function() d:redo() end, shortcut = "Cmd+Shift+Z" },
      "-",
      { "Cu&t", function() d:cut() end, shortcut = "Cmd+X" },
      { "&Copy", function() d:copy() end, shortcut = "Cmd+C" },
      { "&Paste", function() d:paste() end, shortcut = "Cmd+V" },
      { "D&uplicate", function() d:duplicate() end, shortcut = "Cmd+D" },
      { "&Delete", function() d.surface:deleteSelected() end },
      { "Select &All", function() d.surface:selectAll() end, shortcut = "Cmd+A" },
      "-",
      { "&Find...", function() d:find(false) end, shortcut = "Cmd+F" },
      { "Find &Next", function() d:find(true) end, shortcut = "Cmd+G" },
      { "&Go to Line...", function() d:goToLine() end, shortcut = "Cmd+L" },
    } },
    { "&View", {
      { "&Code", function() d:showCode() end, shortcut = "F7" },
      { "&Object", function() d:showDesign() end, shortcut = "Shift+F7" },
      "-",
      { "Show &Grid", function(_, on) d.gridShown = on; d.surface.grid:redraw() end, checked = false },
      { "&Snap to Grid", function(_, on) d.snap = on end, checked = false },
      "-",
      { "Open Code in &Editor", function() d:openInEditor() end },
    } },
    { "F&ormat", {
      { "&Align", {
        { "&Lefts", function() d.surface:arrange("lefts") end },
        { "&Centers", function() d.surface:arrange("centers") end },
        { "&Rights", function() d.surface:arrange("rights") end },
        "-",
        { "&Tops", function() d.surface:arrange("tops") end },
        { "&Middles", function() d.surface:arrange("middles") end },
        { "&Bottoms", function() d.surface:arrange("bottoms") end },
      } },
      { "&Make Same Size", {
        { "&Width", function() d.surface:arrange("width") end },
        { "&Height", function() d.surface:arrange("height") end },
        { "&Both", function() d.surface:arrange("size") end },
      } },
      { "&Center in Form", {
        { "&Horizontally", function() d.surface:arrange("centerH") end },
        { "&Vertically", function() d.surface:arrange("centerV") end },
      } },
      { "&Spacing", {
        { "Make &Horizontal Spacing Equal", function() d.surface:arrange("spaceH") end },
        { "Make &Vertical Spacing Equal", function() d.surface:arrange("spaceV") end },
      } },
      "-",
      { "Bring to &Front", function() d.surface:toFront() end },
      { "Send to &Back", function() d.surface:toBack() end },
      "-",
      { "&Tab Order", function() d:setTabOrder() end },
    } },
    { "&Project", {
      { "&Add Form...", function() d:addForm() end },
      { "Set as &Startup Form", function() d:setStartup() end },
    } },
    { "&Tools", {
      { "&Menu Editor...", function() d:editMenu() end, shortcut = "Cmd+E" },
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
