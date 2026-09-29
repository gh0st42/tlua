package picolua

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"tlua/internal/pico"
)

// A map, the tileset it names, and the picture that names in turn: the three
// files a level is made of, as an editor writes them.
// The sheet a level draws with: three cells of 2x2, the first left blank as
// the convention asks, then colour 8 and colour 12.
func mapSheetPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 6, 2))
	for cell, col := range []uint8{0, 8, 12} {
		if cell == 0 {
			continue // left transparent
		}
		r, g, b := pico.Default.RGB(col)
		for x := 0; x < 2; x++ {
			for y := 0; y < 2; y++ {
				img.Set(cell*2+x, y, color.RGBA{r, g, b, 255})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const (
	levelTMJ = `{
	  "width": 3, "height": 2, "tilewidth": 2, "tileheight": 2,
	  "tilesets": [ { "firstgid": 1, "source": "tiles.tsj" } ],
	  "properties": [ { "name": "title", "value": "the cellar" } ],
	  "layers": [
	    { "type": "tilelayer", "name": "ground", "width": 3, "height": 2,
	      "visible": true, "data": [3, 2, 0, 2, 0, 0] },
	    { "type": "tilelayer", "name": "trees", "width": 3, "height": 2,
	      "visible": true, "data": [0, 0, 3, 0, 0, 0] },
	    { "type": "objectgroup", "name": "spawns", "objects": [
	      { "name": "player", "type": "start", "x": 4, "y": 6,
	        "properties": [ { "name": "lives", "value": 3 } ] } ] }
	  ]
	}`

	levelTSJ = `{
	  "image": "tiles.png", "tilewidth": 2, "tileheight": 2, "columns": 3,
	  "tiles": [ { "id": 1, "properties": [
	    { "name": "flag_0", "type": "bool", "value": true } ] } ]
	}`
)

// level sets up a runtime holding those three files.
func level(t *testing.T, src string) *fixture {
	t.Helper()
	return startWith(t, Options{
		Width: 8, Height: 4,
		ReadFile: func(name string) ([]byte, error) {
			switch name {
			case "maps/level1.tmj":
				return []byte(levelTMJ), nil
			case "maps/tiles.tsj", "gfx/tiles.tsj":
				return []byte(levelTSJ), nil
			case "gfx/tiles.png", "maps/tiles.png":
				return mapSheetPNG(t), nil
			}
			return nil, errNotThere
		},
	}, src)
}

func TestLoadingAMapAndTheArtworkItNames(t *testing.T) {
	f := level(t, `
		level = loadmap("level1")
		usemap(level)`)

	if got := f.str(`level:size()`); got != "3,2" {
		t.Errorf("the map is %q cells", got)
	}
	if got := f.str(`level:tile()`); got != "2,2" {
		t.Errorf("its tiles are %q", got)
	}
	if got := f.str(`table.concat(level:layers(), ",")`); got != "ground,trees,spawns" {
		t.Errorf("its layers are %q", got)
	}
	if got := f.str(`level:props().title`); got != "the cellar" {
		t.Errorf("the map is called %q", got)
	}
	if got := f.str(`tostring(level)`); !strings.Contains(got, "3x2") {
		t.Errorf("a map describes itself as %q", got)
	}
}

func TestAMapIsDrawnByTheCurrentMapCall(t *testing.T) {
	f := level(t, `
		usemap(loadmap("level1"))
		map()`)
	// Both tile layers, in the order the editor had them: the ground, and the
	// one tree standing on the far side of it.
	f.want(`
		cc88cc..
		cc88cc..
		88......
		88......`)
}

func TestAWindowOfAMapGoesWhereItIsPut(t *testing.T) {
	f := level(t, `
		usemap(loadmap("level1"))
		map(0, 0, 4, 2, 1, 1)`)
	f.want(`
		........
		........
		....cc..
		....cc..`)
}

func TestALayerIsReachedByNameOrByPlace(t *testing.T) {
	f := level(t, `
		level = loadmap("level1")
		byname = level:layer("trees")
		byplace = level:layer(2)
		missing = level:layer("nowhere")`)

	if got := f.str(`byname:name(), byplace:name()`); got != "trees,trees" {
		t.Errorf("the second layer is %q both ways", got)
	}
	if got := f.str(`missing == nil`); got != "true" {
		t.Errorf("a layer that is not there gave %q", got)
	}
	if got := f.str(`byname:size()`); got != "3,2" {
		t.Errorf("the layer is %q", got)
	}
}

func TestOneLayerCanBeDrawnOnItsOwn(t *testing.T) {
	f := level(t, `
		local level = loadmap("level1")
		level:layer("trees"):draw()`)
	f.want(`
		....cc..
		....cc..
		........
		........`)
}

func TestALayerCanBeHidden(t *testing.T) {
	f := level(t, `
		local level = loadmap("level1")
		level:layer("ground"):visible(false)
		usemap(level)
		map()`)
	// Only the trees are left.
	f.want(`
		....cc..
		....cc..
		........
		........`)
}

func TestReadingAndWritingTheSquaresOfAMap(t *testing.T) {
	f := level(t, `usemap(loadmap("level1"))`)

	cases := []struct{ expr, want string }{
		{`mget(0, 0)`, "2"},          // the map's 3 is the sheet's sprite 2
		{`mget(2, 0)`, "0"},          // empty
		{`mget(0, 1)`, "1"},          //
		{`mget(2, 0, "trees")`, "2"}, // the same square of another layer
		{`mget(99, 99)`, "0"},        // off the map
	}
	for _, c := range cases {
		if got := f.str(c.expr); got != c.want {
			t.Errorf("%s = %q, want %q", c.expr, got, c.want)
		}
	}

	f.eval(`mset(2, 0, 1)`)
	if got := f.str(`mget(2, 0)`); got != "1" {
		t.Errorf("after writing, the square holds %q", got)
	}
	// And a layer can be written by name.
	f.eval(`mset(0, 0, 0, "trees")`)
	if got := f.str(`mget(0, 0, "trees")`); got != "0" {
		t.Errorf("the trees layer holds %q", got)
	}
}

func TestThingsPlacedOnAMapComeBackAsTables(t *testing.T) {
	f := level(t, `
		level = loadmap("level1")
		spawns = level:objects("spawns")
		player = spawns[1]`)

	cases := []struct{ expr, want string }{
		{`#spawns`, "1"},
		{`player.name`, "player"},
		{`player.class`, "start"},
		{`player.x, player.y`, "4,6"},
		{`player.props.lives`, "3"},
		{`#level:objects("ground")`, "0"}, // a tile layer has none
	}
	for _, c := range cases {
		if got := f.str(c.expr); got != c.want {
			t.Errorf("%s = %q, want %q", c.expr, got, c.want)
		}
	}
}

func TestTheFlagsOnAMapsOwnSheet(t *testing.T) {
	// The tileset says sprite 1 is solid, and that has to survive the trip
	// through the map: this is what makes fget(mget(x, y)) worth writing.
	f := level(t, `
		usemap(loadmap("level1"))
		local sheet = loadpng("tiles")
		usesheet(sheet)`)

	// The tileset flags sprite 1, which is the square at 1,0.
	if got := f.str(`fget(mget(1, 0), 0)`); got != "true" {
		t.Errorf("the square at 1,0 should be solid, got %q", got)
	}
	if got := f.str(`fget(mget(0, 0), 0)`); got != "false" {
		t.Errorf("the square at 0,0 should not be, got %q", got)
	}
}

func TestDrawingOnlyWhatCarriesAFlag(t *testing.T) {
	f := level(t, `
		usemap(loadmap("level1"))
		map(0, 0, 0, 0, 3, 2, 1)`)
	// Only sprite 1 is flagged, so only it is drawn.
	f.want(`
		..88....
		..88....
		88......
		88......`)
}

func TestAMapCallStillDrawsAGridWrittenInTheProgram(t *testing.T) {
	// The table form has not gone anywhere: both kinds of level work.
	f := start(t, 8, 4, `
		local tiles = sprite([[
			..11
			..11
		]], 2, 2)
		map({"01", "10"}, tiles)`)
	f.want(`
		..11....
		..11....
		11......
		11......`)
}

func TestAMapHasToBeLoadedBeforeItIsUsed(t *testing.T) {
	f := start(t, 4, 2, "")
	for _, expr := range []string{`map()`, `mget(0, 0)`, `mset(0, 0, 1)`} {
		err := f.L.DoString(expr)
		if err == nil {
			t.Errorf("%s should say there is no map", expr)
			continue
		}
		if !strings.Contains(err.Error(), "usemap") {
			t.Errorf("%s said %q; it should say what to do", expr, err)
		}
	}
}

func TestAMapThatIsNotThere(t *testing.T) {
	f := level(t, "")
	if got := f.str(`loadmap("nowhere") == nil`); got != "true" {
		t.Errorf("a map that is not there gave %q", got)
	}
	if got := f.str(`select(2, loadmap("nowhere"))`); !strings.Contains(got, "maps/nowhere.tmj") {
		t.Errorf("the error does not say where it looked: %q", got)
	}
}

func TestAMapWhoseArtworkIsMissing(t *testing.T) {
	f := startWith(t, Options{
		Width: 8, Height: 4,
		ReadFile: func(name string) ([]byte, error) {
			if name == "maps/level1.tmj" {
				return []byte(levelTMJ), nil
			}
			return nil, errNotThere
		},
	}, "")

	got := f.str(`select(2, loadmap("level1"))`)
	if !strings.Contains(got, "tiles.tsj") {
		t.Errorf("the error should name the tileset it could not find: %q", got)
	}
}
