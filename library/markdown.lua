---@meta markdown
--- Markdown read into blocks and written back, `require "markdown"`: what
--- a gui MarkdownView shows. `markdown.layout` lays the blocks out on a page
--- and `markdown.render` draws them on a Canvas, for controls of your own;
--- `markdown.editor` edits them, as a gui MarkdownEdit does.
--- A program's own markdown.lua, if it has one, is found first.

local markdown = {}

--- A piece of a block's text, in one style.
---@class markdown.Run
---@field text string "\n" in it is a line break
---@field b? boolean bold
---@field i? boolean italic
---@field code? boolean
---@field link? string where it links to
---@field wiki? string for [[Page]]: what the brackets held, to write it back

--- One block of a document: "p", "h1" to "h6", "quote", "ul" and "ol" (a
--- list item, with its level), "code" (with its lang), "hr", "image" (with
--- src and alt), "table" (with align and rows, the first the header, each
--- cell runs) or "toc" ([[toc]], a list of the headings).
---@class markdown.Block
---@field type string
---@field runs? markdown.Run[]
---@field level? integer 0 to 3, for a list item
---@field lang? string
---@field src? string
---@field alt? string
---@field align? (string|false)[] "left", "center", "right", or false, a column at a time
---@field rows? markdown.Run[][][]

---@class markdown.Heading
---@field level integer 1 to 6
---@field text string
---@field anchor string what a link to it names after #
---@field block integer which block it is

--- The blocks a Markdown text is made of.
---@param text string
---@--- A document's headings, in order, with the anchors links name them by.
---@param blocks markdown.Block[]
---@return markdown.Heading[]
function markdown.headings(blocks) end

--- Where [[target]] leads: "Page Name" is "Page Name.md".
---@param target string
---@return string
function markdown.wikiLink(target) end

return markdown.Block[]
function markdown.parse(text) end

--- Markdown for blocks, in a steady form: it reads back as the same blocks.
---@param blocks markdown.Block[]
---@return string
function markdown.write(blocks) end

--- The runs one line of inline Markdown is made of.
---@param text string
---@return markdown.Run[]
function markdown.inline(text) end

--- Inline Markdown for runs.
---@param runs markdown.Run[]
---@return string
function markdown.writeRuns(runs) end

--- Joins runs of the same style and leaves out empty ones, in place.
---@param runs markdown.Run[]
---@return markdown.Run[]
function markdown.normalize(runs) end

---@param a markdown.Run
---@param b markdown.Run
---@return boolean
function markdown.sameStyle(a, b) end

--- A block's text, its styles left out: "" for a picture or a rule.
---@param block markdown.Block
---@return string
function markdown.textOf(block) end

--- How many positions a block has: its text's bytes, or 1 for a picture or
--- a rule.
---@param block markdown.Block
---@return integer
function markdown.length(block) end

--- Whether a block holds text: all but pictures and rules.
---@param block markdown.Block
---@return boolean
function markdown.hasText(block) end

--- The offset of the UTF-8 character after the one at offset o.
---@param text string
---@param o integer
---@return integer
function markdown.nextChar(text, o) end

--- The offset of the UTF-8 character before offset o.
---@param text string
---@param o integer
---@return integer
function markdown.prevChar(text, o) end

--- The anchor a heading is linked to by, as GitHub makes them:
--- "Getting Started!" is "getting-started".
---@param text string
---@return string
function markdown.anchor(text) end

--- A document being edited: `require("markdown.editor").new(blocks)`, or a
--- MarkdownEdit's `ed`. A position is { b = block, o = byte offset }.
---@class markdown.Editor
---@field blocks markdown.Block[]
---@field caret {b: integer, o: integer}
---@field anchor {b: integer, o: integer} where the selection starts
---@field dirty boolean
local Editor = {}

---@param blocks markdown.Block[]
function Editor:load(blocks) end
---@return string
function Editor:markdown() end
---@param text string typed at the caret, over the selection
function Editor:type(text) end
function Editor:enter() end
function Editor:backspace() end
function Editor:undo() end
function Editor:redo() end
---@return boolean
function Editor:canUndo() end
---@return boolean
function Editor:canRedo() end
---@param key "b"|"i"|"code" bold, italic or code, over the selection or for what is typed next
function Editor:toggle(key) end
---@param key "b"|"i"|"code"
---@return boolean
function Editor:isOn(key) end
---@param t "p"|"h1"|"h2"|"h3"|"quote"|"code"|"ul"|"ol"
---@param toggle? boolean back to a paragraph when it is that already
function Editor:setType(t, toggle) end
---@return string
function Editor:blockType() end
---@param url string "" takes the link off
function Editor:setLink(url) end
---@param p? {b: integer, o: integer}
---@return string? url
function Editor:linkAt(p) end
---@param src string
---@param alt? string
function Editor:insertImage(src, alt) end
function Editor:insertRule() end
---@param text string Markdown, or plain text
function Editor:paste(text) end
---@return string Markdown of the selection
function Editor:copyText() end
function Editor:deleteSelection() end
function Editor:selectAll() end
---@return boolean
function Editor:hasSelection() end
---@return integer words
---@return integer characters
function Editor:stats() end

return markdown
