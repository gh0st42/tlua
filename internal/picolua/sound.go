package picolua

import (
	lua "github.com/yuin/gopher-lua"
)

// Channels is how many sounds can be playing at once, not counting the music.
const Channels = 8

// Sound is what sfx() and music() reach: a host that can actually make a
// noise. Package sound is the one that does, with Ebitengine behind it.
//
// Sounds are handed over as the bytes of a .wav or .ogg file, already found and
// read, because finding them is the console's business and decoding them is the
// host's. The name that comes with them is what was found, so that a host can
// decode a sound once however often it is played.
type Sound interface {
	// Play starts a sound on a channel, or on any free one when given -1, and
	// reports the channel it went to. The data is nil for a sound the host has
	// been given before.
	Play(name string, data []byte, channel int, volume float64) (int, error)

	// Stop silences one channel, or every one of them when given -1.
	Stop(channel int)

	// Music starts the music, replacing whatever was playing, fading in over a
	// number of milliseconds.
	Music(name string, data []byte, volume float64, fadeMS int) error

	// StopMusic fades the music out and stops it.
	StopMusic(fadeMS int)

	// NowPlaying reports the name of the music, or "" for silence.
	NowPlaying() string

	// SetVolume and Volume are the level everything is played at, from 0 to 1.
	SetVolume(v float64)
	Volume() float64
}

// silent is the Sound a runtime has when nothing has been plugged into it: a
// test, or a program being run to see what it draws. It behaves like a host
// that works, so that a program's own logic — which channel a sound went to,
// whether the music is playing — is the same either way, and says nothing.
type silent struct {
	channels [Channels]string
	music    string
	volume   float64
}

func newSilent() *silent { return &silent{volume: 1} }

func (s *silent) Play(name string, _ []byte, channel int, _ float64) (int, error) {
	if channel < 0 {
		channel = 0
		for i, playing := range s.channels {
			if playing == "" {
				channel = i
				break
			}
		}
	}
	if channel >= Channels {
		channel %= Channels
	}
	s.channels[channel] = name
	return channel, nil
}

func (s *silent) Stop(channel int) {
	if channel < 0 {
		s.channels = [Channels]string{}
		return
	}
	if channel < Channels {
		s.channels[channel] = ""
	}
}

func (s *silent) Music(name string, _ []byte, _ float64, _ int) error {
	s.music = name
	return nil
}

func (s *silent) StopMusic(int)       { s.music = "" }
func (s *silent) NowPlaying() string  { return s.music }
func (s *silent) SetVolume(v float64) { s.volume = v }
func (s *silent) Volume() float64     { return s.volume }

// installSound adds the two calls that make a noise.
func (r *Runtime) installSound() {
	r.register(map[string]lua.LGFunction{
		// sfx(name, [channel], [volume]) plays a sound and reports the channel
		// it went to. The name is looked for the way every resource is:
		// sfx("jump") finds sfx/jump.wav or assets/sfx/jump.wav, and
		// sfx("assets/sfx/jump.wav") means exactly that.
		//
		// sfx(-1) stops everything; sfx(-1, channel) stops one channel.
		"sfx": func(L *lua.LState) int {
			if stop, channel := stopArgs(L); stop {
				r.sound.Stop(channel)
				return 0
			}

			name := L.CheckString(1)
			path, data, err := r.find(kindSound, name)
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}

			channel, err := r.sound.Play(path, data, L.OptInt(2, -1), volumeArg(L, 3))
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			L.Push(lua.LNumber(channel))
			return 1
		},

		// music(name, [fade_ms], [volume]) starts the music, which loops until
		// something else is asked for. music(-1) stops it, music(-1, 500)
		// fades it out over half a second, and music() on its own says what is
		// playing.
		"music": func(L *lua.LState) int {
			if isNone(L, 1) {
				if playing := r.sound.NowPlaying(); playing != "" {
					L.Push(lua.LString(playing))
					return 1
				}
				L.Push(lua.LNil)
				return 1
			}
			if stop, _ := stopArgs(L); stop {
				r.sound.StopMusic(L.OptInt(2, 0))
				return 0
			}

			name := L.CheckString(1)
			path, data, err := r.find(kindMusic, name)
			if err == nil {
				err = r.sound.Music(path, data, volumeArg(L, 3), L.OptInt(2, 0))
			}
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			L.Push(lua.LString(path))
			return 1
		},

		// volume() is how loud everything is, from 0 to 1; volume(v) changes
		// it and reports what it was.
		"volume": func(L *lua.LState) int {
			was := r.sound.Volume()
			if !isNone(L, 1) {
				r.sound.SetVolume(clamped(float64(L.CheckNumber(1)), 0, 1))
			}
			L.Push(lua.LNumber(was))
			return 1
		},
	})
}

// stopArgs reads the "stop this" form of sfx() and music(): a negative number,
// as Picotron spells it, or false for anyone who finds that odd.
func stopArgs(L *lua.LState) (stop bool, channel int) {
	switch v := L.Get(1).(type) {
	case lua.LNumber:
		if v < 0 {
			return true, L.OptInt(2, -1)
		}
	case lua.LBool:
		if !bool(v) {
			return true, L.OptInt(2, -1)
		}
	}
	return false, -1
}

// volumeArg reads an optional volume, which is 1 unless asked otherwise.
func volumeArg(L *lua.LState, n int) float64 {
	if isNone(L, n) {
		return 1
	}
	return clamped(float64(L.CheckNumber(n)), 0, 1)
}

// clamped keeps a number between two others.
func clamped(v, lo, hi float64) float64 {
	switch {
	case v < lo:
		return lo
	case v > hi:
		return hi
	}
	return v
}
