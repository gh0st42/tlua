-- markdown.layout is where a Markdown document's blocks go on a page:
-- each block's place and look, its text broken into lines that fit, and
-- the way between positions in the text and points on the page. It
-- measures text with a function it is given, so it works the same on a
-- Canvas and in tests, with no screen.
--
--   local layout = require "markdown.layout"
--   local lay = layout.build(blocks, width, measure, imageSize [, cache [, opts]])
--   measure(text, font) -> width     font = { face, size, bold, italic }
--   imageSize(src)      -> w, h      or nil when the picture is missing
--   opts.scale                       every size and space times this (1)
--
-- Coordinates are the page's: 0 at the left of the text column and at the
-- top of the first block. markdown.render draws what it lays out.

local md = require "tlua.markdown"

local M = {}

-- How each kind of block looks, at scale 1.
M.styles = {
  p = { size = 15, after = 10 },
  h1 = { size = 28, bold = true, before = 16, after = 10 },
  h2 = { size = 22, bold = true, before = 14, after = 8 },
  h3 = { size = 18, bold = true, before = 12, after = 6 },
  h4 = { size = 16, bold = true, before = 10, after = 6 },
  h5 = { size = 15, bold = true, before = 8, after = 6 },
  h6 = { size = 14, bold = true, before = 8, after = 6, color = "#555555" },
  quote = { size = 15, italic = true, indent = 20, after = 10, color = "#4a4a4a" },
  ul = { size = 15, after = 4 },
  ol = { size = 15, after = 4 },
  code = { size = 13, face = "mono", pad = 10, after = 12 },
  hr = { after = 0 },
  image = { after = 12 },
  table = { size = 14, pad = 6, after = 12 },
  toc = { size = 15, after = 12 },
}

M.LIST_INDENT, M.LEVEL_INDENT = 28, 24
M.MAX_IMAGE_HEIGHT = 520

function M.lineHeight(size)
  return math.floor(size * 1.5 + 0.5)
end

-- scaled is styles with every size and space in them times k.
local scaledCache = {}
local function scaled(k)
  if k == 1 then return M.styles end
  local s = scaledCache[k]
  if s then return s end
  s = {}
  for name, st in pairs(M.styles) do
    local c = {}
    for key, v in pairs(st) do
      c[key] = type(v) == "number" and math.floor(v * k + 0.5) or v
    end
    s[name] = c
  end
  scaledCache[k] = s
  return s
end
M.scaled = scaled

-- fontOf is the font a run is drawn in, in a block of style st.
function M.fontOf(st, run)
  local size = st.size or 15
  if run and run.code then
    return { face = "mono", size = size - 2, bold = st.bold or run.b, italic = run.i }
  end
  return { face = st.face or "sans", size = size, bold = st.bold or (run and run.b) or false,
    italic = st.italic or (run and run.i) or false }
end

-- pieces cuts a run's text into what lines break between: words with the
-- spaces after them, spaces at the start, and line breaks on their own.
local function pieces(text)
  local out = {}
  local i = 1
  while i <= #text do
    local c = text:sub(i, i)
    if c == "\n" then
      out[#out + 1] = { text = "\n", o = i - 1 }
      i = i + 1
    else
      local s, e = text:find("^[^\n%s]*[ \t]*", i)
      if e < i then s, e = text:find("^[ \t]+", i) end
      out[#out + 1] = { text = text:sub(s, e), o = i - 1 }
      i = e + 1
    end
  end
  return out
end

-- chars cuts a piece too wide for a line into pieces that fit.
local function chars(piece, width, measure, font)
  local out, start = {}, 1
  local text = piece.text
  local i = 1
  while i <= #text do
    local j = md.nextChar(text, i - 1) + 1 -- the next character's start
    if measure(text:sub(start, j - 1), font) > width and j - 1 > start then
      out[#out + 1] = { text = text:sub(start, i - 1), o = piece.o + start - 1 }
      start = i
    end
    i = j
  end
  out[#out + 1] = { text = text:sub(start), o = piece.o + start - 1 }
  return out
end

-- breakLines lays a block's runs out in lines no wider than width.
local function breakLines(block, st, width, measure)
  local lines = {}
  local baseH = M.lineHeight(st.size or 15)
  local line = { o1 = 0, frags = {}, w = 0, h = baseH }
  local function close(o2, nextO)
    line.o2 = o2
    -- The pieces were measured one by one, each rounded up; a fragment
    -- measured whole is as wide as it is drawn, so the next one starts
    -- right after it.
    local x = 0
    for _, f in ipairs(line.frags) do
      f.x, f.w = x, measure(f.text, f.font)
      x = x + f.w
    end
    line.w = x
    lines[#lines + 1] = line
    line = { o1 = nextO, frags = {}, w = 0, h = baseH }
  end
  local function add(piece, run, font, w)
    local last = line.frags[#line.frags]
    if last and last.run == run and last.o + #last.text == piece.o then
      last.text = last.text .. piece.text
      last.w = last.w + w
    else
      line.frags[#line.frags + 1] = { x = line.w, w = w, text = piece.text, o = piece.o, run = run, font = font }
    end
    line.w = line.w + w
    line.h = math.max(line.h, M.lineHeight(font.size))
  end
  local at = 0
  for _, run in ipairs(block.runs) do
    local font = M.fontOf(st, run)
    for _, piece in ipairs(pieces(run.text)) do
      piece.o = piece.o + at
      if piece.text == "\n" then
        close(piece.o, piece.o + 1)
      else
        local w = measure(piece.text, font)
        local bare = measure((piece.text:gsub("%s+$", "")), font)
        if line.w > 0 and line.w + bare > width then close(piece.o, piece.o) end
        if bare > width then
          for k, part in ipairs(chars(piece, width, measure, font)) do
            if k > 1 then close(part.o, part.o) end
            add(part, run, font, measure(part.text, font))
          end
        else
          add(piece, run, font, w)
        end
      end
    end
    at = at + #run.text
  end
  close(at, at)
  return lines
end

-- tableOf lays out a table in width: each column as wide as its widest
-- cell when they all fit, and otherwise the room shared out by how much
-- more each would like than its longest word, the cells' text wrapped to
-- fit. Its cells' lines are L.subs, each placed in the block.
local function tableOf(L, b, st, width, measure)
  local pad = st.pad
  local ncol = #b.align
  local head = {}
  for k, v in pairs(st) do head[k] = v end
  head.bold = true
  local nat, min = {}, {}
  for k = 1, ncol do nat[k], min[k] = 2 * pad + 8, 2 * pad + 8 end
  for r, row in ipairs(b.rows) do
    local cst = r == 1 and head or st
    for k = 1, ncol do
      local whole, widest = 0, 0
      for _, run in ipairs(row[k] or {}) do
        local font = M.fontOf(cst, run)
        -- Measured a piece at a time, as breakLines measures, so that the
        -- widest cell fits on one line.
        for _, piece in ipairs(pieces(run.text)) do
          whole = whole + measure(piece.text, font)
          widest = math.max(widest, measure((piece.text:gsub("%s+$", "")), font))
        end
      end
      nat[k] = math.max(nat[k], math.ceil(whole) + 2 * pad)
      min[k] = math.max(min[k], math.ceil(widest) + 2 * pad)
    end
  end
  local sumNat, sumMin = 0, 0
  for k = 1, ncol do sumNat, sumMin = sumNat + nat[k], sumMin + min[k] end
  local widths = {}
  for k = 1, ncol do
    if sumNat <= width then
      widths[k] = nat[k]
    elseif sumMin >= width then
      widths[k] = math.max(2 * pad + 8, math.floor(min[k] * width / sumMin))
    else
      local slack = sumNat - sumMin
      widths[k] = min[k] + math.floor((width - sumMin) * (nat[k] - min[k]) / slack)
    end
  end
  local colX, x = {}, 0
  for k = 1, ncol do colX[k], x = x, x + widths[k] end
  L.colX, L.colW, L.tableW, L.rows, L.subs = colX, widths, x, {}, {}
  local y = 0
  for r, row in ipairs(b.rows) do
    local cst = r == 1 and head or st
    local cells, rowH = {}, 0
    for k = 1, ncol do
      local lines = breakLines({ runs = row[k] or {} }, cst, math.max(8, widths[k] - 2 * pad), measure)
      local h = 0
      for _, line in ipairs(lines) do h = h + line.h end
      cells[k] = lines
      rowH = math.max(rowH, h + 2 * pad)
    end
    for k = 1, ncol do
      local cw, ly = widths[k] - 2 * pad, y + pad
      for _, line in ipairs(cells[k]) do
        local a = b.align[k]
        local ax = a == "center" and math.floor((cw - line.w) / 2) or a == "right" and cw - line.w or 0
        L.subs[#L.subs + 1] = { x = colX[k] + pad + math.max(0, ax), y = ly, line = line }
        ly = ly + line.h
      end
    end
    L.rows[r] = { y = y, h = rowH }
    y = y + rowH
  end
  return x, y
end

-- tocOf lays out a list of the document's headings, each a link to it,
-- indented by its level.
local function tocOf(L, blocks, st, width, measure, px)
  local hs = md.headings(blocks)
  local top = math.huge
  for _, h in ipairs(hs) do top = math.min(top, h.level) end
  L.subs = {}
  local y = 0
  for _, h in ipairs(hs) do
    local indent = px((h.level - top) * M.LEVEL_INDENT)
    local block = { runs = { { text = h.text, link = "#" .. h.anchor } } }
    for _, line in ipairs(breakLines(block, st, math.max(40, width - indent), measure)) do
      L.subs[#L.subs + 1] = { x = indent, y = y, line = line }
      y = y + line.h
    end
  end
  if #hs == 0 then
    L.empty = true
    y = M.lineHeight(st.size)
  end
  return y
end

local Layout = {}
Layout.__index = Layout

-- keyOf is what a text block's lines depend on: its kind, its text and
-- styles, the width they are broken to fit, and the scale.
local function keyOf(b, width, k)
  local t = { b.type, b.level or 0, width, k }
  for _, r in ipairs(b.runs) do
    t[#t + 1] = r.text .. "\1" .. (r.b and "b" or "") .. (r.i and "i" or "") .. (r.code and "c" or "") .. (r.link or "")
  end
  return table.concat(t, "\2")
end

-- build lays the blocks out. cache, when given, keeps each text block's
-- lines from one build to the next, so that typing in one paragraph does
-- not break all the others into lines again.
function M.build(blocks, width, measure, imageSize, cache, opts)
  local k = opts and opts.scale or 1
  local styles = scaled(k)
  local function px(v) return math.floor(v * k + 0.5) end
  local lay = setmetatable({ blocks = {}, width = width, measure = measure, scale = k }, Layout)
  local y = 0
  local counters, kinds = {}, {}
  for i, b in ipairs(blocks) do
    local st = styles[b.type] or styles.p
    local nextB = blocks[i + 1]
    local L = { block = b, style = st, x = 0 }
    local isList = b.type == "ul" or b.type == "ol"
    if not isList then counters, kinds = {}, {} end
    y = y + (i > 1 and (st.before or 0) or 0)
    L.y = y
    if b.type == "table" then
      local w, h = tableOf(L, b, st, width, measure)
      L.h, L.contentH = h + st.after, h
      L.lines = { { y = 0, h = h, o1 = 0, o2 = 1, frags = {}, object = true, w = w } }
    elseif b.type == "toc" then
      local h = tocOf(L, blocks, st, width, measure, px)
      L.h, L.contentH = h + st.after, h
      L.lines = { { y = 0, h = h, o1 = 0, o2 = 1, frags = {}, object = true, w = width } }
    elseif b.type == "hr" then
      L.h, L.contentH = px(25), px(25)
      L.lines = { { y = 0, h = L.h, o1 = 0, o2 = 1, frags = {}, object = true, w = width } }
    elseif b.type == "image" then
      local iw, ih = 0, 0
      if imageSize then iw, ih = imageSize(b.src) end
      local dw, dh
      if iw and iw > 0 then
        local s = math.min(k, width / iw, px(M.MAX_IMAGE_HEIGHT) / ih)
        dw, dh = math.max(1, math.floor(iw * s)), math.max(1, math.floor(ih * s))
      else
        dw, dh = math.min(width, px(240)), px(60) -- a missing picture's placeholder
        L.missing = true
      end
      L.imageW, L.imageH = dw, dh
      L.h, L.contentH = dh + st.after, dh
      L.lines = { { y = 0, h = dh, o1 = 0, o2 = 1, frags = {}, object = true, w = dw } }
    else
      local indent, pad = 0, 0
      if isList then
        local level = b.level or 0
        indent = px(M.LIST_INDENT + level * M.LEVEL_INDENT)
        for l = level + 1, 3 do counters[l], kinds[l] = nil, nil end
        if b.type == "ol" then
          counters[level] = kinds[level] == "ol" and counters[level] + 1 or 1
          L.marker = counters[level] .. "."
        else
          L.marker = ({ "•", "◦", "▪", "•" })[level + 1]
        end
        kinds[level] = b.type
      elseif b.type == "quote" then
        indent = st.indent
      elseif b.type == "code" then
        indent, pad = st.pad, st.pad
      end
      L.x, L.pad = indent, pad
      local fit = math.max(40, width - indent - pad)
      local key = cache and keyOf(b, fit, k)
      L.lines = key and cache[key]
      if not L.lines then
        L.lines = breakLines(b, st, fit, measure)
        if key then
          if (cache.n or 0) > 5000 then for c in pairs(cache) do cache[c] = nil end end
          cache[key] = L.lines
          cache.n = (cache.n or 0) + 1
        end
      end
      local ly = pad
      for _, line in ipairs(L.lines) do
        line.y = ly
        ly = ly + line.h
      end
      local after = st.after
      -- The last item of a list, or a quote before something else, gets
      -- a paragraph's space after it.
      if isList and not (nextB and (nextB.type == "ul" or nextB.type == "ol")) then after = px(10) end
      if b.type == "quote" and nextB and nextB.type == "quote" then after = px(6) end
      L.contentH = ly + pad
      L.h = L.contentH + after
    end
    lay.blocks[i] = L
    y = y + L.h
  end
  lay.height = y
  return lay
end

---------------------------------------------------------------- positions

-- lineAt is the line of block L that offset o is on: a line ends before
-- the next begins, so an offset where a line wraps is on the next one.
local function lineAt(L, o)
  for k, line in ipairs(L.lines) do
    if o < line.o2 or (o == line.o2 and (k == #L.lines or L.lines[k + 1].o1 > o)) then
      return line, k
    end
  end
  return L.lines[#L.lines], #L.lines
end

-- xAt is how far along its line offset o is.
local function xAt(lay, L, line, o)
  if line.object then return o == 0 and 0 or line.w end
  for _, f in ipairs(line.frags) do
    if o >= f.o and o <= f.o + #f.text then
      return f.x + lay.measure(f.text:sub(1, o - f.o), f.font)
    end
  end
  if #line.frags > 0 and o >= line.o2 then
    local f = line.frags[#line.frags]
    return f.x + f.w
  end
  return 0
end

-- caret is where the caret at p is drawn: x, y and height on the page.
function Layout:caret(p)
  local L = self.blocks[p.b]
  if not L then return 0, 0, 16 end
  local line = lineAt(L, p.o)
  local x = L.x + xAt(self, L, line, p.o)
  if line.object then x = L.x + (p.o == 0 and -3 or line.w + 2) end
  return x, L.y + line.y, line.h
end

-- offsetIn is the offset on a line nearest to x along it.
local function offsetIn(lay, line, x)
  if line.object then return x < line.w / 2 and 0 or 1 end
  local best, bestD = line.o1, math.huge
  for _, f in ipairs(line.frags) do
    if x >= f.x - 1 and x <= f.x + f.w + 1 or f == line.frags[#line.frags] or f == line.frags[1] then
      local o = 0
      while true do
        local px = f.x + lay.measure(f.text:sub(1, o), f.font)
        local d = math.abs(px - x)
        if d < bestD then best, bestD = f.o + o, d end
        if o >= #f.text then break end
        o = md.nextChar(f.text, o)
      end
    end
  end
  -- A wrapped line's last space is the next line's start, not this one's.
  if best >= line.o2 and line.o2 > line.o1 and best > line.o1 then
    local text = ""
    for _, f in ipairs(line.frags) do text = text .. f.text end
    if text:sub(-1) == " " then best = md.prevChar(text, best - line.o1) + line.o1 end
  end
  return best
end

-- lineOfPoint is the block and the line at y on the page, the nearest
-- when y is in the space between them.
function Layout:lineOfPoint(y)
  local n = #self.blocks
  for i, L in ipairs(self.blocks) do
    if y < L.y + L.h or i == n then
      local line = L.lines[#L.lines]
      for _, ln in ipairs(L.lines) do
        if y < L.y + ln.y + ln.h then line = ln break end
      end
      return i, line
    end
  end
end

-- hit is the position at a point on the page.
function Layout:hit(x, y)
  local n = #self.blocks
  if n == 0 then return { b = 1, o = 0 } end
  if y < 0 then return { b = 1, o = 0 } end
  local i, line = self:lineOfPoint(y)
  return { b = i, o = offsetIn(self, line, x - self.blocks[i].x) }
end

-- linkAt is the address of the link drawn at a point on the page, if one
-- is there, and the position in the text there.
function Layout:linkAt(x, y)
  if #self.blocks == 0 or y < 0 or y >= self.height then return nil end
  local i, line = self:lineOfPoint(y)
  local L = self.blocks[i]
  if y < L.y + line.y or y >= L.y + line.y + line.h then return nil end
  local lx = x - L.x
  for _, f in ipairs(line.frags) do
    if f.run.link and lx >= f.x and lx < f.x + f.w then
      return f.run.link, { b = i, o = offsetIn(self, line, lx) }
    end
  end
  -- The lines inside a table's cells, or a list of contents.
  local ly = y - L.y
  for _, sub in ipairs(L.subs or {}) do
    if ly >= sub.y and ly < sub.y + sub.line.h then
      for _, f in ipairs(sub.line.frags) do
        if f.run.link and lx >= sub.x + f.x and lx < sub.x + f.x + f.w then
          return f.run.link, { b = i, o = 0 }
        end
      end
    end
  end
  return nil
end

-- vertical goes a line up (-1) or down (1), or a page's height when given,
-- keeping to the column it started from (goal). It returns the position
-- and the column, for the next move.
function Layout:vertical(p, dir, goal, height)
  local x, y, h = self:caret(p)
  goal = goal or x
  local ty = dir < 0 and y - (height or 1) - 1 or y + h + (height or 1) - 1
  if height then ty = dir < 0 and y - height or y + height end
  if ty < 0 then return { b = 1, o = 0 }, goal end
  if ty >= self.height then
    local last = #self.blocks
    return { b = last, o = md.length(self.blocks[last].block) }, goal
  end
  local q = self:hit(goal, ty)
  -- Landing on the line it started from means ty fell in the space after
  -- a block: go on to the next block's first line, or the last one's.
  local _, qy = self:caret(q)
  if qy == y and q.b == p.b then
    local other = self.blocks[p.b + dir]
    if not other then return q, goal end
    local oy = dir > 0 and other.y + 1 or other.y + other.contentH - 1
    q = self:hit(goal, oy)
  end
  return q, goal
end

function Layout:lineStart(p)
  local L = self.blocks[p.b]
  local line = lineAt(L, p.o)
  return { b = p.b, o = line.o1 }
end

function Layout:lineEnd(p)
  local L = self.blocks[p.b]
  local line, k = lineAt(L, p.o)
  local o = line.o2
  -- A wrapped line ends before the space it wrapped at.
  if k < #L.lines and o > line.o1 and L.lines[k + 1].o1 == o then
    local text = md.textOf(L.block)
    if text:sub(o, o) == " " then o = o - 1 end
  end
  return { b = p.b, o = o }
end

-- rects are the rectangles a selection from s to e covers, to shade.
function Layout:rects(s, e)
  local out = {}
  for i = s.b, e.b do
    local L = self.blocks[i]
    for _, line in ipairs(L.lines) do
      local a = i == s.b and math.max(s.o, line.o1) or line.o1
      local z = i == e.b and math.min(e.o, line.o2) or line.o2
      local wholeLine = (i ~= e.b) or (e.o > line.o2)
      if line.object then
        -- A table or a list of contents is not shaded: it draws its own
        -- backgrounds, which would show the shade in stripes.
        local shaded = L.block.type ~= "table" and L.block.type ~= "toc"
        if shaded and (i > s.b or s.o == 0) and (i < e.b or e.o == 1) then
          out[#out + 1] = { L.x, L.y + line.y, line.w, line.h }
        end
      elseif z > a or (wholeLine and a <= line.o2 and z >= a) then
        local x1 = xAt(self, L, line, a)
        local x2 = xAt(self, L, line, z)
        if wholeLine then x2 = x2 + 6 end -- the line break, shown as a little space
        if x2 > x1 then out[#out + 1] = { L.x + x1, L.y + line.y, x2 - x1, line.h } end
      end
    end
  end
  return out
end

return M
