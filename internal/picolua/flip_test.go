package picolua

import (
	"strings"
	"testing"

	"tlua/internal/pico"
)

func TestAModalDialogLoopsWhereItStands(t *testing.T) {
	// The shape this is for: the program branches out of _update into
	// something that runs a loop of its own, drawing and waiting for an
	// answer, and then carries on where it left off.
	f := start(t, 4, 2, `
		log = {}
		state = "playing"

		function ask()
			state = "asking"
			while not btn("x") do
				cls(8)
				log[#log+1] = "asking"
				flip()
			end
			state = "answered"
		end

		function _update()
			log[#log+1] = "update"
			if #log == 1 then ask() end
		end

		function _draw()
			log[#log+1] = "draw"
			cls(1)
		end`)

	// The first tick goes into the dialog and stops there.
	f.tick(pico.Frame{})
	if got := f.str(`state`); got != "asking" {
		t.Fatalf("after one tick the program is %q", got)
	}
	f.want(`
		8888
		8888`)

	// It stays there, a frame at a time, for as long as nobody answers.
	for i := 0; i < 3; i++ {
		f.tick(pico.Frame{})
	}
	if got := f.str(`state`); got != "asking" {
		t.Errorf("the dialog let go on its own: %q", got)
	}

	// An answer lets it out, and the tick it was in the middle of finishes.
	var pressed pico.Frame
	pressed.Buttons[0][pico.BtnX] = true
	f.tick(pressed)
	if got := f.str(`state`); got != "answered" {
		t.Errorf("after answering, the program is %q", got)
	}

	// And then the ordinary loop goes on as before.
	f.tick(pico.Frame{})
	if got := f.str(`table.concat(log, " ")`); !strings.HasSuffix(got, "draw update draw") {
		t.Errorf("the loop did not pick up again: %q", got)
	}
	f.want(`
		1111
		1111`)
}

func TestAProgramCanBeAllLoopAndNoCallbacks(t *testing.T) {
	// The whole game in one function, which is the other thing flip() is for.
	f := start(t, 4, 2, `
		frames = 0
		while true do
			frames = frames + 1
			cls(frames)
			flip()
		end`)

	if got := f.str(`frames`); got != "1" {
		t.Errorf("the program has run %q frames before the first tick", got)
	}
	f.want(`
		1111
		1111`)

	for i := 0; i < 4; i++ {
		f.tick(pico.Frame{})
	}
	if got := f.str(`frames`); got != "5" {
		t.Errorf("after four ticks it has run %q frames", got)
	}
	if !f.Running() {
		t.Error("a loop of its own is still a program running")
	}
}

func TestEachFlipIsATickOfItsOwn(t *testing.T) {
	// Time and input move on at a flip exactly as they do at the end of an
	// update, because a flip is the end of a tick.
	f := start(t, 4, 2, `
		seen = {}
		function _update()
			for i = 1, 3 do
				seen[#seen+1] = tostr(frame()) .. ":" .. tostr(btn("x"))
				flip()
			end
		end`)

	var pressed pico.Frame
	pressed.Buttons[0][pico.BtnX] = true
	// Three flips, so three ticks, and the button is let go before the last.
	f.tick(pressed)
	f.tick(pressed)
	f.tick(pico.Frame{})

	if got := f.str(`table.concat(seen, " ")`); got != "0:true 1:true 2:false" {
		t.Errorf("across the flips the program saw %q", got)
	}
	if got := f.Frame(); got != 3 {
		t.Errorf("three flips is %d ticks, want 3", got)
	}
}

func TestWhatWasDrawnBeforeAFlipIsWhatIsShown(t *testing.T) {
	f := start(t, 4, 2, `
		function _update()
			cls(3)
			flip()
			cls(5)
		end`)

	f.tick(pico.Frame{})
	f.want(`
		3333
		3333`)
	f.tick(pico.Frame{})
	f.want(`
		5555
		5555`)
}

func TestExitFromInsideALoopOfItsOwn(t *testing.T) {
	f := start(t, 4, 2, `
		local n = 0
		while true do
			n = n + 1
			if n == 3 then exit(2) end
			flip()
		end`)

	for i := 0; i < 5; i++ {
		if quit, code := f.Quitting(); quit {
			if code != 2 {
				t.Errorf("it asked to stop with %d", code)
			}
			return
		}
		f.tick(pico.Frame{})
	}
	t.Error("the program never asked to stop")
}

func TestAnErrorInsideALoopOfItsOwn(t *testing.T) {
	f := start(t, 4, 2, `
		function _update()
			flip()
			error("boom")
		end`)

	f.tick(pico.Frame{}) // into the update, suspended at the flip
	err := f.Tick(pico.Frame{})
	if err == nil {
		t.Fatal("the error should have come back")
	}
	if got := err.Error(); !strings.Contains(got, "boom") || !strings.Contains(got, "main.lua:4") {
		t.Errorf("the error is %q, want it to say where", got)
	}
	if f.Running() {
		t.Error("a program that failed is not still running")
	}
	// And it stays stopped rather than starting again.
	if err := f.Tick(pico.Frame{}); err != nil {
		t.Errorf("ticking a program that is over gave %v", err)
	}
}

func TestFlippingInsideACoroutineOfYourOwn(t *testing.T) {
	// Yielding somebody else's coroutine would hand control to whoever resumed
	// it, which is not the loop. Saying so beats behaving strangely.
	f := start(t, 4, 2, `
		mine = coroutine.create(function() flip() end)
		ok, why = coroutine.resume(mine)`)

	if got := f.str(`ok`); got != "false" {
		t.Errorf("flipping inside a coroutine reported %q", got)
	}
	if got := f.str(`why`); !strings.Contains(got, "coroutine") {
		t.Errorf("the error is %q; it should say what is wrong", got)
	}
}

func TestAProgramThatSimplyEnds(t *testing.T) {
	// A program with nothing to run every frame still has a window to keep
	// open, so ticking it is quiet rather than an error.
	f := start(t, 4, 2, `cls(7)`)
	for i := 0; i < 3; i++ {
		if err := f.Tick(pico.Frame{}); err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
	}
	f.want(`
		7777
		7777`)
}
