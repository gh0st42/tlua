---@meta beepbox
--- BeepBox's songs, played, `require "beepbox"`: the song JSON BeepBox
--- exports (and fz keeps in assets/bgm) rendered to a sound by fz's port of
--- BeepBox's own synthesiser. The audio module plays a sound; sound:save
--- writes it as a WAV file.

local beepbox = {}

--- The rate every sound it renders has: 44100 a second.
beepbox.rate = 44100

--- The highest pitch a note on a pitch channel can have; 0 is the lowest.
beepbox.maxPitch = 84
--- How many drums a drum channel has: pitches 0 to drums - 1.
beepbox.drums = 12
--- How many pitch channels, and drum channels, a song can have.
beepbox.pitchChannels = 10
beepbox.noiseChannels = 5

---@class beepbox.scale
---@field name string as a song names it ("normal :)")
---@field label string its usual name ("ionian")
---@field notes boolean[] the twelve semitones from the key up, whether each is in it

--- BeepBox's scales, in the order its menu has them. A note's pitch counts
--- from the song's key, so pitch % 12 + 1 is where it is in notes.
---@type beepbox.scale[]
beepbox.scales = {}

--- The twelve keys, C first, as a song names them ("C♯"); white is whether
--- it is a white key on a piano.
---@type { name: string, white: boolean }[]
beepbox.keys = {}

--- How a beat can divide: steps is how many ticks a beat has.
---@type { name: string, steps: integer }[]
beepbox.rhythms = {}

--- A sound: stereo samples at a rate. The audio module plays one.
---@class pcm.sound
---@field frames integer samples on each side
---@field rate integer samples a second
---@field seconds number how long it plays for
local Sound = {}

--- The sound as a 16-bit stereo WAV file, in a string.
---@return string
function Sound:wav() end

--- Writes it as a WAV file; nil and why when that fails.
---@param path string
---@return true? ok
---@return string? why
function Sound:save(path) end

--- Its loudest sample, 0 for silence and 1 at full scale.
---@return number
function Sound:peak() end

--- The song as a sound, and the instrument types in it that could not be
--- played; nil and why for what is not a BeepBox song.
---@param json string the song, as BeepBox exports it
---@return pcm.sound? sound
---@return string[]|string skipped the types left out, or why there is no sound
function beepbox.render(json) end

--- A render going on beside the program.
---@class beepbox.job
local Job = {}

--- Whether it has finished.
---@return boolean
function Job:done() end

--- What beepbox.render would have given, once it has finished.
---@return pcm.sound? sound
---@return string[]|string skipped
function Job:result() end

--- Renders a song on its own, so that a window is not held up: poll
--- job:done() from a timer, then take job:result().
---@param json string
---@return beepbox.job
function beepbox.start(json) end

--- One note of a song's instrument, a third of a second of it, the way the
--- song would play it: for an editor to sound a note as it is placed.
---@param json string the song
---@param channel integer from 1
---@param instrument? integer from 1; 1 unless given
---@param pitch integer a note's pitch, as the song stores it
---@return pcm.sound? sound
---@return string? why
function beepbox.note(json, channel, instrument, pitch) end

---@class beepbox.preset
---@field category string its group, as BeepBox's menu names it ("Retro")
---@field name string
---@field noise boolean for a drum channel
---@field settings string the instrument, as JSON
---@field type string its synthesis type ("chip", "FM", ...)
---@field playable boolean

--- BeepBox's library of 176 named instruments, in the order its menu has
--- them.
---@return beepbox.preset[]
function beepbox.presets() end

--- The name of the preset an instrument is, by its settings (JSON); nil for
--- one of its own.
---@param instrument string
---@return string?
function beepbox.match(instrument) end

return beepbox
