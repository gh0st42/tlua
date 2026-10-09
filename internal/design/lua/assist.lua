-- Help in the code window from a language server, when there is one on
-- PATH (lua-language-server, say; TLUA_LSP names another, or "off"):
--
--   - Ctrl+Space, or typing . or :, lists what could go at the cursor. Typing
--     on narrows the list; Up and Down pick, Enter or Tab put the pick in,
--     Escape closes it.
--   - The mouse resting on a name shows what it is, and so does F1 at the
--     cursor.
--   - Typing ( or , says what the call takes, in the status line.
--
-- The server is told of tlua's own modules, so gui and the form's controls
-- are known to it. Everything it is asked is answered in the background;
-- the window never waits for it.

local gui = require "gui"
local server = require "design.server"

local M = {}
M.__index = M

local LIST_W, LIST_ROWS, ROW = 260, 8, 16
-- The help tip's font, and the room around its text.
local TIP_FONT, TIP_SIZE, TIP_PAD_X, TIP_PAD_Y = "mono", 12, 16, 10

function M.new(d, box, parent)
  local a = setmetatable({ d = d, box = box, shown = {} }, M)
  -- Made after the box, so they lie over it.
  a.list = parent:ListBox {
    left = 0, top = 0, width = LIST_W, height = LIST_ROWS * ROW, visible = false,
    font = "mono", fontSize = 12,
    onChange = function(self) a:describe(self.selected) end,
    onDoubleClick = function() a:accept() end,
  }
  -- The tip is a Label on a Panel a pixel larger, which is its border.
  a.tipFrame = parent:Panel { left = 0, top = 0, width = 10, height = 10, visible = false, color = "#8c8c8c" }
  a.tip = a.tipFrame:Label {
    left = 1, top = 1, width = 8, height = 8, color = "#ffffe1", font = "mono", fontSize = 12,
  }
  box.onKey = function(_, key, text) return a:key(key, text) end
  box.onHover = function(_, pos) a:hovered(pos) end
  return a
end

-- start asks for a server for the project in dir, closing one there was.
function M:start(dir)
  self:stop()
  self.server, self.why = server.start(dir)
end

function M:stop()
  if self.server then self.server:close() end
  self.server = nil
  self:closeList()
  self:hideTip()
end

-- ready says whether the server can be asked; asked by hand, it says why
-- not in the status line.
function M:ready(byHand)
  if not self.server then
    if byHand then self.d:status(self.why or "No language server") end
    return false
  end
  local state, detail = self.server:state()
  if state == "ready" then return true end
  if byHand then
    self.d:status(state == "starting" and ("The language server is starting: " .. detail)
      or ("The language server failed: " .. tostring(detail)))
  end
  return false
end

-- await calls fn with a request's answer once it is there.
function M:await(req, fn)
  gui.every(0.03, function()
    local done, value, why = req:result()
    if not done then return true end
    fn(value, why)
    return false
  end)
end

local function isIn(list, c)
  for _, x in ipairs(list or {}) do
    if x == c then return true end
  end
  return false
end

---------------------------------------------------------------- keys

function M:key(key, text)
  if self.list.visible then
    if key == "Up" or key == "Down" then
      local n = #self.shown
      local i = self.list.selected + (key == "Up" and -1 or 1)
      self.list.selected = math.max(1, math.min(n, i))
      self:describe(self.list.selected)
      return true
    elseif key == "Enter" or key == "Tab" then
      self:accept()
      return true
    elseif key == "Escape" then
      self:closeList()
      return true
    elseif key == "Left" or key == "Right" or key == "Home" or key == "End"
        or key == "PageUp" or key == "PageDown" then
      self:closeList()
    end
  end
  if self:tipShown() then
    self:hideTip()
    if key == "Escape" then return true end
  end
  if key == "Ctrl+Space" then
    self:complete(true)
    return true
  elseif key == "F1" then
    self:help(self.box.cursor, true)
    return true
  end
  -- What the key typed is in the box once it has been handled.
  if text ~= "" then
    gui.after(0, function() self:typed(text) end)
  end
  return false
end

-- typed follows what was typed: the list narrows, and a trigger asks.
function M:typed(text)
  if self.list.visible then
    self:filter()
    return
  end
  if not self:ready(false) then return end
  local complete, signature = self.server:triggers()
  if isIn(complete, text) then
    self:complete(false)
  elseif isIn(signature, text) then
    self:signature()
  elseif text == ")" then
    self.d:status("")
  end
end

---------------------------------------------------------------- completion

-- complete asks what could go at the cursor. A server that has only just
-- started answers nothing until it has read the declarations, so nothing
-- is asked again, a few times, before it is believed.
function M:complete(byHand, tries)
  tries = tries or 0
  if not self:ready(byHand) then return end
  local box = self.box
  local cursor = box.cursor
  local req = self.server:complete(self.d:codePath(), box.text, cursor)
  self.pending = req
  self:await(req, function(items, why)
    if self.pending ~= req then return end
    self.pending = nil
    if not items or #items == 0 then
      if not why and tries < 4 then
        gui.after(0.4, function()
          if self.pending == nil and not self.list.visible and box.cursor == cursor then
            self:complete(byHand, tries + 1)
          end
        end)
        return
      end
      if byHand then self.d:status(why and ("The language server: " .. why) or "Nothing to put here") end
      return
    end
    self.items = items
    self.anchor = items[1].from
    self:filter(byHand)
  end)
end

-- filter shows the items that begin with what has been typed since the
-- list was asked for, and closes it when there are none.
function M:filter(byHand)
  local box = self.box
  local prefix = box.text:sub(self.anchor, box.cursor)
  if box.cursor + 1 < self.anchor or not prefix:match("^[%w_]*$") then
    self:closeList()
    return
  end
  local shown, lower = {}, prefix:lower()
  for _, it in ipairs(self.items or {}) do
    if it.filter:lower():sub(1, #lower) == lower then shown[#shown + 1] = it end
  end
  if #shown == 0 then
    self:closeList()
    if byHand then self.d:status("Nothing begins with " .. prefix) end
    return
  end
  self.shown = shown
  local lines = {}
  for i, it in ipairs(shown) do lines[i] = it.label end
  self.list.items = lines
  self.list.selected = 1
  local rows = math.min(LIST_ROWS, #shown)
  self.list.height = rows * ROW + 4
  self:place(self.list, self.anchor - 1, LIST_W, self.list.height)
  self.list.visible = true
  self:describe(1)
end

-- place puts a popup under the line of text position pos, or over it when
-- there is no room below, and inside the box either way.
function M:place(popup, pos, w, h)
  local box = self.box
  local x, y, lh = box:pointAt(pos)
  if not x then return end
  x = math.max(0, math.min(x, box.width - w - 4))
  local top = y + lh
  if top + h > box.height then
    -- Over the line when there is room there, and otherwise as low as it
    -- can be and still all in the box.
    top = y - h >= 0 and y - h or math.max(0, box.height - h)
  end
  popup.left, popup.top = box.left + x, box.top + top
  popup.width, popup.height = w, h
end

function M:describe(i)
  local it = self.shown[i]
  if not it then return end
  local help = (it.help or ""):gsub("%s+", " ")
  if #help > 160 then help = help:sub(1, 157) .. "..." end
  self.d:status(it.label .. "  (" .. it.kind .. ")" .. (help ~= "" and ("  —  " .. help) or ""))
end

-- accept puts the item picked in place of the word typed.
function M:accept()
  local it = self.shown[self.list.selected]
  self:closeList()
  if not it then return end
  local box = self.box
  box:select(it.from, math.max(it.to, box.cursor))
  box:insert(it.text)
  box:focus()
end

function M:closeList()
  self.list.visible = false
  self.shown, self.items, self.pending = {}, nil, nil
end

---------------------------------------------------------------- help

-- hovered is the mouse resting on the code, or moving on.
function M:hovered(pos)
  if not pos then
    if self.tipFrom == "mouse" then self:hideTip() end
    return
  end
  self:help(pos, false)
end

-- help shows what the server says of what is at pos.
function M:help(pos, byHand)
  if not self:ready(byHand) then return end
  local req = self.server:hover(self.d:codePath(), self.box.text, pos)
  self.helping = req
  self:await(req, function(text)
    if self.helping ~= req then return end
    self.helping = nil
    if not text then
      if byHand then self.d:status("Nothing is known of what is here") end
      return
    end
    self:showTip(text, pos, byHand and "key" or "mouse")
  end)
end

-- wrap breaks a line that is wider than room at its spaces, and a word
-- wider than room between its characters, keeping the line's indent on the
-- lines it makes.
local function wrap(line, room)
  local function fits(s) return (gui.measure(s, TIP_FONT, TIP_SIZE)) <= room end
  if fits(line) then return { line } end
  local indent = line:match("^%s*")
  local out, cur = {}, ""
  local function push(s)
    out[#out + 1] = s
    cur = indent
  end
  for word in line:gmatch("%S+") do
    local try = (cur == "" or cur == indent) and (cur .. word) or (cur .. " " .. word)
    if fits(try) then
      cur = try
    else
      if cur ~= "" and cur ~= indent then push(cur) end
      -- A word too long for a line of its own, cut where it must be.
      local piece = cur
      for ch in word:gmatch("[%z\1-\127\194-\244][\128-\191]*") do
        if not fits(piece .. ch) and piece ~= "" and piece ~= indent then
          push(piece)
          piece = cur
        end
        piece = piece .. ch
      end
      cur = piece
    end
  end
  if cur ~= "" and cur ~= indent then out[#out + 1] = cur end
  return out
end

function M:showTip(text, pos, from)
  local box = self.box
  -- As wide and as tall as the code box allows, and no more than it needs.
  local room = math.max(160, box.width - 2 * TIP_PAD_X - 8)
  local _, lineH = gui.measure("X", TIP_FONT, TIP_SIZE)
  local most = math.max(3, math.floor((box.height - 2 * TIP_PAD_Y - 8) / lineH))
  local lines = {}
  for line in (text .. "\n"):gmatch("(.-)\n") do
    for _, part in ipairs(wrap(line, room)) do lines[#lines + 1] = part end
  end
  while lines[#lines] == "" do lines[#lines] = nil end
  if #lines == 0 then return end
  if #lines > most then
    for i = #lines, most + 1, -1 do lines[i] = nil end
    lines[most] = "..."
  end
  local caption = table.concat(lines, "\n")
  self.tip.caption = caption
  local tw, th = gui.measure(caption, TIP_FONT, TIP_SIZE)
  local w, h = tw + 2 * TIP_PAD_X, th + 2 * TIP_PAD_Y
  self:place(self.tipFrame, pos, w, h)
  self.tip.width, self.tip.height = w - 2, h - 2
  self.tipFrame.visible = true
  self.tipFrom = from
end

function M:hideTip()
  self.tipFrame.visible = false
  self.tipFrom = nil
end

-- tipShown says whether the help tip is up.
function M:tipShown()
  return self.tipFrame.visible
end

-- signature says what the call at the cursor takes, the argument being
-- typed marked.
function M:signature()
  local req = self.server:signature(self.d:codePath(), self.box.text, self.box.cursor)
  self:await(req, function(sig)
    if not sig then return end
    local label = sig.label
    if sig.from > 0 then
      label = label:sub(1, sig.from - 1) .. "[" .. label:sub(sig.from, sig.to) .. "]" .. label:sub(sig.to + 1)
    end
    self.d:status(label)
  end)
end

return M
