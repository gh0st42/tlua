-- tui: a small Turbo Vision-style widget set built on term

local term = require("tlua.term")

local tui = {}

-----------------------------------------------------------------------------
-- Minimal single-inheritance OOP helper
-----------------------------------------------------------------------------
local function class(base)
    local cls = {}
    cls.__index = cls
    if base then
        setmetatable(cls, { __index = base })
    end
    cls.super = base
    function cls.new(...)
        local self = setmetatable({}, cls)
        if self.init then self:init(...) end
        return self
    end
    return cls
end
tui.class = class

local function fit(s, w)
    s = tostring(s or "")
    if w <= 0 then return "" end
    if #s > w then return s:sub(1, w) end
    return s .. string.rep(" ", w - #s)
end

-----------------------------------------------------------------------------
-- Theme (mutate freely to restyle every widget at once)
-----------------------------------------------------------------------------
tui.theme = {
    desktop = { bg = "blue" },
    window = { border = "white", background = "cyan", titlefg = "yellow" },
    normal = { fg = "black", bg = "cyan" },
    focus = { fg = "yellow", bg = "black" },
    selected = { fg = "black", bg = "lightgray" },
    button = { fg = "black", bg = "lightgray" },
    statusbar = { fg = "black", bg = "lightgray" },
}

-----------------------------------------------------------------------------
-- View: base class for every widget and container
-----------------------------------------------------------------------------
local View = class()
tui.View = View

function View:init(x, y, w, h)
    self.x = x or 1
    self.y = y or 1
    self.w = w or 1
    self.h = h or 1
    self.visible = true
    self.focusable = false
end

function View:abspos()
    if not self.parent then
        return self.x, self.y
    end
    local ox, oy = self.parent:origin()
    return ox + self.x - 1, oy + self.y - 1
end

-- where this view's own children's (1,1) lands on screen; override to
-- inset content (e.g. Window insets by its border)
function View:origin()
    return self:abspos()
end

function View:addchild(view)
    view.parent = self
    self.children = self.children or {}
    table.insert(self.children, view)
    return view
end

-- a view can take focus if it is focusable itself, or if it is a container
-- (a Window, say) with something focusable inside it to pass keys on to
local function canfocus(view)
    if not view.visible then return false end
    if view.focusable then return true end
    for _, c in ipairs(view.children or {}) do
        if canfocus(c) then return true end
    end
    return false
end

function View:focusables()
    local t = {}
    for _, c in ipairs(self.children or {}) do
        if canfocus(c) then t[#t + 1] = c end
    end
    return t
end

function View:focused()
    local f = self:focusables()
    if #f == 0 then return nil end
    local i = self.focusindex or 1
    if i < 1 or i > #f then i = 1 end
    self.focusindex = i
    return f[i]
end

function View:isfocused()
    return self.parent ~= nil and self.parent:focused() == self
end

function View:focusnext()
    local f = self:focusables()
    if #f == 0 then return end
    self.focusindex = ((self.focusindex or 0) % #f) + 1
end

function View:focusprev()
    local f = self:focusables()
    if #f == 0 then return end
    self.focusindex = (((self.focusindex or 1) - 2) % #f) + 1
end

function View:drawchildren()
    for _, c in ipairs(self.children or {}) do
        if c.visible then c:draw() end
    end
end

function View:draw()
    self:drawchildren()
end

-- default routing: let the focused child try first, then handle
-- tab/shift-tab ourselves; override in leaf widgets for their own keys
function View:handlekey(key)
    local f = self:focused()
    if f and f:handlekey(key) then return true end
    if key == "tab" then
        self:focusnext()
        return true
    elseif key == "shift-tab" then
        self:focusprev()
        return true
    end
    return false
end

-----------------------------------------------------------------------------
-- Label: static text
-----------------------------------------------------------------------------
local Label = class(View)
tui.Label = Label

function Label:init(x, y, text)
    text = tostring(text or "")
    View.init(self, x, y, #text, 1)
    self.text = text
end

function Label:draw()
    local ax, ay = self:abspos()
    local c = tui.theme.normal
    term.gotoxy(ax, ay)
    term.colorize(c.fg, c.bg)
    io.write(self.text)
    term.normvideo()
end

-----------------------------------------------------------------------------
-- Button
-----------------------------------------------------------------------------
local Button = class(View)
tui.Button = Button

function Button:init(x, y, text, onclick)
    View.init(self, x, y, #tostring(text) + 4, 1)
    self.text = text
    self.onclick = onclick
    self.focusable = true
end

function Button:draw()
    local ax, ay = self:abspos()
    local c = self:isfocused() and tui.theme.focus or tui.theme.button
    term.gotoxy(ax, ay)
    term.colorize(c.fg, c.bg)
    io.write("[ " .. self.text .. " ]")
    term.normvideo()
end

function Button:handlekey(key)
    if key == "enter" or key == "space" then
        if self.onclick then self.onclick(self) end
        return true
    end
    return false
end

-----------------------------------------------------------------------------
-- CheckBox
-----------------------------------------------------------------------------
local CheckBox = class(View)
tui.CheckBox = CheckBox

function CheckBox:init(x, y, text, checked)
    text = tostring(text or "")
    View.init(self, x, y, #text + 4, 1)
    self.text = text
    self.checked = checked and true or false
    self.focusable = true
end

function CheckBox:draw()
    local ax, ay = self:abspos()
    local c = self:isfocused() and tui.theme.focus or tui.theme.normal
    term.gotoxy(ax, ay)
    term.colorize(c.fg, c.bg)
    io.write("[" .. (self.checked and "x" or " ") .. "] " .. self.text)
    term.normvideo()
end

function CheckBox:handlekey(key)
    if key == "space" or key == "enter" then
        self.checked = not self.checked
        if self.onchange then self.onchange(self, self.checked) end
        return true
    end
    return false
end

-----------------------------------------------------------------------------
-- RadioButtons
-----------------------------------------------------------------------------
local RadioButtons = class(View)
tui.RadioButtons = RadioButtons

function RadioButtons:init(x, y, items, selected)
    local w = 0
    for _, item in ipairs(items) do w = math.max(w, #item + 4) end
    View.init(self, x, y, w, #items)
    self.items = items
    self.selected = selected or 1
    self.focusable = true
end

function RadioButtons:draw()
    local ax, ay = self:abspos()
    local focused = self:isfocused()
    for i, item in ipairs(self.items) do
        local c = (focused and i == self.selected) and tui.theme.focus or tui.theme.normal
        term.gotoxy(ax, ay + i - 1)
        term.colorize(c.fg, c.bg)
        io.write("(" .. (i == self.selected and "*" or " ") .. ") " .. item)
    end
    term.normvideo()
end

function RadioButtons:handlekey(key)
    local n = #self.items
    if n == 0 then return false end
    if key == "up" then
        self.selected = ((self.selected - 2) % n) + 1
        return true
    elseif key == "down" then
        self.selected = (self.selected % n) + 1
        return true
    elseif key == "space" or key == "enter" then
        if self.onchange then self.onchange(self, self.selected) end
        return true
    end
    return false
end

-----------------------------------------------------------------------------
-- InputLine: single-line text editor
-----------------------------------------------------------------------------
local InputLine = class(View)
tui.InputLine = InputLine

function InputLine:init(x, y, w, text, maxlen)
    View.init(self, x, y, w, 1)
    self.text = text or ""
    self.maxlen = maxlen or 255
    self.cursor = #self.text + 1
    self.offset = 0
    self.focusable = true
end

function InputLine:draw()
    if self.cursor - self.offset > self.w then self.offset = self.cursor - self.w end
    if self.cursor <= self.offset then self.offset = self.cursor - 1 end
    if self.offset < 0 then self.offset = 0 end

    local ax, ay = self:abspos()
    local c = self:isfocused() and tui.theme.focus or tui.theme.normal
    term.gotoxy(ax, ay)
    term.colorize(c.fg, c.bg)
    io.write(fit(self.text:sub(self.offset + 1), self.w))
    term.normvideo()
end

function InputLine:cursorpos()
    local ax, ay = self:abspos()
    return ax + (self.cursor - self.offset) - 1, ay
end

function InputLine:handlekey(key)
    if key == "left" then
        self.cursor = math.max(1, self.cursor - 1)
        return true
    elseif key == "right" then
        self.cursor = math.min(#self.text + 1, self.cursor + 1)
        return true
    elseif key == "home" then
        self.cursor = 1
        return true
    elseif key == "end" then
        self.cursor = #self.text + 1
        return true
    elseif key == "backspace" then
        if self.cursor > 1 then
            self.text = self.text:sub(1, self.cursor - 2) .. self.text:sub(self.cursor)
            self.cursor = self.cursor - 1
        end
        return true
    elseif key == "delete" then
        self.text = self.text:sub(1, self.cursor - 1) .. self.text:sub(self.cursor + 1)
        return true
    elseif type(key) == "string" and #key == 1 and key:byte() >= 32 and key:byte() < 127 then
        if #self.text < self.maxlen then
            self.text = self.text:sub(1, self.cursor - 1) .. key .. self.text:sub(self.cursor)
            self.cursor = self.cursor + 1
        end
        return true
    end
    return false
end

-----------------------------------------------------------------------------
-- ListBox: scrollable single-select list
-----------------------------------------------------------------------------
local ListBox = class(View)
tui.ListBox = ListBox

function ListBox:init(x, y, w, h, items)
    View.init(self, x, y, w, h)
    self.items = items or {}
    self.selected = 1
    self.offset = 0
    self.focusable = true
end

function ListBox:draw()
    if self.selected - self.offset > self.h then self.offset = self.selected - self.h end
    if self.selected <= self.offset then self.offset = self.selected - 1 end
    if self.offset < 0 then self.offset = 0 end

    local ax, ay = self:abspos()
    local focused = self:isfocused()
    for row = 0, self.h - 1 do
        local idx = self.offset + row + 1
        local item = self.items[idx]
        local c = tui.theme.normal
        if idx == self.selected then
            c = focused and tui.theme.focus or tui.theme.selected
        end
        term.gotoxy(ax, ay + row)
        term.colorize(c.fg, c.bg)
        io.write(fit(item, self.w))
    end
    term.normvideo()
end

function ListBox:handlekey(key)
    local n = #self.items
    if key == "up" then
        self.selected = math.max(1, self.selected - 1)
        return true
    elseif key == "down" then
        self.selected = math.min(n, self.selected + 1)
        return true
    elseif key == "home" then
        self.selected = 1
        return true
    elseif key == "end" then
        self.selected = n
        return true
    elseif key == "pageup" then
        self.selected = math.max(1, self.selected - self.h)
        return true
    elseif key == "pagedown" then
        self.selected = math.min(n, self.selected + self.h)
        return true
    elseif key == "enter" then
        if self.onselect then self.onselect(self, self.selected, self.items[self.selected]) end
        return true
    end
    return false
end

-----------------------------------------------------------------------------
-- StatusBar: a single non-interactive hint line
-----------------------------------------------------------------------------
local StatusBar = class(View)
tui.StatusBar = StatusBar

function StatusBar:init(y, w, text)
    View.init(self, 1, y, w, 1)
    self.text = text or ""
end

function StatusBar:draw()
    local ax, ay = self:abspos()
    local c = tui.theme.statusbar
    term.gotoxy(ax, ay)
    term.colorize(c.fg, c.bg)
    io.write(fit(self.text, self.w))
    term.normvideo()
end

-----------------------------------------------------------------------------
-- Window: bordered, titled container
-----------------------------------------------------------------------------
local Window = class(View)
tui.Window = Window

function Window:init(x, y, w, h, title)
    View.init(self, x, y, w, h)
    self.title = title
end

function Window:origin()
    local ax, ay = self:abspos()
    return ax + 1, ay + 1
end

function Window:draw()
    local ax, ay = self:abspos()
    local c = tui.theme.window
    term.colorize(c.border, c.background)
    for row = 0, self.h - 1 do
        term.gotoxy(ax, ay + row)
        if row == 0 or row == self.h - 1 then
            io.write("+", string.rep("-", math.max(0, self.w - 2)), "+")
        else
            io.write("|", string.rep(" ", math.max(0, self.w - 2)), "|")
        end
    end
    if self.title and self.w > #self.title + 4 then
        local label = " " .. self.title .. " "
        local tx = ax + math.max(1, math.floor((self.w - #label) / 2))
        term.gotoxy(tx, ay)
        term.colorize(c.titlefg, c.background)
        io.write(label)
    end
    term.normvideo()
    self:drawchildren()
end

-----------------------------------------------------------------------------
-- Dialog: a Window that can run its own modal event loop
-----------------------------------------------------------------------------
local Dialog = class(Window)
tui.Dialog = Dialog

function Dialog:init(x, y, w, h, title)
    Window.init(self, x, y, w, h, title)
    self.done = false
    self.result = nil
end

function Dialog:close(result)
    self.done = true
    self.result = result
end

function Dialog:handlekey(key)
    if key == "esc" then
        self:close(nil)
        return true
    end
    return Window.handlekey(self, key)
end

function Dialog:runmodal(app)
    self.done = false
    while not self.done do
        app:redraw(self)
        local key = app:readkey()
        if key then self:handlekey(key) end
    end
    return self.result
end

-----------------------------------------------------------------------------
-- Key decoding: turns raw bytes into named keys ("up", "enter", "ctrl-c",
-- a single printable character, ...)
-----------------------------------------------------------------------------
local escseq = {
    A = "up", B = "down", C = "right", D = "left",
    H = "home", F = "end", Z = "shift-tab",
}
local tildeseq = {
    ["1~"] = "home", ["2~"] = "insert", ["3~"] = "delete",
    ["4~"] = "end", ["5~"] = "pageup", ["6~"] = "pagedown",
}
local ctrlkeys = { [3] = "ctrl-c", [17] = "ctrl-q", [24] = "ctrl-x" }

local function readkey()
    local c = term.readkey()
    if not c then return nil end
    local b = c:byte()

    if b == 27 then
        local c2 = term.readkey(0.05)
        if not c2 then return "esc" end
        if c2 == "[" or c2 == "O" then
            local c3 = term.readkey(0.05)
            if not c3 then return "esc" end
            if escseq[c3] then return escseq[c3] end
            if c3:match("%d") then
                local seq = c3
                for _ = 1, 3 do
                    local c4 = term.readkey(0.05)
                    if not c4 then break end
                    seq = seq .. c4
                    if c4 == "~" then break end
                end
                return tildeseq[seq] or "esc"
            end
        end
        return "esc"
    elseif b == 9 then
        return "tab"
    elseif b == 13 or b == 10 then
        return "enter"
    elseif b == 32 then
        return "space"
    elseif b == 127 or b == 8 then
        return "backspace"
    elseif b < 32 then
        return ctrlkeys[b] or c
    else
        return c
    end
end
tui.readkey = readkey

-----------------------------------------------------------------------------
-- Application: the root container and event loop
-----------------------------------------------------------------------------
local Application = class(View)
tui.Application = Application

function Application:init()
    local w, h = term.size()
    if not w then w, h = 80, 24 end -- on failure the second value is the error
    View.init(self, 1, 1, w, h)
    self.running = false
end

function Application:draw()
    term.clrscr(tui.theme.desktop.bg)
    self:drawchildren()
end

function Application:readkey()
    return readkey()
end

-- redraws the whole desktop, then an optional overlay (e.g. a modal
-- dialog) on top, and positions the terminal cursor for whichever
-- widget is focused
function Application:redraw(overlay)
    self:draw()
    if overlay then overlay:draw() end
    self:updatecursor(overlay or self)
end

function Application:updatecursor(root)
    local v = root
    while v.focused do
        local f = v:focused()
        if not f then break end
        v = f
    end
    if v and v.cursorpos then
        local cx, cy = v:cursorpos()
        term.gotoxy(cx, cy)
        term.cursor(true)
    else
        term.cursor(false)
    end
    term.flush()
end

function Application:quit()
    self.running = false
end

function Application:handlekey(key)
    if key == "ctrl-q" then
        self:quit()
        return true
    end
    return View.handlekey(self, key)
end

function Application:run()
    local w, h = term.size()
    if w and h then
        self.w, self.h = w, h
    end

    term.cursor(false)
    term.raw(true)
    self.running = true

    local ok, err = pcall(function()
        while self.running do
            self:redraw()
            local key = self:readkey()
            if key then self:handlekey(key) end
        end
    end)

    term.raw(false)
    term.normvideo()
    term.clrscr()
    term.cursor(true)
    term.flush()

    if not ok then error(err, 0) end
end

-----------------------------------------------------------------------------
-- messagebox: a ready-made alert/confirm dialog
-----------------------------------------------------------------------------
function tui.messagebox(app, text, buttons)
    buttons = buttons or { "OK" }

    local lines = {}
    for line in (tostring(text) .. "\n"):gmatch("(.-)\n") do
        lines[#lines + 1] = line
    end

    local textw = 0
    for _, l in ipairs(lines) do textw = math.max(textw, #l) end

    local w = math.max(textw + 6, 20)
    local h = #lines + 5
    local dlg = Dialog.new(
        math.floor((app.w - w) / 2) + 1,
        math.floor((app.h - h) / 2) + 1,
        w, h
    )

    for i, l in ipairs(lines) do
        dlg:addchild(Label.new(3, 1 + i, l))
    end

    local bx, by = 3, h - 2
    for _, label in ipairs(buttons) do
        local btn = Button.new(bx, by, label, function() dlg:close(label) end)
        dlg:addchild(btn)
        bx = bx + btn.w + 2
    end

    return dlg:runmodal(app)
end

return tui
