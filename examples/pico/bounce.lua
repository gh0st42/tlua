-- bounce: a hundred balls, gravity, and the walls.
--
-- The shape of nearly every game here: a list of things, _update() moves them,
-- _draw() draws them. add() and all() are the two calls that make that short.

boot()

local w, h = screen()
local balls = {}
local gravity = 0.25

local function spawn()
	add(balls, {
		x = rnd(w),
		y = rnd(h / 2),
		dx = rnd(3) - 1.5,
		dy = 0,
		r = 2 + rnd(5),
		col = 8 + flr(rnd(8)),
	})
end

function _init()
	for _ = 1, 60 do spawn() end
end

function _update()
	if btnp("x") then spawn() end
	if btnp("o") then balls = {} end

	for b in all(balls) do
		b.dy = b.dy + gravity
		b.x = b.x + b.dx
		b.y = b.y + b.dy

		-- Bouncing costs a little of the speed, so the balls settle.
		if b.x - b.r < 0 then b.x, b.dx = b.r, -b.dx * 0.9 end
		if b.x + b.r > w then b.x, b.dx = w - b.r, -b.dx * 0.9 end
		if b.y + b.r > h then b.y, b.dy = h - b.r, -b.dy * 0.8 end
	end
end

function _draw()
	cls(1)
	for b in all(balls) do
		circfill(b.x, b.y, b.r, b.col)
		circ(b.x, b.y, b.r, 7) -- a highlight, to show the outline is a circle too
	end
	-- btnkey() names the key as this keyboard prints it, which is not always
	-- the letter the console binds: the place of a key and its label differ.
	print(#balls .. " balls   " .. btnkey("x") .. " adds one, " .. btnkey("o") .. " clears", 4, 4, 7)
end
