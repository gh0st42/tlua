-- sprites: drawing, flipping, stretching, animating and recolouring.
--
-- A sprite is written out as text, one character per pixel: a hex digit for
-- colours 0 to 15, a dot or a space for the transparent parts. No file to load
-- and nothing to install, which is why every example here is one file.

boot()

local w, h = screen()

local ship = sprite[[
	...cc...
	..cccc..
	.c9cc9c.
	cccccccc
	c.c88c.c
	...88...
]]

local coin = sprite[[
	..aaaa..
	.a9999a.
	a99aa99a
	a9a77a9a
	a9a77a9a
	a99aa99a
	.a9999a.
	..aaaa..
]]

-- A sheet is one surface with several frames side by side, and sspr() takes
-- whichever rectangle of it the animation is up to. This coin turns: wide, then
-- narrow, then edge on.
local spin = sprite[[
	..aaaa.....aa......99...
	.a9999a...a99a.....a9...
	a99aa99a..a99a.....a9...
	a9a77a9a..a99a.....a9...
	a9a77a9a..a99a.....a9...
	a99aa99a..a99a.....a9...
	.a9999a...a99a.....a9...
	..aaaa.....aa......99...
]]

local frame = 0

function _update()
	frame = frame + 1
end

-- label writes the name of what is underneath it.
local function label(text, x, y)
	print(text, x, y, 6)
end

function _draw()
	cls(1)

	label("spr", 16, 14)
	sspr(ship, 0, 0, 8, 6, 16, 24, 32, 24)

	label("flipped", 72, 14)
	sspr(ship, 0, 0, 8, 6, 72, 24, 32, 24, true, false)
	sspr(ship, 0, 0, 8, 6, 108, 24, 32, 24, false, true)
	sspr(ship, 0, 0, 8, 6, 144, 24, 32, 24, true, true)

	label("sspr: any size", 200, 14)
	local x = 200
	for i = 1, 4 do
		local size = i * 8
		sspr(ship, 0, 0, 8, 6, x, 48 - size * 6 / 8, size, size * 6 / 8)
		x = x + size + 4
	end

	label("a sheet of frames", 330, 14)
	for i = 0, 2 do
		sspr(spin, i * 8, 0, 8, 8, 330 + i * 34, 24, 24, 24)
	end
	-- The same three frames, one after another, three ticks each.
	local which = flr(frame / 6) % 3
	sspr(spin, which * 8, 0, 8, 8, 432, 24, 24, 24)

	label("recoloured with pal", 16, 76)
	for i = 0, 7 do
		-- A draw-time swap changes the colours on the way to the screen; the
		-- sprite's own pixels are untouched.
		pal(12, 8 + i)
		sspr(ship, 0, 0, 8, 6, 16 + i * 40, 88, 32, 24)
	end
	pal()

	label("transparency: colour 0, then palt(0, false)", 16, 128)
	rectfill(16, 140, 180, 190, 5)
	sspr(coin, 0, 0, 8, 8, 28, 148, 32, 32)
	palt(0, false)
	sspr(coin, 0, 0, 8, 8, 88, 148, 32, 32)
	palt()
	print("through", 28, 182, 7)
	print("solid", 92, 182, 7)

	label("drawn straight onto the screen at its own size", 220, 128)
	for row = 0, 2 do
		for col = 0, 11 do
			spr(coin, 224 + col * 12, 144 + row * 12)
		end
	end

	-- Two coins bobbing, a quarter of a turn apart.
	for i = 0, 1 do
		spr(coin, w - 30, h / 2 + 20 + sin(t() + i / 4) * 8)
	end

	print("sprites are written as text in the program: no files, no loading", 16, h - 14, 13)
end
