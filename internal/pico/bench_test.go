package pico

import "testing"

// A frame of a busy program: clear, a few hundred shapes, some text, and the
// sprites on top. Drawing has a sixtieth of a second to do all of it and leave
// room for the Lua that asked for it, so what matters here is that none of it
// allocates.
func BenchmarkFrame(b *testing.B) {
	c := New(ScreenWidth, ScreenHeight)
	sprite, err := ParseSprite("7c7|c9c|7c7")
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.Cls(1)
		for n := 0; n < 100; n++ {
			c.Line(n, 0, ScreenWidth-n, ScreenHeight, uint8(n))
			c.RectFill(n, n, n+40, n+30, uint8(n))
			c.CircFill(n*4, n*2, 12, uint8(n))
		}
		for n := 0; n < 200; n++ {
			c.Spr(sprite, n*2, n, false, false)
		}
		c.Print("score 1200\nlives 3", 4, 4, 7)
	}
}

// Turning the framebuffer into pixels happens once a frame whatever else does,
// so it is the one cost no program can avoid.
func BenchmarkPixels(b *testing.B) {
	c := New(ScreenWidth, ScreenHeight)
	c.Cls(12)
	buf := make([]byte, ScreenWidth*ScreenHeight*4)

	b.ReportAllocs()
	b.SetBytes(int64(len(buf)))
	for i := 0; i < b.N; i++ {
		c.Pixels(buf)
	}
}

// A full screen of pset, which is what a per-pixel effect written in Lua does.
func BenchmarkPsetScreen(b *testing.B) {
	c := New(160, 90)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for y := 0; y < 90; y++ {
			for x := 0; x < 160; x++ {
				c.Pset(x, y, uint8(x^y))
			}
		}
	}
}
