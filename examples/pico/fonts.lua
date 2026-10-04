-- fonts: the two that come with the console, and one drawn here.
--
--   tlua play examples/pico/fonts.lua
--
-- font() picks which one print() draws with, and hands back the one it
-- replaced so a routine can borrow a font and put things back after itself.

boot()

local w, h = screen()

-- A font of your own is a sheet of lettering, one cell a character. This one
-- is four digits of 5x7, starting at "0", drawn in a colour it will never be
-- printed in: every pixel that is not colour 0 is ink, and the colour comes
-- from print().
local digits = sprite([[
	.333...3...333..333.
	3...3.33......3....3
	3...3..3......3....3
	3...3..3....33...333
	3...3..3...3.......3
	3...3..3...3.......3
	.333..333..33333.333
]], 5, 7)

local lines = {
	{ "small", "The quick brown fox jumps over the lazy dog" },
	{ "small", "0123456789  !?.,:;()[]{}  +-*/=  <>#@&%$" },
	{ "unscii", "The quick brown fox jumps over the lazy dog" },
	{ "unscii", "0123456789  !?.,:;()[]{}  +-*/=  <>#@&%$" },
	{ "unscii", "héllo wörld  ¿cómo estás?  Привет  Γειά σου" },
	{ "unscii", "░▒▓█  ╔══╗ ╠══╣ ╚══╝  ←↑→↓  ▲▼◄►  ♠♥♦♣" },
}

function _draw()
	cls(1)

	local y = 8
	for i, line in ipairs(lines) do
		local name, text = line[1], line[2]
		font(name)
		print(name, 8, y, 5)
		print(text, 8 + 48, y, i % 2 == 1 and 7 or 6)
		y = y + select(3, font()) + 4
	end

	-- The home-made one. Borrowing it and putting back what was there is the
	-- shape worth copying: a routine that draws its own way need not know what
	-- the rest of the program was using.
	local was = font(digits, 48) -- its first cell is "0"
	print("0123", 8 + 48, y + 6, 10)
	print("0123", 8 + 48 + 60, y + 6, 12) -- the same sheet, another colour
	font(was)
	print("yours", 8, y + 8, 5)

	-- What the box-drawing characters are for: a panel drawn out of text,
	-- which is how every machine with a character set and no sprites did it.
	font("unscii")
	local panel = {
		"╔══════════════════════╗",
		"║  ♦ INVENTORY ♦       ║",
		"╠══════════════════════╣",
		"║  rope          ×1    ║",
		"║  lamp          ×1    ║",
		"║  coins       ×128    ║",
		"╚══════════════════════╝",
	}
	local px = (w - textwidth(panel[1])) / 2
	local py = y + 28
	for i, row in ipairs(panel) do
		print(row, px, py + (i - 1) * 8, i == 2 and 10 or 6)
	end

	-- textwidth() follows whichever font is in hand, so centring works in all
	-- of them without being told which.
	local note = "font() reports the font it replaced"
	print(note, (w - textwidth(note)) / 2, h - 16, 13)
	font("small")
end
