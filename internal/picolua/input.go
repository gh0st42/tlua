package picolua

import (
	"strings"

	lua "github.com/yuin/gopher-lua"

	"tlua/internal/pico"
)

// buttonNames lets a program say what it means instead of counting: btn("left")
// rather than btn(0). The numbers are PICO-8's, so code that counts still works.
var buttonNames = map[string]int{
	"left": pico.BtnLeft, "right": pico.BtnRight,
	"up": pico.BtnUp, "down": pico.BtnDown,
	"o": pico.BtnO, "x": pico.BtnX,
}

// button reads a button argument, by number or by name.
func button(L *lua.LState, n int) int {
	switch v := L.Get(n).(type) {
	case lua.LNumber:
		return int(v)
	case lua.LString:
		if b, ok := buttonNames[strings.ToLower(string(v))]; ok {
			return b
		}
		L.ArgError(n, "no button called "+string(v))
	}
	L.ArgError(n, "button number or name expected")
	return 0
}

func (r *Runtime) installInput() {
	r.register(map[string]lua.LGFunction{
		// btn(button, [player]) is true while it is held. With no arguments it
		// is true while anything at all is held, which is what a title screen
		// wants.
		"btn": func(L *lua.LState) int {
			if isNone(L, 1) {
				L.Push(lua.LBool(r.In.AnyBtn()))
				return 1
			}
			L.Push(lua.LBool(r.In.Btn(L.OptInt(2, 0), button(L, 1))))
			return 1
		},

		// btnp(button, [player]) is true on the tick it went down, and again
		// once it has been held long enough to repeat.
		"btnp": func(L *lua.LState) int {
			L.Push(lua.LBool(r.In.Btnp(L.OptInt(2, 0), button(L, 1))))
			return 1
		},

		// held(button, [player]) is how many ticks it has been down for, which
		// is how a game charges a jump or a shot.
		"held": func(L *lua.LState) int {
			L.Push(lua.LNumber(r.In.HeldFor(L.OptInt(2, 0), button(L, 1))))
			return 1
		},

		// key("x") and keyp("x") read the keyboard directly, for the keys a
		// console never had: "left", "space", "shift", "f1", "escape".
		"key": func(L *lua.LState) int {
			if isNone(L, 1) {
				L.Push(lua.LBool(r.In.AnyKey()))
				return 1
			}
			L.Push(lua.LBool(r.In.Key(L.CheckString(1))))
			return 1
		},

		"keyp": func(L *lua.LState) int {
			L.Push(lua.LBool(r.In.Keyp(L.CheckString(1))))
			return 1
		},

		// mouse() reports where the pointer is in screen pixels, which buttons
		// are down as a bit per button, and how far the wheel has turned since
		// the last tick.
		"mouse": func(L *lua.LState) int {
			x, y, buttons, wheel := r.In.Mouse()
			L.Push(lua.LNumber(x))
			L.Push(lua.LNumber(y))
			L.Push(lua.LNumber(buttons))
			L.Push(lua.LNumber(wheel))
			return 4
		},

		// mousebtn(1) is the left button, 2 the right, 3 the middle; with a
		// second argument it is true only on the tick it was pressed.
		"mousebtn": func(L *lua.LState) int {
			bit := 1 << (L.OptInt(1, 1) - 1)
			if L.OptBool(2, false) {
				L.Push(lua.LBool(r.In.MouseBtnp(bit)))
				return 1
			}
			L.Push(lua.LBool(r.In.MouseBtn(bit)))
			return 1
		},

		// btnkey(button, [player]) is the key that works a button, named as it
		// is printed on the keyboard being used. A program telling somebody
		// which key to press has to ask, because the console binds keys by
		// where they are rather than by what they say, and the two differ on
		// most keyboards that are not American.
		"btnkey": func(L *lua.LState) int {
			L.Push(lua.LString(r.label(L.OptInt(2, 0), button(L, 1))))
			return 1
		},

		// typed() is what was typed this tick, for a program asking for a name.
		"typed": func(L *lua.LState) int {
			L.Push(lua.LString(r.In.Text()))
			return 1
		},
	})
}

// defaultButtonLabel names a button's key by its place, for a runtime with no
// window behind it to ask about the keyboard.
func defaultButtonLabel(player, btn int) string {
	if player < 0 || player >= pico.Players || btn < 0 || btn >= pico.Buttons {
		return ""
	}
	keys := pico.ButtonKeys[player][btn]
	if len(keys) == 0 {
		return ""
	}
	return strings.ToUpper(keys[0])
}
