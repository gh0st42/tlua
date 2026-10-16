// Package audio is the audio module: sounds played out of the speakers by a
// program with a window of its own (the gui module's), through oto.
//
//	local audio = require "audio"
//	local v = audio.play(sound, { loop = true, volume = 0.8 })
//	v:position()          -- where it has got to, in seconds, as heard
//	v:replace(newSound)   -- carries on with another at the end of the loop
//	v:stop()
//
// Sounds come from other modules (beepbox renders them) as pcm sounds. The
// device is opened the first time something is played, at the rate of that
// sound, and every sound after has to have the same rate. The console
// (tlua play) has its own sound, sfx() and music(), and this is not for it.
package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
	lua "github.com/yuin/gopher-lua"

	"tlua/internal/pcm"
)

// Buffer is how much the device is given ahead: short, so a note sounds
// when it is placed, and long enough not to break up.
const Buffer = 60 * time.Millisecond

var device struct {
	mu   sync.Mutex
	ctx  *oto.Context
	rate int
	err  error
}

// open opens the device at a rate, once; after that a sound must match it.
func open(rate int) (*oto.Context, error) {
	device.mu.Lock()
	defer device.mu.Unlock()
	if device.ctx != nil {
		if rate != device.rate {
			return nil, fmt.Errorf("audio: the device plays at %d Hz, not %d", device.rate, rate)
		}
		return device.ctx, nil
	}
	if device.err != nil {
		return nil, device.err
	}
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate: rate, ChannelCount: 2, Format: oto.FormatFloat32LE,
		BufferSize: Buffer, ApplicationName: "tlua",
	})
	if err != nil {
		device.err = fmt.Errorf("audio: %w", err)
		return nil, device.err
	}
	<-ready
	if err := ctx.Err(); err != nil {
		device.err = fmt.Errorf("audio: %w", err)
		return nil, device.err
	}
	device.ctx, device.rate = ctx, rate
	return ctx, nil
}

// ---------------------------------------------------------------- a voice

// stream is what a player reads: a sound from where it has got to, round
// again when it loops, and the next one swapped in at the end.
type stream struct {
	mu     sync.Mutex
	cur    *pcm.Sound
	next   *pcm.Sound
	pos    int // the frame read next
	loop   bool
	ended  bool
	played int // frames handed over in all
}

func (s *stream) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for n+8 <= len(p) {
		if s.pos >= s.cur.Frames() {
			if s.next != nil {
				s.cur, s.next = s.next, nil
				s.pos = 0
			} else if s.loop && s.cur.Frames() > 0 {
				s.pos = 0
			} else {
				s.ended = true
				break
			}
		}
		binary.LittleEndian.PutUint32(p[n:], math.Float32bits(s.cur.Left[s.pos]))
		binary.LittleEndian.PutUint32(p[n+4:], math.Float32bits(s.cur.Right[s.pos]))
		s.pos++
		s.played++
		n += 8
	}
	if s.ended && n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

// Voice is a sound playing.
type Voice struct {
	player *oto.Player
	stream *stream
	rate   int
}

// Play starts a sound. from is where in it, in seconds.
func Play(s *pcm.Sound, loop bool, volume, from float64) (*Voice, error) {
	if s.Rate <= 0 {
		return nil, errors.New("audio: a sound with no rate")
	}
	ctx, err := open(s.Rate)
	if err != nil {
		return nil, err
	}
	st := &stream{cur: s, loop: loop, pos: clampFrame(int(from*float64(s.Rate)), s.Frames())}
	p := ctx.NewPlayer(st)
	p.SetBufferSize(int(Buffer.Seconds()*float64(s.Rate)) * 8)
	p.SetVolume(volume)
	p.Play()
	return &Voice{player: p, stream: st, rate: s.Rate}, nil
}

func clampFrame(f, n int) int {
	if f < 0 {
		return 0
	}
	if f > n {
		return n
	}
	return f
}

// Position is where the voice is in its sound, as heard: what has been read
// less what the device still holds.
func (v *Voice) Position() float64 {
	v.stream.mu.Lock()
	pos, n := v.stream.pos, v.stream.cur.Frames()
	v.stream.mu.Unlock()
	pos -= v.player.BufferedSize() / 8
	if pos < 0 {
		pos += n // still the end of the last time round
	}
	return float64(pos) / float64(v.rate)
}

// Playing says whether it is still going.
func (v *Voice) Playing() bool { return v.player.IsPlaying() }

// Stop stops it; it cannot be started again.
func (v *Voice) Stop() {
	v.player.Pause()
	v.player.Close()
}

// Replace carries on with another sound: at the end of the one playing, so
// a loop goes round seamlessly into it, or now, at the same place in it.
func (v *Voice) Replace(s *pcm.Sound, now bool) error {
	if s.Rate != v.rate {
		return fmt.Errorf("audio: the device plays at %d Hz, not %d", v.rate, s.Rate)
	}
	v.stream.mu.Lock()
	defer v.stream.mu.Unlock()
	if now {
		v.stream.cur, v.stream.next = s, nil
		v.stream.pos = clampFrame(v.stream.pos, s.Frames())
	} else {
		v.stream.next = s
	}
	return nil
}

// ---------------------------------------------------------------- in Lua

const voiceType = "audio.voice"

// Open is the audio module, for require.
func Open(L *lua.LState) int {
	mt := L.NewTypeMetatable(voiceType)
	L.SetField(mt, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"stop": func(L *lua.LState) int {
			checkVoice(L).Stop()
			return 0
		},
		"playing": func(L *lua.LState) int {
			L.Push(lua.LBool(checkVoice(L).Playing()))
			return 1
		},
		"position": func(L *lua.LState) int {
			L.Push(lua.LNumber(checkVoice(L).Position()))
			return 1
		},
		"volume": func(L *lua.LState) int {
			v := checkVoice(L)
			if L.GetTop() >= 2 {
				v.player.SetVolume(clamp(float64(L.CheckNumber(2))))
			}
			L.Push(lua.LNumber(v.player.Volume()))
			return 1
		},
		"replace": func(L *lua.LState) int {
			v := checkVoice(L)
			if err := v.Replace(pcm.Check(L, 2), L.OptString(3, "loop") == "now"); err != nil {
				L.RaiseError("%s", err.Error())
			}
			return 0
		},
	}))
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"play":      luaPlay,
		"available": luaAvailable,
	})
	L.Push(mod)
	return 1
}

func clamp(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func checkVoice(L *lua.LState) *Voice {
	ud := L.CheckUserData(1)
	v, ok := ud.Value.(*Voice)
	if !ok {
		L.ArgError(1, "voice expected")
	}
	return v
}

// audio.play(sound [, { loop, volume, from }]) plays it, and is the voice;
// nil and why when there is no sound to be had.
func luaPlay(L *lua.LState) int {
	s := pcm.Check(L, 1)
	loop, volume, from := false, 1.0, 0.0
	if opts, ok := L.Get(2).(*lua.LTable); ok {
		loop = lua.LVAsBool(opts.RawGetString("loop"))
		if n, ok := opts.RawGetString("volume").(lua.LNumber); ok {
			volume = clamp(float64(n))
		}
		if n, ok := opts.RawGetString("from").(lua.LNumber); ok {
			from = float64(n)
		}
	}
	v, err := Play(s, loop, volume, from)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	ud := L.NewUserData()
	ud.Value = v
	L.SetMetatable(ud, L.GetTypeMetatable(voiceType))
	L.Push(ud)
	return 1
}

// audio.available() says whether sound can be played, opening the device at
// rate (44100 unless said) to find out; false and why when not.
func luaAvailable(L *lua.LState) int {
	if _, err := open(L.OptInt(1, 44100)); err != nil {
		L.Push(lua.LFalse)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LTrue)
	return 1
}
