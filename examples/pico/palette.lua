-- palette: whichever colours are loaded, with the number to ask for each one.
--
-- X switches between the palettes the console comes with; O fades the whole
-- picture by changing what the colours are, not by redrawing anything.

boot()

local w, h = screen()
local builtin = { "default", "vga" }
local chosen = 1
local fade = 0

-- The colours as loaded, so that a fade can be undone.
local original = {}

local function reload()
	local name, size = palette(builtin[chosen])
	original = {}
	for i = 0, size - 1 do original[i] = rgb(i) end
	fade = 0
	return name, size
end

function _init()
	reload()
end

-- dim mixes a colour towards black. The channels are pulled apart, scaled and
-- put back together, which is all a fade is.
local function dim(colour, amount)
	local r = flr(colour / 65536) % 256
	local g = flr(colour / 256) % 256
	local b = colour % 256
	r, g, b = flr(r * amount), flr(g * amount), flr(b * amount)
	return r * 65536 + g * 256 + b
end

function _update()
	if btnp("x") then
		chosen = chosen % #builtin + 1
		reload()
	end

	-- Every frame the fade is on, every colour is set again from the one it
	-- started as: sixty-four writes, and the whole screen changes.
	if btn("o") then
		fade = clamp(fade + 0.02, 0, 1)
	else
		fade = clamp(fade - 0.04, 0, 1)
	end
	for i, colour in pairs(original) do
		palette(i, dim(colour, 1 - fade))
	end
end

function _draw()
	local name, size = palette()

	-- Sixteen across for a big palette, eight for a small one.
	local across = 8
	if size > 64 then across = 16 end
	local down = ceil(size / across)
	local cw, ch = flr(w / across), flr((h - 12) / down)

	cls(0)
	for i = 0, size - 1 do
		local x, y = (i % across) * cw, flr(i / across) * ch
		rectfill(x, y, x + cw - 2, y + ch - 2, i)
		if ch >= 11 then
			-- A shadow under the number, so it can be read on any colour.
			print(tostr(i), x + 3, y + 3, 0)
			print(tostr(i), x + 2, y + 2, 7)
		end
	end

	local label = name .. ": " .. size .. " colours   " ..
		btnkey("x") .. " changes it   hold " .. btnkey("o") .. " to fade"
	print(label, 3, h - 8, 0)
	print(label, 2, h - 9, 7)
end
