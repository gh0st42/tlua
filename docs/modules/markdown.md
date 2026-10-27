# `markdown` — Markdown as blocks

`markdown` reads Markdown into a list of blocks and writes it back. It is
tlua's own, and what the gui's MarkdownView and MarkdownEdit are made of, and
what [`tlua doc`](README.md) draws these pages with. It reads the usual
Markdown, with GitHub's tables, `[[toc]]` for a list of the headings, and
`[[Page]]` wiki links.

```lua
local markdown = require "markdown"
```

A program's own `markdown.lua`, if it has one, is found first. Three more
modules lay out, draw and edit what this one reads:
`markdown.layout`, `markdown.render` and `markdown.editor`.

## Blocks and runs

A document is a list of **blocks**. Each has a `type`:

| `type` | |
|---|---|
| `"p"` | a paragraph |
| `"h1"` to `"h6"` | a heading |
| `"quote"` | a block quote |
| `"ul"`, `"ol"` | one item of a list, with its `level` (0 to 3) |
| `"code"` | a block of code, with its `lang` |
| `"hr"` | a rule |
| `"image"` | a picture, with `src` and `alt` |
| `"table"` | with `align` (`"left"`, `"center"`, `"right"` or `false` for each column) and `rows`, the first row the header; each cell is a list of runs |
| `"toc"` | `[[toc]]`: a list of the headings |

The text of a block is its `runs`: pieces in one style each. A run has
`text` (with `"\n"` for a line break) and, when they apply, `b` (bold), `i`
(italic), `code`, `link` (where it leads) and `wiki` (for `[[Page]]`: what
the brackets held, so that it can be written back).

## Reading

### `markdown.parse(text)`

The blocks of a Markdown text.

```lua
local blocks = markdown.parse("# Title\n\nSome *emphasis* and **bold**.\n\n- one\n- two\n")
for i, b in ipairs(blocks) do
  print(i, b.type, markdown.textOf(b))
end
--> 1   h1   Title
--> 2   p    Some emphasis and bold.
--> 3   ul   one
--> 4   ul   two
```

```lua
for _, run in ipairs(blocks[2].runs) do
  io.write("[", run.text, run.i and " italic" or "", run.b and " bold" or "", "]")
end
--> [Some ][emphasis italic][ and ][bold bold][.]
```

### `markdown.inline(text)`

The runs of one line of inline Markdown, without making a block of it.

### `markdown.headings(blocks)`

The headings, in order: for each, its `level`, its `text`, its `anchor`
(what a link to it names after `#`) and which `block` it is.

```lua
for _, h in ipairs(markdown.headings(blocks)) do
  print(h.level, h.text, h.anchor)       --> 1   Title   title
end
```

### `markdown.anchor(text)`

The anchor a heading gets, as GitHub makes them:

```lua
print(markdown.anchor("Getting Started!"))   --> getting-started
```

### `markdown.wikiLink(target)`

Where `[[target]]` leads: the file it names.

```lua
print(markdown.wikiLink("Page Name"))        --> Page Name.md
```

## Writing

### `markdown.write(blocks)`

Markdown for the blocks, in a steady form: what it writes reads back as the
same blocks, so a program can load a document, change it and save it without
the file shifting about.

```lua
local text = markdown.write(blocks)
```

### `markdown.writeRuns(runs)`

Inline Markdown for a list of runs.

## Working with text

Positions in a block count bytes of its text, and these keep to whole UTF-8
characters.

- `markdown.textOf(block)`: the text of a block with its styles left out;
  `""` for a picture or a rule.
- `markdown.length(block)`: how many positions a block has: its bytes, or 1
  for a picture or a rule.
- `markdown.hasText(block)`: whether a block holds text; pictures and rules
  do not.
- `markdown.nextChar(text, o)`, `markdown.prevChar(text, o)`: the offset of
  the character after or before offset `o`.
- `markdown.normalize(runs)`: joins neighbouring runs of one style and drops
  empty ones, in place.
- `markdown.sameStyle(a, b)`: whether two runs are styled alike.

## Editing

### `markdown.editor`

```lua
local editor = require "markdown.editor"
local ed = editor.new(markdown.parse("one two one"))
```

An editor is a document with a caret and a selection, as a MarkdownEdit
control has. A position is `{ b = block, o = byte offset }`; the editor has
`blocks`, `caret`, `anchor` (where the selection starts) and `dirty`.

- **Writing:** `ed:type(text)`, `ed:enter()`, `ed:backspace()`,
  `ed:paste(text)`, `ed:deleteSelection()`, `ed:selectAll()`,
  `ed:hasSelection()`, `ed:copyText()` (the selection as Markdown).
- **Undo:** `ed:undo()`, `ed:redo()`, `ed:canUndo()`, `ed:canRedo()`.
- **Style and structure:** `ed:toggle("b")`, `ed:isOn(key)`,
  `ed:setType(type)`, `ed:blockType()`, `ed:setLink(url)`, `ed:linkAt(p)`,
  `ed:insertImage(src, alt)`, `ed:insertRule()`.
- **Tables:** `ed:insertTable(rows, columns)`, `ed:insertRow(after)`,
  `ed:insertColumn(after)`, `ed:deleteRow()`, `ed:deleteColumn()`,
  `ed:setAlign(how)`, `ed:cellTab(dir)`, `ed:enterCell(b, r, k)`,
  `ed:leaveTable(before)`, `ed:tableAt()`.
- **Out:** `ed:markdown()` is the document as Markdown, `ed:load(blocks)`
  puts another in, `ed:stats()` returns the number of words and characters.

### Find and replace

`ed:find(needle, options, back)` selects the next match after the selection
(before it with `back`) and returns whether there was one, and whether it
went round the end of the document to find it. `options` is a table with
`matchCase`, `whole` (a match is a whole word) and `wrap` (on unless false).
Tables are not searched.

```lua
local ed = editor.new(markdown.parse("one two one"))
print(ed:countMatches("one"))             --> 2   0   (how many; which is selected)
print(ed:replaceAll("one", "1"))          --> 2
print(ed:find("two"))                     --> true   false
```

- `ed:isMatch(needle, options)`: whether the selection is a match exactly.
- `ed:countMatches(needle, options)`: how many matches there are, and which
  the selection is (0 for none).
- `ed:replace(needle, with, options)`: puts `with` in place of the selection
  if it is a match, in its style, and selects the next one.
- `ed:replaceAll(needle, with, options)`: every match, as one step of undo;
  returns how many.

A MarkdownView control searches too: see [gui](../gui.md#formatted-text-markdownview).
