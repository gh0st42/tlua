package lualib

// png is tlua's own: pictures read, made, changed and written, by any
// program, the console's or not.
//
//	local png = require "png"
//	local img = png.load("photo.png")       -- PNG, JPEG or GIF
//	print(img.width, img.height)
//	local r, g, b, a = img:get(0, 0)          -- 0 to 255, from the top left
//	img:set(10, 10, 255, 0, 0)                -- or img:set(10, 10, "#ff0000")
//	img:save("out.png")
//	local copy = png.new(64, 64, "#ffffff")
//
// A gui Canvas draws one with g:image(img, x, y).

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

const imageType = "png.image"

// Image is a picture made by the png module. Version counts its changes,
// so that whoever shows it (a gui Canvas) knows when to look again.
type Image struct {
	Pix     *image.NRGBA
	Version int
}

// ToImage reports the picture a Lua value is, if it is one.
func ToImage(v lua.LValue) (*Image, bool) {
	ud, ok := v.(*lua.LUserData)
	if !ok {
		return nil, false
	}
	img, ok := ud.Value.(*Image)
	return img, ok
}

func openPng(L *lua.LState) int {
	mt := L.NewTypeMetatable(imageType)
	methods := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"get":    imageGet,
		"set":    imageSet,
		"fill":   imageFill,
		"pixels": imagePixels,
		"clone":  imageClone,
		"crop":   imageCrop,
		"encode": imageEncode,
		"save":   imageSave,
	})
	L.SetField(mt, "__index", L.NewFunction(func(L *lua.LState) int {
		img := checkImage(L, 1)
		switch L.CheckString(2) {
		case "width":
			L.Push(lua.LNumber(img.Pix.Rect.Dx()))
		case "height":
			L.Push(lua.LNumber(img.Pix.Rect.Dy()))
		default:
			L.Push(L.GetField(methods, L.CheckString(2)))
		}
		return 1
	}))
	L.SetField(mt, "__tostring", L.NewFunction(func(L *lua.LState) int {
		img := checkImage(L, 1)
		L.Push(lua.LString(fmt.Sprintf("image %dx%d", img.Pix.Rect.Dx(), img.Pix.Rect.Dy())))
		return 1
	}))

	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"load":       pngLoad,
		"decode":     pngDecode,
		"new":        pngNew,
		"fromPixels": pngFromPixels,
		"encode":     imageEncode,
	})
	L.Push(mod)
	return 1
}

func pushImage(L *lua.LState, pix *image.NRGBA) {
	ud := L.NewUserData()
	ud.Value = &Image{Pix: pix}
	L.SetMetatable(ud, L.GetTypeMetatable(imageType))
	L.Push(ud)
}

func checkImage(L *lua.LState, n int) *Image {
	img, ok := ToImage(L.Get(n))
	if !ok {
		L.ArgError(n, "image expected")
	}
	return img
}

// toNRGBA copies any picture into the one shape the module works in.
func toNRGBA(src image.Image) *image.NRGBA {
	if n, ok := src.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Rect, src, b.Min, draw.Src)
	return dst
}

func decodeImage(L *lua.LState, data []byte) int {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	pushImage(L, toNRGBA(src))
	return 1
}

// pngLoad is png.load(path): the picture in a PNG, JPEG or GIF file, or
// nil and why. A fused program or a bundle reads its own files first.
func pngLoad(L *lua.LState) int {
	data, err := readFile(L, L.CheckString(1))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	return decodeImage(L, data)
}

// pngDecode is png.decode(data): the same, from the file's bytes.
func pngDecode(L *lua.LState) int {
	return decodeImage(L, []byte(L.CheckString(1)))
}

func checkSize(L *lua.LState, n int) int {
	v := L.CheckInt(n)
	if v < 1 || v > 1<<15 {
		L.ArgError(n, "a size from 1 to 32768")
	}
	return v
}

// pngNew is png.new(width, height [, colour]): a picture all of one
// colour, transparent black unless one is given.
func pngNew(L *lua.LState) int {
	w, h := checkSize(L, 1), checkSize(L, 2)
	pix := image.NewNRGBA(image.Rect(0, 0, w, h))
	if L.GetTop() >= 3 {
		c := checkColor(L, 3)
		fillNRGBA(pix, c)
	}
	pushImage(L, pix)
	return 1
}

// pngFromPixels is png.fromPixels(width, height, rgba): a picture from
// width*height*4 bytes, a row at a time from the top, as img:pixels()
// gives them.
func pngFromPixels(L *lua.LState) int {
	w, h := checkSize(L, 1), checkSize(L, 2)
	data := L.CheckString(3)
	if len(data) != w*h*4 {
		L.ArgError(3, fmt.Sprintf("%d bytes for %dx%d, not %d", w*h*4, w, h, len(data)))
	}
	pix := image.NewNRGBA(image.Rect(0, 0, w, h))
	copy(pix.Pix, data)
	pushImage(L, pix)
	return 1
}

// checkColor reads a colour from argument n on: r, g, b [, a] from 0 to
// 255, or a string "#rrggbb" or "#rrggbbaa".
func checkColor(L *lua.LState, n int) color.NRGBA {
	if s, ok := L.Get(n).(lua.LString); ok {
		c, err := parseHexColor(string(s))
		if err != nil {
			L.ArgError(n, err.Error())
		}
		return c
	}
	byteAt := func(i int, def int) uint8 {
		v := L.OptInt(i, def)
		if v < 0 || v > 255 {
			L.ArgError(i, "a colour component from 0 to 255")
		}
		return uint8(v)
	}
	return color.NRGBA{R: byteAt(n, 0), G: byteAt(n+1, 0), B: byteAt(n+2, 0), A: byteAt(n+3, 255)}
}

func parseHexColor(s string) (color.NRGBA, error) {
	hex := strings.TrimPrefix(s, "#")
	if len(hex) != 6 && len(hex) != 8 || len(hex) == len(s) {
		return color.NRGBA{}, fmt.Errorf("a colour is \"#rrggbb\" or \"#rrggbbaa\", not %q", s)
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("a colour is \"#rrggbb\" or \"#rrggbbaa\", not %q", s)
	}
	if len(hex) == 6 {
		v = v<<8 | 0xff
	}
	return color.NRGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}, nil
}

func fillNRGBA(pix *image.NRGBA, c color.NRGBA) {
	for i := 0; i < len(pix.Pix); i += 4 {
		pix.Pix[i], pix.Pix[i+1], pix.Pix[i+2], pix.Pix[i+3] = c.R, c.G, c.B, c.A
	}
}

// point reads x and y at arguments n and n+1, and reports whether they are
// inside the picture.
func point(L *lua.LState, img *Image, n int) (int, int, bool) {
	x, y := L.CheckInt(n), L.CheckInt(n+1)
	return x, y, image.Pt(x, y).In(img.Pix.Rect)
}

// imageGet is img:get(x, y): r, g, b, a from 0 to 255; nothing outside it.
func imageGet(L *lua.LState) int {
	img := checkImage(L, 1)
	x, y, in := point(L, img, 2)
	if !in {
		return 0
	}
	c := img.Pix.NRGBAAt(x, y)
	L.Push(lua.LNumber(c.R))
	L.Push(lua.LNumber(c.G))
	L.Push(lua.LNumber(c.B))
	L.Push(lua.LNumber(c.A))
	return 4
}

// imageSet is img:set(x, y, r, g, b [, a]) or img:set(x, y, "#rrggbb");
// a point outside the picture is left alone.
func imageSet(L *lua.LState) int {
	img := checkImage(L, 1)
	x, y, in := point(L, img, 2)
	c := checkColor(L, 4)
	if in {
		img.Pix.SetNRGBA(x, y, c)
		img.Version++
	}
	return 0
}

// imageFill is img:fill(colour [, x, y, w, h]): all of it, or a rectangle.
func imageFill(L *lua.LState) int {
	img := checkImage(L, 1)
	var c color.NRGBA
	next := 3
	if _, ok := L.Get(2).(lua.LString); ok {
		c = checkColor(L, 2)
	} else {
		c = checkColor(L, 2)
		next = 6
	}
	if L.GetTop() >= next {
		r := image.Rect(L.CheckInt(next), L.CheckInt(next+1),
			L.CheckInt(next)+L.CheckInt(next+2), L.CheckInt(next+1)+L.CheckInt(next+3))
		draw.Draw(img.Pix, r.Intersect(img.Pix.Rect), &image.Uniform{C: c}, image.Point{}, draw.Src)
	} else {
		fillNRGBA(img.Pix, c)
	}
	img.Version++
	return 0
}

// imagePixels is img:pixels(): width*height*4 bytes of r, g, b, a, a row
// at a time from the top.
func imagePixels(L *lua.LState) int {
	img := checkImage(L, 1)
	L.Push(lua.LString(img.Pix.Pix))
	return 1
}

func imageClone(L *lua.LState) int {
	img := checkImage(L, 1)
	pix := image.NewNRGBA(img.Pix.Rect)
	copy(pix.Pix, img.Pix.Pix)
	pushImage(L, pix)
	return 1
}

// imageCrop is img:crop(x, y, w, h): a new picture of that part.
func imageCrop(L *lua.LState) int {
	img := checkImage(L, 1)
	x, y := L.CheckInt(2), L.CheckInt(3)
	r := image.Rect(x, y, x+checkSize(L, 4), y+checkSize(L, 5)).Intersect(img.Pix.Rect)
	if r.Empty() {
		L.ArgError(2, "the rectangle is outside the picture")
	}
	pushImage(L, toNRGBA(img.Pix.SubImage(r)))
	return 1
}

// imageEncode is img:encode(), or png.encode(img): the picture as a PNG
// file's bytes.
func imageEncode(L *lua.LState) int {
	img := checkImage(L, 1)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img.Pix); err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LString(buf.String()))
	return 1
}

// imageSave is img:save(path): writes a PNG; true, or nil and why.
func imageSave(L *lua.LState) int {
	img := checkImage(L, 1)
	path := L.CheckString(2)
	var buf bytes.Buffer
	err := png.Encode(&buf, img.Pix)
	if err == nil {
		err = os.WriteFile(path, buf.Bytes(), 0o644)
	}
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LTrue)
	return 1
}
