# Sound for programs with a window

Two modules make music for a gui program: `beepbox` renders a song to a
sound, and `audio` plays a sound out of the speakers. The console
(`tlua play`) has its own sound, `sfx()` and `music()`, and does not use
them.

## beepbox

```lua
local beepbox = require "beepbox"
local sound, skipped = beepbox.render(json)   -- nil and why for what is not a song
sound:save("song.wav")                         -- or sound:wav(), the file as a string
local job = beepbox.start(json)                -- the same, beside the program
if job:done() then sound = job:result() end
local note = beepbox.note(json, 1, 1, 36)      -- channel, instrument, pitch: a third of a second
for _, p in ipairs(beepbox.presets()) do print(p.category, p.name) end
```

A song is [BeepBox](https://www.beepbox.co)'s song JSON, as BeepBox exports
it and [fz](https://github.com/gh0st42/fz) keeps it in `assets/bgm`. It is
rendered by fz's port of BeepBox 4.2.2's synthesiser, the Go module
`github.com/gh0st42/fz/bbx`, which is checked against BeepBox itself: all
nine instrument types, the effects, envelopes and transitions. A song of a
few minutes renders in well under a second, which `beepbox.start` keeps
off the program's thread; poll `job:done()` from a `gui.every` timer.

`beepbox.presets()` is BeepBox's library of 176 named instruments, each
with its settings as JSON; `beepbox.match(settings)` names the preset an
instrument is. `beepbox.scales`, `beepbox.keys` and `beepbox.rhythms` are
BeepBox's tables, and `maxPitch`, `drums`, `pitchChannels` and
`noiseChannels` its limits. A note's pitch counts from the song's key.

## audio

```lua
local audio = require "audio"
local voice = audio.play(sound, { loop = true, volume = 0.8, from = 4 })
voice:position()             -- seconds into the sound, as heard
voice:replace(newSound)      -- carries on with it at the loop's end; "now" for straight away
voice:stop()
```

The device opens at the first sound played, at that sound's rate, and every
sound after must have the same rate (beepbox's are all 44100). Voices mix,
so a note can be heard over a song. The position takes away what the device
still holds, so a playhead drawn from it is where the music is. `replace`
at the loop's end is what lets an editor play a loop without a gap while the
song is rendered again after each change.

It is [oto](https://github.com/ebitengine/oto), which needs no cgo on any
platform: on Linux it finds ALSA or PulseAudio when it runs.
`audio.available()` says whether there is sound to be had, and why not.
