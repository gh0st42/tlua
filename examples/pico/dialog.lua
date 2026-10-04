-- dialog: running a loop of your own, with flip().
--
-- flip() shows what has been drawn and waits for the next tick. The program
-- carries on from exactly where it was, with fresh input — so a loop can live
-- anywhere, not only in _update.
--
-- That is what a modal dialog needs: branch out of the game, loop there until
-- the question is answered, and carry on. No state machine, no "am I in a
-- dialog" flag threaded through everything; the answer is simply what the
-- function returns.
--
-- X answers, left and right choose. Two of the boxes below ask a question the
-- ordinary way, from inside _update, and the third hands the whole loop over.

boot()

local w, h = screen()
local answers = { "not asked yet", "not asked yet", "not asked yet" }
local chosen = 1

-- ask draws a box and stays in it until somebody answers, which may be many
-- frames later. To the code that called it, it is one line that returns yes or
-- no.
local function ask(question)
	local pick = 1
	while true do
		-- A question deserves the bigger font. Taking it for the dialog and
		-- putting back whatever was in hand is what font() reporting the one
		-- it replaced is for: this routine does not know, or need to know,
		-- what the rest of the program draws with.
		local was = font("unscii")
		-- Whatever was on the screen when we were called is still there, so
		-- the dialog sits over the game rather than replacing it.
		fillp(0xa5a5, true)
		rectfill(0, 0, w, h, 0)
		fillp()

		local bw, bh = 230, 70
		local bx, by = (w - bw) / 2, (h - bh) / 2
		rrectfill(bx, by, bw, bh, 6, 1)
		rrect(bx, by, bw, bh, 6, 12)
		-- textwidth() follows the font in hand, so centring needs no arithmetic
		-- of its own when the font changes.
		print(question, (w - textwidth(question)) / 2, by + 14, 7)

		for i, word in ipairs({ "yes", "no" }) do
			local x = bx + 56 + (i - 1) * 86
			if i == pick then
				rrectfill(x - 8, by + 36, 42, 16, 5, 12)
			end
			print(word, x, by + 40, i == pick and 1 or 6)
		end
		local how = btnkey("x") .. " to answer"
		print(how, (w - textwidth(how)) / 2, by + bh - 14, 13)

		font(was) -- the game behind this is drawn in its own font

		-- Show it, wait a tick, and carry on from here. Reading the buttons
		-- afterwards rather than before matters: the press that opened this
		-- dialog is still "just pressed" on the tick it opened, and would
		-- answer the question as well as ask it.
		flip()

		if btnp("left") then pick = 1 end
		if btnp("right") then pick = 2 end
		if btnp("x") then return pick == 1 end
	end
end

function _update()
	if btnp("up") then chosen = max(chosen - 1, 1) end
	if btnp("down") then chosen = min(chosen + 1, #answers) end

	if btnp("x") then
		-- Branching out of the ordinary loop and into one of our own. The
		-- game is still here when it comes back.
		local said = ask(({
			"shall we begin?",
			"are you quite sure?",
			"really, truly sure?",
		})[chosen])
		answers[chosen] = said and "yes" or "no"
	end
end

function _draw()
	cls(1)
	print("dialog", 8, 8, 7)
	print("up and down choose a question, " .. btnkey("x") .. " asks it", 8, 20, 6)

	for i, answer in ipairs(answers) do
		local y = 44 + (i - 1) * 16
		if i == chosen then
			rrectfill(6, y - 4, 200, 14, 4, 5)
		end
		print("question " .. i, 12, y, i == chosen and 7 or 13)
		print(answer, 90, y, answer == "yes" and 11 or (answer == "no" and 8 or 6))
	end

	print("the dialog runs its own loop and stays put", 8, h - 22, 13)
	print("until it is answered; the game waits where it stood", 8, h - 12, 13)
end
