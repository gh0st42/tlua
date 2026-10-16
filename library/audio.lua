---@meta audio
--- Sounds out of the speakers, `require "audio"`, for a program with a
--- window of its own: a gui program. The console (tlua play) has its own
--- sound, sfx() and music(). Sounds come from beepbox.render and the like.
--- The device opens at the first sound played, at its rate, and every
--- sound after has to have the same rate.

local audio = {}

--- A sound playing.
---@class audio.voice
local Voice = {}

--- Where it has got to in its sound, in seconds, as heard: round again when
--- it loops.
---@return number
function Voice:position() end

---@return boolean
function Voice:playing() end

--- Stops it; it cannot be started again.
function Voice:stop() end

--- Its volume, 0 to 1, set to v when given.
---@param v? number
---@return number
function Voice:volume(v) end

--- Carries on with another sound: at the end of the one playing ("loop",
--- as it is unless said), so that a loop goes round into it without a gap,
--- or "now", at the same place in it.
---@param sound pcm.sound
---@param when? "loop"|"now"
function Voice:replace(sound, when) end

---@class audio.PlayOptions
---@field loop? boolean round again at the end
---@field volume? number 0 to 1
---@field from? number where to start, in seconds

--- Plays a sound; nil and why when there is no sound to be had.
---@param sound pcm.sound
---@param opts? audio.PlayOptions
---@return audio.voice? voice
---@return string? why
function audio.play(sound, opts) end

--- Whether sounds can be played, opening the device at rate (44100) to find
--- out; false and why when not.
---@param rate? integer
---@return boolean ok
---@return string? why
function audio.available(rate) end

return audio
