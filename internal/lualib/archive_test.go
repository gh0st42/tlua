package lualib

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"hash/adler32"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// runIn is run with globals set first.
func runIn(t *testing.T, globals map[string]string, src string) *lua.LState {
	t.Helper()
	L := lua.NewState()
	t.Cleanup(L.Close)
	Open(L)
	for k, v := range globals {
		L.SetGlobal(k, lua.LString(v))
	}
	if err := L.DoString(src); err != nil {
		t.Fatal(err)
	}
	return L
}

func TestZlibAsLuaZlibHasIt(t *testing.T) {
	run(t, `
		local zlib = require "zlib"
		local data = ("hello tlua "):rep(1000)
		local s = zlib.deflate(9)
		local z = s(data:sub(1, 5000)) .. s(data:sub(5001), "finish")
		local inf = zlib.inflate()
		local a, eof1 = inf(z:sub(1, 30))
		local b, eof2, bin, bout = inf(z:sub(31) .. "TRAILER")
		assert(a .. b == data and not eof1 and eof2, "the stream comes out whole")
		assert(bin == #z and bout == #data, "what follows the stream is not taken")
		local c = zlib.crc32(); c("hel")
		assert(c("lo") == zlib.crc32("hello"))
		local ad = zlib.adler32(); ad("hel")
		assert(ad("lo") == zlib.adler32("hello"))
		assert(zlib.crc32(zlib.crc32("hel"), "lo") == zlib.crc32("hello"))
	`)
}

func TestZlibAgreesWithGo(t *testing.T) {
	data := bytes.Repeat([]byte("hello tlua "), 500)
	var zbuf, gbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	zw.Write(data)
	zw.Close()
	gw := gzip.NewWriter(&gbuf)
	gw.Write(data)
	gw.Close()
	L := runIn(t, map[string]string{"data": string(data), "z": zbuf.String(), "gz": gbuf.String()}, `
		zlib = require "zlib"
		assert(zlib.decompress(z) == data and zlib.decompress(gz) == data)
		assert(zlib.gunzip(gz) == data and zlib.inflate()(z) == data)
		assert(zlib.inflate(47)(gz) == data, "32 more detects the header")
		assert(zlib.decompress("not compressed") == nil)
		mine, mygz = zlib.compress(data), zlib.gzip(data)
		crc, adler = zlib.crc32(data), zlib.adler32(data)
	`)
	for name, open := range map[string]func(io.Reader) (io.Reader, error){
		"mine": func(r io.Reader) (io.Reader, error) { return zlib.NewReader(r) },
		"mygz": func(r io.Reader) (io.Reader, error) { return gzip.NewReader(r) },
	} {
		r, err := open(bytes.NewReader([]byte(lua.LVAsString(L.GetGlobal(name)))))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got, _ := io.ReadAll(r); !bytes.Equal(got, data) {
			t.Errorf("%s does not read back", name)
		}
	}
	if got := uint32(lua.LVAsNumber(L.GetGlobal("crc"))); got != crc32.ChecksumIEEE(data) {
		t.Errorf("crc32 %d", got)
	}
	if got := uint32(lua.LVAsNumber(L.GetGlobal("adler"))); got != adler32.Checksum(data) {
		t.Errorf("adler32 %d", got)
	}
}

func TestPng(t *testing.T) {
	dir := t.TempDir()
	runIn(t, map[string]string{"dir": dir}, `
		local png = require "png"
		local img = png.new(40, 20, "#ffffff")
		assert(img.width == 40 and img.height == 20 and tostring(img) == "image 40x20")
		img:set(0, 0, 255, 0, 0)
		img:set(39, 19, "#00ff0080")
		img:fill("#0000ff", 10, 5, 4, 3)
		assert(select(1, img:get(0, 0)) == 255 and img:get(40, 0) == nil)
		local _, g, _, a = img:get(39, 19)
		assert(g == 255 and a == 128)
		assert(select(3, img:get(11, 6)) == 255)
		assert(img:save(dir .. "/t.png"))
		assert(png.load(dir .. "/t.png"):pixels() == img:pixels())
		assert(png.decode(img:encode()):pixels() == img:pixels())
		local part = img:crop(10, 5, 4, 3)
		assert(part.width == 4 and part.height == 3 and select(3, part:get(0, 0)) == 255)
		local raw = png.fromPixels(2, 1, "\255\0\0\255\0\0\255\255")
		assert(select(3, raw:get(1, 0)) == 255)
		local c = img:clone(); c:set(0, 0, "#000000")
		assert(select(1, img:get(0, 0)) == 255, "a clone is a copy")
		assert(png.load(dir .. "/none.png") == nil and png.decode("garbage") == nil)
		assert(not pcall(img.set, img, 0, 0, "red"), "a colour is #rrggbb")
	`)
}

func TestZip(t *testing.T) {
	dir := t.TempDir()
	runIn(t, map[string]string{"dir": dir}, `
		local zip = require "zip"
		local out = zip.create(dir .. "/a.zip")
		out:add("x.txt", "alpha\nbeta\n")
		out:add("sub/y.bin", "raw", { store = true, modified = 1000000000 })
		assert(out:close() == true)
		local arc = assert(zip.open(dir .. "/a.zip"))
		assert(table.concat(arc:list(), ",") == "x.txt,sub/y.bin")
		assert(arc:read("sub/y.bin") == "raw" and arc:exists("x.txt") and not arc:exists("z"))
		local info = arc:info("sub/y.bin")
		assert(info.method == "store" and info.size == 3 and info.modified == 1000000000)
		assert(arc:read("missing") == nil)
		-- LuaZip's way
		local names = {}
		for f in arc:files() do names[#names + 1] = f.filename .. "=" .. f.uncompressed_size end
		assert(table.concat(names, ",") == "x.txt=11,sub/y.bin=3")
		local f = arc:open("x.txt")
		assert(f:read("*l") == "alpha" and f:read() == "beta" and f:read() == nil)
		f:close()
		-- in memory
		local mem = zip.create()
		mem:add("m.txt", "in memory")
		assert(zip.load(mem:close()):read("m.txt") == "in memory")
		assert(zip.load("not a zip") == nil)
	`)
}

// A fused program or a bundle reads its pictures and archives out of
// itself first.
func TestModulesReadThroughTheReader(t *testing.T) {
	dir := t.TempDir()
	L := lua.NewState()
	defer L.Close()
	Open(L)
	defer Forget(L)
	run := func(src string) {
		t.Helper()
		if err := L.DoString(src); err != nil {
			t.Fatal(err)
		}
	}
	run(`png = require "png"; zip = require "zip"
		pic = png.new(2, 2, "#ff0000"):encode()
		local mem = zip.create(); mem:add("in.txt", "inner"); arc = mem:close()`)
	files := map[string]string{
		"art/pic.png": lua.LVAsString(L.GetGlobal("pic")),
		"data.zip":    lua.LVAsString(L.GetGlobal("arc")),
	}
	SetReader(L, func(name string) ([]byte, error) {
		if data, ok := files[name]; ok {
			return []byte(data), nil
		}
		return os.ReadFile(name)
	})
	os.WriteFile(filepath.Join(dir, "disk.png"), []byte(files["art/pic.png"]), 0o644)
	L.SetGlobal("dir", lua.LString(dir))
	run(`assert(png.load("art/pic.png").width == 2)
		assert(zip.open("data.zip"):read("in.txt") == "inner")
		assert(png.load(dir .. "/disk.png").width == 2, "and the disk after")`)
}
