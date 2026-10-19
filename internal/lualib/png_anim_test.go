package lualib

import (
	"path/filepath"
	"testing"
)

// The pictures in testdata/anim are six frames of a coloured circle that
// moves (centre 20+18i, 25+12*(i mod 3), colour 40i+20, 220-30i, 160) and a
// white box at the corner, on nothing, made by ImageMagick, ffmpeg and
// libwebp's img2webp: a GIF that clears to the background after each frame,
// an APNG with frames cut down to what changed, and WebPs, lossless and
// lossy, with frames cut down too.
func TestPngFrames(t *testing.T) {
	dir, err := filepath.Abs("testdata/anim")
	if err != nil {
		t.Fatal(err)
	}
	runIn(t, map[string]string{"dir": dir}, `
		local png = require "png"

		-- check says the frames are the six pictures, to within tol in each colour.
		local function check(name, frames, delay, tol)
			assert(frames, name .. ": no frames")
			assert(#frames == 6, name .. ": " .. #frames .. " frames")
			assert(frames.width == 140 and frames.height == 100, name .. ": size")
			assert(frames.loops == 0 and frames.truncated == false, name .. ": loops")
			for i = 0, 5 do
				local f = frames[i + 1]
				assert(math.abs(f.delay - delay) < 1e-6, name .. ": delay " .. f.delay)
				assert(f.image.width == 140 and f.image.height == 100)
				local cx, cy = 20 + 18 * i, 25 + 12 * (i % 3)
				local r, g, b, a = f.image:get(cx, cy)
				assert(a == 255, name .. ": the circle is there in frame " .. i)
				assert(math.abs(r - (40 * i + 20)) <= tol and math.abs(g - (220 - 30 * i)) <= tol
					and math.abs(b - 160) <= tol, name .. ": its colour in frame " .. i .. ": " .. r .. "," .. g .. "," .. b)
				if i >= 2 then
					local ox, oy = 20 + 18 * (i - 2), 25 + 12 * ((i - 2) % 3)
					assert(select(4, f.image:get(ox, oy)) == 0, name .. ": the circle of two frames ago is gone in frame " .. i)
				end
				assert(select(4, f.image:get(139, 99)) == 0, name .. ": nothing at the corner")
				local br, _, _, ba = f.image:get(15, 10)
				assert(ba == 255 and br >= 255 - math.max(tol, 8), name .. ": the white box stays in frame " .. i)
			end
		end

		check("gif", png.frames(dir .. "/ag.gif"), 0.12, 12)
		check("apng", png.frames(dir .. "/a.apng"), 0.125, 2)
		check("lossless webp", png.frames(dir .. "/a_lossless.webp"), 0.125, 0)
		check("lossy webp", png.frames(dir .. "/a_lossy.webp"), 0.125, 24)

		-- The same from the bytes.
		local f = assert(io.open(dir .. "/a_lossless.webp", "rb"))
		local data = f:read("*a"); f:close()
		check("decodeFrames", png.decodeFrames(data), 0.125, 0)

		-- A picture that does not move has one frame, with no delay; WebP stills read.
		local still = assert(png.frames(dir .. "/s.webp"))
		assert(#still == 1 and still[1].delay == 0 and still.width == 160 and still.height == 120)
		assert(still.loops == 0 and still.truncated == false)
		local one = assert(png.decodeFrames(png.new(8, 4, "#ff0000"):encode()))
		assert(#one == 1 and one[1].delay == 0 and one.width == 8 and one[1].image:get(0, 0) == 255)

		-- png.load reads a WebP, and gives the first frame of one that moves.
		local webp = assert(png.load(dir .. "/s.webp"))
		assert(webp.width == 160 and webp.height == 120)
		local first = assert(png.load(dir .. "/a_lossless.webp"))
		assert(first:pixels() == png.frames(dir .. "/a_lossless.webp")[1].image:pixels())
		assert(png.load(dir .. "/ag.gif").width == 140)

		-- What cannot be read says why.
		local nothing, why = png.decodeFrames("garbage")
		assert(nothing == nil and why)
		nothing, why = png.frames(dir .. "/none.gif")
		assert(nothing == nil and why)
		local cut = data:sub(1, #data - 40)
		nothing, why = png.decodeFrames(cut)
		assert(nothing == nil and why, "a cut-off file")
	`)
}

// Frames take memory, a whole picture each: past a limit, only the first is
// kept, and the table says so.
func TestPngFramesTruncated(t *testing.T) {
	dir, err := filepath.Abs("testdata/anim")
	if err != nil {
		t.Fatal(err)
	}
	saved := maxAnimationBytes
	defer func() { maxAnimationBytes = saved }()
	maxAnimationBytes = 140 * 100 * 4 * 3 // room for three frames of six
	runIn(t, map[string]string{"dir": dir}, `
		local png = require "png"
		for _, name in ipairs { "ag.gif", "a.apng", "a_lossless.webp", "a_lossy.webp" } do
			local frames = assert(png.frames(dir .. "/" .. name))
			assert(#frames == 1 and frames.truncated == true, name)
			assert(frames.width == 140 and frames.height == 100 and frames[1].delay == 0, name .. ": a still, with no delay")
		end
	`)
	maxAnimationBytes = 140 * 100 * 4 / 2 // not even one
	runIn(t, map[string]string{"dir": dir}, `
		local png = require "png"
		for _, name in ipairs { "ag.gif", "a.apng", "a_lossless.webp" } do
			local frames, why = png.frames(dir .. "/" .. name)
			assert(frames == nil and why:find("too big"), name .. ": " .. tostring(why))
		end
	`)
}
