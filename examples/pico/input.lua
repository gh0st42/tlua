-- input: everything the console can tell a program about who is playing.
--
-- Buttons come from the arrow keys with Z and X beside them, from ESDF for a
-- second player, and from any game pads plugged in. key() reads the keyboard
-- directly for anything a console never had.

local w, h = screen()
local names = { "left", "right", "up", "down", "o", "x" }
local watched = { "space", "shift", "ctrl", "alt", "enter", "escape", "tab", "a", "1" }
local wheel = 0
local trail = {}

function _update()
	local _, _, _, turn = mouse()
	wheel = wheel + turn

	local mx, my, buttons = mouse()
	if buttons > 0 then
		add(trail, { x = mx, y = my, life = 30, col = buttons })
	end
	for i = #trail, 1, -1 do
		trail[i].life = trail[i].life - 1
		if trail[i].life <= 0 then deli(trail, i) end
	end
end

-- lamp draws a box that lights up while something is held.
local function lamp(x, y, label, on, pressed)
	local col = 5
	if on then col = 11 end
	if pressed then col = 10 end
	rectfill(x, y, x + 26, y + 10, col)
	rect(x, y, x + 26, y + 10, 6)
	print(label, x + 3, y + 3, on and 0 or 6)
end

function _draw()
	cls(1)
	print("buttons", 8, 6, 7)
	for player = 0, 1 do
		print("player " .. (player + 1), 8, 20 + player * 30, 6)
		for i, name in ipairs(names) do
			lamp(50 + (i - 1) * 30, 16 + player * 30, name,
				btn(name, player), btnp(name, player))
		end
	end

	print("keys", 8, 84, 7)
	for i, name in ipairs(watched) do
		lamp(50 + ((i - 1) % 9) * 30, 80, name, key(name), keyp(name))
	end

	local typing = typed()
	if typing ~= "" then print("typed: " .. typing, 8, 96, 10) end

	print("mouse", 8, 112, 7)
	local mx, my, buttons = mouse()
	print("at " .. mx .. "," .. my .. "   buttons " .. buttons .. "   wheel " .. flr(wheel), 50, 112, 6)
	print("hold a button and move to draw", 50, 122, 13)

	for spot in all(trail) do
		circfill(spot.x, spot.y, spot.life / 8, 8 + spot.col)
	end

	-- A pointer of its own, since the console has no cursor to lend.
	line(mx - 4, my, mx + 4, my, 7)
	line(mx, my - 4, mx, my + 4, 7)

	-- The console binds keys by where they are, so what they are printed with
	-- depends on the keyboard; btnkey() asks.
	print("player 1 is the arrows with " .. btnkey("o", 0) .. " and " .. btnkey("x", 0) ..
		"   player 2 is ESDF with " .. btnkey("o", 1) .. " and " .. btnkey("x", 1), 8, h - 22, 13)
	print("held X for " .. held("x") .. " ticks", 8, h - 12, 12)
	print(flr(fps()) .. " fps", w - 40, h - 12, 12)
end
