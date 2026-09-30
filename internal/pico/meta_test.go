package pico

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// pngWith builds a PNG carrying a text chunk, the way a sheet editor leaves
// one: the chunk goes in before the end of the file.
func pngWith(t *testing.T, key, text string) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{255, 0, 77, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if key == "" {
		return buf.Bytes()
	}

	body := append([]byte(key+"\x00"), text...)
	chunk := make([]byte, 12+len(body))
	binary.BigEndian.PutUint32(chunk, uint32(len(body)))
	copy(chunk[4:], "tEXt")
	copy(chunk[8:], body)
	binary.BigEndian.PutUint32(chunk[8+len(body):], crc32.ChecksumIEEE(chunk[4:8+len(body)]))

	// Put it in front of the last chunk, which is the end marker.
	raw := buf.Bytes()
	const iend = 12
	out := append([]byte{}, raw[:len(raw)-iend]...)
	out = append(out, chunk...)
	return append(out, raw[len(raw)-iend:]...)
}

func TestReadingSpriteFlagsOutOfAPng(t *testing.T) {
	data := pngWith(t, "fz_meta", `{"tile_size":16,"flags":{"0":1,"3":130,"7":255}}`)

	m := ReadPNGMeta(data)
	if m.TileW != 16 || m.TileH != 16 {
		t.Errorf("the tile size is %dx%d, want 16x16", m.TileW, m.TileH)
	}
	for n, want := range map[int]uint8{0: 1, 3: 130, 7: 255} {
		if got := m.Flags[n]; got != want {
			t.Errorf("sprite %d has flags %d, want %d", n, got, want)
		}
	}
	if len(m.Flags) != 3 {
		t.Errorf("it found %d sprites with flags, want 3", len(m.Flags))
	}
}

func TestAPngThatSaysNothingAboutItself(t *testing.T) {
	for _, data := range [][]byte{
		pngWith(t, "", ""),                      // no chunk at all
		pngWith(t, "something_else", `{"a":1}`), // somebody else's chunk
		pngWith(t, "fz_meta", `not json`),       // ours, but nonsense
		[]byte("this is not a png"),             //
		nil,                                     //
	} {
		if m := ReadPNGMeta(data); !m.Empty() {
			t.Errorf("a picture with nothing to say reported %+v", m)
		}
	}
}

func TestDecodingAPngTakesItsGridAndFlagsWithIt(t *testing.T) {
	s, err := DecodePNG(pngWith(t, "fz_meta", `{"tile_size":2,"flags":{"1":5}}`), Default)
	if err != nil {
		t.Fatal(err)
	}
	if w, h, count := s.Grid(); w != 2 || h != 2 || count != 4 {
		t.Errorf("the grid is %dx%d with %d cells, want 2x2 with 4", w, h, count)
	}
	if got := s.Flags(1); got != 5 {
		t.Errorf("sprite 1 has flags %d, want 5", got)
	}
	if !s.AnyFlags() {
		t.Error("the sheet should know it has flags")
	}
}

func TestReadingSpriteFlagsOutOfATiledTileset(t *testing.T) {
	const tsj = `{
	  "columns": 4, "image": "tiles.png", "imagewidth": 64, "imageheight": 16,
	  "tilewidth": 16, "tileheight": 16, "tilecount": 4,
	  "tiles": [
	    { "id": 1, "properties": [
	        { "name": "flag_0", "type": "bool", "value": true },
	        { "name": "flag_3", "type": "bool", "value": true } ] },
	    { "id": 2, "properties": [
	        { "name": "flag_7", "type": "bool", "value": true },
	        { "name": "flag_2", "type": "bool", "value": false },
	        { "name": "name", "type": "string", "value": "water" } ] },
	    { "id": 3, "properties": [
	        { "name": "flag_9", "type": "bool", "value": true } ] }
	  ]
	}`

	m, err := ReadTSJ([]byte(tsj))
	if err != nil {
		t.Fatal(err)
	}
	if m.TileW != 16 || m.TileH != 16 {
		t.Errorf("the tile size is %dx%d", m.TileW, m.TileH)
	}
	if got := m.Flags[1]; got != 1|8 {
		t.Errorf("sprite 1 has flags %d, want bits 0 and 3", got)
	}
	// A flag that is there but false is not set, and one that says anything
	// other than a bit number is not a flag.
	if got := m.Flags[2]; got != 1<<7 {
		t.Errorf("sprite 2 has flags %d, want bit 7 alone", got)
	}
	if _, ok := m.Flags[3]; ok {
		t.Error("there is no flag 9")
	}
}

func TestSomethingThatIsNotATileset(t *testing.T) {
	if _, err := ReadTSJ([]byte("not json at all")); err == nil {
		t.Error("that is not a tileset")
	}
	// A tileset with nothing in it is still a tileset.
	m, err := ReadTSJ([]byte(`{"tilewidth":8,"tileheight":8}`))
	if err != nil || m.TileW != 8 || len(m.Flags) != 0 {
		t.Errorf("an empty tileset read as %+v, %v", m, err)
	}
}

func TestApplyingMetadataLeavesWhatWasAskedForAlone(t *testing.T) {
	s := NewSurface(16, 16)
	s.SetGrid(4, 4) // asked for outright
	s.Apply(Meta{TileW: 8, TileH: 8, Flags: map[int]uint8{2: 3}})

	if w, _, _ := s.Grid(); w != 4 {
		t.Errorf("the grid is %d, want the 4 that was asked for", w)
	}
	if got := s.Flags(2); got != 3 {
		t.Errorf("the flags did not arrive: %d", got)
	}
}

func TestFlagsOfASprite(t *testing.T) {
	s := NewSurface(16, 16)
	s.SetGrid(8, 8)

	if s.AnyFlags() {
		t.Error("a fresh sheet has no flags")
	}
	s.SetFlag(1, 0, true)
	s.SetFlag(1, 7, true)
	if got := s.Flags(1); got != 1|128 {
		t.Errorf("flags are %d, want bits 0 and 7", got)
	}
	if !s.Flag(1, 0) || !s.Flag(1, 7) || s.Flag(1, 3) {
		t.Error("the wrong bits are set")
	}

	s.SetFlag(1, 0, false)
	if s.Flag(1, 0) {
		t.Error("that bit was cleared")
	}

	// Out of range is quietly nothing, either way round.
	s.SetFlag(0, 9, true)
	s.SetFlag(-1, 0, true)
	s.SetFlags(maxSprites+1, 255)
	if s.Flag(0, 9) || s.Flags(-1) != 0 || s.Flags(99) != 0 {
		t.Error("a flag that cannot exist reported as set")
	}

	// Flags travel with a copy.
	if got := s.Clone().Flags(1); got != 128 {
		t.Errorf("a copy has flags %d", got)
	}
}

func TestATilesetSaysMoreAboutATileThanItsFlags(t *testing.T) {
	// A map editor lets a tile carry anything, and a game may well want the
	// name of a material or a number of hit points rather than a bit.
	const tsj = `{
	  "image": "tiles.png", "tilewidth": 8, "tileheight": 8, "columns": 4,
	  "properties": [
	    { "name": "material", "type": "string", "value": "stone" },
	    { "name": "solid", "type": "bool", "value": true } ],
	  "tiles": [
	    { "id": 1, "properties": [
	        { "name": "flag_0", "type": "bool", "value": true },
	        { "name": "material", "type": "string", "value": "ice" },
	        { "name": "damage", "type": "int", "value": 3 } ] },
	    { "id": 2, "properties": [
	        { "name": "spawns", "type": "class",
	          "value": { "what": "bat", "many": 2 } } ] }
	  ]
	}`

	m, err := ReadTSJ([]byte(tsj))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Sheet["material"]; got != "stone" {
		t.Errorf("the tileset's own material is %v", got)
	}
	if got := m.Props[1]["material"]; got != "ice" {
		t.Errorf("sprite 1 is made of %v", got)
	}
	if got := m.Props[1]["damage"]; got != float64(3) {
		t.Errorf("sprite 1 does %v damage", got)
	}
	// A flag is a property in the file, and stays one here as well as becoming
	// a bit, since a game may prefer to ask for it by name.
	if got := m.Props[1]["flag_0"]; got != true {
		t.Errorf("flag_0 as a property is %v", got)
	}
	if got := m.Flags[1]; got != 1 {
		t.Errorf("flag_0 as a bit is %d", got)
	}
	// A grouped property arrives as the table it is.
	group, ok := m.Props[2]["spawns"].(map[string]any)
	if !ok || group["what"] != "bat" {
		t.Errorf("the grouped property is %#v", m.Props[2]["spawns"])
	}

	s := NewSurface(32, 8)
	s.Apply(m)
	if v, ok := s.Prop(1, "material"); !ok || v != "ice" {
		t.Errorf("the sprite's own material is %v", v)
	}
	if v, ok := s.Prop(2, "material"); !ok || v != "stone" {
		t.Errorf("a sprite with none of its own should fall back to the sheet's, got %v", v)
	}
	if _, ok := s.Prop(3, "nothing"); ok {
		t.Error("that property is nowhere")
	}
	if got := s.Props(1)["solid"]; got != true {
		t.Errorf("the sheet's own properties are inherited too: %v", got)
	}
}

func TestPropertiesOfASpriteCanBeWrittenAndTravelWithTheArtwork(t *testing.T) {
	s := NewSurface(16, 16)
	s.SetGrid(8, 8)
	if s.AnyProps() {
		t.Error("fresh artwork carries nothing")
	}

	s.SetSheetProp("material", "stone")
	s.SetProp(1, "material", "ice")
	s.SetProp(1, "damage", 3)
	if !s.AnyProps() {
		t.Error("something was set")
	}
	if v, _ := s.Prop(1, "material"); v != "ice" {
		t.Errorf("sprite 1 is made of %v", v)
	}
	if v, _ := s.Prop(0, "material"); v != "stone" {
		t.Errorf("sprite 0 falls back to %v", v)
	}

	// A copy carries them, and changing the copy leaves the original alone.
	c := s.Clone()
	if v, _ := c.Prop(1, "damage"); v != 3 {
		t.Errorf("the copy lost a property: %v", v)
	}
	c.SetProp(1, "damage", 9)
	if v, _ := s.Prop(1, "damage"); v != 3 {
		t.Errorf("writing on the copy changed the original: %v", v)
	}

	// Props hands out a table of its own, so keeping it is safe.
	got := s.Props(1)
	got["material"] = "lava"
	if v, _ := s.Prop(1, "material"); v != "ice" {
		t.Errorf("the sheet followed the table it handed out: %v", v)
	}
}
