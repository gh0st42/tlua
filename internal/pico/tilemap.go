package pico

// A tile map: a grid of sprites, in layers, with things placed on it.
//
// This is what a map editor produces and what a game walks around in. The
// console does not care where it came from — package pico reads Tiled's own
// format in tmj.go, and a map built by hand is the same thing.

// Cell is one square of a layer: which sprite of which sheet, and how it is
// turned.
//
// Sprite 0 is nothing. A map editor means "no tile" by it and this console
// means "the blank first cell of the sheet" by it, and since a sheet's first
// cell is left blank precisely so that a level can say nothing, the two agree.
type Cell struct {
	Sprite int32
	Sheet  uint8
	Turn   Orientation
}

// Empty reports whether there is nothing in this square.
func (c Cell) Empty() bool { return c.Sprite == 0 }

// LayerKind tells the two sorts of layer apart.
type LayerKind uint8

const (
	TileLayer LayerKind = iota
	ObjectLayer
)

// Layer is one sheet of the map: either a grid of cells or a list of things
// placed on it.
type Layer struct {
	Name    string
	Kind    LayerKind
	Visible bool

	W, H  int    // in cells, for a tile layer
	Cells []Cell // W*H, row major

	Objects []Object
}

// At reports the cell at a place, and nothing at all outside the layer.
func (l *Layer) At(x, y int) Cell {
	if l.Kind != TileLayer || x < 0 || y < 0 || x >= l.W || y >= l.H {
		return Cell{}
	}
	return l.Cells[y*l.W+x]
}

// Set puts a sprite in a square, leaving the sheet and the turn it had.
func (l *Layer) Set(x, y int, sprite int) {
	if l.Kind != TileLayer || x < 0 || y < 0 || x >= l.W || y >= l.H {
		return
	}
	l.Cells[y*l.W+x].Sprite = int32(sprite)
}

// SetCell puts a whole cell in a square.
func (l *Layer) SetCell(x, y int, c Cell) {
	if l.Kind != TileLayer || x < 0 || y < 0 || x >= l.W || y >= l.H {
		return
	}
	l.Cells[y*l.W+x] = c
}

// Shape is what an object was drawn as in the editor. A rectangle is the usual
// one; the others are for a spot to stand at, a round area, or an outline that
// follows the scenery.
type Shape uint8

const (
	ShapeRect Shape = iota
	ShapeEllipse
	ShapePoint    // a place with no size
	ShapePolygon  // a closed outline
	ShapePolyline // an open one
)

// String names a shape the way a program would ask for it.
func (s Shape) String() string {
	switch s {
	case ShapeEllipse:
		return "ellipse"
	case ShapePoint:
		return "point"
	case ShapePolygon:
		return "polygon"
	case ShapePolyline:
		return "polyline"
	}
	return "rect"
}

// Point is a corner of an outline, relative to the object's own place.
type Point struct{ X, Y float64 }

// Object is something placed on the map rather than drawn into it: where a
// player starts, a door, a trigger, a lamp to draw.
type Object struct {
	ID    int // the number the editor gave it, unique in the map
	Name  string
	Class string // Tiled calls this the type
	X, Y  float64
	W, H  float64

	// Rotation is in turns, like every other angle here, clockwise from
	// upright. Tiled writes degrees; this is those over three hundred and
	// sixty.
	Rotation float64
	Visible  bool

	Shape  Shape
	Points []Point // the corners of a polygon or polyline, and nothing otherwise

	// Tile is the sprite an object was given in the editor, for the kind of
	// object that is a piece of scenery rather than a region. HasTile says
	// whether it was given one at all, since sprite 0 is a real sprite.
	//
	// Tiled anchors such an object at its bottom left, so Y is the foot of the
	// sprite rather than its top.
	Tile    Cell
	HasTile bool

	Props map[string]any
}

// Tileset is one of the sheets a map draws from, and where the map said to
// find it. Sheet is filled in by whoever loads the map, since finding a file is
// not the console's business.
type Tileset struct {
	FirstGID int
	Source   string
	Sheet    *Surface
}

// Tilemap is the whole thing.
type Tilemap struct {
	W, H         int // in cells
	TileW, TileH int // in pixels
	Layers       []*Layer
	Tilesets     []Tileset
	Props        map[string]any
}

// Layer reports a layer by name, or nil. Names are how a game says which part
// of a map it means — "walls", "background" — without counting.
func (m *Tilemap) Layer(name string) *Layer {
	for _, l := range m.Layers {
		if l.Name == name {
			return l
		}
	}
	return nil
}

// LayerAt reports a layer by its place in the map, counted from one as the
// editor shows them, or nil.
func (m *Tilemap) LayerAt(n int) *Layer {
	if n < 1 || n > len(m.Layers) {
		return nil
	}
	return m.Layers[n-1]
}

// Tiles reports the tile layers, in order, which is what drawing the whole map
// walks over.
func (m *Tilemap) Tiles() []*Layer {
	out := make([]*Layer, 0, len(m.Layers))
	for _, l := range m.Layers {
		if l.Kind == TileLayer {
			out = append(out, l)
		}
	}
	return out
}

// Sheet reports the sheet a cell's sprite comes from.
func (m *Tilemap) Sheet(c Cell) *Surface {
	if int(c.Sheet) >= len(m.Tilesets) {
		return nil
	}
	return m.Tilesets[c.Sheet].Sheet
}

// DrawLayer paints a window of one layer.
//
//	tx, ty  the cell of the map the window starts at
//	sx, sy  where that cell lands on the screen
//	tw, th  how many cells across and down to draw
//	flags   when set, only sprites carrying one of these flags are drawn,
//	        which is how one layer of artwork becomes two of scenery
func (c *Console) DrawLayer(m *Tilemap, l *Layer, tx, ty, sx, sy, tw, th int, flags uint8) {
	if m == nil || l == nil || l.Kind != TileLayer {
		return
	}
	for row := 0; row < th; row++ {
		for col := 0; col < tw; col++ {
			cell := l.At(tx+col, ty+row)
			if cell.Empty() {
				continue
			}
			sheet := m.Sheet(cell)
			if sheet == nil {
				continue
			}
			if flags != 0 && sheet.Flags(int(cell.Sprite))&flags == 0 {
				continue
			}
			c.SprCell(sheet, int(cell.Sprite),
				sx+col*m.TileW, sy+row*m.TileH, 1, 1, cell.Turn)
		}
	}
}

// DrawMap paints a window of every visible tile layer, back to front.
func (c *Console) DrawMap(m *Tilemap, tx, ty, sx, sy, tw, th int, flags uint8) {
	if m == nil {
		return
	}
	for _, l := range m.Layers {
		if l.Kind == TileLayer && l.Visible {
			c.DrawLayer(m, l, tx, ty, sx, sy, tw, th, flags)
		}
	}
}
