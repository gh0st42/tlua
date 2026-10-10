---@meta markdown
--- Markdown read into blocks and written back, `require "markdown"`: what
--- a gui MarkdownView shows. `markdown.layout` lays the blocks out on a page
--- and `markdown.render` draws them on a Canvas, for controls of your own.
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

return markdown
