-- cellar: a level drawn in a map editor, rather than typed into the program.
--
-- This one is a folder rather than a single file, because a map is made of
-- three: the level itself, the tileset it draws with, and the picture that
-- names. They sit where such things go, and nothing here says where that is:
--
--   maps/level1.tmj    the level, as Tiled writes it
--   gfx/tiles.tsj      the tileset it names
--   gfx/tiles.png      the artwork, carrying its own tile size and flags
--
-- Run it with: tlua play examples/pico/cellar
--
-- What makes it work without a table of tile numbers anywhere is sprite flags.
-- The artwork says which tiles are solid and which are water; the program asks
-- fget(mget(x, y), SOLID) and never learns that a wall is sprite 1.

local SOLID, WATER = 0, 1

local w, h = screen()
local level, player, treasure
local splash = 0

function _init()
	level = loadmap("level1")
	usemap(level)
	usesheet(loadpng("tiles"))

	-- Where to start, and what to look for, both placed in the editor.
	for thing in all(level:objects("things")) do
		if thing.class == "start" then
			player = { x = thing.x, y = thing.y, speed = thing.props.speed or 1 }
		elseif thing.class == "pickup" then
			treasure = { x = thing.x, y = thing.y, taken = false }
		end
	end
end

-- blocked reports whether a box of the world is inside anything solid.
local function blocked(x, y)
	local tw = select(1, level:tile())
	for _, corner in ipairs({ { 1, 1 }, { 6, 1 }, { 1, 6 }, { 6, 6 } }) do
		if fget(mget(flr((x + corner[1]) / tw), flr((y + corner[2]) / tw)), SOLID) then
			return true
		end
	end
	return false
end

-- wading reports whether the player is standing in water, which slows them.
local function wading()
	local tw = select(1, level:tile())
	return fget(mget(flr((player.x + 4) / tw), flr((player.y + 4) / tw)), WATER)
end

function _update()
	local speed = player.speed
	if wading() then
		speed = speed / 2
		splash = splash + 1
	end

	local dx, dy = 0, 0
	if btn("left") then dx = -speed end
	if btn("right") then dx = speed end
	if btn("up") then dy = -speed end
	if btn("down") then dy = speed end

	-- One axis at a time, so a wall stops the walk into it and nothing else.
	if not blocked(player.x + dx, player.y) then player.x = player.x + dx end
	if not blocked(player.x, player.y + dy) then player.y = player.y + dy end

	if treasure and not treasure.taken then
		if abs(player.x - treasure.x) < 6 and abs(player.y - treasure.y) < 6 then
			treasure.taken = true
		end
	end
end

-- view reports where the camera goes along one axis: following what it is
-- watching, held inside the level, or centred when the level is the smaller.
local function view(at, level_size, screen_size)
	if level_size <= screen_size then
		return -(screen_size - level_size) / 2
	end
	return mid(at - screen_size / 2, 0, level_size - screen_size)
end

function _draw()
	cls(1)

	-- The view follows the player about a level bigger than the screen, and
	-- sits still in the middle of one that is not.
	local mw, mh = level:size()
	local tw, th = level:tile()
	camera(view(player.x, mw * tw, w), view(player.y, mh * th, h))

	map()

	if treasure and not treasure.taken then
		local bob = sin(t()) * 2
		circfill(treasure.x + 4, treasure.y + 4 + bob, 3, 10)
		circ(treasure.x + 4, treasure.y + 4 + bob, 3, 9)
	end

	-- The player: a box, as ever.
	rectfill(player.x + 1, player.y + 1, player.x + 6, player.y + 6, 14)
	pset(player.x + 3, player.y + 3, 0)
	pset(player.x + 5, player.y + 3, 0)

	camera()
	print(level:props().title or "somewhere", 6, 6, 7)
	if wading() then
		print("wading", 6, 16, 12)
	end
	if treasure and treasure.taken then
		print("you have the treasure", 6, h - 12, 10)
	else
		print("arrows to walk; the water slows you", 6, h - 12, 13)
	end
end
