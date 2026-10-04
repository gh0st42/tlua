package game

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"tlua/internal/payload"
	"tlua/internal/pico"
)

// keys builds the two readers hostShortcut wants: what went down this tick, and
// what is being held.
func keys(pressed []ebiten.Key, down ...ebiten.Key) (func(ebiten.Key) bool, func(...ebiten.Key) bool) {
	was := func(k ebiten.Key) bool {
		for _, p := range pressed {
			if p == k {
				return true
			}
		}
		return false
	}
	holding := func(any ...ebiten.Key) bool {
		for _, want := range any {
			for _, d := range down {
				if d == want {
					return true
				}
			}
		}
		return false
	}
	return was, holding
}

func TestTheWindowsOwnShortcuts(t *testing.T) {
	cases := []struct {
		name    string
		pressed []ebiten.Key
		down    []ebiten.Key
		want    hostAction
		takes   ebiten.Key
	}{
		{
			name:    "F11 on its own is fullscreen",
			pressed: []ebiten.Key{ebiten.KeyF11},
			want:    actionFullscreen, takes: ebiten.KeyF11,
		},
		{
			name:    "so is alt-enter",
			pressed: []ebiten.Key{ebiten.KeyEnter},
			down:    []ebiten.Key{ebiten.KeyAltLeft},
			want:    actionFullscreen, takes: ebiten.KeyEnter,
		},
		{
			name:    "either alt",
			pressed: []ebiten.Key{ebiten.KeyEnter},
			down:    []ebiten.Key{ebiten.KeyAltRight},
			want:    actionFullscreen, takes: ebiten.KeyEnter,
		},
		{
			name:    "ctrl-D shows the frame rate",
			pressed: []ebiten.Key{ebiten.KeyD},
			down:    []ebiten.Key{ebiten.KeyControlLeft},
			want:    actionOverlay, takes: ebiten.KeyD,
		},
		{
			name:    "command works where control does",
			pressed: []ebiten.Key{ebiten.KeyD},
			down:    []ebiten.Key{ebiten.KeyMetaLeft},
			want:    actionOverlay, takes: ebiten.KeyD,
		},
		{
			name:    "ctrl-Q closes the window",
			pressed: []ebiten.Key{ebiten.KeyQ},
			down:    []ebiten.Key{ebiten.KeyControlRight},
			want:    actionQuit, takes: ebiten.KeyQ,
		},
		{
			name:    "enter on its own belongs to the program",
			pressed: []ebiten.Key{ebiten.KeyEnter},
			want:    actionNone,
		},
		{
			name:    "and so does D",
			pressed: []ebiten.Key{ebiten.KeyD},
			want:    actionNone,
		},
		{
			name: "a held key does not fire again",
			down: []ebiten.Key{ebiten.KeyAltLeft, ebiten.KeyEnter},
			want: actionNone,
		},
		{
			name:    "alt with something else is nothing",
			pressed: []ebiten.Key{ebiten.KeyA},
			down:    []ebiten.Key{ebiten.KeyAltLeft},
			want:    actionNone,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			was, holding := keys(c.pressed, c.down...)
			got, used := hostShortcut(was, holding)
			if got != c.want {
				t.Fatalf("action %v, want %v", got, c.want)
			}
			if c.want == actionNone {
				if len(used) != 0 {
					t.Errorf("it took %v without doing anything", used)
				}
				return
			}
			if len(used) != 1 || used[0] != c.takes {
				t.Errorf("it took %v, want %v", used, c.takes)
			}
		})
	}
}

func TestAKeyAShortcutTookIsKeptFromTheProgram(t *testing.T) {
	used := []ebiten.Key{ebiten.KeyD}
	if !consumed(used, ebiten.KeyD) {
		t.Error("D was taken by ctrl-D")
	}
	if consumed(used, ebiten.KeyS) {
		t.Error("S was not")
	}
	if consumed(nil, ebiten.KeyD) {
		t.Error("nothing was taken")
	}
}

func TestDIsASecondPlayersDownKey(t *testing.T) {
	// Which is the whole reason ctrl-D has to take the key: without that,
	// showing the frame rate would also walk player two into a pit.
	if !pico.ButtonsHeld([]string{"d"})[1][pico.BtnDown] {
		t.Skip("D is no longer a direction; the suppression matters less")
	}
}

func TestAButtonIsNamedAsTheKeyboardPrintsIt(t *testing.T) {
	// Before a window is open there is no layout to ask about, so the answer is
	// the key's place. What matters here is that it is a name and not empty:
	// the layout-dependent half cannot be tested without a window.
	if got := buttonLabel(0, pico.BtnO); got == "" {
		t.Error("the O button has no name")
	}
	if got := buttonLabel(0, pico.BtnLeft); got == "" {
		t.Error("left has no name")
	}
	for _, c := range [][2]int{{-1, 0}, {pico.Players, 0}, {0, -1}, {0, pico.Buttons}} {
		if got := buttonLabel(c[0], c[1]); got != "" {
			t.Errorf("player %d button %d is called %q", c[0], c[1], got)
		}
	}
}

func TestEveryKeyTheConsoleBindsIsOneTheWindowKnows(t *testing.T) {
	// The console names keys by their place; if one of those names is not a
	// key the window library has, that button would quietly never work.
	for player := range pico.ButtonKeys {
		for button, keys := range pico.ButtonKeys[player] {
			for _, name := range keys {
				if _, ok := keysByName[name]; !ok {
					t.Errorf("player %d button %d is bound to %q, which is not a key",
						player, button, name)
				}
			}
		}
	}
}

func TestWhatTheFrameRateCounterSays(t *testing.T) {
	lines := overlayLines(59.6, 60.0, 480, 270, 3)
	if len(lines) != 2 {
		t.Fatalf("it says %d lines, want 2", len(lines))
	}
	if !strings.Contains(lines[0], "60 fps") || !strings.Contains(lines[0], "60 tps") {
		t.Errorf("first line is %q", lines[0])
	}
	if lines[1] != "480x270  x3" {
		t.Errorf("second line is %q, want the size and the scale", lines[1])
	}

	// The number keeps its place as it changes, so the box does not jitter.
	short := overlayLines(9, 60, 480, 270, 3)
	if len(short[0]) != len(lines[0]) {
		t.Errorf("%q and %q are different widths", short[0], lines[0])
	}
}

func TestTheFrameRateIsColouredByHowItIsHoldingUp(t *testing.T) {
	cases := []struct {
		rate float64
		want uint8
	}{
		{60, overlayGood},
		{58, overlayGood},
		{45, overlayFair},
		{30, overlayFair},
		{12, overlayPoor},
		{0, overlayPoor},
	}
	for _, c := range cases {
		if got := rateColor(c.rate, 60); got != c.want {
			t.Errorf("%v fps is colour %d, want %d", c.rate, got, c.want)
		}
	}
}

func TestTheCounterDrawsItself(t *testing.T) {
	var o overlay
	picture := o.render(59.6, 60, 480, 270, 3)

	// It is as big as what it says, and no bigger.
	if picture.H != 2*pico.LineHeight+overlayPad*2-1 {
		t.Errorf("the box is %d tall", picture.H)
	}
	if picture.W < pico.Small.Width("480x270  x3") {
		t.Errorf("the box is %d wide, too narrow for what it says", picture.W)
	}

	// A border all the way round, and ink inside it.
	corners := [][2]int{{0, 0}, {picture.W - 1, 0}, {0, picture.H - 1}, {picture.W - 1, picture.H - 1}}
	for _, c := range corners {
		if picture.Get(c[0], c[1]) != overlayEdge {
			t.Errorf("corner %v is %d, want the border colour", c, picture.Get(c[0], c[1]))
		}
	}
	ink := 0
	for _, col := range picture.Pix {
		if col == overlayGood {
			ink++
		}
	}
	if ink == 0 {
		t.Error("the frame rate is not drawn in the colour that says it is keeping up")
	}

	// Asked again at a different rate, it stays the same size.
	again := o.render(12, 60, 480, 270, 3)
	if again.W != picture.W || again.H != picture.H {
		t.Errorf("the box changed size from %dx%d to %dx%d", picture.W, picture.H, again.W, again.H)
	}
}

func TestAProgramsFilesAreFoundBesideIt(t *testing.T) {
	// A game started from somewhere else still means its own art.png.
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "art.png"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	elsewhere := t.TempDir()
	if err := os.WriteFile(filepath.Join(elsewhere, "other.txt"), []byte("theirs"), 0o644); err != nil {
		t.Fatal(err)
	}

	read := besideProgram(home)

	if got, err := read("art.png"); err != nil || string(got) != "mine" {
		t.Errorf("a file beside the program read as %q, %v", got, err)
	}
	// An absolute path is meant literally.
	absolute := filepath.Join(elsewhere, "other.txt")
	if got, err := read(absolute); err != nil || string(got) != "theirs" {
		t.Errorf("an absolute path read as %q, %v", got, err)
	}
	if _, err := read("nothing.png"); err == nil {
		t.Error("a file that is nowhere should be an error")
	}
}

func TestAFusedProgramReadsWhatIsAttachedToItFirst(t *testing.T) {
	archive := archiveOf(t, map[string]string{
		payload.EntryName: "-- the game\n",
		"art.png":         "attached",
	})
	read := attachedFirst(&payload.Payload{Archive: archive})

	if got, err := read("art.png"); err != nil || string(got) != "attached" {
		t.Errorf("read %q, %v; want what was attached", got, err)
	}

	// What is not attached still comes off the disk, so a file put beside the
	// executable is readable.
	dir := t.TempDir()
	outside := filepath.Join(dir, "extra.txt")
	if err := os.WriteFile(outside, []byte("beside it"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := read(outside); err != nil || string(got) != "beside it" {
		t.Errorf("read %q, %v", got, err)
	}

	// And a program with a single file attached, rather than an archive, reads
	// the disk for everything.
	plain := attachedFirst(&payload.Payload{})
	if _, err := plain("art.png"); err == nil {
		t.Error("there is nothing attached to read")
	}
}

// archiveOf builds a payload archive for a test.
func archiveOf(t *testing.T, files map[string]string) *payload.Archive {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	archive, err := payload.NewArchive(zr)
	if err != nil {
		t.Fatal(err)
	}
	return archive
}
