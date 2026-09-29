-- plasma: every pixel worked out in Lua, sixty times a second.
--
-- At 480x270 that would be a hundred and thirty thousand pset() calls a frame,
-- which is more than Lua will do in a sixtieth of a second. So the program asks
-- for a smaller screen: vid(2) is 160x90, a quarter of the console's own size,
-- and the window scales whatever it is given up to fit. A fantasy console gets
-- to choose its own limits, and this is how.

vid(2)
local w, h = screen()

-- Which colours to run through, darkest to brightest. The ramps at 40 and up
-- are laid out for exactly this.
local ramp = { 40, 41, 42, 43, 44, 45, 51, 50, 49, 48, 47, 46 }

function _draw()
	local now = t()
	for y = 0, h - 1 do
		for x = 0, w - 1 do
			-- Three waves at angles to each other; where they meet decides the
			-- colour.
			local v = sin(x / 50 + now / 4)
				+ sin(y / 40 - now / 5)
				+ sin((x + y) / 70 + now / 3)
			local i = flr((v + 3) / 6 * #ramp) + 1
			pset(x, y, ramp[mid(i, 1, #ramp)])
		end
	end

	print("plasma at " .. w .. "x" .. h .. ", " .. flr(fps()) .. " fps", 4, 4, 7)
end
