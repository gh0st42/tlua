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
local assist = require "design.assist"

local M = {}

local W, H = 1180, 760
local MENU, OUTPUT, STATUS = 25, 110, 24
local LEFT, RIGHT = 210, 300

-- startDialog asks, before the designer opens, whether dir is the folder
-- the user meant: "here" to open the project in it (or start one there),
-- "choose" to pick another, "none" for no project for now.
-- shortPath is a folder as it fits in width at the dialog's font: the
-- home folder as ~, and what is still too long cut out of the middle.
local function shortPath(dir, width)
  local home = os.getenv("HOME")
  if home and home ~= "" and dir:sub(1, #home) == home then dir = "~" .. dir:sub(#home + 1) end
  local function fits(s) return (gui.measure(s, "mono", 12)) <= width end
  local ok, fit = pcall(fits, dir)
  if not ok or fit then return dir end
  local keep = #dir
  while keep > 8 do
    keep = keep - 1
    local head = math.floor(keep / 3)
    local short = dir:sub(1, head) .. "…" .. dir:sub(#dir - (keep - head) + 1)
    if fits(short) then return short end
  end
  return dir
end

local function startDialog(dir, hasForms)
  local choice = "none"
  local f = gui.Form { caption = "tlua design", width = 480, height = 150 }
  f:Label {
    left = 16, top = 14, width = 448, height = 24,
    caption = hasForms and "The current folder has a project:" or "The current folder has no project yet:",
  }
  f:Label { left = 16, top = 40, width = 448, height = 24, caption = shortPath(dir, 448), font = "mono", fontSize = 12, textColor = "#404040" }
  local function pick(c) return function() choice = c; f:close() end end
  f:Button { caption = hasForms and "Open It" or "Start One Here", left = 16, top = 100, width = 140, default = true, onClick = pick("here") }
  f:Button { caption = "Choose a Folder...", left = 166, top = 100, width = 160, onClick = pick("choose") }
  f:Button { caption = "No Project", left = 336, top = 100, width = 128, onClick = pick("none") }
  function f:onKey(key)
    if key == "Escape" then self:close() return true end
  end
  f:showModal()
  return choice
end

-- pickProject is the folder the designer opens when none was named: the
-- current one, or one the user picks, made into a project when it has
-- none and the user says so; nil for none. opts.answerStart and
-- opts.answerDir answer its questions for tests, and then it asks once.
local function pickProject(dir, opts)
  while true do
    local hasForms = project.forms(dir)[1] ~= nil
    local choice = opts.answerStart or startDialog(dir, hasForms)
    if choice == "here" then
      if not hasForms then project.create(dir) end
      return dir
    elseif choice == "choose" then
      local other = opts.answerDir
      if other == nil then other = gui.choosedir { title = "Open a project, or a folder for a new one" } end
      if other then
        if project.forms(other)[1] then return other end
        if opts.create or gui.msgbox("There is no project in " .. other .. ". Start one there?", "yesno", "tlua design") == "yes" then
          project.create(other)
          return other
        end
      end
      if opts.answerStart then return nil end
      -- Chooser cancelled, or no project wanted there: ask again.
    else
      return nil
    end
  end
end

-- start opens the designer on the project in opts.dir: made there first if
-- it has no forms and opts.create says so, or asked about otherwise. With
-- opts.pick, the folder was not named, and the user picks it first. It
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
  -- The toolbox scrolls, for a project with many controls of its own.
  local tools = leftPane:Scroll { left = 6, top = 178, width = LEFT - 12, height = splitH - 184, grow = true, color = "#ffffff" }
  d.toolbox = toolbox.new(tools, {
    left = 0, top = 0, width = LEFT - 12 - 16, height = (#model.tools + 1) * toolbox.ROW,
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
  -- Completion and help from a language server, laid over the code.
  d.assist = assist.new(d, d.codeBox, codePage)

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

  -- prompt is every line of text the designer asks for; answerText, when
  -- set, is the answer.
  function d:prompt(message, title, default)
    if self.answerText ~= nil then
      self.asked = message
      return self.answerText
    end
    return gui.inputbox(message, title, default)
  end

  -- chooseColor and chooseFile are the colours and files the designer
  -- asks for; answerColor and answerFile, when set, are the answers, and
  -- false is Cancel.
  function d:chooseColor(title, color)
    if self.answerColor ~= nil then
      self.asked = title
      return self.answerColor or nil
    end
    return gui.choosecolor { title = title, color = color }
  end

  function d:chooseFile(title, dir)
    if self.answerFile ~= nil then
      self.asked = title
      return self.answerFile or nil
    end
    return gui.openfile {
      title = title, dir = dir,
      filter = "Images\t*.{png,jpg,jpeg,gif,bmp,svg}\nAll files\t*",
    }
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

  -- popup is every menu the designer pops up; answerMenu, when set, picks
  -- the item with that caption without showing it, for tests.
  function d:popup(items)
    if self.answerMenu ~= nil then
      self.menuAsked = items
      for _, item in ipairs(items) do
        if type(item) == "table" and type(item[2]) == "function"
            and item[1]:gsub("&", "") == self.answerMenu and item.enabled ~= false then
          item[2]()
        end
      end
      return
    end
    gui.popup(items)
  end

  -- contextMenu is a right click on the form or a control: what can be
  -- done to what is selected, as VB6's had it.
  function d:contextMenu(node)
    local paste = { "&Paste", function() d:paste() end, shortcut = "Cmd+V", enabled = self.clip ~= nil }
    local code = { "View &Code", function() d:doubleClicked(node) end }
    if node == self.doc then
      self:popup {
        paste,
        { "Select &All", function() d.surface:selectAll() end },
        "-",
        code,
        { "&Menu Editor...", function() d:editMenu() end },
      }
      return
    end
    self:popup {
      { "Cu&t", function() d:cut() end, shortcut = "Cmd+X" },
      { "&Copy", function() d:copy() end, shortcut = "Cmd+C" },
      paste,
      { "D&uplicate", function() d:duplicate() end, shortcut = "Cmd+D" },
      { "&Delete", function() d.surface:deleteSelected() end },
      "-",
      { "Bring to &Front", function() d.surface:toFront() end },
      { "Send to &Back", function() d.surface:toBack() end },
      "-",
      code,
    }
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
    -- The other controls selected with it take the value too; not when it
    -- is no longer selected itself, as with an edit kept as the selection
    -- moves on.
    local selected = self.surface:selectedNodes()
    local among = false
    for _, n in ipairs(selected) do among = among or n == node end
    if ok and among and prop ~= "name" and node ~= self.doc then
      for _, other in ipairs(selected) do
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
    self.surface:add(nodes, self.surface:pasteHolder())
  end

  function d:duplicate()
    local list = self.surface:outermost()
    if #list == 0 then return end
    local nodes = {}
    for _, e in ipairs(list) do nodes[#nodes + 1] = e.node end
    self:checkpoint()
    self.surface:add(model.pasteNodes(model.copyText(nodes), self.doc, 8), self.surface:sameHolder() or self.doc)
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

  -- pack saves the project and runs a tlua subcommand that packs it into
  -- out, saying how it went in the output pane.
  function d:pack(command, out, what)
    self:save()
    self.output.items = {}
    self:print("> tlua " .. command .. " -o " .. out .. " .")
    gui.spawn {
      gui.interpreter, command, "-o", out, self.dir,
      onOutput = function(line) d:print(line) end,
      onExit = function(status)
        d:print(status == 0 and ("> made " .. out) or ("> " .. command .. " exited with " .. status))
        d:status(status == 0 and ("Made " .. out) or (what .. " failed"))
      end,
    }
  end

  -- packTo asks where to write what the project is packed into, beside the
  -- project's folder to begin with.
  function d:packTo(title, ext)
    if not self.dir then return end
    local name = self.dir:match("([^/\\]+)[/\\]?$") or "app"
    return gui.savefile {
      title = title, file = name .. ext, dir = self.dir:match("^(.*)[/\\]"),
      filter = ext ~= "" and ("Bundle\t*" .. ext .. "\nAll\t*") or nil,
    }
  end

  -- makeExe packs the project into one executable, with tlua fuse.
  function d:makeExe()
    local out = self:packTo("Make EXE", "")
    if out then self:makeExeTo(out) end
  end

  function d:makeExeTo(out)
    self:pack("fuse", out, "Make EXE")
  end

  -- exportBundle packs the project into a bundle, with tlua bundle: one
  -- file that "tlua app.ztl" runs on any platform, with no executable made
  -- for each.
  function d:exportBundle()
    local out = self:packTo("Export Bundle", ".ztl")
    if out then self:exportBundleTo(out) end
  end

  function d:exportBundleTo(out)
    self:pack("bundle", out, "Export Bundle")
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

  -- loadControls defines the project's own controls, from controls/, and
  -- puts them in the toolbox. A control that does not load says why below.
  function d:loadControls()
    for _, name in ipairs(project.controls(self.dir)) do
      local ok, err = pcall(require, "controls." .. name)
      if not ok then self:print("controls/" .. name .. ".lua: " .. tostring(err)) end
    end
    model.refreshKinds()
    self.toolbox:setTools(model.definedKinds())
  end

  function d:newControl()
    local n = 1
    while project.exists(self.dir .. "/controls/Control" .. n .. ".lua") do n = n + 1 end
    local name = self:prompt("A name for the new control (a capital first):", "New Control", "Control" .. n)
    if not name or name == "" then return end
    if not name:match("^%u[%w_]*$") or model.kinds()[name] then
      self:ask(name .. " cannot be a control's name: start with a capital, and pick one that is not taken.", "ok", "New Control")
      return
    end
    if not project.addControl(self.dir, name) then
      self:ask("There is a controls/" .. name .. ".lua already.", "ok", "New Control")
      return
    end
    self:loadControls()
    self:status("controls/" .. name .. ".lua is yours to change; it is in the toolbox")
  end

  function d:openProject(dir)
    self.dir = dir
    -- The project's modules, its own controls among them, are found from
    -- its folder, as they are when it runs.
    package.path = dir .. "/?.lua;" .. package.path
    -- Image paths in a layout are relative to the form's code, which lives
    -- in forms/; the designer looks for them from there too.
    lfs.chdir(dir .. "/forms")
    self.assist:start(dir)
    local forms = project.forms(dir)
    self.tree.items = forms
    self:loadControls()
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
    self.assist:closeList()
    self.assist:hideTip()
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
    self.assist:closeList()
    self.assist:hideTip()
  end

  -- fillObjects lists the form and its controls in the object box, and the
  -- events of the one selected in the event box.
  function d:fillObjects()
    self.codeNodes = { self.doc }
    local items = { "(" .. self.formName .. ")" }
    model.walk(self.doc, function(node)
      if node.name and node.kind ~= "Page" then
        self.codeNodes[#self.codeNodes + 1] = node
        items[#items + 1] = node.name
      end
    end)
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
      local what = self:prompt("Find:", "Find", self.lastFind or "")
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
    local n = tonumber(self:prompt("Line:", "Go to Line", tostring(self.codeBox.line)) or "")
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
    local name = self:prompt("A name for the new form:", "Add Form", "Form" .. n)
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
      { "&Export Bundle...", function() d:exportBundle() end },
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
      { "Ta&bs", {
        { "&Add Page", function() d.surface:addPage() end },
        { "&Remove Page", function() d.surface:removePage() end },
      } },
      "-",
      { "&Tab Order", function() d:setTabOrder() end },
    } },
    { "&Project", {
      { "&Add Form...", function() d:addForm() end },
      { "Set as &Startup Form", function() d:setStartup() end },
      "-",
      { "&New Control...", function() d:newControl() end },
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
    d.assist:stop()
  end

  ---------------------------------------------------------------- the project

  local dir = opts.dir
  if dir and opts.pick then
    dir = pickProject(dir, opts)
  elseif dir and not project.forms(dir)[1] then
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
