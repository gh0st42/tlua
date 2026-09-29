package picolua

import (
	"strings"
	"testing"
)

func TestWhereAResourceIsLookedFor(t *testing.T) {
	cases := []struct {
		kind kind
		name string
		want string
	}{
		{
			// A name on its own: every extension in every place.
			kindSound, "jump",
			"jump.wav jump.ogg sfx/jump.wav sfx/jump.ogg assets/sfx/jump.wav assets/sfx/jump.ogg assets/jump.wav assets/jump.ogg",
		},
		{
			// A filename: taken as it is, but still looked for in each place.
			kindSound, "jump.wav",
			"jump.wav sfx/jump.wav assets/sfx/jump.wav assets/jump.wav",
		},
		{
			// A path: found by the first place, which is the program's own.
			kindSound, "assets/sfx/jump.wav",
			"assets/sfx/jump.wav sfx/assets/sfx/jump.wav assets/sfx/assets/sfx/jump.wav assets/assets/sfx/jump.wav",
		},
		{
			// Music is looked for where two different tools put it, and among
			// the sound effects after that.
			kindMusic, "theme",
			"theme.ogg theme.wav music/theme.ogg music/theme.wav assets/music/theme.ogg assets/music/theme.wav bgm/theme.ogg bgm/theme.wav assets/bgm/theme.ogg assets/bgm/theme.wav sfx/theme.ogg sfx/theme.wav assets/sfx/theme.ogg assets/sfx/theme.wav assets/theme.ogg assets/theme.wav",
		},
		{
			kindImage, "player",
			"player.png gfx/player.png assets/gfx/player.png assets/player.png",
		},
		{
			kindImage, "gfx/player.png",
			"gfx/player.png gfx/gfx/player.png assets/gfx/gfx/player.png assets/gfx/player.png",
		},
		{
			// An extension nobody here knows is still a filename.
			kindData, "config.json",
			"config.json data/config.json assets/data/config.json maps/config.json assets/maps/config.json assets/config.json",
		},
	}

	for _, c := range cases {
		got := strings.Join(candidates(c.kind, c.name), " ")
		if got != c.want {
			t.Errorf("%s %q looked in:\n  %s\nwant:\n  %s", c.kind.what, c.name, got, c.want)
		}
	}
}

func TestAnAbsolutePathIsMeantLiterally(t *testing.T) {
	got := candidates(kindSound, "/opt/sounds/jump.wav")
	if len(got) != 1 || got[0] != "/opt/sounds/jump.wav" {
		t.Errorf("looked in %v", got)
	}
}

func TestNothingIsLookedForWhenNothingIsAsked(t *testing.T) {
	if got := candidates(kindSound, "   "); got != nil {
		t.Errorf("looked in %v", got)
	}
}

func TestFindingAResourceAndRememberingWhere(t *testing.T) {
	reads := 0
	f := startWith(t, Options{
		Width: 2, Height: 1,
		ReadFile: func(name string) ([]byte, error) {
			reads++
			if name == "assets/sfx/jump.wav" {
				return []byte("RIFF"), nil
			}
			return nil, errNotThere
		},
	}, "")

	path, data, err := f.find(kindSound, "jump")
	if err != nil {
		t.Fatal(err)
	}
	if path != "assets/sfx/jump.wav" || string(data) != "RIFF" {
		t.Errorf("found %q, %q", path, data)
	}
	first := reads

	// Asked again, it goes straight there rather than looking through the
	// places it already ruled out.
	if _, _, err := f.find(kindSound, "jump"); err != nil {
		t.Fatal(err)
	}
	if reads-first != 1 {
		t.Errorf("the second look cost %d reads, want 1", reads-first)
	}
}

func TestWhenAResourceIsNowhereTheErrorSaysWhereItLooked(t *testing.T) {
	f := startWith(t, Options{
		Width: 2, Height: 1,
		ReadFile: func(string) ([]byte, error) { return nil, errNotThere },
	}, "")

	_, _, err := f.find(kindSound, "jump")
	if err == nil {
		t.Fatal("there is no such sound")
	}
	for _, want := range []string{"jump", "sfx/jump.wav", "assets/sfx/jump.ogg"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}
}
