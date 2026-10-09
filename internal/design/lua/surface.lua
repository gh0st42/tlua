-- The design surface: the form being designed, built from its layout with
-- the real controls, so what is seen is what will run. A transparent Canvas
-- lies over it and takes the mouse, so the controls under it never react;
-- it draws the grid, the selection and its handles, and turns drags into
-- changes to the layout.
--
-- The overlay's corner is the form's, so a control's left and top are
-- overlay coordinates too.
--
-- Several controls can be selected. The one selected last is the one the
-- others are lined up with, drawn with solid handles as VB6 drew it; sizing
-- by the handles is for one control at a time.

local gui = require "gui"
local model = require "design.model"

local M = {}
M.__index = M

local MARGIN = 16 -- around the form, on the surface
local TITLE = 22  -- the drawn title bar above it
local HANDLE = 6
local MIN = 4     -- the smallest a control can be dragged to
local BLUE = "#1e64d8"

-- The eight handles round a selected control, as where they sit on its
-- rectangle (0 to 1 along each side) and which edges they move.
local handles = {
  { 0, 0, "nw" }, { 0.5, 0, "n" }, { 1, 0, "ne" }, { 1, 0.5, "e" },
  { 1, 1, "se" }, { 0.5, 1, "s" }, { 0, 1, "sw" }, { 0, 0.5, "w" },
}
-- The form's own: its right edge, its bottom, and the corner between.
local formHandles = { { 1, 0.5, "e" }, { 0.5, 1, "s" }, { 1, 1, "se" } }

-- new makes the surface in area (a Scroll). d is the designer, which is told
-- when the selection or the layout changes, and says which tool is picked
-- and whether the grid is on.
function M.new(area, d)
  local s = setmetatable({ area = area, d = d, entries = {}, sels = {} }, M)

  s.title = area:Canvas { left = MARGIN, top = MARGIN, width = 100, height = TITLE }
  function s.title.onDraw(_, g)
    local w, h = g:size()
    g:color("#2a5ca8"); g:fill(0, 0, w, h)
    g:color("white"); g:font("sans", 13)
    g:text(tostring(model.value(s.doc, "caption") or ""), 8, 0, w - 16, h, "left")
  end

  s.host = area:Panel { left = MARGIN, top = MARGIN + TITLE, width = 100, height = 100 }
  -- The grid is drawn on the form's background, under the controls, as
  -- VB6 drew its dots: it is made first, so everything built after it is
  -- in front.
  s.grid = s.host:Canvas { left = 0, top = 0, width = 100, height = 100 }
  function s.grid.onDraw(_, g)
    if not d.gridShown then return end
    local w, h = g:size()
    g:color("#6a6a6a")
    for y = model.GRID, h - 1, model.GRID do
      for x = model.GRID, w - 1, model.GRID do g:point(x, y) end
    end
  end
  s.overlay = area:Canvas {
    transparent = true,
    left = MARGIN, top = MARGIN + TITLE, width = 100, height = 100,
  }
  s.overlay.onDraw = function(_, g) s:draw(g) end
  s.overlay.onMouseDown = function(_, x, y, _, double, mods) s:down(x, y, double, mods) end
  s.overlay.onMouseDrag = function(_, x, y) s:dragTo(x, y) end
  s.overlay.onMouseUp = function(_, x, y) s:up(x, y) end
  s.overlay.onKey = function(_, key) return s:key(key) end
  return s
end

-- load shows a form's layout, built anew. keep lists the names of controls
-- to select again, after an undo.
function M:load(doc, keep)
  for _, e in ipairs(self.entries) do e.obj:remove() end
  self.entries, self.sels, self.sel = {}, {}, nil
  self.doc = doc
  self.drag = nil
  for _, node in ipairs(doc) do
    self.entries[#self.entries + 1] = { node = node, obj = gui.load(node, self.host) }
  end
  self:fitForm()
  local again = {}
  for _, name in ipairs(keep or {}) do
    for _, e in ipairs(self.entries) do
      if e.node.name == name then again[#again + 1] = e end
    end
  end
  self:selectMany(again)
end

-- fitForm sizes and colours the form's parts from its layout.
function M:fitForm()
  local w, h = model.value(self.doc, "width"), model.value(self.doc, "height")
  self.host.width, self.host.height = w, h
  self.host.color = self.doc.color or "#c0c0c0"
  self.grid.width, self.grid.height = w, h
  self.grid.color = self.host.color
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

function M:isSelected(e)
  for _, x in ipairs(self.sels) do
    if x == e then return true end
  end
  return false
end

-- selected lists the selected controls' layouts, in the form's order.
function M:selectedNodes()
  local out = {}
  for _, e in ipairs(self.entries) do
    if self:isSelected(e) then out[#out + 1] = e.node end
  end
  return out
end

function M:announce()
  self.overlay:redraw()
  self.d:selected(self.sel and self.sel.node or self.doc, #self.sels)
end

-- select makes one control, or the form for nil, the selection.
function M:select(e)
  self.sels = e and { e } or {}
  self.sel = e
  self:announce()
end

-- selectMany selects several; the last is the one lined up with.
function M:selectMany(list)
  self.sels = list
  self.sel = list[#list]
  self:announce()
end

-- toggle adds a control to the selection, or takes it out (Shift-click).
function M:toggle(e)
  if self:isSelected(e) then
    for i, x in ipairs(self.sels) do
      if x == e then table.remove(self.sels, i) break end
    end
    self.sel = self.sels[#self.sels]
  else
    self.sels[#self.sels + 1] = e
    self.sel = e
  end
  self:announce()
end

function M:selectNode(node)
  self:select(self:entryOf(node))
end

function M:selectAll()
  local all = {}
  for _, e in ipairs(self.entries) do all[#all + 1] = e end
  self:selectMany(all)
end

---------------------------------------------------------------- drawing

function M:draw(g)
  local w, h = model.value(self.doc, "width"), model.value(self.doc, "height")
  local d = self.drag
  if d and (d.mode == "place" or d.mode == "band") then
    local x, y = math.min(d.x0, d.x1), math.min(d.y0, d.y1)
    g:color(BLUE)
    g:rect(x, y, math.abs(d.x1 - d.x0), math.abs(d.y1 - d.y0))
  end

  if self.tabMode then
    -- Each control that takes the keyboard, with its place in the order.
    g:font("sans", 11)
    for _, e in ipairs(self.entries) do
      if model.kinds()[e.node.kind].props.tabIndex then
        local l, t = rect(e)
        local n = e.node.tabIndex
        g:color(n and BLUE or "#808080"); g:fill(l, t, 22, 16)
        g:color("white"); g:text(n and tostring(n) or "-", l, t, 22, 16, "center")
      end
    end
    return
  end

  if #self.sels == 0 then
    for _, hd in ipairs(formHandles) do
      self:handle(g, hd[1] * w, hd[2] * h, false)
    end
    return
  end
  for _, e in ipairs(self.sels) do
    local x0, y0, cw, ch = rect(e)
    g:color(BLUE)
    g:rect(x0 - 1, y0 - 1, cw + 2, ch + 2)
    for _, hd in ipairs(handles) do
      self:handle(g, x0 + hd[1] * cw, y0 + hd[2] * ch, e == self.sel and #self.sels > 1)
    end
  end
end

function M:handle(g, x, y, solid)
  g:color(solid and BLUE or "white"); g:fill(x - HANDLE / 2, y - HANDLE / 2, HANDLE, HANDLE)
  g:color(BLUE); g:rect(x - HANDLE / 2, y - HANDLE / 2, HANDLE, HANDLE)
end

---------------------------------------------------------------- the mouse

-- snap puts a coordinate on the grid, while snapping is on.
function M:snap(v)
  if self.d.snap then return model.snap(v) end
  return math.floor(v + 0.5)
end

function M:down(x, y, double, mods)
  local tool = self.d:tool()
  local shift = mods and mods:find("Shift")
  if self.tabMode then
    local e = self:hit(x, y)
    if e and model.kinds()[e.node.kind].props.tabIndex then self:nextInTabOrder(e) end
    return
  end
  if double and not tool then
    -- To the code, at the default event of what was double-clicked.
    local e = self:hit(x, y)
    if e ~= self.sel then self:select(e) end
    self.drag = nil
    self.d:doubleClicked(e and e.node or self.doc)
    return
  end
  if tool then
    x, y = self:snap(x), self:snap(y)
    self.drag = { mode = "place", kind = tool, x0 = x, y0 = y, x1 = x, y1 = y }
    return
  end
  if #self.sels == 1 and not shift then
    local l, t, w, h = rect(self.sel)
    local hd = handleAt(handles, l, t, w, h, x, y)
    if hd then
      self.drag = { mode = "resize", edges = hd, x = x, y = y, l = l, t = t, w = w, h = h }
      return
    end
  end
  local e = self:hit(x, y)
  if shift then
    if e then self:toggle(e) end
    return
  end
  if not e then
    local w, h = model.value(self.doc, "width"), model.value(self.doc, "height")
    local hd = handleAt(formHandles, 0, 0, w, h, x, y)
    if hd then
      self:select(nil)
      self.drag = { mode = "form", edges = hd, x = x, y = y, w = w, h = h }
      return
    end
    -- Empty form: a rubber band, which selects what it touches.
    self.drag = { mode = "band", x0 = x, y0 = y, x1 = x, y1 = y }
    return
  end
  if not self:isSelected(e) then self:select(e) end
  -- Dragging any one of the selection drags them all.
  local start = {}
  for _, s in ipairs(self.sels) do start[s] = { s.obj.left, s.obj.top } end
  self.drag = { mode = "move", x = x, y = y, start = start, lead = e }
end

function M:dragTo(x, y)
  local d = self.drag
  if not d then return end
  if d.mode == "place" then
    d.x1, d.y1 = self:snap(x), self:snap(y)
    self.overlay:redraw()
  elseif d.mode == "band" then
    d.x1, d.y1 = x, y
    self.overlay:redraw()
  elseif d.mode == "move" then
    -- A click that wanders a pixel or two is still only a click.
    if not d.moved and math.abs(x - d.x) < 3 and math.abs(y - d.y) < 3 then return end
    if not d.moved then self.d:checkpoint() end
    d.moved = true
    -- The control dragged lands on the grid; the others keep their places
    -- relative to it.
    local lead = d.start[d.lead]
    local dx = self:snap(lead[1] + x - d.x) - lead[1]
    local dy = self:snap(lead[2] + y - d.y) - lead[2]
    for e, p in pairs(d.start) do
      self:setRect(e, math.max(0, p[1] + dx), math.max(0, p[2] + dy))
    end
  elseif d.mode == "resize" then
    if not d.moved then self.d:checkpoint() end
    local l, t, w, h = d.l, d.t, d.w, d.h
    local right, bottom = d.l + d.w, d.t + d.h
    local px, py = self:snap(x - d.x + (d.edges:find("w") and d.l or right)), self:snap(y - d.y + (d.edges:find("n") and d.t or bottom))
    local e = d.edges
    if e:find("w") then l = math.min(px, right - MIN); w = right - l end
    if e:find("e") then w = math.max(MIN, px - d.l) end
    if e:find("n") then t = math.min(py, bottom - MIN); h = bottom - t end
    if e:find("s") then h = math.max(MIN, py - d.t) end
    d.moved = true
    self:setRect(self.sel, l, t, w, h)
  elseif d.mode == "form" then
    if not d.moved then self.d:checkpoint() end
    local w, h = d.w, d.h
    if d.edges:find("e") then w = math.max(40, self:snap(d.w + x - d.x)) end
    if d.edges:find("s") then h = math.max(40, self:snap(d.h + y - d.y)) end
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
    local x1, y1 = self:snap(x), self:snap(y)
    local l, t = math.min(d.x0, x1), math.min(d.y0, y1)
    local w, h = math.abs(x1 - d.x0), math.abs(y1 - d.y0)
    if w < MIN and h < MIN then
      self:place(d.kind, l, t) -- a click: its own size
    else
      self:place(d.kind, l, t, w, h)
    end
    self.d:resetTool()
  elseif d.mode == "band" then
    local l, t = math.min(d.x0, x), math.min(d.y0, y)
    local r, b = math.max(d.x0, x), math.max(d.y0, y)
    local touched = {}
    if r - l >= MIN or b - t >= MIN then
      for _, e in ipairs(self.entries) do
        local el, et, ew, eh = rect(e)
        if el < r and el + ew > l and et < b and et + eh > t then touched[#touched + 1] = e end
      end
    end
    self:selectMany(touched)
  elseif d.moved then
    self.d:changed()
  end
  self.overlay:redraw()
end

---------------------------------------------------------------- changes

-- place adds a control of a kind to the form; without a position, in the
-- middle of it.
function M:place(kind, l, t, w, h)
  self.d:checkpoint()
  local node = model.newControl(self.doc, kind, l or 0, t or 0, w, h)
  if not l then
    node.left = math.max(0, math.floor((model.value(self.doc, "width") - node.width) / 2))
    node.top = math.max(0, math.floor((model.value(self.doc, "height") - node.height) / 2))
  end
  self:add({ node })
  return node
end

-- add puts layouts on the form, as made or pasted, and selects them.
function M:add(nodes)
  local added = {}
  for _, node in ipairs(nodes) do
    self.doc[#self.doc + 1] = node
    local e = { node = node, obj = gui.load(node, self.host) }
    self.entries[#self.entries + 1] = e
    added[#added + 1] = e
  end
  self:selectMany(added)
  self.d:changed()
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
  if #self.sels == 0 then return end
  self.d:checkpoint()
  for _, e in ipairs(self.sels) do
    e.obj:remove()
    model.remove(self.doc, e.node)
    for i, x in ipairs(self.entries) do
      if x == e then table.remove(self.entries, i) break end
    end
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
  if #self.sels == 0 then return end
  self.d:checkpoint()
  for _, node in ipairs(self:selectedNodes()) do model.toFront(self.doc, node) end
  self:restack()
  self.d:changed()
end

function M:toBack()
  if #self.sels == 0 then return end
  self.d:checkpoint()
  local nodes = self:selectedNodes()
  for i = #nodes, 1, -1 do model.toBack(self.doc, nodes[i]) end
  self:restack()
  self.d:changed()
end

-- arrange lines the selection up with the control selected last; centring
-- and spacing work with the form too.
function M:arrange(op)
  local min = (op == "centerH" or op == "centerV") and 1 or (op:find("^space") and 3 or 2)
  if #self.sels < min then
    self.d:status(min == 1 and "Select a control first" or ("Select at least " .. min .. " controls"))
    return
  end
  self.d:checkpoint()
  local rects = {}
  local ref
  for _, e in ipairs(self.sels) do
    local l, t, w, h = rect(e)
    rects[#rects + 1] = { node = e.node, l = l, t = t, w = w, h = h }
    if e == self.sel then ref = rects[#rects] end
  end
  local out = model.arrange(op, rects, ref, model.value(self.doc, "width"), model.value(self.doc, "height"))
  for _, e in ipairs(self.sels) do
    local r = out[e.node]
    self:setRect(e, r.l, r.t, r.w, r.h)
  end
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

---------------------------------------------------------------- tab order

-- tabOrder starts and ends tab-order mode, in which controls are clicked in
-- the order Tab is to visit them. Starting it clears the order there was.
function M:tabOrder(on)
  if on then
    self.d:checkpoint()
    self.tabMode = { next = 1 }
    for _, e in ipairs(self.entries) do
      if e.node.tabIndex then self:setProp(e.node, "tabIndex", nil) end
    end
    self.d:status("Tab order: click the controls in the order Tab is to visit them; Escape ends")
  else
    self.tabMode = nil
    self.d:status("")
  end
  self.overlay:redraw()
end

function M:nextInTabOrder(e)
  if e.node.tabIndex then return end
  self:setProp(e.node, "tabIndex", self.tabMode.next)
  self.tabMode.next = self.tabMode.next + 1
  self.overlay:redraw()
end

---------------------------------------------------------------- keys

-- key is a key pressed with the surface's focus: Delete deletes, the arrows
-- move the selection a pixel, and with Shift size it.
function M:key(key)
  if self.tabMode then
    if key == "Escape" then self.d:setTabOrder(false) end
    return true
  end
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
  if m and #self.sels > 0 then
    self.d:checkpoint(shift and "size-keys" or "move-keys")
    for _, e in ipairs(self.sels) do
      local l, t, w, h = rect(e)
      if shift then
        self:setRect(e, nil, nil, math.max(MIN, w + m[1]), math.max(MIN, h + m[2]))
      else
        self:setRect(e, math.max(0, l + m[1]), math.max(0, t + m[2]))
      end
    end
    self.d:changed()
    return true
  end
  return false
end

M.MARGIN, M.TITLE = MARGIN, TITLE

return M
