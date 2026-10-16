// Package pcm is a sound as its samples, the way modules hand sounds to one
// another: the beepbox module renders one, and the audio module plays it.
//
// In Lua a sound is a userdata with its length and rate, written out as a
// WAV file or kept as one in a string:
//
//	local s = beepbox.render(song)
//	print(s.seconds, s.rate, s.frames)
//	s:save("song.wav")
//	local bytes = s:wav()
package pcm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"

	lua "github.com/yuin/gopher-lua"
)

// Sound is stereo samples between -1 and 1, Rate a second.
type Sound struct {
	Left, Right []float32
	Rate        int
}

// Frames is how many samples each side has.
func (s *Sound) Frames() int { return len(s.Left) }

// Seconds is how long it plays for.
func (s *Sound) Seconds() float64 {
	if s.Rate <= 0 {
		return 0
	}
	return float64(s.Frames()) / float64(s.Rate)
}

// WAV is the sound as a 16-bit stereo WAV file.
func (s *Sound) WAV() []byte {
	n := s.Frames()
	var b bytes.Buffer
	data := uint32(n * 4)
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, 36+data)
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(&b, binary.LittleEndian, uint16(2))
	binary.Write(&b, binary.LittleEndian, uint32(s.Rate))
	binary.Write(&b, binary.LittleEndian, uint32(s.Rate*4))
	binary.Write(&b, binary.LittleEndian, uint16(4))
	binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, data)
	out := make([]byte, 4)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint16(out[0:], uint16(toInt16(s.Left[i])))
		binary.LittleEndian.PutUint16(out[2:], uint16(toInt16(s.Right[i])))
		b.Write(out)
	}
	return b.Bytes()
}

func toInt16(v float32) int16 {
	if v != v { // NaN
		return 0
	}
	f := math.Round(float64(v) * 32767)
	if f > 32767 {
		f = 32767
	} else if f < -32768 {
		f = -32768
	}
	return int16(f)
}

// Peak is the loudest sample, 0 for silence.
func (s *Sound) Peak() float64 {
	var p float32
	for i := range s.Left {
		if a := abs(s.Left[i]); a > p {
			p = a
		}
		if a := abs(s.Right[i]); a > p {
			p = a
		}
	}
	return float64(p)
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// ---------------------------------------------------------------- in Lua

const typeName = "pcm.sound"

// To is the sound a Lua value holds, if it holds one.
func To(v lua.LValue) (*Sound, bool) {
	ud, ok := v.(*lua.LUserData)
	if !ok {
		return nil, false
	}
	s, ok := ud.Value.(*Sound)
	return s, ok
}

// Check is argument n as a sound, or an argument error.
func Check(L *lua.LState, n int) *Sound {
	s, ok := To(L.Get(n))
	if !ok {
		L.ArgError(n, "sound expected")
	}
	return s
}

// Push puts a sound on the stack as a userdata.
func Push(L *lua.LState, s *Sound) {
	ud := L.NewUserData()
	ud.Value = s
	L.SetMetatable(ud, metatable(L))
	L.Push(ud)
}

func metatable(L *lua.LState) lua.LValue {
	if mt := L.GetTypeMetatable(typeName); mt != lua.LNil {
		return mt
	}
	mt := L.NewTypeMetatable(typeName)
	methods := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		// wav is the sound as a WAV file, in a string.
		"wav": func(L *lua.LState) int {
			L.Push(lua.LString(Check(L, 1).WAV()))
			return 1
		},
		// save writes it as a WAV file.
		"save": func(L *lua.LState) int {
			s, path := Check(L, 1), L.CheckString(2)
			if err := os.WriteFile(path, s.WAV(), 0o644); err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			L.Push(lua.LTrue)
			return 1
		},
		// peak is its loudest sample, 0 to 1.
		"peak": func(L *lua.LState) int {
			L.Push(lua.LNumber(Check(L, 1).Peak()))
			return 1
		},
	})
	L.SetField(mt, "__index", L.NewFunction(func(L *lua.LState) int {
		s := Check(L, 1)
		switch L.CheckString(2) {
		case "frames":
			L.Push(lua.LNumber(s.Frames()))
		case "rate":
			L.Push(lua.LNumber(s.Rate))
		case "seconds":
			L.Push(lua.LNumber(s.Seconds()))
		default:
			L.Push(L.GetField(methods, L.CheckString(2)))
		}
		return 1
	}))
	L.SetField(mt, "__tostring", L.NewFunction(func(L *lua.LState) int {
		s := Check(L, 1)
		L.Push(lua.LString(fmt.Sprintf("sound %.2fs at %d Hz", s.Seconds(), s.Rate)))
		return 1
	}))
	return mt
}
