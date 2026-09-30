package picolua

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
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

// errNotThere is what a test's file reader says about a file it does not have.
var errNotThere = errors.New("no such file")

// start loads a program onto a small screen, so that a test can write out what
// it expects to see.
func start(t *testing.T, w, h int, src string) *fixture {
	t.Helper()
	return startWith(t, Options{Width: w, Height: h, Seed: 1}, src)
}

// startWith is start for a test that cares how the runtime was set up.
func startWith(t *testing.T, opts Options, src string) *fixture {
	t.Helper()
	L := lua.NewState()
	t.Cleanup(L.Close)

	log := &strings.Builder{}
	if opts.Out == nil {
		opts.Out = log
	}
	rt := New(L, opts)

	fn, err := L.Load(strings.NewReader(src), "main.lua")
	if err != nil {
		t.Fatalf("loading the program: %v", err)
	}
	if err := rt.Start(fn, nil); err != nil {
		t.Fatalf("running the program: %v", err)
	}
	return &fixture{Runtime: rt, t: t, log: log}
}

// tick runs one frame of the program: input, and then everything it does with
// it, which is _update and _draw unless it has taken the loop over itself.
func (f *fixture) tick(frame pico.Frame) {
	f.t.Helper()
	if err := f.Tick(frame); err != nil {
		f.t.Fatalf("tick: %v", err)
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

	// _init has already run: starting the program is what runs it.
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
	for i := 0; i < 3; i++ {
		if err := f.Tick(pico.Frame{}); err != nil {
			t.Errorf("tick %d: %v", i, err)
		}
	}
	if !f.Running() {
		t.Error("a program with no callbacks is still running, doing nothing")
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
	if err := f.Tick(pico.Frame{}); err == nil {
		t.Error("calling a number should be an error")
	}
	if f.Running() {
		t.Error("a program that failed is not still running")
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
	for _, expr := range []string{`spr("x", 0, 0)`, `target(7)`, `map({}, 3)`} {
		if err := f.L.DoString(expr); err == nil {
			t.Errorf("%s should be refused", expr)
		} else if !strings.Contains(err.Error(), "surface") {
			t.Errorf("%s said %q; it should mention a surface", expr, err)
		}
	}

	// A number means a sprite of the current sheet, so asking for one before
	// there is a sheet says that instead, and says what to do about it.
	err := f.L.DoString(`spr(1, 0, 0)`)
	if err == nil {
		t.Fatal("there is no current sheet to take sprite 1 from")
	}
	for _, want := range []string{"current sheet", "sheet()"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}
}

func TestMapDrawsSpritesByNumber(t *testing.T) {
	// A sheet of four cells, each two pixels square, with the first left
	// blank — which is what makes 0 mean empty in a level.
	src := `
		tiles = sprite([[
			..11
			..11
			2233
			2233
		]], 2, 2)`

	f := start(t, 8, 4, src+`
		map({"01", "23"}, tiles, 0, 0)`)
	f.want(`
		..11....
		..11....
		2233....
		2233....`)

	// The same written as numbers, and sprite 0 left out of it.
	f = start(t, 8, 4, src+`
		map({{1, 0}, {0, 3}}, tiles, 0, 0)`)
	f.want(`
		11......
		11......
		..33....
		..33....`)
}

func TestMapTakesItsTileSizeFromTheSheet(t *testing.T) {
	f := start(t, 8, 4, `
		local sheet = sprite([[
			..11
			..11
			2233
			2233
		]], 2, 2)
		map({"1"}, sheet)`)
	f.want(`
		11......
		11......
		........
		........`)
}

func TestMapLeavesSpriteZeroAloneUnlessAsked(t *testing.T) {
	src := `
		tiles = sprite([[
			7711
			7711
		]], 2, 2)`

	// Sprite 0 is there on the sheet, and still not drawn.
	f := start(t, 4, 2, src+`
		cls(3)
		map({"00"}, tiles)`)
	f.want(`
		3333
		3333`)

	// Unless the map is told to draw it.
	f = start(t, 4, 2, src+`
		cls(3)
		map({"00"}, tiles, 0, 0, 2, 2, true)`)
	f.want(`
		7777
		7777`)
}

func TestMapCanBeOffsetAndClipped(t *testing.T) {
	f := start(t, 4, 2, `
		local sheet = sprite([[
			..11
			..11
		]], 2, 2)
		map({"1"}, sheet, 2, 0, 2, 2)`)
	f.want(`
		..11
		..11`)
}

func TestRoundedRectanglesTakeAWidthAndAHeight(t *testing.T) {
	f := start(t, 8, 6, `
		rrectfill(1, 1, 6, 4, 1, 7)`)
	f.want(`
		........
		..7777..
		.777777.
		.777777.
		..7777..
		........`)

	f = start(t, 8, 6, `rrect(1, 1, 6, 4, 1, 8)`)
	f.want(`
		........
		..8888..
		.8....8.
		.8....8.
		..8888..
		........`)
}

func TestARoundedRectangleWithoutARadiusStillDrawsOne(t *testing.T) {
	// The radius may be left out, as a circle's may; the colour is then the
	// sixth argument, so a colour passed as the fifth is a radius.
	f := start(t, 12, 10, `
		color(7)
		rrectfill(0, 0, 12, 10)`)
	if f.Screen().Get(0, 0) != 0 {
		t.Errorf("the corner should have been rounded away:\n%s", f.dump())
	}
	if f.Screen().Get(6, 5) != 7 {
		t.Errorf("the middle should be filled:\n%s", f.dump())
	}
}

func TestThePaletteCanBeAskedAboutAndChanged(t *testing.T) {
	f := start(t, 4, 2, "")

	if got := f.str(`palette()`); got != "default,64" {
		t.Errorf("palette() = %q, want the default one", got)
	}
	if got := f.str(`palette("vga")`); got != "vga,256" {
		t.Errorf("palette(\"vga\") = %q", got)
	}
	if got := f.str(`rgb(1)`); got != "170" { // VGA blue, 0x0000aa
		t.Errorf("rgb(1) under VGA is %q, want 170", got)
	}
	if got := f.str(`palette("default")`); got != "default,64" {
		t.Errorf("going back gave %q", got)
	}

	// One entry at a time, which is what a fade is made of.
	if got := f.str(`palette(0)`); got != "0" {
		t.Errorf("colour 0 is %q, want black", got)
	}
	if got := f.str(`palette(0, 0xff8800)`); got != "0" {
		t.Errorf("setting an entry reported %q as its old value", got)
	}
	if got := f.str(`palette(0)`); got != "16746496" { // 0xff8800
		t.Errorf("colour 0 is now %q", got)
	}
}

func TestAPaletteCanBeGivenOutright(t *testing.T) {
	f := start(t, 2, 1, `
		name, size = palette{ 0x000000, 0xff0000, { 0, 255, 0 } }
		pset(0, 0, 1)
		pset(1, 0, 2)`)
	if got := f.str(`name, size`); got != "custom,3" {
		t.Errorf("palette(table) = %q, want custom,3", got)
	}
	if got := f.str(`rgb(1), rgb(2)`); got != "16711680,65280" {
		t.Errorf("the colours came out as %q", got)
	}
	// A palette of three wraps at three: colour 4 is colour 1.
	if got := f.str(`pget(0, 0), pget(1, 0)`); got != "1,2" {
		t.Errorf("pixels are %q", got)
	}
	f.eval(`pset(0, 0, 4)`)
	if got := f.str(`pget(0, 0)`); got != "1" {
		t.Errorf("colour 4 in a palette of three is %q, want 1", got)
	}
}

func TestAPaletteCanBeLoadedFromAFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dusk.gpl")
	if err := os.WriteFile(path, []byte("GIMP Palette\nName: Dusk\n0 0 0\n1 2 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := start(t, 2, 1, "")
	if got := f.str(fmt.Sprintf("palette(%q)", path)); got != "Dusk,2" {
		t.Errorf("loading a palette gave %q", got)
	}
	if got := f.str(`rgb(1)`); got != "66051" { // 0x010203
		t.Errorf("colour 1 is %q", got)
	}
}

func TestAPaletteThatIsNeitherIsAnError(t *testing.T) {
	f := start(t, 2, 1, "")
	err := f.L.DoString(`palette("wat")`)
	if err == nil {
		t.Fatal("an unknown palette should be refused")
	}
	// The message has to say what would have worked.
	for _, want := range []string{"wat", "default", "vga"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}
	if err := f.L.DoString(`palette{}`); err == nil {
		t.Error("an empty palette should be refused")
	}
}

func TestChangingThePaletteChangesThePictureNotThePixels(t *testing.T) {
	f := start(t, 1, 1, `
		pset(0, 0, 8)
		before = rgb(8)
		palette(8, 0x102030)
		after = rgb(8)`)
	if got := f.str(`pget(0, 0)`); got != "8" {
		t.Errorf("the pixel is %q; only what 8 looks like should have changed", got)
	}
	if got := f.str(`before == after`); got != "false" {
		t.Error("the colour should have changed")
	}
}

func TestVideoModes(t *testing.T) {
	f := start(t, 4, 2, "")
	// A size that is not one of the modes has no number.
	if got := f.str(`vid()`); got != "-1,4,2" {
		t.Errorf("vid() = %q", got)
	}

	cases := []struct{ call, want string }{
		{`vid(0)`, "480,270"}, // Picotron's numbering
		{`vid(1)`, "320,180"},
		{`vid(2)`, "240,180"},
		{`vid(3)`, "240,135"},
		{`vid(4)`, "160,90"},
		{`vid(13)`, "320,200"}, // what a VGA card called mode 13h
	}
	for _, c := range cases {
		if got := f.str(c.call); got != c.want {
			t.Errorf("%s = %q, want %q", c.call, got, c.want)
		}
		if got := f.str(`screen()`); got != c.want {
			t.Errorf("after %s the screen is %q, want %q", c.call, got, c.want)
		}
	}

	if got := f.str(`vid()`); got != "13,320,200" {
		t.Errorf("vid() = %q, want the mode it was put in", got)
	}
	if w, h := f.Screen().W, f.Screen().H; w != 320 || h != 200 {
		t.Errorf("the framebuffer is %dx%d", w, h)
	}

	// The host is told, so that the window can be refitted.
	if _, changed := f.Window(); !changed {
		t.Error("a change of resolution should reach the host")
	}
}

func TestAVideoModeThatIsNotOneIsAnError(t *testing.T) {
	f := start(t, 4, 2, "")
	err := f.L.DoString(`vid(7)`)
	if err == nil {
		t.Fatal("there is no mode 7")
	}
	if !strings.Contains(err.Error(), "480x270") {
		t.Errorf("the error should list the modes there are: %v", err)
	}
}

func TestDrawingSurvivesAChangeOfResolution(t *testing.T) {
	f := start(t, 4, 2, `
		vid(4)
		cls(3)
		rectfill(0, 0, 159, 89, 9)`)
	if got := f.Screen().Get(159, 89); got != 9 {
		t.Errorf("the far corner of the new screen is %d, want 9", got)
	}
}

func TestFilesAreReadThroughTheHost(t *testing.T) {
	// Everything a program loads goes through one door, so that a host which
	// keeps its files somewhere other than the disk — a game fused into one
	// executable — only has to answer at that door.
	asked := []string{}
	L := lua.NewState()
	defer L.Close()
	rt := New(L, Options{
		Width: 4, Height: 2, Out: io.Discard,
		ReadFile: func(name string) ([]byte, error) {
			asked = append(asked, name)
			switch name {
			case "level.txt":
				return []byte("1,2,3"), nil
			case "art.png":
				return smallPNG(t), nil
			}
			return nil, fmt.Errorf("no such thing as %s", name)
		},
	})
	_ = rt

	if err := L.DoString(`
		level = fetch("level.txt")
		art, arterr = loadpng("art.png")
		missing, err = fetch("nothing.txt")
	`); err != nil {
		t.Fatal(err)
	}

	if got := lua.LVAsString(L.GetGlobal("level")); got != "1,2,3" {
		t.Errorf("fetch gave %q", got)
	}
	if L.GetGlobal("art") == lua.LNil {
		t.Errorf("loadpng did not read through the host: %v", L.GetGlobal("arterr"))
	}
	if L.GetGlobal("missing") != lua.LNil {
		t.Error("a file that is not there should come back as nothing")
	}
	if got := lua.LVAsString(L.GetGlobal("err")); !strings.Contains(got, "nothing.txt") {
		t.Errorf("the error does not name the file: %q", got)
	}
	// A name that is a path is found first time; one that is nowhere is looked
	// for in each of the places a game keeps such things.
	if asked[0] != "level.txt" || asked[1] != "art.png" {
		t.Errorf("the first two were asked for as %v", asked[:2])
	}
	searched := strings.Join(asked[2:], " ")
	for _, want := range []string{"nothing.txt", "data/nothing.txt", "assets/nothing.txt"} {
		if !strings.Contains(searched, want) {
			t.Errorf("a missing file was not looked for as %q: %v", want, asked[2:])
		}
	}
}

func TestWithoutAHostFilesComeFromTheDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "greeting.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := start(t, 2, 1, "")
	if got := f.str(fmt.Sprintf("fetch(%q)", path)); got != "hello" {
		t.Errorf("fetch read %q", got)
	}
}

// smallPNG is two pixels of a known colour, for a test that needs a real image.
func smallPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	r, g, b := pico.Default.RGB(8)
	img.Set(0, 0, color.RGBA{r, g, b, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// soundFixture is a runtime whose sounds can be looked at rather than heard.
func soundFixture(t *testing.T, files map[string]string, src string) (*fixture, *silent) {
	t.Helper()
	heard := newSilent()
	f := startWith(t, Options{
		Width: 4, Height: 2, Seed: 1,
		Sound: heard,
		ReadFile: func(name string) ([]byte, error) {
			if body, ok := files[name]; ok {
				return []byte(body), nil
			}
			return nil, errNotThere
		},
	}, src)
	return f, heard
}

func TestSfxFindsASoundAndPlaysIt(t *testing.T) {
	f, heard := soundFixture(t, map[string]string{
		"sfx/jump.wav":        "RIFF jump",
		"assets/sfx/coin.ogg": "OggS coin",
		"blip.wav":            "RIFF blip",
	}, "")

	cases := []struct{ call, want string }{
		{`sfx("jump")`, "0"}, // found as sfx/jump.wav
		{`sfx("coin")`, "1"}, // found as assets/sfx/coin.ogg
		{`sfx("blip.wav")`, "2"},
		{`sfx("assets/sfx/coin.ogg")`, "3"}, // spelled out in full
	}
	for _, c := range cases {
		if got := f.str(c.call); got != c.want {
			t.Errorf("%s = %q, want channel %q", c.call, got, c.want)
		}
	}

	if heard.channels[0] != "sfx/jump.wav" {
		t.Errorf("channel 0 is playing %q", heard.channels[0])
	}
	if heard.channels[1] != "assets/sfx/coin.ogg" {
		t.Errorf("channel 1 is playing %q", heard.channels[1])
	}
}

func TestSfxCanBeToldWhichChannel(t *testing.T) {
	f, heard := soundFixture(t, map[string]string{"sfx/jump.wav": "RIFF"}, "")
	if got := f.str(`sfx("jump", 5)`); got != "5" {
		t.Errorf("sfx on channel 5 went to %q", got)
	}
	if heard.channels[5] == "" {
		t.Error("channel 5 is silent")
	}

	f.eval(`sfx(-1, 5)`)
	if heard.channels[5] != "" {
		t.Error("stopping channel 5 left something on it")
	}
}

func TestSfxStopsEverythingWhenAsked(t *testing.T) {
	f, heard := soundFixture(t, map[string]string{"sfx/a.wav": "RIFF"}, "")
	for i := 0; i < 3; i++ {
		f.eval(`sfx("a")`)
	}
	f.eval(`sfx(-1)`)
	for i, playing := range heard.channels {
		if playing != "" {
			t.Errorf("channel %d is still playing %q", i, playing)
		}
	}

	// false says the same thing, for anyone who finds -1 strange.
	f.eval(`sfx("a")`)
	f.eval(`sfx(false)`)
	if heard.channels[0] != "" {
		t.Error("sfx(false) should stop everything too")
	}
}

func TestASoundThatIsNowhereIsNotAnError(t *testing.T) {
	// A missing sound should not stop a game in its tracks; it comes back as
	// nothing, with a message saying where it was looked for.
	f, _ := soundFixture(t, nil, "")
	if got := f.str(`sfx("nope") == nil`); got != "true" {
		t.Errorf("a missing sound gave %q", got)
	}
	if got := f.str(`select(2, sfx("nope"))`); !strings.Contains(got, "sfx/nope.wav") {
		t.Errorf("the message does not say where it looked: %q", got)
	}
}

func TestMusicPlaysLoopsAndStops(t *testing.T) {
	f, heard := soundFixture(t, map[string]string{
		"music/theme.ogg": "OggS theme",
		"sfx/jingle.wav":  "RIFF jingle",
	}, "")

	if got := f.str(`music("theme")`); got != "music/theme.ogg" {
		t.Errorf("music started %q", got)
	}
	if heard.music != "music/theme.ogg" {
		t.Errorf("the music is %q", heard.music)
	}
	if got := f.str(`music()`); got != "music/theme.ogg" {
		t.Errorf("music() reported %q", got)
	}

	// Music is looked for among the sounds as well, since a small game keeps
	// everything in one place.
	if got := f.str(`music("jingle")`); got != "sfx/jingle.wav" {
		t.Errorf("music found %q", got)
	}

	f.eval(`music(-1)`)
	if heard.music != "" {
		t.Errorf("the music is still %q", heard.music)
	}
	if got := f.str(`music() == nil`); got != "true" {
		t.Errorf("with nothing playing, music() gave %q", got)
	}
}

func TestVolumeIsReadAndSet(t *testing.T) {
	f, heard := soundFixture(t, nil, "")
	if got := f.str(`volume()`); got != "1" {
		t.Errorf("the volume starts at %q", got)
	}
	if got := f.str(`volume(0.25)`); got != "1" {
		t.Errorf("setting the volume reported %q as what it was", got)
	}
	if heard.volume != 0.25 {
		t.Errorf("the volume is %v", heard.volume)
	}
	f.eval(`volume(9)`)
	if heard.volume != 1 {
		t.Errorf("a volume above one should be held to one, got %v", heard.volume)
	}
}

func TestAProgramWithNoSoundBehavesTheSame(t *testing.T) {
	// No host to play anything: a program still gets a channel back and can
	// ask what is playing, so its own logic does not have to care.
	f := startWith(t, Options{
		Width: 2, Height: 1,
		ReadFile: func(string) ([]byte, error) { return []byte("RIFF"), nil },
	}, "")
	if got := f.str(`sfx("jump")`); got != "0" {
		t.Errorf("sfx gave %q", got)
	}
	if got := f.str(`music("theme")`); got == "" {
		t.Error("music gave nothing back")
	}
}

// sheetSrc is four cells of two pixels square, the first left blank, which is
// the shape a level expects of a sheet.
const sheetSrc = `
	tiles = sprite([[
		..11
		..11
		2233
		2233
	]], 2, 2)`

func TestSprDrawsASpriteOfASheetByNumber(t *testing.T) {
	f := start(t, 8, 2, sheetSrc+`
		spr(tiles, 1, 0, 0)
		spr(tiles, 2, 2, 0)
		spr(tiles, 3, 4, 0)
		spr(tiles, 0, 6, 0)`)
	f.want(`
		112233..
		112233..`)
}

func TestSprDrawsAPictureAtAPlaceAsBefore(t *testing.T) {
	// A surface that was never cut into sprites is a picture, and is drawn
	// whole at the place it is given.
	f := start(t, 6, 2, `
		local logo = sprite[[
			1212
			1212
		]]
		spr(logo, 1, 0)`)
	f.want(`
		.1212.
		.1212.`)
}

func TestASpriteCanSpanSeveralCells(t *testing.T) {
	f := start(t, 4, 4, sheetSrc+`
		spr(tiles, 0, 0, 0, 2, 2)`)
	f.want(`
		..11
		..11
		2233
		2233`)
}

func TestASpriteThatIsNotOnTheSheetDrawsNothing(t *testing.T) {
	f := start(t, 4, 2, sheetSrc+`
		cls(5)
		spr(tiles, 9, 0, 0)
		spr(tiles, -1, 2, 0)`)
	f.want(`
		5555
		5555`)
}

func TestTheCurrentSheetIsPicotronsOwnCall(t *testing.T) {
	f := start(t, 8, 2, sheetSrc+`
		usesheet(tiles)
		spr(1, 0, 0)     -- Picotron's own spelling
		spr(3, 2, 0)`)
	f.want(`
		1133....
		1133....`)

	// It says which sheet that is, and hands back the one it replaced.
	if got := f.str(`usesheet() == tiles`); got != "true" {
		t.Errorf("usesheet() reported %q", got)
	}
}

func TestOnlyASheetCanBeTheCurrentOne(t *testing.T) {
	f := start(t, 4, 2, `picture = sprite[[11|11]]`)
	err := f.L.DoString(`usesheet(picture)`)
	if err == nil {
		t.Fatal("a picture is not a sheet")
	}
	if !strings.Contains(err.Error(), "grid") {
		t.Errorf("the error should say what is missing: %v", err)
	}
}

func TestSsprCanUseTheCurrentSheetToo(t *testing.T) {
	f := start(t, 4, 2, sheetSrc+`
		usesheet(tiles)
		sspr(2, 0, 2, 2, 0, 0)`)
	f.want(`
		11..
		11..`)

	// And says so when there is none.
	f = start(t, 4, 2, "")
	if err := f.L.DoString(`sspr(0, 0, 2, 2, 0, 0)`); err == nil {
		t.Error("there is no current sheet")
	}
}

func TestSgetAndSsetReadTheCurrentSheet(t *testing.T) {
	f := start(t, 4, 2, sheetSrc+`
		usesheet(tiles)
		was = sget(2, 0)
		sset(0, 0, 9)
		now = sget(0, 0)`)
	if got := f.str(`was, now`); got != "1,9" {
		t.Errorf("sget/sset gave %q, want 1,9", got)
	}

	// Writing to the sheet changes what is drawn from it.
	f.eval(`spr(0, 0, 0)`)
	if got := f.Screen().Get(0, 0); got != 9 {
		t.Errorf("the pixel drawn is %d, want the 9 that was written", got)
	}
}

func TestASurfaceSaysHowItIsCutUp(t *testing.T) {
	f := start(t, 4, 2, sheetSrc+`
		picture = sprite[[11|11]]`)

	if got := f.str(`tiles:grid()`); got != "2,2,4" {
		t.Errorf("the sheet's grid is %q, want 2,2,4", got)
	}
	if got := f.str(`picture:grid()`); got != "0,0,0" {
		t.Errorf("a picture's grid is %q", got)
	}

	// A grid can be put on afterwards, and the call hands the surface back.
	if got := f.str(`picture:grid(1, 1) == picture`); got != "true" {
		t.Errorf("grid(w, h) gave back %q", got)
	}
	if got := f.str(`picture:grid()`); got != "1,1,4" {
		t.Errorf("after cutting, the grid is %q", got)
	}
	// One number means square cells.
	f.eval(`picture:grid(2)`)
	if got := f.str(`picture:grid()`); got != "2,2,1" {
		t.Errorf("a square grid is %q", got)
	}
}

func TestASpriteCanBeTakenOffTheSheet(t *testing.T) {
	f := start(t, 4, 2, sheetSrc+`
		one = tiles:sprite(1)
		big = tiles:sprite(0, 2, 2)
		none = tiles:sprite(9)`)

	if got := f.str(`one:size()`); got != "2,2" {
		t.Errorf("a sprite taken off the sheet is %q", got)
	}
	if got := f.str(`one:get(0, 0)`); got != "1" {
		t.Errorf("its first pixel is %q", got)
	}
	if got := f.str(`big:size()`); got != "4,4" {
		t.Errorf("a sprite of four cells is %q", got)
	}
	if got := f.str(`none == nil`); got != "true" {
		t.Errorf("a sprite that is not there gave %q", got)
	}

	// It is a copy: drawing on it leaves the sheet as it was.
	f.eval(`one:set(0, 0, 8)`)
	if got := f.str(`tiles:sprite(1):get(0, 0)`); got != "1" {
		t.Errorf("the sheet changed with the copy: %q", got)
	}
}

func TestLoadingAPngAsASheet(t *testing.T) {
	f, _ := soundFixture(t, nil, "")
	_ = f

	g := startWith(t, Options{
		Width: 8, Height: 4,
		ReadFile: func(name string) ([]byte, error) {
			if name == "gfx/tiles.png" {
				return sheetPNG(t), nil
			}
			return nil, errNotThere
		},
	}, `
		tiles = loadpng("tiles", 2, 2)
		plain = loadpng("tiles")`)

	if got := g.str(`tiles:grid()`); got != "2,2,2" {
		t.Errorf("the sheet's grid is %q, want 2,2,2", got)
	}
	if got := g.str(`plain:grid()`); got != "0,0,0" {
		t.Errorf("without a size it should be a picture, got %q", got)
	}

	g.eval(`spr(tiles, 1, 0, 0)`)
	if got := g.Screen().Get(0, 0); got != 12 {
		t.Errorf("sprite 1 drew colour %d, want the second cell's 12", got)
	}
}

// sheetPNG is two cells of two pixels square: one of colour 8, one of 12.
func sheetPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for x := 0; x < 4; x++ {
		col := uint8(8)
		if x >= 2 {
			col = 12
		}
		r, g, b := pico.Default.RGB(col)
		for y := 0; y < 2; y++ {
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSpriteFlagsComeWithTheArtwork(t *testing.T) {
	// A sheet drawn in an editor carries its tile size and its flags in the
	// PNG itself, so loading it is all a program has to do.
	f := startWith(t, Options{
		Width: 8, Height: 4,
		ReadFile: func(name string) ([]byte, error) {
			if name == "gfx/tiles.png" {
				return flaggedPNG(t), nil
			}
			return nil, errNotThere
		},
	}, `
		tiles = loadpng("tiles")
		usesheet(tiles)`)

	if got := f.str(`tiles:grid()`); got != "2,2,2" {
		t.Errorf("the sheet cut itself into %q, want 2,2,2", got)
	}

	cases := []struct{ expr, want string }{
		{`fget(1)`, "5"},       // all eight at once
		{`fget(1, 0)`, "true"}, // and one at a time
		{`fget(1, 2)`, "true"},
		{`fget(1, 1)`, "false"},
		{`fget(0)`, "0"},        // a sprite nobody flagged
		{`fget(99)`, "0"},       // and one that is not there
		{`tiles:flags(1)`, "5"}, // the same, for a sheet said out loud
	}
	for _, c := range cases {
		if got := f.str(c.expr); got != c.want {
			t.Errorf("%s = %q, want %q", c.expr, got, c.want)
		}
	}
}

func TestAProgramCanSetFlagsItself(t *testing.T) {
	f := start(t, 4, 2, `
		tiles = sprite([[
			1122
			1122
		]], 2, 2)
		usesheet(tiles)`)

	f.eval(`fset(0, 3, true)`)
	if got := f.str(`fget(0), fget(0, 3)`); got != "8,true" {
		t.Errorf("after setting bit 3: %q", got)
	}
	f.eval(`fset(0, 3, false)`)
	if got := f.str(`fget(0)`); got != "0" {
		t.Errorf("after clearing it: %q", got)
	}

	// All eight at once.
	f.eval(`fset(1, 255)`)
	if got := f.str(`fget(1), fget(1, 7)`); got != "255,true" {
		t.Errorf("after setting them all: %q", got)
	}

	// And on a sheet that is not the current one.
	f.eval(`tiles:flags(0, 6)`)
	if got := f.str(`fget(0)`); got != "6" {
		t.Errorf("the method form set %q", got)
	}
}

func TestFlagsCanComeFromATilesetFileInstead(t *testing.T) {
	// The same information, written where Tiled can see it. A sheet exported
	// for other tools keeps its flags this way.
	const tsj = `{"tilewidth":2,"tileheight":2,"tiles":[
		{"id":0,"properties":[{"name":"flag_1","type":"bool","value":true}]}]}`

	f := startWith(t, Options{
		Width: 8, Height: 4,
		ReadFile: func(name string) ([]byte, error) {
			switch name {
			case "gfx/tiles.png":
				return plainPNG(t), nil
			case "gfx/tiles.tsj":
				return []byte(tsj), nil
			}
			return nil, errNotThere
		},
	}, `
		tiles = loadpng("tiles")
		usesheet(tiles)`)

	if got := f.str(`tiles:grid()`); got != "2,2,2" {
		t.Errorf("the tileset's tile size was not taken up: %q", got)
	}
	if got := f.str(`fget(0, 1)`); got != "true" {
		t.Errorf("the flag from the tileset is %q", got)
	}
}

func TestWhatTheProgramAsksForWinsOverWhatTheFileSays(t *testing.T) {
	f := startWith(t, Options{
		Width: 8, Height: 4,
		ReadFile: func(name string) ([]byte, error) {
			if name == "gfx/tiles.png" {
				return flaggedPNG(t), nil // which says 2x2
			}
			return nil, errNotThere
		},
	}, `tiles = loadpng("tiles", 4, 2)`)

	if got := f.str(`tiles:grid()`); got != "4,2,1" {
		t.Errorf("the grid is %q, want the 4x2 the program asked for", got)
	}
	// The flags still arrive, since nothing else said otherwise.
	if got := f.str(`tiles:flags(1)`); got != "5" {
		t.Errorf("flags are %q", got)
	}
}

func TestFlagsNeedASheetToBeOn(t *testing.T) {
	f := start(t, 2, 1, "")
	if err := f.L.DoString(`fget(0)`); err == nil {
		t.Error("there is no current sheet to have flags")
	}
}

// flaggedPNG is a four by two picture that says it is a sheet of 2x2 sprites,
// the second of which carries flags 0 and 2.
func flaggedPNG(t *testing.T) []byte {
	t.Helper()
	return pngWithMeta(t, plainPNG(t), `{"tile_size":2,"flags":{"1":5}}`)
}

func plainPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	r, g, b := pico.Default.RGB(8)
	img.Set(0, 0, color.RGBA{r, g, b, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// pngWithMeta puts a text chunk in a PNG, as a sheet editor does.
func pngWithMeta(t *testing.T, raw []byte, meta string) []byte {
	t.Helper()
	body := append([]byte("fz_meta\x00"), meta...)
	chunk := make([]byte, 12+len(body))
	binary.BigEndian.PutUint32(chunk, uint32(len(body)))
	copy(chunk[4:], "tEXt")
	copy(chunk[8:], body)
	binary.BigEndian.PutUint32(chunk[8+len(body):], crc32.ChecksumIEEE(chunk[4:8+len(body)]))

	const iend = 12
	out := append([]byte{}, raw[:len(raw)-iend]...)
	out = append(out, chunk...)
	return append(out, raw[len(raw)-iend:]...)
}

func TestThePictureHasTheLastWordOnItsOwnSprites(t *testing.T) {
	// Both files can carry flags. For a sprite they both mention, the picture
	// wins outright — it is where a sheet editor keeps the truth, and the
	// tileset is what it exports. A sprite only the tileset mentions keeps
	// what it says there.
	const tsj = `{"tilewidth":2,"tileheight":2,"tiles":[
		{"id":0,"properties":[{"name":"flag_4","type":"bool","value":true}]},
		{"id":1,"properties":[{"name":"flag_4","type":"bool","value":true}]}]}`

	f := startWith(t, Options{
		Width: 8, Height: 4,
		ReadFile: func(name string) ([]byte, error) {
			switch name {
			case "gfx/tiles.png":
				return flaggedPNG(t), nil // which says sprite 1 has flags 5
			case "gfx/tiles.tsj":
				return []byte(tsj), nil
			}
			return nil, errNotThere
		},
	}, `
		tiles = loadpng("tiles")
		usesheet(tiles)`)

	if got := f.str(`fget(1)`); got != "5" {
		t.Errorf("sprite 1 has flags %q; the picture says 5", got)
	}
	if got := f.str(`fget(0)`); got != "16" {
		t.Errorf("sprite 0 has flags %q; only the tileset mentions it, and says bit 4", got)
	}
}

func TestThingsThatCanDescribeThemselvesDoSo(t *testing.T) {
	// print, printh and tostr all ask a value to describe itself, so that a
	// map or a sheet says what it is rather than where it is.
	f := start(t, 40, 10, `
		s = sprite([[11|11]], 1, 1)
		said = tostr(s)`)
	if got := f.str(`said`); got != "surface 2x2" {
		t.Errorf("tostr of a surface gave %q", got)
	}
	if got := f.str(`tostr(12) .. "/" .. tostr(nil)`); got != "12/nil" {
		t.Errorf("ordinary values still print as they did: %q", got)
	}
}

// TestASpriteCanBeTurnedAsWellAsFlipped is the eighth way of putting a sprite
// down: across its own diagonal, which is what a map's flipd means and what
// spr() could not ask for until it had a turn argument of its own.
func TestASpriteCanBeTurnedAsWellAsFlipped(t *testing.T) {
	// An L, so that every one of the eight orientations looks different.
	const art = `local s = sprite[[
		77.
		7..
		7..
	]]`

	f := start(t, 3, 3, art+`
		spr(s, 0, 0)`)
	f.want(`
		77.
		7..
		7..`)

	// Mirrored left to right.
	f = start(t, 3, 3, art+`
		spr(s, 0, 0, true)`)
	f.want(`
		.77
		..7
		..7`)

	// Turned: rows become columns, so the foot of the L moves across the top.
	f = start(t, 3, 3, art+`
		spr(s, 0, 0, false, false, true)`)
	f.want(`
		777
		7..
		...`)

	// And the two together, which is a quarter turn rather than a mirror.
	f = start(t, 3, 3, art+`
		spr(s, 0, 0, true, false, true)`)
	f.want(`
		777
		..7
		...`)
}

// TestASheetSpriteAndAStretchedOneTurnTheSameWay checks the argument reaches
// the other two ways of drawing, since each counts its arguments differently.
func TestASheetSpriteAndAStretchedOneTurnTheSameWay(t *testing.T) {
	const art = `local sheet = sprite([[
		77.
		7..
		7..
	]], 3, 3)`

	f := start(t, 3, 3, art+`
		spr(sheet, 0, 0, 0, 1, 1, false, false, true)`)
	f.want(`
		777
		7..
		...`)

	f = start(t, 3, 3, art+`
		sspr(sheet, 0, 0, 3, 3, 0, 0, 3, 3, false, false, true)`)
	f.want(`
		777
		7..
		...`)
}

// TestTheCoroutineCallsAreTheOnesThisLineageUses covers the short names, which
// are the language's own functions under the names a program written for one of
// these consoles will call them by.
func TestTheCoroutineCallsAreTheOnesThisLineageUses(t *testing.T) {
	f := start(t, 4, 4, `
		job = cocreate(function(from)
			for i = from, from + 2 do yield(i) end
			return "done"
		end)
		steps = {}
		while costatus(job) ~= "dead" do
			local ok, v = coresume(job, 10)
			add(steps, tostr(v))
		end`)

	if got := f.str(`table.concat(steps, ",")`); got != "10,11,12,done" {
		t.Errorf("the coroutine ran %q", got)
	}
	if got := f.str(`costatus(job)`); got != "dead" {
		t.Errorf("it finished %q", got)
	}
	if got := f.str(`cocreate == coroutine.create, yield == coroutine.yield`); got != "true,true" {
		t.Errorf("they should be the language's own: %q", got)
	}
}
