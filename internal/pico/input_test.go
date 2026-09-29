package pico

import "testing"

// hold feeds n ticks with one button held by player 0.
func hold(in *Input, button, n int) {
	var f Frame
	f.Buttons[0][button] = true
	for i := 0; i < n; i++ {
		in.Update(f)
	}
}

func TestBtnIsTrueWhileHeld(t *testing.T) {
	in := NewInput()
	if in.Btn(0, BtnLeft) {
		t.Error("nothing is held yet")
	}
	hold(in, BtnLeft, 3)
	if !in.Btn(0, BtnLeft) {
		t.Error("left should be held")
	}
	if in.Btn(0, BtnRight) {
		t.Error("only left is held")
	}
	in.Update(Frame{})
	if in.Btn(0, BtnLeft) {
		t.Error("left was let go")
	}
}

func TestBtnpIsTrueOnlyOnThePressAndThenOnTheRepeat(t *testing.T) {
	in := NewInput()
	var f Frame
	f.Buttons[0][BtnX] = true

	in.Update(f)
	if !in.Btnp(0, BtnX) {
		t.Fatal("the first tick of a press should report one")
	}
	for tick := 2; tick <= RepeatDelay; tick++ {
		in.Update(f)
		if in.Btnp(0, BtnX) {
			t.Fatalf("tick %d repeated before the delay was up", tick)
		}
	}
	in.Update(f) // RepeatDelay+1
	if !in.Btnp(0, BtnX) {
		t.Fatalf("a button held for %d ticks should repeat", RepeatDelay+1)
	}
	for i := 1; i < RepeatRate; i++ {
		in.Update(f)
		if in.Btnp(0, BtnX) {
			t.Fatal("it repeated faster than the rate")
		}
	}
	in.Update(f)
	if !in.Btnp(0, BtnX) {
		t.Fatalf("it should repeat every %d ticks", RepeatRate)
	}
}

func TestReleasingAButtonStartsItsCountAgain(t *testing.T) {
	in := NewInput()
	hold(in, BtnO, 20)
	in.Update(Frame{})
	hold(in, BtnO, 1)
	if !in.Btnp(0, BtnO) || in.HeldFor(0, BtnO) != 1 {
		t.Errorf("a fresh press should read as one tick, got %d", in.HeldFor(0, BtnO))
	}
}

func TestButtonsAreTrackedPerPlayer(t *testing.T) {
	in := NewInput()
	var f Frame
	f.Buttons[1][BtnUp] = true
	in.Update(f)
	if !in.Btn(1, BtnUp) {
		t.Error("player two is holding up")
	}
	if in.Btn(0, BtnUp) {
		t.Error("player one is not")
	}
	if !in.AnyBtn() {
		t.Error("somebody is holding something")
	}
}

func TestOutOfRangePlayersAndButtonsAreQuietlyEmpty(t *testing.T) {
	in := NewInput()
	hold(in, BtnLeft, 1)
	for _, c := range [][2]int{{-1, 0}, {Players, 0}, {0, -1}, {0, Buttons}} {
		if in.Btn(c[0], c[1]) || in.Btnp(c[0], c[1]) {
			t.Errorf("player %d button %d should read as nothing", c[0], c[1])
		}
	}
}

func TestKeysAreHeldAndRepeatLikeButtons(t *testing.T) {
	in := NewInput()
	f := Frame{Keys: []string{"a", "space"}}
	in.Update(f)
	if !in.Key("a") || !in.Key("space") {
		t.Error("both keys should be held")
	}
	if !in.Keyp("a") {
		t.Error("the first tick is a press")
	}
	in.Update(f)
	if in.Keyp("a") {
		t.Error("the second tick is not")
	}
	if !in.Key("A") {
		t.Error("key names should not be case sensitive")
	}
	in.Update(Frame{})
	if in.Key("a") || in.AnyKey() {
		t.Error("the keys were let go")
	}
}

func TestKeyNamesAcceptTheNamesAPersonWouldType(t *testing.T) {
	cases := map[string][2]string{
		"left":  {"arrowleft", ""},
		"LEFT":  {"arrowleft", ""},
		"esc":   {"escape", ""},
		"5":     {"digit5", ""},
		"-":     {"minus", ""},
		"shift": {"shiftleft", "shiftright"},
		"ctrl":  {"controlleft", "controlright"},
		"cmd":   {"metaleft", "metaright"},
		"f1":    {"f1", ""},
		"z":     {"z", ""},
	}
	for name, want := range cases {
		a, b := KeyNames(name)
		if a != want[0] || b != want[1] {
			t.Errorf("KeyNames(%q) = %q,%q, want %q,%q", name, a, b, want[0], want[1])
		}
	}
}

func TestEitherSideOfAPairedKeyCounts(t *testing.T) {
	in := NewInput()
	in.Update(Frame{Keys: []string{"shiftright"}})
	if !in.Key("shift") {
		t.Error("the right shift should answer for shift")
	}
	if !in.Keyp("shift") {
		t.Error("and its press should too")
	}
}

func TestMouseReportsWhereItIsAndWhatIsDown(t *testing.T) {
	in := NewInput()
	in.Update(Frame{MouseX: 10, MouseY: 20, MouseButtons: MouseLeft, WheelY: -2})
	x, y, buttons, wheel := in.Mouse()
	if x != 10 || y != 20 || buttons != MouseLeft || wheel != -2 {
		t.Errorf("mouse reads %d,%d buttons=%d wheel=%v", x, y, buttons, wheel)
	}
	if !in.MouseBtn(MouseLeft) || in.MouseBtn(MouseRight) {
		t.Error("only the left button is down")
	}
	if !in.MouseBtnp(MouseLeft) {
		t.Error("the button went down this tick")
	}
	in.Update(Frame{MouseX: 10, MouseY: 20, MouseButtons: MouseLeft})
	if in.MouseBtnp(MouseLeft) {
		t.Error("a held mouse button is not a fresh click")
	}
	if !in.MouseBtn(MouseLeft) {
		t.Error("it is still down, though")
	}
}

func TestTextIsWhatWasTypedThisTick(t *testing.T) {
	in := NewInput()
	in.Update(Frame{Text: []rune("hi")})
	if got := in.Text(); got != "hi" {
		t.Errorf("text is %q", got)
	}
	in.Update(Frame{})
	if got := in.Text(); got != "" {
		t.Errorf("text should not carry over, got %q", got)
	}
}
