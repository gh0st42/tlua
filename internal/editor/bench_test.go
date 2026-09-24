package editor

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// largeSource is a file big enough that anything proportional to its size shows
// up: 5000 lines of ordinary Lua.
func largeSource(lines int) string {
	var b strings.Builder
	for i := 0; i < lines; i++ {
		fmt.Fprintf(&b, "local value_%d = string.format(\"%%d\", %d) -- line %d\n", i, i, i)
	}
	return b.String()
}

// BenchmarkDraw measures one redraw, which is what every keystroke costs.
func BenchmarkDraw(b *testing.B) {
	for _, lines := range []int{100, 5000} {
		b.Run(fmt.Sprintf("%d-lines", lines), func(b *testing.B) {
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				b.Fatal(err)
			}
			defer screen.Fini()

			area := newCodeArea()
			area.SetText(largeSource(lines), false)
			area.SetRect(0, 0, 80, 25)
			area.Draw(screen) // settle the first scan

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				area.Draw(screen)
			}
		})
	}
}

// BenchmarkDrawAfterEdit measures a redraw that follows a change, which is what
// typing costs: the scan of line starts is thrown away and rebuilt.
func BenchmarkDrawAfterEdit(b *testing.B) {
	for _, lines := range []int{100, 5000} {
		b.Run(fmt.Sprintf("%d-lines", lines), func(b *testing.B) {
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				b.Fatal(err)
			}
			defer screen.Fini()

			area := newCodeArea()
			area.SetText(largeSource(lines), false)
			area.SetRect(0, 0, 80, 25)
			area.Draw(screen)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				area.invalidate()
				area.Draw(screen)
			}
		})
	}
}

// BenchmarkDrawAtTheEnd measures a redraw with the view scrolled to the bottom,
// where the line states above have to be known.
func BenchmarkDrawAtTheEnd(b *testing.B) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		b.Fatal(err)
	}
	defer screen.Fini()

	area := newCodeArea()
	text := largeSource(5000)
	area.SetText(text, true) // cursor at the end
	area.SetRect(0, 0, 80, 25)
	area.Draw(screen)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		area.invalidate()
		area.Draw(screen)
	}
}
