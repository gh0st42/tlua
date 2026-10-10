-- markdown is tlua's own: Markdown read into blocks and written back, the
-- part of it that a MarkdownView shows and microword edits.
--
--   local md = require "markdown"
--   local blocks = md.parse("# Hello\n\nSome *text* and a [link](page.md).\n")
--   print(md.write(blocks))
--
-- A program's own markdown.lua, if it has one, is found before this one.
--
-- A document is a list of blocks:
--   { type = "p" | "h1" .. "h6" | "quote", runs = {...} }
--   { type = "ul" | "ol", level = 0..3, runs = {...} }      a list item
--   { type = "code", lang = "lua", runs = { { text = "..." } } }
--   { type = "hr" }
--   { type = "image", src = "assets/cat.png", alt = "a cat" }
--   { type = "table", align = { "left", "center", "right" or false ... },
--     rows = { { runs, runs, ... }, ... } }   the first row is the header
--   { type = "toc" }                          [[toc]]: the headings, listed
--
-- runs are the block's text, a piece at a time, each with its style:
--   { text = "word", b = true, i = true, code = true, link = "https://..." }
-- A "\n" in a run's text is a line break inside the block. A wiki link,
-- [[Page Name]] or [[Page Name|what it says]], is a link to "Page Name.md"
-- with wiki = "Page Name", so that it is written back as it was.
--
-- write(parse(text)) gives back text in a steady form; what it writes it
-- reads back exactly.

local M = {}

---------------------------------------------------------------- the text of blocks

-- A position in a document is { b = block, o = offset }: the offset is in
-- bytes of the block's text, at a character's start. A picture or a rule
-- counts as one character.

local OBJECTS = { image = true, hr = true, table = true, toc = true }

-- hasText says whether a block holds text: all but pictures, rules,
-- tables and lists of contents, which a position steps over as one.
function M.hasText(b) return not OBJECTS[b.type] end

-- textOf is a block's text, its styles left out.
function M.textOf(b)
  if not M.hasText(b) then return "" end
  local t = {}
  for i, r in ipairs(b.runs) do t[i] = r.text end
  return table.concat(t)
end

-- length is how many offsets a block has: its text's bytes, or 1.
function M.length(b)
  if not M.hasText(b) then return 1 end
  local n = 0
  for _, r in ipairs(b.runs) do n = n + #r.text end
  return n
end

-- prevChar and nextChar step over one UTF-8 character from offset o.
function M.prevChar(text, o)
  if o <= 0 then return 0 end
  o = o - 1
  while o > 0 and text:byte(o + 1) and text:byte(o + 1) >= 0x80 and text:byte(o + 1) < 0xC0 do o = o - 1 end
  return o
end

function M.nextChar(text, o)
  if o >= #text then return #text end
  o = o + 1
  while o < #text and text:byte(o + 1) >= 0x80 and text:byte(o + 1) < 0xC0 do o = o + 1 end
  return o
end

-- anchor is the name a heading is linked to by, as GitHub makes them:
-- "## Getting Started!" is #getting-started.
function M.anchor(text)
  return (text:lower():gsub("[^%w%s_%-\128-\255]", ""):gsub("%s", "-"))
end

-- headings lists a document's headings: level, text, the anchor a link to
-- it names (a second heading of the same name gets -1 after it, as on
-- GitHub), and the block it is.
function M.headings(blocks)
  local out, seen = {}, {}
  for i, b in ipairs(blocks) do
    local digit = b.type:match("^h(%d)$")
    if digit then
      local text = M.textOf(b):gsub("\n", " ")
      local anchor = M.anchor(text)
      if seen[anchor] then
        seen[anchor] = seen[anchor] + 1
        anchor = anchor .. "-" .. seen[anchor]
      else
        seen[anchor] = 0
      end
      out[#out + 1] = { level = tonumber(digit), text = text, anchor = anchor, block = i }
    end
  end
  return out
end

-- wikiLink is where [[target]] leads: a page, Markdown unless it says
-- otherwise, beside the one it is on, and a heading on it after #.
function M.wikiLink(target)
  local page, frag = target:match("^([^#]*)(#?.*)$")
  if page ~= "" and not page:match("%.%w+$") then page = page .. ".md" end
  return page .. frag
end

---------------------------------------------------------------- inline

local function sameStyle(a, b)
  return (a.b or false) == (b.b or false) and (a.i or false) == (b.i or false)
    and (a.code or false) == (b.code or false) and a.link == b.link and a.wiki == b.wiki
end
M.sameStyle = sameStyle

-- normalize joins runs of the same style and drops empty ones, in place.
function M.normalize(runs)
  local out = {}
  for _, r in ipairs(runs) do
    if r.text ~= "" then
      local last = out[#out]
      if last and sameStyle(last, r) then
        last.text = last.text .. r.text
      else
        out[#out + 1] = { text = r.text, b = r.b or nil, i = r.i or nil, code = r.code or nil, link = r.link, wiki = r.wiki }
      end
    end
  end
  for i = #runs, 1, -1 do runs[i] = nil end
  for i, r in ipairs(out) do runs[i] = r end
  return runs
end

local function styled(base, extra)
  local s = { b = base.b, i = base.i, code = base.code, link = base.link, wiki = base.wiki }
  for k, v in pairs(extra) do s[k] = v end
  return s
end

local PUNCT = "[%!%\"%#%$%%%&%'%(%)%*%+%,%-%.%/%:%;%<%=%>%?%@%[%\\%]%^%_%`%{%|%}%~]"

-- opens says whether a mark of n characters at i can open emphasis: it
-- must be followed by something other than a space. closer finds where
-- one opened before from can close: after something other than a space.
local function opens(s, i, n)
  local after = s:sub(i + n, i + n)
  return after ~= "" and not after:match("%s")
end

local function closer(s, mark, from)
  if not opens(s, from - #mark, #mark) then return nil end
  local j = from
  while true do
    j = s:find(mark, j, true)
    if not j then return nil end
    if j > from and not s:sub(j - 1, j - 1):match("%s") then return j end
    j = j + 1
  end
end

-- inline reads a line's text into runs, in style base.
local function inline(s, base, out)
  out = out or {}
  local buf = {}
  local function flush()
    if #buf > 0 then
      local r = styled(base, {})
      r.text = table.concat(buf)
      out[#out + 1] = r
      buf = {}
    end
  end
  local i, n = 1, #s
  while i <= n do
    local c = s:sub(i, i)
    local two, three = s:sub(i, i + 1), s:sub(i, i + 2)
    if c == "\\" and i < n and s:sub(i + 1, i + 1):match(PUNCT) then
      buf[#buf + 1] = s:sub(i + 1, i + 1)
      i = i + 2
    elseif c == "\\" and i == n then
      buf[#buf + 1] = "\n" -- a backslash ending a line is a line break
      i = i + 1
    elseif c == "`" then
      local ticks = s:match("^`+", i)
      local j = s:find(ticks, i + #ticks, true)
      if j then
        flush()
        local code = s:sub(i + #ticks, j - 1)
        if #ticks > 1 then code = code:gsub("^ ", ""):gsub(" $", "") end
        local r = styled(base, { code = true })
        r.text = code
        out[#out + 1] = r
        i = j + #ticks
      else
        buf[#buf + 1] = ticks
        i = i + #ticks
      end
    elseif (three == "***" or three == "___") and closer(s, three, i + 3) then
      local j = closer(s, three, i + 3)
      flush()
      inline(s:sub(i + 3, j - 1), styled(base, { b = true, i = true }), out)
      i = j + 3
    elseif (two == "**" or two == "__") and closer(s, two, i + 2) then
      local j = closer(s, two, i + 2)
      flush()
      inline(s:sub(i + 2, j - 1), styled(base, { b = true }), out)
      i = j + 2
    elseif (c == "*" or (c == "_" and not s:sub(i - 1, i - 1):match("%w"))) then
      -- The closing one: the same mark, not doubled, not inside a word for _.
      local j = i + 1
      local found
      while true do
        j = s:find(c, j, true)
        if not j then break end
        if s:sub(j + 1, j + 1) == c then
          j = j + 2
        elseif c == "_" and s:sub(j + 1, j + 1):match("%w") then
          j = j + 1
        else
          found = j
          break
        end
      end
      if found and found > i + 1 and opens(s, i, 1) and not s:sub(found - 1, found - 1):match("%s") then
        flush()
        inline(s:sub(i + 1, found - 1), styled(base, { i = true }), out)
        i = found + 1
      else
        buf[#buf + 1] = c
        i = i + 1
      end
    elseif two == "[[" and s:match("^%[%[[^%[%]\n]+%]%]", i) then
      -- [[Page]] or [[Page|what it says]]: a wiki link.
      local inner = s:match("^%[%[([^%[%]\n]+)%]%]", i)
      local target, label = inner:match("^(.-)|(.*)$")
      target = target or inner
      flush()
      local r = styled(base, { link = M.wikiLink(target), wiki = target })
      r.text = label or target
      out[#out + 1] = r
      i = i + #inner + 4
    elseif c == "[" then
      -- [text](url): the brackets may nest one level.
      local depth, j = 0, i
      while j <= n do
        local d = s:sub(j, j)
        if d == "\\" then j = j + 1
        elseif d == "[" then depth = depth + 1
        elseif d == "]" then
          depth = depth - 1
          if depth == 0 then break end
        end
        j = j + 1
      end
      local url, k = nil, nil
      if j <= n and s:sub(j + 1, j + 1) == "(" then
        k = s:find(")", j + 2, true)
        if k then url = s:sub(j + 2, k - 1):gsub("%s+\".*\"$", "") end
      end
      if url then
        flush()
        inline(s:sub(i + 1, j - 1), styled(base, { link = url }), out)
        i = k + 1
      else
        buf[#buf + 1] = c
        i = i + 1
      end
    elseif c == "<" and s:match("^<(%a[%w+.-]*:[^>%s]+)>", i) then
      local url = s:match("^<(%a[%w+.-]*:[^>%s]+)>", i)
      flush()
      local r = styled(base, { link = url })
      r.text = url
      out[#out + 1] = r
      i = i + #url + 2
    else
      buf[#buf + 1] = c
      i = i + 1
    end
  end
  flush()
  return out
end

function M.inline(s)
  return M.normalize(inline(s, {}))
end

-- escape keeps text from being read as markup.
local function escape(text)
  return (text:gsub("[\\`%*_%[%]<]", "\\%0"))
end

-- writeRuns writes runs as inline Markdown. Each run carries its own marks,
-- so a run's style never leaks into the next.
local function writeRuns(runs)
  local out = {}
  local i = 1
  while i <= #runs do
    local r = runs[i]
    if r.wiki then
      -- A wiki link is written back as it was read: its words plain.
      local words = {}
      while runs[i] and runs[i].link == r.link and runs[i].wiki == r.wiki do
        words[#words + 1] = runs[i].text
        i = i + 1
      end
      local text = table.concat(words)
      out[#out + 1] = "[[" .. r.wiki .. (text ~= r.wiki and "|" .. text or "") .. "]]"
    elseif r.link then
      -- A link holds the runs that share its address.
      local inner = {}
      while runs[i] and runs[i].link == r.link and not runs[i].wiki do
        local c = runs[i]
        inner[#inner + 1] = { text = c.text, b = c.b, i = c.i, code = c.code }
        i = i + 1
      end
      out[#out + 1] = "[" .. writeRuns(inner) .. "](" .. r.link:gsub("[%s%)]", function(ch)
        return ("%%%02X"):format(ch:byte())
      end) .. ")"
    else
      local text
      if r.code then
        local ticks = "`"
        while r.text:find(ticks, 1, true) do ticks = ticks .. "`" end
        local pad = (r.text:sub(1, 1) == "`" or r.text:sub(-1) == "`") and " " or ""
        if #ticks > 1 then pad = " " end
        text = ticks .. pad .. r.text .. pad .. ticks
      else
        text = escape(r.text)
      end
      -- Line breaks inside the run are written as a backslash at the end of
      -- the line; marks do not span them, so they are closed and opened.
      local mark = (r.b and r.i) and "***" or r.b and "**" or r.i and "*" or ""
      local pieces = {}
      local first = true
      for piece in (text .. "\n"):gmatch("(.-)\n") do
        if not first then pieces[#pieces + 1] = "\\\n" end
        if piece ~= "" then
          -- Marks go round the words, leaving the spaces outside.
          local lead, body, trail = piece:match("^(%s*)(.-)(%s*)$")
          if body == "" then pieces[#pieces + 1] = piece
          else pieces[#pieces + 1] = lead .. mark .. body .. mark .. trail end
        end
        first = false
      end
      out[#out + 1] = table.concat(pieces)
      i = i + 1
    end
  end
  return table.concat(out)
end

M.writeRuns = writeRuns

---------------------------------------------------------------- blocks

local function isBlank(line) return line:match("^%s*$") ~= nil end

local TOC = "^%s*%[%[[Tt][Oo][Cc]%]%]%s*$"

local function startsBlock(line)
  return line:match("^%s*#+%s") or line:match("^%s*>") or line:match("^%s*[-*+]%s")
    or line:match("^%s*%d+[.)]%s") or line:match("^%s*```") or line:match("^%s*~~~")
    or line:match("^%s*([-*_])%s*%1%s*%1[%s%-%*_]*$") or line:match(TOC)
end

---------------------------------------------------------------- tables

-- cellsOf splits a table row at its pipes, as GitHub does: the pipes at
-- either end are left out, an escaped one, \|, is a pipe in a cell, inside
-- code too, and so is one inside a wiki link, [[Page|text]].
local function cellsOf(line)
  line = line:gsub("^%s*|", "")
  if line:match("|%s*$") and not line:match("\\|%s*$") then line = line:gsub("|%s*$", "") end
  local cells, buf = {}, {}
  local wiki = false
  local i = 1
  while i <= #line do
    local c = line:sub(i, i)
    if c == "\\" and line:sub(i + 1, i + 1) == "|" then
      buf[#buf + 1] = "|"
      i = i + 2
    elseif line:sub(i, i + 1) == "[[" or line:sub(i, i + 1) == "]]" then
      wiki = line:sub(i, i + 1) == "[["
      buf[#buf + 1] = line:sub(i, i + 1)
      i = i + 2
    else
      if c == "|" and not wiki then
        cells[#cells + 1] = table.concat(buf)
        buf = {}
      else
        buf[#buf + 1] = c
      end
      i = i + 1
    end
  end
  cells[#cells + 1] = table.concat(buf)
  for k, cell in ipairs(cells) do cells[k] = cell:match("^%s*(.-)%s*$") end
  return cells
end

-- alignsOf reads a table's delimiter row, |---|:--:|--:|, into how each
-- column is aligned (false where it does not say), or nil when it is not
-- one.
local function alignsOf(line)
  if not line or not line:find("-", 1, true) or not line:match("^[%s|:%-]+$") then return nil end
  local out = {}
  for k, cell in ipairs(cellsOf(line)) do
    if not cell:match("^:?%-+:?$") then return nil end
    local left, right = cell:sub(1, 1) == ":", cell:sub(-1) == ":"
    out[k] = left and right and "center" or right and "right" or left and "left" or false
  end
  return out
end

-- tableAt reads the table that starts at line i, if one does: a header
-- row, a delimiter row with as many cells, and the rows after them, until
-- a blank line or another kind of block. It returns the block and the line
-- after it.
local function tableAt(lines, i)
  local line = lines[i]
  if not line:find("|", 1, true) then return nil end
  local align = alignsOf(lines[i + 1])
  local header = cellsOf(line)
  if not align or #align ~= #header then return nil end
  local rows = {}
  local function row(cells)
    local r = {}
    for k = 1, #align do r[k] = M.inline(cells[k] or "") end
    rows[#rows + 1] = r
  end
  row(header)
  i = i + 2
  while i <= #lines and not isBlank(lines[i]) and lines[i]:find("|", 1, true) and not startsBlock(lines[i]) do
    row(cellsOf(lines[i]))
    i = i + 1
  end
  return { type = "table", align = align, rows = rows }, i
end

-- paragraphText joins a paragraph's lines: a line ending in a backslash or
-- two spaces breaks the line there, any other ending is a space.
local function joinLines(lines)
  local out = {}
  for k, line in ipairs(lines) do
    line = line:gsub("^%s+", "")
    if k < #lines then
      if line:match("  $") then line = line:gsub("%s+$", "") .. "\\\n"
      elseif line:match("\\$") then line = line .. "\n"
      else line = line:gsub("%s+$", "") .. " " end
    else
      line = line:gsub("%s+$", "")
    end
    out[#out + 1] = line
  end
  return table.concat(out)
end

-- runsOf reads a paragraph's joined text, line breaks and all.
local function runsOf(text)
  local runs = {}
  local first = true
  for piece in (text .. "\n"):gmatch("(.-)\n") do
    if not first then runs[#runs + 1] = { text = "\n" } end
    piece = piece:gsub("\\$", "")
    for _, r in ipairs(inline(piece, {})) do runs[#runs + 1] = r end
    first = false
  end
  -- A line break takes the style of what is before it, so it joins that run.
  for k = 2, #runs do
    if runs[k].text == "\n" then
      local p = runs[k - 1]
      runs[k].b, runs[k].i, runs[k].code, runs[k].link, runs[k].wiki = p.b, p.i, p.code, p.link, p.wiki
    end
  end
  return M.normalize(runs)
end

function M.parse(text)
  text = text:gsub("\r\n?", "\n")
  local lines = {}
  for line in (text .. "\n"):gmatch("(.-)\n") do lines[#lines + 1] = line end
  local blocks = {}
  local indents, listGoing = {}, false -- the indents of the list being read
  local i = 1
  while i <= #lines do
    local line = lines[i]
    if not listGoing then indents = {} end
    listGoing = false
    local fence = line:match("^%s*(```+)") or line:match("^%s*(~~~+)")
    if isBlank(line) then
      listGoing = #indents > 0 and lines[i + 1] ~= nil
        and (lines[i + 1]:match("^%s*[-*+]%s") or lines[i + 1]:match("^%s*%d+[.)]%s")) and true or false
      i = i + 1
    elseif line:match(TOC) then
      blocks[#blocks + 1] = { type = "toc" }
      i = i + 1
    elseif tableAt(lines, i) then
      local tbl, nexti = tableAt(lines, i)
      blocks[#blocks + 1] = tbl
      i = nexti
    elseif fence then
      local lang = line:match("^%s*[`~]+%s*([%w_+-]*)") or ""
      local body = {}
      i = i + 1
      while i <= #lines and not lines[i]:match("^%s*" .. fence:gsub("%p", "%%%0") .. "%s*$") do
        body[#body + 1] = lines[i]
        i = i + 1
      end
      i = i + 1
      blocks[#blocks + 1] = { type = "code", lang = lang ~= "" and lang or nil, runs = M.normalize { { text = table.concat(body, "\n") } } }
    elseif line:match("^%s*([-*_])%s*%1%s*%1[%s%-%*_]*$") and not line:match("^%s*[-*+]%s+%S") then
      blocks[#blocks + 1] = { type = "hr" }
      i = i + 1
    elseif line:match("^%s*#+%s") or line:match("^%s*#+$") then
      local hashes, rest = line:match("^%s*(#+)%s*(.-)%s*$")
      rest = rest:gsub("%s+#+$", "")
      local level = math.min(#hashes, 6)
      blocks[#blocks + 1] = { type = "h" .. level, runs = runsOf(rest) }
      i = i + 1
    elseif line:match("^%s*!%[.-%]%(.-%)%s*$") then
      local alt, src = line:match("^%s*!%[(.-)%]%((.-)%)%s*$")
      src = src:gsub("%s+\".*\"$", "")
      blocks[#blocks + 1] = { type = "image", src = src, alt = alt }
      i = i + 1
    elseif line:match("^%s*>") then
      -- Each paragraph of a quote is a quote block.
      local para = {}
      while i <= #lines and lines[i]:match("^%s*>") do
        local inner = lines[i]:gsub("^%s*>%s?", "")
        if isBlank(inner) then
          if #para > 0 then blocks[#blocks + 1] = { type = "quote", runs = runsOf(joinLines(para)) } end
          para = {}
        else
          para[#para + 1] = inner
        end
        i = i + 1
      end
      if #para > 0 then blocks[#blocks + 1] = { type = "quote", runs = runsOf(joinLines(para)) } end
    elseif line:match("^%s*[-*+]%s") or line:match("^%s*%d+[.)]%s") then
      local indent, rest = line:match("^(%s*)[-*+]%s+(.*)$")
      local kind = "ul"
      if not indent then
        indent, rest = line:match("^(%s*)%d+[.)]%s+(.*)$")
        kind = "ol"
      end
      local para = { rest }
      i = i + 1
      -- Lines after it that are indented, and not items, carry it on.
      while i <= #lines and not isBlank(lines[i]) and lines[i]:match("^%s+%S")
        and not lines[i]:match("^%s*[-*+]%s") and not lines[i]:match("^%s*%d+[.)]%s") do
        para[#para + 1] = lines[i]
        i = i + 1
      end
      -- The level is how many items of the list it is indented past.
      local w = #indent:gsub("\t", "    ")
      while #indents > 0 and indents[#indents] > w do indents[#indents] = nil end
      if #indents == 0 or indents[#indents] < w then indents[#indents + 1] = w end
      local level = math.min(3, #indents - 1)
      blocks[#blocks + 1] = { type = kind, level = level, runs = runsOf(joinLines(para)) }
      listGoing = true
    else
      local para = {}
      while i <= #lines and not isBlank(lines[i]) and (#para == 0 or not startsBlock(lines[i])) do
        para[#para + 1] = lines[i]
        i = i + 1
      end
      blocks[#blocks + 1] = { type = "p", runs = runsOf(joinLines(para)) }
    end
  end
  return blocks
end

-- guard keeps a line of text from being read as the start of another kind
-- of block. escape has taken care of the marks inside it.
local function guard(line)
  if line:match("^%s*%d+[.)]%s") or line:match("^%s*%d+[.)]$") then
    return (line:gsub("^(%s*%d+)([.)])", "%1\\%2"))
  end
  if line:match("^%s*[#>]") or line:match("^%s*[-+]%s") or line:match("^%s*[-+]$")
    or line:match("^%s*~~~") or line:match("^%s*%-%s*%-%s*%-") then
    return (line:gsub("^(%s*)", "%1\\", 1))
  end
  return line
end

-- lines splits a block's written text at its line breaks, guarding each.
local function guarded(text)
  local out = {}
  for line in (text .. "\n"):gmatch("(.-)\n") do out[#out + 1] = guard(line) end
  return out
end

function M.write(blocks)
  local out = {}
  local prev
  local counters, kinds = {}, {}
  -- Markdown has no empty paragraph: they are left out.
  local written = {}
  for _, b in ipairs(blocks) do
    if not (b.type == "p" and #(b.runs or {}) == 0) then written[#written + 1] = b end
  end
  for _, b in ipairs(written) do
    local t = b.type
    local list = t == "ul" or t == "ol"
    local prevList = prev and (prev.type == "ul" or prev.type == "ol")
    -- What goes between blocks: nothing between the items of a list, a
    -- ">" between the paragraphs of a quote, a blank line otherwise.
    if prev then
      if list and prevList then -- nothing
      elseif t == "quote" and prev.type == "quote" then out[#out + 1] = ">"
      else out[#out + 1] = "" end
    end
    if not list then counters, kinds = {}, {} end
    if t == "code" then
      local text = (b.runs and b.runs[1] and b.runs[1].text) or ""
      local fence = "```"
      while text:find(fence, 1, true) do fence = fence .. "`" end
      out[#out + 1] = fence .. (b.lang or "")
      if text ~= "" then out[#out + 1] = text end
      out[#out + 1] = fence
    elseif t == "hr" then
      out[#out + 1] = "---"
    elseif t == "toc" then
      out[#out + 1] = "[[toc]]"
    elseif t == "table" then
      local DELIMS = { left = ":---", center = ":---:", right = "---:" }
      for r, row in ipairs(b.rows) do
        local cells = {}
        for k = 1, #b.align do
          -- A cell is one line, and its pipes are escaped.
          local text = writeRuns(row[k] or {}):gsub("\\\n", " "):gsub("\n", " ")
          -- Pipes are escaped, but for the one in a wiki link.
          text = text:gsub("(%[%[[^%]]-%]%])", function(w) return (w:gsub("|", "\0")) end)
            :gsub("|", "\\|"):gsub("%z", "|")
          cells[k] = text
        end
        out[#out + 1] = "| " .. table.concat(cells, " | ") .. " |"
        if r == 1 then
          local d = {}
          for k, a in ipairs(b.align) do d[k] = DELIMS[a] or "---" end
          out[#out + 1] = "| " .. table.concat(d, " | ") .. " |"
        end
      end
    elseif t == "image" then
      out[#out + 1] = "![" .. escape(b.alt or "") .. "](" .. (b.src or ""):gsub("[%s%)]", function(ch)
        return ("%%%02X"):format(ch:byte())
      end) .. ")"
    else
      local lines = guarded(writeRuns(b.runs))
      local h = t:match("^h(%d)$")
      if h then
        out[#out + 1] = ("#"):rep(tonumber(h)) .. " " .. table.concat(lines, " "):gsub("\\$", "")
      elseif t == "quote" then
        for _, line in ipairs(lines) do out[#out + 1] = "> " .. line end
      elseif list then
        local level = b.level or 0
        for l = level + 1, 3 do counters[l], kinds[l] = nil, nil end
        local marker = "-"
        if t == "ol" then
          counters[level] = kinds[level] == "ol" and counters[level] + 1 or 1
          marker = counters[level] .. "."
        end
        kinds[level] = t
        local indent = ("    "):rep(level)
        for k, line in ipairs(lines) do
          out[#out + 1] = (k == 1 and indent .. marker .. " " or indent .. "    ") .. line
        end
      else
        for _, line in ipairs(lines) do out[#out + 1] = line end
      end
    end
    prev = b
  end
  return table.concat(out, "\n") .. "\n"
end

return M
