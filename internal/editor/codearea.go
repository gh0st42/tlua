package editor

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

// codeArea is tview's text area with Lua colouring painted over it.
//
// tview draws a text area in a single style, and re-implementing an editing
// widget to get coloured text would mean re-implementing its cursor, its
// selection and its undo. Instead this restyles what tview just drew: the
// glyphs on screen stay exactly where the widget put them, and only the colour
// of each cell changes. That keeps the work proportional to the visible window
// rather than to the size of the file.
type codeArea struct {
	*tview.TextArea

	hl    highlighter
	stale bool // the text changed, so the cached lines are out of date
}

func newCodeArea() *codeArea {
	area := &codeArea{TextArea: tview.NewTextArea(), stale: true}
	area.SetWrap(false)
	area.SetTextStyle(textStyle)
	area.SetSelectedStyle(selectionStyle)
	area.SetBackgroundColor(egaBlue)
	return area
}

// invalidate marks the cached scan as out of date; the editor calls it whenever
// the text changes.
func (a *codeArea) invalidate() { a.stale = true }

func (a *codeArea) Draw(screen tcell.Screen) {
	a.TextArea.Draw(screen)

	x, y, width, height := a.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}
	if a.stale {
		a.hl.reset(a.GetText())
		a.stale = false
	}

	rowOffset, columnOffset := a.GetOffset()
	_, selectionBg, _ := selectionStyle.Decompose()

	for row := 0; row < height; row++ {
		runes, classes := a.hl.classesFor(rowOffset + row)
		if runes == nil {
			break
		}
		column := 0
		for i, r := range runes {
			w := cellWidth(r)
			style := classStyles[classes[i]]
			for cell := 0; cell < w; cell++ {
				sx := x + column + cell - columnOffset
				if sx < x {
					continue
				}
				if sx >= x+width {
					break
				}
				mainc, combc, existing, _ := screen.GetContent(sx, y+row)
				// Selected text keeps the selection colours; everything else
				// takes the colour of its token.
				if _, bg, _ := existing.Decompose(); bg == selectionBg {
					continue
				}
				screen.SetContent(sx, y+row, mainc, combc, style)
			}
			column += w
			if column-columnOffset >= width {
				break
			}
		}
	}
}

// cellWidth is how many columns tview gives a rune: tabs are a fixed width,
// everything else is its display width.
func cellWidth(r rune) int {
	if r == '\t' {
		return tview.TabSize
	}
	if w := uniseg.StringWidth(string(r)); w > 0 {
		return w
	}
	return 1
}
