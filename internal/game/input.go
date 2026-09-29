package game

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"tlua/internal/pico"
)

// buttonKeys is the keyboard as two game pads, the way PICO-8 lays it out:
// player one on the arrow keys with Z and X beside them, player two on ESDF
// with the keys around it. Players three and four are pads only.
var buttonKeys = [pico.Players][pico.Buttons][]ebiten.Key{
	0: {
		pico.BtnLeft:  {ebiten.KeyArrowLeft},
		pico.BtnRight: {ebiten.KeyArrowRight},
		pico.BtnUp:    {ebiten.KeyArrowUp},
		pico.BtnDown:  {ebiten.KeyArrowDown},
		pico.BtnO:     {ebiten.KeyZ, ebiten.KeyC, ebiten.KeyN},
		pico.BtnX:     {ebiten.KeyX, ebiten.KeyV, ebiten.KeyM},
	},
	1: {
		pico.BtnLeft:  {ebiten.KeyS},
		pico.BtnRight: {ebiten.KeyF},
		pico.BtnUp:    {ebiten.KeyE},
		pico.BtnDown:  {ebiten.KeyD},
		pico.BtnO:     {ebiten.KeyShiftLeft, ebiten.KeyA},
		pico.BtnX:     {ebiten.KeyTab, ebiten.KeyQ},
	},
}

// padButtons is a standard pad's face and direction buttons, in the console's
// own order. A pad that reports no standard layout falls back to its stick.
var padButtons = [pico.Buttons][]ebiten.StandardGamepadButton{
	pico.BtnLeft:  {ebiten.StandardGamepadButtonLeftLeft},
	pico.BtnRight: {ebiten.StandardGamepadButtonLeftRight},
	pico.BtnUp:    {ebiten.StandardGamepadButtonLeftTop},
	pico.BtnDown:  {ebiten.StandardGamepadButtonLeftBottom},
	pico.BtnO:     {ebiten.StandardGamepadButtonRightBottom, ebiten.StandardGamepadButtonRightLeft},
	pico.BtnX:     {ebiten.StandardGamepadButtonRightRight, ebiten.StandardGamepadButtonRightTop},
}

// stickThreshold is how far a stick has to be pushed to count as a direction.
const stickThreshold = 0.4

// mouseButtons pairs the window's buttons with the bits a program sees. It is a
// fixed list rather than a map because this is read sixty times a second, and a
// map literal in that loop would be an allocation each time round.
var mouseButtons = [...]struct {
	bit    int
	button ebiten.MouseButton
}{
	{pico.MouseLeft, ebiten.MouseButtonLeft},
	{pico.MouseRight, ebiten.MouseButtonRight},
	{pico.MouseMiddle, ebiten.MouseButtonMiddle},
}

// reader turns the window's input into a frame for the console. Its scratch
// slices are kept between frames, because this runs sixty times a second and
// none of it should be work for the garbage collector.
type reader struct {
	keys  []ebiten.Key
	names []string
	pads  []ebiten.GamepadID
	typed []rune

	justPressedKeys []ebiten.Key
}

// poll reads the keys that went down this tick. It is called before the frame
// is built, because the window's own shortcuts are decided from these and take
// their keys out of what the program is shown.
func (rd *reader) poll() {
	rd.justPressedKeys = inpututil.AppendJustPressedKeys(rd.justPressedKeys[:0])
}

// frame reads everything the program can ask about this tick, less whatever the
// window's own shortcuts took. The view is needed because the mouse has to be
// reported in console pixels, not in the pixels of whatever size the window
// happens to be.
func (rd *reader) frame(v view, used []ebiten.Key) pico.Frame {
	var f pico.Frame

	// Keys, both as names for key() and as buttons for btn().
	rd.keys = inpututil.AppendPressedKeys(rd.keys[:0])
	rd.names = rd.names[:0]
	for _, k := range rd.keys {
		if consumed(used, k) {
			continue
		}
		rd.names = append(rd.names, keyName(k))
	}
	f.Keys = rd.names

	for player := range buttonKeys {
		for button, keys := range buttonKeys[player] {
			for _, k := range keys {
				if ebiten.IsKeyPressed(k) && !consumed(used, k) {
					f.Buttons[player][button] = true
					break
				}
			}
		}
	}

	rd.pads = ebiten.AppendGamepadIDs(rd.pads[:0])
	for player, id := range rd.pads {
		if player >= pico.Players {
			break
		}
		readPad(&f, player, id)
	}

	x, y := ebiten.CursorPosition()
	f.MouseX, f.MouseY = v.consolePixel(x, y)
	for _, m := range mouseButtons {
		if ebiten.IsMouseButtonPressed(m.button) {
			f.MouseButtons |= m.bit
		}
	}
	f.WheelX, f.WheelY = ebiten.Wheel()

	rd.typed = ebiten.AppendInputChars(rd.typed[:0])
	f.Text = rd.typed
	return f
}

// consumed reports whether one of the window's own shortcuts took a key this
// tick.
func consumed(used []ebiten.Key, k ebiten.Key) bool {
	for _, c := range used {
		if c == k {
			return true
		}
	}
	return false
}

// justPressed reports whether a key went down on this tick, which is how the
// window's own shortcuts avoid firing every frame the key is held.
func (rd *reader) justPressed(key ebiten.Key) bool {
	for _, k := range rd.justPressedKeys {
		if k == key {
			return true
		}
	}
	return false
}

// readPad folds a pad's buttons into the frame, on top of whatever the keyboard
// has already said, so that either can drive the same player.
func readPad(f *pico.Frame, player int, id ebiten.GamepadID) {
	if ebiten.IsStandardGamepadLayoutAvailable(id) {
		for button, pad := range padButtons {
			for _, b := range pad {
				if ebiten.IsStandardGamepadButtonPressed(id, b) {
					f.Buttons[player][button] = true
					break
				}
			}
		}
		x := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		y := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		applyStick(f, player, x, y)
		return
	}

	// An unknown pad still has axes; the first two are a stick often enough to
	// be worth reading, and its buttons are at least in a usable order.
	applyStick(f, player,
		ebiten.GamepadAxisValue(id, 0),
		ebiten.GamepadAxisValue(id, 1))
	if ebiten.IsGamepadButtonPressed(id, ebiten.GamepadButton0) {
		f.Buttons[player][pico.BtnO] = true
	}
	if ebiten.IsGamepadButtonPressed(id, ebiten.GamepadButton1) {
		f.Buttons[player][pico.BtnX] = true
	}
}

// applyStick turns a stick position into the four directions.
func applyStick(f *pico.Frame, player int, x, y float64) {
	switch {
	case x < -stickThreshold:
		f.Buttons[player][pico.BtnLeft] = true
	case x > stickThreshold:
		f.Buttons[player][pico.BtnRight] = true
	}
	switch {
	case y < -stickThreshold:
		f.Buttons[player][pico.BtnUp] = true
	case y > stickThreshold:
		f.Buttons[player][pico.BtnDown] = true
	}
}

// keyName is what a program calls a key: the window library's own name in lower
// case, which is what pico.KeyNames resolves the friendly spellings to.
func keyName(k ebiten.Key) string {
	return strings.ToLower(k.String())
}
