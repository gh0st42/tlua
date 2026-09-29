package picolua

import (
	"io"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"tlua/internal/pico"
)

// fixture is a Lua program with the API installed, ready to be ticked.
type fixture struct {
	*Runtime
	t   *testing.T
	log *strings.Builder
}

// start loads a program onto a small screen, so that a test can write out what
// it expects to see.
func start(t *testing.T, w, h int, src string) *fixture {
	t.Helper()
	L := lua.NewState()
	t.Cleanup(L.Close)

	log := &strings.Builder{}
	rt := New(L, Options{Width: w, Height: h, Out: log, Seed: 1})

	fn, err := L.Load(strings.NewReader(src), "main.lua")
	if err != nil {
		t.Fatalf("loading the program: %v", err)
	}
	L.Push(fn)
	if err := L.PCall(0, lua.MultRet, nil); err != nil {
		t.Fatalf("running the program: %v", err)
	}
	return &fixture{Runtime: rt, t: t, log: log}
}

// tick runs one frame: input, _update, then _draw.
func (f *fixture) tick(frame pico.Frame) {
	f.t.Helper()
	if err := f.Tick(frame); err != nil {
		f.t.Fatalf("_update: %v", err)
	}
	if err := f.Draw(); err != nil {
		f.t.Fatalf("_draw: %v", err)
	}
}

// dump renders the screen one character per pixel, colour 0 as a dot.
func (f *fixture) dump() string {
	const digits = "0123456789abcdef"
	s := f.Screen()
	var b strings.Builder
	for y := 0; y < s.H; y++ {
		if y > 0 {
			b.WriteByte('\n')
		}
		for x := 0; x < s.W; x++ {
			switch col := s.Pix[y*s.W+x]; {
			case col == 0:
				b.WriteByte('.')
			case int(col) < len(digits):
				b.WriteByte(digits[col])
			default:
				b.WriteByte('#')
			}
		}
	}
	return b.String()
}

func (f *fixture) want(picture string) {
	f.t.Helper()
	lines := strings.Split(strings.Trim(picture, "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	if got, want := f.dump(), strings.Join(lines, "\n"); got != want {
		f.t.Errorf("screen:\n%s\nwant:\n%s", got, want)
	}
}

// eval runs an expression and reports what it gave back.
func (f *fixture) eval(expr string) []lua.LValue {
	f.t.Helper()
	L := f.L
	top := L.GetTop()
	if err := L.DoString("return " + expr); err != nil {
		f.t.Fatalf("%s: %v", expr, err)
	}
	out := make([]lua.LValue, 0, L.GetTop()-top)
	for i := top + 1; i <= L.GetTop(); i++ {
		out = append(out, L.Get(i))
	}
	L.SetTop(top)
	return out
}

// str evaluates an expression and reports its results as text, which keeps a
// table of cases short.
func (f *fixture) str(expr string) string {
	f.t.Helper()
	parts := []string{}
	for _, v := range f.eval(expr) {
		parts = append(parts, tostr(v))
	}
	return strings.Join(parts, ",")
}

func TestDrawingCallsLandOnTheScreen(t *testing.T) {
	f := start(t, 6, 4, `
		function _draw()
			cls(1)
			rectfill(1, 1, 4, 2, 7)
			pset(0, 0, 8)
		end`)
	f.tick(pico.Frame{})
	f.want(`
		811111
		177771
		177771
		111111`)
}

func TestTopLevelDrawingIsKeptWhenThereIsNoDrawCallback(t *testing.T) {
	// The shortest program there is: draw something and let it sit there.
	f := start(t, 3, 2, `cls(5)`)
	if f.Has(CallbackDraw) || f.Has(CallbackUpdate) {
		t.Fatal("this program has no callbacks")
	}
	f.tick(pico.Frame{})
	f.want(`
		555
		555`)
}

func TestCallbacksRunWhenTheyShould(t *testing.T) {
	f := start(t, 2, 1, `
		order = {}
		function _init() order[#order+1] = "init" end
		function _update() order[#order+1] = "update" end
		function _draw() order[#order+1] = "draw" end`)

	if err := f.Init(); err != nil {
		t.Fatal(err)
	}
	f.tick(pico.Frame{})
	f.tick(pico.Frame{})

	if got := f.str(`table.concat(order, " ")`); got != "init update draw update draw" {
		t.Errorf("callbacks ran %q", got)
	}
	if got := f.Frame(); got != 2 {
		t.Errorf("frame count is %d, want 2", got)
	}
}

func TestAMissingCallbackIsNotAnError(t *testing.T) {
	f := start(t, 2, 1, `x = 1`)
	if err := f.Init(); err != nil {
		t.Errorf("Init: %v", err)
	}
	if err := f.Tick(pico.Frame{}); err != nil {
		t.Errorf("Tick: %v", err)
	}
	if err := f.Draw(); err != nil {
		t.Errorf("Draw: %v", err)
	}
}

func TestAnErrorInUpdateIsReportedWithWhereItHappened(t *testing.T) {
	f := start(t, 2, 1, "\n\nfunction _update()\n  error(\"boom\")\nend")
	err := f.Tick(pico.Frame{})
	if err == nil {
		t.Fatal("the error should have come back")
	}
	// The message has to name the file and the line, both because that is what
	// a person needs and because the editor finds the line by reading it.
	if got := err.Error(); !strings.Contains(got, "main.lua:4:") || !strings.Contains(got, "boom") {
		t.Errorf("error is %q, want it to say main.lua:4 and boom", got)
	}
}

func TestSomethingThatIsNotAFunctionIsAnErrorRatherThanACrash(t *testing.T) {
	f := start(t, 2, 1, `_draw = 3`)
	if err := f.Draw(); err == nil {
		t.Error("calling a number should be an error")
	}
}

func TestAColourArgumentBecomesThePen(t *testing.T) {
	f := start(t, 4, 1, `
		rectfill(0, 0, 0, 0, 9)  -- names a colour...
		rectfill(2, 0, 2, 0)     -- ...and the next call inherits it`)
	f.want(`9.9.`)
	if got := f.str(`color(3)`); got != "9" {
		t.Errorf("color() reported %q as the previous colour, want 9", got)
	}
}

func TestCameraClipAndPaletteAreDrivenFromLua(t *testing.T) {
	f := start(t, 6, 3, `
		camera(-2, 0)
		pal(7, 3)
		rectfill(0, 0, 1, 2, 7)  -- drawn two to the right, in colour 3
		camera()
		clip(0, 0, 1, 1)
		rectfill(0, 0, 5, 2, 8)  -- only the top left pixel survives
		clip()
	`)
	f.want(`
		8.33..
		..33..
		..33..`)
	if got := f.str(`select("#", clip())`); got != "4" {
		t.Errorf("clip() gave %s results, want 4", got)
	}
}

func TestFillPatternFromLua(t *testing.T) {
	f := start(t, 4, 2, `
		color(7, 2)
		fillp(0xa5a5)
		rectfill(0, 0, 3, 1)`)
	f.want(`
		2727
		7272`)
	if got := f.str(`fillp()`); got != "42405" { // 0xa5a5
		t.Errorf("fillp reported %q as the previous pattern", got)
	}
}

func TestPrintDrawsOnTheScreenAndPrinthOnTheTerminal(t *testing.T) {
	f := start(t, 8, 6, `
		print("1", 0, 0, 7)
		printh("to the terminal")`)
	if f.Screen().Get(1, 0) == 0 {
		t.Errorf("print drew nothing:\n%s", f.dump())
	}
	if got := f.log.String(); got != "to the terminal\n" {
		t.Errorf("printh wrote %q", got)
	}
}

func TestBarePrintsStackDownTheScreen(t *testing.T) {
	f := start(t, 8, 14, `
		cursor(0, 0)
		print("1")
		print("1")`)
	if f.Screen().Get(1, 0) == 0 || f.Screen().Get(1, pico.LineHeight) == 0 {
		t.Errorf("the second print did not follow the first:\n%s", f.dump())
	}
}

func TestPrintReportsWhereTheTextEnded(t *testing.T) {
	f := start(t, 40, 10, "")
	if got := f.str(`print("abc", 10, 0, 7)`); got != "22" { // 10 + 3 characters
		t.Errorf("print reported %q, want 22", got)
	}
	if got := f.str(`textwidth("abc")`); got != "12" {
		t.Errorf("textwidth is %q, want 12", got)
	}
}

func TestScreenReportsItsSize(t *testing.T) {
	f := start(t, 64, 32, "")
	if got := f.str(`screen()`); got != "64,32" {
		t.Errorf("screen() is %q", got)
	}
}

func TestExitAsksToStop(t *testing.T) {
	f := start(t, 2, 1, `function _update() exit(3) end`)
	if quit, _ := f.Quitting(); quit {
		t.Fatal("nothing has asked to stop yet")
	}
	f.tick(pico.Frame{})
	quit, code := f.Quitting()
	if !quit || code != 3 {
		t.Errorf("Quitting() = %v, %d; want true, 3", quit, code)
	}
}

func TestWindowRequestsReachTheHost(t *testing.T) {
	f := start(t, 8, 8, "")
	if _, changed := f.Window(); changed {
		t.Error("nothing has been asked for yet")
	}

	f.eval(`window{title = "Snake", scale = 3, fullscreen = true}`)
	win, changed := f.Window()
	if !changed {
		t.Fatal("the host was not told")
	}
	if win.Title != "Snake" || win.Scale != 3 || !win.Fullscreen {
		t.Errorf("window is %+v", win)
	}
	if _, changed := f.Window(); changed {
		t.Error("the same request was reported twice")
	}
}

func TestWindowCanChangeTheResolution(t *testing.T) {
	f := start(t, 8, 8, `window{width = 4, height = 2}`)
	if got := f.str(`screen()`); got != "4,2" {
		t.Errorf("screen() is %q, want 4,2", got)
	}
	if w, h := f.Screen().W, f.Screen().H; w != 4 || h != 2 {
		t.Errorf("the framebuffer is %dx%d", w, h)
	}
	f.eval(`cls(3)`)
	f.want(`
		3333
		3333`)
}

func TestARidiculousResolutionIsRefused(t *testing.T) {
	L := lua.NewState()
	defer L.Close()
	New(L, Options{Out: io.Discard})
	if err := L.DoString(`window{width = 100000, height = 100000}`); err == nil {
		t.Error("a screen of ten thousand million pixels should be refused")
	}
}

func TestWildNumbersAreClampedRatherThanBreakingAnything(t *testing.T) {
	// A program working out coordinates can easily produce something silly; it
	// should draw nothing and carry on, not hang or crash.
	f := start(t, 8, 8, `
		local huge = 1e18
		line(-huge, -huge, huge, huge, 7)
		rectfill(huge, huge, huge, huge, 7)
		circfill(0/0, 0/0, 0/0, 7)
		spr(surface(2, 2), huge, -huge)
		sspr(surface(2, 2), 0, 0, 2, 2, 0, 0, huge, huge)
		pset(huge, 0, 7)
		print("x", -huge, huge, 7)`)
	// The diagonal is the one of those that should be visible.
	if f.Screen().Get(4, 4) == 0 {
		t.Errorf("the long diagonal should still cross the screen:\n%s", f.dump())
	}
}

func TestSurfacesAreDrawnAndCanBeDrawnOn(t *testing.T) {
	f := start(t, 6, 3, `
		local stamp = surface(2, 2)
		target(stamp)
		rectfill(0, 0, 1, 1, 9)
		target()
		for i = 0, 2 do spr(stamp, i * 2, 1) end`)
	f.want(`
		......
		999999
		999999`)
}

func TestTargetReportsWhatItReplaced(t *testing.T) {
	// Handing back the previous target is what lets a routine draw somewhere
	// else and put things back without knowing where it was called from.
	f := start(t, 4, 2, `
		a, b = surface(2, 2), surface(2, 2)
		first = target(a)
		second = target(b)
		third = target()`)
	if got := f.str(`first == nil, second == a, third == b`); got != "true,true,true" {
		t.Errorf("the chain of targets reported %q, want true,true,true", got)
	}
}

func TestSpriteArtBecomesASurface(t *testing.T) {
	f := start(t, 3, 3, `
		local s = sprite[[
			.7.
			777
			.7.
		]]
		spr(s, 0, 0)
		size = { s:width(), s:height() }`)
	f.want(`
		.7.
		777
		.7.`)
	if got := f.str(`size[1], size[2]`); got != "3,3" {
		t.Errorf("the sprite is %s, want 3,3", got)
	}
}

func TestBadSpriteArtIsAnError(t *testing.T) {
	f := start(t, 2, 2, "")
	if err := f.L.DoString(`sprite("zz")`); err == nil {
		t.Error("art that is not colours should be refused")
	}
}

func TestSurfaceMethods(t *testing.T) {
	f := start(t, 4, 2, `
		s = surface(2, 2)
		s:fill(4)
		s:set(0, 0, 9)
		copy = s:clone()
		copy:set(1, 1, 2)`)
	cases := []struct{ expr, want string }{
		{`s:get(0, 0)`, "9"},
		{`s:get(1, 1)`, "4"},
		{`s:get(9, 9)`, "0"},
		{`s:size()`, "2,2"},
		{`copy:get(1, 1)`, "2"},
		{`s:get(1, 1)`, "4"}, // the clone has its own pixels
		{`tostring(s)`, "surface 2x2"},
	}
	for _, c := range cases {
		expr, want := c.expr, c.want
		if got := f.str(expr); got != want {
			t.Errorf("%s = %q, want %q", expr, got, want)
		}
	}
}

func TestPassingSomethingThatIsNotASurfaceIsAnError(t *testing.T) {
	f := start(t, 2, 2, "")
	for _, expr := range []string{`spr(1, 0, 0)`, `spr("x", 0, 0)`, `target(7)`, `map({}, 3)`} {
		if err := f.L.DoString(expr); err == nil {
			t.Errorf("%s should be refused", expr)
		} else if !strings.Contains(err.Error(), "surface") {
			t.Errorf("%s said %q; it should mention a surface", expr, err)
		}
	}
}

func TestMapDrawsTilesWrittenEitherWay(t *testing.T) {
	// A sheet of four tiles, each two pixels square.
	src := `
		sheet = sprite[[
			1122
			1122
			3344
			3344
		]]`

	f := start(t, 8, 4, src+`
		map({"12", "34"}, sheet, 0, 0, 2, 2)`)
	f.want(`
		1122....
		1122....
		3344....
		3344....`)

	f = start(t, 8, 4, src+`
		map({{1, 0}, {0, 4}}, sheet, 0, 0, 2, 2)`)
	f.want(`
		11......
		11......
		..44....
		..44....`)
}

func TestMapCanBeOffsetAndClipped(t *testing.T) {
	f := start(t, 4, 2, `
		local sheet = sprite[[
			11
			11
		]]
		map({"1"}, sheet, 2, 0, 2, 2)`)
	f.want(`
		..11
		..11`)
}
