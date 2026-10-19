---@meta png
--- Pictures, `require "png"`: read from PNG, JPEG, GIF and WebP, made, changed
--- and written as PNG, by any program. A gui Canvas draws one with g:image.
--- `png.frames` reads the animated ones (GIF, APNG, WebP) a frame at a time.

local png = {}

--- A picture of width by height pixels, each r, g, b, a from 0 to 255.
--- Coordinates count from 0, from the top left.
---@class png.Image
---@field width integer
---@field height integer
local Image = {}

--- The colour at x, y: r, g, b, a; nothing outside the picture.
---@param x integer
---@param y integer
---@return integer? r
---@return integer? g
---@return integer? b
---@return integer? a
function Image:get(x, y) end

--- Sets x, y to r, g, b [, a], or to "#rrggbb" or "#rrggbbaa". A point
--- outside the picture is left alone.
---@param x integer
---@param y integer
---@param r integer|string
---@param g? integer
---@param b? integer
---@param a? integer
function Image:set(x, y, r, g, b, a) end

--- Fills all of it with a colour, or with ("#rrggbb", x, y, w, h) a
--- rectangle.
---@param color string|integer
---@param ... integer
function Image:fill(color, ...) end

--- width*height*4 bytes of r, g, b, a, a row at a time from the top.
---@return string
function Image:pixels() end

---@return png.Image
function Image:clone() end

--- A new picture of the part at x, y of w by h.
---@return png.Image
function Image:crop(x, y, w, h) end

--- The picture as the bytes of a PNG file.
---@return string? data
---@return string? why
function Image:encode() end

--- Writes a PNG file.
---@param path string
---@return true? ok
---@return string? why
function Image:save(path) end

--- The picture in a PNG, JPEG, GIF or WebP file, read out of a fused program
--- or bundle first; nil and why. An animation gives its first frame: see
--- `png.frames` for the others.
---@param path string
---@return png.Image? image
---@return string? why
function png.load(path) end

--- The same, from the file's bytes.
---@param data string
---@return png.Image? image
---@return string? why
function png.decode(data) end

--- One frame of an animation: the whole picture as it looks then, so a frame
--- is shown without the ones before it, and how long it stays, in seconds.
---@class png.Frame
---@field image png.Image
---@field delay number seconds; 0 for a picture that does not move

--- The frames of a picture: a list of frames, with the picture's `width` and
--- `height`, `loops` (how many times it plays in all; 0 for ever) and `truncated`
--- (true when it was too big to hold, and only its first frame is here).
--- It reads animated GIF, APNG and WebP, and gives a picture that does not
--- move (of any format `png.load` reads) as a list of one frame with no
--- delay. A delay of a hundredth of a second or less is given as a tenth,
--- as browsers show it. The frames take 4 bytes a pixel each, and an
--- animation that would take more than 256 MB gives its first frame only.
---@class png.Frames
---@field width integer
---@field height integer
---@field loops integer
---@field truncated boolean
---@field [integer] png.Frame

--- The frames of the picture in a file, read out of a fused program or
--- bundle first; nil and why.
---@param path string
---@return png.Frames? frames
---@return string? why
function png.frames(path) end

--- The same, from the file's bytes.
---@param data string
---@return png.Frames? frames
---@return string? why
function png.decodeFrames(data) end

--- A picture all of one colour: transparent unless one is given.
---@param width integer
---@param height integer
---@param color? string|integer
---@param ... integer
---@return png.Image
function png.new(width, height, color, ...) end

--- A picture from width*height*4 bytes, as Image:pixels() gives them.
---@param width integer
---@param height integer
---@param rgba string
---@return png.Image
function png.fromPixels(width, height, rgba) end

---@param image png.Image
---@return string? data
---@return string? why
function png.encode(image) end

return png
