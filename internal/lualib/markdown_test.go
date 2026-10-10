package lualib

import (
	"os"
	"path/filepath"
	"testing"
)

// markdownHelpers are what the tests below share: show writes runs with
// their styles, roundTrip says text comes back as written, and measure
// makes every character 8 units wide, so a 15 point line is 23 high.
const markdownHelpers = `
local md = require "markdown"
local layout = require "markdown.layout"
local function eq(got, want, what)
  if got ~= want then
    error(("%s: got %q, want %q"):format(what or "value", tostring(got), tostring(want)), 2)
  end
end
local function show(runs)
  local out = {}
  for _, r in ipairs(runs) do
    local marks = (r.b and "b" or "") .. (r.i and "i" or "") .. (r.code and "c" or "") .. (r.link and ("@" .. r.link) or "")
    out[#out + 1] = r.text:gsub("\n", "\\n") .. (marks ~= "" and ("{" .. marks .. "}") or "")
  end
  return table.concat(out, "|")
end
local function roundTrip(text)
  local again = md.write(md.parse(text))
  eq(again, text)
  eq(md.write(md.parse(again)), again, "stable")
end
local function measure(text) return #text:gsub("[\128-\191]", "") * 8 end
`

func TestMarkdownReadsAndWrites(t *testing.T) {
	run(t, markdownHelpers+`
eq(show(md.inline("a **b** *c* ***d*** `+"`e`"+` [f](g)")), "a |b{b}| |c{i}| |d{bi}| |e{c}| |f{@g}")
eq(show(md.inline("snake_case_name and _it_")), "snake_case_name and |it{i}")
eq(show(md.inline("2 * 3 * 4")), "2 * 3 * 4")
eq(show(md.inline("[**bold link**](u)")), "bold link{b@u}")
eq(show(md.inline("<https://x.org>")), "https://x.org{@https://x.org}")
eq(show(md.inline("\\*not\\* `+"`` a`b ``"+`")), "*not* |a`+"`"+`b{c}")

local b = md.parse("# One\n\ntext\n\n- a\n    - b\n1. c\n\n> q\n\n`+"```"+`lua\nx = 1\n`+"```"+`\n\n---\n\n![alt](assets/p.png)\n")
local kinds = {}
for i, x in ipairs(b) do kinds[i] = x.type .. (x.level and tostring(x.level) or "") end
eq(table.concat(kinds, " "), "h1 p ul0 ul1 ol0 quote code hr image")
eq(b[7].lang, "lua"); eq(show(b[7].runs), "x = 1")
eq(b[9].src, "assets/p.png"); eq(b[9].alt, "alt")

b = md.parse("- a\n  - b\n    - c\n- d\n")
eq(b[1].level, 0); eq(b[2].level, 1); eq(b[3].level, 2); eq(b[4].level, 0)

b = md.parse("one\\\ntwo  \nthree\nfour\n")
eq(show(b[1].runs), "one\\ntwo\\nthree four")

roundTrip("# Title *here*\n\nSome **bold**, *italic*, ***both***, `+"`code`"+` and a [link](http://x.org).\n")
roundTrip("- one\n- two\n    - nested\n1. first\n2. second\n")
roundTrip("> quoted\n>\n> more\n")
roundTrip("`+"````\\ncode ``` inside\\n````"+`\n")
roundTrip("text\\\nafter a break\n")
roundTrip("\\# not a heading\n\n1\\. not a list\n\n\\- nor this\n")
roundTrip("---\n\n![a b](assets/a%20b.png)\n")

local blocks = {
  { type = "p", runs = md.normalize { { text = "a*b_c [d] `+"`e`"+` <f> \\g" }, { text = " bold", b = true }, { text = " both", b = true, i = true } } },
  { type = "ol", level = 0, runs = { { text = "x" } } },
  { type = "ol", level = 1, runs = { { text = "y" } } },
  { type = "ol", level = 0, runs = { { text = "z" } } },
}
local back = md.parse(md.write(blocks))
eq(show(back[1].runs), "a*b_c [d] `+"`e`"+` <f> \\g |bold{b}| |both{bi}") -- spaces stay outside the marks
eq(md.write(blocks):match("\n2%. z"), "\n2. z") -- z goes on from x, past y's list
eq(back[3].level, 1)
`)
}

func TestMarkdownBlockText(t *testing.T) {
	run(t, markdownHelpers+`
local b = md.parse("Hé **x**\n\n---\n")
eq(md.textOf(b[1]), "Hé x"); eq(md.length(b[1]), 5)
eq(md.hasText(b[2]), false); eq(md.length(b[2]), 1); eq(md.textOf(b[2]), "")
eq(md.nextChar("Hé", 1), 3); eq(md.prevChar("Hé", 3), 1)
eq(md.anchor("Getting Started!"), "getting-started")
eq(md.anchor("What's new in 0.5?"), "whats-new-in-05")
`)
}

func TestMarkdownLayout(t *testing.T) {
	run(t, markdownHelpers+`
local lay = layout.build(md.parse("aaa bbb ccc ddd\n"), 64, measure)
local lines = lay.blocks[1].lines
eq(#lines, 2)
eq(lines[1].o1, 0); eq(lines[1].o2, 8)
eq(lines[2].o1, 8); eq(lines[2].o2, 15)

lay = layout.build(md.parse("abcdefghijklmnop\n"), 64, measure)
eq(#lay.blocks[1].lines, 2, "a word too long for a line is broken")

lay = layout.build(md.parse("aaa bbb ccc ddd\n\nnext\n"), 64, measure)
local x, y = lay:caret { b = 1, o = 2 }
eq(x, 16); eq(y, 0)
local p = lay:hit(17, 30)
eq(p.b, 1); eq(p.o, 10)
local down = lay:vertical({ b = 1, o = 2 }, 1)
eq(down.b, 1); eq(down.o, 10)
down = lay:vertical(down, 1, 16)
eq(down.b, 2); eq(down.o, 2)
eq(lay:lineEnd({ b = 1, o = 2 }).o, 7)
eq(lay:lineStart({ b = 1, o = 10 }).o, 8)

lay = layout.build(md.parse("1. a\n2. b\n    1. c\n3. d\n- e\n"), 300, measure)
eq(lay.blocks[1].marker, "1."); eq(lay.blocks[2].marker, "2.")
eq(lay.blocks[3].marker, "1."); eq(lay.blocks[4].marker, "3.")
eq(lay.blocks[5].marker, "•")
assert(lay.blocks[3].x > lay.blocks[1].x, "nested items are further in")

lay = layout.build(md.parse("![x](a.png)\n"), 200, measure, function() return 400, 100 end)
eq(lay.blocks[1].imageW, 200); eq(lay.blocks[1].imageH, 50)

lay = layout.build(md.parse("aaa bbb\n\nccc\n"), 300, measure)
local r = lay:rects({ b = 1, o = 4 }, { b = 2, o = 1 })
eq(#r, 2)
eq(r[1][1], 32)
eq(r[2][3], 8)
`)
}

func TestMarkdownLayoutLinksAndScale(t *testing.T) {
	run(t, markdownHelpers+`
local lay = layout.build(md.parse("go [there](b.md) now\n"), 300, measure)
eq(lay:linkAt(2, 5), nil, "before the link")
local url, p = lay:linkAt(30, 5)
eq(url, "b.md"); eq(p.b, 1); eq(p.o, 4)
eq(lay:linkAt(30, 40), nil, "below the line")

-- At twice the size, everything is twice as far: a paragraph's line is 45
-- high, and so is its space after.
local one = layout.build(md.parse("a\n\nb\n"), 300, measure)
local two = layout.build(md.parse("a\n\nb\n"), 300, measure, nil, nil, { scale = 2 })
eq(one.blocks[2].y, 33); eq(two.blocks[2].y, 65)
eq(two.blocks[1].lines[1].frags[1].font.size, 30)
eq(two.scale, 2)

-- A layout cache keeps lines for one scale apart from another's.
local cache = {}
layout.build(md.parse("a\n"), 300, measure, nil, cache)
local big = layout.build(md.parse("a\n"), 300, measure, nil, cache, { scale = 2 })
eq(big.blocks[1].lines[1].h, 45)
`)
}

// A program's own markdown.lua is the one it gets.
func TestMarkdownOfTheProgramsOwnComesFirst(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "markdown.lua"), []byte(`return { mine = true }`), 0o644); err != nil {
		t.Fatal(err)
	}
	runIn(t, map[string]string{"dir": dir}, `
		package.path = dir .. "/?.lua;" .. package.path
		assert(require("markdown").mine, "the program's own")
		-- tlua's layout uses tlua's markdown all the same.
		local lay = require("markdown.layout").build(require("tlua.markdown").parse("a b\n"), 100,
			function(text) return #text end)
		assert(#lay.blocks == 1)
	`)
}

func TestMarkdownTables(t *testing.T) {
	run(t, markdownHelpers+`
local b = md.parse("| a | **b** | c |\n|---|:-:|--:|\n| 1 | `+"`x\\\\|y`"+` | [[Page|go]] |\n| 2 |\n\nafter\n")
eq(#b, 2); eq(b[1].type, "table"); eq(b[2].type, "p")
local t = b[1]
eq(t.align[1], false); eq(t.align[2], "center"); eq(t.align[3], "right")
eq(#t.rows, 3)
eq(show(t.rows[1][2]), "b{b}")
eq(show(t.rows[2][2]), "x|y{c}", "an escaped pipe in code")
eq(show(t.rows[2][3]), "go{@Page.md}", "a wiki link's pipe is its own")
eq(#t.rows[3][2], 0, "a short row is filled out")
eq(md.hasText(t), false); eq(md.length(t), 1)
roundTrip("| a | **b** | c |\n| --- | :---: | ---: |\n| 1 | `+"`x\\\\|y`"+` | [[Page|go]] |\n| 2 |  |  |\n")
roundTrip("| one |\n| :--- |\n| a \\| b |\n")

-- Not tables: no delimiter row, or one that does not match the header.
eq(md.parse("a | b\nc | d\n")[1].type, "p")
eq(md.parse("| a | b |\n|---|\n")[1].type, "p")
`)
}

func TestMarkdownWikiLinksAndContents(t *testing.T) {
	run(t, markdownHelpers+`
eq(show(md.inline("see [[Getting Started]] and [[Notes#todo|the list]] or [[a.txt]]")),
  "see |Getting Started{@Getting Started.md}| and |the list{@Notes.md#todo}| or |a.txt{@a.txt}")
roundTrip("see [[Getting Started]] and [[Notes#todo|the list]]\n")
eq(md.inline("[[]]")[1].text, "[[]]", "an empty one is text")

local b = md.parse("[[toc]]\n\n# A\n\n## B\n\n## B\n\n[[TOC]]\n")
eq(b[1].type, "toc"); eq(b[5].type, "toc")
eq(md.write(b), "[[toc]]\n\n# A\n\n## B\n\n## B\n\n[[toc]]\n")
local hs = md.headings(b)
eq(#hs, 3); eq(hs[1].level, 1); eq(hs[2].anchor, "b"); eq(hs[3].anchor, "b-1"); eq(hs[3].block, 4)
`)
}

func TestMarkdownLayoutOfTablesAndContents(t *testing.T) {
	run(t, markdownHelpers+`
-- Columns as wide as their widest cell, with 6 of padding each side.
local lay = layout.build(md.parse("| ab | c |\n|---|--:|\n| abcd | [x](u) |\n"), 300, measure)
local L = lay.blocks[1]
eq(L.colW[1], 4 * 8 + 12); eq(L.colW[2], 8 + 12); eq(L.tableW, 64)
eq(#L.rows, 2); eq(L.rows[2].y, L.rows[1].h)
-- The link in the right-aligned cell can be clicked where it is drawn.
local sub = L.subs[#L.subs]
eq(sub.x, L.colX[2] + 6)
eq((lay:linkAt(sub.x + 2, L.rows[2].y + 8)), "u")
eq(lay:linkAt(2, L.rows[2].y + 8), nil)

-- Too wide: the long cell wraps, the short one keeps its word.
lay = layout.build(md.parse("| a | b |\n|---|---|\n| xx | aaa bbb ccc ddd eee fff |\n"), 100, measure)
L = lay.blocks[1]
assert(L.tableW <= 100, "fits: " .. L.tableW)
assert(L.colW[1] >= 2 * 8 + 12, "a word is not broken")
assert(L.rows[2].h > 2 * 23, "the long cell wraps")

-- A list of contents links to each heading, indented by level.
lay = layout.build(md.parse("[[toc]]\n\n# A\n\n## B b\n"), 300, measure)
L = lay.blocks[1]
eq(#L.subs, 2)
eq(L.subs[1].x, 0); eq(L.subs[2].x, 24)
eq((lay:linkAt(26, L.subs[2].y + 5)), "#b-b")
lay = layout.build(md.parse("[[toc]]\n"), 300, measure)
eq(lay.blocks[1].empty, true)
`)
}
