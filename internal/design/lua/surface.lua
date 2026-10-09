-- The design surface: the form being designed, built from its layout with
-- the real controls, so what is seen is what will run. A transparent Canvas
-- lies over it and takes the mouse, so the controls under it never react;
-- it draws the selection and its handles, and turns drags into changes to
-- the layout.
--
-- The overlay's corner is the form's, so positions on the surface are form
-- coordinates. A control inside a Frame, a Panel or a page of a Tabs keeps
-- its left and top relative to that container, as layouts write them; the
-- surface converts as it goes.
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
local TABROW = 25 -- the row of tabs above a Tabs' pages
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
  local s = setmetatable({ area = area, d = d, entries = {}, sels = {}, objs = {}, cache = {} }, M)

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
  s.overlay.onMouseDown = function(_, x, y, button, double, mods) s:down(x, y, double, mods, button) end
  s.overlay.onMouseDrag = function(_, x, y) s:dragTo(x, y) end
  s.overlay.onMouseUp = function(_, x, y) s:up(x, y) end
  s.overlay.onKey = function(_, key) return s:key(key) end
  return s
end

---------------------------------------------------------------- the tree on screen

-- bind notes the control built for a layout, and those built inside it,
-- which are found by name (an unnamed one is part of what holds it).
function M:bind(node, obj)
  self.objs[node] = obj
  for _, child in ipairs(node) do
    local c = child.name and self.host:find(child.name)
    if c then self:bind(child, c) end
  end
end

function M:unbind(node)
  self.objs[node], self.cache[node] = nil, nil
  for _, child in ipairs(node) do self:unbind(child) end
end

-- objOf is what a layout node was built as: the host stands for the form.
function M:objOf(node)
  if node == self.doc then return self.host end
  return self.objs[node]
end

-- index lists every control there is, those that hold others before what
-- they hold, so that the last that contains a point is the one on top. An
-- entry stays the same table for as long as its control is there.
function M:index()
  local entries = {}
  model.walk(self.doc, function(node, parent)
    local obj = self.objs[node]
    if obj then
      local e = self.cache[node] or { node = node }
      self.cache[node] = e
      e.obj, e.parent = obj, parent
      entries[#entries + 1] = e
    end
  end)
  self.entries = entries
  local keep = {}
  for _, e in ipairs(self.sels) do
    if self.cache[e.node] == e and self.objs[e.node] then keep[#keep + 1] = e end
  end
  self.sels = keep
  if self.sel and not self.objs[self.sel.node] then self.sel = keep[#keep] end
end

-- load shows a form's layout, built anew. keep lists the names of controls
-- to select again, after an undo.
function M:load(doc, keep)
  for _, node in ipairs(self.doc or {}) do
    if self.objs[node] then self.objs[node]:remove() end
  end
  self.objs, self.cache, self.sels, self.sel = {}, {}, {}, nil
  self.doc = doc
  self.drag = nil
  for _, node in ipairs(doc) do
    self:bind(node, gui.load(node, self.host))
  end
  self:index()
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

-- at is where a control's corner is in form coordinates.
function M:at(obj)
  local x, y = 0, 0
  local o = obj
  while o and o ~= self.host do
    x, y = x + o.left, y + o.top
    o = o.parent
  end
  return x, y
end

-- rect is where a control is in form coordinates, read from the control.
function M:rect(e)
  local x, y = self:at(e.obj)
  return x, y, e.obj.width, e.obj.height
end

-- origin is the corner that what a holder holds is measured from.
function M:origin(holder)
  if holder == self.doc then return 0, 0 end
  return self:at(self.objs[holder])
end

-- holderSize is how much room a holder has.
function M:holderSize(holder)
  if holder == self.doc then return model.value(self.doc, "width"), model.value(self.doc, "height") end
  local obj = self.objs[holder]
  return obj.width, obj.height
end

-- shown says whether a control can be seen: not on a page that is not.
function M:shown(e)
  local o = e.obj
  while o and o ~= self.host do
    if not o.visible then return false end
    o = o.parent
  end
  return true
end

local function inside(x, y, l, t, w, h)
  return x >= l and x < l + w and y >= t and y < t + h
end

local function handleAt(list, x0, y0, w, h, x, y)
  for _, hd in ipairs(list) do
    local hx, hy = x0 + hd[1] * w, y0 + hd[2] * h
    if math.abs(x - hx) <= HANDLE and math.abs(y - hy) <= HANDLE then return hd[3] end
  end
end

-- hit is the topmost control under a point. A page is not one: a click on
-- one is a click on its Tabs.
function M:hit(x, y)
  for i = #self.entries, 1, -1 do
    local e = self.entries[i]
    if e.node.kind ~= "Page" and self:shown(e) and inside(x, y, self:rect(e)) then return e end
  end
end

-- holderAt is what a control put at a point goes into: the topmost Frame,
-- Panel or showing page there, or the form. Controls being moved are not
-- somewhere they can go into.
function M:holderAt(x, y, moving)
  for i = #self.entries, 1, -1 do
    local e = self.entries[i]
    if model.holders[e.node.kind] and self:shown(e) and inside(x, y, self:rect(e)) then
      local ok = true
      for _, m in ipairs(moving or {}) do
        if model.within(e.node, m.node) then ok = false end
      end
      if ok then return e.node end
    end
  end
  return self.doc
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

-- selectedNodes lists the selected controls' layouts, in the form's order.
function M:selectedNodes()
  local out = {}
  for _, e in ipairs(self.entries) do
    if self:isSelected(e) then out[#out + 1] = e.node end
  end
  return out
end

-- outermost leaves out selected controls that are inside other selected
-- ones: they go wherever those go.
function M:outermost()
  local out = {}
  for _, e in ipairs(self.sels) do
    local inner = false
    for _, f in ipairs(self.sels) do
      if f ~= e and model.within(e.node, f.node) then inner = true end
    end
    if not inner then out[#out + 1] = e end
  end
  return out
end

-- sameHolder is what holds every selected control, if one thing does.
function M:sameHolder()
  local holder
  for _, e in ipairs(self.sels) do
    if holder and e.parent ~= holder then return nil end
    holder = e.parent
  end
  return holder
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
  for _, e in ipairs(self.entries) do
    if e.parent == self.doc then all[#all + 1] = e end
  end
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
  if d and d.mode == "move" and d.moved and d.into and d.into ~= self.doc then
    -- Where a drop would put the controls.
    local e = self:entryOf(d.into)
    if e then
      local x, y, cw, ch = self:rect(e)
      g:color("#2a9d3a"); g:rect(x - 2, y - 2, cw + 4, ch + 4)
    end
  end

  if self.tabMode then
    -- Each control that takes the keyboard, with its place in the order.
    g:font("sans", 11)
    for _, e in ipairs(self.entries) do
      if model.kinds()[e.node.kind].props.tabIndex and self:shown(e) then
        local l, t = self:rect(e)
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
    if self:shown(e) then
      local x0, y0, cw, ch = self:rect(e)
      g:color(BLUE)
      g:rect(x0 - 1, y0 - 1, cw + 2, ch + 2)
      for _, hd in ipairs(handles) do
        self:handle(g, x0 + hd[1] * cw, y0 + hd[2] * ch, e == self.sel and #self.sels > 1)
      end
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

-- contextClick says whether a press asks for a context menu: the right
-- button, or Ctrl with the left one on a Mac.
local function contextClick(button, mods)
  return button == 3 or (button == 1 and gui.platform == "darwin" and mods and mods:find("Ctrl") ~= nil)
end

function M:down(x, y, double, mods, button)
  local tool = self.d:tool()
  local shift = mods and mods:find("Shift")
  if contextClick(button, mods) and not tool and not self.tabMode then
    -- What is under the mouse is selected, unless it is selected already
    -- (with others, perhaps), and its menu comes up.
    local e = self:hit(x, y)
    if not (e and self:isSelected(e)) then self:select(e) end
    self.drag = nil
    self.d:contextMenu(e and e.node or self.doc)
    return
  end
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
    local l, t, w, h = self:rect(self.sel)
    local hd = handleAt(handles, l, t, w, h, x, y)
    if hd then
      local rl, rt = self.sel.obj.left, self.sel.obj.top
      self.drag = { mode = "resize", edges = hd, x = x, y = y, l = rl, t = rt, w = w, h = h }
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
  end
  if not e or (model.holders[e.node.kind] and not self:isSelected(e) and #self.sels > 0 and self:sameHolder() == e.node) then
    -- Empty form, or empty room in the container holding the selection: a
    -- rubber band, which selects what it touches in what it starts in.
    self.drag = { mode = "band", x0 = x, y0 = y, x1 = x, y1 = y, holder = self:holderAt(x, y), on = e }
    if e then self.drag.holder = e.node end
    return
  end
  if e.node.kind == "Tabs" and self:isSelected(e) then
    local _, t = self:rect(e)
    if y - t < TABROW and #e.node > 0 then
      -- A click on the row of tabs of a selected Tabs shows its next page.
      e.obj.selected = (e.obj.selected % #e.node) + 1
      self:index()
      self.overlay:redraw()
      self.d:moved(e.node)
      return
    end
  end
  if not self:isSelected(e) then self:select(e) end
  -- Dragging any one of the selection drags them all.
  local start = {}
  for _, s in ipairs(self:outermost()) do start[s] = { s.obj.left, s.obj.top } end
  self.drag = { mode = "move", x = x, y = y, start = start, lead = start[e] and e or self:outermost()[1] }
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
    local moving = {}
    for e in pairs(d.start) do moving[#moving + 1] = e end
    d.into = self:holderAt(x, y, moving)
  elseif d.mode == "resize" then
    if not d.moved then self.d:checkpoint() end
    local l, t, w, h = d.l, d.t, d.w, d.h
    local right, bottom = d.l + d.w, d.t + d.h
    local px = self:snap(x - d.x + (d.edges:find("w") and d.l or right))
    local py = self:snap(y - d.y + (d.edges:find("n") and d.t or bottom))
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
    local holder = self:holderAt(d.x0, d.y0)
    local ox, oy = self:origin(holder)
    if w < MIN and h < MIN then
      self:place(d.kind, l - ox, t - oy, nil, nil, holder) -- a click: its own size
    else
      self:place(d.kind, l - ox, t - oy, w, h, holder)
    end
    self.d:resetTool()
  elseif d.mode == "band" then
    local l, t = math.min(d.x0, x), math.min(d.y0, y)
    local r, b = math.max(d.x0, x), math.max(d.y0, y)
    if r - l < MIN and b - t < MIN then
      -- No band after all, but a click: on a container, or the form.
      self:select(d.on)
      self.overlay:redraw()
      return
    end
    local touched = {}
    do
      for _, e in ipairs(self.entries) do
        if e.parent == d.holder and self:shown(e) then
          local el, et, ew, eh = self:rect(e)
          if el < r and el + ew > l and et < b and et + eh > t then touched[#touched + 1] = e end
        end
      end
    end
    self:selectMany(touched)
  elseif d.moved then
    if d.mode == "move" then
      local holder = self:sameHolder()
      local moving = {}
      for e in pairs(d.start) do moving[#moving + 1] = e end
      local into = self:holderAt(x, y, moving)
      if holder and into ~= holder then self:reparent(moving, into) end
    end
    self.d:changed()
  end
  self.overlay:redraw()
end

---------------------------------------------------------------- changes

-- place adds a control of a kind to a holder (the form, unless given); with
-- no position, in the middle of it.
function M:place(kind, l, t, w, h, holder)
  holder = holder or self.doc
  self.d:checkpoint()
  local node = model.newControl(self.doc, kind, l or 0, t or 0, w, h)
  if not l then
    local hw, hh = self:holderSize(holder)
    node.left = math.max(0, math.floor((hw - (node.width or 0)) / 2))
    node.top = math.max(0, math.floor((hh - (node.height or 0)) / 2))
  end
  self:add({ node }, holder)
  return node
end

-- add puts layouts in a holder (the form, unless given), as made or pasted,
-- and selects them.
function M:add(nodes, holder)
  holder = holder or self.doc
  local into = self:objOf(holder)
  for _, node in ipairs(nodes) do
    holder[#holder + 1] = node
    self:bind(node, gui.load(node, into))
  end
  self:index()
  local added = {}
  for _, node in ipairs(nodes) do added[#added + 1] = self.cache[node] end
  self:selectMany(added)
  self.d:changed()
end

-- pasteHolder is where pasted controls go: into the container selected, or
-- the page showing on the Tabs selected, or else the form.
function M:pasteHolder()
  local e = self.sel
  if not e then return self.doc end
  if e.node.kind == "Frame" or e.node.kind == "Panel" then return e.node end
  if e.node.kind == "Tabs" and #e.node > 0 then return e.node[e.obj.selected] or e.node[1] end
  return self.doc
end

-- reparent moves controls into another holder, where they stay where they
-- were on the form.
function M:reparent(list, into)
  local target = self:objOf(into)
  local ox, oy = self:origin(into)
  for _, e in ipairs(list) do
    local ax, ay = self:at(e.obj)
    model.removeNode(self.doc, e.node)
    into[#into + 1] = e.node
    target:add(e.obj)
    self:setRect(e, math.max(0, ax - ox), math.max(0, ay - oy))
  end
  self:index()
end

-- setRect moves and sizes a control, on screen and in the layout, in the
-- coordinates of what holds it.
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

-- inside counts the controls a node holds, all the way down.
local function inside(node)
  local n = 0
  model.walk(node, function() n = n + 1 end)
  return n
end

function M:deleteSelected()
  local list = self:outermost()
  if #list == 0 then return end
  -- A container goes with what it holds, so that is asked first.
  local held, names = 0, {}
  for _, e in ipairs(list) do
    local n = inside(e.node)
    if n > 0 then
      held = held + n
      names[#names + 1] = e.node.name or e.node.kind
    end
  end
  if held > 0 then
    local what = #names == 1 and names[1] or (#names .. " containers")
    local question = ("Delete %s and the %d control%s in it?"):format(what, held, held == 1 and "" or "s")
    if self.d:ask(question, "yesno", "Delete") ~= "yes" then return end
  end
  self.d:checkpoint()
  for _, e in ipairs(list) do
    e.obj:remove()
    model.removeNode(self.doc, e.node)
    self:unbind(e.node)
  end
  self.sels, self.sel = {}, nil
  self:index()
  self:select(nil)
  self.d:changed()
end

-- restack puts a holder's controls on screen in the layout's order.
function M:restack(holder)
  for _, child in ipairs(holder) do
    if self.objs[child] then self.objs[child]:raise() end
  end
  self:index()
  self.overlay:redraw()
end

function M:toFront()
  if #self.sels == 0 then return end
  self.d:checkpoint()
  for _, node in ipairs(self:selectedNodes()) do
    local parent = model.parentOf(self.doc, node)
    model.toFront(parent, node)
    self:restack(parent)
  end
  self.d:changed()
end

function M:toBack()
  if #self.sels == 0 then return end
  self.d:checkpoint()
  local nodes = self:selectedNodes()
  for i = #nodes, 1, -1 do
    local parent = model.parentOf(self.doc, nodes[i])
    model.toBack(parent, nodes[i])
    self:restack(parent)
  end
  self.d:changed()
end

-- arrange lines the selection up with the control selected last; centring
-- and spacing work with what holds them too.
function M:arrange(op)
  local min = (op == "centerH" or op == "centerV") and 1 or (op:find("^space") and 3 or 2)
  if #self.sels < min then
    self.d:status(min == 1 and "Select a control first" or ("Select at least " .. min .. " controls"))
    return
  end
  local holder = self:sameHolder()
  if not holder then
    self.d:status("Select controls in the same container to line them up")
    return
  end
  self.d:checkpoint()
  local rects = {}
  local ref
  for _, e in ipairs(self.sels) do
    rects[#rects + 1] = { node = e.node, l = e.obj.left, t = e.obj.top, w = e.obj.width, h = e.obj.height }
    if e == self.sel then ref = rects[#rects] end
  end
  local hw, hh = self:holderSize(holder)
  local out = model.arrange(op, rects, ref, hw, hh)
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
  local e = self.cache[node]
  if not e then return false, "no such control" end
  local info = model.kinds()[node.kind]
  if info.defined and info.props[prop] and not model.kinds().Label.props[prop] then
    -- A control of the project's own is built from its props: it is built
    -- again, so that what it draws follows.
    model.set(node, prop, value)
    self:rebuild(e)
    return true
  end
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
  if node.kind == "Tabs" and prop == "selected" then self:index() end
  self.overlay:redraw()
  self.d:changed()
  return true
end

function M:rebuild(e)
  local parent = model.parentOf(self.doc, e.node)
  e.obj:remove()
  self:bind(e.node, gui.load(e.node, self:objOf(parent)))
  self:restack(parent)
  self.d:changed()
end

---------------------------------------------------------------- pages

-- tabsSelected is the Tabs selected, also when one of its pages' controls
-- is not.
function M:tabsSelected()
  local e = self.sel
  if e and e.node.kind == "Tabs" then return e end
end

function M:addPage()
  local e = self:tabsSelected()
  if not e then self.d:status("Select a Tabs to add a page to") return end
  self.d:checkpoint()
  local node = { kind = "Page", name = model.uniqueName(self.doc, "Page"), caption = "Page " .. (#e.node + 1) }
  e.node[#e.node + 1] = node
  self:bind(node, gui.load(node, e.obj))
  e.obj.selected = #e.node
  self:index()
  self.overlay:redraw()
  self.d:changed()
end

function M:removePage()
  local e = self:tabsSelected()
  if not e or #e.node == 0 then self.d:status("Select a Tabs to take a page from") return end
  self.d:checkpoint()
  local i = e.obj.selected
  local page = e.node[i]
  self.objs[page]:remove()
  table.remove(e.node, i)
  self:unbind(page)
  e.obj.selected = math.min(i, #e.node)
  self:index()
  self.overlay:redraw()
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
    for _, e in ipairs(shift and self.sels or self:outermost()) do
      local l, t, w, h = e.obj.left, e.obj.top, e.obj.width, e.obj.height
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
