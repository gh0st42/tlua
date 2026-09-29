package pico

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
)

// packed writes tile numbers the way Tiled's base64 layers hold them.
func packed(t *testing.T, gids []uint32, how string) string {
	t.Helper()
	raw := make([]byte, len(gids)*4)
	for i, g := range gids {
		binary.LittleEndian.PutUint32(raw[i*4:], g)
	}

	var buf bytes.Buffer
	switch how {
	case "":
		buf.Write(raw)
	case "zlib":
		w := zlib.NewWriter(&buf)
		w.Write(raw)
		w.Close()
	case "gzip":
		w := gzip.NewWriter(&buf)
		w.Write(raw)
		w.Close()
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// tmjWith builds a small map with one tile layer holding these numbers.
func tmjWith(t *testing.T, gids []uint32, encoding, compression string) []byte {
	t.Helper()
	var data string
	if encoding == "base64" {
		data = fmt.Sprintf("%q", packed(t, gids, compression))
	} else {
		parts := make([]string, len(gids))
		for i, g := range gids {
			parts[i] = fmt.Sprint(g)
		}
		data = "[" + strings.Join(parts, ",") + "]"
	}

	return []byte(fmt.Sprintf(`{
	  "width": 2, "height": 2, "tilewidth": 8, "tileheight": 8,
	  "tilesets": [ { "firstgid": 1, "source": "tiles.tsj" } ],
	  "layers": [ {
	    "type": "tilelayer", "name": "ground", "width": 2, "height": 2,
	    "visible": true, "encoding": %q, "compression": %q, "data": %s
	  } ]
	}`, encoding, compression, data))
}

func TestReadingAMapHoweverItIsWritten(t *testing.T) {
	gids := []uint32{1, 2, 0, 3}
	for _, how := range []struct{ encoding, compression string }{
		{"csv", ""},
		{"", ""}, // an older file with no encoding named
		{"base64", ""},
		{"base64", "zlib"},
		{"base64", "gzip"},
	} {
		name := how.encoding + " " + how.compression
		t.Run(strings.TrimSpace(name), func(t *testing.T) {
			m, err := ReadTMJ(tmjWith(t, gids, how.encoding, how.compression))
			if err != nil {
				t.Fatal(err)
			}
			if m.W != 2 || m.H != 2 || m.TileW != 8 {
				t.Errorf("the map is %dx%d of %d", m.W, m.H, m.TileW)
			}
			l := m.Layer("ground")
			if l == nil {
				t.Fatal("there is no layer called ground")
			}
			// The map's numbers start at one; a sprite's start at zero.
			for i, want := range []int32{0, 1, 0, 2} {
				if got := l.Cells[i].Sprite; got != want {
					t.Errorf("square %d holds sprite %d, want %d", i, got, want)
				}
			}
			if !l.At(0, 1).Empty() {
				t.Error("the third square is empty in the file")
			}
		})
	}
}

func TestAMapCompressedInAWayWeCannotRead(t *testing.T) {
	_, err := ReadTMJ(tmjWith(t, []uint32{1}, "base64", "zstd"))
	if err == nil {
		t.Fatal("zstd is not read")
	}
	// The message has to name a way out, since the file is not broken.
	for _, want := range []string{"zstd", "zlib"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}
}

func TestATurnedTileKeepsItsTurn(t *testing.T) {
	const (
		h = 0x80000000
		v = 0x40000000
		d = 0x20000000
	)
	gids := []uint32{1 | h, 1 | v, 1 | d, 1 | h | v | d}
	m, err := ReadTMJ(tmjWith(t, gids, "csv", ""))
	if err != nil {
		t.Fatal(err)
	}

	l := m.Layer("ground")
	want := []Orientation{FlipX, FlipY, Turn, FlipX | FlipY | Turn}
	for i, w := range want {
		if got := l.Cells[i].Turn; got != w {
			t.Errorf("square %d is turned %d, want %d", i, got, w)
		}
		// The flip bits must not be mistaken for part of the number.
		if got := l.Cells[i].Sprite; got != 0 {
			t.Errorf("square %d holds sprite %d, want 0", i, got)
		}
	}
}

func TestAMapOfSeveralSheets(t *testing.T) {
	const tmj = `{
	  "width": 4, "height": 1, "tilewidth": 8, "tileheight": 8,
	  "tilesets": [
	    { "firstgid": 1, "source": "one.tsj" },
	    { "firstgid": 5, "source": "two.tsj" } ],
	  "layers": [ { "type": "tilelayer", "name": "a", "width": 4, "height": 1,
	    "data": [1, 4, 5, 9] } ]
	}`
	m, err := ReadTMJ([]byte(tmj))
	if err != nil {
		t.Fatal(err)
	}

	cells := m.Layer("a").Cells
	want := []struct {
		sheet  uint8
		sprite int32
	}{
		{0, 0}, // the first sheet's first sprite
		{0, 3},
		{1, 0}, // and where the second sheet's numbering starts
		{1, 4},
	}
	for i, w := range want {
		if cells[i].Sheet != w.sheet || cells[i].Sprite != w.sprite {
			t.Errorf("square %d is sheet %d sprite %d, want %d and %d",
				i, cells[i].Sheet, cells[i].Sprite, w.sheet, w.sprite)
		}
	}
}

func TestReadingThingsPlacedOnAMap(t *testing.T) {
	const tmj = `{
	  "width": 1, "height": 1, "tilewidth": 8, "tileheight": 8,
	  "tilesets": [ { "firstgid": 1, "source": "t.tsj" } ],
	  "properties": [ { "name": "title", "value": "the cellar" } ],
	  "layers": [
	    { "type": "tilelayer", "name": "ground", "width": 1, "height": 1, "data": [1] },
	    { "type": "objectgroup", "name": "spawns", "objects": [
	      { "name": "player", "type": "start", "x": 16, "y": 32, "width": 8, "height": 8,
	        "properties": [
	          { "name": "facing", "value": "left" },
	          { "name": "lives", "value": 3 },
	          { "name": "boss", "value": true } ] },
	      { "name": "door", "class": "exit", "x": 40, "y": 8 } ] },
	    { "type": "imagelayer", "name": "sky" }
	  ]
	}`
	m, err := ReadTMJ([]byte(tmj))
	if err != nil {
		t.Fatal(err)
	}

	if got := len(m.Layers); got != 2 {
		t.Errorf("the map has %d layers; the image layer is not one it can use", got)
	}
	if got := m.Props["title"]; got != "the cellar" {
		t.Errorf("the map is called %v", got)
	}

	spawns := m.Layer("spawns")
	if spawns == nil || spawns.Kind != ObjectLayer || len(spawns.Objects) != 2 {
		t.Fatalf("the object layer read as %+v", spawns)
	}
	player := spawns.Objects[0]
	if player.Name != "player" || player.Class != "start" || player.X != 16 || player.W != 8 {
		t.Errorf("the player object is %+v", player)
	}
	if player.Props["facing"] != "left" || player.Props["lives"] != float64(3) || player.Props["boss"] != true {
		t.Errorf("its properties are %+v", player.Props)
	}
	// Newer files say "class" where older ones say "type".
	if got := spawns.Objects[1].Class; got != "exit" {
		t.Errorf("the door is a %q", got)
	}

	// Layers can be reached by where they are as well as by name.
	if l := m.LayerAt(1); l == nil || l.Name != "ground" {
		t.Errorf("the first layer is %v", l)
	}
	if m.LayerAt(0) != nil || m.LayerAt(9) != nil {
		t.Error("there is no layer there")
	}
	if got := len(m.Tiles()); got != 1 {
		t.Errorf("%d layers have tiles on them, want 1", got)
	}
}

func TestSomethingThatIsNotAMap(t *testing.T) {
	for _, data := range []string{
		"not json",
		`{"width":2}`, // no tile size
		`{"width":2,"height":2,"tilewidth":8,"tileheight":8,"layers":[
			{"type":"tilelayer","name":"a","width":2,"height":2,"data":"not base64 at all","encoding":"base64"}]}`,
	} {
		if _, err := ReadTMJ([]byte(data)); err == nil {
			t.Errorf("this should not read as a map: %s", data)
		}
	}
}

func TestDrawingAWindowOfAMap(t *testing.T) {
	sheet, err := ParseSprite(`
		..1122
		..1122
		..3344
		..3344`)
	if err != nil {
		t.Fatal(err)
	}
	sheet.SetGrid(2, 2)

	m, err := ReadTMJ([]byte(`{
	  "width": 2, "height": 2, "tilewidth": 2, "tileheight": 2,
	  "tilesets": [ { "firstgid": 1, "source": "t.tsj" } ],
	  "layers": [ { "type": "tilelayer", "name": "a", "width": 2, "height": 2,
	    "visible": true, "data": [2, 3, 5, 0] } ]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	m.Tilesets[0].Sheet = sheet

	// The map's 2, 3 and 5 are the sheet's sprites 1, 2 and 4, and its 0 is
	// nothing at all.
	c := New(4, 4)
	c.DrawMap(m, 0, 0, 0, 0, 2, 2, 0)
	wantScreen(t, c, `
		1122
		1122
		33..
		33..`)

	// A window of it, put somewhere else on the screen.
	c.Cls(0)
	c.DrawMap(m, 1, 0, 2, 2, 1, 1, 0)
	wantScreen(t, c, `
		....
		....
		..22
		..22`)
}

func TestDrawingOnlyTheSpritesCarryingAFlag(t *testing.T) {
	sheet, err := ParseSprite(`
		..1122
		..1122`)
	if err != nil {
		t.Fatal(err)
	}
	sheet.SetGrid(2, 2)
	sheet.SetFlag(1, 0, true) // only sprite 1 is solid

	m, err := ReadTMJ([]byte(`{
	  "width": 2, "height": 1, "tilewidth": 2, "tileheight": 2,
	  "tilesets": [ { "firstgid": 1, "source": "t.tsj" } ],
	  "layers": [ { "type": "tilelayer", "name": "a", "width": 2, "height": 1,
	    "visible": true, "data": [2, 3] } ]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	m.Tilesets[0].Sheet = sheet

	c := New(4, 2)
	c.DrawMap(m, 0, 0, 0, 0, 2, 1, 1<<0)
	wantScreen(t, c, `
		11..
		11..`)
}
