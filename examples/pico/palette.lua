-- palette: all 64 colours, with the number to ask for each one by.
--
-- 0-15 are PICO-8's palette and 16-31 its extended one, both exactly as they
-- are there. 32-63 are tlua's own: a grey scale and four ramps, for shading.

local w, h = screen()
local across, down = 8, 8
local cw, ch = flr(w / across), flr((h - 12) / down)

-- shadowed draws text twice so that it can be read on any colour underneath.
local function shadowed(text, x, y)
	print(text, x + 1, y + 1, 0)
	print(text, x, y, 7)
end

function _draw()
	cls(0)
	for i = 0, 63 do
		local x, y = (i % across) * cw, flr(i / across) * ch
		rectfill(x, y, x + cw - 2, y + ch - 2, i)
		shadowed(tostr(i), x + 3, y + 3)
	end
	shadowed("0-15 pico-8   16-31 its extended set   32-63 tlua's ramps", 4, h - 9)
end
