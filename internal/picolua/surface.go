package picolua

import (
	"fmt"
	"path/filepath"
	"strings"

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

		// grid() says how the surface is cut into sprites: the size of a cell
		// and how many there are, or nothing at all for a plain picture.
		// grid(w, h) cuts it, and gives the surface back so that the two can
		// be said in one breath.
		"grid": func(L *lua.LState) int {
			s := checkSurface(L, 1)
			if isNone(L, 2) {
				w, h, count := s.Grid()
				L.Push(lua.LNumber(w))
				L.Push(lua.LNumber(h))
				L.Push(lua.LNumber(count))
				return 3
			}
			s.SetGrid(coord(L, 2), optCoord(L, 3, coord(L, 2)))
			L.Push(L.Get(1))
			return 1
		},

		// flags(n) is the eight flags of a sprite of this sheet; flags(n, mask)
		// sets them. It is fget and fset for a sheet that is not the current
		// one.
		"flags": func(L *lua.LState) int {
			s := checkSurface(L, 1)
			n := L.CheckInt(2)
			if isNone(L, 3) {
				L.Push(lua.LNumber(s.Flags(n)))
				return 1
			}
			s.SetFlags(n, uint8(L.CheckInt(3)&0xff))
			L.Push(L.Get(1))
			return 1
		},

		// props(n) is everything else the artwork said about a sprite — what a
		// map editor lets you hang on a tile beyond its eight flags, such as a
		// material, a name or a number of hit points. props() on its own is
		// what the sheet says about every one of its sprites.
		//
		// A sprite's own beats the sheet's, and both are plain tables, so
		//
		//	if sheet:props(n).ice then slide() end
		//
		// works and so does walking them with pairs(). prop() is the same
		// question about one key, without making a table to ask it.
		"props": func(L *lua.LState) int {
			s := checkSurface(L, 1)
			if isNone(L, 2) {
				L.Push(propsOf(L, s.SheetProps()))
				return 1
			}
			L.Push(propsOf(L, s.Props(L.CheckInt(2))))
			return 1
		},

		// prop(n, key, [missing]) is one property of a sprite, or what to say
		// instead when it has none. prop(n, key, value) would be a setter, but
		// a third argument reads better as the answer for a tile that never
		// had one — set() does the writing:
		//
		//	sheet:prop(n, "material")            -- "ice", or nil
		//	sheet:prop(n, "damage", 0)           -- a number either way
		//	sheet:setprop(n, "material", "ice")  -- written onto the sprite
		"prop": func(L *lua.LState) int {
			s := checkSurface(L, 1)
			v, ok := s.Prop(L.CheckInt(2), L.CheckString(3))
			if !ok {
				L.Push(L.Get(4))
				return 1
			}
			L.Push(propValue(L, v))
			return 1
		},

		// setprop(n, key, value) writes one, over whatever the artwork said.
		// Giving nothing for the value puts the sheet's own answer back.
		"setprop": func(L *lua.LState) int {
			s := checkSurface(L, 1)
			s.SetProp(L.CheckInt(2), L.CheckString(3), goValue(L.Get(4)))
			L.Push(L.Get(1))
			return 1
		},

		// sprite(n) is one cell of a sheet as a surface of its own.
		//
		// It is a copy, which is the simple thing: changing it does not change
		// the sheet, and it can be drawn on. A window onto the sheet's own
		// pixels would save the copying and let target() draw into a cell, but
		// it would mean teaching every drawing loop about a row length that is
		// not the surface's width. If that is ever wanted, this is where it
		// goes.
		"sprite": func(L *lua.LState) int {
			s := checkSurface(L, 1)
			x, y, w, h, ok := s.Cell(L.CheckInt(2), optCoord(L, 3, 1), optCoord(L, 4, 1))
			if !ok {
				L.Push(lua.LNil)
				return 1
			}
			out := pico.NewSurface(w, h)
			for row := 0; row < h; row++ {
				for col := 0; col < w; col++ {
					out.Set(col, row, s.Get(x+col, y+row))
				}
			}
			L.Push(r.newSurface(out))
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
			s := pico.NewSurface(w, h)
			setGrid(L, s, 3)
			L.Push(r.newSurface(s))
			return 1
		},

		// sprite[[...]] reads a sprite written out as text: a hex digit per
		// pixel, '.' or a space for the transparent parts.
		"sprite": func(L *lua.LState) int {
			s, err := pico.ParseSprite(L.CheckString(1))
			if err != nil {
				L.RaiseError("%s", err.Error())
			}
			setGrid(L, s, 2)
			L.Push(r.newSurface(s))
			return 1
		},

		// loadpng(path) reads a PNG and reduces it to the palette in use, so the
		// same file under a different palette gives a different picture. Where
		// it reads from is the host's business: a game fused into a single
		// executable finds its artwork inside itself.
		"loadpng": func(L *lua.LState) int {
			path, data, err := r.find(kindImage, L.CheckString(1))
			if err == nil {
				var s *pico.Surface
				s, err = pico.DecodePNG(data, r.Vid.Palette())
				if err == nil {
					// A tileset file beside the picture says the same things
					// the picture does, where other tools can see them. It is
					// read first, so that what the artwork itself says wins.
					r.applyTileset(s, path)
					pngMeta := pico.ReadPNGMeta(data)
					s.Apply(pngMeta)
					setGrid(L, s, 2) // and what the program asks for wins over both
					L.Push(r.newSurface(s))
					return 1
				}
				err = fmt.Errorf("%s: %w", path, err)
			}
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		},

		// fetch(path) reads a file and gives it back: a level, a table of
		// numbers, whatever a game keeps beside itself. It reads the same way
		// loadpng does — out of a fused executable first, then from beside the
		// program — so a game does not care which it is.
		//
		// What store() saved is looked for first, and comes back as the table
		// or the string that was saved rather than as the text it was written
		// as. That way a game can ship a file of its own and have the player's
		// own version of it take over once there is one:
		//
		//	local settings = fetch("settings")   -- theirs, or the one shipped
		"fetch": func(L *lua.LState) int {
			name := L.CheckString(1)
			data, ok := r.readSaved(name)
			if !ok {
				var err error
				if _, data, err = r.find(kindData, name); err != nil {
					L.Push(lua.LNil)
					L.Push(lua.LString(err.Error()))
					return 2
				}
			}
			if stored(data) {
				v, err := decode(L, string(data))
				if err != nil {
					L.Push(lua.LNil)
					L.Push(lua.LString(fmt.Sprintf("%s: %v", name, err)))
					return 2
				}
				L.Push(v)
				return 1
			}
			L.Push(lua.LString(data))
			return 1
		},

		// spr draws a sprite, in whichever of the three ways it was asked.
		//
		//	spr(n, x, y, [w], [h], [flip_x], [flip_y], [turn])
		//	spr(sheet, n, x, y, [w], [h], [flip_x], [flip_y], [turn])
		//	spr(picture, x, y, [flip_x], [flip_y], [turn])
		//
		// The first is Picotron's own call. The second is the same thing with
		// the sheet said out loud, because there is more than one here. The
		// third is what a surface that was never cut into sprites means, and
		// there is no mistaking which is which: a sheet takes a sprite number
		// and a picture takes a place.
		//
		// turn mirrors the sprite across its own diagonal. With the two flips
		// it makes all eight ways of putting a sprite down, which is what a
		// map has always been able to ask for — it is a tile's flipd — and
		// what a program could not until now.
		"spr": func(L *lua.LState) int {
			sheet, at := r.spriteSheet(L, "spr")
			if sheet == nil {
				s := checkSurface(L, 1)
				r.Vid.Spr(s, coord(L, 2), coord(L, 3),
					pico.Flips(L.OptBool(4, false), L.OptBool(5, false), L.OptBool(6, false)))
				return 0
			}
			n := L.CheckInt(at)
			r.Vid.SprCell(sheet, n,
				coord(L, at+1), coord(L, at+2),
				optCoord(L, at+3, 1), optCoord(L, at+4, 1),
				pico.Flips(L.OptBool(at+5, false), L.OptBool(at+6, false), L.OptBool(at+7, false)))
			return 0
		},

		// sspr takes a rectangle of pixels — not of cells — and stretches it
		// to fit, either from a surface said out loud or from the current
		// sheet:
		//
		//	sspr(sx, sy, sw, sh, dx, dy, [dw, dh], [flip_x], [flip_y], [turn])
		//	sspr(sheet, sx, sy, sw, sh, dx, dy, [dw, dh], [flip_x], [flip_y], [turn])
		"sspr": func(L *lua.LState) int {
			if _, ok := L.Get(1).(*lua.LUserData); ok {
				return r.stretch(L, checkSurface(L, 1), 2)
			}
			return r.stretch(L, r.currentSheet(L, "sspr"), 1)
		},

		// usesheet(s) makes a sheet the one that spr(n, ...) and sget() mean,
		// the way Picotron has one spritesheet in hand at a time. It reports
		// the sheet it replaced, and usesheet() on its own asks without
		// changing anything.
		//
		// The name is not "sheet" because that is what anyone would call the
		// variable holding one, and a program that did would lose the call it
		// needed at the moment it needed it.
		"usesheet": func(L *lua.LState) int {
			was := r.currentValue
			if !isNone(L, 1) {
				s := checkSurface(L, 1)
				if !s.Gridded() {
					L.ArgError(1, "this surface is a picture, not a sheet: give it a grid first")
				}
				r.current, r.currentValue = s, L.Get(1)
			}
			L.Push(was)
			return 1
		},

		// fget(n) is all eight flags of a sprite as a number, and fget(n, bit)
		// is one of them as a true or a false. fset writes them the same two
		// ways. Both mean the current sheet, as sget and sset do.
		//
		// Flags are eight bits a sprite that mean whatever a game decides:
		// solid, water, deadly, a thing to pick up. They come with the artwork
		// rather than with the program, which is how a level can be walked on
		// without a table of tile numbers written out anywhere.
		"fget": func(L *lua.LState) int {
			sheet := r.currentSheet(L, "fget")
			n := L.CheckInt(1)
			if isNone(L, 2) {
				L.Push(lua.LNumber(sheet.Flags(n)))
				return 1
			}
			L.Push(lua.LBool(sheet.Flag(n, L.CheckInt(2))))
			return 1
		},

		"fset": func(L *lua.LState) int {
			sheet := r.currentSheet(L, "fset")
			n := L.CheckInt(1)
			if isNone(L, 3) {
				sheet.SetFlags(n, uint8(L.CheckInt(2)&0xff))
				return 0
			}
			sheet.SetFlag(n, L.CheckInt(2), lua.LVAsBool(L.Get(3)))
			return 0
		},

		// sget and sset read and write the pixels of the current sheet, which
		// is where a program keeps the artwork it is working on.
		"sget": func(L *lua.LState) int {
			L.Push(lua.LNumber(r.currentSheet(L, "sget").Get(coord(L, 1), coord(L, 2))))
			return 1
		},

		"sset": func(L *lua.LState) int {
			r.currentSheet(L, "sset").Set(coord(L, 1), coord(L, 2), r.optColor(L, 3, r.Vid.Pen()))
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

		// map draws a level, in either of the two ways there is one:
		//
		//	map([tx], [ty], [sx], [sy], [tw], [th], [flags])
		//	map(cells, sheet, [x], [y], [tile_w], [tile_h], [draw_zero])
		//
		// The first draws a window of the current map — the one usemap() was
		// given, loaded from a map editor — and is Picotron's own call. With
		// flags, only sprites carrying one of them are drawn, which is how one
		// layer of artwork becomes two of scenery.
		//
		// The second draws a grid of sprite numbers written out in the program
		// itself. A row is a table of numbers or a string of hex digits, so a
		// small level can be as readable in the source as a sprite is.
		//
		// The numbers are sprite numbers, counted from zero as everywhere
		// else. Sprite 0 is left undrawn unless the last argument asks for it,
		// which is what lets a dot in a level mean empty sky: leave the first
		// cell of the sheet blank and nothing else has to be said.
		//
		// Tiles are the size of the sheet's own cells unless told otherwise.
		"map": func(L *lua.LState) int {
			// A table is a grid of sprite numbers written out in the program;
			// anything else means the current map, which is Picotron's own
			// call. Both are useful: one for a level typed into the source,
			// one for a level drawn in an editor.
			if _, ok := L.Get(1).(*lua.LTable); !ok {
				r.drawMap(L, r.theMap(L, "map"), nil, 1)
				return 0
			}
			cells := L.CheckTable(1)
			sheet := checkSurface(L, 2)
			dx, dy := optCoord(L, 3, 0), optCoord(L, 4, 0)

			tw, th, _ := sheet.Grid()
			if tw == 0 {
				tw, th = 8, 8
			}
			tw, th = optCoord(L, 5, tw), optCoord(L, 6, th)
			if tw <= 0 || th <= 0 {
				L.ArgError(5, "a tile cannot be smaller than a pixel")
			}
			drawZero := L.OptBool(7, false)
			across := max(sheet.W/tw, 1)

			for row := 1; row <= cells.Len(); row++ {
				y := dy + (row-1)*th
				forEachTile(L, cells.RawGetInt(row), func(col, tile int) {
					if tile < 0 || (tile == 0 && !drawZero) {
						return
					}
					sx := (tile % across) * tw
					sy := (tile / across) * th
					r.Vid.SSpr(sheet, sx, sy, tw, th, dx+col*tw, y, tw, th, pico.Upright)
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

// stretch is the body of sspr, once it is known which surface is meant and
// where its own arguments start.
func (r *Runtime) stretch(L *lua.LState, s *pico.Surface, at int) int {
	sx, sy := coord(L, at), coord(L, at+1)
	sw, sh := coord(L, at+2), coord(L, at+3)
	dx, dy := coord(L, at+4), coord(L, at+5)
	r.Vid.SSpr(s, sx, sy, sw, sh, dx, dy,
		optCoord(L, at+6, sw), optCoord(L, at+7, sh),
		pico.Flips(L.OptBool(at+8, false), L.OptBool(at+9, false), L.OptBool(at+10, false)))
	return 0
}

// spriteSheet works out which of spr's three forms was used: it reports the
// sheet a sprite number is to be read from and where that number sits in the
// arguments, or nothing at all when a whole picture was handed over.
func (r *Runtime) spriteSheet(L *lua.LState, called string) (*pico.Surface, int) {
	switch L.Get(1).(type) {
	case lua.LNumber:
		return r.currentSheet(L, called), 1
	case *lua.LUserData:
		if s := checkSurface(L, 1); s.Gridded() {
			return s, 2
		}
		return nil, 0
	}
	L.ArgError(1, "a sprite number or a surface expected")
	return nil, 0
}

// currentSheet reports the sheet a program said to work on, complaining in a
// way that says what to do about it when there is none.
func (r *Runtime) currentSheet(L *lua.LState, called string) *pico.Surface {
	if r.current == nil {
		L.RaiseError("%s: there is no current sheet; hand one to sheet() first, "+
			"or name the one you mean", called)
	}
	return r.current
}

// setGrid reads an optional cell size from a call that makes a surface, so
// that a sheet can be cut as it is loaded.
func setGrid(L *lua.LState, s *pico.Surface, at int) {
	if isNone(L, at) {
		return
	}
	w := coord(L, at)
	s.SetGrid(w, optCoord(L, at+1, w))
}

// applyTileset reads the Tiled tileset that goes with a picture, when there is
// one: the same file with .tsj in place of .png.
//
// It is how a sheet exported for other tools keeps its sprite flags, and it is
// looked for quietly — a picture without one is the ordinary case, not a
// mistake.
func (r *Runtime) applyTileset(s *pico.Surface, pngPath string) {
	tsj := strings.TrimSuffix(pngPath, filepath.Ext(pngPath)) + ".tsj"
	data, err := r.read(tsj)
	if err != nil {
		return
	}
	meta, err := pico.ReadTSJ(data)
	if err != nil {
		return
	}
	s.Apply(meta)
}
