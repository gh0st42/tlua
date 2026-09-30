package pico

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
)

// Meta is what a sheet's artwork says about itself: how big its sprites are,
// which flags they carry, and whatever else the editor labelled them with.
//
// Sprite flags are eight bits a sprite that mean whatever a game decides —
// solid, water, deadly, collectable. They belong with the artwork rather than
// with the program, because whoever drew the tile is who knows what it is, and
// every tool in this lineage stores them that way.
type Meta struct {
	TileW, TileH int
	Flags        map[int]uint8

	// Props is what a tileset said about each sprite beyond its flags, and
	// Sheet is what it said about itself for every sprite to inherit. A PNG
	// carries neither: they come from a .tsj, where an editor can give a tile
	// a material, a name or a number.
	Props map[int]map[string]any
	Sheet map[string]any
}

// Empty reports whether the metadata says anything at all.
func (m Meta) Empty() bool {
	return m.TileW <= 0 && len(m.Flags) == 0 && len(m.Props) == 0 && len(m.Sheet) == 0
}

/* --- what a PNG carries --- */

// metaKey is the PNG text chunk sprite flags travel in. The name is the one fz
// writes, since that is the editor these sheets come out of.
const metaKey = "fz_meta"

// pngMeta is the JSON inside that chunk: a tile size, and a mask per sprite
// keyed by its number written out as a string.
type pngMeta struct {
	TileSize int               `json:"tile_size"`
	Flags    map[string]uint8  `json:"flags"`
	Extra    map[string]string `json:"-"`
}

// ReadPNGMeta reports what a PNG says about its own sprites, in the text chunk
// a sheet editor leaves there. A PNG without one says nothing, which is not an
// error: most PNGs are pictures rather than sheets.
func ReadPNGMeta(data []byte) Meta {
	raw, ok := pngTextChunk(data, metaKey)
	if !ok {
		return Meta{}
	}
	var m pngMeta
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return Meta{}
	}

	out := Meta{TileW: m.TileSize, TileH: m.TileSize}
	if len(m.Flags) > 0 {
		out.Flags = make(map[int]uint8, len(m.Flags))
		for key, mask := range m.Flags {
			n, err := strconv.Atoi(key)
			if err != nil || n < 0 {
				continue
			}
			out.Flags[n] = mask
		}
	}
	return out
}

// pngTextChunk reports the text a PNG carries under a key.
//
// The image decoder throws these away, so the file is walked again here. A PNG
// is a signature and then a run of chunks, each a length, a four letter type,
// the bytes, and a checksum; a tEXt chunk is a key, a zero byte, and the text.
func pngTextChunk(data []byte, key string) (string, bool) {
	const signature = "\x89PNG\r\n\x1a\n"
	if len(data) < len(signature) || string(data[:len(signature)]) != signature {
		return "", false
	}

	for pos := len(signature); pos+12 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[pos:]))
		if n < 0 || pos+12+n > len(data) {
			return "", false // a length that runs off the end: give up quietly
		}
		typ := string(data[pos+4 : pos+8])
		body := data[pos+8 : pos+8+n]

		if typ == "tEXt" {
			if i := bytes.IndexByte(body, 0); i >= 0 && string(body[:i]) == key {
				return string(body[i+1:]), true
			}
		}
		if typ == "IEND" {
			break
		}
		pos += 12 + n
	}
	return "", false
}

/* --- what a Tiled tileset carries --- */

// tilesetJSON is the part of a Tiled .tsj that matters here: how big the tiles
// are, and the properties hung on each of them.
type tilesetJSON struct {
	TileWidth  int            `json:"tilewidth"`
	TileHeight int            `json:"tileheight"`
	Properties []propertyJSON `json:"properties"`
	Tiles      []struct {
		ID         int            `json:"id"`
		Properties []propertyJSON `json:"properties"`
	} `json:"tiles"`
}

// flagProperty is what a sprite flag is called in a Tiled tileset: flag_0 to
// flag_7, each a boolean that is there when the bit is set.
const flagProperty = "flag_"

// ReadTSJ reports what a Tiled tileset file says about a sheet's sprites.
//
// It is the same information the PNG carries, written where other tools can
// see it: a sheet editor exports one from the other. Reading both means a sheet
// works whichever of the two came along with it.
func ReadTSJ(data []byte) (Meta, error) {
	var ts tilesetJSON
	if err := json.Unmarshal(data, &ts); err != nil {
		return Meta{}, fmt.Errorf("not a Tiled tileset: %w", err)
	}

	out := Meta{TileW: ts.TileWidth, TileH: ts.TileHeight, Sheet: properties(ts.Properties)}
	for _, tile := range ts.Tiles {
		var mask uint8
		for _, p := range tile.Properties {
			bit, ok := flagBit(p.Name)
			if !ok {
				continue
			}
			if on, ok := p.Value.(bool); ok && on {
				mask |= 1 << uint(bit)
			}
		}
		if mask != 0 {
			if out.Flags == nil {
				out.Flags = map[int]uint8{}
			}
			out.Flags[tile.ID] = mask
		}
		// The flags are kept as properties as well as a mask, since that is
		// how the file has them and a game may prefer to ask by name.
		if props := properties(tile.Properties); props != nil {
			if out.Props == nil {
				out.Props = map[int]map[string]any{}
			}
			out.Props[tile.ID] = props
		}
	}
	return out, nil
}

// flagBit reads the bit number out of a property called flag_3.
func flagBit(name string) (int, bool) {
	if len(name) <= len(flagProperty) || name[:len(flagProperty)] != flagProperty {
		return 0, false
	}
	bit, err := strconv.Atoi(name[len(flagProperty):])
	if err != nil || bit < 0 || bit > 7 {
		return 0, false
	}
	return bit, true
}

// Apply puts metadata onto a surface: the grid, unless it already has one that
// was asked for outright, and the flags and properties on top of whatever is
// there.
func (s *Surface) Apply(m Meta) {
	if !s.Gridded() && m.TileW > 0 && m.TileH > 0 {
		s.SetGrid(m.TileW, m.TileH)
	}
	for n, mask := range m.Flags {
		s.SetFlags(n, mask)
	}
	for key, v := range m.Sheet {
		s.SetSheetProp(key, v)
	}
	for n, props := range m.Props {
		for key, v := range props {
			s.SetProp(n, key, v)
		}
	}
}
