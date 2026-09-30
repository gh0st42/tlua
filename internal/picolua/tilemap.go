package picolua

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"tlua/internal/pico"
)

// mapType is the name the map metatable is registered under.
const mapType = "pico.map"

// installMaps adds loading, drawing and reading a tile map.
func (r *Runtime) installMaps() {
	mt := r.L.NewTypeMetatable(mapType)
	r.mapMeta = mt

	r.L.SetField(mt, "__index", r.L.SetFuncs(r.L.NewTable(), map[string]lua.LGFunction{
		// size() is the map in cells, tile() the size of one of them.
		"size": func(L *lua.LState) int {
			m := checkMap(L, 1)
			L.Push(lua.LNumber(m.W))
			L.Push(lua.LNumber(m.H))
			return 2
		},
		"tile": func(L *lua.LState) int {
			m := checkMap(L, 1)
			L.Push(lua.LNumber(m.TileW))
			L.Push(lua.LNumber(m.TileH))
			return 2
		},

		// layers() names them in order, so that a program can look at a map it
		// was not written for.
		"layers": func(L *lua.LState) int {
			m := checkMap(L, 1)
			out := L.NewTable()
			for _, l := range m.Layers {
				out.Append(lua.LString(l.Name))
			}
			L.Push(out)
			return 1
		},

		// layer(which) is one of them, by name or by its place counted from
		// one. Both, because a map made in an editor has names worth using and
		// a map made in a hurry does not.
		"layer": func(L *lua.LState) int {
			l := r.findLayer(L, checkMap(L, 1), 2)
			if l == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(r.newLayer(checkMap(L, 1), l))
			return 1
		},

		// objects(which) is what was placed on a layer, as plain tables.
		"objects": func(L *lua.LState) int {
			L.Push(objectsOf(L, r.findLayer(L, checkMap(L, 1), 2)))
			return 1
		},

		// sheet(n) is one of the sheets the map draws with, counted from one,
		// which is what an object's sprite is numbered against.
		"sheet": func(L *lua.LState) int {
			m := checkMap(L, 1)
			n := L.OptInt(2, 1)
			if n < 1 || n > len(m.Tilesets) || m.Tilesets[n-1].Sheet == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(r.newSurface(m.Tilesets[n-1].Sheet))
			return 1
		},

		// sheets() is how many of them there are.
		"sheets": func(L *lua.LState) int {
			L.Push(lua.LNumber(len(checkMap(L, 1).Tilesets)))
			return 1
		},

		// props() is what the map itself was labelled with in the editor.
		"props": func(L *lua.LState) int {
			L.Push(propsOf(L, checkMap(L, 1).Props))
			return 1
		},

		// draw(tx, ty, sx, sy, tw, th, [flags]) is map() for a map that is not
		// the current one.
		"draw": func(L *lua.LState) int {
			m := checkMap(L, 1)
			r.drawMap(L, m, nil, 2)
			return 0
		},
	}))

	r.L.SetField(mt, "__tostring", r.L.NewFunction(func(L *lua.LState) int {
		m := checkMap(L, 1)
		L.Push(lua.LString(fmt.Sprintf("map %dx%d of %dx%d tiles, %d layers",
			m.W, m.H, m.TileW, m.TileH, len(m.Layers))))
		return 1
	}))

	// A layer of a map, which can be drawn and read on its own.
	lmt := r.L.NewTypeMetatable(mapType + ".layer")
	r.layerMeta = lmt
	r.L.SetField(lmt, "__index", r.L.SetFuncs(r.L.NewTable(), map[string]lua.LGFunction{
		"name": func(L *lua.LState) int {
			L.Push(lua.LString(checkLayer(L, 1).layer.Name))
			return 1
		},
		"size": func(L *lua.LState) int {
			l := checkLayer(L, 1).layer
			L.Push(lua.LNumber(l.W))
			L.Push(lua.LNumber(l.H))
			return 2
		},
		"visible": func(L *lua.LState) int {
			l := checkLayer(L, 1)
			if !isNone(L, 2) {
				l.layer.Visible = lua.LVAsBool(L.Get(2))
			}
			L.Push(lua.LBool(l.layer.Visible))
			return 1
		},
		"objects": func(L *lua.LState) int {
			L.Push(objectsOf(L, checkLayer(L, 1).layer))
			return 1
		},
		"get": func(L *lua.LState) int {
			l := checkLayer(L, 1)
			L.Push(lua.LNumber(l.layer.At(coord(L, 2), coord(L, 3)).Sprite))
			return 1
		},
		"set": func(L *lua.LState) int {
			l := checkLayer(L, 1)
			l.layer.Set(coord(L, 2), coord(L, 3), L.CheckInt(4))
			return 0
		},
		"draw": func(L *lua.LState) int {
			l := checkLayer(L, 1)
			r.drawMap(L, l.owner, l.layer, 2)
			return 0
		},
	}))

	r.register(map[string]lua.LGFunction{
		// loadmap(name) reads a Tiled map and everything it draws with: the
		// tilesets it names, and the pictures those name in turn.
		"loadmap": func(L *lua.LState) int {
			m, err := r.loadMap(L.CheckString(1))
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			L.Push(r.newMap(m))
			return 1
		},

		// usemap(m) makes a map the one map(), mget() and mset() mean, as
		// usesheet() does for sprites.
		"usemap": func(L *lua.LState) int {
			was := r.currentMapValue
			if !isNone(L, 1) {
				r.currentMap, r.currentMapValue = checkMap(L, 1), L.Get(1)
			}
			L.Push(was)
			return 1
		},

		// mget(x, y, [layer]) is the sprite in a square of the current map,
		// and mset puts one there. Without a layer they mean the first one,
		// which is the only one most maps have.
		"mget": func(L *lua.LState) int {
			m := r.theMap(L, "mget")
			l := r.layerOr(L, m, 3, 0)
			if l == nil {
				L.Push(lua.LNumber(0))
				return 1
			}
			L.Push(lua.LNumber(l.At(coord(L, 1), coord(L, 2)).Sprite))
			return 1
		},

		"mset": func(L *lua.LState) int {
			m := r.theMap(L, "mset")
			if l := r.layerOr(L, m, 4, 0); l != nil {
				l.Set(coord(L, 1), coord(L, 2), L.CheckInt(3))
			}
			return 0
		},
	})
}

/* --- the Lua values --- */

// mapLayer is a layer with the map it belongs to, since drawing one needs the
// map's sheets and tile size.
type mapLayer struct {
	owner *pico.Tilemap
	layer *pico.Layer
}

func (r *Runtime) newMap(m *pico.Tilemap) *lua.LUserData {
	ud := r.L.NewUserData()
	ud.Value = m
	r.L.SetMetatable(ud, r.mapMeta)
	return ud
}

func (r *Runtime) newLayer(m *pico.Tilemap, l *pico.Layer) *lua.LUserData {
	ud := r.L.NewUserData()
	ud.Value = &mapLayer{owner: m, layer: l}
	r.L.SetMetatable(ud, r.layerMeta)
	return ud
}

func checkMap(L *lua.LState, n int) *pico.Tilemap {
	if ud, ok := L.Get(n).(*lua.LUserData); ok {
		if m, ok := ud.Value.(*pico.Tilemap); ok {
			return m
		}
	}
	L.ArgError(n, "map expected")
	return nil
}

func checkLayer(L *lua.LState, n int) *mapLayer {
	if ud, ok := L.Get(n).(*lua.LUserData); ok {
		if l, ok := ud.Value.(*mapLayer); ok {
			return l
		}
	}
	L.ArgError(n, "map layer expected")
	return nil
}

// findLayer reads a layer argument, which is a name or a number counted from
// one, and reports nothing for a layer that is not there.
func (r *Runtime) findLayer(L *lua.LState, m *pico.Tilemap, at int) *pico.Layer {
	switch v := L.Get(at).(type) {
	case lua.LString:
		return m.Layer(string(v))
	case lua.LNumber:
		return m.LayerAt(int(v))
	}
	L.ArgError(at, "a layer name or number expected")
	return nil
}

// layerOr reports the layer an optional argument names, or the nth tile layer
// when it was left out.
func (r *Runtime) layerOr(L *lua.LState, m *pico.Tilemap, at, fallback int) *pico.Layer {
	if !isNone(L, at) {
		return r.findLayer(L, m, at)
	}
	tiles := m.Tiles()
	if fallback < len(tiles) {
		return tiles[fallback]
	}
	return nil
}

// theMap reports the current map, saying what to do when there is none.
func (r *Runtime) theMap(L *lua.LState, called string) *pico.Tilemap {
	if r.currentMap == nil {
		L.RaiseError("%s: there is no current map; hand one to usemap() first", called)
	}
	return r.currentMap
}

// objectsOf turns what was placed on a layer into a list of plain tables.
func objectsOf(L *lua.LState, l *pico.Layer) *lua.LTable {
	out := L.NewTable()
	if l == nil {
		return out
	}
	for _, o := range l.Objects {
		t := L.NewTable()
		L.SetField(t, "id", lua.LNumber(o.ID))
		L.SetField(t, "name", lua.LString(o.Name))
		L.SetField(t, "class", lua.LString(o.Class))
		L.SetField(t, "x", lua.LNumber(o.X))
		L.SetField(t, "y", lua.LNumber(o.Y))
		L.SetField(t, "w", lua.LNumber(o.W))
		L.SetField(t, "h", lua.LNumber(o.H))
		L.SetField(t, "rotation", lua.LNumber(o.Rotation))
		L.SetField(t, "visible", lua.LBool(o.Visible))
		L.SetField(t, "shape", lua.LString(o.Shape.String()))
		if len(o.Points) > 0 {
			corners := L.NewTable()
			for _, p := range o.Points {
				corner := L.NewTable()
				L.SetField(corner, "x", lua.LNumber(p.X))
				L.SetField(corner, "y", lua.LNumber(p.Y))
				corners.Append(corner)
			}
			L.SetField(t, "points", corners)
		}
		// A piece of scenery placed as an object carries a sprite, and the
		// sheet it belongs to, so that it can be drawn: spr(m:sheet(o.sheet),
		// o.sprite, o.x, o.y - h). An object that is only a region has none of
		// these fields at all, which is how a program tells them apart.
		if o.HasTile {
			L.SetField(t, "sprite", lua.LNumber(o.Tile.Sprite))
			L.SetField(t, "sheet", lua.LNumber(int(o.Tile.Sheet)+1))
			L.SetField(t, "flipx", lua.LBool(o.Tile.Turn&pico.FlipX != 0))
			L.SetField(t, "flipy", lua.LBool(o.Tile.Turn&pico.FlipY != 0))
			L.SetField(t, "flipd", lua.LBool(o.Tile.Turn&pico.Turn != 0))
		}
		L.SetField(t, "props", propsOf(L, o.Props))
		out.Append(t)
	}
	return out
}

// propsOf turns the labels an editor puts on things into a table, keeping the
// kind of value each one was given.
func propsOf(L *lua.LState, props map[string]any) *lua.LTable {
	out := L.NewTable()
	for name, v := range props {
		L.SetField(out, name, propValue(L, v))
	}
	return out
}

// propValue turns one value from a file into a Lua one. An editor writes
// strings, numbers and truths; a grouped property is a table of them, and a
// list is a table too, so both go through whole rather than disappearing.
func propValue(L *lua.LState, v any) lua.LValue {
	switch v := v.(type) {
	case string:
		return lua.LString(v)
	case float64:
		return lua.LNumber(v)
	case bool:
		return lua.LBool(v)
	case map[string]any:
		return propsOf(L, v)
	case []any:
		out := L.NewTable()
		for _, item := range v {
			out.Append(propValue(L, item))
		}
		return out
	}
	return lua.LNil
}

// goValue is propValue the other way about, for a property a program sets
// itself. Only the three kinds an editor writes are kept: a table would have to
// be copied, and a property is meant to be a label rather than a structure.
func goValue(v lua.LValue) any {
	switch v := v.(type) {
	case lua.LString:
		return string(v)
	case lua.LNumber:
		return float64(v)
	case lua.LBool:
		return bool(v)
	}
	return nil
}

/* --- drawing --- */

// drawMap is the body of map(), m:draw() and layer:draw(): a window of a map,
// or of one of its layers, wherever the arguments start.
//
//	(tx, ty, sx, sy, tw, th, [flags])
//
// All of them may be left out, and then the whole map is drawn at the origin,
// which is what a small game wants and what map() on its own does.
func (r *Runtime) drawMap(L *lua.LState, m *pico.Tilemap, only *pico.Layer, at int) {
	if m == nil {
		return
	}
	tx, ty := optCoord(L, at, 0), optCoord(L, at+1, 0)
	sx, sy := optCoord(L, at+2, 0), optCoord(L, at+3, 0)
	tw, th := optCoord(L, at+4, m.W-tx), optCoord(L, at+5, m.H-ty)
	flags := uint8(L.OptInt(at+6, 0) & 0xff)

	if only != nil {
		r.Vid.DrawLayer(m, only, tx, ty, sx, sy, tw, th, flags)
		return
	}
	r.Vid.DrawMap(m, tx, ty, sx, sy, tw, th, flags)
}

/* --- loading --- */

// loadMap reads a map and the artwork it draws with.
//
// A map names its tilesets, and each tileset names its picture; both are looked
// for the way every other resource is, so a map found in assets/maps finds its
// sheets in assets/gfx without either file having to say so.
func (r *Runtime) loadMap(name string) (*pico.Tilemap, error) {
	mapPath, data, err := r.find(kindMap, name)
	if err != nil {
		return nil, err
	}
	m, err := pico.ReadTMJ(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", mapPath, err)
	}

	for i := range m.Tilesets {
		sheet, err := r.loadTileset(m.Tilesets[i].Source, path.Dir(filepath.ToSlash(mapPath)))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", mapPath, err)
		}
		m.Tilesets[i].Sheet = sheet
	}
	if len(m.Tilesets) == 0 {
		return nil, fmt.Errorf("%s: the map names no tilesets to draw with", mapPath)
	}
	return m, nil
}

// loadTileset opens one of a map's tilesets, and the picture it names.
func (r *Runtime) loadTileset(source, beside string) (*pico.Surface, error) {
	tsjPath, data, err := r.findBeside(kindTileset, source, beside)
	if err != nil {
		return nil, err
	}
	meta, err := pico.ReadTSJ(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tsjPath, err)
	}
	image, err := pico.TilesetImage(data)
	if err != nil || image == "" {
		return nil, fmt.Errorf("%s: the tileset names no picture", tsjPath)
	}

	_, png, err := r.findBeside(kindImage, image, path.Dir(filepath.ToSlash(tsjPath)))
	if err != nil {
		return nil, err
	}
	sheet, err := pico.DecodePNG(png, r.Vid.Palette())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", image, err)
	}
	// What the tileset says, then what the picture says over the top of it.
	sheet.Apply(meta)
	sheet.Apply(pico.ReadPNGMeta(png))
	return sheet, nil
}

// findBeside looks for a file the way any resource is looked for, and also
// beside the file that named it — which is how a map's tileset, written as
// "tiles.tsj", is found next to the map.
func (r *Runtime) findBeside(k kind, name, dir string) (string, []byte, error) {
	if dir != "" && dir != "." && !filepath.IsAbs(name) {
		beside := path.Join(dir, name)
		if data, err := r.read(beside); err == nil {
			return beside, data, nil
		}
	}
	return r.find(k, strings.TrimPrefix(name, "./"))
}
