-- modes: the same picture at every resolution the console has.
--
-- X steps through the video modes. Nothing in the drawing changes — it is
-- written against screen(), so it lays itself out to whatever it is given —
-- and the window scales the result up by a whole number either way.

local modes = { 0, 1, 2, 13 }
local chosen = 1
local spin = 0

function _update()
	if btnp("x") then
		chosen = chosen % #modes + 1
		vid(modes[chosen])
	end
	spin = spin + 0.004
end

function _draw()
	local w, h = screen()
	local mode = vid()
	cls(1)

	-- A frame around the edge, so that the size of the screen is plain.
	rrect(1, 1, w - 2, h - 2, 6, 13)

	-- Panels that lay themselves out to whatever the screen turned out to be.
	local pad = flr(w / 40)
	local panel = flr((w - pad * 5) / 4)
	local tall = flr(h / 3)
	for i = 0, 3 do
		local x = pad + i * (panel + pad)
		rrectfill(x, tall, panel, tall, flr(panel / 4), 2 + i)
		rrect(x, tall, panel, tall, flr(panel / 4), 7)
		-- Clear of the rounded corner, which eats anything in it.
		print(tostr(i + 1), x + flr(panel / 4) + 2, tall + 6, 7)
	end

	-- Something round, to show what the pixels are doing.
	local cx, cy, rad = w / 2, tall / 2, flr(tall / 3)
	circfill(cx, cy, rad, 12)
	circ(cx, cy, rad, 7)
	line(cx, cy, cx + cos(spin) * rad, cy + sin(spin) * rad, 0)

	print("vid(" .. mode .. ")   " .. w .. "x" .. h, 8, 8, 7)
	print("X: next mode", 8, h - 12, 6)
end
