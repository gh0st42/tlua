-- shapes: every drawing call there is, and what the fill patterns do.
--
-- Press X to step through the dither patterns; the shapes below the line are
-- drawn with whichever is showing.

boot()

local w, h = screen()

-- A 4x4 dither is sixteen bits, the top left pixel being the highest one.
local patterns = {
	{ 0x0000, "none" },
	{ 0xa5a5, "checks" },
	{ 0x5a5a, "checks, the other way" },
	{ 0x0f0f, "stripes" },
	{ 0x8421, "diagonal" },
	{ 0xfefe, "mostly solid" },
}
local chosen = 2 -- start on a pattern, so there is something to see

function _update()
	if btnp("x") then chosen = chosen % #patterns + 1 end
end

function _draw()
	cls(1)

	-- Outlines on one row, the same shapes filled on the next.
	fillp()
	print("outlines  (the rounded one is rrect: x, y, width, height)", 8, 8, 6)
	line(16, 24, 68, 52, 7)
	rect(84, 24, 140, 52, 8)
	circ(172, 38, 14, 9)
	oval(200, 24, 260, 52, 10)
	tri(280, 52, 308, 24, 336, 52, 11)
	rrect(352, 24, 56, 28, 8, 14)      -- a width and a height, not a corner
	for i = 0, 28 do
		pset(424 + i, 24 + i, 7)       -- pset, one pixel at a time
		pset(425 + i, 52 - i, 12)
	end

	print("filled", 8, 66, 6)
	rectfill(84, 80, 140, 108, 8)
	circfill(172, 94, 14, 9)
	ovalfill(200, 80, 260, 108, 10)
	trifill(280, 108, 308, 80, 336, 108, 11)
	rrectfill(352, 80, 56, 28, 8, 14)
	for i = 0, 6 do
		circfill(24 + i * 8, 94, 3, 7 + i) -- a row of dots, for the palette
	end

	line(0, 124, w, 124, 13)

	-- Everything below the line is drawn through a dither pattern.
	local pattern, name = patterns[chosen][1], patterns[chosen][2]
	print("the same, dithered between two colours", 8, 132, 6)
	color(12, 2)
	fillp(pattern)
	rectfill(84, 148, 140, 200)
	circfill(172, 174, 26)
	ovalfill(200, 148, 260, 200)
	trifill(280, 200, 308, 148, 336, 200)
	rrectfill(344, 148, 48, 52, 12)

	-- With a second argument the pattern leaves holes instead of drawing the
	-- second colour, so whatever is behind shows through.
	rectfill(402, 148, 470, 200, 3)
	fillp(pattern, true)
	circfill(436, 174, 24, 10)

	fillp()
	print("fillp " .. tostr(pattern, true) .. ": " .. name, 8, h - 32, 7)
	print("press " .. btnkey("x") .. " for the next pattern", 8, h - 22, 12)
	print("on the right the pattern punches holes instead", 8, h - 12, 13)
end
