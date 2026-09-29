package picolua

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"

	"tlua/internal/pico"
)

// surfaceType is the name the surface metatable is registered under.
const surfaceType = "pico.surface"

// installSurfaceType gives surfaces their methods, so that a sprite sheet can
// answer for its own size and pixels.
func (r *Runtime) installSurfaceType() {
	mt := r.L.NewTypeMetatable(surfaceType)
	r.surfaceMeta = mt

	r.L.SetField(mt, "__index", r.L.SetFuncs(r.L.NewTable(), map[string]lua.LGFunction{
		"width":  func(L *lua.LState) int { L.Push(lua.LNumber(checkSurface(L, 1).W)); return 1 },
		"height": func(L *lua.LState) int { L.Push(lua.LNumber(checkSurface(L, 1).H)); return 1 },
		"size": func(L *lua.LState) int {
			s := checkSurface(L, 1)
			L.Push(lua.LNumber(s.W))
			L.Push(lua.LNumber(s.H))
			return 2
		},
		"get": func(L *lua.LState) int {
			L.Push(lua.LNumber(checkSurface(L, 1).Get(coord(L, 2), coord(L, 3))))
			return 1
		},
		"set": func(L *lua.LState) int {
			checkSurface(L, 1).Set(coord(L, 2), coord(L, 3), r.optColor(L, 4, r.Vid.Pen()))
			return 0
		},
		"fill": func(L *lua.LState) int {
			checkSurface(L, 1).Fill(r.optColor(L, 2, 0))
			return 0
		},
		"clone": func(L *lua.LState) int {
			L.Push(r.newSurface(checkSurface(L, 1).Clone()))
			return 1
		},
	}))

	r.L.SetField(mt, "__tostring", r.L.NewFunction(func(L *lua.LState) int {
		s := checkSurface(L, 1)
		L.Push(lua.LString(fmt.Sprintf("surface %dx%d", s.W, s.H)))
		return 1
	}))
}

// newSurface wraps a surface as a Lua value.
func (r *Runtime) newSurface(s *pico.Surface) *lua.LUserData {
	ud := r.L.NewUserData()
	ud.Value = s
	r.L.SetMetatable(ud, r.surfaceMeta)
	return ud
}

// checkSurface reads a surface argument, complaining in the way a Lua library
// should when it is given something else.
func checkSurface(L *lua.LState, n int) *pico.Surface {
	if ud, ok := L.Get(n).(*lua.LUserData); ok {
		if s, ok := ud.Value.(*pico.Surface); ok {
			return s
		}
	}
	L.ArgError(n, "surface expected")
	return nil
}

// installSurfaces adds the calls that make, draw and redirect surfaces.
func (r *Runtime) installSurfaces() {
	r.register(map[string]lua.LGFunction{
		// surface(w, h) is a blank sheet, every pixel colour 0 and so
		// transparent until something is drawn on it.
		"surface": func(L *lua.LState) int {
			w, h := coord(L, 1), coord(L, 2)
			if w < 0 || h < 0 || w*h > maxSurfacePixels {
				L.RaiseError("surface: %dx%d is not a size a surface can be", w, h)
			}
			L.Push(r.newSurface(pico.NewSurface(w, h)))
			return 1
		},

		// sprite[[...]] reads a sprite written out as text: a hex digit per
		// pixel, '.' or a space for the transparent parts.
		"sprite": func(L *lua.LState) int {
			s, err := pico.ParseSprite(L.CheckString(1))
			if err != nil {
				L.RaiseError("%s", err.Error())
			}
			L.Push(r.newSurface(s))
			return 1
		},

		// loadpng(path) reads a PNG and reduces it to the palette in use, so the
		// same file under a different palette gives a different picture.
		"loadpng": func(L *lua.LState) int {
			s, err := pico.LoadPNG(L.CheckString(1), r.Vid.Palette())
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			L.Push(r.newSurface(s))
			return 1
		},

		"spr": func(L *lua.LState) int {
			s := checkSurface(L, 1)
			r.Vid.Spr(s, coord(L, 2), coord(L, 3), L.OptBool(4, false), L.OptBool(5, false))
			return 0
		},

		// sspr(sheet, sx, sy, sw, sh, dx, dy, [dw, dh], [flip_x], [flip_y])
		// takes a rectangle of a sheet and stretches it to fit.
		"sspr": func(L *lua.LState) int {
			s := checkSurface(L, 1)
			sx, sy := coord(L, 2), coord(L, 3)
			sw, sh := coord(L, 4), coord(L, 5)
			dx, dy := coord(L, 6), coord(L, 7)
			r.Vid.SSpr(s, sx, sy, sw, sh, dx, dy,
				optCoord(L, 8, sw), optCoord(L, 9, sh),
				L.OptBool(10, false), L.OptBool(11, false))
			return 0
		},

		// target(surface) sends later drawing to that surface; target() sends it
		// back to the screen. Either way it reports what it replaced, so a
		// program can put things back as they were.
		"target": func(L *lua.LState) int {
			previous := r.currentTarget
			if isNone(L, 1) {
				r.Vid.SetTarget(nil)
				r.currentTarget = lua.LNil
			} else {
				r.Vid.SetTarget(checkSurface(L, 1))
				r.currentTarget = L.Get(1)
			}
			L.Push(previous)
			return 1
		},

		// map(cells, sheet, x, y, [tile_w, tile_h]) draws a grid of tiles cut
		// from a sheet. A row of cells is either a table of tile numbers or a
		// string of hex digits, and 0 means nothing at all, so a level can be
		// written out in the program as readably as a sprite can.
		"map": func(L *lua.LState) int {
			cells := L.CheckTable(1)
			sheet := checkSurface(L, 2)
			dx, dy := optCoord(L, 3, 0), optCoord(L, 4, 0)
			tw, th := optCoord(L, 5, 8), optCoord(L, 6, 8)
			if tw <= 0 || th <= 0 {
				L.ArgError(5, "a tile cannot be smaller than a pixel")
			}
			across := max(sheet.W/tw, 1)

			for row := 1; row <= cells.Len(); row++ {
				y := dy + (row-1)*th
				forEachTile(L, cells.RawGetInt(row), func(col, tile int) {
					if tile <= 0 {
						return
					}
					sx := ((tile - 1) % across) * tw
					sy := ((tile - 1) / across) * th
					r.Vid.SSpr(sheet, sx, sy, tw, th, dx+col*tw, y, tw, th, false, false)
				})
			}
			return 0
		},
	})
}

// maxSurfacePixels caps what a program can ask for in one go: sixteen million
// pixels is far more than any of this is for, and stops a typo in a size from
// asking the machine for all of its memory at once.
const maxSurfacePixels = 1 << 24

// forEachTile walks one row of a map, in whichever of the two shapes it was
// written.
func forEachTile(L *lua.LState, row lua.LValue, fn func(col, tile int)) {
	switch row := row.(type) {
	case lua.LString:
		for i, r := range string(row) {
			switch {
			case r == '.' || r == ' ':
				fn(i, 0)
			default:
				fn(i, hexDigit(r))
			}
		}
	case *lua.LTable:
		for i := 1; i <= row.Len(); i++ {
			if n, ok := row.RawGetInt(i).(lua.LNumber); ok {
				fn(i-1, int(n))
			}
		}
	}
}

// hexDigit reads one tile number from a map written as text, reporting nothing
// for a character that is not one.
func hexDigit(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 'a' && r <= 'f':
		return int(r-'a') + 10
	case r >= 'A' && r <= 'F':
		return int(r-'A') + 10
	}
	return 0
}
