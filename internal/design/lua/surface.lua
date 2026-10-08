-- The design surface: the form being designed, built from its layout with
-- the real controls, so what is seen is what will run. A transparent Canvas
-- lies over it and takes the mouse, so the controls under it never react;
-- it draws the selection and its handles, and turns drags into changes to
-- the layout.
--
-- The overlay's corner is the form's, so a control's left and top are
-- overlay coordinates too.

local gui = require "gui"
local model = require "design.model"

local M = {}
M.__index = M

local MARGIN = 16 -- around the form, on the surface
local TITLE = 22  -- the drawn title bar above it
local HANDLE = 6
local MIN = 4     -- the smallest a control can be dragged to

-- The eight handles round a selected control, as where they sit on its
-- rectangle (0 to 1 along each side) and which edges they move.
local handles = {
  { 0, 0, "nw" }, { 0.5, 0, "n" }, { 1, 0, "ne" }, { 1, 0.5, "e" },
  { 1, 1, "se" }, { 0.5, 1, "s" }, { 0, 1, "sw" }, { 0, 0.5, "w" },
}
-- The form's own: its right edge, its bottom, and the corner between.
local formHandles = { { 1, 0.5, "e" }, { 0.5, 1, "s" }, { 1, 1, "se" } }

-- new makes the surface in area (a Scroll). d is the designer, which is told
-- when the selection or the layout changes, and says which tool is picked.
function M.new(area, d)
  local s = setmetatable({ area = area, d = d, entries = {} }, M)

  s.title = area:Canvas { left = MARGIN, top = MARGIN, width = 100, height = TITLE }
  function s.title.onDraw(_, g)
    local w, h = g:size()
    g:color("#2a5ca8"); g:fill(0, 0, w, h)
    g:color("white"); g:font("sans", 13)
    g:text(tostring(model.value(s.doc, "caption") or ""), 8, 0, w - 16, h, "left")
  end

  s.host = area:Panel { left = MARGIN, top = MARGIN + TITLE, width = 100, height = 100 }
  s.overlay = area:Canvas {
    transparent = true,
    left = MARGIN, top = MARGIN + TITLE, width = 100, height = 100,
  }
  s.overlay.onDraw = function(_, g) s:draw(g) end
  s.overlay.onMouseDown = function(_, x, y, button, double) s:down(x, y) end
  s.overlay.onMouseDrag = function(_, x, y) s:dragTo(x, y) end
  s.overlay.onMouseUp = function(_, x, y) s:up(x, y) end
  s.overlay.onKey = function(_, key) return s:key(key) end
  return s
end

-- load shows a form's layout, built anew.
function M:load(doc)
  for _, e in ipairs(self.entries) do e.obj:remove() end
  self.entries = {}
  self.doc = doc
  self.sel = nil
  self.drag = nil
  for _, node in ipairs(doc) do
    self.entries[#self.entries + 1] = { node = node, obj = gui.load(node, self.host) }
  end
  self:fitForm()
end

-- fitForm sizes and colours the form's parts from its layout.
function M:fitForm()
  local w, h = model.value(self.doc, "width"), model.value(self.doc, "height")
  self.host.width, self.host.height = w, h
  self.host.color = self.doc.color or "#c0c0c0"
  self.title.width = w
  -- The overlay reaches past the form, for the handles on its edges.
  self.overlay.width, self.overlay.height = w + HANDLE * 2, h + HANDLE * 2
  self.title:redraw()
  self.overlay:redraw()
end

-- rect is where a control is, read from the control itself.
local function rect(e)
  return e.obj.left, e.obj.top, e.obj.width, e.obj.height
end

local function handleAt(list, x0, y0, w, h, x, y)
  for _, hd in ipairs(list) do
    local hx, hy = x0 + hd[1] * w, y0 + hd[2] * h
    if math.abs(x - hx) <= HANDLE and math.abs(y - hy) <= HANDLE then return hd[3] end
  end
end

-- hit is the topmost control under a point.
function M:hit(x, y)
  for i = #self.entries, 1, -1 do
    local e = self.entries[i]
    local l, t, w, h = rect(e)
    if x >= l and x < l + w and y >= t and y < t + h then return e end
  end
end

function M:entryOf(node)
  for _, e in ipairs(self.entries) do
    if e.node == node then return e end
  end
end

-- select makes a control, or the form for nil, the one being edited.
function M:select(e)
  self.sel = e
  self.overlay:redraw()
  self.d:selected(e and e.node or self.doc)
end

function M:selectNode(node)
  self:select(self:entryOf(node))
end

function M:draw(g)
  local blue = "#1e64d8"
  local d = self.drag
  if d and d.mode == "place" then
    local x, y = math.min(d.x0, d.x1), math.min(d.y0, d.y1)
    g:color(blue)
    g:rect(x, y, math.abs(d.x1 - d.x0), math.abs(d.y1 - d.y0))
  end
  local list, x0, y0, w, h
  if self.sel then
    x0, y0, w, h = rect(self.sel)
    g:color(blue)
    g:rect(x0 - 1, y0 - 1, w + 2, h + 2)
    list = handles
  else
    x0, y0 = 0, 0
    w, h = model.value(self.doc, "width"), model.value(self.doc, "height")
    list = formHandles
  end
  for _, hd in ipairs(list) do
    local hx, hy = x0 + hd[1] * w, y0 + hd[2] * h
    g:color("white"); g:fill(hx - HANDLE / 2, hy - HANDLE / 2, HANDLE, HANDLE)
    g:color(blue); g:rect(hx - HANDLE / 2, hy - HANDLE / 2, HANDLE, HANDLE)
  end
end

function M:down(x, y)
  local tool = self.d:tool()
  if tool then
    self.drag = { mode = "place", kind = tool, x0 = x, y0 = y, x1 = x, y1 = y }
    return
  end
  if self.sel then
    local l, t, w, h = rect(self.sel)
    local hd = handleAt(handles, l, t, w, h, x, y)
    if hd then
      self.drag = { mode = "resize", edges = hd, x = x, y = y, l = l, t = t, w = w, h = h }
      return
    end
  end
  local e = self:hit(x, y)
  if not e then
    local w, h = model.value(self.doc, "width"), model.value(self.doc, "height")
    local hd = handleAt(formHandles, 0, 0, w, h, x, y)
    if hd then
      self:select(nil)
      self.drag = { mode = "form", edges = hd, x = x, y = y, w = w, h = h }
      return
    end
  end
  if e ~= self.sel or not e then self:select(e) end
  if e then
    self.drag = { mode = "move", x = x, y = y, l = e.obj.left, t = e.obj.top }
  end
end

function M:dragTo(x, y)
  local d = self.drag
  if not d then return end
  if d.mode == "place" then
    d.x1, d.y1 = x, y
    self.overlay:redraw()
  elseif d.mode == "move" then
    -- A click that wanders a pixel or two is still only a click.
    if not d.moved and math.abs(x - d.x) < 3 and math.abs(y - d.y) < 3 then return end
    d.moved = true
    self:setRect(self.sel, math.max(0, d.l + x - d.x), math.max(0, d.t + y - d.y))
  elseif d.mode == "resize" then
    local dx, dy = x - d.x, y - d.y
    local l, t, w, h = d.l, d.t, d.w, d.h
    local e = d.edges
    if e:find("w") then l, w = math.min(d.l + dx, d.l + d.w - MIN), math.max(MIN, d.w - dx) end
    if e:find("e") then w = math.max(MIN, d.w + dx) end
    if e:find("n") then t, h = math.min(d.t + dy, d.t + d.h - MIN), math.max(MIN, d.h - dy) end
    if e:find("s") then h = math.max(MIN, d.h + dy) end
    d.moved = true
    self:setRect(self.sel, l, t, w, h)
  elseif d.mode == "form" then
    local w, h = d.w, d.h
    if d.edges:find("e") then w = math.max(40, d.w + x - d.x) end
    if d.edges:find("s") then h = math.max(40, d.h + y - d.y) end
    d.moved = true
    model.set(self.doc, "width", w)
    model.set(self.doc, "height", h)
    self:fitForm()
    self.d:moved(self.doc)
  end
end

function M:up(x, y)
  local d = self.drag
  self.drag = nil
  if not d then return end
  if d.mode == "place" then
    local l, t = math.min(d.x0, x), math.min(d.y0, y)
    local w, h = math.abs(x - d.x0), math.abs(y - d.y0)
    if w < MIN and h < MIN then w, h = nil, nil end -- a click: its own size
    self:place(d.kind, l, t, w, h)
    self.d:resetTool()
  elseif d.moved then
    self.d:changed()
  end
  self.overlay:redraw()
end

-- place adds a control of a kind to the form; without a position, in the
-- middle of it.
function M:place(kind, l, t, w, h)
  local node = model.newControl(self.doc, kind, l or 0, t or 0, w, h)
  if not l then
    node.left = math.max(0, math.floor((model.value(self.doc, "width") - node.width) / 2))
    node.top = math.max(0, math.floor((model.value(self.doc, "height") - node.height) / 2))
  end
  self.doc[#self.doc + 1] = node
  local e = { node = node, obj = gui.load(node, self.host) }
  self.entries[#self.entries + 1] = e
  self:select(e)
  self.d:changed()
  return node
end

-- setRect moves and sizes a control, on screen and in the layout.
function M:setRect(e, l, t, w, h)
  local values = { left = l, top = t, width = w, height = h }
  for _, p in ipairs { "left", "top", "width", "height" } do
    local v = values[p]
    if v then
      v = math.floor(v + 0.5)
      e.obj[p] = v
      model.set(e.node, p, v)
    end
  end
  self.overlay:redraw()
  self.d:moved(e.node)
end

function M:deleteSelected()
  local e = self.sel
  if not e then return end
  e.obj:remove()
  model.remove(self.doc, e.node)
  for i, x in ipairs(self.entries) do
    if x == e then table.remove(self.entries, i) break end
  end
  self:select(nil)
  self.d:changed()
end

-- restack puts the controls on screen in the layout's order.
function M:restack()
  table.sort(self.entries, function(a, b)
    return model.indexOf(self.doc, a.node) < model.indexOf(self.doc, b.node)
  end)
  for _, e in ipairs(self.entries) do e.obj:raise() end
  self.overlay:redraw()
end

function M:toFront()
  if not self.sel then return end
  model.toFront(self.doc, self.sel.node)
  self:restack()
  self.d:changed()
end

function M:toBack()
  if not self.sel then return end
  model.toBack(self.doc, self.sel.node)
  self:restack()
  self.d:changed()
end

-- setProp changes a property of the form or of a control from the grid.
-- It returns false and why when the control will not take the value.
function M:setProp(node, prop, value)
  if node == self.doc then
    model.set(self.doc, prop, value)
    if prop == "caption" or prop == "width" or prop == "height" or prop == "color" then
      self:fitForm()
    end
    self.d:changed()
    return true
  end
  local e = self:entryOf(node)
  if not e then return false, "no such control" end
  local ok, err = pcall(function() e.obj[prop] = value end)
  if not ok and tostring(err):find("can only be given when it is made") then
    -- It picks the widget: the control is made again, in its place.
    model.set(node, prop, value)
    self:rebuild(e)
    ok, err = true, nil
  end
  if not ok then
    return false, (tostring(err):gsub("^.-gui: ", ""))
  end
  model.set(node, prop, value)
  self.overlay:redraw()
  self.d:changed()
  return true
end

function M:rebuild(e)
  e.obj:remove()
  e.obj = gui.load(e.node, self.host)
  self:restack()
  self.d:changed()
end

-- key is a key pressed with the surface's focus: Delete deletes, the arrows
-- move the selection a pixel, and with Shift size it.
function M:key(key)
  if key == "Delete" or key == "Backspace" then
    self:deleteSelected()
    return true
  end
  if key == "Escape" then
    self.d:resetTool()
    self:select(nil)
    return true
  end
  local moves = { Left = { -1, 0 }, Right = { 1, 0 }, Up = { 0, -1 }, Down = { 0, 1 } }
  local shift = key:find("^Shift%+") ~= nil
  local m = moves[(key:gsub("^Shift%+", ""))]
  if m and self.sel then
    local l, t, w, h = rect(self.sel)
    if shift then
      self:setRect(self.sel, nil, nil, math.max(MIN, w + m[1]), math.max(MIN, h + m[2]))
    else
      self:setRect(self.sel, math.max(0, l + m[1]), math.max(0, t + m[2]))
    end
    self.d:changed()
    return true
  end
  return false
end

M.MARGIN, M.TITLE = MARGIN, TITLE

return M
