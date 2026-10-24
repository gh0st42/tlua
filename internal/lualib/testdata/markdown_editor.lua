-- The tests of markdown.editor: the document as typing, Enter, Backspace,
-- styles, lists, links, pictures, the clipboard and undo change it. Run by
-- markdown_editor_test.go; each test(name, fn) that fails is reported, and
-- the script fails at the end if any did.

local md = require "markdown"
local editor = require "markdown.editor"

local failed = {}
local function test(name, fn)
  local ok, err = pcall(fn)
  if not ok then failed[#failed + 1] = name .. ": " .. tostring(err) end
end
local function eq(got, want, what)
  if got ~= want then
    error(("%s: got %q, want %q"):format(what or "value", tostring(got), tostring(want)), 2)
  end
end
local function ok(cond, what)
  if not cond then error((what or "condition") .. " is not true", 2) end
end

---------------------------------------------------------------- editing

local function ed(text)
  return editor.new(md.parse(text or ""))
end

local function typeText(e, s)
  for ch in s:gmatch("[%z\1-\127\194-\244][\128-\191]*") do e:type(ch) end
end

test("typing and Enter", function()
  local e = ed()
  typeText(e, "Hello")
  e:enter()
  typeText(e, "world")
  eq(e:markdown(), "Hello\n\nworld\n")
  eq(e.caret.b, 2); eq(e.caret.o, 5)
end)

test("markdown shortcuts at a paragraph's start", function()
  local e = ed()
  typeText(e, "## Head")
  eq(e:blockType(), "h2")
  e:enter()
  eq(e:blockType(), "p") -- after a heading comes a paragraph
  typeText(e, "- item")
  e:enter()
  typeText(e, "1. x")
  eq(e:markdown(), "## Head\n\n- item\n1. x\n")
  e:enter(); e:enter() -- Enter on an empty item ends the list
  eq(e:blockType(), "p")
  typeText(e, "---")
  e:enter()
  eq(e.blocks[#e.blocks - 1].type, "hr")
end)

test("bold while typing, and over a selection", function()
  local e = ed()
  typeText(e, "a ")
  e:toggle("b")
  typeText(e, "bold")
  e:toggle("b")
  typeText(e, " c")
  eq(e:markdown(), "a **bold** c\n")
  e.anchor, e.caret = { b = 1, o = 0 }, { b = 1, o = 1 }
  e:toggle("i")
  eq(e:markdown(), "*a* **bold** c\n")
  ok(e:isOn("i"), "on over the selection")
  e:toggle("i")
  eq(e:markdown(), "a **bold** c\n")
end)

test("Backspace and Delete across blocks", function()
  local e = ed("one\n\ntwo\n")
  e:setCaret { b = 2, o = 0 }
  e:backspace()
  eq(e:markdown(), "onetwo\n")
  eq(e.caret.o, 3)
  e:delete()
  eq(e:markdown(), "onewo\n")
  e:backspace(); e:backspace()
  eq(e:markdown(), "owo\n")
end)

test("Backspace at a heading's or item's start makes it a paragraph", function()
  local e = ed("# H\n\n    - x\n")
  e:setCaret { b = 1, o = 0 }
  e:backspace()
  eq(e:blockType(), "p")
  local f = ed("- a\n    - b\n")
  f:setCaret { b = 2, o = 0 }
  f:backspace()
  eq(f.blocks[2].level, 0)
  f:backspace()
  eq(f.blocks[2].type, "p")
end)

test("deleting a selection over several blocks", function()
  local e = ed("# Alpha\n\nbeta\n\ngamma\n")
  e.anchor, e.caret = { b = 1, o = 2 }, { b = 3, o = 3 }
  e:backspace()
  eq(e:markdown(), "# Alma\n")
end)

test("UTF-8 is moved over and deleted a character at a time", function()
  local e = ed()
  typeText(e, "héé")
  e:backspace()
  eq(e:markdown(), "hé\n")
  e:move(-1)
  eq(e.caret.o, 1)
end)

test("undo and redo", function()
  local e = ed()
  typeText(e, "abc")
  e:enter()
  typeText(e, "d")
  eq(e:markdown(), "abc\n\nd\n")
  e:undo()
  eq(e:markdown(), "abc\n"); eq(#e.blocks, 2) -- the empty paragraph is not written
  e:undo()
  eq(#e.blocks, 1)
  e:undo()
  eq(e:markdown(), "\n")
  e:redo(); e:redo()
  eq(#e.blocks, 2)
end)

test("links", function()
  local e = ed("see here now\n")
  e.anchor, e.caret = { b = 1, o = 4 }, { b = 1, o = 8 }
  e:setLink("https://x.org")
  eq(e:markdown(), "see [here](https://x.org) now\n")
  e:setCaret { b = 1, o = 6 }
  eq(e:linkAt(), "https://x.org")
  e:setLink("https://y.org") -- the link the caret is in changes
  eq(e:markdown(), "see [here](https://y.org) now\n")
  e:setLink("")
  eq(e:markdown(), "see here now\n")
end)

test("lists: kinds, toggling and indent", function()
  local e = ed("a\n\nb\n")
  e.anchor, e.caret = { b = 1, o = 0 }, { b = 2, o = 1 }
  e:setType("ul", true)
  eq(e:markdown(), "- a\n- b\n")
  e:setCaret { b = 2, o = 0 }
  ok(e:indent(1), "indented")
  eq(e:markdown(), "- a\n    - b\n")
  e.anchor, e.caret = { b = 1, o = 0 }, { b = 2, o = 1 }
  e:setType("ul", true) -- all lists already: back to paragraphs
  eq(e:markdown(), "a\n\nb\n")
end)

test("code blocks keep their text plain and their lines", function()
  local e = ed("**x** y\n")
  e:setType("code")
  eq(e:markdown(), "```\nx y\n```\n")
  e:setCaret { b = 1, o = 3 }
  e:enter()
  typeText(e, "z")
  eq(e:markdown(), "```\nx y\nz\n```\n")
  e:enter(); e:enter() -- Enter on an empty last line leaves the block
  eq(#e.blocks, 2)
  eq(e:blockType(), "p")
end)

test("pictures and rules", function()
  local e = ed("text\n")
  e:setCaret { b = 1, o = 4 }
  e:insertImage("assets/a.png", "a")
  eq(e:markdown(), "text\n\n![a](assets/a.png)\n")
  eq(e:blockType(), "p") -- the caret goes on after it
  e:setCaret { b = 2, o = 1 }
  e:backspace()
  eq(e:markdown(), "text\n")
  e:insertRule()
  ok(e:markdown():find("%-%-%-"), "a rule")
end)

test("tables, lists of contents and wiki links are kept as they were", function()
  local text = "[[toc]]\n\n# Head\n\n| a | b |\n| --- | ---: |\n| 1 | [[Page|go]] |\n\nsee [[Other Page]]\n"
  local e = ed(text)
  eq(e:markdown(), text)
  -- A table is one thing to the caret, as a picture is.
  e:setCaret { b = 3, o = 1 }
  e:backspace()
  eq(e:markdown(), "[[toc]]\n\n# Head\n\nsee [[Other Page]]\n")
  -- Typing inside a wiki link keeps it one.
  e:setCaret { b = #e.blocks, o = 5 }
  typeText(e, "X")
  eq(e:markdown(), "[[toc]]\n\n# Head\n\nsee [[Other Page|OXther Page]]\n")
end)

test("copy and paste keep the formatting", function()
  local e = ed("a **bold** word\n\nsecond\n")
  e.anchor, e.caret = { b = 1, o = 2 }, { b = 1, o = 6 }
  eq(e:copyText(), "**bold**")
  e.anchor, e.caret = { b = 1, o = 2 }, { b = 2, o = 3 }
  local text = e:copyText()
  eq(text, "**bold** word\n\nsec")
  e:setCaret { b = 2, o = 6 }
  e:paste(text)
  eq(e:markdown(), "a **bold** word\n\nsecond**bold** word\n\nsec\n")
  local f = ed("x\n")
  f:setCaret { b = 1, o = 1 }
  f:paste("- one\n- two")
  eq(f:markdown(), "xone\n\n- two\n")
end)

test("word and character counts", function()
  local w, c = ed("# Two words\n\nthree more words é\n"):stats()
  eq(w, 6); eq(c, 27)
end)


test("a block put in at a paragraph's start goes before it", function()
  local e = ed("# One\n\ntext\n")
  e:insertBlock { type = "toc" }
  eq(e:markdown(), "[[toc]]\n\n# One\n\ntext\n")
  eq(e.caret.b, 2) eq(e.caret.o, 0)
  e:setCaret { b = 3, o = 2 }
  e:insertRule()
  eq(e:markdown(), "[[toc]]\n\n# One\n\nte\n\n---\n\nxt\n")
end)


---------------------------------------------------------------- tables

local TABLE = "| Name | Qty |\n| :--- | ---: |\n| Tea | 2 |\n| Milk | 1 |\n"

test("a table is edited a cell at a time", function()
  local e = ed("before\n\n" .. TABLE .. "\nafter\n")
  e:setCaret { b = 2, o = 0 }
  ok(e:cellTab(1), "tab enters the table")
  eq(e.cell.r, 1) eq(e.cell.k, 1) eq(e.cell.o, 4) -- the end of "Name"
  typeText(e, "s")
  e:cellTab(1)
  e:backspace()
  typeText(e, "Count | n")
  eq(md.textOf { runs = e:cellRuns() }, "QtCount | n")
  ok(e:markdown():find("| Names | QtCount \\| n |", 1, true), "the pipe is escaped")
end)

test("Tab and Enter move between cells, and Tab at the end adds a row", function()
  local e = ed(TABLE)
  e:enterCell(1, 2, 2, 0)
  e:cellTab(1)
  eq(e.cell.r, 3) eq(e.cell.k, 1)
  e:cellTab(1) e:cellTab(1)
  eq(e.cell.r, 4) eq(e.cell.k, 1)
  eq(#e.blocks[1].rows, 4)
  e:cellTab(-1)
  eq(e.cell.r, 3) eq(e.cell.k, 2)
  e:enter() -- down
  eq(e.cell.r, 4) eq(e.cell.k, 2)
  e:enter() -- out of the last row: a paragraph after the table
  eq(e.caret.b, 2) eq(e.blocks[2].type, "p") ok(e.cell == nil, "left the table")
end)

test("the arrows walk through a table's cells and out of it", function()
  local e = ed("a\n\n" .. TABLE .. "\nb\n")
  e:setCaret { b = 1, o = 1 }
  e:move(1) -- into the table, its first cell
  ok(e.cell, "in a cell") eq(e.caret.b, 2) eq(e.cell.o, 0)
  e:move(1) e:move(1) e:move(1) e:move(1) -- through "Name"
  eq(e.cell.k, 1) eq(e.cell.o, 4)
  e:move(1) -- the next cell
  eq(e.cell.k, 2) eq(e.cell.o, 0)
  e:move(-1) -- back to the end of the one before
  eq(e.cell.k, 1) eq(e.cell.o, 4)
  e:enterCell(2, 3, 2, 1)
  e:move(1) -- the last cell's end: out, after the table
  ok(e.cell == nil, "out") eq(e.caret.b, 2) eq(e.caret.o, 1)
  e:move(-1) -- and back into the last cell
  eq(e.cell.r, 3) eq(e.cell.k, 2)
end)

test("Delete and Backspace stay inside a cell", function()
  local e = ed(TABLE)
  e:enterCell(1, 2, 1, 0)
  e:backspace() -- the start of a cell: nothing
  eq(md.textOf { runs = e:cellRuns() }, "Tea")
  e:delete()
  eq(md.textOf { runs = e:cellRuns() }, "ea")
  e.cell.o = 2
  e:delete() -- the end: nothing, the next cell is not joined
  eq(md.textOf { runs = e:cellRuns() }, "ea")
  e:backspace()
  eq(md.textOf { runs = e:cellRuns() }, "e")
end)

test("typing in a cell keeps the style of the text before it", function()
  local e = ed("| **a** | b |\n| --- | --- |\n| c | d |\n")
  e:enterCell(1, 1, 1)
  typeText(e, "z")
  ok(e:markdown():find("| **az** | b |", 1, true), e:markdown())
end)

test("paste in a cell is one line", function()
  local e = ed(TABLE)
  e:enterCell(1, 2, 2)
  e:paste("x\ny")
  eq(md.textOf { runs = e:cellRuns() }, "2x y")
end)

test("inserting a table", function()
  local e = ed("text\n")
  e:setCaret { b = 1, o = 4 }
  e:insertTable(2, 3)
  local t = e.blocks[2]
  eq(t.type, "table") eq(#t.rows, 2) eq(#t.align, 3)
  eq(e.cell.r, 1) eq(e.cell.k, 1)
  typeText(e, "A")
  e:cellTab(1) typeText(e, "B")
  ok(e:markdown():find("| A | B |  |", 1, true), e:markdown())
  eq(e.blocks[3].type, "p") -- somewhere to go on typing
  local f = ed()
  f:insertTable()
  eq(f.blocks[1].type, "table") eq(#f.blocks[1].rows, 3)
end)

test("rows and columns are added and removed", function()
  local e = ed(TABLE)
  local function cell(r, k) return md.textOf { runs = e.blocks[1].rows[r][k] } end
  e:enterCell(1, 2, 1) -- "Tea"
  e:insertRow(true)
  eq(#e.blocks[1].rows, 4) eq(cell(3, 1), "") eq(cell(4, 1), "Milk")
  eq(e.cell.r, 2)
  e:insertRow(false) -- above the row: the caret goes with it
  eq(#e.blocks[1].rows, 5) eq(e.cell.r, 3) eq(cell(3, 1), "Tea")
  e:deleteRow() e:deleteRow()
  eq(#e.blocks[1].rows, 3) eq(cell(2, 1), "")
  e:enterCell(1, 1, 1)
  e:insertRow(false) -- never above the header
  eq(cell(1, 1), "Name") eq(cell(2, 1), "")
  e:enterCell(1, 1, 2)
  e:insertColumn(true)
  eq(#e.blocks[1].align, 3) eq(#e.blocks[1].rows[3], 3)
  eq(cell(1, 2), "Qty")
  e:insertColumn(false)
  eq(e.cell.k, 3)
  e:deleteColumn() e:deleteColumn()
  eq(#e.blocks[1].align, 2)
  ok(e:deleteColumn() and not e:deleteColumn(), "a table keeps a column")
  local one = ed("| a |\n| --- |\n")
  one:enterCell(1, 1, 1)
  ok(not one:deleteRow(), "a table keeps its header")
end)

test("alignment is set for the caret's column", function()
  local e = ed(TABLE)
  e:enterCell(1, 2, 1)
  e:setAlign("center")
  ok(e:markdown():find("| :---: | ---: |", 1, true), e:markdown())
  e:setAlign(false)
  ok(e:markdown():find("| --- | ---: |", 1, true), e:markdown())
end)

test("undo restores a table's cells", function()
  local e = ed(TABLE)
  e:enterCell(1, 2, 1)
  typeText(e, "!")
  e:insertRow(true)
  e:undo()
  eq(#e.blocks[1].rows, 3)
  ok(e:markdown():find("| Tea! |", 1, true), e:markdown())
  e:undo()
  eq(e:markdown(), TABLE)
  e:redo() e:redo()
  eq(#e.blocks[1].rows, 4)
  ok(e.cell == nil, "undo leaves the cell")
end)

test("a table's words are counted", function()
  local w = ed(TABLE):stats()
  eq(w, 6)
end)

test("cells are found in a layout", function()
  local layout = require "markdown.layout"
  local function measure(text) return #text * 7 end
  local e = ed("before\n\n" .. TABLE)
  local lay = layout.build(e.blocks, 400, measure, nil, {}, {})
  e:enterCell(2, 3, 1, 2)
  local x, y, h = lay:cellCaret(2, 3, 1, 2)
  local rx, ry, rw, rh = lay:cellRect(2, 3, 1)
  ok(x > rx and x < rx + rw and y >= ry and y + h <= ry + rh, "the caret is in its cell")
  local r, k, o = lay:cellHit(2, x + 1, y + 2)
  eq(r, 3, "row") eq(k, 1, "col") eq(o, 2, "offset")
  -- Up and down by lines.
  ok(e:cellLine(lay, -1), "up a row")
  eq(e.cell.r, 2, "row after up")
  ok(e:cellLine(lay, 1), "down a row")
  eq(e.cell.r, 3)
  e:enterCell(2, 1, 1, 1)
  ok(not e:cellLine(lay, -1), "up out of the table")
  e:moveLine(lay, -1)
  eq(e.caret.b, 1) ok(e.cell == nil, "out")
  e:moveLine(lay, 1)
  ok(e.cell ~= nil, "down into the table") eq(e.cell.r, 1)
  e:cellEdge(lay, 1)
  eq(e.cell.o, 4)
  e:cellEdge(lay, -1)
  eq(e.cell.o, 0)
end)

if #failed > 0 then error(#failed .. " failed:\n" .. table.concat(failed, "\n"), 0) end
