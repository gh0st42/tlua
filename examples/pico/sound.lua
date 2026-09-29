-- sound: sfx() and music(), and where they look for what you ask them for.
--
-- This example ships with no sound files, because everything here is one file
-- of text and a .wav is not that. Drop one in beside it and it will be found:
--
--   sfx/jump.wav        or  assets/sfx/jump.wav
--   music/theme.ogg     or  assets/music/theme.ogg
--
-- A name with no extension is looked for with each of the ones the console
-- knows: sfx("jump") finds jump.wav or jump.ogg, wherever it is. A name with a
-- path is taken as it stands. Nothing is ever an error: a sound that is not
-- there comes back as nil and a message saying where it looked, which is what
-- the bottom of this screen shows.

local w, h = screen()
local wanted = { "jump", "coin", "hit", "theme" }
local last = "nothing yet"
local heard = {}

-- try plays a sound and remembers how it went, so the screen can show it.
local function try(name)
	local channel, why = sfx(name)
	if channel then
		heard[name] = "channel " .. channel
		last = name .. " played on channel " .. channel
	else
		heard[name] = "not found"
		last = why
	end
end

function _update()
	if btnp("x") then try(wanted[1 + frame() % #wanted]) end
	if btnp("o") then
		if music() then
			music(-1, 500) -- half a second to fade out
			last = "music fading out"
		else
			local playing, why = music("theme", 500)
			last = playing and ("music: " .. playing) or why
		end
	end
	if btnp("up") then volume(min(volume() + 0.1, 1)) end
	if btnp("down") then volume(max(volume() - 0.1, 0)) end
end

function _draw()
	cls(1)
	print("sound", 8, 8, 7)
	print(btnkey("x") .. " plays a sound   " .. btnkey("o") .. " starts or stops the music", 8, 20, 6)
	print("up and down change the volume (" .. flr(volume() * 100) .. "%)", 8, 30, 6)

	print("what it tried:", 8, 48, 13)
	local y = 60
	for name in all(wanted) do
		print(name, 16, y, 7)
		print(heard[name] or "-", 80, y, heard[name] == "not found" and 8 or 11)
		y = y + 10
	end

	print("music: " .. tostr(music() or "silent"), 8, y + 8, 12)

	-- The last thing that happened, which for a missing sound is the list of
	-- everywhere it was looked for.
	print("last:", 8, h - 40, 13)
	local text = tostr(last)
	local line, at = "", h - 30
	for word in all(split(text, " ", false)) do
		if textwidth(line .. " " .. word) > w - 16 then
			print(line, 8, at, 6)
			line, at = word, at + 8
		else
			line = line == "" and word or (line .. " " .. word)
		end
	end
	print(line, 8, at, 6)
end
