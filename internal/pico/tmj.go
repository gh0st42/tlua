package pico

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// Reading Tiled's own map format, which is what every map editor in this
// lineage can write.

// How a map editor packs the three ways a tile can be turned into the top bits
// of its number.
const (
	flippedH = 0x80000000
	flippedV = 0x40000000
	flippedD = 0x20000000
	gidMask  = 0x0fffffff
)

// mapJSON is the part of a .tmj that matters here.
type mapJSON struct {
	Width      int `json:"width"`
	Height     int `json:"height"`
	TileWidth  int `json:"tilewidth"`
	TileHeight int `json:"tileheight"`

	Layers []struct {
		Name        string          `json:"name"`
		Type        string          `json:"type"`
		Visible     *bool           `json:"visible"`
		Width       int             `json:"width"`
		Height      int             `json:"height"`
		Data        json.RawMessage `json:"data"`
		Encoding    string          `json:"encoding"`
		Compression string          `json:"compression"`
		Objects     []objectJSON    `json:"objects"`
	} `json:"layers"`

	Tilesets []struct {
		FirstGID int    `json:"firstgid"`
		Source   string `json:"source"`
	} `json:"tilesets"`

	Properties []propertyJSON `json:"properties"`
}

type objectJSON struct {
	ID         int            `json:"id"`
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Class      string         `json:"class"`
	X          float64        `json:"x"`
	Y          float64        `json:"y"`
	Width      float64        `json:"width"`
	Height     float64        `json:"height"`
	Rotation   float64        `json:"rotation"`
	Visible    *bool          `json:"visible"`
	GID        *uint32        `json:"gid"`
	Ellipse    bool           `json:"ellipse"`
	Point      bool           `json:"point"`
	Polygon    []pointJSON    `json:"polygon"`
	Polyline   []pointJSON    `json:"polyline"`
	Properties []propertyJSON `json:"properties"`
}

type pointJSON struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type propertyJSON struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

// ReadTMJ reads a Tiled map. The tilesets it names are left for the caller to
// find and open, since where a file lives is not the console's business.
func ReadTMJ(data []byte) (*Tilemap, error) {
	var raw mapJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("not a Tiled map: %w", err)
	}
	if raw.TileWidth <= 0 || raw.TileHeight <= 0 {
		return nil, fmt.Errorf("the map does not say how big its tiles are")
	}

	m := &Tilemap{
		W: raw.Width, H: raw.Height,
		TileW: raw.TileWidth, TileH: raw.TileHeight,
		Props: properties(raw.Properties),
	}
	for _, ts := range raw.Tilesets {
		m.Tilesets = append(m.Tilesets, Tileset{FirstGID: ts.FirstGID, Source: ts.Source})
	}

	for _, l := range raw.Layers {
		layer := &Layer{Name: l.Name, W: l.Width, H: l.Height, Visible: true}
		if l.Visible != nil {
			layer.Visible = *l.Visible
		}

		switch l.Type {
		case "objectgroup":
			layer.Kind = ObjectLayer
			for _, o := range l.Objects {
				layer.Objects = append(layer.Objects, m.objectOf(o))
			}

		case "tilelayer":
			gids, err := layerData(l.Data, l.Encoding, l.Compression, l.Width*l.Height)
			if err != nil {
				return nil, fmt.Errorf("layer %q: %w", l.Name, err)
			}
			layer.Cells = make([]Cell, len(gids))
			for i, gid := range gids {
				layer.Cells[i] = m.cellOf(gid)
			}

		default:
			continue // a group or an image layer: nothing to draw from here
		}
		m.Layers = append(m.Layers, layer)
	}
	return m, nil
}

// objectOf reads one thing placed on a layer.
func (m *Tilemap) objectOf(o objectJSON) Object {
	class := o.Class
	if class == "" {
		class = o.Type // what older files call it
	}

	out := Object{
		ID: o.ID, Name: o.Name, Class: class,
		X: o.X, Y: o.Y, W: o.Width, H: o.Height,
		Rotation: o.Rotation / 360, // the editor writes degrees; angles here are turns
		Visible:  o.Visible == nil || *o.Visible,
		Props:    properties(o.Properties),
	}

	switch {
	case o.Point:
		out.Shape = ShapePoint
	case o.Ellipse:
		out.Shape = ShapeEllipse
	case len(o.Polygon) > 0:
		out.Shape, out.Points = ShapePolygon, points(o.Polygon)
	case len(o.Polyline) > 0:
		out.Shape, out.Points = ShapePolyline, points(o.Polyline)
	}

	// An object given a tile carries the same number, flip bits and all, that
	// a square of a tile layer does.
	if o.GID != nil {
		out.Tile, out.HasTile = m.cellOf(*o.GID), true
	}
	return out
}

// points turns an outline's corners into the console's own.
func points(list []pointJSON) []Point {
	out := make([]Point, len(list))
	for i, p := range list {
		out[i] = Point{X: p.X, Y: p.Y}
	}
	return out
}

// cellOf turns one of the map's tile numbers into a cell: which sheet it comes
// from, which sprite of that sheet, and which way round.
func (m *Tilemap) cellOf(gid uint32) Cell {
	var c Cell
	if gid&flippedH != 0 {
		c.Turn |= FlipX
	}
	if gid&flippedV != 0 {
		c.Turn |= FlipY
	}
	if gid&flippedD != 0 {
		c.Turn |= Turn
	}

	id := int(gid & gidMask)
	if id == 0 {
		return Cell{} // nothing here
	}
	// The sheet is the last one whose numbering starts at or below this tile.
	for i, ts := range m.Tilesets {
		if id >= ts.FirstGID {
			c.Sheet = uint8(i)
			c.Sprite = int32(id - ts.FirstGID)
		}
	}
	return c
}

// layerData reads a layer's tile numbers, in whichever of the four ways the
// editor wrote them.
func layerData(raw json.RawMessage, encoding, compression string, want int) ([]uint32, error) {
	switch encoding {
	case "", "csv":
		var out []uint32
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("reading the tiles: %w", err)
		}
		return out, nil

	case "base64":
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, fmt.Errorf("reading the tiles: %w", err)
		}
		packed, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			return nil, fmt.Errorf("reading the tiles: %w", err)
		}
		if packed, err = decompress(packed, compression); err != nil {
			return nil, err
		}
		if len(packed) < want*4 {
			return nil, fmt.Errorf("there are %d tiles of data for %d squares", len(packed)/4, want)
		}
		out := make([]uint32, want)
		for i := range out {
			out[i] = binary.LittleEndian.Uint32(packed[i*4:])
		}
		return out, nil
	}
	return nil, fmt.Errorf("the tiles are written as %q, which is not something this reads", encoding)
}

// decompress unpacks layer data. Tiled writes zlib or gzip, both of which the
// standard library has; zstd would be another dependency, and is named here so
// that a file using it says so rather than looking corrupt.
func decompress(data []byte, how string) ([]byte, error) {
	var (
		r   io.ReadCloser
		err error
	)
	switch how {
	case "":
		return data, nil
	case "zlib":
		r, err = zlib.NewReader(bytes.NewReader(data))
	case "gzip":
		r, err = gzip.NewReader(bytes.NewReader(data))
	case "zstd":
		return nil, fmt.Errorf("the map is compressed with zstd, which this does not read: save it as zlib, gzip or uncompressed")
	default:
		return nil, fmt.Errorf("the map is compressed with %q, which this does not read", how)
	}
	if err != nil {
		return nil, fmt.Errorf("unpacking the tiles: %w", err)
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("unpacking the tiles: %w", err)
	}
	return out, nil
}

// properties turns a list of named values into a table, keeping whatever type
// the editor gave each one.
func properties(list []propertyJSON) map[string]any {
	if len(list) == 0 {
		return nil
	}
	out := make(map[string]any, len(list))
	for _, p := range list {
		out[p.Name] = p.Value
	}
	return out
}

// TilesetImage reports the picture a Tiled tileset names, so that a map can
// find the artwork its tilesets point at.
func TilesetImage(data []byte) (string, error) {
	var ts struct {
		Image string `json:"image"`
	}
	if err := json.Unmarshal(data, &ts); err != nil {
		return "", fmt.Errorf("not a Tiled tileset: %w", err)
	}
	return ts.Image, nil
}
