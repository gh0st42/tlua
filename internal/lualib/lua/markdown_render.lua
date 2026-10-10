-- markdown.render draws what markdown.layout laid out, with the g a gui
-- Canvas's onDraw gets: a MarkdownView draws with it, and so can a control
-- of a program's own, an editor say.
--
--   local render = require "markdown.render"
--   local measurer = render.measurer()      -- layout's measure, cached
--   function canvas:onDraw(g)
--     measurer.g = g
--     local lay = layout.build(blocks, width, measurer.measure, imageSize)
--     for _, L in ipairs(lay.blocks) do render.block(g, L, x0, y0, width, opts) end
--     measurer.g = nil
--   end
--
-- opts: colors (a table like render.colors, in part or whole), image(src)
-- giving the png image a picture block shows (nil when it is missing), and
-- the measurer's measure. Nothing here needs the gui module itself.

local layout = require "tlua.markdown.layout"

local M = {}

M.colors = {
  text = "#1d1d1f", link = "#1a5fb4", codeBg = "#f3f4f6", codeLine = "#dfe2e6",
  inlineCode = "#eef0f3", quoteBar = "#c4c8cf", rule = "#c8c8c8", selection = "#b4d5fe",
  missing = "#808080", tableLine = "#d0d7de", tableHead = "#f3f4f6", tableStripe = "#f8f9fa",
}

-- style is a font's style as g:font takes it.
function M.style(font)
  if font.bold and font.italic then return "bold italic" end
  if font.bold then return "bold" end
  if font.italic then return "italic" end
  return nil
end

function M.font(g, font)
  g:font(font.face, font.size, M.style(font))
end

local function fontKey(font)
  return font.face .. font.size .. (font.bold and "b" or "") .. (font.italic and "i" or "")
end

-- measurer makes the measure a layout is built with. It measures text
-- exactly while its g is set, that is while the Canvas draws, and keeps
-- the width of every character it has seen, to add up when it is asked
-- from a click or a key, when the Canvas cannot measure.
function M.measurer()
  local m = { g = nil, cache = {}, chars = {}, size = 0 }
  local function exact(text, font, key)
    M.font(m.g, font)
    local w = m.g:measure(text)
    m.cache[key .. "\0" .. text] = w
    m.size = m.size + 1
    return w
  end
  function m.measure(text, font)
    if text == "" then return 0 end
    local key = fontKey(font)
    local w = m.cache[key .. "\0" .. text]
    if w then return w end
    if m.size > 20000 then m.cache, m.size = {}, 0 end
    local chars = m.chars[key]
    if not chars then chars = {}; m.chars[key] = chars end
    if m.g then
      for ch in text:gmatch("[%z\1-\127\194-\244][\128-\191]*") do
        if not chars[ch] then chars[ch] = exact(ch, font, key) end
      end
      return exact(text, font, key)
    end
    local sum = 0
    for ch in text:gmatch("[%z\1-\127\194-\244][\128-\191]*") do
      sum = sum + (chars[ch] or font.size * 0.55)
    end
    return sum
  end
  return m
end

-- drawLine draws one line of text, its left at x and its top at y.
local function drawLine(g, line, x, y, st, inCode, opts, color, px)
  local measure = opts and opts.measure
  for _, f in ipairs(line.frags) do
    local text = f.text:gsub("\n", "")
    if text ~= "" then
      local fx = x + f.x
      if f.run.code and not inCode then
        g:color(color("inlineCode"))
        g:fill(fx - 1, y + px(3), f.w + 2, line.h - px(6))
      end
      M.font(g, f.font)
      g:color(f.run.link and color("link") or st.color or color("text"))
      g:text(text, fx, y, f.w + px(20), line.h, "left")
      if f.run.link then
        -- Underlined, but not under the spaces after the words.
        local trail = text:match("%s+$")
        local tw = trail and (measure and measure(trail, f.font) or g:measure(trail)) or 0
        local base = y + math.floor((line.h + f.font.size) / 2)
        g:fill(fx, base, f.w - tw, math.max(1, px(1)))
      end
    end
  end
end

-- block draws one laid-out block, L, of a layout whose text column starts
-- at x0, y0 on the Canvas and is columnW wide.
function M.block(g, L, x0, y0, columnW, opts)
  local colors = opts and opts.colors or M.colors
  local function color(name) return colors[name] or M.colors[name] end
  local k = opts and opts.scale or 1
  local function px(v) return math.floor(v * k + 0.5) end
  local b, st = L.block, L.style
  local x, y = x0 + L.x, y0 + L.y
  if b.type == "hr" then
    g:color(color("rule"))
    g:fill(x0, y + math.floor(L.contentH / 2), columnW, math.max(1, px(1)))
    return
  end
  if b.type == "image" then
    local img = opts and opts.image and opts.image(b.src)
    if img then
      g:scaling("smooth")
      g:image(img, x, y, L.imageW, L.imageH)
    else
      g:color("#f0f0f0"); g:fill(x, y, L.imageW, L.imageH)
      g:color("#b0b0b0"); g:rect(x, y, L.imageW, L.imageH)
      g:font("sans", px(12))
      g:color(color("missing"))
      g:text("picture missing: " .. tostring(b.src), x + px(8), y, L.imageW - px(16), L.imageH, "center")
    end
    return
  end
  if b.type == "table" then
    for r, row in ipairs(L.rows) do
      local bg = r == 1 and color("tableHead") or (r % 2 == 1 and color("tableStripe"))
      if bg then g:color(bg); g:fill(x, y + row.y, L.tableW, row.h) end
    end
    g:color(color("tableLine"))
    for r, row in ipairs(L.rows) do
      for k, cx in ipairs(L.colX) do g:rect(x + cx, y + row.y, L.colW[k] + 1, row.h + 1) end
    end
    for _, sub in ipairs(L.subs) do
      drawLine(g, sub.line, x + sub.x, y + sub.y, L.style, false, opts, color, px)
    end
    return
  end
  if b.type == "toc" then
    if L.empty then
      g:font("sans", L.style.size, "italic")
      g:color(color("missing"))
      g:text("no headings to list", x, y, columnW, L.contentH, "left")
    end
    for _, sub in ipairs(L.subs) do
      drawLine(g, sub.line, x + sub.x, y + sub.y, L.style, false, opts, color, px)
    end
    return
  end
  if b.type == "code" then
    g:color(color("codeBg")); g:fill(x0, y, columnW, L.contentH)
    g:color(color("codeLine")); g:rect(x0, y, columnW, L.contentH)
  elseif b.type == "quote" then
    g:color(color("quoteBar")); g:fill(x0 + px(2), y, px(3), L.contentH)
  end
  if L.marker then
    local first = L.lines[1]
    M.font(g, layout.fontOf(st))
    g:color(color("text"))
    g:text(L.marker, x - px(26), y + first.y, px(20), first.h, "right")
  end
  for _, line in ipairs(L.lines) do
    drawLine(g, line, x, y + line.y, st, b.type == "code", opts, color, px)
  end
end

return M
