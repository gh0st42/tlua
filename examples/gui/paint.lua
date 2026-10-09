#!/usr/bin/env tlua
-- A pixel editor: a picture from the png module, drawn eight times its size
-- on a Canvas, painted with the mouse, opened and saved as PNG, and packed
-- into a zip with a note beside it.

local gui = bootgui()
local png = require "png"
local zip = require "zip"

local W, H, ZOOM = 32, 24, 8
local pic = png.new(W, H, "#ffffff")
local ink = "#1f4e9c"

local frm = gui.Form { caption = "Paint", width = W * ZOOM + 140, height = H * ZOOM + 20 }

local board = frm:Canvas { left = 10, top = 10, width = W * ZOOM, height = H * ZOOM }
function board:onDraw(g)
  g:scaling("nearest") -- each pixel a sharp square
  g:image(pic, 0, 0, W * ZOOM, H * ZOOM)
end
local function paint(x, y)
  pic:set(math.floor(x / ZOOM), math.floor(y / ZOOM), ink)
  board:redraw()
end
board.onMouseDown = function(_, x, y) paint(x, y) end
board.onMouseDrag = function(_, x, y) paint(x, y) end

local status = frm:Label { left = W * ZOOM + 20, top = H * ZOOM - 40, width = 110, height = 40 }

local function button(caption, top, onClick)
  frm:Button { caption = caption, left = W * ZOOM + 20, top = top, width = 110, onClick = onClick }
end

button("Colour...", 10, function()
  ink = gui.choosecolor { title = "Ink", color = ink } or ink
end)
button("Clear", 46, function()
  pic:fill("#ffffff")
  board:redraw()
end)
button("Open...", 92, function()
  local path = gui.openfile { title = "Open a picture", filter = "Pictures\t*.{png,jpg,gif}" } --[[@as string?]]
  if not path then return end
  local img, why = png.load(path)
  if not img then
    gui.msgbox(tostring(why), "ok", "Open")
    return
  end
  -- Whatever its size, the picture is the board's: its top left, cut to fit.
  pic:fill("#ffffff")
  for y = 0, math.min(H, img.height) - 1 do
    for x = 0, math.min(W, img.width) - 1 do
      local r, g, b, a = img:get(x, y)
      pic:set(x, y, r or 0, g, b, a)
    end
  end
  board:redraw()
end)
button("Save...", 128, function()
  local path = gui.savefile { title = "Save the picture", file = "picture.png" }
  if path then
    local ok, why = pic:save(path)
    status.caption = ok and "Saved" or tostring(why)
  end
end)
button("Pack...", 164, function()
  local path = gui.savefile { title = "Pack into a zip", file = "picture.zip" }
  if not path then return end
  local out = zip.create(path)
  out:add("picture.png", assert(pic:encode()), { store = true }) -- a PNG is compressed already
  out:add("about.txt", ("A %dx%d picture, painted in tlua.\n"):format(W, H))
  local ok, why = out:close()
  status.caption = ok and "Packed" or tostring(why)
end)

frm:show()
