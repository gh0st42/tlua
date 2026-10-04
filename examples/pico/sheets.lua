-- sheets: one picture cut into sprites, and drawn by number.
--
-- A surface with a grid on it is a sprite sheet: spr() then takes the number of
-- a sprite rather than a place to put a picture. The cells can be any size —
-- 8x8, 16x16, whatever the artwork was drawn at — and the same pixels can be
-- read at more than one size, because a grid is only a way of counting.
--
--   spr(sheet, n, x, y)          sprite n of that sheet
--   spr(sheet, n, x, y, 2, 2)    two cells by two, from n
--   usesheet(sheet)  spr(n, x, y)   the same, Picotron's own spelling
--
-- Sprites are counted from zero. map() leaves sprite 0 undrawn unless told
-- otherwise, so leaving the first cell blank is what makes a dot in a level
-- mean empty sky.

boot()

local w, h = screen()

-- Four cells of eight pixels: nothing, a ball, a box, a spark.
local small = sprite([[
	..........cccc..55555555...aa...
	.........c7ccccc56666665...aa...
	........c7cccccc56555565.a.aa.a.
	........cccccccc56555565..aaaa..
	........cccccccc56555565aaaaaaaa
	........cccccccc56555565..aaaa..
	.........cccccc.56666665.a.aa.a.
	..........cccc..55555555...aa...
]], 8, 8)

-- Two of sixteen. Bigger cells, same idea.
local big = sprite([[
	....ffffffff...........88.......
	..ffffffffffff........8888......
	.ffffffffffffff......888888.....
	ffffffffffffffff....88888888....
	ffffffffffffffff...8888888888...
	fff00ffffff00fff..888888888888..
	fff00ffffff00fff.88888888888888.
	ffffffffffffffff8888888888888888
	ffffffffffffffff.55555555555555.
	ffff8ffffff8ffff.5cc55555555cc5.
	fffff888888fffff.5cc55555555cc5.
	ffffffffffffffff.55555555555555.
	ffffffffffffffff.55444455555555.
	.ffffffffffffff..55444455555555.
	..ffffffffffff...55444455555555.
	....ffffffff.....55444455555555.
]], 16, 16)

local spin = 0

function _update()
	spin = spin + 1
	-- X re-cuts the small sheet at another size, to show that the grid is a
	-- way of reading the pixels and nothing more.
	if btnp("x") then
		local cw = small:grid()
		small:grid(cw == 8 and 4 or 8)
	end
end

local function label(text, x, y)
	print(text, x, y, 6)
end

function _draw()
	cls(1)

	local cw, ch, count = small:grid()
	label("a sheet of " .. cw .. "x" .. ch .. ", " .. count .. " sprites   " ..
		btnkey("x") .. " cuts it the other way", 8, 8)

	-- Every sprite on the sheet, by number.
	for n = 0, count - 1 do
		local x = 8 + n * (cw + 4)
		if x < w - cw then
			spr(small, n, x, 20)
			print(tostr(n), x, 20 + ch + 2, 13)
		end
	end

	label("sixteen by sixteen", 8, 56)
	spr(big, 0, 8, 68)
	spr(big, 1, 32, 68)

	label("flipped, and spanning two cells", 100, 56)
	spr(big, 1, 100, 68)             -- the house as it is drawn
	spr(big, 1, 120, 68, 1, 1, true) -- and the other way round
	spr(big, 0, 148, 68, 2, 1)       -- both cells at once, as one wide sprite

	label("the current sheet: usesheet() then spr(n, x, y)", 8, 96)
	usesheet(big)
	for n = 0, 1 do
		spr(n, 8 + n * 20, 108)
	end

	-- A sprite taken off the sheet is a surface of its own, and can be drawn
	-- on, stretched, or used as a picture.
	label("taken off the sheet with sheet:sprite(n)", 8, 136)
	local ball = small:sprite(1)
	for i = 0, 5 do
		local size = 8 + i * 6
		sspr(ball, 0, 0, ball:width(), ball:height(), 8 + i * 34, 148, size, size)
	end

	-- And the whole sheet is still just a picture underneath.
	label("the sheet itself, as one picture", 8, 200)
	sspr(small, 0, 0, small:width(), small:height(), 8, 212, small:width() * 2, small:height() * 2)

	local bob = sin(spin / 60) * 6
	spr(big, 1, w - 40, h / 2 + bob)
end
