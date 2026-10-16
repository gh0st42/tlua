-- term demo, Turbo Pascal and QBasic style: tlua examples/console/term_demo.lua

local term = require("term")

term.clrscr()
term.gotoxy(1, 1)
term.normvideo()

term.println("term demo")
term.println("---------")

term.println("16 EGA text colors (QBasic order):")
for i = 0, 15 do
    term.print(string.format("{%d}%2d ", i, i))
end
term.println("")

term.println("Foreground + background examples (inline tags):")
term.println("{yellow,blue}Yellow on blue{reset}")
term.println("{white,red}White on red{reset}")
term.println("")

term.println("Clear screen to a color:")
term.sleep(0.5)
term.clrscr("blue")
term.println("{white}This screen was cleared to a blue background.{reset}")
term.sleep(1)
term.normvideo()
term.clrscr()

term.println("Cursor demo:")
term.gotoxy(1, 13)
term.println("{lightgreen}This line was written using gotoxy().{reset}")
term.gotoxy(1, 15)

term.println("")
term.println("Interactive demo:")
term.println("Hiding cursor for a moment...")
term.cursor(false)
term.sleep(0.7)
term.cursor(true)
term.println("Cursor visible again.")

local ok, err = term.raw(true)
if not ok then
    term.println("{red}term.raw(true) failed:{reset} " .. tostring(err))
else
    local width, height = term.size()
    if width and height then
        term.println(string.format("Terminal size: %d x %d", width, height))
    else
        term.println("Terminal size: unavailable")
    end

    term.println("Press any key to continue...")
    term.cursor(false)
    local key = term.readkey()
    term.sleep(0.2)
    term.raw(false)
    term.cursor(true)

    -- The key itself is arbitrary input, so double any literal braces it
    -- contains before embedding it in a tag-parsed string (see "Escaping
    -- dynamic content" in docs/modules/term.md).
    local key_repr = string.format("%q", key or ""):gsub("[{}]", "%1%1")
    term.println("Key pressed: {lightgreen}" .. key_repr .. "{reset}")
end
