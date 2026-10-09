module tlua

go 1.27.1

require (
	github.com/gdamore/tcell/v2 v2.8.1
	github.com/hajimehoshi/ebiten/v2 v2.10.4
	github.com/pwiecz/go-fltk v0.0.0-20260522213023-3e944122e7b1
	github.com/rivo/tview v0.42.0
	github.com/rivo/uniseg v0.4.7
	github.com/yuin/gopher-lua v1.1.2
	golang.org/x/sys v0.47.0
)

require (
	github.com/ebitengine/gomobile v0.0.0-20260820040257-d11f821a26a6 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.5.0 // indirect
	github.com/ebitengine/purego v0.11.0 // indirect
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/jfreymuth/oggvorbis v1.0.5 // indirect
	github.com/jfreymuth/pulse v0.1.3 // indirect
	github.com/jfreymuth/vorbis v1.0.2 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/term v0.28.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

// A patched copy; third_party/gopher-lua/PATCHES.md says what and why.
replace github.com/yuin/gopher-lua => ./third_party/gopher-lua
