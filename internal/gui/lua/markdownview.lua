-- MarkdownView: formatted text with links, for help, notes and wikis. It
-- shows Markdown (from text, or a file), drawn on a Canvas with
-- markdown.layout and markdown.render, and follows links: to a heading of
-- the page, to another page, which it shows in its place with a history to
-- go back through, or out to the browser.
--
-- The gui module defines it when it opens, as a script would with
-- gui.define; host is what it needs from Go: read(path), the program's own
-- files first.

local gui, host = ...
-- The markdown modules are tlua's, required when the first one is made, so
-- that the gui module opens in a state without them.
local md, layout, render

local BAR = 12   -- the scrollbar's width
local PAD = 12   -- around the text, at size 15
local STEP = 40  -- a wheel's turn, or a press of Up or Down

-- dirname is the folder a page is in, as links from it are read: "" for
-- one in the current folder.
local function dirname(path)
  return path:match("^(.*)[/\\]") or ""
end

-- join reads a relative link from a page in folder base.
local function join(base, rel)
  if rel:match("^[/\\]") or rel:match("^%a:[/\\]") or base == "" then return rel end
  local parts = {}
  for part in (base .. "/" .. rel):gmatch("[^/\\]+") do
    if part == ".." and #parts > 0 and parts[#parts] ~= ".." then
      parts[#parts] = nil
    elseif part ~= "." then
      parts[#parts + 1] = part
    end
  end
  local lead = base:match("^[/\\]") and "/" or ""
  return lead .. table.concat(parts, "/")
end

local function unescape(s)
  return (s:gsub("%%(%x%x)", function(h) return string.char(tonumber(h, 16)) end))
end

-- isPage says whether a link leads to a page to show here rather than a
-- file to open: Markdown, or a name with no extension at all.
local function isPage(path)
  local ext = path:match("%.([%w]+)$")
  return not ext or ext:lower() == "md" or ext:lower() == "markdown"
end

local WEB = { http = true, https = true, mailto = true, ftp = true, file = true }

gui.define {
  name = "MarkdownView",
  events = { "onLink", "onNavigate", "onHover" },
  props = {
    text = { type = "string", default = "" },
    file = { type = "file", default = "" },
    textSize = { type = "integer", default = 15 },
  },
  build = function(parent, opts)
    md, layout, render = require "tlua.markdown", require "tlua.markdown.layout", require "tlua.markdown.render"
    local c = parent:Canvas { width = 360, height = 240 }
    -- Its state is kept here rather than in fields: a field assigned
    -- redraws the Canvas, and these change while it draws.
    local s = {
      measurer = render.measurer(), cache = {}, images = {},
      scrollY = 0, history = {}, future = {},
    }

    local function scale(self)
      return math.max(6, tonumber(self.textSize) or 15) / 15
    end

    local function base(self)
      local file = self.file or ""
      return file ~= "" and dirname(file) or ""
    end

    -- picture is the png image a picture block shows: the program's from
    -- pictures, by its src, or read once from the page's folder.
    local function picture(self, src)
      local given = self.pictures and self.pictures[src]
      if type(given) == "table" and not given.width then given = given.image end
      if given then return given end
      local path = join(base(self), unescape(src))
      local img = s.images[path]
      if img == nil then
        local png = require "png"
        img = png.load(path) or false
        s.images[path] = img
      end
      return img or nil
    end

    -- sync makes what is shown follow file and text, whichever changed.
    local function sync(self)
      local file = self.file or ""
      if file ~= s.loadedFile then
        s.loadedFile = file
        s.images = {}
        if file ~= "" then
          local data, err = host.read(file)
          local text = data or ("# Not found\n\n`" .. file .. "`: " .. tostring(err) .. "\n")
          s.missing = not data and tostring(err) or nil
          if self.text ~= text then self.text = text end
        end
      end
      local text = self.text or ""
      if text ~= s.parsedText then
        s.parsedText = text
        s.blocks = md.parse(text)
        s.lay = nil
        s.anchor, s.caret = nil, nil
        if not s.keepScroll then s.scrollY = 0 end
        s.keepScroll = nil
      end
    end

    -- relayout lays the page out for the Canvas's width, while it draws.
    local function relayout(self, W)
      local k = scale(self)
      local width = math.max(40, W - BAR - 2 * math.floor(PAD * k))
      if not s.lay or s.lay.width ~= width or s.lay.scale ~= k then
        s.lay = layout.build(s.blocks, width, s.measurer.measure,
          function(src)
            local img = picture(self, src)
            if img then return img.width, img.height end
          end, s.cache, { scale = k })
      end
      return s.lay
    end

    local function origin(self)
      local pad = math.floor(PAD * scale(self))
      return pad, pad - s.scrollY
    end

    local function total(self)
      return s.lay and s.lay.height + 2 * math.floor(PAD * scale(self)) or 0
    end

    ---------------------------------------------------------------- what it shows

    -- headings lists the page's headings: level, text and the anchor a
    -- link to it names.
    local function headings(self)
      sync(self)
      return md.headings(s.blocks)
    end

    function c:headings()
      local out = headings(self)
      for _, h in ipairs(out) do h.block = nil end
      return out
    end

    -- scrollTo brings a heading to the top, by its anchor ("#usage" or
    -- "usage"); false when the page has no such heading.
    function c:scrollTo(anchor)
      anchor = tostring(anchor or ""):gsub("^#", "")
      for _, h in ipairs(headings(self)) do
        if h.anchor == anchor then
          s.anchor = h.block
          self:redraw()
          return true
        end
      end
      return false
    end

    ---------------------------------------------------------------- pages

    local function navigate(self, file, anchor, scrollY)
      self.file = file
      s.keepScroll = scrollY ~= nil
      sync(self)
      if scrollY then s.scrollY = scrollY end
      if anchor and anchor ~= "" then self:scrollTo(anchor) end
      self:redraw()
      self:fire("onNavigate", self.file)
      if s.missing then return false, s.missing end
      return true
    end

    -- find is the file a link to a page means: as it is written, or with
    -- .md after a name without an extension, or with dashes for its spaces
    -- as a GitHub wiki names [[Page Name]].
    local function find(path)
      if host.read(path) then return path end
      local dir, name = path:match("^(.-)([^/\\]*)$")
      local tries = {}
      if not name:match("%.%w+$") then tries[#tries + 1] = name .. ".md" end
      tries[#tries + 1] = name:gsub(" ", "-")
      if not name:match("%.%w+$") then tries[#tries + 1] = name:gsub(" ", "-") .. ".md" end
      for _, try in ipairs(tries) do
        if host.read(dir .. try) then return dir .. try end
      end
      return path
    end

    -- open shows a page, a path read as a link from the page shown is, and
    -- "#anchor" after it if it has one. It is gone back from with back().
    function c:open(path)
      sync(self)
      path = tostring(path)
      local file, anchor = path:match("^([^#]*)#?(.*)$")
      if file == "" then return self:scrollTo(anchor) end
      local current = self.file or ""
      if current ~= "" then
        s.history[#s.history + 1] = { file = current, scrollY = s.scrollY }
        s.future = {}
      end
      return navigate(self, find(join(base(self), file)), anchor)
    end

    function c:back()
      local h = table.remove(s.history)
      if not h then return false end
      s.future[#s.future + 1] = { file = self.file, scrollY = s.scrollY }
      navigate(self, h.file, nil, h.scrollY)
      return true
    end

    function c:forward()
      local h = table.remove(s.future)
      if not h then return false end
      s.history[#s.history + 1] = { file = self.file, scrollY = s.scrollY }
      navigate(self, h.file, nil, h.scrollY)
      return true
    end

    function c:canGoBack() return #s.history > 0 end
    function c:canGoForward() return #s.future > 0 end

    -- follow does what a click on a link does, unless onLink did it.
    local function follow(self, url)
      if self:fire("onLink", url) == true then return end
      if url:match("^#") then
        self:scrollTo(url)
        return
      end
      local scheme = url:match("^(%a[%w+.-]+):")
      if scheme then
        if WEB[scheme:lower()] then gui.openurl(url) end
        return
      end
      local path = url:match("^[^#]*")
      if isPage(path) then
        self:open(unescape(url))
      else
        gui.openurl(join(base(self), unescape(path)))
      end
    end

    -- follow is there for a program's own link handling to fall back on.
    function c:follow(url)
      follow(self, tostring(url))
    end

    ---------------------------------------------------------------- selection

    local function range()
      if not s.sel or not s.caret then return nil end
      local a, b = s.sel, s.caret
      if b.b < a.b or (b.b == a.b and b.o < a.o) then a, b = b, a end
      if a.b == b.b and a.o == b.o then return nil end
      return a, b
    end

    function c:selectedText()
      sync(self)
      local a, b = range()
      if not a then return "" end
      local out = {}
      for i = a.b, b.b do
        local text = md.textOf(s.blocks[i])
        local from = i == a.b and a.o or 0
        local to = i == b.b and b.o or #text
        out[#out + 1] = text:sub(from + 1, to)
      end
      return table.concat(out, "\n")
    end

    function c:copy()
      local text = self:selectedText()
      if text ~= "" then gui.clipboard(text) end
      return text ~= ""
    end

    function c:selectAll()
      sync(self)
      local n = #s.blocks
      if n == 0 then return end
      s.sel, s.caret = { b = 1, o = 0 }, { b = n, o = md.length(s.blocks[n]) }
      self:redraw()
    end

    -- search selects where text is next found after the selection, and
    -- shows it: the case of letters aside, from the top again at the end.
    -- It is false when the page does not have it.
    function c:search(text)
      sync(self)
      text = tostring(text or ""):lower()
      if text == "" or #s.blocks == 0 then return false end
      local _, b = range()
      local start = b or { b = 1, o = 0 }
      local n = #s.blocks
      for step = 0, n do
        local i = (start.b - 1 + step) % n + 1
        local hay = md.textOf(s.blocks[i]):lower()
        local from = (step == 0) and start.o + 1 or 1
        local at = hay:find(text, from, true)
        if not at and step == n then at = hay:find(text, 1, true) end
        if at then
          s.sel, s.caret = { b = i, o = at - 1 }, { b = i, o = at - 1 + #text }
          s.reveal = true
          self:redraw()
          return true
        end
      end
      return false
    end

    ---------------------------------------------------------------- drawing

    function c:onDraw(g)
      local W, H = g:size()
      sync(self)
      s.measurer.g = g
      local lay = relayout(self, W)
      local k = scale(self)
      local x0 = math.floor(PAD * k)
      if s.anchor and lay.blocks[s.anchor] then
        s.scrollY = lay.blocks[s.anchor].y
        s.anchor = nil
      end
      if s.reveal and s.caret then
        local _, cy, ch = lay:caret(s.caret)
        local pad = math.floor(PAD * k)
        if cy + pad - s.scrollY < 0 then s.scrollY = cy end
        if cy + ch + pad - s.scrollY > H then s.scrollY = cy + ch + 2 * pad - H end
        s.reveal = nil
      end
      s.scrollY = math.max(0, math.min(s.scrollY, math.max(0, total(self) - H)))
      local _, y0 = origin(self)
      local width = lay.width

      g:color(self.color or "#ffffff")
      g:fill(0, 0, W, H)
      local a, b = range()
      if a then
        g:color(render.colors.selection)
        for _, r in ipairs(lay:rects(a, b)) do
          if y0 + r[2] + r[4] >= 0 and y0 + r[2] <= H then g:fill(x0 + r[1], y0 + r[2], r[3], r[4]) end
        end
      end
      local opts = {
        scale = k, measure = s.measurer.measure,
        image = function(src) return picture(self, src) end,
        colors = { text = self.textColor or render.colors.text },
      }
      for _, L in ipairs(lay.blocks) do
        if y0 + L.y + L.h >= 0 and y0 + L.y <= H then render.block(g, L, x0, y0, width, opts) end
      end

      local all = total(self)
      if all > H then
        local th = math.max(24, math.floor(H * H / all))
        local ty = math.floor((H - th) * s.scrollY / (all - H))
        g:color("#e4e5e9"); g:fill(W - BAR, 0, BAR, H)
        g:color("#a9abb2"); g:fill(W - BAR + 2, ty, BAR - 4, th)
      end
      s.measurer.g = nil
    end

    ---------------------------------------------------------------- the mouse

    local function pageAt(self, x, y)
      local x0, y0 = origin(self)
      return x - x0, y - y0
    end

    local function scrollBarTo(self, y)
      s.scrollY = (y / self.height) * total(self) - self.height / 2
      self:redraw()
    end

    local function hover(self, url)
      if url ~= s.hoverLink then
        s.hoverLink = url
        local want = url and "hand" or "default"
        if self.pointer ~= want then self.pointer = want end
        self:fire("onHover", url)
      end
    end

    function c:onMouseDown(x, y, button, double)
      if not s.lay or button == 3 then return end
      if x >= self.width - BAR and total(self) > self.height then
        s.scrolling = true
        scrollBarTo(self, y)
        return
      end
      local px, py = pageAt(self, x, y)
      local p = s.lay:hit(px, py)
      s.press = { x = x, y = y, link = (s.lay:linkAt(px, py)) }
      if double and md.hasText(s.blocks[p.b]) then
        -- A double click selects the word there.
        local text = md.textOf(s.blocks[p.b])
        local from, to = p.o, p.o
        while from > 0 and text:sub(from, from):match("[%w_\128-\255]") do from = from - 1 end
        while to < #text and text:sub(to + 1, to + 1):match("[%w_\128-\255]") do to = to + 1 end
        s.sel, s.caret = { b = p.b, o = from }, { b = p.b, o = to }
        s.press.link = nil
      else
        s.sel, s.caret = p, p
      end
      self:redraw()
    end

    function c:onMouseDrag(x, y)
      if s.scrolling then scrollBarTo(self, y) return end
      if not s.press or not s.lay then return end
      if math.abs(x - s.press.x) + math.abs(y - s.press.y) > 3 then s.press.link = nil end
      if y < 0 then s.scrollY = s.scrollY - STEP / 2
      elseif y > self.height then s.scrollY = s.scrollY + STEP / 2 end
      s.caret = s.lay:hit(pageAt(self, x, y))
      self:redraw()
    end

    function c:onMouseUp()
      local press = s.press
      s.press, s.scrolling = nil, false
      if press and press.link then
        s.sel, s.caret = nil, nil
        follow(self, press.link)
      end
    end

    function c:onMouseMove(x, y)
      if not s.lay or x >= self.width - BAR then hover(self, nil) return end
      hover(self, (s.lay:linkAt(pageAt(self, x, y))))
    end

    function c:onMouseLeave()
      hover(self, nil)
    end

    function c:onMouseWheel(_, dy)
      s.scrollY = s.scrollY + dy * STEP
      self:redraw()
    end

    ---------------------------------------------------------------- keys

    function c:onKey(key)
      local page = math.max(STEP, self.height - STEP)
      local moves = {
        Up = -STEP, Down = STEP, PageUp = -page, PageDown = page,
        Space = page, ["Shift+Space"] = -page,
        Home = -math.huge, End = math.huge,
        ["Ctrl+Home"] = -math.huge, ["Ctrl+End"] = math.huge,
        ["Cmd+Up"] = -math.huge, ["Cmd+Down"] = math.huge,
      }
      if moves[key] then
        local d = moves[key]
        s.scrollY = d == -math.huge and 0 or d == math.huge and total(self) or s.scrollY + d
        self:redraw()
        return true
      end
      if key == "Ctrl+c" or key == "Cmd+c" then return self:copy() end
      if key == "Ctrl+a" or key == "Cmd+a" then self:selectAll() return true end
      if key == "Alt+Left" or key == "Cmd+[" then return self:back() end
      if key == "Alt+Right" or key == "Cmd+]" then return self:forward() end
      return false
    end

    return c
  end,
}
