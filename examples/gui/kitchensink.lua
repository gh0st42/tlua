#!/usr/bin/env tlua

-- Everything the gui module offers, in one window: every control, a menu,
-- tabs and frames, a tree and a table, a canvas to draw on, the dialogs and
-- file choosers, the clipboard, drag and drop, timers, key presses,
-- resizing, colours and fonts, a modal form of our own, and a close guard.

local gui = bootgui()

local form = gui.Form {
  caption = "Kitchen sink",
  width = 640,
  height = 516,
  resizable = true,
}

local status = form:Label {
  caption = "Ready.",
  left = 8, top = 481, width = 516, height = 28,
}
local function say(fmt, ...)
  status.caption = string.format(fmt, ...)
end

local tabs = form:Tabs { left = 8, top = 33, width = 624, height = 440, grow = true }

----------------------------------------------------------------------------
-- Text: single-line, password, read-only and multi-line text boxes.

local text = tabs:Page { caption = "Text" }

text:Label { caption = "Name", left = 12, top = 12, width = 80 }
local name = text:TextBox { left = 92, top = 12, width = 220 }
local greeting = text:Label { caption = "Type a name.", left = 324, top = 12, width = 288 }
function name:onChange()
  greeting.caption = self.text == "" and "Type a name." or "Hello, " .. self.text .. "!"
end

text:Label { caption = "Password", left = 12, top = 48, width = 80 }
local password = text:TextBox { left = 92, top = 48, width = 220, password = true }
local strength = text:TextBox {
  left = 324, top = 48, width = 288, readOnly = true, text = "(read-only) nothing typed yet",
}
function password:onChange()
  local n = #self.text
  strength.text = string.format("(read-only) %d characters: %s", n,
    n == 0 and "nothing typed yet" or n < 8 and "too short" or "long enough")
end

local notes = text:TextBox {
  text = "Type a few lines here.\nThe box grows with the window.\n",
  multiLine = true,
  left = 12, top = 88, width = 600, height = 280,
  grow = true,
}
local saved = notes.text
local counter = text:Label { left = 12, top = 376, width = 600, height = 24 }
local function count()
  local _, lines = notes.text:gsub("\n", "")
  counter.caption = string.format("%d characters, %d lines", #notes.text, lines)
end
function notes:onChange() count() end

----------------------------------------------------------------------------
-- Choices: check boxes, radio buttons in a frame, a combo box, a list.

local choices = tabs:Page { caption = "Choices" }

choices:CheckBox {
  caption = "Monospaced notes", left = 12, top = 12, width = 200,
  onChange = function(self) notes.font = self.checked and "mono" or "sans" end,
}
local showCounter = choices:CheckBox {
  caption = "Show the counter", left = 12, top = 44, width = 200, checked = true,
  onChange = function(self) counter.visible = self.checked end,
}

-- Radio buttons are one group per parent, which is what the frame is for.
local theme = choices:Frame { caption = "Notes colour", left = 12, top = 84, width = 200, height = 124 }
for i, t in ipairs { { "White", "#ffffff" }, { "Warm", "#fff4dc" }, { "Cool", "#e4f0ff" } } do
  theme:RadioButton {
    caption = t[1], left = 12, top = 24 + (i - 1) * 30, width = 170, checked = i == 1,
    onChange = function(self)
      if self.checked then notes.color = t[2] end
    end,
  }
end

choices:Label { caption = "Notes font size", left = 240, top = 12, width = 120 }
choices:ComboBox {
  left = 360, top = 12, width = 80,
  items = { "12", "14", "18", "24" }, selected = 2,
  onChange = function(self) notes.fontSize = tonumber(self.text) end,
}

local fruit = choices:ListBox {
  left = 240, top = 52, width = 200, height = 156,
  items = { "Apple", "Banana", "Cherry", "Damson" },
}
function fruit:onChange() say("Picked %s (item %d).", self.text, self.selected) end
function fruit:onDoubleClick() gui.msgbox("You double-clicked " .. self.text .. ".", "ok", "List") end
-- Dragging a fruit out carries its name: drop it on the Draw tab, or in
-- any other program that takes text.
function fruit:onDrag() return self.text end

choices:Button {
  caption = "Add a fruit...", left = 452, top = 52, width = 160,
  onClick = function()
    -- A dialog from inside a handler: it waits for itself, not the form.
    local new = gui.inputbox("Name a fruit:", "Add a fruit")
    if new and new ~= "" then
      local items = fruit.items
      items[#items + 1] = new
      fruit.items = items -- assigning the list again redraws it
      fruit.selected = #items
    end
  end,
}
choices:Button {
  caption = "Remove it", left = 452, top = 88, width = 160,
  onClick = function()
    local items, i = fruit.items, fruit.selected
    if i == 0 then return say("Pick a fruit first.") end
    table.remove(items, i)
    fruit.items = items
  end,
}
choices:Label {
  caption = "Double-click a fruit, or add your own.",
  left = 240, top = 216, width = 372,
}

----------------------------------------------------------------------------
-- Numbers: sliders, spinners, a progress bar and a timer.

local numbers = tabs:Page { caption = "Numbers" }

numbers:Label { caption = "Slider", left = 12, top = 12, width = 80 }
local slider = numbers:Slider { left = 92, top = 12, width = 320, value = 25 }
local spinner = numbers:Spinner { left = 424, top = 12, width = 80, value = 25 }
local progress = numbers:ProgressBar { left = 92, top = 52, width = 412, height = 24 }

local function setAll(v)
  slider.value, spinner.value, progress.value = v, v, v
  progress.caption = string.format("%d%%", v)
end
slider.onChange = function(self) setAll(self.value) end
spinner.onChange = function(self) setAll(self.value) end
setAll(25)

local timer
local animate = numbers:Button { caption = "Animate", left = 92, top = 92, width = 120 }
function animate:onClick()
  if timer then
    timer:stop()
    timer = nil
    self.caption = "Animate"
    return
  end
  self.caption = "Stop"
  timer = gui.every(0.03, function()
    setAll((progress.value + 1) % 101)
  end)
end

numbers:Label { caption = "Step 0.5", left = 12, top = 132, width = 80 }
numbers:Spinner { left = 92, top = 132, width = 80, min = 0, max = 5, step = 0.5, value = 2.5 }

local level = numbers:Label { caption = "50", left = 556, top = 12, width = 50, align = "center" }
numbers:Slider {
  vertical = true, left = 566, top = 44, width = 30, height = 200, value = 50,
  onChange = function(self) level.caption = string.format("%d", self.value) end,
}

----------------------------------------------------------------------------
-- Dialogs: message boxes, input, file choosers, and a modal form.

local dialogs = tabs:Page { caption = "Dialogs" }
local answer = dialogs:Label { caption = "Each button reports here.", left = 12, top = 216, width = 600 }
local function report(what, result)
  if type(result) == "table" then result = table.concat(result, ", ") end
  answer.caption = string.format("%s returned %s", what, result == nil and "nil" or "\"" .. tostring(result) .. "\"")
end

local buttons = {
  { "Message", function() return gui.msgbox("Just so you know.", "ok", "Message") end },
  { "Yes / No", function() return gui.msgbox("Is this useful?", "yesno", "Question") end },
  { "Yes / No / Cancel", function() return gui.msgbox("Save the notes?", "yesnocancel", "Question") end },
  { "Retry / Cancel", function() return gui.msgbox("That did not work.", "retrycancel", "Problem") end },
  { "Ask for text", function() return gui.inputbox("What is your favourite colour?", "Input", "blue") end },
  { "Open a file", function() return gui.openfile { title = "Open", filter = "Lua\t*.lua\nAll\t*" } end },
  { "Open several", function() return gui.openfile { title = "Open several", multiple = true } end },
  { "Save as", function() return gui.savefile { title = "Save as", file = "notes.txt" } end },
  { "Choose a folder", function() return gui.choosedir "Choose a folder" end },
}
for i, b in ipairs(buttons) do
  local col, row = (i - 1) % 3, math.floor((i - 1) / 3)
  dialogs:Button {
    caption = b[1], left = 12 + col * 200, top = 12 + row * 40, width = 188,
    onClick = function() report(b[1], b[2]()) end,
  }
end

-- A form of our own, shown modally: showModal waits until it is closed.
dialogs:Button {
  caption = "A modal form", left = 12, top = 132, width = 188,
  onClick = function()
    local ask = gui.Form { caption = "Rate it", width = 300, height = 120 }
    local stars = ask:Slider { left = 16, top = 16, width = 268, min = 1, max = 5, value = 3 }
    local rating
    ask:Button {
      caption = "OK", default = true, left = 184, top = 76, width = 100,
      onClick = function() rating = stars.value; ask:close() end,
    }
    ask:showModal()
    report("The modal form", rating and string.format("%d stars", rating))
  end,
}

----------------------------------------------------------------------------
-- Looks: colours, fonts, alignment, tooltips, images, disabled controls.

local looks = tabs:Page { caption = "Looks" }
looks:Label { caption = "Sans, left", left = 12, top = 12, width = 290, color = "#ffffff" }
looks:Label { caption = "Serif 18, centred", left = 12, top = 48, width = 290, font = "serif", fontSize = 18, align = "center", color = "#fff4dc" }
looks:Label { caption = "Mono, right, blue", left = 12, top = 84, width = 290, font = "mono", textColor = "blue", align = "right", color = "#e4f0ff" }
looks:Label { caption = "Hover over me", tooltip = "Tooltips work on every control.", left = 12, top = 120, width = 290 }
looks:Button { caption = "Disabled", enabled = false, left = 12, top = 156, width = 140 }
looks:Button { caption = "Coloured", color = "orange", textColor = "white", left = 162, top = 156, width = 140 }

-- Image paths are found next to the script that names them.
looks:Label { caption = "An image, as it is and fitted:", left = 320, top = 12, width = 290 }
looks:Image { file = "../pico/cellar/gfx/tiles.png", left = 320, top = 48, width = 290, height = 24 }
looks:Image { file = "../pico/cellar/gfx/tiles.png", fit = true, left = 320, top = 80, width = 290, height = 60 }

----------------------------------------------------------------------------
-- Data: a tree and a table.

local data = tabs:Page { caption = "Data" }

-- A tree's items are plain tables: a string is a leaf, {label, children} a
-- branch, and its open field is kept up to date as it is opened and closed.
local project = {
  "README.md",
  { "cmd", { { "tlua", { "main.lua" } } } },
  { "internal", { { "gui", { "module.go", "fltk.go", "tree.go" } }, { "interp", { "interp.go" } } }, open = true },
  { "examples", { { "gui", { "kitchensink.lua", "hello.lua" } } } },
}
data:Tree {
  items = project, left = 12, top = 12, width = 240, height = 360, grow = true,
  onChange = function(self) say("Tree: %s", self.path) end,
  onToggle = function(self, path, open) say("%s %s.", open and "Opened" or "Closed", path) end,
  onDoubleClick = function(self) say("Double-clicked %s.", self.text) end,
}

local languages = {
  { "Lua", "Roberto Ierusalimschy", 1993 },
  { "Go", "Griesemer, Pike, Thompson", 2009 },
  { "C", "Dennis Ritchie", 1972 },
  { "Smalltalk", "Alan Kay", 1972 },
  { "Lisp", "John McCarthy", 1958 },
}
local grid = data:Table {
  columns = { "Language", "Made by", "Year" },
  columnWidths = { 90, 200, 50 },
  rows = languages,
  left = 264, top = 12, width = 348, height = 320, grow = true,
  onChange = function(self)
    local row = languages[self.selected]
    if row then say("%s, %d.", row[1], row[3]) end
  end,
  onDoubleClick = function(self)
    local row = languages[self.selected]
    gui.msgbox(string.format("%s was made by %s in %d.", row[1], row[2], row[3]), "ok", "Table")
  end,
}
data:Button {
  caption = "Sort by year", left = 264, top = 344, width = 140,
  onClick = function()
    table.sort(languages, function(a, b) return a[3] < b[3] end)
    grid.rows = languages
  end,
}
data:Button {
  caption = "Copy the row", left = 412, top = 344, width = 140,
  onClick = function()
    local row = languages[grid.selected]
    if not row then return say("Pick a row first.") end
    gui.clipboard(table.concat(row, "\t"))
    say("Copied %s to the clipboard.", row[1])
  end,
}

----------------------------------------------------------------------------
-- Draw: a canvas to paint on, drag and drop, and the clipboard.

local draw = tabs:Page { caption = "Draw" }
local strokes, stamps = {}, {}
local ink = "#1f5fbf"

local canvas = draw:Canvas { left = 12, top = 12, width = 440, height = 360, grow = true }
function canvas:onDraw(g)
  local w, h = g:size()
  g:color("#eeeeee")
  for x = 0, w, 20 do g:line(x, 0, x, h) end
  for y = 0, h, 20 do g:line(0, y, w, y) end
  g:width(3)
  for _, s in ipairs(strokes) do
    g:color(s.color)
    for i = 3, #s, 2 do g:line(s[i - 2], s[i - 1], s[i], s[i + 1]) end
  end
  g:width(1)
  g:font("sans", 16)
  for _, t in ipairs(stamps) do
    g:color(t.color)
    g:text(t.text, t.x, t.y)
  end
  g:color("#999999")
  g:font("sans", 12)
  g:text("Drag to draw. Drop text here.", 0, h - 20, w - 6, 18, "right")
end
function canvas:onMouseDown(x, y)
  strokes[#strokes + 1] = { color = ink, x, y }
end
function canvas:onMouseDrag(x, y)
  local s = strokes[#strokes]
  s[#s + 1], s[#s + 2] = x, y
  self:redraw()
end
function canvas:onMouseMove(x, y) say("Canvas %d, %d", x, y) end
function canvas:onDrop(text)
  stamps[#stamps + 1] = { text = text:gsub("\n.*", ""), x = 20, y = 20 + 24 * #stamps, color = ink }
  self:redraw()
end

draw:Label { caption = "Ink", left = 464, top = 12, width = 40 }
draw:ComboBox {
  left = 504, top = 12, width = 108,
  items = { "Blue", "Red", "Green", "Black" }, selected = 1,
  onChange = function(self)
    ink = ({ Blue = "#1f5fbf", Red = "#d03030", Green = "#2f9f4f", Black = "#000000" })[self.text]
  end,
}
draw:Button {
  caption = "Clear", left = 464, top = 48, width = 148,
  onClick = function() strokes, stamps = {}, {}; canvas:redraw() end,
}
draw:Button {
  caption = "Paste text", left = 464, top = 84, width = 148,
  onClick = function() canvas:onDrop(gui.clipboard()) end,
}

-- Drop anything here: text, or files from the desktop, one path per line.
local dropZone = draw:Label {
  caption = "Drop files or text here", align = "center", color = "#fff4dc",
  left = 464, top = 132, width = 148, height = 100,
}
function dropZone:onDrop(text, lines)
  self.caption = string.format("%d line%s dropped:\n%s", #lines, #lines == 1 and "" or "s", lines[1] or "")
end

-- And a label to drag out of.
draw:Label {
  caption = "Drag me somewhere", align = "center", color = "#e4f0ff",
  left = 464, top = 244, width = 148, height = 40,
  onDrag = function() return "Hello from the kitchen sink" end,
}

----------------------------------------------------------------------------
-- The menu, which every form can have one of.

local function readFile(path)
  local f = assert(io.open(path, "r"))
  local s = f:read("*a")
  f:close()
  return s
end

form:Menu {
  { "&File", {
    { "&Open into notes...", shortcut = "Cmd+O", function()
      local path = gui.openfile { title = "Open into notes" }
      if path then
        notes.text = readFile(path)
        saved = notes.text
        count()
        tabs.selected = 1
        say("Opened %s.", path)
      end
    end },
    { "&Save notes as...", shortcut = "Cmd+S", function()
      local path = gui.savefile { title = "Save notes", file = "notes.txt" }
      if path then
        local f = assert(io.open(path, "w"))
        f:write(notes.text)
        f:close()
        saved = notes.text
        say("Saved %s.", path)
      end
    end },
    "-",
    { "&Quit", shortcut = "Cmd+Q", function() form:close() end },
  } },
  { "&View", {
    { "Show the &counter", checked = true, function(_, on)
      counter.visible = on
      showCounter.checked = on
    end },
    { "&Next tab", shortcut = "F6", function()
      tabs.selected = tabs.selected % 7 + 1
    end },
  } },
  { "&Help", {
    { "&About", shortcut = "F1", function()
      gui.msgbox("tlua gui kitchen sink\n\nForm, Menu, Tabs, Page, Frame, Label, Button,\n"
        .. "TextBox, CheckBox, RadioButton, ComboBox, ListBox,\n"
        .. "Tree, Table, Slider, Spinner, ProgressBar, Image, Canvas", "ok", "About")
    end },
  } },
}

----------------------------------------------------------------------------
-- The form itself: a close button, key presses, resizing, a close guard.

form:Button {
  caption = "Close", left = 532, top = 481, width = 100,
  onClick = function() form:close() end,
}

-- Keys the focused control does not use come here. Returning true would
-- keep them from anything else, the menu's shortcuts included.
function form:onKey(key)
  say("Pressed %s.", key)
end

function form:onResize()
  say("The form is %d x %d.", self.width, self.height)
end

-- Returning false keeps the form open.
function form:onClose()
  if notes.text ~= saved then
    return gui.msgbox("The notes have changed. Close anyway?", "yesno", "Kitchen sink") == "yes"
  end
end

function form:onUnload()
  if timer then timer:stop() end
  print(string.format("closing with %d characters of notes", #notes.text))
end

count()
form:show()
