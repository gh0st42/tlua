package picolua

import (
	"fmt"
	"io"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"tlua/internal/pico"
)

func TestTheMathHelpers(t *testing.T) {
	f := start(t, 2, 1, "")
	cases := []struct{ expr, want string }{
		{`flr(1.7)`, "1"},
		{`flr(-1.2)`, "-2"},
		{`ceil(1.2)`, "2"},
		{`abs(-3)`, "3"},
		{`sqrt(9)`, "3"},
		{`sqrt(-1)`, "0"}, // rather than an error
		{`sgn(-2)`, "-1"},
		{`sgn(0)`, "1"}, // as on PICO-8
		{`min(3, 5)`, "3"},
		{`max(3, 5)`, "5"},
		{`mid(5, 1, 9)`, "5"},
		{`mid(-5, 1, 9)`, "1"},
		{`mid(50, 1, 9)`, "9"},
		{`clamp(5, 1, 9)`, "5"},
		{`clamp(-5, 1, 9)`, "1"},
		{`clamp(50, 1, 9)`, "9"},
		{`clamp(50, 9, 1)`, "9"}, // whichever way round the limits are given

		// A turn is a whole circle, and a quarter turn points down the screen.
		{`cos(0)`, "1"},
		{`sin(0)`, "0"},
		{`flr(sin(0.25) * 100)`, "-100"},
		{`flr(cos(0.5) * 100)`, "-100"},
		{`atan2(1, 0)`, "0"},
		{`atan2(0, -1)`, "0.25"}, // up the screen is a quarter turn
		{`atan2(-1, 0)`, "0.5"},
		{`atan2(0, 1)`, "0.75"}, // and down it is three quarters
	}
	for _, c := range cases {
		if got := f.str(c.expr); got != c.want {
			t.Errorf("%s = %q, want %q", c.expr, got, c.want)
		}
	}
}

func TestAnAngleSurvivesTheRoundTrip(t *testing.T) {
	// atan2 has to undo sin and cos, whatever way round they run; this is the
	// property a program relies on when it turns a velocity back into a heading.
	f := start(t, 2, 1, `
		worst = 0
		for i = 0, 15 do
			local a = i / 16
			local back = atan2(cos(a), sin(a))
			worst = max(worst, abs(back - a))
		end`)
	if got := f.str(`worst < 0.0001`); got != "true" {
		t.Errorf("atan2 does not undo sin and cos; worst error %s", f.str(`worst`))
	}
}

func TestSinAndCosAgreeWithTheScreen(t *testing.T) {
	// A quarter turn should move a sprite down the screen, not up it: the point
	// of sin() running backwards is that x,y from an angle lands where the
	// numbers say it will.
	f := start(t, 2, 1, "")
	if got := f.str(`sin(0.25) < 0, cos(0) > 0`); got != "true,true" {
		t.Errorf("angles do not follow the screen: %s", got)
	}
}

func TestRandomIsRepeatableFromASeed(t *testing.T) {
	f := start(t, 2, 1, `
		srand(7)
		a = { rnd(), rnd(100), rnd{ "x", "y", "z" } }
		srand(7)
		b = { rnd(), rnd(100), rnd{ "x", "y", "z" } }`)
	if got := f.str(`a[1] == b[1], a[2] == b[2], a[3] == b[3]`); got != "true,true,true" {
		t.Errorf("the same seed gave different numbers: %s", got)
	}
	if got := f.str(`a[2] >= 0 and a[2] < 100`); got != "true" {
		t.Errorf("rnd(100) is out of range: %s", f.str(`a[2]`))
	}
	if got := f.str(`rnd{} == nil`); got != "true" {
		t.Errorf("rnd of an empty table should be nothing, got %s", got)
	}
}

func TestTheTableHelpers(t *testing.T) {
	f := start(t, 2, 1, `
		t = {}
		added = add(t, "a")
		add(t, "b")
		add(t, "c")
		add(t, "first", 1)
		removed = del(t, "b")
		last = deli(t)
		joined = table.concat(t, ",")

		sum = 0
		for v in all({1, 2, 3}) do sum = sum + v end

		seen = {}
		foreach({4, 5}, function(v) seen[#seen+1] = v end)`)

	cases := []struct{ expr, want string }{
		{`added`, "a"},
		{`removed`, "b"},
		{`last`, "c"},
		{`joined`, "first,a"},
		{`sum`, "6"},
		{`table.concat(seen, ",")`, "4,5"},
		{`count({1, 2, 3})`, "3"},
		{`count({1, 2, 2}, 2)`, "2"},
		{`del({1}, 9) == nil`, "true"},
		{`deli({}, 4) == nil`, "true"},
	}
	for _, c := range cases {
		if got := f.str(c.expr); got != c.want {
			t.Errorf("%s = %q, want %q", c.expr, got, c.want)
		}
	}
}

func TestTheStringHelpers(t *testing.T) {
	f := start(t, 2, 1, "")
	cases := []struct{ expr, want string }{
		{`sub("hello", 2, 3)`, "el"},
		{`sub("hello", 2)`, "ello"},
		{`sub("hello", -2)`, "lo"},
		{`sub("hello", 9)`, ""},
		{`table.concat(split("1,2,3"), "|")`, "1|2|3"},
		{`type(split("1,2")[1])`, "number"},
		{`type(split("1,2", ",", false)[1])`, "string"},
		{`table.concat(split("a b", " "), "|")`, "a|b"},
		{`table.concat(split("abc", ""), "|")`, "a|b|c"},
		{`tostr(12)`, "12"},
		{`tostr(1.5)`, "1.5"},
		{`tostr(nil)`, "nil"},
		{`tostr(true)`, "true"},
		{`tostr(255, true)`, "0xff"},
		{`tonum("42") + 1`, "43"},
		{`tonum("x") == nil`, "true"},
		{`tonum({}) == nil`, "true"},
		{`chr(104, 105)`, "hi"},
		{`ord("A")`, "65"},
		{`ord("ABC", 2)`, "66"},
		{`ord("A", 9) == nil`, "true"},
	}
	for _, c := range cases {
		if got := f.str(c.expr); got != c.want {
			t.Errorf("%s = %q, want %q", c.expr, got, c.want)
		}
	}
}

func TestTimeCountsTicksRatherThanWallClock(t *testing.T) {
	// A counted clock keeps a game's motion in step with what is drawn, and
	// lets a test run frames as fast as it likes.
	f := start(t, 2, 1, `function _update() end`)
	if got := f.str(`t()`); got != "0" {
		t.Errorf("t() starts at %q", got)
	}
	for i := 0; i < 30; i++ {
		f.tick(pico.Frame{})
	}
	if got := f.str(`t()`); got != "0.5" {
		t.Errorf("after 30 ticks t() is %q, want 0.5", got)
	}
	if got := f.str(`frame()`); got != "30" {
		t.Errorf("frame() is %q", got)
	}
}

func TestTheClockCanComeFromTheHost(t *testing.T) {
	f := start(t, 2, 1, "")
	f.clock = func() float64 { return 12.5 }
	if got := f.str(`t()`); got != "12.5" {
		t.Errorf("t() is %q", got)
	}
	if got := f.str(`time()`); got != "12.5" {
		t.Errorf("time() is %q", got)
	}
}

/* --- input --- */

// pressing builds a frame with one button held by one player.
func pressing(player, button int) pico.Frame {
	var f pico.Frame
	f.Buttons[player][button] = true
	return f
}

func TestBtnReadsButtonsByNumberAndByName(t *testing.T) {
	f := start(t, 2, 1, `function _update() end`)
	f.tick(pressing(0, pico.BtnLeft))

	cases := []struct{ expr, want string }{
		{`btn(0)`, "true"},
		{`btn("left")`, "true"},
		{`btn("LEFT")`, "true"},
		{`btn(1)`, "false"},
		{`btn("right")`, "false"},
		{`btn()`, "true"}, // anything at all
		{`btn(0, 1)`, "false"},
		{`btnp("left")`, "true"},
		{`held("left")`, "1"},
	}
	for _, c := range cases {
		if got := f.str(c.expr); got != c.want {
			t.Errorf("%s = %q, want %q", c.expr, got, c.want)
		}
	}

	f.tick(pressing(0, pico.BtnLeft))
	if got := f.str(`btn("left"), btnp("left"), held("left")`); got != "true,false,2" {
		t.Errorf("on the second tick: %s", got)
	}
}

func TestBtnRefusesAButtonThatIsNotOne(t *testing.T) {
	f := start(t, 2, 1, "")
	for _, expr := range []string{`btn("wiggle")`, `btn({})`} {
		if err := f.L.DoString(expr); err == nil {
			t.Errorf("%s should be refused", expr)
		}
	}
}

func TestKeysAndMouseReachLua(t *testing.T) {
	f := start(t, 2, 1, `function _update() end`)
	f.tick(pico.Frame{
		Keys:         []string{"arrowleft", "z", "shiftright"},
		Text:         []rune("hi"),
		MouseX:       12,
		MouseY:       34,
		MouseButtons: pico.MouseLeft,
		WheelY:       -1,
	})

	cases := []struct{ expr, want string }{
		{`key("left")`, "true"},
		{`key("z")`, "true"},
		{`key("shift")`, "true"},
		{`key("q")`, "false"},
		{`key()`, "true"},
		{`keyp("left")`, "true"},
		{`mouse()`, "12,34,1,-1"},
		{`mousebtn(1)`, "true"},
		{`mousebtn(2)`, "false"},
		{`mousebtn(1, true)`, "true"},
		{`typed()`, "hi"},
	}
	for _, c := range cases {
		if got := f.str(c.expr); got != c.want {
			t.Errorf("%s = %q, want %q", c.expr, got, c.want)
		}
	}
}

func TestEveryDocumentedNameIsThere(t *testing.T) {
	// The reference lists these; a missing one is a program that will not run
	// for a reason nobody can see.
	names := strings.Fields(`
		cls color pset pget line rect rectfill circ circfill oval ovalfill
		tri trifill print cursor textwidth textheight camera clip pal palt
		rrect rrectfill fillp screen palette rgb surface sprite loadpng fetch
		spr sspr target map usesheet sget sset fget fset
		loadmap usemap mget mset
		btn btnp held key keyp mouse mousebtn btnkey typed
		t time frame fps setfps printh exit window fullscreen vid
		sfx music volume
		flr ceil abs sqrt sgn sin cos atan2 min max mid clamp rnd srand
		add del deli all foreach count sub split tostr tonum chr ord`)

	f := start(t, 2, 1, "")
	for _, name := range names {
		if got := f.str(`type(` + name + `)`); got != "function" {
			t.Errorf("%s is %s, want a function", name, got)
		}
	}
}

func TestAButtonCanSayWhichKeyWorksIt(t *testing.T) {
	// Without a window there is no keyboard layout to ask about, so the answer
	// is the place the key sits. What matters is that a program always gets
	// something to print rather than an empty string.
	f := start(t, 2, 1, "")
	cases := []struct{ expr, want string }{
		{`btnkey("o")`, "Z"},
		{`btnkey("x")`, "X"},
		{`btnkey("left")`, "ARROWLEFT"},
		{`btnkey("o", 1)`, "SHIFTLEFT"},
		{`btnkey(4)`, "Z"},
	}
	for _, c := range cases {
		if got := f.str(c.expr); got != c.want {
			t.Errorf("%s = %q, want %q", c.expr, got, c.want)
		}
	}
}

func TestTheHostCanNameTheKeysItself(t *testing.T) {
	// Which is what happens with a window open: the label comes from the
	// keyboard layout rather than from the place.
	L := lua.NewState()
	defer L.Close()
	New(L, Options{
		Width: 2, Height: 1, Out: io.Discard,
		ButtonLabel: func(player, button int) string {
			return fmt.Sprintf("p%d/b%d", player, button)
		},
	})
	if err := L.DoString(`answer = btnkey("o", 1)`); err != nil {
		t.Fatal(err)
	}
	if got := lua.LVAsString(L.GetGlobal("answer")); got != "p1/b4" {
		t.Errorf("btnkey asked for %q", got)
	}
}

func TestTostrTakesASecondResultInItsStride(t *testing.T) {
	// A call that reports a value and an error hands over both; tostr() is
	// usually what the first one is wrapped in, and the second one must not
	// turn that into a complaint.
	f := start(t, 2, 1, `
		local function two() return 12, "and a message" end
		plain = tostr(two())
		hex = tostr(255, 1)`)
	if got := f.str(`plain`); got != "12" {
		t.Errorf("tostr of two results gave %q", got)
	}
	if got := f.str(`hex`); got != "0xff" {
		t.Errorf("a number should ask for hexadecimal, got %q", got)
	}

	cases := []struct{ expr, want string }{
		{`tostr(255, true)`, "0xff"},
		{`tostr(255, 1)`, "0xff"},
		{`tostr(255, false)`, "255"},
		{`tostr(255, 0)`, "255"},
		{`tostr(255, nil)`, "255"},
		{`tostr(255, "a message")`, "255"}, // a stray second result, not a flag
	}
	for _, c := range cases {
		if got := f.str(c.expr); got != c.want {
			t.Errorf("%s = %q, want %q", c.expr, got, c.want)
		}
	}
}

func TestAskingToRunAtAnotherRate(t *testing.T) {
	asked := []int{}
	f := startWith(t, Options{
		Width: 2, Height: 1,
		SetTPS: func(rate int) { asked = append(asked, rate) },
	}, `function _update() end`)

	if got := f.str(`setfps()`); got != "60" {
		t.Errorf("a program runs at %q a second to begin with", got)
	}

	if got := f.str(`setfps(30)`); got != "60" {
		t.Errorf("setfps reported %q as the rate it replaced", got)
	}
	if len(asked) != 1 || asked[0] != 30 {
		t.Errorf("the host was asked for %v", asked)
	}

	// Seconds are counted at the rate the program runs at, so half a second is
	// half a second whatever that rate is.
	for i := 0; i < 15; i++ {
		f.tick(pico.Frame{})
	}
	if got := f.str(`t()`); got != "0.5" {
		t.Errorf("after 15 ticks at 30 a second, t() is %q, want 0.5", got)
	}

	// Running with the screen, whatever it manages.
	if got := f.str(`setfps(-1)`); got != "30" {
		t.Errorf("setfps reported %q", got)
	}
	if asked[len(asked)-1] != -1 {
		t.Errorf("the host was asked for %v", asked)
	}
}

func TestARateNothingCanRunAt(t *testing.T) {
	f := start(t, 2, 1, "")
	for _, expr := range []string{`setfps(0)`, `setfps(-2)`, `setfps(100000)`} {
		err := f.L.DoString(expr)
		if err == nil {
			t.Errorf("%s should be refused", expr)
			continue
		}
		if !strings.Contains(err.Error(), "-1") {
			t.Errorf("%s said %q; it should say what would work", expr, err)
		}
	}
}

func TestAProgramWithNoHostStillCountsItsSeconds(t *testing.T) {
	// No host to tell, but the clock still follows what was asked for.
	f := start(t, 2, 1, `function _update() end`)
	f.eval(`setfps(120)`)
	for i := 0; i < 60; i++ {
		f.tick(pico.Frame{})
	}
	if got := f.str(`t()`); got != "0.5" {
		t.Errorf("after 60 ticks at 120 a second, t() is %q, want 0.5", got)
	}
}
