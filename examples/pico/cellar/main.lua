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
--
-- The tileset says more than that. Each tile carries a material, and water
-- carries how much it slows a walk, so how wading feels is set in the editor
-- rather than here. Flags are the bit a loop tests; properties are the detail
-- behind it.

boot()

local SOLID, WATER = 0, 1

local w, h = screen()
local level, tiles, player, treasure
local splash = 0

function _init()
	level = loadmap("level1")
	usemap(level)
	tiles = loadpng("tiles")
	usesheet(tiles)

	-- Where to start, and what to look for, both placed in the editor. The
	-- treasure was placed as a tile, so it arrives with a sprite to draw it
	-- with; anything placed as a plain box has no sprite field at all.
	for thing in all(level:objects("things")) do
		if thing.class == "start" then
			player = { x = thing.x, y = thing.y, speed = thing.props.speed or 1 }
		elseif thing.class == "pickup" then
			treasure = {
				x = thing.x, y = thing.y - select(2, level:tile()), -- its foot is its y
				sprite = thing.sprite, sheet = thing.sheet, taken = false,
			}
		end
	end
end

-- underfoot reports the sprite the player is standing on, which is what the
-- questions below are all really about.
local function underfoot()
	local tw, th = level:tile()
	return mget(flr((player.x + 4) / tw), flr((player.y + 4) / th))
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
	return fget(underfoot(), WATER)
end

function _update()
	local speed = player.speed
	if wading() then
		-- How much water slows a walk is the tileset's business, not this
		-- program's: prop() asks the artwork, and 1 is for a tile that says
		-- nothing about it.
		speed = speed / tiles:prop(underfoot(), "slowdown", 1)
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
		spr(level:sheet(treasure.sheet), treasure.sprite, treasure.x, treasure.y + bob)
	end

	-- The player: a box, as ever.
	rectfill(player.x + 1, player.y + 1, player.x + 6, player.y + 6, 14)
	pset(player.x + 3, player.y + 3, 0)
	pset(player.x + 5, player.y + 3, 0)

	camera()
	print(level:props().title or "somewhere", 6, 6, 7)
	-- What the player is standing on, as the tileset has it. The walls say
	-- nothing of their own, so they answer with what the sheet says for
	-- every sprite on it.
	print(tiles:prop(underfoot(), "material", "nothing"), 6, 16, 12)
	if treasure and treasure.taken then
		print("you have the treasure", 6, h - 12, 10)
	else
		print("arrows to walk; the water slows you", 6, h - 12, 13)
	end
end
