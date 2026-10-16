package lualib

// beepbox is BeepBox's songs, played: the song JSON BeepBox exports (and fz
// keeps in assets/bgm) rendered to a sound by fz's port of BeepBox's own
// synthesiser, the bbx module, which is checked against BeepBox itself.
//
//	local beepbox = require "beepbox"
//	local sound = beepbox.render(json)          -- or nil and why
//	sound:save("song.wav")
//	local job = beepbox.start(json)             -- the same, without waiting
//	if job:done() then sound = job:result() end
//	local note = beepbox.note(json, 1, 1, 24)   -- channel, instrument, pitch
//	for _, p in ipairs(beepbox.presets()) do print(p.category, p.name) end
//
// A sound is a pcm sound, which the audio module plays.

import (
	"encoding/json"
	"sync"

	"github.com/gh0st42/fz/bbx"
	lua "github.com/yuin/gopher-lua"

	"tlua/internal/pcm"
)

const beepboxJobType = "beepbox.job"

func openBeepbox(L *lua.LState) int {
	jobMT := L.NewTypeMetatable(beepboxJobType)
	L.SetField(jobMT, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"done":   jobDone,
		"result": jobResult,
	}))
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"render":  beepboxRender,
		"start":   beepboxStart,
		"note":    beepboxNote,
		"presets": beepboxPresets,
		"match":   beepboxMatch,
	})
	L.SetField(mod, "rate", lua.LNumber(bbx.SampleRate))
	L.SetField(mod, "maxPitch", lua.LNumber(bbx.MaxPitch))
	L.SetField(mod, "drums", lua.LNumber(bbx.DrumCount))
	L.SetField(mod, "pitchChannels", lua.LNumber(bbx.PitchChannelMax))
	L.SetField(mod, "noiseChannels", lua.LNumber(bbx.NoiseChannelMax))
	L.SetField(mod, "scales", beepboxScales(L))
	L.SetField(mod, "keys", beepboxKeys(L))
	L.SetField(mod, "rhythms", beepboxRhythms(L))
	L.Push(mod)
	return 1
}

// scales are BeepBox's, as a song names them ("normal :)"), each with its
// usual name ("ionian") and the twelve semitones above the key it has, as
// booleans from 1 (the key itself) to 12.
func beepboxScales(L *lua.LState) *lua.LTable {
	t := L.NewTable()
	for _, sc := range bbx.Scales {
		e := L.NewTable()
		L.SetField(e, "name", lua.LString(sc.Name))
		L.SetField(e, "label", lua.LString(sc.RealName))
		notes := L.NewTable()
		for _, on := range sc.Flags {
			notes.Append(lua.LBool(on))
		}
		L.SetField(e, "notes", notes)
		t.Append(e)
	}
	return t
}

// keys are the twelve a song can be in, C first, as a song names them.
func beepboxKeys(L *lua.LState) *lua.LTable {
	t := L.NewTable()
	for _, k := range bbx.Keys {
		e := L.NewTable()
		L.SetField(e, "name", lua.LString(k.Name))
		L.SetField(e, "white", lua.LBool(k.IsWhiteKey))
		t.Append(e)
	}
	return t
}

// rhythms are how a beat divides, by the steps a note's ticks count.
func beepboxRhythms(L *lua.LState) *lua.LTable {
	t := L.NewTable()
	for _, r := range bbx.Rhythms {
		e := L.NewTable()
		L.SetField(e, "name", lua.LString(r.Name))
		L.SetField(e, "steps", lua.LNumber(r.StepsPerBeat))
		t.Append(e)
	}
	return t
}

// render renders a song, as a sound and the instrument types it could not
// play; nil and why for what is not a song.
func render(data string) (*pcm.Sound, []string, error) {
	song, err := bbx.ParseSong([]byte(data))
	if err != nil {
		return nil, nil, err
	}
	res, err := bbx.Render(song)
	if err != nil {
		return nil, nil, err
	}
	return &pcm.Sound{Left: res.Left, Right: res.Right, Rate: bbx.SampleRate}, res.Skipped, nil
}

func pushRendered(L *lua.LState, s *pcm.Sound, skipped []string, err error) int {
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	pcm.Push(L, s)
	t := L.NewTable()
	for _, name := range skipped {
		t.Append(lua.LString(name))
	}
	L.Push(t)
	return 2
}

// beepbox.render(json) is the song as a sound, and the instrument types it
// had to leave out.
func beepboxRender(L *lua.LState) int {
	s, skipped, err := render(L.CheckString(1))
	return pushRendered(L, s, skipped, err)
}

// job is a render going on beside the program.
type job struct {
	mu      sync.Mutex
	done    bool
	sound   *pcm.Sound
	skipped []string
	err     error
}

// beepbox.start(json) renders a song on a goroutine, so a window is not held
// up; job:done() says when it has finished, and job:result() is what
// beepbox.render would have given.
func beepboxStart(L *lua.LState) int {
	data := L.CheckString(1)
	j := &job{}
	go func() {
		s, skipped, err := render(data)
		j.mu.Lock()
		j.sound, j.skipped, j.err, j.done = s, skipped, err, true
		j.mu.Unlock()
	}()
	ud := L.NewUserData()
	ud.Value = j
	L.SetMetatable(ud, L.GetTypeMetatable(beepboxJobType))
	L.Push(ud)
	return 1
}

func checkJob(L *lua.LState) *job {
	ud := L.CheckUserData(1)
	j, ok := ud.Value.(*job)
	if !ok {
		L.ArgError(1, "beepbox job expected")
	}
	return j
}

func jobDone(L *lua.LState) int {
	j := checkJob(L)
	j.mu.Lock()
	defer j.mu.Unlock()
	L.Push(lua.LBool(j.done))
	return 1
}

func jobResult(L *lua.LState) int {
	j := checkJob(L)
	j.mu.Lock()
	done, s, skipped, err := j.done, j.sound, j.skipped, j.err
	j.mu.Unlock()
	if !done {
		L.Push(lua.LNil)
		L.Push(lua.LString("not finished"))
		return 2
	}
	return pushRendered(L, s, skipped, err)
}

// beepbox.note(json, channel, instrument, pitch) is one note of a song's
// instrument, a third of a second of it, the way the song would play it:
// for an editor to sound a note as it is placed. Channels and instruments
// count from 1; pitch is a note's pitch as the song stores it.
func beepboxNote(L *lua.LState) int {
	song, err := bbx.ParseSong([]byte(L.CheckString(1)))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	res := bbx.AuditionNote(song, L.CheckInt(2)-1, L.OptInt(3, 1)-1, L.CheckInt(4))
	if res == nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("nothing to play there"))
		return 2
	}
	pcm.Push(L, &pcm.Sound{Left: res.Left, Right: res.Right, Rate: bbx.SampleRate})
	return 1
}

// beepbox.presets() is BeepBox's library of named instruments, in the order
// its menu has them: category, name, noise (for a drum channel), the
// instrument's settings as JSON, and whether it can be played here.
func beepboxPresets(L *lua.LState) int {
	bbx.LoadPresets()
	t := L.NewTable()
	for _, p := range bbx.PresetList {
		e := L.NewTable()
		L.SetField(e, "category", lua.LString(bbx.CategoryLabel(p.Category)))
		L.SetField(e, "name", lua.LString(p.Name))
		L.SetField(e, "noise", lua.LBool(p.Noise))
		L.SetField(e, "settings", lua.LString(p.Settings))
		L.SetField(e, "type", lua.LString(p.Type()))
		L.SetField(e, "playable", lua.LBool(p.Playable()))
		t.Append(e)
	}
	L.Push(t)
	return 1
}

// beepbox.match(instrument) is the name of the preset an instrument's
// settings (JSON) are, or nil for one of its own.
func beepboxMatch(L *lua.LState) int {
	var inst bbx.Instrument
	if json.Unmarshal([]byte(L.CheckString(1)), &inst) != nil {
		L.Push(lua.LNil)
		return 1
	}
	if name, ok := bbx.MatchPreset(inst); ok {
		L.Push(lua.LString(name))
		return 1
	}
	L.Push(lua.LNil)
	return 1
}
