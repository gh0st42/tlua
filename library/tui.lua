---@meta tui
--- Turbo Vision-like windows and controls in the terminal, `require "tui"`,
--- from turbolua, on term. Positions are columns and rows from 1, inside
--- the parent. Keys arrive by name: "up", "enter", "tab", "esc", "ctrl-q",
--- or the character typed.

local tui = {}

---@alias tui.Key string

--- The colours each kind of view is drawn in; change it to restyle all.
tui.theme = {
	desktop = { bg = "blue" },
	window = { border = "white", background = "cyan", titlefg = "yellow" },
	normal = { fg = "black", bg = "cyan" },
	focus = { fg = "yellow", bg = "black" },
	selected = { fg = "black", bg = "lightgray" },
	button = { fg = "black", bg = "lightgray" },
	statusbar = { fg = "black", bg = "lightgray" },
}

--- A class with base's methods; `init(self, ...)` runs on `new(...)`.
---@param base? table
---@return table
function tui.class(base) end

--- The next key from the terminal, by name.
---@return tui.Key?
function tui.readkey() end

---@class tui.View
---@field x integer
---@field y integer
---@field w integer
---@field h integer
---@field visible boolean
---@field focusable boolean
---@field parent? tui.View
---@field children? tui.View[]
tui.View = {}

---@return tui.View
function tui.View.new(x, y, w, h) end

---@return integer x, integer y
function tui.View:abspos() end

---@return integer x, integer y
function tui.View:origin() end

---@generic T: tui.View
---@param view T
---@return T
function tui.View:addchild(view) end

function tui.View:draw() end

--- Handles a key; true when it was used.
---@param key tui.Key
---@return boolean
function tui.View:handlekey(key) end

---@return tui.View?
function tui.View:focused() end

---@return boolean
function tui.View:isfocused() end

function tui.View:focusnext() end
function tui.View:focusprev() end

---@class tui.Label: tui.View
---@field text string
tui.Label = {}
---@return tui.Label
function tui.Label.new(x, y, text) end

---@class tui.Button: tui.View
---@field text string
---@field onclick? fun(self: tui.Button)
tui.Button = {}
---@param onclick? fun(self: tui.Button)
---@return tui.Button
function tui.Button.new(x, y, text, onclick) end

---@class tui.CheckBox: tui.View
---@field text string
---@field checked boolean
---@field onchange? fun(self: tui.CheckBox, checked: boolean)
tui.CheckBox = {}
---@return tui.CheckBox
function tui.CheckBox.new(x, y, text, checked) end

---@class tui.RadioButtons: tui.View
---@field items string[]
---@field selected integer
---@field onchange? fun(self: tui.RadioButtons, selected: integer)
tui.RadioButtons = {}
---@param items string[]
---@return tui.RadioButtons
function tui.RadioButtons.new(x, y, items, selected) end

---@class tui.InputLine: tui.View
---@field text string
---@field maxlen integer
tui.InputLine = {}
---@return tui.InputLine
function tui.InputLine.new(x, y, w, text, maxlen) end

---@class tui.ListBox: tui.View
---@field items string[]
---@field selected integer
---@field onselect? fun(self: tui.ListBox, index: integer, item: string)
tui.ListBox = {}
---@param items string[]
---@return tui.ListBox
function tui.ListBox.new(x, y, w, h, items) end

---@class tui.StatusBar: tui.View
---@field text string
tui.StatusBar = {}
---@return tui.StatusBar
function tui.StatusBar.new(y, w, text) end

---@class tui.Window: tui.View
---@field title? string
tui.Window = {}
---@return tui.Window
function tui.Window.new(x, y, w, h, title) end

---@class tui.Dialog: tui.Window
---@field result any
tui.Dialog = {}
---@return tui.Dialog
function tui.Dialog.new(x, y, w, h, title) end

--- Ends runmodal, which returns result.
function tui.Dialog:close(result) end

--- Draws the dialog over app and handles keys until it is closed (Esc
--- closes it with nil).
---@param app tui.Application
---@return any result
function tui.Dialog:runmodal(app) end

--- The screen, and the loop that reads and hands out keys.
---@class tui.Application: tui.View
---@field running boolean
tui.Application = {}
---@return tui.Application
function tui.Application.new() end

--- Raw keys and no cursor until quit (or Ctrl+Q); the terminal is put
--- back afterwards, also when a callback raises an error.
function tui.Application:run() end

function tui.Application:quit() end

---@param overlay? tui.View
function tui.Application:redraw(overlay) end

---@return tui.Key?
function tui.Application:readkey() end

--- A message with a button for each label; the label pressed, or nil.
---@param app tui.Application
---@param text string
---@param buttons? string[] { "OK" } when left out
---@return string?
function tui.messagebox(app, text, buttons) end

return tui
