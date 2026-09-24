package editor

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"

	"tlua/internal/lsp"
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

	// diagnostics are marked over the colouring, so that a mistake shows
	// through whatever the text happens to be made of.
	diagnostics []lsp.Diagnostic
}

// setDiagnostics takes what the language server said about this text.
func (a *codeArea) setDiagnostics(list []lsp.Diagnostic) { a.diagnostics = list }

// marks returns the byte range of a line that a diagnostic covers, and the style
// to draw it in. A range of no width, which is what a server sends when the
// trouble is where something is missing, marks the whole line, since a mark of
// no width would not be seen.
func (a *codeArea) marks(lineIndex int, line string) (spans []markSpan) {
	for _, d := range a.diagnostics {
		style, ok := diagnosticStyles[d.Severity]
		if !ok {
			// A note or a hint is underlined rather than coloured over.
			style = textStyle.Underline(true)
		}
		if lineIndex < d.Range.Start.Line || lineIndex > d.Range.End.Line {
			continue
		}

		from, to := 0, len(line)
		if lineIndex == d.Range.Start.Line {
			from = lineOffset(line, d.Range.Start.Character)
		}
		if lineIndex == d.Range.End.Line {
			to = lineOffset(line, d.Range.End.Character)
		}
		if to <= from {
			from, to = 0, len(line)
		}
		spans = append(spans, markSpan{from: from, to: to, style: style})
	}
	return spans
}

// markSpan is a stretch of one line to draw differently.
type markSpan struct {
	from, to int
	style    tcell.Style
}

// lineOffset turns a protocol character, counted in UTF-16 units, into a byte
// offset into one line.
func lineOffset(line string, character int) int {
	offset, err := lsp.Offset(line, lsp.Position{Line: 0, Character: character})
	if err != nil {
		return len(line)
	}
	return offset
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
		a.hl.update(a.GetText())
		a.stale = false
	}

	rowOffset, columnOffset := a.GetOffset()
	_, selectionBg, _ := selectionStyle.Decompose()

	for row := 0; row < height; row++ {
		line, classes := a.hl.classesFor(rowOffset + row)
		if classes == nil {
			break
		}
		var spans []markSpan
		if len(a.diagnostics) > 0 {
			spans = a.marks(rowOffset+row, line)
		}

		column := 0
		for i, r := range line {
			w := cellWidth(r)
			style := classStyles[classes[i]]
			for _, span := range spans {
				if i >= span.from && i < span.to {
					style = span.style
					break
				}
			}
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
