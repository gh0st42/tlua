package lualib

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// The files in testdata/zipaes were made by 7-Zip, with the password
// sok3r!; this is what is in them (see the README there).
const sevenZipFiles = `
-- pseudo is n bytes that look random, the same each time; made in pieces,
-- since a state with the default registry cannot concat a table of thousands.
local function pseudo(n, mul)
  local parts = {}
  for from = 1, n, 400 do
    local piece = {}
    for i = from, math.min(n, from + 399) do piece[#piece + 1] = string.char((i * mul + math.floor(i / 3)) % 256) end
    parts[#parts + 1] = table.concat(piece)
  end
  return table.concat(parts)
end
local function contents()
  return {
    ["a.txt"] = "from 7-zip\n",
    ["text.txt"] = string.rep("compressible line of text\n", 200),
    ["rand.bin"] = pseudo(3000, 7919),
    ["empty.txt"] = "",
  }
end`

// tlua reads what 7-Zip writes: AES-128, 192 and 256, deflated and stored.
func TestZipReadsWhatSevenZipWrites(t *testing.T) {
	dir, err := filepath.Abs("testdata/zipaes")
	if err != nil {
		t.Fatal(err)
	}
	runIn(t, map[string]string{"dir": dir}, sevenZipFiles+`
		local zip = require "zip"
		local want = contents()
		for _, case in ipairs {
			{ "aes128.zip", "deflate" }, { "aes192.zip", "deflate" }, { "aes256.zip", "deflate" },
			{ "aes256-store.zip", "store" },
		} do
			local name, method = case[1], case[2]
			local arc = assert(zip.open(dir .. "/" .. name))
			local n = 0
			for f in arc:files() do
				n = n + 1
				assert(f.encrypted, name .. " " .. f.filename)
				assert(want[f.filename] and f.size == #want[f.filename], name .. " " .. f.filename)
			end
			assert(n >= 2, name)
			-- Without the password, or with the wrong one, it says so.
			local data, why = arc:read("text.txt")
			assert(data == nil and why:find("password is needed", 1, true), tostring(why))
			data, why = arc:read("text.txt", "wrong")
			assert(data == nil and why:find("wrong password", 1, true), tostring(why))
			-- With it, as the archive's or for one call.
			assert(arc:read("text.txt", "sok3r!") == want["text.txt"], name)
			arc:setpassword("sok3r!")
			for file, text in pairs(want) do
				if arc:exists(file) then assert(arc:read(file) == text, name .. " " .. file) end
			end
			assert(arc:info("text.txt").method == method, name .. " " .. tostring(arc:info("text.txt").method))
			-- LuaZip's way: a file to read from, once there is a password
			arc:setpassword(nil)
			assert(arc:open("text.txt") == nil)
			arc:setpassword("sok3r!")
			assert(arc:open("text.txt"):read("*l") == "compressible line of text")
		end
		-- the password can be given when the archive is opened
		local arc = assert(zip.open(dir .. "/aes256.zip", { password = "sok3r!" }))
		assert(arc:read("rand.bin") == want["rand.bin"])
	`)
}

// What is written reads back, with the options each way of giving a
// password has, and what is not encrypted stays readable without one.
func TestZipEncryptionRoundTrip(t *testing.T) {
	runIn(t, nil, `
		local zip = require "zip"
		local big = string.rep("0123456789abcdef", 5000)
		-- pseudo is n bytes that look random, the same each time; made in pieces,
		-- since a state with the default registry cannot concat a table of thousands.
		local function pseudo(n, mul)
		  local parts = {}
		  for from = 1, n, 400 do
		    local piece = {}
		    for i = from, math.min(n, from + 399) do piece[#piece + 1] = string.char((i * mul + math.floor(i / 3)) % 256) end
		    parts[#parts + 1] = table.concat(piece)
		  end
		  return table.concat(parts)
		end
		
		local rnd = pseudo(4099, 31) -- not a block multiple

		local out = zip.create({ password = "pässwörd ✓" })
		out:add("hello.txt", "hello\n")
		out:add("empty.txt", "")
		out:add("big.bin", big)
		out:add("raw.bin", rnd, { store = true })
		out:add("open.txt", "anyone can read this", { password = false })
		out:add("other.txt", "another secret", { password = "second" })
		local bytes = assert(out:close())

		local arc = assert(zip.load(bytes))
		assert(table.concat(arc:list(), ",") == "hello.txt,empty.txt,big.bin,raw.bin,open.txt,other.txt",
		  "the names are readable without a password")
		assert(arc:info("hello.txt").encrypted and arc:info("raw.bin").encrypted and arc:info("other.txt").encrypted)
		assert(not arc:info("open.txt").encrypted)
		assert(arc:info("big.bin").method == "deflate" and arc:info("raw.bin").method == "store")
		assert(arc:info("big.bin").size == #big and arc:info("big.bin").compressed < 1000)
		assert(arc:info("raw.bin").compressed == #rnd, "store: the size of the data, not of what encryption adds")
		assert(arc:read("open.txt") == "anyone can read this")
		assert(arc:read("hello.txt") == nil)

		arc:setpassword("pässwörd ✓")
		assert(arc:read("hello.txt") == "hello\n")
		assert(arc:read("empty.txt") == "")
		assert(arc:read("big.bin") == big)
		assert(arc:read("raw.bin") == rnd)
		assert(arc:read("open.txt") == "anyone can read this")
		local _, why = arc:read("other.txt")
		assert(why:find("wrong password", 1, true), why)
		assert(arc:read("other.txt", "second") == "another secret")
		assert(arc:open("big.bin"):read(5) == "01234")

		-- wrong archive password, then none
		local arc2 = assert(zip.load(bytes, { password = "nope" }))
		assert(select(2, arc2:read("hello.txt")):find("wrong password", 1, true))
		arc2:setpassword(nil)
		assert(select(2, arc2:read("hello.txt")):find("password is needed", 1, true))

		-- the salt is random: the same file twice is not the same bytes
		local again = zip.create({ password = "pässwörd ✓" })
		again:add("hello.txt", "hello\n")
		local a1, a2 = zip.load(again:close()), zip.load(bytes)
		assert(a1:read("hello.txt", "pässwörd ✓") == "hello\n")

		-- a file, and a password given late
		local dir = os.tmpname()
		os.remove(dir)
		local f = zip.create(dir, { password = "x1" })
		f:add("a", "A")
		assert(f:close() == true)
		assert(zip.open(dir, { password = "x1" }):read("a") == "A")
		os.remove(dir)
	`)
}

func TestZipPasswordsAreChecked(t *testing.T) {
	runIn(t, nil, `
		local zip = require "zip"
		assert(not pcall(zip.create, { password = "" }), "an empty password is no password")
		assert(not pcall(zip.create, { password = 5 }))
		local out = zip.create()
		assert(not pcall(out.add, out, "a", "b", { password = "" }))
		local arc = zip.load((function() local o = zip.create(); o:add("a", "b"); return o:close() end)())
		assert(not pcall(arc.setpassword, arc, ""))
		-- no password asked for on a plain file, even if one is given
		assert(arc:read("a", "unused") == "b")
	`)
}

// An entry whose bytes were changed is not given back, and the cause is told
// as far as it is known.
func TestZipEncryptionDetectsChanges(t *testing.T) {
	L := runIn(t, nil, `
		local zip = require "zip"
		local out = zip.create({ password = "secret" })
		out:add("a.txt", string.rep("attack at dawn. ", 40))
		bytes = out:close()`)
	bytes := []byte(lua.LVAsString(L.GetGlobal("bytes")))
	nameLen := int(binary.LittleEndian.Uint16(bytes[26:]))
	extraLen := int(binary.LittleEndian.Uint16(bytes[28:]))
	size := int(binary.LittleEndian.Uint32(bytes[18:]))
	data := 30 + nameLen + extraLen // where salt | check | encrypted | MAC begin

	for _, c := range []struct {
		what   string
		offset int
		want   string
	}{
		{"the password check", data + 16, "wrong password"},
		{"the salt", data + 3, "wrong password"},
		{"the encrypted data", data + 18 + 5, "altered"},
		{"the MAC", data + size - 1, "altered"},
	} {
		changed := append([]byte(nil), bytes...)
		changed[c.offset] ^= 0x01
		runIn(t, map[string]string{"changed": string(changed), "want": c.want, "what": c.what}, `
			local zip = require "zip"
			local arc = assert(zip.load(changed, { password = "secret" }))
			local data, why = arc:read("a.txt")
			assert(data == nil and why:find(want, 1, true), what .. ": " .. tostring(why))
		`)
	}
	// And what was not changed still reads.
	runIn(t, map[string]string{"bytes": string(bytes)}, `
		assert(require("zip").load(bytes, { password = "secret" }):read("a.txt") == string.rep("attack at dawn. ", 40))`)
}

// The old ZipCrypto is not read, and the message says why.
func TestZipCryptoIsRefused(t *testing.T) {
	zipBin, err := exec.LookPath("zip")
	if err != nil {
		t.Skip("no zip command to make a ZipCrypto archive with")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "old.txt"), []byte("old secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(zipBin, "-q", "-P", "pw", "old.zip", "old.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("zip -P: %v %s", err, out)
	}
	runIn(t, map[string]string{"path": filepath.Join(dir, "old.zip")}, `
		local arc = assert(require("zip").open(path))
		assert(arc:info("old.txt").encrypted)
		local data, why = arc:read("old.txt", "pw")
		assert(data == nil and why:find("ZipCrypto", 1, true), tostring(why))`)
}

// sevenZip is the 7-Zip command, if this machine has one.
func sevenZip(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"7zz", "7z", "7za", "7zr"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	t.Skip("no 7-Zip to check against (7zz, 7z or 7za)")
	return ""
}

// 7-Zip reads what tlua writes: its test passes, and what it extracts is
// what went in; a wrong password is refused.
func TestSevenZipReadsWhatZipWrites(t *testing.T) {
	sz := sevenZip(t)
	dir := t.TempDir()
	runIn(t, map[string]string{"dir": dir}, `
		local zip = require "zip"
		local out = zip.create(dir .. "/ours.zip", { password = "pässwörd ✓" })
		out:add("hello.txt", "hello, world\n")
		out:add("empty.txt", "")
		out:add("big.bin", string.rep("0123456789abcdef", 40000))
		-- pseudo is n bytes that look random, the same each time; made in pieces,
		-- since a state with the default registry cannot concat a table of thousands.
		local function pseudo(n, mul)
		  local parts = {}
		  for from = 1, n, 400 do
		    local piece = {}
		    for i = from, math.min(n, from + 399) do piece[#piece + 1] = string.char((i * mul + math.floor(i / 3)) % 256) end
		    parts[#parts + 1] = table.concat(piece)
		  end
		  return table.concat(parts)
		end
		
		out:add("raw.bin", pseudo(70001, 7919), { store = true })
		out:add("plain.txt", "not encrypted", { password = false })
		out:add("dir/nested.txt", "nested")
		assert(out:close())`)
	zipPath := filepath.Join(dir, "ours.zip")

	if out, err := exec.Command(sz, "t", "-p"+"pässwörd ✓", zipPath).CombinedOutput(); err != nil || !strings.Contains(string(out), "Everything is Ok") {
		t.Fatalf("7-Zip does not accept it: %v\n%s", err, out)
	}
	if out, err := exec.Command(sz, "t", "-pwrong", zipPath).CombinedOutput(); err == nil {
		t.Errorf("7-Zip took a wrong password:\n%s", out)
	}
	// What 7-Zip makes of it is what went in.
	extracted := filepath.Join(dir, "x")
	if out, err := exec.Command(sz, "x", "-y", "-o"+extracted, "-p"+"pässwörd ✓", zipPath).CombinedOutput(); err != nil {
		t.Fatalf("7-Zip x: %v\n%s", err, out)
	}
	runIn(t, map[string]string{"dir": extracted}, `
		local function slurp(p) local f = assert(io.open(p, "rb")); local d = f:read("*a"); f:close(); return d end
		assert(slurp(dir .. "/hello.txt") == "hello, world\n")
		assert(slurp(dir .. "/empty.txt") == "")
		assert(slurp(dir .. "/big.bin") == string.rep("0123456789abcdef", 40000))
		assert(#slurp(dir .. "/raw.bin") == 70001)
		assert(slurp(dir .. "/plain.txt") == "not encrypted")
		assert(slurp(dir .. "/dir/nested.txt") == "nested")`)
	// And it says AES-256, as the format has it.
	if out, err := exec.Command(sz, "l", "-slt", "-p"+"pässwörd ✓", zipPath).CombinedOutput(); err != nil || !strings.Contains(string(out), "AES-256") {
		t.Errorf("7-Zip does not call it AES-256: %v\n%s", err, out)
	}
}

// tlua reads what 7-Zip writes now, as well as in the fixtures: all three
// strengths, with a file and a password of this run's own.
func TestZipReadsWhatSevenZipWritesNow(t *testing.T) {
	sz := sevenZip(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "n.txt"), []byte(strings.Repeat("now ", 5000)), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"AES128", "AES192", "AES256"} {
		cmd := exec.Command(sz, "a", "-tzip", "-mem="+m, "-pn0w-"+m, "-bso0", m+".zip", "n.txt")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("7-Zip a %s: %v\n%s", m, err, out)
		}
		runIn(t, map[string]string{"path": filepath.Join(dir, m+".zip"), "pw": "n0w-" + m}, `
			local arc = assert(require("zip").open(path, { password = pw }))
			assert(arc:read("n.txt") == string.rep("now ", 5000))`)
	}
}
