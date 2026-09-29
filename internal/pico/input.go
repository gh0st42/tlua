package pico

import "strings"

// How a held button or key repeats: nothing for a quarter of a second, then
// four times a second, which is what PICO-8 does and what a menu written
// against btnp() expects.
const (
	RepeatDelay = 15
	RepeatRate  = 4
)

// Mouse button bits, as a program sees them in the third result of mouse().
const (
	MouseLeft = 1 << iota
	MouseRight
	MouseMiddle
)

// Frame is the raw input state for one tick, as the host reads it off the
// keyboard, the mouse and any pads.
type Frame struct {
	// Buttons is who is holding which of the six console buttons.
	Buttons [Players][Buttons]bool
	// Keys names every key held, in the console's own spelling: a single
	// letter, "space", "arrowleft", "shiftleft" and so on.
	Keys []string
	// Text is what was typed this tick, for a program asking for a name.
	Text []rune

	MouseX, MouseY int
	MouseButtons   int
	WheelX, WheelY float64
}

// Input is how long every button and key has been held, which is all that is
// needed to answer both "is it down" and "has it just been pressed".
type Input struct {
	btn  [Players][Buttons]int
	keys map[string]int
	next map[string]int // scratch for the frame being taken in

	mouseX, mouseY int
	mouseButtons   int
	prevButtons    int
	wheelX, wheelY float64
	text           string
}

// NewInput creates an input state with nothing held.
func NewInput() *Input {
	return &Input{keys: map[string]int{}, next: map[string]int{}}
}

// Update takes in one tick of raw state. Counting frames rather than storing
// "pressed" flags means the edge and the repeat both fall out of one number,
// and a tick the program never looked at cannot lose a press.
func (in *Input) Update(f Frame) {
	for p := range in.btn {
		for b := range in.btn[p] {
			if f.Buttons[p][b] {
				in.btn[p][b]++
			} else {
				in.btn[p][b] = 0
			}
		}
	}

	for _, name := range f.Keys {
		k := strings.ToLower(name)
		in.next[k] = in.keys[k] + 1
	}
	in.keys, in.next = in.next, in.keys
	clear(in.next)

	in.mouseX, in.mouseY = f.MouseX, f.MouseY
	in.prevButtons, in.mouseButtons = in.mouseButtons, f.MouseButtons
	in.wheelX, in.wheelY = f.WheelX, f.WheelY
	in.text = string(f.Text)
}

// Btn reports whether a button is held.
func (in *Input) Btn(player, button int) bool {
	return in.heldFor(player, button) > 0
}

// Btnp reports whether a button has just been pressed, or has been held long
// enough to repeat.
func (in *Input) Btnp(player, button int) bool {
	return pressed(in.heldFor(player, button))
}

// HeldFor reports how many ticks a button has been held.
func (in *Input) HeldFor(player, button int) int { return in.heldFor(player, button) }

func (in *Input) heldFor(player, button int) int {
	if player < 0 || player >= Players || button < 0 || button >= Buttons {
		return 0
	}
	return in.btn[player][button]
}

// AnyBtn reports whether any player is holding anything, which is what a title
// screen waits for.
func (in *Input) AnyBtn() bool {
	for p := range in.btn {
		for b := range in.btn[p] {
			if in.btn[p][b] > 0 {
				return true
			}
		}
	}
	return false
}

// Key reports whether a key is held, by any of its names.
func (in *Input) Key(name string) bool {
	a, b := KeyNames(name)
	return in.keys[a] > 0 || (b != "" && in.keys[b] > 0)
}

// Keyp reports whether a key has just been pressed, or is repeating.
func (in *Input) Keyp(name string) bool {
	a, b := KeyNames(name)
	if pressed(in.keys[a]) {
		return true
	}
	return b != "" && pressed(in.keys[b])
}

// AnyKey reports whether any key at all is held.
func (in *Input) AnyKey() bool { return len(in.keys) > 0 }

// Text reports the characters typed this tick.
func (in *Input) Text() string { return in.text }

// Mouse reports where the pointer is, which buttons are down and how far the
// wheel turned.
func (in *Input) Mouse() (x, y, buttons int, wheel float64) {
	return in.mouseX, in.mouseY, in.mouseButtons, in.wheelY
}

// Wheel reports both wheel axes.
func (in *Input) Wheel() (x, y float64) { return in.wheelX, in.wheelY }

// MouseBtn reports whether a mouse button is down; MouseBtnp reports whether it
// went down this tick. Mouse buttons do not repeat: a click is a click.
func (in *Input) MouseBtn(bit int) bool { return in.mouseButtons&bit != 0 }

func (in *Input) MouseBtnp(bit int) bool {
	return in.mouseButtons&bit != 0 && in.prevButtons&bit == 0
}

// pressed reports whether something held for n ticks counts as a press now:
// the tick it went down, then the tick after the delay runs out, then every
// RepeatRate ticks after that.
func pressed(n int) bool {
	return n == 1 || (n > RepeatDelay && (n-RepeatDelay-1)%RepeatRate == 0)
}

// keyAliases spells the keys a program is likely to ask for by a name other
// than the one the host reports. The host's names come from the window
// library; these are the names a person would type.
var keyAliases = map[string]string{
	"left":      "arrowleft",
	"right":     "arrowright",
	"up":        "arrowup",
	"down":      "arrowdown",
	"esc":       "escape",
	"return":    "enter",
	"del":       "delete",
	"ins":       "insert",
	"pgup":      "pageup",
	"pgdn":      "pagedown",
	"pgdown":    "pagedown",
	"capslock":  "capslock",
	"-":         "minus",
	"=":         "equal",
	",":         "comma",
	".":         "period",
	"/":         "slash",
	"\\":        "backslash",
	";":         "semicolon",
	"'":         "quote",
	"[":         "bracketleft",
	"]":         "bracketright",
	"`":         "backquote",
	"backtick":  "backquote",
	"spacebar":  "space",
	"backspace": "backspace",
}

// keyPairs are the keys that come in twos, where asking for "shift" means
// either of them.
var keyPairs = map[string][2]string{
	"shift":   {"shiftleft", "shiftright"},
	"ctrl":    {"controlleft", "controlright"},
	"control": {"controlleft", "controlright"},
	"alt":     {"altleft", "altright"},
	"option":  {"altleft", "altright"},
	"cmd":     {"metaleft", "metaright"},
	"meta":    {"metaleft", "metaright"},
	"super":   {"metaleft", "metaright"},
	"win":     {"metaleft", "metaright"},
}

// KeyNames turns what a program asks for into the one or two names the host
// reports: "left" is the left arrow, "shift" is either shift key, a digit is
// the digit row, and anything else is passed through as it stands.
func KeyNames(name string) (string, string) {
	name = strings.ToLower(strings.TrimSpace(name))
	if pair, ok := keyPairs[name]; ok {
		return pair[0], pair[1]
	}
	if alias, ok := keyAliases[name]; ok {
		return alias, ""
	}
	// A bare digit is the number row; the keypad is "numpad0" and up.
	if len(name) == 1 && name[0] >= '0' && name[0] <= '9' {
		return "digit" + name, ""
	}
	return name, ""
}
