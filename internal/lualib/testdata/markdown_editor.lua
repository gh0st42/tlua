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

if #failed > 0 then error(#failed .. " failed:\n" .. table.concat(failed, "\n"), 0) end
