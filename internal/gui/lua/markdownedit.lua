-- MarkdownEdit: Markdown edited as it looks, a word processor's way. The
-- document is markdown.editor's, laid out by markdown.layout and drawn by
-- markdown.render, as a MarkdownView draws a page, with the caret and the
-- selection; the keyboard and the mouse become the editor's calls.
--
-- The gui module defines it when it opens, as a script would with
-- gui.define; host is what it needs from Go, which it does not use yet.

local gui, host = ...
-- The markdown modules are tlua's, required when the first one is made, so
-- that the gui module opens in a state without them.
local editor, layout, render

local BAR = 12           -- the scrollbar's width
local MARGIN = 56        -- around the text, on paper
local PAD = 16           -- around the text, without paper
local TOP = 40           -- above the first block and below the last, on paper
local MAX_WIDTH = 700    -- the widest the text column gets, at size 15

gui.define {
  name = "MarkdownEdit",
  events = { "onChange", "onSelect", "onMenu", "onLink", "onHover" },
  props = {
    textSize = { type = "integer", default = 15 },
    paper = { type = "boolean", default = false },
  },
  build = function(parent, opts)
    editor = require "tlua.markdown.editor"
    layout, render = require "tlua.markdown.layout", require "tlua.markdown.render"
    local c = parent:Canvas { width = 400, height = 300 }
    c.ed = editor.new()
    c.pictures = {}
    c.pointer = "text"
    -- Its state is kept here rather than in fields: a field assigned
    -- redraws the Canvas, and these change while it draws.
    local s = {
      measurer = render.measurer(), cache = {},
      scrollY = 0, needsLayout = true, reveal = true,
    }

    local function scale(self)
      return math.max(6, tonumber(self.textSize) or 15) / 15
    end

    -- picture is the png image for a picture's src: pictures[src], or its
    -- image field when it holds a table.
    local function picture(self, src)
      local p = self.pictures and self.pictures[src]
      if type(p) == "table" and not p.width then p = p.image end
      return p
    end

    -- top is the room above the first block and below the last.
    local function top(self)
      return self.paper and TOP or math.floor(PAD * scale(self))
    end

    -- origin is where the text column starts on the Canvas, and how wide
    -- it is.
    function c:origin()
      local k = scale(self)
      local margin = self.paper and MARGIN or math.floor(PAD * k)
      local columnW = math.max(120, math.min(math.floor(MAX_WIDTH * k), self.width - BAR - 2 * margin))
      local x = math.floor((self.width - BAR - columnW) / 2)
      return x, top(self) - s.scrollY, columnW
    end

    -- load shows a document: Markdown text, or blocks markdown.parse made,
    -- with the pictures it shows by their src.
    function c:load(doc, pictures)
      if type(doc) == "string" then doc = require("tlua.markdown").parse(doc) end
      self.ed:load(doc or {})
      if pictures then self.pictures = pictures end
      s.scrollY = 0
      s.needsLayout = true
      s.reveal = true
      self:redraw()
      self:fire("onSelect")
    end

    -- markdown is the document as Markdown text.
    function c:markdown()
      return self.ed:markdown()
    end

    function c:changed()
      s.needsLayout = true
      s.reveal = true
      self:redraw()
      self:fire("onChange")
      self:fire("onSelect")
    end

    function c:moved()
      s.reveal = true
      self:redraw()
      self:fire("onSelect")
    end

    -- layout is the page as last laid out, for a program that needs to know
    -- where things are; nil before the first draw.
    function c:layout() return s.lay end

    ---------------------------------------------------------------- drawing

    local function relayout(self, columnW)
      local k = scale(self)
      if s.needsLayout or not s.lay or s.lay.width ~= columnW or s.lay.scale ~= k then
        s.lay = layout.build(self.ed.blocks, columnW, s.measurer.measure, function(src)
          local img = picture(self, src)
          if img then return img.width, img.height end
        end, s.cache, { scale = k })
        s.needsLayout = false
      end
      return s.lay
    end

    function c:onDraw(g)
      local W, H = g:size()
      local x0, _, columnW = self:origin()
      s.measurer.g = g
      local lay = relayout(self, columnW)
      local ed = self.ed
      local pad = top(self)
      local total = lay.height + 2 * pad
      if s.reveal then
        local _, cy, ch = lay:caret(ed.caret)
        if cy + pad - s.scrollY < 8 then s.scrollY = cy + pad - 8 end
        if cy + ch + pad - s.scrollY > H - 8 then s.scrollY = cy + ch + pad - H + 8 end
        s.reveal = false
      end
      s.scrollY = math.max(0, math.min(s.scrollY, math.max(0, total - H)))
      local y0 = pad - s.scrollY

      if self.paper then
        g:color("#e7e8ec"); g:fill(0, 0, W, H)
        g:color(self.color or "#ffffff")
        g:fill(x0 - MARGIN, y0 - pad + 12, columnW + 2 * MARGIN, lay.height + 2 * pad - 24)
        g:color("#d6d7db")
        g:rect(x0 - MARGIN, y0 - pad + 12, columnW + 2 * MARGIN, lay.height + 2 * pad - 24)
      else
        g:color(self.color or "#ffffff"); g:fill(0, 0, W, H)
      end

      -- The selection, under the text.
      if ed:hasSelection() then
        local a, b = ed:range()
        g:color(render.colors.selection)
        for _, r in ipairs(lay:rects(a, b)) do
          if y0 + r[2] + r[4] >= 0 and y0 + r[2] <= H then g:fill(x0 + r[1], y0 + r[2], r[3], r[4]) end
        end
      end

      local opts = {
        scale = scale(self), measure = s.measurer.measure,
        image = function(src) return picture(self, src) end,
        colors = { text = self.textColor or render.colors.text },
      }
      for _, L in ipairs(lay.blocks) do
        if y0 + L.y + L.h >= 0 and y0 + L.y <= H then render.block(g, L, x0, y0, columnW, opts) end
      end

      -- A picture, rule or table that is selected gets a frame.
      if ed:hasSelection() then
        local a, b = ed:range()
        for i = a.b, b.b do
          local L = lay.blocks[i]
          if L and not editor.hasText(L.block) and (i > a.b or a.o == 0) and (i < b.b or b.o == 1) then
            g:color("#2f6fd8"); g:width(2)
            g:rect(x0 + L.x - 2, y0 + L.y - 2, math.max(8, L.w or (L.lines[1] and L.lines[1].w) or columnW) + 4, L.h + 4)
            g:width(1)
          end
        end
      end

      -- The caret.
      local cx, cy, ch = lay:caret(ed.caret)
      g:color(self.textColor or "#000000")
      g:fill(x0 + cx, y0 + cy + 2, 2, ch - 4)

      -- The scrollbar.
      if total > H then
        local th = math.max(24, math.floor(H * H / total))
        local ty = math.floor((H - th) * s.scrollY / (total - H))
        g:color("#dcdde1"); g:fill(W - BAR, 0, BAR, H)
        g:color("#a9abb2"); g:fill(W - BAR + 2, ty, BAR - 4, th)
      end
      s.measurer.g = nil
    end

    ---------------------------------------------------------------- the mouse

    local function scrollBarTo(self, y)
      local total = s.lay.height + 2 * top(self)
      s.scrollY = (y / self.height) * total - self.height / 2
      self:redraw()
    end

    local function pageAt(self, x, y)
      local x0, y0 = self:origin()
      return x - x0, y - y0
    end

    local function hitAt(self, x, y)
      if not s.lay then return self.ed.caret end
      return s.lay:hit(pageAt(self, x, y))
    end

    function c:onMouseDown(x, y, button, double, mods)
      mods = mods or ""
      local total = s.lay and s.lay.height + 2 * top(self) or 0
      if x >= self.width - BAR and total > self.height then
        s.scrolling = true
        scrollBarTo(self, y)
        return
      end
      s.scrolling = false
      local p = hitAt(self, x, y)
      local ed = self.ed
      if button == 3 then
        -- A right click outside the selection moves the caret there first.
        local a, b = ed:range()
        local inside = ed:hasSelection() and not editor.before(p, a) and not editor.before(b, p)
        if not inside then ed:setCaret(p) end
        self:redraw()
        self:fire("onSelect")
        self:fire("onMenu", x, y)
        return
      end
      -- Cmd- or Ctrl-click follows a link, which is onLink's to do.
      if mods:find("Cmd") or mods:find("Ctrl") then
        local url = s.lay and s.lay:linkAt(pageAt(self, x, y)) or ed:linkAt(p)
        if url then
          self:fire("onLink", url)
          return
        end
      end
      if double then
        ed:selectWord(p)
      else
        ed:setCaret(p, mods:find("Shift") ~= nil)
      end
      ed.goalX = nil
      s.dragging = true
      self:redraw()
      self:fire("onSelect")
    end

    function c:onMouseDrag(x, y)
      if s.scrolling then scrollBarTo(self, y) return end
      if not s.dragging then return end
      -- Dragging past the top or bottom scrolls.
      if y < 0 then s.scrollY = s.scrollY - 20
      elseif y > self.height then s.scrollY = s.scrollY + 20 end
      self.ed:setCaret(hitAt(self, x, y), true)
      self:redraw()
      self:fire("onSelect")
    end

    function c:onMouseUp()
      s.dragging = false
      s.scrolling = false
    end

    function c:onMouseWheel(dx, dy)
      s.scrollY = s.scrollY + dy * 40
      self:redraw()
    end

    -- Over a link the pointer is a hand, and onHover says which; a Cmd- or
    -- Ctrl-click follows it.
    local function hover(self, url)
      if url ~= s.hoverLink then
        s.hoverLink = url
        local want = url and "hand" or "text"
        if self.pointer ~= want then self.pointer = want end
        self:fire("onHover", url)
      end
    end

    function c:onMouseMove(x, y)
      if not s.lay or x >= self.width - BAR then hover(self, nil) return end
      hover(self, (s.lay:linkAt(pageAt(self, x, y))))
    end

    function c:onMouseLeave()
      hover(self, nil)
    end

    ---------------------------------------------------------------- keys

    -- edits are keys that change the document; the rest only move.
    function c:onKey(key, text)
      local ed, lay = self.ed, s.lay
      local shift = key:find("Shift%+") ~= nil
      local base = key:gsub("Shift%+", "")
      local page = math.max(40, self.height - 80)
      local moves = {
        Left = function() ed:move(-1, shift) end,
        Right = function() ed:move(1, shift) end,
        ["Alt+Left"] = function() ed:move(-1, shift, true) end,
        ["Alt+Right"] = function() ed:move(1, shift, true) end,
        ["Ctrl+Left"] = function() ed:move(-1, shift, true) end,
        ["Ctrl+Right"] = function() ed:move(1, shift, true) end,
        Up = function() if lay then ed:moveLine(lay, -1, shift) end end,
        Down = function() if lay then ed:moveLine(lay, 1, shift) end end,
        Home = function() if lay then ed:moveLineEdge(lay, -1, shift) end end,
        End = function() if lay then ed:moveLineEdge(lay, 1, shift) end end,
        ["Cmd+Left"] = function() if lay then ed:moveLineEdge(lay, -1, shift) end end,
        ["Cmd+Right"] = function() if lay then ed:moveLineEdge(lay, 1, shift) end end,
        ["Cmd+Up"] = function() ed:moveToDocument(-1, shift) end,
        ["Cmd+Down"] = function() ed:moveToDocument(1, shift) end,
        ["Ctrl+Home"] = function() ed:moveToDocument(-1, shift) end,
        ["Ctrl+End"] = function() ed:moveToDocument(1, shift) end,
        PageUp = function() if lay then ed:movePage(lay, -1, shift, page) end end,
        PageDown = function() if lay then ed:movePage(lay, 1, shift, page) end end,
      }
      local keepGoal = base == "Up" or base == "Down" or base == "PageUp" or base == "PageDown"
      if moves[base] then
        local goal = ed.goalX
        moves[base]()
        if keepGoal then ed.goalX = ed.goalX or goal else ed.goalX = nil end
        if base == "PageUp" then s.scrollY = s.scrollY - page end
        if base == "PageDown" then s.scrollY = s.scrollY + page end
        self:moved()
        return true
      end
      ed.goalX = nil
      local edits = {
        Backspace = function() ed:backspace() end,
        ["Alt+Backspace"] = function() ed:deleteWord() end,
        ["Ctrl+Backspace"] = function() ed:deleteWord() end,
        Delete = function() ed:delete() end,
        Enter = function() ed:enter() end,
        ["Shift+Enter"] = function() ed:lineBreak() end,
      }
      local edit = edits[key]
      if key == "Tab" or key == "Shift+Tab" then
        local delta = key == "Tab" and 1 or -1
        if ed:indent(delta) then self:changed() return true end
        if key == "Tab" and ed:blockType() == "code" then ed:type("    ") self:changed() return true end
        return false -- Tab goes on to the next control
      end
      if edit then
        edit()
        self:changed()
        return true
      end
      local typed = text ~= "" and text:byte(1) >= 32 and text:byte(1) ~= 127
      if typed and not key:find("Cmd%+") and not key:find("Ctrl%+") and not key:find("Meta%+") then
        ed:type(text)
        self:changed()
        return true
      end
      return false
    end

    if opts.pictures then c.pictures = opts.pictures end
    return c
  end,
}
