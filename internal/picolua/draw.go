package picolua

import (
	lua "github.com/yuin/gopher-lua"

	"tlua/internal/pico"
)

// installDrawing puts the graphics calls in place. Every one of them takes its
// colour last and optionally, and passing one sets the pen, so that a program
// can either name a colour every time or set it once with color().
func (r *Runtime) installDrawing() {
	r.register(map[string]lua.LGFunction{
		"cls": func(L *lua.LState) int {
			r.Vid.Cls(optColorIndex(L, 1, 0))
			return 0
		},

		"color": func(L *lua.LState) int {
			old := r.Vid.Color(optColorIndex(L, 1, 6))
			if !isNone(L, 2) {
				r.Vid.SetPenAlt(optColorIndex(L, 2, 0))
			}
			L.Push(lua.LNumber(old))
			return 1
		},

		"pset": func(L *lua.LState) int {
			r.Vid.Pset(coord(L, 1), coord(L, 2), r.penArg(L, 3))
			return 0
		},

		"pget": func(L *lua.LState) int {
			L.Push(lua.LNumber(r.Vid.Pget(coord(L, 1), coord(L, 2))))
			return 1
		},

		"line": func(L *lua.LState) int {
			r.Vid.Line(coord(L, 1), coord(L, 2), coord(L, 3), coord(L, 4), r.penArg(L, 5))
			return 0
		},

		"rect": func(L *lua.LState) int {
			r.Vid.Rect(coord(L, 1), coord(L, 2), coord(L, 3), coord(L, 4), r.penArg(L, 5))
			return 0
		},

		"rectfill": func(L *lua.LState) int {
			r.Vid.RectFill(coord(L, 1), coord(L, 2), coord(L, 3), coord(L, 4), r.penArg(L, 5))
			return 0
		},

		"circ": func(L *lua.LState) int {
			r.Vid.Circ(coord(L, 1), coord(L, 2), optCoord(L, 3, 4), r.penArg(L, 4))
			return 0
		},

		"circfill": func(L *lua.LState) int {
			r.Vid.CircFill(coord(L, 1), coord(L, 2), optCoord(L, 3, 4), r.penArg(L, 4))
			return 0
		},

		"oval": func(L *lua.LState) int {
			r.Vid.Oval(coord(L, 1), coord(L, 2), coord(L, 3), coord(L, 4), r.penArg(L, 5))
			return 0
		},

		"ovalfill": func(L *lua.LState) int {
			r.Vid.OvalFill(coord(L, 1), coord(L, 2), coord(L, 3), coord(L, 4), r.penArg(L, 5))
			return 0
		},

		"tri": func(L *lua.LState) int {
			r.Vid.Tri(coord(L, 1), coord(L, 2), coord(L, 3), coord(L, 4), coord(L, 5), coord(L, 6), r.penArg(L, 7))
			return 0
		},

		"trifill": func(L *lua.LState) int {
			r.Vid.TriFill(coord(L, 1), coord(L, 2), coord(L, 3), coord(L, 4), coord(L, 5), coord(L, 6), r.penArg(L, 7))
			return 0
		},

		// print(text), print(text, x, y) or print(text, x, y, colour). Without a
		// position it draws at the cursor and moves it down a line, so that
		// several prints in a row stack up.
		"print": func(L *lua.LState) int {
			text := tostr(L.CheckAny(1))
			cx, cy := r.Vid.CursorAt()
			switch {
			case isNone(L, 2) && isNone(L, 3):
				r.Vid.PrintLine(text, r.penArg(L, 4))
			default:
				r.Vid.Print(text, optCoord(L, 2, cx), optCoord(L, 3, cy), r.penArg(L, 4))
			}
			x, _ := r.Vid.CursorAt()
			L.Push(lua.LNumber(x + pico.TextWidth(text)))
			return 1
		},

		"cursor": func(L *lua.LState) int {
			x, y := r.Vid.CursorAt()
			r.Vid.Cursor(optCoord(L, 1, 0), optCoord(L, 2, 0))
			if !isNone(L, 3) {
				r.Vid.Color(optColorIndex(L, 3, 6))
			}
			L.Push(lua.LNumber(x))
			L.Push(lua.LNumber(y))
			return 2
		},

		"textwidth": func(L *lua.LState) int {
			L.Push(lua.LNumber(pico.TextWidth(tostr(L.CheckAny(1)))))
			return 1
		},

		"textheight": func(L *lua.LState) int {
			L.Push(lua.LNumber(pico.TextHeight(tostr(L.CheckAny(1)))))
			return 1
		},

		// camera() with no arguments puts the view back at the origin.
		"camera": func(L *lua.LState) int {
			x, y := r.Vid.Camera(optCoord(L, 1, 0), optCoord(L, 2, 0))
			L.Push(lua.LNumber(x))
			L.Push(lua.LNumber(y))
			return 2
		},

		// clip() with no arguments lifts clipping; with a fifth argument the
		// rectangle is narrowed to what is already in force.
		"clip": func(L *lua.LState) int {
			var old pico.Rect
			if isNone(L, 1) {
				old = r.Vid.ClipRect()
				r.Vid.ClipReset()
			} else {
				old = r.Vid.Clip(coord(L, 1), coord(L, 2), coord(L, 3), coord(L, 4), L.OptBool(5, false))
			}
			L.Push(lua.LNumber(old.X0))
			L.Push(lua.LNumber(old.Y0))
			L.Push(lua.LNumber(old.X1 - old.X0))
			L.Push(lua.LNumber(old.Y1 - old.Y0))
			return 4
		},

		// pal() puts both palettes back; pal(from, to) remaps a colour as it is
		// drawn; pal(from, to, 1) remaps the finished picture instead;
		// pal(table) applies a whole set of swaps at once.
		"pal": func(L *lua.LState) int {
			if isNone(L, 1) {
				r.Vid.ResetPal()
				return 0
			}
			if tbl, ok := L.Get(1).(*lua.LTable); ok {
				screen := L.OptInt(2, 0) == 1
				tbl.ForEach(func(k, v lua.LValue) {
					from, okFrom := k.(lua.LNumber)
					to, okTo := v.(lua.LNumber)
					if okFrom && okTo {
						r.Vid.Pal(toColorIndex(float64(from)), toColorIndex(float64(to)), screen)
					}
				})
				return 0
			}
			from := toColorIndex(float64(L.CheckNumber(1)))
			to := toColorIndex(float64(L.CheckNumber(2)))
			r.Vid.Pal(from, to, L.OptInt(3, 0) == 1)
			return 0
		},

		// palt() makes colour 0 the only transparent one again; palt(col, on)
		// chooses; palt(col) reads as "make it transparent".
		"palt": func(L *lua.LState) int {
			if isNone(L, 1) {
				r.Vid.ResetPal()
				return 0
			}
			r.Vid.Palt(toColorIndex(float64(L.CheckNumber(1))), L.OptBool(2, true))
			return 0
		},

		// fillp(pattern) dithers later fills with a 4x4 pattern, the top left
		// pixel being the highest bit. A second argument leaves the pattern's
		// set bits untouched instead of drawing them in the second pen colour;
		// fillp() turns dithering off.
		"fillp": func(L *lua.LState) int {
			pattern := uint16(0)
			if !isNone(L, 1) {
				pattern = uint16(int64(L.CheckNumber(1)) & 0xffff)
			}
			L.Push(lua.LNumber(r.Vid.Fillp(pattern, L.OptBool(2, false))))
			return 1
		},

		// screen() reports the size of the display, which is what centring
		// anything needs.
		"screen": func(L *lua.LState) int {
			L.Push(lua.LNumber(r.Vid.Screen.W))
			L.Push(lua.LNumber(r.Vid.Screen.H))
			return 2
		},
	})
}
