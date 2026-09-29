-- paint: drawing with the mouse, onto a surface of its own.
--
-- The canvas is a surface rather than the screen, so the palette down the side
-- can be drawn over the top of it every frame without smearing the picture. Any
-- program that needs a layer works this way: target() it, draw, target() back.
--
-- Left button paints, right button rubs out, the wheel changes the brush, C
-- clears, and the number keys 1 to 9 pick a colour without the mouse.

local w, h = screen()
local swatch = 14
local barWidth = swatch + 6

local canvas = surface(w - barWidth, h)
local colour = 8
local brush = 3
local lastX, lastY

-- The sixteen colours down the side: PICO-8's palette, which is as many as fit.
local choices = {}
for i = 0, 15 do add(choices, i) end

function _init()
	canvas:fill(7)
end

-- stroke joins the last position to this one, so that a quick sweep of the
-- mouse leaves a line rather than a dotted trail.
local function stroke(x, y, col)
	target(canvas)
	if lastX then
		-- A thick line is a row of circles along a thin one.
		local steps = max(abs(x - lastX), abs(y - lastY))
		for i = 0, steps do
			local px = lastX + (x - lastX) * i / max(steps, 1)
			local py = lastY + (y - lastY) * i / max(steps, 1)
			circfill(px, py, brush, col)
		end
	else
		circfill(x, y, brush, col)
	end
	target()
end

function _update()
	local mx, my, buttons, turn = mouse()

	brush = mid(brush + turn, 0, 12)
	if keyp("c") then canvas:fill(7) end
	for i = 1, 9 do
		if keyp(tostr(i)) then colour = choices[i] end
	end

	-- The palette is on the right; clicking it picks a colour instead of painting.
	if mx >= w - barWidth then
		if buttons > 0 then
			local i = flr(my / swatch) + 1
			if choices[i] then colour = choices[i] end
		end
		lastX, lastY = nil, nil
		return
	end

	if buttons == 0 then
		lastX, lastY = nil, nil
		return
	end
	if buttons == 1 then
		stroke(mx, my, colour)
	else
		stroke(mx, my, 7) -- any other button rubs out
	end
	lastX, lastY = mx, my
end

function _draw()
	cls(5)
	spr(canvas, 0, 0)

	-- The palette, with the chosen colour marked.
	for i, col in ipairs(choices) do
		local y = (i - 1) * swatch
		rectfill(w - barWidth + 3, y + 1, w - 3, y + swatch - 2, col)
		if col == colour then
			-- Two rings, dark then light, so the mark shows against any colour.
			rect(w - barWidth + 1, y, w - 1, y + swatch - 1, 0)
			rect(w - barWidth + 2, y + 1, w - 2, y + swatch - 2, 7)
		end
		print(tostr(i <= 9 and i or ""), w - barWidth - 5, y + 4, 6)
	end

	-- The brush, shown where the pointer is.
	local mx, my = mouse()
	if mx < w - barWidth then
		circ(mx, my, brush, 0)
		circ(mx, my, brush + 1, 7)
	end

	print("left paints  right rubs out  wheel: size " .. brush .. "  C clears", 4, h - 10, 0)
end
