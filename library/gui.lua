---@meta gui
--- The desktop GUI module, `require "gui"`, written out for a language server.
---
--- This file is never run: it declares what every call takes and gives back, so
--- that an editor with lua-language-server behind it can complete these names,
--- show their arguments while they are being typed, and check the tables
--- controls are made from. The .luarc.json at the root points the server here.
---
--- docs/gui.md is the same API in prose, with what each call means.

--- Makes this program a desktop application: from here on `form:show()` returns
--- at once, and the program runs until its last form is closed. Returns the
--- gui module, so `local gui = bootgui()` is all a program needs.
---@return gui
function bootgui() end

---@alias gui.Color string `"#rrggbb"`, `"#rgb"`, or black, white, gray, red, green, blue, yellow, orange, purple
---@alias gui.Font "sans"|"serif"|"mono"
---@alias gui.Align "left"|"center"|"right"
---@alias gui.Buttons "ok"|"okcancel"|"yesno"|"yesnocancel"|"retrycancel"
---@alias gui.Answer "ok"|"cancel"|"yes"|"no"|"retry"

----------------------------------------------------------------------------
-- What every object has.

---@class gui.Object
---@field name string what its form calls it: frm.<name>; unique on the form
---@field parent? gui.Object what holds it; add() moves it
---@field caption string
---@field left integer measured from the container's corner
---@field top integer
---@field width integer
---@field height integer
---@field visible boolean
---@field enabled boolean
---@field tooltip string
---@field color? gui.Color the background
---@field textColor? gui.Color
---@field font? gui.Font
---@field fontSize? integer
---@field grow boolean stretches with a resizable form; given when it is made
---@field onDrop? fun(self: self, text: string, lines: string[]) something was dropped on it; files arrive as one path per line
---@field onDrag? fun(self: self): string? the user drags out of it; return the text to carry
---@field [string] any fields of the script's own, which controls written in Lua keep their state in
local Object = {}

--- Sets a handler: `obj:on("click", fn)` is `obj.onClick = fn`, and nil removes it.
---@param event string "click" or "onClick", and the like
---@param fn? function
---@return self
function Object:on(event, fn) end

--- Calls the object's handler for an event, if it has one, with self and the
--- arguments given, and returns what the handler returns. A control written in
--- Lua raises its own events with this.
---@param event string
---@param ... any
---@return any
function Object:fire(event, ...) end

--- Gives it the keyboard.
function Object:focus() end

--- Takes it off its container and frees what it had on screen. It can be
--- added somewhere again.
function Object:remove() end

--- Puts it in front of the others in its container.
function Object:raise() end

--- Puts it behind the others in its container.
function Object:lower() end

--- The control of that name on this object's form, or nil.
---@param name string
---@return gui.Object?
function Object:find(name) end

--- Asks for it to be drawn again: a Canvas whose picture has changed, mostly.
function Object:redraw() end

--- What every object can be made with.
---@class gui.Options
---@field name? string
---@field caption? string
---@field left? integer
---@field top? integer
---@field width? integer
---@field height? integer
---@field visible? boolean
---@field enabled? boolean
---@field tooltip? string
---@field color? gui.Color
---@field textColor? gui.Color
---@field font? gui.Font
---@field fontSize? integer
---@field grow? boolean
---@field parent? gui.Container where `gui.Button{}` and the like go, instead of the latest Form
---@field onDrop? fun(self: gui.Object, text: string, lines: string[])
---@field onDrag? fun(self: gui.Object): string?

----------------------------------------------------------------------------
-- Containers: what holds controls, and makes them.

---@class gui.Container: gui.Object
local Container = {}

--- Adopts a control, moving it from wherever it was.
---@generic T: gui.Object
---@param control T
---@return T
function Container:add(control) end

---@param opts? gui.LabelOptions
---@return gui.Label
function Container:Label(opts) end

---@param opts? gui.ButtonOptions
---@return gui.Button
function Container:Button(opts) end

---@param opts? gui.TextBoxOptions
---@return gui.TextBox
function Container:TextBox(opts) end

---@param opts? gui.CheckBoxOptions
---@return gui.CheckBox
function Container:CheckBox(opts) end

--- One radio button among those sharing a parent is on.
---@param opts? gui.CheckBoxOptions
---@return gui.RadioButton
function Container:RadioButton(opts) end

---@param opts? gui.ComboBoxOptions
---@return gui.ComboBox
function Container:ComboBox(opts) end

---@param opts? gui.ListBoxOptions
---@return gui.ListBox
function Container:ListBox(opts) end

---@param opts? gui.TreeOptions
---@return gui.Tree
function Container:Tree(opts) end

---@param opts? gui.TableOptions
---@return gui.Table
function Container:Table(opts) end

---@param opts? gui.SliderOptions
---@return gui.Slider
function Container:Slider(opts) end

---@param opts? gui.SpinnerOptions
---@return gui.Spinner
function Container:Spinner(opts) end

---@param opts? gui.ProgressBarOptions
---@return gui.ProgressBar
function Container:ProgressBar(opts) end

---@param opts? gui.ImageOptions
---@return gui.Image
function Container:Image(opts) end

---@param opts? gui.CanvasOptions
---@return gui.Canvas
function Container:Canvas(opts) end

---@param opts? gui.Options
---@return gui.Frame
function Container:Frame(opts) end

---@param opts? gui.TabsOptions
---@return gui.Tabs
function Container:Tabs(opts) end

--- A group of controls with no border or caption.
---@param opts? gui.Options
---@return gui.Panel
function Container:Panel(opts) end

--- Shows part of what it holds, with scrollbars for the rest.
---@param opts? gui.Options
---@return gui.Scroll
function Container:Scroll(opts) end

--- Its controls tile it edge to edge, and the user drags the lines between.
---@param opts? gui.Options
---@return gui.Splitter
function Container:Splitter(opts) end

----------------------------------------------------------------------------
-- Forms.

---@class gui.Form: gui.Container
---@field resizable boolean given when it is made
---@field onClose? fun(self: gui.Form): boolean? the close box, Escape or close(); false keeps it open
---@field onUnload? fun(self: gui.Form): boolean? runs after onClose; false keeps it open too
---@field onKey? fun(self: gui.Form, key: string, text: string): boolean? a key the focused control did not use, as "a", "Ctrl+s", "F5"; true keeps it from going further
---@field onResize? fun(self: gui.Form)
local Form = {}

--- Puts the form up. Under bootgui() that is all; otherwise it also waits
--- until the form is closed, and raises any error a handler raised meanwhile.
function Form:show() end

--- Puts the form up in front of the others and waits until it is closed.
function Form:showModal() end

--- Closes the form, as its close box would: onClose and onUnload can refuse.
function Form:close() end

--- The form's menu bar, along its top: leave it 25 pixels.
---@param items gui.MenuItem[]
---@return gui.Menu
function Form:Menu(items) end

---@class gui.FormOptions: gui.Options
---@field resizable? boolean
---@field onClose? fun(self: gui.Form): boolean?
---@field onUnload? fun(self: gui.Form): boolean?
---@field onKey? fun(self: gui.Form, key: string, text: string): boolean?
---@field onResize? fun(self: gui.Form)

---@class gui.Menu: gui.Object
---@field items gui.MenuItem[] assign it again after changing it

--- `{"&Open", fn, shortcut = "Cmd+O"}`, `{"&File", {...}}` for a submenu, or
--- "-" for a line between items. Cmd is Command on a Mac and Ctrl elsewhere.
---@alias gui.MenuItem string|gui.MenuEntry

---@class gui.MenuEntry
---@field [1] string the caption; & marks the letter to underline
---@field [2]? fun(caption: string, checked: boolean)|gui.MenuItem[]
---@field shortcut? string
---@field checked? boolean makes it a toggle
---@field enabled? boolean
---@field onClick? fun(caption: string, checked: boolean)

----------------------------------------------------------------------------
-- Controls.

---@class gui.Label: gui.Object
---@field text string the same as its caption
---@field align gui.Align
---@field image string a picture beside the caption

---@class gui.LabelOptions: gui.Options
---@field text? string
---@field align? gui.Align
---@field image? string

---@class gui.Button: gui.Object
---@field default boolean Enter presses it; given when it is made
---@field image string a picture beside the caption
---@field onClick? fun(self: gui.Button)

---@class gui.ButtonOptions: gui.Options
---@field default? boolean
---@field image? string
---@field onClick? fun(self: gui.Button)

---@class gui.TextBox: gui.Object
---@field text string
---@field value string the same as text
---@field multiLine boolean given when it is made, like password and readOnly
---@field password boolean
---@field readOnly boolean
---@field lineNumbers boolean given when it is made, like syntax and acceptsTab
---@field syntax ""|"lua" colours the text as Lua
---@field acceptsTab boolean Tab indents and Enter keeps the indentation
---@field line integer the cursor's line, from 1; setting it moves there
---@field cursor integer how many bytes come before the cursor
---@field selectedText string what is selected
---@field onChange? fun(self: gui.TextBox)

local TextBox = {}

--- Selects bytes i to j, counted as string.sub counts them, and puts the
--- cursor after them.
---@param i integer
---@param j? integer
function TextBox:select(i, j) end

---@class gui.TextBoxOptions: gui.Options
---@field text? string
---@field multiLine? boolean
---@field password? boolean
---@field readOnly? boolean
---@field lineNumbers? boolean
---@field syntax? ""|"lua"
---@field acceptsTab? boolean
---@field onChange? fun(self: gui.TextBox)

---@class gui.CheckBox: gui.Object
---@field checked boolean
---@field value boolean the same as checked
---@field onChange? fun(self: gui.CheckBox)

---@class gui.RadioButton: gui.CheckBox

---@class gui.CheckBoxOptions: gui.Options
---@field checked? boolean
---@field onChange? fun(self: gui.CheckBox)

---@class gui.ComboBox: gui.Object
---@field items string[] assign it again after changing it
---@field selected integer 1 for the first item, 0 for none
---@field text string the selected item; assigning one selects it
---@field onChange? fun(self: gui.ComboBox)

---@class gui.ComboBoxOptions: gui.Options
---@field items? string[]
---@field selected? integer
---@field onChange? fun(self: gui.ComboBox)

---@class gui.ListBox: gui.ComboBox
---@field onDoubleClick? fun(self: gui.ListBox)

---@class gui.ListBoxOptions: gui.ComboBoxOptions
---@field onDoubleClick? fun(self: gui.ListBox)

--- A string is a leaf; `{"label", {children...}, open = true}` is a branch.
---@alias gui.TreeItem string|gui.TreeBranch

---@class gui.TreeBranch
---@field [1] string
---@field [2]? gui.TreeItem[]
---@field open? boolean kept up to date as the branch is opened and closed

---@class gui.Tree: gui.Object
---@field items gui.TreeItem[] assign it again after changing it
---@field path string the selected node, its labels joined with "/"; "" for none; assigning one selects it
---@field text string the selected node's own label
---@field onChange? fun(self: gui.Tree)
---@field onDoubleClick? fun(self: gui.Tree)
---@field onToggle? fun(self: gui.Tree, path: string, open: boolean)

---@class gui.TreeOptions: gui.Options
---@field items? gui.TreeItem[]
---@field path? string
---@field onChange? fun(self: gui.Tree)
---@field onDoubleClick? fun(self: gui.Tree)
---@field onToggle? fun(self: gui.Tree, path: string, open: boolean)

---@class gui.Table: gui.Object
---@field columns string[]
---@field rows any[][] each row a table of cells; assign it again after changing it
---@field columnWidths? integer[]
---@field selected integer 1 for the first row, 0 for none
---@field onChange? fun(self: gui.Table)
---@field onDoubleClick? fun(self: gui.Table)

---@class gui.TableOptions: gui.Options
---@field columns? string[]
---@field rows? any[][]
---@field columnWidths? integer[]
---@field selected? integer
---@field onChange? fun(self: gui.Table)
---@field onDoubleClick? fun(self: gui.Table)

---@class gui.Slider: gui.Object
---@field min number
---@field max number
---@field step number
---@field value number
---@field vertical boolean given when it is made
---@field onChange? fun(self: gui.Slider)

---@class gui.SliderOptions: gui.Options
---@field min? number
---@field max? number
---@field step? number
---@field value? number
---@field vertical? boolean
---@field onChange? fun(self: gui.Slider)

---@class gui.Spinner: gui.Object
---@field min number
---@field max number
---@field step number a step with a fraction takes fractions
---@field value number
---@field onChange? fun(self: gui.Spinner)

---@class gui.SpinnerOptions: gui.Options
---@field min? number
---@field max? number
---@field step? number
---@field value? number
---@field onChange? fun(self: gui.Spinner)

---@class gui.ProgressBar: gui.Object
---@field min number
---@field max number
---@field value number

---@class gui.ProgressBarOptions: gui.Options
---@field min? number
---@field max? number
---@field value? number

---@class gui.Image: gui.Object
---@field file string PNG, JPEG, BMP, SVG or GIF; a relative path is looked for next to the script
---@field fit boolean grown to fit as well as shrunk

---@class gui.ImageOptions: gui.Options
---@field file? string
---@field fit? boolean

---@class gui.Canvas: gui.Object
---@field transparent boolean draws only what onDraw draws, over what is under it; given when it is made
---@field onKey? fun(self: gui.Canvas, key: string, text: string): boolean? clicking it takes the keyboard
---@field onDraw? fun(self: gui.Canvas, g: gui.Graphics)
---@field onMouseDown? fun(self: gui.Canvas, x: integer, y: integer, button: integer, double: boolean) button 1 is left, 2 middle, 3 right; double for a double click
---@field onMouseUp? fun(self: gui.Canvas, x: integer, y: integer, button: integer)
---@field onMouseDrag? fun(self: gui.Canvas, x: integer, y: integer)
---@field onMouseMove? fun(self: gui.Canvas, x: integer, y: integer)
---@field onMouseWheel? fun(self: gui.Canvas, dx: integer, dy: integer)
---@field onMouseEnter? fun(self: gui.Canvas)
---@field onMouseLeave? fun(self: gui.Canvas)

---@class gui.CanvasOptions: gui.Options
---@field transparent? boolean draws only what onDraw draws, over what is under it; given when it is made
---@field onKey? fun(self: gui.Canvas, key: string, text: string): boolean? clicking it takes the keyboard
---@field onDraw? fun(self: gui.Canvas, g: gui.Graphics)
---@field onMouseDown? fun(self: gui.Canvas, x: integer, y: integer, button: integer, double: boolean)
---@field onMouseUp? fun(self: gui.Canvas, x: integer, y: integer, button: integer)
---@field onMouseDrag? fun(self: gui.Canvas, x: integer, y: integer)
---@field onMouseMove? fun(self: gui.Canvas, x: integer, y: integer)
---@field onMouseWheel? fun(self: gui.Canvas, dx: integer, dy: integer)
---@field onMouseEnter? fun(self: gui.Canvas)
---@field onMouseLeave? fun(self: gui.Canvas)

---@class gui.Frame: gui.Container
---@class gui.Panel: gui.Container
---@class gui.Scroll: gui.Container
---@class gui.Splitter: gui.Container

---@class gui.Tabs: gui.Object
---@field selected integer 1 for the first page
---@field onChange? fun(self: gui.Tabs)
local Tabs = {}

---@param opts? gui.Options its caption is the tab's
---@return gui.Page
function Tabs:Page(opts) end

---@class gui.TabsOptions: gui.Options
---@field selected? integer
---@field onChange? fun(self: gui.Tabs)

---@class gui.Page: gui.Container

----------------------------------------------------------------------------
-- Drawing on a Canvas.

--- What a Canvas's onDraw draws with, in the canvas's own coordinates. It only
--- works inside onDraw.
---@class gui.Graphics
local Graphics = {}

---@return integer width, integer height
function Graphics:size() end

---@param c gui.Color
function Graphics:color(c) end

---@param n integer
function Graphics:width(n) end

---@param name gui.Font
---@param size? integer 14 by default
function Graphics:font(name, size) end

---@param x number
---@param y number
function Graphics:point(x, y) end

---@param x1 number
---@param y1 number
---@param x2 number
---@param y2 number
function Graphics:line(x1, y1, x2, y2) end

---@param x number
---@param y number
---@param w number
---@param h number
function Graphics:rect(x, y, w, h) end

---@param x number
---@param y number
---@param w number
---@param h number
function Graphics:fill(x, y, w, h) end

---@param x number
---@param y number
---@param r number
function Graphics:circle(x, y, r) end

---@param x number
---@param y number
---@param r number
function Graphics:disc(x, y, r) end

--- Part of the ellipse in that box, from angle a1 to a2 in degrees.
---@param x number
---@param y number
---@param w number
---@param h number
---@param a1 number
---@param a2 number
function Graphics:arc(x, y, w, h, a1, a2) end

---@param x number
---@param y number
---@param w number
---@param h number
---@param a1 number
---@param a2 number
function Graphics:pie(x, y, w, h, a1, a2) end

--- Fills a convex shape through three or more x, y pairs.
---@param ... number
function Graphics:polygon(...) end

--- Outlines a shape through three or more x, y pairs.
---@param ... number
function Graphics:loop(...) end

--- Text with its top left at x, y; or, given w and h, in that box.
---@param s string
---@param x number
---@param y number
---@param w? number
---@param h? number
---@param align? gui.Align
function Graphics:text(s, x, y, w, h, align) end

---@param s string
---@return integer width, integer height
function Graphics:measure(s) end

---@param file string
---@param x number
---@param y number
---@param w? number
---@param h? number
function Graphics:image(file, x, y, w, h) end

----------------------------------------------------------------------------
-- The module.

---@class gui
local gui = {}

---@param opts? gui.FormOptions
---@return gui.Form
function gui.Form(opts) end

--- The controls made with gui.X go on the Form made most recently, or in
--- `parent =`.
---@param opts? gui.LabelOptions
---@return gui.Label
function gui.Label(opts) end

---@param opts? gui.ButtonOptions
---@return gui.Button
function gui.Button(opts) end

---@param opts? gui.TextBoxOptions
---@return gui.TextBox
function gui.TextBox(opts) end

---@param opts? gui.CheckBoxOptions
---@return gui.CheckBox
function gui.CheckBox(opts) end

---@param opts? gui.CheckBoxOptions
---@return gui.RadioButton
function gui.RadioButton(opts) end

---@param opts? gui.ComboBoxOptions
---@return gui.ComboBox
function gui.ComboBox(opts) end

---@param opts? gui.ListBoxOptions
---@return gui.ListBox
function gui.ListBox(opts) end

---@param opts? gui.TreeOptions
---@return gui.Tree
function gui.Tree(opts) end

---@param opts? gui.TableOptions
---@return gui.Table
function gui.Table(opts) end

---@param opts? gui.SliderOptions
---@return gui.Slider
function gui.Slider(opts) end

---@param opts? gui.SpinnerOptions
---@return gui.Spinner
function gui.Spinner(opts) end

---@param opts? gui.ProgressBarOptions
---@return gui.ProgressBar
function gui.ProgressBar(opts) end

---@param opts? gui.ImageOptions
---@return gui.Image
function gui.Image(opts) end

---@param opts? gui.CanvasOptions
---@return gui.Canvas
function gui.Canvas(opts) end

---@param opts? gui.Options
---@return gui.Frame
function gui.Frame(opts) end

---@param opts? gui.TabsOptions
---@return gui.Tabs
function gui.Tabs(opts) end

---@param items gui.MenuItem[]
---@return gui.Menu
function gui.Menu(items) end

--- Shows a message, and returns the button pressed. Closing the box any other
--- way answers with the last button.
---@param message string
---@param buttons? gui.Buttons "ok" by default
---@param title? string
---@return gui.Answer
function gui.msgbox(message, buttons, title) end

--- Asks for a line of text: what was typed, or nil when cancelled.
---@param prompt string
---@param title? string
---@param default? string
---@return string?
function gui.inputbox(prompt, title, default) end

---@class gui.FileOptions
---@field title? string
---@field filter? string as "Lua\t*.lua\nAll\t*"
---@field dir? string
---@field file? string

---@class gui.OpenOptions: gui.FileOptions
---@field multiple? boolean returns a list of paths

---@param opts? string|gui.OpenOptions a title, or options
---@return string|string[]|nil
function gui.openfile(opts) end

---@param opts? string|gui.FileOptions
---@return string?
function gui.savefile(opts) end

---@param opts? string|gui.FileOptions
---@return string?
function gui.choosedir(opts) end

--- The text on the clipboard; `gui.clipboard(text)` puts text there instead.
---@overload fun(text: string)
---@return string
function gui.clipboard() end

--- The tlua that is running, for running another program with it.
---@type string
gui.interpreter = ""

---@class gui.Process
local Process = {}

function Process.kill() end

---@return boolean
function Process.running() end

---@class gui.SpawnOptions
---@field [integer] string the command and its arguments
---@field dir? string
---@field onOutput? fun(line: string, stream: "stdout"|"stderr")
---@field onExit? fun(code: integer)

--- Runs a program beside this one, passing on what it prints a line at a
--- time, on the GUI's thread, while a form is up.
---@param opts gui.SpawnOptions
---@return gui.Process
function gui.spawn(opts) end

---@class gui.Timer
local Timer = {}

--- Cancels it.
function Timer:stop() end

--- Runs fn once, seconds from now, while a form is up.
---@param seconds number
---@param fn fun(timer: gui.Timer)
---@return gui.Timer
function gui.after(seconds, fn) end

--- Runs fn every so many seconds, until it returns false or is stopped.
---@param seconds number
---@param fn fun(timer: gui.Timer): boolean?
---@return gui.Timer
function gui.every(seconds, fn) end

---@class gui.Definition
---@field name string a capital first, like the built-in kinds
---@field events? string[] events of its own, as "onChange"
---@field props? table<string, gui.PropInfo> properties of its own: offered by a designer, kept in a layout, reported by gui.kinds
---@field build fun(parent: gui.Container, opts: table<string, any>): gui.Object makes the control in parent and returns it

---@alias gui.PropType "string"|"number"|"integer"|"boolean"|"color"|"choice"|"file"|"name"|"list"|"rows"|"tree"|"menu"

---@class gui.PropInfo
---@field type gui.PropType
---@field default? any
---@field choices? string[] for a choice
---@field fixed? true only given when the control is made

---@class gui.KindInfo
---@field props table<string, gui.PropInfo>
---@field events string[]
---@field holds string[] the kinds it can be made inside it
---@field width? integer its size when made without one
---@field height? integer
---@field defined? true made with gui.define

--- What a part of a layout is: a kind, its properties, and its controls in
--- the array part, each written the same way.
---@class gui.Layout: gui.Options
---@field kind string
---@field [integer] gui.Layout

--- Builds what a layout describes and returns its top: a Form, unless parent
--- is given to build it in. A string is the path of a file returning a
--- layout; without .lua at the end it means its .form.lua file, looked for
--- next to the script that names it. Layout files run with nothing in scope.
---@param layout string|gui.Layout
---@param parent? gui.Container
---@return gui.Form|gui.Object
function gui.load(layout, parent) end

--- The layout of an object as it stands, from what is on screen: only what
--- differs from the defaults, and no handlers.
---@param obj gui.Object
---@return gui.Layout
function gui.dump(obj) end

--- Writes an object's layout, or a layout table as it is, to a Lua file, in
--- a steady order.
---@param obj gui.Object|gui.Layout
---@param path string
function gui.save(obj, path) end

--- Every kind, built in or defined, with its properties, events and what it
--- holds. A fresh table each time.
---@return table<string, gui.KindInfo>
function gui.kinds() end

--- Names a control written in Lua. After it, `container:Name{...}` and
--- `gui.Name{...}` make one; the table it is made from goes to build, and
--- the common properties and handlers in it are applied to what build returns.
---@param def gui.Definition
function gui.define(def) end

return gui
