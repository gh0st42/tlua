#!/usr/bin/env tlua
-- Events beyond clicks. Everything a control or the form hears about is
-- written to the list on the right, as it happens:
--
--   form      onLoad, onShow, onActivate / onDeactivate (click another
--             window and back), onResize (drag a corner)
--   controls  onFocus / onBlur (Tab through the boxes), onShow / onHide
--             (the pages), onScroll, onResize
--   Label     a link: pointer = "hand", onClick, onMouseEnter / onMouseLeave
--   Panel     onPaint draws tick marks over the bar it holds

local gui = bootgui()

local frm = gui.Form { caption = "Events", width = 620, height = 420, resizable = true }

local events = frm:ListBox { left = 340, top = 10, width = 270, height = 400, grow = true }
local nameBox

-- say adds a line to the log, and shows the latest.
local lines = {}
local function say(text)
  lines[#lines + 1] = text
  if #lines > 300 then table.remove(lines, 1) end
  events.items = lines
  events.selected = #lines
end

----------------------------------------------------------------------------
-- The form.

function frm:onLoad()
  say("form: onLoad (" .. self.width .. "x" .. self.height .. ")")
  -- The place to fill in what the form starts with.
  nameBox.text = "Ada"
end
function frm:onShow() say("form: onShow") end
function frm:onActivate() say("form: onActivate"); self.caption = "Events" end
function frm:onDeactivate() say("form: onDeactivate"); self.caption = "Events (not in front)" end
function frm:onResize() say("form: onResize " .. self.width .. "x" .. self.height) end

----------------------------------------------------------------------------
-- Focus: a box that is checked when it is left.

frm:Label { caption = "Name:", left = 10, top = 14, width = 50, height = 24 }
nameBox = frm:TextBox { left = 64, top = 10, width = 150, height = 28 }
local hint = frm:Label { left = 222, top = 14, width = 110, height = 24, textColor = "#a00000" }
local other = frm:TextBox { left = 64, top = 44, width = 150, height = 28, text = "Tab here" }
frm:Label { caption = "Other:", left = 10, top = 48, width = 50, height = 24 }

function nameBox:onFocus() say("name: onFocus"); hint.caption = "" end
function nameBox:onBlur()
  say("name: onBlur")
  hint.caption = self.text == "" and "needs a name" or ""
end
function other:onFocus() say("other: onFocus") end
function other:onBlur() say("other: onBlur") end

----------------------------------------------------------------------------
-- A Label as a link: the hand, a colour while the mouse is over it, a click.

local clicks = 0
local link = frm:Label {
  caption = "Click me, I am a link", left = 10, top = 84, width = 200, height = 24,
  textColor = "#0050c0", pointer = "hand",
}
function link:onMouseEnter() self.textColor = "#c04000"; say("link: onMouseEnter") end
function link:onMouseLeave() self.textColor = "#0050c0"; say("link: onMouseLeave") end
function link:onClick(x, y)
  clicks = clicks + 1
  say(("link: onClick at %d,%d (%d)"):format(x, y, clicks))
end
function link:onDoubleClick() say("link: onDoubleClick") end

----------------------------------------------------------------------------
-- Painting over a control: the bar is a ProgressBar, and the Panel that holds
-- it draws its tick marks, and a marker, on top.

local meter = frm:Panel { left = 10, top = 120, width = 320, height = 56 }
local bar = meter:ProgressBar { left = 0, top = 0, width = 320, height = 30, value = 40 }
local slider = meter:Slider { left = 0, top = 34, width = 320, height = 22, min = 0, max = 100, value = 40 }

local paints = 0
function meter:onPaint(g)
  paints = paints + 1
  local w = g:size()
  g:color("#ffffff")
  g:width(1)
  for tick = 0, 10 do
    local x = math.floor((w - 1) * tick / 10)
    g:line(x, 0, x, tick % 5 == 0 and 14 or 8)
  end
  g:color("#c00000")
  g:width(2)
  local at = math.floor((w - 1) * slider.value / 100)
  g:line(at, 0, at, 30)
  g:color("#000000")
  g:font("sans", 11)
  g:text(("painted %d times"):format(paints), 4, 16)
end
function slider:onChange() bar.value = self.value; meter:redraw() end
function meter:onResize() say("meter: onResize " .. self.width .. "x" .. self.height) end

----------------------------------------------------------------------------
-- A Scroll tells where it was scrolled to.

local scroll = frm:Scroll { left = 10, top = 190, width = 320, height = 130 }
for i = 1, 20 do
  scroll:Label { caption = "Row " .. i, left = 6, top = (i - 1) * 24 + 4, width = 280, height = 22 }
end
function scroll:onScroll(x, y) say(("scroll: onScroll %d,%d"):format(x, y)) end

local jump = frm:Button { caption = "Scroll to row 12", left = 10, top = 328, width = 150, height = 28 }
function jump:onClick() scroll.scrollY = 11 * 24 end

----------------------------------------------------------------------------
-- Pages hear when they are shown and hidden.

local tabs = frm:Tabs { left = 10, top = 366, width = 320, height = 44 }
local one = tabs:Page { caption = "One" }
local two = tabs:Page { caption = "Two" }
function one:onShow() say("page One: onShow") end
function one:onHide() say("page One: onHide") end
function two:onShow() say("page Two: onShow") end
function two:onHide() say("page Two: onHide") end

frm:show()
