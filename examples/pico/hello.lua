-- hello: the smallest program the console will run.
--
--   tlua play examples/pico/hello.lua
--   tlua examples/pico/hello.lua       -- the same, because of the boot() below
--
-- _draw() is called sixty times a second. t() is how long the program has been
-- running, and sin() takes turns rather than radians: sin(0.5) is half a circle.

-- boot() says this program wants a window and the console. It is needed only
-- when the program is run as an ordinary script, and does no harm when it is
-- not, so a file that says it runs either way. It goes first: nothing else
-- here exists until it has.
boot()

local w, h = screen()
local greeting = "hello from tlua"

function _draw()
	cls(1)

	local bob = sin(t() / 2) * 10
	print(greeting, (w - textwidth(greeting)) / 2, h / 2 - 12 + bob, 7)

	circfill(w / 2, h / 2 + 20, 8 + sin(t()) * 3, 12)
	print("press ctrl-Q to quit", 4, h - 10, 13)
end
