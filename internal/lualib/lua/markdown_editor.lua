-- markdown.editor is tlua's own: a Markdown document being edited, and
-- everything that edits it, with no GUI in it: typing, Enter and Backspace,
-- styles, the kinds of block, lists, links, pictures, the clipboard, and
-- undo. The gui's MarkdownEdit draws what is here and turns keys and
-- clicks into these calls; a program can drive it the same way.
--
--   local editor = require "markdown.editor"
--   local e = editor.new(require("markdown").parse("# Notes\n"))
--   e:moveToDocument(1)
--   e:type("!")
--   print(e:markdown())          --> # Notes!
--
-- A position is { b = block, o = offset }: the offset is in bytes of the
-- block's text (always at a character's start). A picture, a rule, a table
-- or a list of contents counts as one character, so the caret can stand
-- before it (0) or after it (1). The selection runs from anchor to caret.
--
-- Moving up and down a line, or to a line's start or end, needs to know
-- where the lines break; those calls take the layout (markdown.layout).
-- The document is the markdown module's.

local md = require "tlua.markdown"

local M = {}

M.TYPES = { "p", "h1", "h2", "h3", "quote", "code", "ul", "ol" }

-- A block's text, and stepping over its characters, are the markdown
-- module's.
local hasText = md.hasText
M.hasText, M.textOf, M.length = md.hasText, md.textOf, md.length

local function copyRun(r, text)
  return { text = text or r.text, b = r.b, i = r.i, code = r.code, link = r.link, wiki = r.wiki }
end

local function copyBlock(b)
  local c = {}
  for k, v in pairs(b) do c[k] = v end
  if b.runs then
    c.runs = {}
    for i, r in ipairs(b.runs) do c.runs[i] = copyRun(r) end
  end
  if b.rows then
    c.rows, c.align = {}, {}
    for k, a in ipairs(b.align) do c.align[k] = a end
    for r, row in ipairs(b.rows) do
      c.rows[r] = {}
      for k, cell in ipairs(row) do
        c.rows[r][k] = {}
        for i, run in ipairs(cell) do c.rows[r][k][i] = copyRun(run) end
      end
    end
  end
  return c
end
M.copyBlock = copyBlock

-- split cuts runs at byte o into two new lists.
local function split(runs, o)
  local left, right, at = {}, {}, 0
  for _, r in ipairs(runs) do
    local n = #r.text
    if at + n <= o then left[#left + 1] = copyRun(r)
    elseif at >= o then right[#right + 1] = copyRun(r)
    else
      left[#left + 1] = copyRun(r, r.text:sub(1, o - at))
      right[#right + 1] = copyRun(r, r.text:sub(o - at + 1))
    end
    at = at + n
  end
  return left, right
end

local function concat(...)
  local out = {}
  for _, list in ipairs { ... } do
    for _, r in ipairs(list) do out[#out + 1] = r end
  end
  return md.normalize(out)
end

-- plain strips the styles off runs, for a code block.
local function plain(runs)
  local t = {}
  for i, r in ipairs(runs) do t[i] = r.text end
  return md.normalize { { text = table.concat(t) } }
end

local function newParagraph(runs)
  return { type = "p", runs = runs or {} }
end

-- UTF-8: the start of the character before or after byte offset o.
local prevChar, nextChar = md.prevChar, md.nextChar
M.prevChar, M.nextChar = prevChar, nextChar

local function isWordByte(c)
  return c and (c >= 128 or string.char(c):match("[%w_]") ~= nil)
end

local function wordLeft(text, o)
  while o > 0 and not isWordByte(text:byte(o)) do o = o - 1 end
  while o > 0 and isWordByte(text:byte(o)) do o = o - 1 end
  return o
end

local function wordRight(text, o)
  while o < #text and not isWordByte(text:byte(o + 1)) do o = o + 1 end
  while o < #text and isWordByte(text:byte(o + 1)) do o = o + 1 end
  return o
end

local function before(p, q) return p.b < q.b or (p.b == q.b and p.o < q.o) end
local function same(p, q) return p.b == q.b and p.o == q.o end
M.before = before

---------------------------------------------------------------- the editor

local Editor = {}
Editor.__index = Editor

function M.new(blocks)
  local e = setmetatable({}, Editor)
  e:load(blocks or {})
  return e
end

function Editor:load(blocks)
  if #blocks == 0 then blocks = { newParagraph() } end
  self.blocks = blocks
  self.caret = { b = 1, o = 0 }
  self.anchor = { b = 1, o = 0 }
  self.pending, self.cell = nil, nil
  self.undoStack, self.redoStack = {}, {}
  self.lastKind = nil
  self.dirty = false
end

function Editor:markdown()
  return md.write(self.blocks)
end

function Editor:block(i) return self.blocks[i or self.caret.b] end

function Editor:hasSelection() return not same(self.anchor, self.caret) end

-- range is the selection, start first.
function Editor:range()
  if before(self.caret, self.anchor) then return self.caret, self.anchor end
  return self.anchor, self.caret
end

function Editor:setCaret(p, extend)
  self.cell = nil
  self.caret = { b = p.b, o = p.o }
  if not extend then self.anchor = { b = p.b, o = p.o } end
  self.pending = nil
  self.lastKind = nil
end

function Editor:selectAll()
  self.cell = nil
  self.anchor = { b = 1, o = 0 }
  local last = #self.blocks
  self.caret = { b = last, o = M.length(self.blocks[last]) }
  self.pending = nil
end

-- clamp keeps a position inside the document.
function Editor:clamp(p)
  local b = math.max(1, math.min(#self.blocks, p.b))
  return { b = b, o = math.max(0, math.min(M.length(self.blocks[b]), p.o)) }
end

---------------------------------------------------------------- undo

local function snapshot(self)
  local blocks = {}
  for i, b in ipairs(self.blocks) do blocks[i] = copyBlock(b) end
  return { blocks = blocks, caret = { b = self.caret.b, o = self.caret.o },
    anchor = { b = self.anchor.b, o = self.anchor.o } }
end

-- checkpoint keeps the document as it is, for undo, before a change.
-- Typing one character after another is one step.
function Editor:checkpoint(kind)
  self.dirty = true
  if kind and kind == self.lastKind then return end
  self.undoStack[#self.undoStack + 1] = snapshot(self)
  if #self.undoStack > 200 then table.remove(self.undoStack, 1) end
  self.redoStack = {}
  self.lastKind = kind
end

local function restore(self, s)
  self.blocks = s.blocks
  self.caret, self.anchor = s.caret, s.anchor
  self.pending, self.lastKind, self.cell = nil, nil, nil
  self.dirty = true
end

function Editor:canUndo() return #self.undoStack > 0 end
function Editor:canRedo() return #self.redoStack > 0 end

function Editor:undo()
  local s = table.remove(self.undoStack)
  if not s then return false end
  self.redoStack[#self.redoStack + 1] = snapshot(self)
  restore(self, s)
  return true
end

function Editor:redo()
  local s = table.remove(self.redoStack)
  if not s then return false end
  self.undoStack[#self.undoStack + 1] = snapshot(self)
  restore(self, s)
  return true
end

---------------------------------------------------------------- deleting

-- remove deletes from s to e (s first) and leaves the caret at s.
local function remove(self, s, e)
  local blocks = self.blocks
  -- All of it: what is left is an empty paragraph.
  if s.b == 1 and s.o == 0 and e.b == #blocks and e.o == M.length(blocks[e.b]) then
    self.blocks = { newParagraph() }
    self:setCaret { b = 1, o = 0 }
    return
  end
  if s.b == e.b then
    local b = blocks[s.b]
    if hasText(b) then
      local left = split(b.runs, s.o)
      local _, right = split(b.runs, e.o)
      b.runs = concat(left, right)
    elseif s.o == 0 and e.o == 1 then
      blocks[s.b] = newParagraph()
    end
    self:setCaret(s)
    return
  end
  local first, last = blocks[s.b], blocks[e.b]
  local head, tail
  if hasText(first) then
    head = first
    head.runs = (split(first.runs, s.o))
  elseif s.o == 1 then
    head = first
  end
  if hasText(last) then
    tail = last
    local _, right = split(last.runs, e.o)
    tail.runs = right
  elseif e.o == 0 then
    tail = last
  end
  local keep = {}
  if head and tail and hasText(head) and hasText(tail) then
    -- Deleting from a block's very start takes the whole of it: what is
    -- left is the last block's rest, and of its kind.
    if s.o == 0 then
      head.type, head.level, head.lang = tail.type, tail.level, tail.lang
    end
    head.runs = concat(head.runs, head.type == "code" and plain(tail.runs) or tail.runs)
    if head.type == "code" then head.runs = plain(head.runs) end
    keep = { head }
  else
    if head then keep[#keep + 1] = head end
    if tail then keep[#keep + 1] = tail end
  end
  if #keep == 0 then keep = { newParagraph() } end
  for _ = s.b, e.b do table.remove(blocks, s.b) end
  for k = #keep, 1, -1 do table.insert(blocks, s.b, keep[k]) end
  if head then self:setCaret(s) else self:setCaret { b = s.b, o = 0 } end
end

function Editor:deleteSelection()
  if not self:hasSelection() then return false end
  self:checkpoint("delete")
  remove(self, self:range())
  self.lastKind = nil
  return true
end

---------------------------------------------------------------- styles

-- styleAt is the style typing at p takes: that of the character before it
-- (the first one's at a block's start), without a link that ends there.
function Editor:styleAt(p)
  local b = self.blocks[p.b]
  if not hasText(b) or b.type == "code" then return {} end
  local at, found, after = 0, nil, nil
  for _, r in ipairs(b.runs) do
    local n = #r.text
    if p.o > at and p.o <= at + n then
      found = r
      after = p.o == at + n
      break
    end
    at = at + n
  end
  found = found or b.runs[1]
  if not found then return {} end
  local s = { b = found.b, i = found.i, code = found.code, link = found.link, wiki = found.wiki }
  if after or p.o == 0 then s.link, s.wiki = nil, nil end
  return s
end

-- forEachSelected calls fn(block, runsInRange, setRuns) for the text of
-- the selection, block by block, skipping code blocks.
local function eachSelected(self, fn)
  local s, e = self:range()
  for i = s.b, e.b do
    local b = self.blocks[i]
    if hasText(b) and b.type ~= "code" then
      local o1 = i == s.b and s.o or 0
      local o2 = i == e.b and e.o or M.length(b)
      if o2 > o1 then
        local left, rest = split(b.runs, o1)
        local mid, right = split(rest, o2 - o1)
        fn(mid)
        b.runs = concat(left, mid, right)
      end
    end
  end
end

-- isOn says whether a style ("b", "i" or "code") is on: all through the
-- selection, or for what is typed next.
function Editor:isOn(key)
  if not self:hasSelection() then
    local s = self.pending or self:styleAt(self.caret)
    return s[key] and true or false
  end
  local all, any = true, false
  eachSelected(self, function(runs)
    for _, r in ipairs(runs) do
      any = true
      if not r[key] then all = false end
    end
  end)
  -- eachSelected rebuilt the runs; that changes nothing but their tables.
  return any and all
end

-- toggle turns a style on or off: through the selection, or for typing.
function Editor:toggle(key)
  local on = not self:isOn(key)
  if not self:hasSelection() then
    local s = self.pending or self:styleAt(self.caret)
    self.pending = { b = s.b, i = s.i, code = s.code, link = s.link, wiki = s.wiki }
    self.pending[key] = on or nil
    return
  end
  self:checkpoint("style")
  eachSelected(self, function(runs)
    for _, r in ipairs(runs) do r[key] = on or nil end
  end)
  self.lastKind = nil
end

-- linkAt is the link the caret is in, and where it starts and ends.
function Editor:linkAt(p)
  p = p or self.caret
  local b = self.blocks[p.b]
  if not hasText(b) then return nil end
  local at = 0
  for k, r in ipairs(b.runs) do
    local n = #r.text
    if r.link and p.o >= at and p.o <= at + n then
      -- The link may go on in the runs around (bold inside it, say).
      local s, e = at, at + n
      local j = k - 1
      while j >= 1 and b.runs[j].link == r.link do s = s - #b.runs[j].text; j = j - 1 end
      j = k + 1
      while b.runs[j] and b.runs[j].link == r.link do e = e + #b.runs[j].text; j = j + 1 end
      return r.link, s, e
    end
    at = at + n
  end
  return nil
end

-- setLink makes the selection, or the link the caret is in, link to url;
-- an empty url takes the link away. With nothing selected and no link
-- there, it puts the address in as a link.
function Editor:setLink(url)
  if url == "" then url = nil end
  if not self:hasSelection() then
    local old, s, e = self:linkAt()
    if old then
      self.anchor, self.caret = { b = self.caret.b, o = s }, { b = self.caret.b, o = e }
    elseif url then
      self:insertRuns { { text = url, link = url } }
      return
    else
      return
    end
  end
  self:checkpoint("link")
  eachSelected(self, function(runs)
    for _, r in ipairs(runs) do r.link, r.wiki = url, nil end
  end)
  self.lastKind = nil
end

---------------------------------------------------------------- blocks

function Editor:blockType()
  return self.blocks[self.caret.b].type
end

-- setType makes the blocks of the selection another kind. Making lists or
-- quotes of blocks that are all that kind already makes them paragraphs.
function Editor:setType(t, toggle)
  local s, e = self:range()
  if toggle then
    local all = true
    for i = s.b, e.b do
      if hasText(self.blocks[i]) and self.blocks[i].type ~= t then all = false end
    end
    if all then t = "p" end
  end
  self:checkpoint("type")
  for i = s.b, e.b do
    local b = self.blocks[i]
    if hasText(b) then
      if t == "code" and b.type ~= "code" then b.runs = plain(b.runs) end
      b.type = t
      if t == "ul" or t == "ol" then b.level = b.level or 0 else b.level = nil end
      if t ~= "code" then b.lang = nil end
    end
  end
  self.lastKind = nil
end

-- indent moves list items in (1) or out (-1); false when nothing is a list.
function Editor:indent(delta)
  local s, e = self:range()
  local any = false
  for i = s.b, e.b do
    local b = self.blocks[i]
    if b.type == "ul" or b.type == "ol" then any = true end
  end
  if not any then return false end
  self:checkpoint("indent")
  for i = s.b, e.b do
    local b = self.blocks[i]
    if b.type == "ul" or b.type == "ol" then
      b.level = math.max(0, math.min(3, (b.level or 0) + delta))
    end
  end
  self.lastKind = nil
  return true
end

-- insertBlock puts an image or a rule in after the caret's block, or in
-- place of it when it is an empty paragraph, and a paragraph after it to
-- go on typing in when nothing follows.
function Editor:insertBlock(block)
  self:deleteSelection()
  self:checkpoint("insert")
  local i = self.caret.b
  local cur = self.blocks[i]
  if cur.type == "p" and M.length(cur) == 0 then
    self.blocks[i] = block
  elseif self.caret.o == 0 then
    -- At a block's start the new one goes before it, leaving it whole.
    table.insert(self.blocks, i, block)
  else
    if hasText(cur) and self.caret.o < M.length(cur) then
      -- In the middle of a paragraph: the rest goes after the new block.
      local left, right = split(cur.runs, self.caret.o)
      cur.runs = left
      table.insert(self.blocks, i + 1, { type = cur.type, level = cur.level, runs = right })
    end
    i = i + 1
    table.insert(self.blocks, i, block)
  end
  if not self.blocks[i + 1] then self.blocks[i + 1] = newParagraph() end
  self:setCaret { b = i + 1, o = 0 }
end

function Editor:insertImage(src, alt)
  self:insertBlock { type = "image", src = src, alt = alt or "" }
end

function Editor:insertRule()
  self:insertBlock { type = "hr" }
end

---------------------------------------------------------------- typing

-- insertRuns puts styled text in at the caret, in place of the selection.
function Editor:insertRuns(runs, kind)
  if self.cell then return self:cellInsert(runs, kind) end
  if self:hasSelection() then
    self:checkpoint("delete")
    remove(self, self:range())
  end
  self:checkpoint(kind or "insert")
  local p = self.caret
  local b = self.blocks[p.b]
  if not hasText(b) then
    -- On an image or a rule: the text goes in a new paragraph beside it.
    local at = p.o == 0 and p.b or p.b + 1
    table.insert(self.blocks, at, newParagraph())
    p = { b = at, o = 0 }
    b = self.blocks[at]
  end
  if b.type == "code" then runs = plain(runs) end
  local n = 0
  for _, r in ipairs(runs) do n = n + #r.text end
  local left, right = split(b.runs, p.o)
  b.runs = concat(left, runs, right)
  local pending = self.pending
  self:setCaret { b = p.b, o = p.o + n }
  self.pending = pending
  self.lastKind = kind
end

-- shortcuts turn what is typed at a paragraph's start into its kind:
-- "# " a heading, "- " a list, "1. " a numbered one, "> " a quote.
local shortcuts = {
  { "^#$", "h1" }, { "^##$", "h2" }, { "^###$", "h3" },
  { "^[-*+]$", "ul" }, { "^%d+[.)]$", "ol" }, { "^>$", "quote" },
}

-- type puts text in as if typed, in the style for typing at the caret.
function Editor:type(text)
  local p = self.caret
  local b = self.blocks[p.b]
  local canShortcut = b.type == "p" or b.type == "ul" or b.type == "ol" or b.type == "quote"
  if text == " " and not self:hasSelection() and canShortcut then
    local before = M.textOf(b):sub(1, p.o)
    for _, sc in ipairs(shortcuts) do
      if before:match(sc[1]) then
        self:checkpoint("shortcut")
        local _, right = split(b.runs, p.o)
        b.runs = right
        b.type = sc[2]
        if sc[2] == "ul" or sc[2] == "ol" then b.level = 0 end
        self:setCaret { b = p.b, o = 0 }
        return
      end
    end
  end
  local style = self.pending or (self.cell and self:cellStyle()) or self:styleAt(p)
  local run = { text = text, b = style.b, i = style.i, code = style.code, link = style.link, wiki = style.wiki }
  self:insertRuns({ run }, "type")
end

-- lineBreak is Shift+Enter: a new line in the same block.
function Editor:lineBreak()
  if self.cell then return end
  local b = self.blocks[self.caret.b]
  if not hasText(b) then return self:enter() end
  local style = self.pending or self:styleAt(self.caret)
  self:insertRuns({ { text = "\n", b = style.b, i = style.i, code = style.code } }, "break")
end

local function isList(b) return b.type == "ul" or b.type == "ol" end

-- enter is Enter: a new block, or a new line in a code block.
function Editor:enter()
  if self.cell then return self:cellEnter() end
  self:deleteSelection()
  local p = self.caret
  local b = self.blocks[p.b]
  self:checkpoint("enter")
  if not hasText(b) then
    local at = p.o == 0 and p.b or p.b + 1
    table.insert(self.blocks, at, newParagraph())
    self:setCaret { b = p.o == 0 and p.b + 1 or at, o = 0 }
    return
  end
  local text = M.textOf(b)
  if b.type == "code" then
    -- Enter on an empty last line leaves the code block.
    if p.o == #text and text:sub(-1) == "\n" then
      b.runs = plain { { text = text:sub(1, -2) } }
      table.insert(self.blocks, p.b + 1, newParagraph())
      self:setCaret { b = p.b + 1, o = 0 }
    else
      local left, right = split(b.runs, p.o)
      b.runs = concat(left, { { text = "\n" } }, right)
      self:setCaret { b = p.b, o = p.o + 1 }
    end
    return
  end
  -- Enter in an empty item or quote ends the list or quote.
  if text == "" and (isList(b) or b.type == "quote") then
    if isList(b) and (b.level or 0) > 0 then
      b.level = b.level - 1
    else
      b.type, b.level = "p", nil
    end
    return
  end
  -- "---" and Enter is a rule.
  if b.type == "p" and text:match("^%-%-%-+$") and p.o == #text then
    self.blocks[p.b] = { type = "hr" }
    table.insert(self.blocks, p.b + 1, newParagraph())
    self:setCaret { b = p.b + 1, o = 0 }
    return
  end
  local left, right = split(b.runs, p.o)
  b.runs = md.normalize(left)
  local t = b.type
  -- After a heading comes a paragraph, unless the heading was split.
  if t:match("^h%d$") and #right == 0 then t = "p" end
  table.insert(self.blocks, p.b + 1, { type = t, level = b.level, runs = md.normalize(right) })
  local pending = self.pending
  self:setCaret { b = p.b + 1, o = 0 }
  self.pending = pending
end

function Editor:backspace()
  if self.cell then return self:cellDelete(-1) end
  if self:deleteSelection() then return end
  local p = self.caret
  local b = self.blocks[p.b]
  if p.o > 0 then
    if not hasText(b) then
      self:checkpoint("delete")
      self.blocks[p.b] = newParagraph()
      self:setCaret { b = p.b, o = 0 }
      return
    end
    self:checkpoint("backspace")
    remove(self, { b = p.b, o = prevChar(M.textOf(b), p.o) }, p)
    self.lastKind = "backspace"
    return
  end
  -- At a block's start.
  if hasText(b) and b.type ~= "p" then
    self:checkpoint("unformat")
    if isList(b) and (b.level or 0) > 0 then
      b.level = b.level - 1
    else
      if b.type == "code" then b.runs = plain(b.runs) end
      b.type, b.level, b.lang = "p", nil, nil
    end
    self.lastKind = nil
    return
  end
  if p.b == 1 then return end
  local prev = self.blocks[p.b - 1]
  self:checkpoint("join")
  if not hasText(prev) then
    table.remove(self.blocks, p.b - 1)
    self:setCaret { b = p.b - 1, o = 0 }
  elseif not hasText(b) then
    self:setCaret { b = p.b - 1, o = M.length(prev) }
  else
    remove(self, { b = p.b - 1, o = M.length(prev) }, p)
  end
  self.lastKind = nil
end

function Editor:delete()
  if self.cell then return self:cellDelete(1) end
  if self:deleteSelection() then return end
  local p = self.caret
  local b = self.blocks[p.b]
  if p.o < M.length(b) then
    self:checkpoint("delete")
    if not hasText(b) then
      self.blocks[p.b] = newParagraph()
      self:setCaret { b = p.b, o = 0 }
    else
      remove(self, p, { b = p.b, o = nextChar(M.textOf(b), p.o) })
    end
    self.lastKind = "delete"
    return
  end
  local nxt = self.blocks[p.b + 1]
  if not nxt then return end
  self:checkpoint("join")
  if not hasText(nxt) then
    table.remove(self.blocks, p.b + 1)
  else
    remove(self, p, { b = p.b + 1, o = 0 })
  end
  self.lastKind = nil
end

-- deleteWord is Alt+Backspace: back to the start of the word.
function Editor:deleteWord()
  if self.cell then return self:cellDelete(-1, true) end
  if self:hasSelection() then return self:deleteSelection() end
  local p = self.caret
  local b = self.blocks[p.b]
  if p.o == 0 or not hasText(b) then return self:backspace() end
  self:checkpoint("delete")
  remove(self, { b = p.b, o = wordLeft(M.textOf(b), p.o) }, p)
  self.lastKind = nil
end

---------------------------------------------------------------- moving

-- move goes a character (or a word) left or right, across blocks.
function Editor:move(dir, extend, byWord)
  if self.cell then return self:cellMove(dir, byWord) end
  if not extend and self:hasSelection() then
    local s, e = self:range()
    self:setCaret(dir < 0 and s or e)
    return
  end
  local p = self.caret
  local b = self.blocks[p.b]
  local text = M.textOf(b)
  local o
  if dir < 0 then
    if p.o == 0 then
      if p.b > 1 then
        self:setCaret({ b = p.b - 1, o = M.length(self.blocks[p.b - 1]) }, extend)
        if not extend then self:intoTable(-1) end
      end
      return
    end
    if b.type == "table" and not extend then return self:intoTable(-1) end
    o = not hasText(b) and 0 or (byWord and wordLeft(text, p.o) or prevChar(text, p.o))
  else
    if p.o >= M.length(b) then
      if p.b < #self.blocks then
        self:setCaret({ b = p.b + 1, o = 0 }, extend)
        if not extend then self:intoTable(1) end
      end
      return
    end
    if b.type == "table" and not extend then return self:intoTable(1) end
    o = not hasText(b) and 1 or (byWord and wordRight(text, p.o) or nextChar(text, p.o))
  end
  self:setCaret({ b = p.b, o = o }, extend)
end

function Editor:moveToDocument(dir, extend)
  if dir < 0 then self:setCaret({ b = 1, o = 0 }, extend)
  else self:setCaret({ b = #self.blocks, o = M.length(self.blocks[#self.blocks]) }, extend) end
end

-- These go by the lines the layout has broken the text into.
function Editor:moveLine(lay, dir, extend)
  if self.cell then
    if self:cellLine(lay, dir) then return end
    -- Out of the table: on to the block above or below it.
    self:setCaret({ b = self.caret.b, o = dir < 0 and 0 or 1 })
  end
  local goal = self.goalX
  local p, x = lay:vertical(self.caret, dir, goal)
  self:setCaret(p, extend)
  self.goalX = x
  if not extend then self:intoTable(dir, lay, x) end
end

function Editor:moveLineEdge(lay, dir, extend)
  if self.cell then return self:cellEdge(lay, dir) end
  self:setCaret(dir < 0 and lay:lineStart(self.caret) or lay:lineEnd(self.caret), extend)
end

function Editor:movePage(lay, dir, extend, height)
  if self.cell then self:setCaret { b = self.caret.b, o = dir < 0 and 0 or 1 } end
  local p, x = lay:vertical(self.caret, dir, self.goalX, height)
  self:setCaret(p, extend)
  self.goalX = x
  if not extend then self:intoTable(dir, lay, x) end
end

-- selectWord selects the word at p, for a double click.
function Editor:selectWord(p)
  local b = self.blocks[p.b]
  if not hasText(b) then
    self.anchor, self.caret = { b = p.b, o = 0 }, { b = p.b, o = 1 }
    return
  end
  local text = M.textOf(b)
  local s, e = p.o, p.o
  while s > 0 and isWordByte(text:byte(s)) do s = s - 1 end
  while e < #text and isWordByte(text:byte(e + 1)) do e = e + 1 end
  self.anchor, self.caret = { b = p.b, o = s }, { b = p.b, o = e }
  self.pending = nil
end

---------------------------------------------------------------- the clipboard

-- selectedBlocks is the selection as blocks of its own.
function Editor:selectedBlocks()
  local s, e = self:range()
  local out = {}
  for i = s.b, e.b do
    local b = copyBlock(self.blocks[i])
    if hasText(b) then
      local o1 = i == s.b and s.o or 0
      local o2 = i == e.b and e.o or M.length(b)
      local _, rest = split(b.runs, o1)
      b.runs = (split(rest, o2 - o1))
      out[#out + 1] = b
    elseif not (i == s.b and s.o == 1) and not (i == e.b and e.o == 0) then
      out[#out + 1] = b
    end
  end
  return out
end

-- copyText is the selection as Markdown: inline only when it is a part of
-- one block, so that pasting it into a line keeps it in the line.
function Editor:copyText()
  if not self:hasSelection() then return nil end
  local blocks = self:selectedBlocks()
  local s, e = self:range()
  if s.b == e.b and #blocks == 1 and hasText(blocks[1]) then
    if blocks[1].type == "code" then return M.textOf(blocks[1]) end
    return md.writeRuns(blocks[1].runs)
  end
  return (md.write(blocks):gsub("\n$", ""))
end

-- paste puts Markdown in at the caret: one line as text in the block it
-- lands in, more as blocks of their own.
function Editor:paste(text)
  text = text:gsub("\r\n?", "\n")
  if text == "" then return end
  if self.cell then text = text:gsub("\n+", " ") end
  local here = self.blocks[self.caret.b]
  if here.type == "code" and not self:hasSelection() then
    return self:insertRuns({ { text = text } }, "paste")
  end
  if not text:find("\n") then
    return self:insertRuns(md.inline(text), "paste")
  end
  local pasted = md.parse(text)
  if #pasted == 0 then return end
  if self:hasSelection() then
    self:checkpoint("delete")
    remove(self, self:range())
  end
  self:checkpoint("paste")
  local p = self.caret
  local b = self.blocks[p.b]
  if not hasText(b) then
    local at = p.o == 0 and p.b or p.b + 1
    for k, pb in ipairs(pasted) do table.insert(self.blocks, at + k - 1, pb) end
    local last = at + #pasted - 1
    self:setCaret { b = last, o = M.length(self.blocks[last]) }
    return
  end
  local left, right = split(b.runs, p.o)
  b.runs = left
  local idx = p.b
  for k, pb in ipairs(pasted) do
    if k == 1 and hasText(pb) then
      b.runs = concat(b.runs, b.type == "code" and plain(pb.runs) or pb.runs)
      if k == #pasted and #left == 0 then b.type, b.level = pb.type, pb.level end
    else
      idx = idx + 1
      table.insert(self.blocks, idx, pb)
    end
  end
  local last = self.blocks[idx]
  if hasText(last) then
    local o = M.length(last)
    last.runs = concat(last.runs, last.type == "code" and plain(right) or right)
    self:setCaret { b = idx, o = o }
  else
    table.insert(self.blocks, idx + 1, { type = b.type, level = b.level, runs = md.normalize(right) })
    self:setCaret { b = idx + 1, o = 0 }
  end
end

---------------------------------------------------------------- tables

-- A table is edited a cell at a time. The caret stays on the table (o = 0)
-- and e.cell = { r = row, k = column, o = offset in the cell's text } says
-- which cell it is in; it is nil when the caret is anywhere else. Typing,
-- Backspace, Delete, Enter, the arrows and paste work in the cell as in a
-- paragraph, but for a cell being one line and having no selection.

local function cellLen(runs)
  local n = 0
  for _, r in ipairs(runs) do n = n + #r.text end
  return n
end

local function cellString(runs)
  local t = {}
  for i, r in ipairs(runs) do t[i] = r.text end
  return table.concat(t)
end

-- tableAt is the table the caret is on or in, or nil.
function Editor:tableAt()
  local b = self.blocks[self.caret.b]
  if b and b.type == "table" then return b, self.caret.b end
end

-- cellRuns are the runs of the cell the caret is in, or nil.
function Editor:cellRuns()
  local t = self.cell and self:tableAt()
  return t and t.rows[self.cell.r][self.cell.k]
end

-- enterCell puts the caret in a cell of the table that is block b; o is the
-- offset in its text, the end when not given.
function Editor:enterCell(b, r, k, o)
  local t = self.blocks[b]
  if not t or t.type ~= "table" then return false end
  r = math.max(1, math.min(#t.rows, r))
  k = math.max(1, math.min(#t.align, k))
  local len = cellLen(t.rows[r][k])
  o = o == nil and len or math.max(0, math.min(len, o))
  self:setCaret { b = b, o = 0 }
  self.cell = { r = r, k = k, o = o }
  return true
end

-- intoTable moves the caret on a table into one of its cells: the first
-- going forward (1), the last going back (-1), or, with the layout and
-- the column x, the one below or above x.
function Editor:intoTable(dir, lay, x)
  local t, b = self:tableAt()
  if not t or self.cell then return end
  if lay and x then
    local L = lay.blocks[b]
    local r, k, o = lay:cellHit(b, x, dir > 0 and L.y + 1 or L.y + L.contentH - 1)
    if r then return self:enterCell(b, r, k, o) end
  end
  if dir > 0 then self:enterCell(b, 1, 1, 0)
  else self:enterCell(b, #t.rows, #t.align) end
end

-- leaveTable puts the caret out of the cell: after the table, or before it.
function Editor:leaveTable(before)
  local _, b = self:tableAt()
  if b then self:setCaret { b = b, o = before and 0 or 1 } end
end

-- cellStyle is the style typing in the cell takes: that of the text before
-- the caret, without a link.
function Editor:cellStyle()
  local runs = self:cellRuns()
  if not runs then return {} end
  local at, found = 0, nil
  for _, r in ipairs(runs) do
    if self.cell.o > at and self.cell.o <= at + #r.text then found = r break end
    at = at + #r.text
  end
  found = found or runs[1]
  if not found then return {} end
  return { b = found.b, i = found.i, code = found.code }
end

-- cellInsert puts runs into the cell at the caret; a line break is a space.
function Editor:cellInsert(runs, kind)
  local t = self:tableAt()
  if not t then return end
  local c = self.cell
  local clean, n = {}, 0
  for i, r in ipairs(runs) do
    clean[i] = copyRun(r, (r.text:gsub("[\r\n]+", " ")))
    n = n + #clean[i].text
  end
  self:checkpoint(kind or "insert")
  local left, right = split(t.rows[c.r][c.k], c.o)
  t.rows[c.r][c.k] = concat(left, clean, right)
  c.o = c.o + n
  self.lastKind = kind
end

-- cellDelete deletes the character (or word) before the caret in the cell
-- (dir -1) or after it (1).
function Editor:cellDelete(dir, byWord)
  local runs = self:cellRuns()
  if not runs then return end
  local c = self.cell
  local text = cellString(runs)
  local a, z = c.o, c.o
  if dir < 0 then
    if c.o == 0 then return end
    a = byWord and wordLeft(text, c.o) or prevChar(text, c.o)
  else
    if c.o >= #text then return end
    z = nextChar(text, c.o)
  end
  self:checkpoint(dir < 0 and "backspace" or "delete")
  local t = self:tableAt()
  local left = split(runs, a)
  local _, right = split(runs, z)
  t.rows[c.r][c.k] = concat(left, right)
  c.o = a
  self.lastKind = dir < 0 and "backspace" or "delete"
end

-- cellMove goes a character (or a word) left or right in the cell, and on
-- into the next cell, or out of the table, at its ends.
function Editor:cellMove(dir, byWord)
  local t, b = self:tableAt()
  local runs = self:cellRuns()
  if not runs then return end
  local c = self.cell
  local text = cellString(runs)
  self.lastKind, self.pending = nil, nil
  if dir < 0 then
    if c.o > 0 then
      c.o = byWord and wordLeft(text, c.o) or prevChar(text, c.o)
    elseif c.k > 1 then self:enterCell(b, c.r, c.k - 1)
    elseif c.r > 1 then self:enterCell(b, c.r - 1, #t.align)
    else self:leaveTable(true) end
  else
    if c.o < #text then
      c.o = byWord and wordRight(text, c.o) or nextChar(text, c.o)
    elseif c.k < #t.align then self:enterCell(b, c.r, c.k + 1, 0)
    elseif c.r < #t.rows then self:enterCell(b, c.r + 1, 1, 0)
    else self:leaveTable() end
  end
end

-- cellLine goes a line up or down inside the table, keeping to the column;
-- false when that would leave the table.
function Editor:cellLine(lay, dir)
  local t, b = self:tableAt()
  if not t or not self.cell then return false end
  local c = self.cell
  local x, y, h = lay:cellCaret(b, c.r, c.k, c.o)
  x = self.goalX or x
  local L = lay.blocks[b]
  -- Above the first line of a cell is the cell above, below the last the
  -- one below.
  local _, ry, _, rh = lay:cellRect(b, c.r, c.k)
  local pad = L.style.pad
  local ty
  if dir < 0 then ty = y - 2 < ry + pad and ry - 1 or y - 2
  else ty = y + h + 1 > ry + rh - pad and ry + rh + 1 or y + h + 1 end
  if ty < L.y or ty >= L.y + L.contentH then return false end
  local r, k, o = lay:cellHit(b, x, ty)
  if not r then return false end
  self:enterCell(b, r, k, o)
  self.goalX = x
  return true
end

-- cellEdge goes to the start (-1) or end (1) of the line in the cell.
function Editor:cellEdge(lay, dir)
  local _, b = self:tableAt()
  local c = self.cell
  if not c then return end
  local o = lay:cellLineEdge(b, c.r, c.k, c.o, dir)
  c.o = o
end

-- cellEnter is Enter in a cell: down to the cell below, and out of the
-- table, into the paragraph after it, from the last row.
function Editor:cellEnter()
  local t, b = self:tableAt()
  if not t or not self.cell then return end
  local c = self.cell
  if c.r < #t.rows then return self:enterCell(b, c.r + 1, c.k, c.o) end
  self:checkpoint("enter")
  if not self.blocks[b + 1] then self.blocks[b + 1] = newParagraph() end
  self:setCaret { b = b + 1, o = 0 }
end

-- cellTab is Tab (1) or Shift+Tab (-1) in a table: to the next cell or the
-- one before, with a new row after the last cell. It enters the table's
-- first cell when the caret is on it but not in a cell. False when the
-- caret is not on a table.
function Editor:cellTab(dir)
  local t, b = self:tableAt()
  if not t then return false end
  local c = self.cell
  if not c then return self:enterCell(b, 1, 1) end
  if dir < 0 then
    if c.k > 1 then self:enterCell(b, c.r, c.k - 1)
    elseif c.r > 1 then self:enterCell(b, c.r - 1, #t.align) end
  else
    if c.k < #t.align then self:enterCell(b, c.r, c.k + 1)
    elseif c.r < #t.rows then self:enterCell(b, c.r + 1, 1)
    else
      self:insertRow(true)
      self:enterCell(b, #t.rows, 1)
    end
  end
  return true
end

local function emptyRow(n)
  local row = {}
  for k = 1, n do row[k] = {} end
  return row
end

-- insertTable puts a table of rows by columns (the first row is its
-- header) in after the caret's block, as a picture or a rule goes, and the
-- caret in its first cell.
function Editor:insertTable(rows, cols)
  rows, cols = math.max(1, rows or 3), math.max(1, cols or 3)
  local t = { type = "table", align = {}, rows = {} }
  for k = 1, cols do t.align[k] = false end
  for r = 1, rows do t.rows[r] = emptyRow(cols) end
  self:insertBlock(t)
  local b = self.caret.b - 1
  self:enterCell(b, 1, 1, 0)
  return true
end

-- cellOrLast is the row and column an edit of the table's shape is about:
-- the cell the caret is in, or the table's last row and first column.
local function whereIn(self, t)
  local c = self.cell
  if c then return c.r, c.k end
  return #t.rows, 1
end

-- insertRow adds an empty row after the caret's row (or before it, with
-- false, but never above the header).
function Editor:insertRow(after)
  local t = self:tableAt()
  if not t then return false end
  local r = whereIn(self, t)
  self:checkpoint()
  local at = after == false and math.max(2, r) or r + 1
  table.insert(t.rows, at, emptyRow(#t.align))
  -- The caret stays with its own row, which a row put before it pushes down.
  if self.cell and after == false and r >= 2 then self.cell.r = r + 1 end
  self.lastKind = nil
  return true
end

-- insertColumn adds an empty column after the caret's column (or before it,
-- with false).
function Editor:insertColumn(after)
  local t = self:tableAt()
  if not t then return false end
  local _, k = whereIn(self, t)
  self:checkpoint()
  local at = after == false and k or k + 1
  table.insert(t.align, at, false)
  for _, row in ipairs(t.rows) do table.insert(row, at, {}) end
  if self.cell and after == false then self.cell.k = k + 1 end
  self.lastKind = nil
  return true
end

-- deleteRow removes the caret's row; a table keeps at least its header.
function Editor:deleteRow()
  local t = self:tableAt()
  if not t or #t.rows < 2 then return false end
  local r = whereIn(self, t)
  self:checkpoint()
  table.remove(t.rows, r)
  if self.cell then
    self.cell.r = math.min(r, #t.rows)
    self.cell.o = math.min(self.cell.o, cellLen(t.rows[self.cell.r][self.cell.k]))
  end
  self.lastKind = nil
  return true
end

-- deleteColumn removes the caret's column; a table keeps at least one.
function Editor:deleteColumn()
  local t = self:tableAt()
  if not t or #t.align < 2 then return false end
  local _, k = whereIn(self, t)
  self:checkpoint()
  table.remove(t.align, k)
  for _, row in ipairs(t.rows) do table.remove(row, k) end
  if self.cell then
    self.cell.k = math.min(k, #t.align)
    self.cell.o = math.min(self.cell.o, cellLen(t.rows[self.cell.r][self.cell.k]))
  end
  self.lastKind = nil
  return true
end

-- setAlign aligns the caret's column: "left", "center", "right", or false.
function Editor:setAlign(how)
  local t = self:tableAt()
  if not t then return false end
  local _, k = whereIn(self, t)
  self:checkpoint()
  t.align[k] = how or false
  self.lastKind = nil
  return true
end

---------------------------------------------------------------- find and replace

-- opts, for these: matchCase (letters differ by case, else they do not; only
-- ASCII letters have two), whole (a match is a whole word) and wrap (go
-- round at the end of the document: on unless false). A match is in one
-- block's text; tables are not searched.

-- findIn is where needle is in text from byte from: its first and last byte,
-- or nil.
local function findIn(text, needle, from, opts)
  local hay, n = text, needle
  if not opts.matchCase then hay, n = text:lower(), needle:lower() end
  while true do
    local s, e = hay:find(n, from, true)
    if not s then return nil end
    if not opts.whole or (not isWordByte(text:byte(s - 1)) and not isWordByte(text:byte(e + 1))) then return s, e end
    from = s + 1
  end
end

-- lastIn is the last match in text that begins before byte limit.
local function lastIn(text, needle, limit, opts)
  local from, ls, le = 1, nil, nil
  while true do
    local s, e = findIn(text, needle, from, opts)
    if not s or s > limit then return ls, le end
    ls, le = s, e
    from = s + 1
  end
end

-- find selects the next match of needle after the selection (before it,
-- with back) and says whether there was one; a second result is true when
-- it went round the end to find it.
function Editor:find(needle, opts, back)
  opts = opts or {}
  if needle == nil or needle == "" then return false end
  local a, z = self:range()
  local n = #self.blocks
  local wrap = opts.wrap ~= false
  local function mark(i, s, e)
    self.anchor, self.caret = { b = i, o = s - 1 }, { b = i, o = e }
    self.cell, self.pending, self.lastKind = nil, nil, nil
  end
  for step = 0, n do
    local wrapped = false
    local i
    if back then
      i = a.b - step
      if i < 1 then i, wrapped = i + n, true end
    else
      i = z.b + step
      if i > n then i, wrapped = i - n, true end
    end
    if step > 0 and wrapped and not wrap then return false end
    local b = self.blocks[i]
    if hasText(b) then
      local text = M.textOf(b)
      local s, e
      if back then
        local limit = (step == 0) and a.o or #text + 1
        -- A match must end where the selection begins, or before.
        if step == 0 then
          local from, ls, le = 1, nil, nil
          while true do
            local ms, me = findIn(text, needle, from, opts)
            if not ms or me > limit then break end
            ls, le, from = ms, me, ms + 1
          end
          s, e = ls, le
        else
          s, e = lastIn(text, needle, limit, opts)
        end
      else
        local from = (step == 0) and z.o + 1 or 1
        s, e = findIn(text, needle, from, opts)
      end
      if s then
        mark(i, s, e)
        return true, wrapped or step == n
      end
    end
  end
  return false
end

-- isMatch says whether the selection is a match of needle exactly.
function Editor:isMatch(needle, opts)
  opts = opts or {}
  if needle == nil or needle == "" or not self:hasSelection() then return false end
  local a, z = self:range()
  if a.b ~= z.b or not hasText(self.blocks[a.b]) then return false end
  local text = M.textOf(self.blocks[a.b])
  local s, e = findIn(text, needle, a.o + 1, opts)
  return s == a.o + 1 and e == z.o
end

-- countMatches is how many matches there are, and which of them the
-- selection is (0 when none).
function Editor:countMatches(needle, opts)
  opts = opts or {}
  if needle == nil or needle == "" then return 0, 0 end
  local a = self:range()
  local count, which = 0, 0
  for i, b in ipairs(self.blocks) do
    if hasText(b) then
      local text, from = M.textOf(b), 1
      while true do
        local s, e = findIn(text, needle, from, opts)
        if not s then break end
        count = count + 1
        if which == 0 and a.b == i and a.o == s - 1 and self:isMatch(needle, opts) then which = count end
        from = s + 1
      end
    end
  end
  return count, which
end

-- replace puts with in place of the selection when that is a match of
-- needle, and selects the next one. It says whether it replaced, and
-- whether it found a next.
function Editor:replace(needle, with, opts)
  local replaced = false
  if self:isMatch(needle, opts) then
    local a = self:range()
    local style = self:styleAt({ b = a.b, o = a.o + 1 })
    local run = { text = with, b = style.b, i = style.i, code = style.code, link = style.link, wiki = style.wiki }
    self:insertRuns({ run }, "replace")
    self.lastKind = nil
    replaced = true
  end
  return replaced, self:find(needle, opts)
end

-- replaceAll replaces every match, as one step of undo, and says how many.
function Editor:replaceAll(needle, with, opts)
  opts = opts or {}
  if needle == nil or needle == "" then return 0 end
  local total = 0
  local checked = false
  for i, b in ipairs(self.blocks) do
    if hasText(b) then
      local text = M.textOf(b)
      local from = 1
      while true do
        local s, e = findIn(text, needle, from, opts)
        if not s then break end
        if not checked then self:checkpoint("replace") checked = true end
        local style = b.type == "code" and {} or self:styleAt({ b = i, o = s })
        local run = { text = with, b = style.b, i = style.i, code = style.code, link = style.link, wiki = style.wiki }
        local left = split(b.runs, s - 1)
        local _, right = split(b.runs, e)
        b.runs = concat(left, b.type == "code" and plain({ run }) or { run }, right)
        text = M.textOf(b)
        from = s + #with
        total = total + 1
      end
    end
  end
  if total > 0 then
    self.lastKind = nil
    self:setCaret(self:clamp(self.caret))
  end
  return total
end

---------------------------------------------------------------- counting

function Editor:stats()
  local words, chars = 0, 0
  for _, b in ipairs(self.blocks) do
    local t = M.textOf(b)
    if b.type == "table" then
      local cells = {}
      for _, row in ipairs(b.rows) do
        for _, cell in ipairs(row) do cells[#cells + 1] = M.textOf { runs = cell } end
      end
      t = table.concat(cells, " ")
    end
    words = words + select(2, t:gsub("%S+", ""))
    chars = chars + #t:gsub("[\128-\191]", "")
  end
  return words, chars
end

return M
