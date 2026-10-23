-- The toolbox: the kinds of control there are, each with a small picture,
-- drawn on a Canvas. Clicking one picks it for drawing on the form;
-- double-clicking puts one of its own size in the middle of the form; the
-- pointer at the top picks nothing.

local model = require "design.model"

local M = {}

local ROW = 20 -- twenty rows fit the pane: every kind, and one of the project's

-- glyphs draw each kind in a box 20 wide and 16 high at x, y.
local glyphs = {}

function glyphs.Pointer(g, x, y)
  g:color("black")
  g:polygon(x + 6, y + 1, x + 6, y + 14, x + 9, y + 11, x + 13, y + 11)
  g:line(x + 9, y + 11, x + 12, y + 16)
end

function glyphs.Label(g, x, y)
  g:color("black"); g:font("serif", 15); g:text("A", x + 5, y - 1)
end

function glyphs.TextBox(g, x, y)
  g:color("white"); g:fill(x, y + 2, 20, 12)
  g:color("#404040"); g:rect(x, y + 2, 20, 12)
  g:font("sans", 9); g:text("ab", x + 2, y + 3); g:line(x + 15, y + 4, x + 15, y + 11)
end

function glyphs.Button(g, x, y)
  g:color("#d8d8d8"); g:fill(x, y + 3, 20, 11)
  g:color("#404040"); g:rect(x, y + 3, 20, 11)
end

function glyphs.CheckBox(g, x, y)
  g:color("white"); g:fill(x + 3, y + 2, 12, 12)
  g:color("#404040"); g:rect(x + 3, y + 2, 12, 12)
  g:width(2); g:line(x + 5, y + 8, x + 8, y + 11); g:line(x + 8, y + 11, x + 13, y + 4); g:width(1)
end

function glyphs.RadioButton(g, x, y)
  g:color("white"); g:disc(x + 9, y + 8, 6)
  g:color("#404040"); g:circle(x + 9, y + 8, 6); g:disc(x + 9, y + 8, 3)
end

function glyphs.ComboBox(g, x, y)
  glyphs.TextBox(g, x, y)
  g:color("#d8d8d8"); g:fill(x + 13, y + 3, 6, 10)
  g:color("black"); g:polygon(x + 14, y + 7, x + 18, y + 7, x + 16, y + 10)
end

function glyphs.ListBox(g, x, y)
  g:color("white"); g:fill(x + 1, y, 18, 16)
  g:color("#404040"); g:rect(x + 1, y, 18, 16)
  for i = 0, 2 do g:line(x + 4, y + 4 + i * 4, x + 15, y + 4 + i * 4) end
end

function glyphs.Frame(g, x, y)
  g:color("#707070"); g:rect(x + 1, y + 4, 18, 11)
  g:color("black"); g:font("sans", 8); g:text("xy", x + 3, y)
end

function glyphs.Panel(g, x, y)
  g:color("#c8c8c8"); g:fill(x + 1, y + 2, 18, 12)
  g:color("#909090"); g:rect(x + 1, y + 2, 18, 12)
end

function glyphs.Image(g, x, y)
  g:color("#bfe0ff"); g:fill(x + 1, y + 1, 18, 14)
  g:color("#3a8a3a"); g:polygon(x + 2, y + 14, x + 8, y + 6, x + 13, y + 14)
  g:color("#e8b000"); g:disc(x + 14, y + 5, 2)
  g:color("#404040"); g:rect(x + 1, y + 1, 18, 14)
end

function glyphs.Canvas(g, x, y)
  g:color("white"); g:fill(x + 1, y + 1, 18, 14)
  g:color("#404040"); g:rect(x + 1, y + 1, 18, 14)
  g:color("#d03030"); g:arc(x + 3, y + 3, 14, 18, 30, 150)
end

function glyphs.Slider(g, x, y)
  g:color("#404040"); g:line(x + 1, y + 8, x + 19, y + 8)
  g:color("#d8d8d8"); g:fill(x + 8, y + 3, 5, 10)
  g:color("#404040"); g:rect(x + 8, y + 3, 5, 10)
end

function glyphs.Spinner(g, x, y)
  g:color("white"); g:fill(x + 1, y + 2, 12, 12)
  g:color("#404040"); g:rect(x + 1, y + 2, 12, 12)
  g:polygon(x + 14, y + 7, x + 19, y + 7, x + 16, y + 3)
  g:polygon(x + 14, y + 9, x + 19, y + 9, x + 16, y + 13)
end

function glyphs.ProgressBar(g, x, y)
  g:color("white"); g:fill(x, y + 4, 20, 8)
  g:color("#4682c8"); g:fill(x, y + 4, 12, 8)
  g:color("#404040"); g:rect(x, y + 4, 20, 8)
end

function glyphs.Tree(g, x, y)
  g:color("#404040")
  g:line(x + 3, y + 3, x + 3, y + 13); g:line(x + 3, y + 8, x + 8, y + 8); g:line(x + 3, y + 13, x + 8, y + 13)
  g:fill(x + 1, y + 1, 5, 4); g:fill(x + 9, y + 6, 9, 4); g:fill(x + 9, y + 11, 9, 4)
end

function glyphs.Table(g, x, y)
  g:color("white"); g:fill(x + 1, y + 1, 18, 14)
  g:color("#404040"); g:rect(x + 1, y + 1, 18, 14)
  g:line(x + 1, y + 5, x + 19, y + 5); g:line(x + 1, y + 10, x + 19, y + 10)
  g:line(x + 7, y + 1, x + 7, y + 15); g:line(x + 13, y + 1, x + 13, y + 15)
end

function glyphs.MarkdownView(g, x, y)
  g:color("white"); g:fill(x + 1, y + 1, 18, 14)
  g:color("#404040"); g:rect(x + 1, y + 1, 18, 14)
  g:fill(x + 4, y + 4, 8, 2)
  g:line(x + 4, y + 8, x + 15, y + 8)
  g:color("#1a5fb4"); g:line(x + 4, y + 11, x + 11, y + 11)
end

function glyphs.MarkdownEdit(g, x, y)
  g:color("white"); g:fill(x + 1, y + 1, 18, 14)
  g:color("#404040"); g:rect(x + 1, y + 1, 18, 14)
  g:fill(x + 4, y + 4, 8, 2)
  g:line(x + 4, y + 8, x + 12, y + 8)
  g:color("#1a5fb4"); g:fill(x + 14, y + 6, 1, 7)
end

-- A control of the project's own: VB6's user-control box.
function glyphs.Defined(g, x, y)
  g:color("#fff4dc"); g:fill(x + 1, y + 1, 18, 14)
  g:color("#a06a00"); g:rect(x + 1, y + 1, 18, 14)
  g:font("sans", 9); g:text("U", x + 1, y + 1, 18, 14, "center")
end

-- miniature draws a component's layout small, in the box: what it is made
-- of, as a picture of its own for its row. Parts are boxes, with a fill that
-- says what kind they are.
local fills = { TextBox = "white", Button = "#d8d8d8", ComboBox = "white", ListBox = "white",
  Table = "white", Tree = "white", Canvas = "white", Image = "#bfe0ff", Panel = "#c8c8c8" }

local function miniature(g, x, y, doc)
  local w, h = doc.width or 240, doc.height or 120
  local k = math.min(18 / w, 14 / h)
  local ox, oy = x + 1 + (18 - w * k) / 2, y + 1 + (14 - h * k) / 2
  g:color("#a06a00"); g:rect(x, y, 20, 16)
  local function draw(node, left, top)
    for _, child in ipairs(node) do
      if child.kind ~= "Menu" and child.kind ~= "Page" then
        local cl, ct = left + (child.left or 0), top + (child.top or 0)
        local cw = child.width or (model.kinds()[child.kind] or {}).width or 60
        local ch = child.height or (model.kinds()[child.kind] or {}).height or 24
        local px, py = math.floor(ox + cl * k), math.floor(oy + ct * k)
        local pw, ph = math.max(2, math.floor(cw * k)), math.max(2, math.floor(ch * k))
        if child.kind == "Label" then
          g:color("#404040"); g:line(px, py + math.floor(ph / 2), px + pw, py + math.floor(ph / 2))
        else
          g:color(fills[child.kind] or "#f0f0f0"); g:fill(px, py, pw, ph)
          g:color("#404040"); g:rect(px, py, pw, ph)
        end
        draw(child, cl, ct)
      end
    end
  end
  draw(doc, 0, 0)
end

function glyphs.Tabs(g, x, y)
  g:color("#404040")
  g:rect(x + 1, y + 5, 18, 10); g:rect(x + 1, y + 1, 7, 5); g:rect(x + 8, y + 2, 7, 4)
end

-- new makes the toolbox in parent. onPick(kind) is told what was picked
-- (nil for the pointer), onPlace(kind) of a double-click.
function M.new(parent, opts)
  local tools = { "Pointer" }
  for _, k in ipairs(model.tools) do tools[#tools + 1] = k end
  local builtIn = #tools

  local layouts = {}  -- components by name, for their pictures
  local extra = {}    -- rows after the tools: palettes, and what they hold

  local c = parent:Canvas {
    left = opts.left, top = opts.top, width = opts.width, height = opts.height,
    grow = opts.grow, color = "#ffffff",
  }
  c.picked = 1

  function c:onDraw(g)
    local w = g:size()
    for i, kind in ipairs(tools) do
      local y = (i - 1) * ROW
      if i == self.picked then
        g:color("#cfe0f7"); g:fill(0, y, w, ROW)
      end
      if layouts[kind] then
        miniature(g, 6, y + 3, layouts[kind])
      else
        (glyphs[kind] or glyphs.Defined)(g, 6, y + 3)
      end
      g:color("black"); g:font("sans", 13)
      g:text(kind, 34, y, w - 34, ROW, "left")
    end
    -- The palettes: a heading for each, and its components not in the
    -- project yet, which a click brings in.
    for j, row in ipairs(extra) do
      local y = (#tools + j - 1) * ROW
      if row.entry then
        g:color("#707070"); g:font("sans", 13)
        g:text(row.entry.name, 34, y, w - 34, ROW, "left")
        g:color("#a06a00"); g:font("sans", 12)
        g:text("+", 8, y, 18, ROW, "center")
      else
        g:color("#e8e8e8"); g:fill(0, y, w, ROW)
        g:color("#505050"); g:font("sans", 11)
        g:text(row.heading, 6, y, w - 6, ROW, "left")
      end
    end
  end

  function c:onMouseDown(x, y, button, double)
    local i = math.floor(y / ROW) + 1
    if i > #tools and extra[i - #tools] then
      local row = extra[i - #tools]
      if row.entry and opts.onPalette then opts.onPalette(row.entry) end
      return
    end
    if not tools[i] then return end
    self.picked = i
    local kind = i > 1 and tools[i] or nil
    if double and kind then
      self.picked = 1
      opts.onPick(nil)
      opts.onPlace(kind)
    else
      opts.onPick(kind)
    end
  end

  -- tool is what is picked: a kind, or nil for the pointer.
  function c:tool()
    return self.picked > 1 and tools[self.picked] or nil
  end

  function c:reset()
    self.picked = 1
  end

  -- setTools puts the project's own controls after the built-in ones.
  -- components says which of them are components, by name, with each one's
  -- layout, to draw as its picture.
  function c:setTools(defined, components)
    for i = #tools, builtIn + 1, -1 do tools[i] = nil end
    for _, k in ipairs(defined) do tools[#tools + 1] = k end
    layouts = components or {}
    if self.picked > #tools then self.picked = 1 end
    self.height = (#tools + #extra) * ROW
    self:redraw()
  end

  -- setPalette lists what the palettes hold: entries are { folder =, name = },
  -- in the order of their folders.
  function c:setPalette(entries)
    extra = {}
    local last
    for _, e in ipairs(entries) do
      if e.folder ~= last then
        extra[#extra + 1] = { heading = "Palette  " .. (e.folder:match("([^/\\]+)[/\\]?$") or e.folder) }
        last = e.folder
      end
      extra[#extra + 1] = { entry = e }
    end
    self.height = (#tools + #extra) * ROW
    self:redraw()
  end

  -- paletteRow is where a palette component's row is, for tests.
  function c:paletteRow(name)
    for j, row in ipairs(extra) do
      if row.entry and row.entry.name == name then return 40, (#tools + j - 1) * ROW + ROW / 2 end
    end
  end

  -- rowOf is where a kind's row is, for tests: its middle, in the toolbox.
  function c:rowOf(kind)
    for i, k in ipairs(tools) do
      if k == kind then return 40, (i - 1) * ROW + ROW / 2 end
    end
  end

  return c
end

M.ROW = ROW

return M
