-- tui demo, for a terminal of at least 80x24: tlua examples/console/tui_demo.lua

local tui = require("tui")

local app = tui.Application.new()

local win = tui.Window.new(3, 2, app.w - 4, app.h - 4, "tui demo")
app:addchild(win)

win:addchild(tui.Label.new(3, 2, "Name:"))
local name_input = tui.InputLine.new(9, 2, 20, "")
win:addchild(name_input)

local subscribe = tui.CheckBox.new(3, 4, "Subscribe to newsletter", false)
win:addchild(subscribe)

win:addchild(tui.Label.new(3, 6, "Favorite color:"))
local colors = tui.RadioButtons.new(3, 7, { "Red", "Green", "Blue" }, 1)
win:addchild(colors)

win:addchild(tui.Label.new(3, 11, "Pick a fruit:"))
local fruits = tui.ListBox.new(3, 12, 20, 4, { "Apple", "Banana", "Cherry", "Date", "Elderberry" })
win:addchild(fruits)

local function on_submit()
    local color = colors.items[colors.selected]
    local fruit = fruits.items[fruits.selected] or "(none)"
    local name = name_input.text ~= "" and name_input.text or "(empty)"
    local msg = string.format(
        "Name: %s\nSubscribed: %s\nColor: %s\nFruit: %s",
        name, tostring(subscribe.checked), color, fruit
    )
    tui.messagebox(app, msg, { "OK" })
end

win:addchild(tui.Button.new(3, 17, "Submit", on_submit))
win:addchild(tui.Button.new(14, 17, "Quit", function()
    if tui.messagebox(app, "Quit the demo?", { "Yes", "No" }) == "Yes" then
        app:quit()
    end
end))

app:addchild(tui.StatusBar.new(app.h, app.w,
    "Tab: next field   Space/Enter: activate   Esc: cancel   Ctrl+Q: quit"))

app:run()
