-- snake: a whole game in a hundred lines.
--
-- Arrow keys to turn, X to start again. Shows the shape of a game with states
-- in it: a title, a game, and an ending, each drawn and updated differently.
--
-- The best score outlives the game: store() keeps it where the person's other
-- saved games go, and fetch() reads it back the next time this is run.

boot()

local w, h = screen()
local cell = 10
local cols, rows = flr(w / cell) - 2, flr((h - 20) / cell)
local left, top = (w - cols * cell) / 2, 20

local snake, food, heading, next_heading, score, best, state, ticks
best = 0

local function grid_to_screen(c)
	return left + c.x * cell, top + c.y * cell
end

local function free(x, y)
	for part in all(snake) do
		if part.x == x and part.y == y then return false end
	end
	return true
end

local function drop_food()
	-- Guess a few times, which is quick while there is room, then give up
	-- guessing and take the first free square. A board nearly full of snake
	-- would have the guessing going round for ever.
	for _ = 1, 100 do
		local x, y = flr(rnd(cols)), flr(rnd(rows))
		if free(x, y) then
			food = { x = x, y = y }
			return
		end
	end
	for y = 0, rows - 1 do
		for x = 0, cols - 1 do
			if free(x, y) then
				food = { x = x, y = y }
				return
			end
		end
	end
	food = { x = 0, y = 0 } -- nowhere left: the board is entirely snake
end

local function start()
	snake = {}
	for i = 0, 3 do
		add(snake, { x = flr(cols / 2) - i, y = flr(rows / 2) })
	end
	heading = { x = 1, y = 0 }
	next_heading = heading
	score = 0
	ticks = 0
	state = "playing"
	drop_food()
end

function _init()
	-- Whatever was saved last time, if anything was, and if it is still a
	-- number: a save file is text and can be edited into nonsense.
	local saved = fetch("best")
	best = type(saved) == "number" and saved or 0

	start()
	state = "title"
end

local function turn(x, y)
	-- A snake cannot double back on itself, only turn.
	if heading.x ~= -x or heading.y ~= -y then
		next_heading = { x = x, y = y }
	end
end

local function step()
	heading = next_heading
	local head = snake[1]
	local ahead = { x = head.x + heading.x, y = head.y + heading.y }

	if ahead.x < 0 or ahead.x >= cols or ahead.y < 0 or ahead.y >= rows then
		state = "over"
		return
	end
	for part in all(snake) do
		if part.x == ahead.x and part.y == ahead.y then
			state = "over"
			return
		end
	end

	add(snake, ahead, 1)
	if ahead.x == food.x and ahead.y == food.y then
		score = score + 1
		if score > best then
			best = score
			store("best", best) -- kept for the next time this is run
		end
		drop_food()
	else
		deli(snake) -- no food, so the tail keeps up with the head
	end
end

function _update()
	if state ~= "playing" then
		if btnp("x") then start() end
		return
	end

	if btnp("left") then turn(-1, 0) end
	if btnp("right") then turn(1, 0) end
	if btnp("up") then turn(0, -1) end
	if btnp("down") then turn(0, 1) end

	-- Faster as the snake grows, but never faster than four frames a step.
	ticks = ticks + 1
	local every = max(4, 10 - flr(score / 4))
	if ticks % every == 0 then step() end
end

local function centred(text, y, col)
	print(text, (w - textwidth(text)) / 2, y, col)
end

function _draw()
	cls(1)

	-- The pen, and the board it is drawn on.
	rectfill(left - 2, top - 2, left + cols * cell + 1, top + rows * cell + 1, 0)
	rect(left - 3, top - 3, left + cols * cell + 2, top + rows * cell + 2, 5)

	local fx, fy = grid_to_screen(food)
	circfill(fx + cell / 2, fy + cell / 2, cell / 2 - 1, 8)

	for i, part in ipairs(snake) do
		local x, y = grid_to_screen(part)
		local col = 11
		if i == 1 then col = 10 end -- the head
		rectfill(x, y, x + cell - 2, y + cell - 2, col)
	end

	print("score " .. score, 8, 8, 7)
	print("best " .. best, w - 60, 8, 6)

	-- The panels sit above the middle, so that the snake waiting underneath can
	-- be seen behind them. They are set in the bigger font — the score above is
	-- a number in the corner and the small one suits it, but a sentence meant
	-- to be read across the room is not.
	if state ~= "playing" then font("unscii") end

	if state == "title" then
		local top = h / 3 - 24
		rectfill(0, top, w, top + 48, 0)
		rect(0, top, w - 1, top + 48, 5)
		centred("▓ SNAKE ▓", top + 6, 10)
		centred("arrow keys to turn", top + 22, 7)
		centred("press " .. btnkey("x") .. " to start", top + 34, 12)
	elseif state == "over" then
		local top = h / 3 - 16
		rectfill(0, top, w, top + 36, 0)
		rect(0, top, w - 1, top + 36, 5)
		centred("caught yourself out", top + 8, 8)
		centred(btnkey("x") .. " to go again", top + 22, 12)
	end

	font("small")
end
