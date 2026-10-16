package lualib

import "testing"

// A song of one bar: a chip square playing C, E, G, C an octave up, and a
// drum channel with one hit, as BeepBox exports it.
const beepboxSong = `{
  "format": "BeepBox", "version": 9, "scale": "Free", "key": "C",
  "introBars": 0, "loopBars": 1, "beatsPerBar": 4, "ticksPerBeat": 4,
  "beatsPerMinute": 120, "layeredInstruments": false, "patternInstruments": false,
  "channels": [
    { "type": "pitch",
      "instruments": [{ "type": "chip", "volume": 100, "eqFilter": [], "effects": [],
        "fadeInSeconds": 0, "fadeOutTicks": -1, "wave": "square", "unison": "none", "envelopes": [] }],
      "patterns": [{ "notes": [
        { "pitches": [24], "points": [{ "tick": 0, "pitchBend": 0, "volume": 100 }, { "tick": 4, "pitchBend": 0, "volume": 100 }] },
        { "pitches": [28], "points": [{ "tick": 4, "pitchBend": 0, "volume": 100 }, { "tick": 8, "pitchBend": 0, "volume": 100 }] },
        { "pitches": [31], "points": [{ "tick": 8, "pitchBend": 0, "volume": 100 }, { "tick": 12, "pitchBend": 0, "volume": 100 }] },
        { "pitches": [36], "points": [{ "tick": 12, "pitchBend": 0, "volume": 100 }, { "tick": 16, "pitchBend": 0, "volume": 100 }] }
      ] }],
      "sequence": [1], "octaveScrollBar": 3 },
    { "type": "drum",
      "instruments": [{ "type": "noise", "volume": 100, "eqFilter": [], "effects": [],
        "fadeInSeconds": 0, "fadeOutTicks": -1, "wave": "white", "envelopes": [] }],
      "patterns": [{ "notes": [
        { "pitches": [4], "points": [{ "tick": 0, "pitchBend": 0, "volume": 100 }, { "tick": 2, "pitchBend": 0, "volume": 100 }] }
      ] }],
      "sequence": [1] }
  ]
}`

func TestBeepboxRendersASong(t *testing.T) {
	runIn(t, map[string]string{"song": beepboxSong, "dir": t.TempDir()}, `
local beepbox = require "beepbox"
assert(beepbox.rate == 44100)
local s, skipped = assert(beepbox.render(song))
assert(#skipped == 0)
-- A bar of four beats at 120 is two seconds.
assert(math.abs(s.seconds - 2) < 0.01, s.seconds)
assert(s.rate == 44100 and s.frames == math.floor(s.seconds * 44100 + 0.5))
assert(s:peak() > 0.01, "it is not silent")
assert(tostring(s):find("^sound 2.00s"))
local wav = s:wav()
assert(wav:sub(1, 4) == "RIFF" and #wav == 44 + s.frames * 4)
assert(s:save(dir .. "/song.wav"))
assert(io.open(dir .. "/song.wav"):read("*a") == wav)

-- The same without waiting.
local job = beepbox.start(song)
local t = os.clock()
while not job:done() do assert(os.clock() - t < 10, "never finished") end
local again = assert(job:result())
assert(again.frames == s.frames)

-- What is not a song says so.
local none, why = beepbox.render('{"format":"Other"}')
assert(none == nil and why:find("not a BeepBox song"))
assert(beepbox.render("{") == nil)
local bad = beepbox.start("{")
while not bad:done() do end
assert(bad:result() == nil, "a job says what went wrong too")
`)
}

func TestBeepboxSoundsANoteAndKnowsItsPresets(t *testing.T) {
	runIn(t, map[string]string{"song": beepboxSong}, `
local beepbox = require "beepbox"
local note = assert(beepbox.note(song, 1, 1, 24))
assert(note.seconds > 0.3 and note.seconds < 0.4 and note:peak() > 0.01)
assert(beepbox.note(song, 2, 1, 4):peak() > 0, "a drum too")
assert(beepbox.note(song, 9, 1, 24) == nil, "no such channel")

assert(#beepbox.scales == 12 and beepbox.scales[7].name == "normal :)" and beepbox.scales[7].label == "ionian")
assert(beepbox.scales[7].notes[1] and not beepbox.scales[7].notes[2] and #beepbox.scales[7].notes == 12)
assert(#beepbox.keys == 12 and beepbox.keys[1].name == "C" and not beepbox.keys[2].white)
assert(beepbox.rhythms[2].steps == 4 and beepbox.maxPitch == 84 and beepbox.drums == 12)
assert(beepbox.pitchChannels == 10 and beepbox.noiseChannels == 5)

local presets = beepbox.presets()
assert(#presets == 176, #presets)
local first = presets[1]
assert(first.category == "Retro" and first.name == "square wave" and first.noise == false)
assert(first.playable and first.type == "chip" and first.settings:find('"wave"'))
local drums = 0
for _, p in ipairs(presets) do if p.noise then drums = drums + 1 end end
assert(drums > 0 and drums < 176)
-- An instrument is known by its settings.
assert(beepbox.match(presets[10].settings) == presets[10].name)
assert(beepbox.match('{"type":"chip","wave":"square","volume":3}') == nil)
assert(beepbox.match("not json") == nil)
`)
}
