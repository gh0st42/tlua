package editor

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// The 16 colours of the EGA palette, under their classic names. tcell maps
// these onto the terminal's first sixteen slots, which is what a DOS text mode
// had; nothing here reaches for 256-colour or true-colour values.
const (
	egaBlack        = tcell.ColorBlack
	egaBlue         = tcell.ColorNavy
	egaGreen        = tcell.ColorGreen
	egaCyan         = tcell.ColorTeal
	egaRed          = tcell.ColorMaroon
	egaMagenta      = tcell.ColorPurple
	egaBrown        = tcell.ColorOlive
	egaLightGray    = tcell.ColorSilver
	egaDarkGray     = tcell.ColorGray
	egaLightBlue    = tcell.ColorBlue
	egaLightGreen   = tcell.ColorLime
	egaLightCyan    = tcell.ColorAqua
	egaLightRed     = tcell.ColorRed
	egaLightMagenta = tcell.ColorFuchsia
	egaYellow       = tcell.ColorYellow
	egaWhite        = tcell.ColorWhite
)

// How those colours are used, following Turbo Pascal and QBasic: a blue
// desktop, light grey bars top and bottom with red hotkey letters, a green
// selection bar, and program output on black like the DOS screen it replaced.
var (
	textStyle      = tcell.StyleDefault.Foreground(egaLightGray).Background(egaBlue)
	selectionStyle = tcell.StyleDefault.Foreground(egaBlue).Background(egaLightGray)
	cursorStyle    = tcell.StyleDefault.Foreground(egaBlue).Background(egaLightGray)
)

// Dialogs are the grey panels of a DOS IDE: black on light grey, white input
// fields, a green selection bar and green buttons.
var (
	dialogTextStyle          = tcell.StyleDefault.Foreground(egaBlack).Background(egaLightGray)
	dialogSelectedStyle      = tcell.StyleDefault.Foreground(egaBlack).Background(egaGreen)
	dialogFieldStyle         = tcell.StyleDefault.Foreground(egaBlack).Background(egaWhite)
	dialogButtonStyle        = tcell.StyleDefault.Foreground(egaBlack).Background(egaGreen)
	dialogButtonFocusedStyle = tcell.StyleDefault.Foreground(egaWhite).Background(egaGreen).Bold(true)
)

// The function list colours a name by how it is reachable. These are the dark
// end of the palette, because they sit on a light grey panel.
var outlineTags = map[entryKind]string{
	kindGlobal: "[black]",
	kindLocal:  "[navy]",
	kindNested: "[maroon]",
}

// Colour tags for the bars and the output pane, used with dynamic colours.
const (
	tagBar       = "[black:silver]"
	tagHotkey    = "[maroon:silver]"
	tagBarDim    = "[gray:silver]"
	tagTabIdle   = "[black:teal]"
	tagTabActive = "[yellow:navy]"
	tagOutput    = "[silver:black]"
	tagErr       = "[red:black]"
	tagNote      = "[yellow:black]"
	tagOK        = "[lime:black]"
)

// applyTheme paints tview's shared styles with the palette above.
func applyTheme() {
	tview.Styles = tview.Theme{
		PrimitiveBackgroundColor:    egaBlue,
		ContrastBackgroundColor:     egaCyan,
		MoreContrastBackgroundColor: egaGreen,
		BorderColor:                 egaLightGray,
		TitleColor:                  egaYellow,
		GraphicsColor:               egaLightGray,
		PrimaryTextColor:            egaLightGray,
		SecondaryTextColor:          egaYellow,
		TertiaryTextColor:           egaLightGreen,
		InverseTextColor:            egaBlue,
		ContrastSecondaryTextColor:  egaBlack,
	}
}

// dialogColors gives a box the light grey face of a DOS dialog.
func dialogColors(b *tview.Box) {
	b.SetBackgroundColor(egaLightGray)
	b.SetBorder(true)
	b.SetBorderColor(egaBlack)
	b.SetTitleColor(egaBlack)
}
