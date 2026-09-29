-- platformer: a tile map, gravity, and a camera that follows.
--
-- The level is written out as text and turned into a grid of tile numbers once.
-- That grid is both what map() draws and what the collision reads, so what is
-- solid is always what can be seen.

local w, h = screen()
local tile = 8

-- Five cells of eight pixels: nothing, earth, grass, stone, coin. The first
-- is left blank on purpose, because map() does not draw sprite 0 — which is
-- what lets a dot in the level below mean open sky.
local tiles = sprite([[
	........444444443333333366666666........
	........445444443333333365555556..9999..
	........444444444434443465555556.9aaaa9.
	........444445444444444465555556.9a99a9.
	........444444444454444465555556.9a99a9.
	........454444444444444465555556.9aaaa9.
	........444444444444454465555556..9999..
	........444444544444444466666666........
]], 8, 8)

local EARTH, GRASS, STONE, COIN = 1, 2, 3, 4

-- What a tile is, kept as flags on the artwork rather than as a list of tile
-- numbers down here. A sheet drawn in an editor arrives with its flags already
-- on it — loadpng() reads them out of the PNG — so a level can be walked on
-- without the program knowing which number means stone.
local SOLID, PICKUP = 0, 1

usesheet(tiles)
fset(EARTH, SOLID, true)
fset(GRASS, SOLID, true)
fset(STONE, SOLID, true)
fset(COIN, PICKUP, true)

-- The level as it starts, written as sprite numbers: 1 earth, 2 grass, 3 stone,
-- 4 coin, and a dot for the blank sprite 0, which is air.
local layout = {
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"................................................................................",
	"......................4.......................4.................................",
	"............................................222222..............................",
	"....................33333.......................................................",
	"....................................4...........................................",
	"..................................22222.........................4...............",
	"..............................................................22222.............",
	"..........................4.....................................................",
	"........................22222..............4....................................",
	".........................................22222...........4......................",
	"................4......................................22222....................",
	"..............22222....................................................4........",
	"...................................4................................2222222.....",
	".................................222222...........4.............................",
	"........4.............4.........................222222..........................",
	"......222222.........2222.......................................................",
	"............................................333.................................",
	"...............................................................333..............",
	"...4..........................................................................4.",
	"22222222222222222222222222.....2222222222222222222222222.....2222222222222222222",
	"11111111111111111111111111.....1111111111111111111111111.....1111111111111111111",
	"11111111111111111111111111.....1111111111111111111111111.....1111111111111111111",
	"11111111111111111111111111.....1111111111111111111111111.....1111111111111111111",
	"11111111111111111111111111.....1111111111111111111111111.....1111111111111111111",
	"11111111111111111111111111.....1111111111111111111111111.....1111111111111111111",
}

local rows, cols = #layout, #layout[1]
local cells = {}
local player = {}
local coins, collected = 0, 0

-- cell reads the grid. Off the sides counts as solid, so the player cannot walk
-- out of the level; above and below it is open air.
local function cell(col, row)
	if row < 1 or row > rows then return 0 end
	if col < 0 or col >= cols then return STONE end
	return cells[row][col + 1]
end

local function solid(col, row)
	return fget(cell(col, row), SOLID)
end

-- take picks up a coin by rubbing it out of the grid, which also stops it being
-- drawn.
local function take(col, row)
	if fget(cell(col, row), PICKUP) then
		cells[row][col + 1] = 0
		collected = collected + 1
	end
end

-- at reports which tile a point in the world is in. Rows count from 1, as the
-- level is written.
local function at(x, y)
	return flr(x / tile), flr(y / tile) + 1
end

-- How long a jump is remembered for, and how long after walking off a ledge it
-- still counts. Without the first, a press a moment before landing is thrown
-- away; without the second, a press a moment after stepping off is. Both feel
-- like the button not working, and both are measured in frames.
local jumpBuffer, coyoteTime = 8, 6

function _init()
	cells, coins, collected = {}, 0, 0
	for row = 1, rows do
		local line = {}
		for col = 1, cols do
			local ch = sub(layout[row], col, col)
			local what = 0
			if ch ~= "." then what = tonum(ch) or 0 end
			if fget(what, PICKUP) then coins = coins + 1 end
			add(line, what)
		end
		add(cells, line)
	end
	player = {
		x = 24, y = 80, dx = 0, dy = 0, w = 6, h = 10,
		facing = 1, grounded = false,
		buffered = 0, coyote = 0,
	}
end

function _update()
	local p = player

	local speed = 0
	if btn("left") then speed = -1.5 end
	if btn("right") then speed = 1.5 end
	if speed ~= 0 then p.facing = sgn(speed) end

	p.dy = min(p.dy + 0.3, 6) -- gravity, up to a terminal speed

	-- held() counts the ticks a button has been down, so held() == 1 is the
	-- press itself and nothing else: btnp() would also fire again while the
	-- button is held, which is a repeat rather than a jump.
	if held("o") == 1 then p.buffered = jumpBuffer end
	if p.grounded then p.coyote = coyoteTime end

	if p.buffered > 0 and p.coyote > 0 then
		p.dy = -4.6
		p.buffered, p.coyote = 0, 0 -- so that one press is one jump
	end
	p.buffered = max(p.buffered - 1, 0)
	p.coyote = max(p.coyote - 1, 0)

	-- One axis at a time, so that hitting a wall and landing on a floor are
	-- told apart rather than cancelling each other out.
	p.x = p.x + speed
	if speed ~= 0 then
		local edge = p.x
		if speed > 0 then edge = p.x + p.w - 1 end
		local col, head = at(edge, p.y)
		local _, foot = at(edge, p.y + p.h - 1)
		if solid(col, head) or solid(col, foot) then
			p.x = p.x - speed
		end
	end

	p.y = p.y + p.dy
	p.grounded = false
	local edge = p.y
	if p.dy > 0 then edge = p.y + p.h - 1 end
	local leftCol, row = at(p.x, edge)
	local rightCol = at(p.x + p.w - 1, edge)
	if solid(leftCol, row) or solid(rightCol, row) then
		if p.dy > 0 then
			p.y = (row - 1) * tile - p.h
			p.grounded = true
		else
			p.y = row * tile
		end
		p.dy = 0
	end

	-- Coins are collected by standing in them.
	take(at(p.x, p.y))
	take(at(p.x + p.w - 1, p.y))
	take(at(p.x, p.y + p.h - 1))
	take(at(p.x + p.w - 1, p.y + p.h - 1))

	if p.y > rows * tile + 40 then _init() end -- fallen off the world
end

function _draw()
	cls(12)

	-- The camera follows the player and stops at the ends of the level.
	local p = player
	local camx = mid(p.x - w / 2, 0, cols * tile - w)
	camera(camx, 0)

	-- Hills on the horizon, moved at half the camera's speed: the oldest trick
	-- there is for depth. They are drawn before the level, so the ground stands
	-- in front of them.
	for i = 0, 11 do
		local x = i * 110 + camx * 0.5
		ovalfill(x, 168, x + 150, 264, 47)      -- far hills, darker
		ovalfill(x + 55, 196, x + 175, 268, 27) -- nearer ones, brighter
	end

	map(cells, tiles) -- the tile size comes from the sheet

	-- The player is a box with a face on it, which is all a placeholder needs.
	rectfill(p.x, p.y, p.x + p.w - 1, p.y + p.h - 1, 14)
	local eye = p.x + 1
	if p.facing > 0 then eye = p.x + 3 end
	pset(eye, p.y + 3, 0)
	pset(eye + 2, p.y + 3, 0)

	camera()
	print("coins " .. collected .. "/" .. coins, 6, 6, 7)
	-- btnkey() names the key as this keyboard prints it: the console binds the
	-- place rather than the letter, and they are not the same everywhere.
	print("arrows to move, " .. btnkey("o") .. " to jump", 6, h - 10, 7)
end
