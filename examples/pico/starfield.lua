-- starfield: three hundred stars, one pset each.
--
-- Up and down change the speed. The stars are kept in three dimensions and
-- divided by their distance, which is the whole of perspective.

local w, h = screen()
local cx, cy = w / 2, h / 2
local stars = {}
local speed = 1

local function place(star)
	star.x = rnd(200) - 100
	star.y = rnd(200) - 100
	star.z = 100
end

function _init()
	for i = 1, 300 do
		local star = {}
		place(star)
		star.z = rnd(100) -- scattered through the depth to begin with
		add(stars, star)
	end
end

function _update()
	if btn("up") then speed = min(speed + 0.05, 4) end
	if btn("down") then speed = max(speed - 0.05, 0.1) end

	for star in all(stars) do
		star.z = star.z - speed
		if star.z <= 1 then place(star) end
	end
end

function _draw()
	cls(0)
	for star in all(stars) do
		local scale = 64 / star.z
		local x, y = cx + star.x * scale, cy + star.y * scale

		-- Nearer stars are brighter and, once near enough, more than a pixel.
		local col = 1
		if star.z < 70 then col = 13 end
		if star.z < 40 then col = 6 end
		if star.z < 15 then col = 7 end

		if star.z < 8 then
			circfill(x, y, 1, col)
		else
			pset(x, y, col)
		end
	end

	print("up and down change the speed (" .. sub(tostr(speed), 1, 4) .. ")", 4, h - 10, 5)
end
