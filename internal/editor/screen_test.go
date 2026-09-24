package editor

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// dump renders the simulated screen as text, one string per row.
func dump(screen tcell.SimulationScreen) []string {
	cells, w, h := screen.GetContents()
	rows := make([]string, 0, h)
	for y := 0; y < h; y++ {
		var sb strings.Builder
		for x := 0; x < w; x++ {
			runes := cells[y*w+x].Runes
			if len(runes) == 0 || runes[0] == 0 {
				sb.WriteRune(' ')
				continue
			}
			sb.WriteRune(runes[0])
		}
		rows = append(rows, strings.TrimRight(sb.String(), " "))
	}
	return rows
}

// colorAt reports the foreground and background drawn at one cell.
func colorAt(screen tcell.SimulationScreen, x, y int) (tcell.Color, tcell.Color) {
	cells, w, _ := screen.GetContents()
	style := cells[y*w+x].Style
	fg, bg, _ := style.Decompose()
	return fg, bg
}

func TestScreenLayout(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t,
		write(t, filepath.Join(dir, "main.lua"), "print('hello')\nlocal x = 1\n"),
		write(t, filepath.Join(dir, "lib.lua"), "return 1\n"))

	redraw(t, e)
	rows := dump(screen)
	t.Logf("screen:\n%s", strings.Join(rows, "\n"))

	if !strings.Contains(rows[0], "File") || !strings.Contains(rows[0], "Search") ||
		!strings.Contains(rows[0], "Help") {
		t.Errorf("menu bar row = %q", rows[0])
	}
	// The buffer bar lists both files and marks the primary one with ».
	if !strings.Contains(rows[1], "1:»main.lua") || !strings.Contains(rows[1], "2:lib.lua") {
		t.Errorf("buffer bar = %q", rows[1])
	}
	// The active window is framed and titled.
	if !strings.Contains(rows[2], "main.lua") {
		t.Errorf("window title row = %q", rows[2])
	}
	if !strings.Contains(rows[3], "print('hello')") {
		t.Errorf("first text row = %q", rows[3])
	}
	// The status line greets on startup, then falls back to the key hints once
	// the user touches the keyboard.
	last := rows[len(rows)-1]
	if !strings.Contains(last, "F5 runs the file marked") || !strings.Contains(last, "L1 C1") {
		t.Errorf("status bar = %q", last)
	}
	press(screen, tcell.KeyEscape, 0, tcell.ModNone)
	waitFor(t, e, "the greeting to clear", func() bool { return e.status == "" })
	redraw(t, e)
	last = dump(screen)[len(rows)-1]
	if !strings.Contains(last, "F5 Run") || !strings.Contains(last, "F10 Menu") {
		t.Errorf("key hints = %q", last)
	}

	// EGA colours: grey bars, blue desktop, red hotkey letters.
	if fg, bg := colorAt(screen, 1, 0); fg != egaRed || bg != egaLightGray {
		t.Errorf("menu hotkey is %v on %v, want red on light grey", fg, bg)
	}
	// Row 3 is "print('hello')": a builtin, then a string, on the blue desktop.
	if fg, bg := colorAt(screen, 1, 3); fg != egaLightGreen || bg != egaBlue {
		t.Errorf("print is %v on %v, want light green on blue", fg, bg)
	}
	if fg, _ := colorAt(screen, 7, 3); fg != egaYellow {
		t.Errorf("the string literal is %v, want yellow", fg)
	}
	// Row 4 is "local x = 1": keyword, name, number.
	if fg, _ := colorAt(screen, 1, 4); fg != egaWhite {
		t.Errorf("the keyword local is %v, want white", fg)
	}
	if fg, _ := colorAt(screen, 7, 4); fg != egaLightGray {
		t.Errorf("the identifier x is %v, want light grey", fg)
	}
	if fg, _ := colorAt(screen, 11, 4); fg != egaLightCyan {
		t.Errorf("the number is %v, want light cyan", fg)
	}
	if _, bg := colorAt(screen, 1, len(rows)-1); bg != egaLightGray {
		t.Errorf("status bar background is %v, want light grey", bg)
	}
}

func TestMenuDropdownRendersUnderItsTitle(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "-- main\n"))

	run := menuIndex(t, e, "Run")
	press(screen, tcell.KeyRune, 'r', tcell.ModAlt) // the Run menu
	waitFor(t, e, "the Run menu", func() bool { return e.openMenu == run })
	redraw(t, e)

	rows := dump(screen)
	t.Logf("screen:\n%s", strings.Join(rows, "\n"))

	joined := strings.Join(rows, "\n")
	for _, want := range []string{"Run", "F5", "Check syntax", "F9", "Set as primary file"} {
		if !strings.Contains(joined, want) {
			t.Errorf("dropdown is missing %q", want)
		}
	}
	// It hangs below the Run title rather than at the left edge.
	if col := strings.Index(rows[3], "Run"); col < 10 {
		t.Errorf("dropdown starts at column %d of %q", col, rows[3])
	}
}

// colorMap renders one row as a letter per cell, so a test can show what the
// highlighter actually painted.
func colorMap(screen tcell.SimulationScreen, row, fromX int) string {
	letters := map[tcell.Color]byte{
		egaLightGray: '.', egaWhite: 'K', egaLightGreen: 'B', egaYellow: 'S',
		egaLightCyan: 'N', egaDarkGray: 'C',
	}
	cells, w, _ := screen.GetContents()
	out := make([]byte, 0, w)
	for x := fromX; x < w; x++ {
		fg, _, _ := cells[row*w+x].Style.Decompose()
		letter, ok := letters[fg]
		if !ok {
			letter = '?'
		}
		out = append(out, letter)
	}
	return strings.TrimRight(string(out), ".")
}

func TestSyntaxColoursOnScreen(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), strings.Join([]string{
		`local Counter = {}          -- a comment`,
		`function Counter.new(start)`,
		`  return { n = start or 0x10 }`,
		`end`,
		`print("count: " .. 1.5e2)`,
		`local text = [[a long`,
		`string over two lines]]`,
	}, "\n")))
	redraw(t, e)

	// Skip the window frame: only the text itself is interesting.
	textX, textY, _, _ := e.buffers[0].area.GetInnerRect()

	rows := dump(screen)
	for i := 0; i < 7; i++ {
		t.Logf("%-44s %s", rows[textY+i], colorMap(screen, textY+i, textX))
	}

	// line 1: "local" a keyword, the trailing comment grey.
	if got := colorMap(screen, textY, textX); !strings.HasPrefix(got, "KKKKK") ||
		!strings.Contains(got, "CCCCCCCCCCCC") {
		t.Errorf("row 1 colours: %s", got)
	}
	// line 3: a hex number.
	if got := colorMap(screen, textY+2, textX); !strings.Contains(got, "NNNN") {
		t.Errorf("row 3 colours: %s", got)
	}
	// line 5: print is a builtin, the string yellow, the float cyan.
	if got := colorMap(screen, textY+4, textX); !strings.HasPrefix(got, "BBBBB") ||
		!strings.Contains(got, "SSSS") || !strings.Contains(got, "NNNNN") {
		t.Errorf("row 5 colours: %s", got)
	}
	// The long string carries onto the next line.
	if got := colorMap(screen, textY+6, textX); !strings.HasPrefix(got, "SSSSSS") {
		t.Errorf("continuation line colours: %s", got)
	}
}

func TestOutlineDialogOnScreen(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), strings.Join([]string{
		"local M = {}",
		"",
		"function M.open(path)",
		"end",
		"",
		"local function parse(text)",
		"end",
		"",
		"function M:close()",
		"end",
	}, "\n")))

	press(screen, tcell.KeyF2, 0, tcell.ModAlt)
	waitFor(t, e, "the function list", func() bool { return e.modals == 1 })
	redraw(t, e)

	rows := dump(screen)
	t.Logf("screen:\n%s", strings.Join(rows, "\n"))
	joined := strings.Join(rows, "\n")
	// The file is the root entry, the end of the file the last, the names carry
	// no kind word, and a legend explains the colours.
	for _, want := range []string{
		"Functions in main.lua", "1  main.lua", "3  M.open", "6  parse", "9  M:close",
		"(end of file)", "global  local  nested",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the function list is missing %q", want)
		}
	}
	if strings.Contains(joined, "method   M:close") || strings.Contains(joined, "local    parse") {
		t.Error("the kind column is back")
	}
}

// panelBackgrounds reports the background colours filling a primitive's rect.
func panelBackgrounds(screen tcell.SimulationScreen, p tview.Primitive) map[tcell.Color]int {
	x, y, width, height := p.GetRect()
	cells, screenWidth, screenHeight := screen.GetContents()
	found := map[tcell.Color]int{}
	for row := y; row < y+height && row < screenHeight; row++ {
		for col := x; col < x+width && col < screenWidth; col++ {
			_, bg, _ := cells[row*screenWidth+col].Style.Decompose()
			found[bg]++
		}
	}
	return found
}

// TestPanelsAreFilledWithTheirOwnColour is the guard for a bug that looked like
// this: tview builds widget text styles from the theme's primitive background,
// which here is the blue desktop, and prints without keeping what is already
// underneath. Setting only a foreground colour left every label sitting on a
// blue stripe, so a grey menu panel was grey except behind its text.
func TestPanelsAreFilledWithTheirOwnColour(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"),
		"local M = {}\nfunction M.run()\nend\n"))

	panels := []struct {
		name string
		open func()
	}{
		{"the File menu", func() { e.openMenuAt(menuIndexOf(e, "File")) }},
		{"the Search menu", func() { e.openMenuAt(menuIndexOf(e, "Search")) }},
		{"the function list", e.outlineDialog},
		{"the buffer list", e.bufferListDialog},
		{"the Find dialog", e.findDialog},
		{"the Replace dialog", e.replaceDialog},
		{"the Open dialog", e.openDialog},
		{"the Go to line dialog", e.gotoDialog},
		{"the help", e.showHelp},
		{"a message", func() { e.message("Message", "Something happened.") }},
		{"a confirmation", func() { e.confirm("Confirm", "Really?", func() {}) }},
	}

	for _, panel := range panels {
		onEditor(t, e, panel.open)
		redraw(t, e)

		var backgrounds map[tcell.Color]int
		onEditor(t, e, func() {
			target := tview.Primitive(nil)
			if e.openMenu >= 0 {
				target = e.menuList
			} else if n := len(e.modalStack); n > 0 {
				target = e.modalStack[n-1]
			}
			if target == nil {
				t.Errorf("%s did not open", panel.name)
				return
			}
			backgrounds = panelBackgrounds(screen, target)
		})

		if n := backgrounds[egaBlue]; n > 0 {
			t.Errorf("%s has %d cells of desktop blue in it", panel.name, n)
		}
		if backgrounds[egaLightGray] == 0 {
			t.Errorf("%s is not grey at all: %v", panel.name, backgrounds)
		}

		onEditor(t, e, func() {
			if e.openMenu >= 0 {
				e.closeMenu()
				return
			}
			for _, page := range []string{"outline", "buffers", "find", "replace", "open",
				"prompt", "help", "message", "confirm"} {
				if e.pages.HasPage(page) {
					e.closeModal(page)
				}
			}
		})
	}
}

// menuIndexOf is the non-test-helper twin of menuIndex, for use inside a
// callback already running on the editor's goroutine.
func menuIndexOf(e *Editor, title string) int {
	for i, m := range e.menus {
		if m.title == title {
			return i
		}
	}
	return -1
}

func TestFindDialogOnScreen(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"),
		"local total = 0\nfor i = 1, 10 do total = total + i end\nprint(total)\n"))

	press(screen, tcell.KeyCtrlF, 0, tcell.ModNone)
	waitFor(t, e, "the Find dialog", func() bool { return e.modals == 1 })
	typeText(screen, "total")
	// Wait for the typing to land in the field before drawing: injected keys
	// and queued redraws arrive by different routes.
	waitFor(t, e, "the search term to reach the field", func() bool {
		form, ok := e.modalStack[0].(*tview.Form)
		return ok && form.GetFormItem(0).(*tview.InputField).GetText() == "total"
	})
	redraw(t, e)

	rows := dump(screen)
	t.Logf("screen:\n%s", strings.Join(rows, "\n"))

	// The term has to be visible in the dialog's own field, not merely
	// somewhere on screen: the buffer behind it says "total" too.
	fieldRow := ""
	for _, row := range rows {
		if strings.Contains(row, "Find:") {
			fieldRow = row
		}
	}
	if !strings.Contains(fieldRow, "total") {
		t.Errorf("the field row reads %q", fieldRow)
	}
	joined := strings.Join(rows, "\n")
	for _, want := range []string{"Case sensitive", "Cancel"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the Find dialog is missing %q", want)
		}
	}
}

func TestSearchMenuOnScreen(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "-- main\n"))

	search := menuIndex(t, e, "Search")
	press(screen, tcell.KeyRune, 's', tcell.ModAlt)
	waitFor(t, e, "the Search menu", func() bool { return e.openMenu == search })
	redraw(t, e)

	rows := dump(screen)
	t.Logf("screen:\n%s", strings.Join(rows, "\n"))
	joined := strings.Join(rows, "\n")
	for _, want := range []string{"Find...", "Ctrl-F", "Find next", "F7", "Replace...", "Ctrl-R", "Go to line..."} {
		if !strings.Contains(joined, want) {
			t.Errorf("the Search menu is missing %q", want)
		}
	}
}

// colorOfWordIn reports the foreground colour a word is drawn in, looking only
// inside one primitive: the text behind a dialog says the same words.
func colorOfWordIn(t *testing.T, screen tcell.SimulationScreen, p tview.Primitive, word string) tcell.Color {
	t.Helper()
	x, y, width, height := p.GetRect()
	rows := dump(screen)
	for row := y; row < y+height && row < len(rows); row++ {
		line := rows[row]
		if len(line) < x {
			continue
		}
		end := x + width
		if end > len(line) {
			end = len(line)
		}
		if col := strings.Index(line[x:end], word); col >= 0 {
			fg, _ := colorAt(screen, x+col, row)
			return fg
		}
	}
	t.Fatalf("%q is not inside the panel", word)
	return tcell.ColorDefault
}

// TestFunctionListColoursByKind checks the point of dropping the kind column:
// the colour says whether a function is global, local or nested.
func TestFunctionListColoursByKind(t *testing.T) {
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), strings.Join([]string{
		"local M = {}",                               // 1
		"function M.global_one()",                    // 2
		"  local_callback = function() return 1 end", // 3 nested
		"end",                        // 4
		"local function local_one()", // 5
		"end",                        // 6
	}, "\n")))

	press(screen, tcell.KeyF2, 0, tcell.ModAlt)
	waitFor(t, e, "the function list", func() bool { return e.modals == 1 })
	redraw(t, e)
	t.Logf("screen:\n%s", strings.Join(dump(screen), "\n"))

	cases := []struct {
		word string
		want tcell.Color
		why  string
	}{
		{"main.lua", egaBlack, "the file, as the root row"},
		{"M.global_one", egaBlack, "a global"},
		{"local_callback", egaRed, "nested inside another function"},
		{"local_one", egaBlue, "a local"},
		{"(end of file)", egaDarkGray, "the end row"},
	}
	panel := e.modalStack[0]
	for _, c := range cases {
		if got := colorOfWordIn(t, screen, panel, c.word); got != c.want {
			t.Errorf("%s (%s) is %v, want %v", c.word, c.why, got, c.want)
		}
	}

	// The legend spells the code out in the same colours.
	if got := colorOfWordIn(t, screen, panel, "nested"); got != egaRed {
		t.Errorf("the legend's \"nested\" is %v", got)
	}
}

func TestCompletionPopupOnScreen(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"),
		"local t = {}\nlocal x = pri\nprint(x)\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 1, 13)
		b.area.Select(offset, offset)
	})
	press(screen, tcell.KeyCtrlSpace, 0, tcell.ModNone)
	waitFor(t, e, "the completion list", func() bool { return e.modals == 1 })
	redraw(t, e)

	rows := dump(screen)
	t.Logf("screen:\n%s", strings.Join(rows, "\n"))
	joined := strings.Join(rows, "\n")
	for _, want := range []string{"completions", "print", "function", "table_insert", "for_loop", "snippet"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the popup is missing %q", want)
		}
	}
	// It hangs under the cursor rather than in the middle of the screen.
	x, y, _, _ := e.modalStack[0].GetRect()
	cursor := e.cursorScreenPosition(e.buffers[0])
	if y != cursor.row+1 {
		t.Errorf("the popup is on row %d, want just under the cursor at %d", y, cursor.row+1)
	}
	if x > cursor.col {
		t.Errorf("the popup starts at column %d, right of the cursor at %d", x, cursor.col)
	}
	// Its panel is grey, like every other one.
	if n := panelBackgrounds(screen, e.modalStack[0])[egaBlue]; n > 0 {
		t.Errorf("the popup has %d cells of desktop blue", n)
	}
}

func TestHoverBoxOnScreen(t *testing.T) {
	withLanguageServer(t)
	dir := t.TempDir()
	e, screen := start(t, write(t, filepath.Join(dir, "main.lua"), "print(1)\n"))
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanHover() })

	press(screen, tcell.KeyF11, 0, tcell.ModNone)
	waitFor(t, e, "the help box", func() bool { return e.modals == 1 })
	redraw(t, e)

	rows := dump(screen)
	t.Logf("screen:\n%s", strings.Join(rows, "\n"))
	if !strings.Contains(strings.Join(rows, "\n"), "function print(...)") {
		t.Error("the help box does not show what the server said")
	}
	if n := panelBackgrounds(screen, e.modalStack[0])[egaBlue]; n > 0 {
		t.Errorf("the help box has %d cells of desktop blue", n)
	}
}
