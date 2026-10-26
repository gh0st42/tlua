package lualib

// WinZip's AES encryption for zip entries, the one 7-Zip, WinZip and Python's
// pyzipper read and write (AE-1 and AE-2). The standard library's zip cannot
// encrypt, but it can hand over and take an entry's bytes as they lie in the
// archive (File.OpenRaw, Writer.CreateRaw), so everything else about a zip
// stays the standard library's and this file only seals and opens entries.
//
// An encrypted entry has method 99 in the archive, and says in a 0x9901
// extra field which method the data really has (stored or deflated), and
// how strong the AES is. Its data is
//
//	salt | 2-byte password check | AES-CTR of the (compressed) file | 10-byte MAC
//
// The keys come from PBKDF2-HMAC-SHA1 over the password and the salt, 1000
// rounds: an encryption key, an authentication key and the 2-byte check.
// CTR counts from 1 with the counter little-endian, which is not what
// crypto/cipher's CTR does. The MAC is HMAC-SHA1 over the encrypted bytes.
// AE-2 leaves the entry's CRC-32 at 0, as the MAC covers the data; tlua
// writes AES-256 as AE-2 and reads 128, 192 and 256 as either.
//
// Names, sizes and dates are not encrypted: a zip has no way to.

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

const (
	zipMethodAES  = 99
	zipFlagCrypt  = 0x1
	aesExtraID    = 0x9901
	aesIterations = 1000
	aesVerifyLen  = 2
	aesMACLen     = 10
	aesStrength   = 3 // AES-256, which is what is written
)

var (
	errNeedPassword  = errors.New("a password is needed")
	errWrongPassword = errors.New("wrong password")
	errAltered       = errors.New("the data has been altered, or is damaged (or the password is wrong)")
)

// aesInfo is what an entry's 0x9901 extra field says.
type aesInfo struct {
	version  uint16 // 1 (AE-1) or 2 (AE-2)
	strength int    // 1, 2 or 3: AES-128, 192 or 256
	method   uint16 // how the data was packed before it was encrypted: 0 or 8
}

func (a aesInfo) saltLen() int { return 4 * (a.strength + 1) }
func (a aesInfo) keyLen() int  { return 8 * (a.strength + 1) }

// overhead is what encryption adds to the data.
func (a aesInfo) overhead() int { return a.saltLen() + aesVerifyLen + aesMACLen }

// aesOf reads an encrypted entry's extra field; false for one that is not
// encrypted this way.
func aesOf(f *zip.File) (aesInfo, bool) {
	if f.Method != zipMethodAES {
		return aesInfo{}, false
	}
	extra := f.Extra
	for len(extra) >= 4 {
		id, size := binary.LittleEndian.Uint16(extra), int(binary.LittleEndian.Uint16(extra[2:]))
		if 4+size > len(extra) {
			break
		}
		body := extra[4 : 4+size]
		extra = extra[4+size:]
		if id != aesExtraID || size != 7 || string(body[2:4]) != "AE" {
			continue
		}
		a := aesInfo{
			version:  binary.LittleEndian.Uint16(body),
			strength: int(body[4]),
			method:   binary.LittleEndian.Uint16(body[5:]),
		}
		if (a.version == 1 || a.version == 2) && a.strength >= 1 && a.strength <= 3 {
			return a, true
		}
	}
	return aesInfo{}, false
}

// aesExtra is the extra field for an AES-256 AE-2 entry packed with method.
func aesExtra(method uint16) []byte {
	b := make([]byte, 11)
	binary.LittleEndian.PutUint16(b, aesExtraID)
	binary.LittleEndian.PutUint16(b[2:], 7)
	binary.LittleEndian.PutUint16(b[4:], 2)
	b[6], b[7] = 'A', 'E'
	b[8] = aesStrength
	binary.LittleEndian.PutUint16(b[9:], method)
	return b
}

// aesHeader fills in a header for an AES entry of sealedLen bytes in the
// archive that holds plainLen bytes. Writer.CreateRaw, which takes it, does
// none of what Writer.CreateHeader does for an ordinary entry, so it is done
// here: the date, in MS-DOS form and as the extended timestamp Info-ZIP
// writes, the flag for a UTF-8 name, and the versions (WinZip's AES needs
// 5.1 to read). The caller sets hdr.Name and hdr.Modified.
func aesHeader(hdr *zip.FileHeader, method uint16, sealedLen, plainLen int) {
	hdr.Method = zipMethodAES
	hdr.Flags |= zipFlagCrypt
	if hasMultibyte(hdr.Name) {
		hdr.Flags |= 0x800 // the name is UTF-8
	}
	hdr.CreatorVersion = hdr.CreatorVersion&0xff00 | 20
	hdr.ReaderVersion = 51
	hdr.CompressedSize64, hdr.UncompressedSize64 = uint64(sealedLen), uint64(plainLen)
	hdr.CRC32 = 0 // AE-2: the MAC covers the data

	hdr.Extra = aesExtra(method)
	if !hdr.Modified.IsZero() {
		t := hdr.Modified
		hdr.ModifiedDate = uint16(t.Day() + int(t.Month())<<5 + (t.Year()-1980)<<9)
		hdr.ModifiedTime = uint16(t.Second()/2 + t.Minute()<<5 + t.Hour()<<11)
		var ts [9]byte
		binary.LittleEndian.PutUint16(ts[0:], 0x5455) // extended timestamp
		binary.LittleEndian.PutUint16(ts[2:], 5)
		ts[4] = 1 // the modification time is there
		binary.LittleEndian.PutUint32(ts[5:], uint32(t.Unix()))
		hdr.Extra = append(hdr.Extra, ts[:]...)
	}
}

// hasMultibyte says whether a name is valid UTF-8 that is not plain ASCII.
func hasMultibyte(name string) bool {
	if !utf8.ValidString(name) {
		return false
	}
	for i := 0; i < len(name); i++ {
		if name[i] >= 0x80 {
			return true
		}
	}
	return false
}

// ctrLE is AES in counter mode as WinZip has it: the counter is 128 bits,
// little-endian, and starts at 1.
type ctrLE struct {
	block   cipher.Block
	counter [16]byte
	stream  [16]byte
	used    int
}

func newCTRLE(b cipher.Block) *ctrLE {
	c := &ctrLE{block: b, used: 16}
	c.counter[0] = 1
	return c
}

func (c *ctrLE) XORKeyStream(dst, src []byte) {
	for i, v := range src {
		if c.used == 16 {
			c.block.Encrypt(c.stream[:], c.counter[:])
			for j := range c.counter {
				c.counter[j]++
				if c.counter[j] != 0 {
					break
				}
			}
			c.used = 0
		}
		dst[i] = v ^ c.stream[c.used]
		c.used++
	}
}

// aesKeys derives an entry's keys from the password and its salt.
func aesKeys(password string, salt []byte, keyLen int) (enc, auth, check []byte, err error) {
	k, err := pbkdf2.Key(sha1.New, password, salt, aesIterations, 2*keyLen+aesVerifyLen)
	if err != nil {
		return nil, nil, nil, err
	}
	return k[:keyLen], k[keyLen : 2*keyLen], k[2*keyLen:], nil
}

// aesSeal packs data (deflated, unless store) and encrypts it with AES-256:
// what goes into the archive for the entry, and the method it was packed with.
func aesSeal(data []byte, store bool, password string) (sealed []byte, method uint16, err error) {
	body, method := data, uint16(zip.Store)
	if !store {
		var packed bytes.Buffer
		fw, err := flate.NewWriter(&packed, 5)
		if err != nil {
			return nil, 0, err
		}
		if _, err = fw.Write(data); err == nil {
			err = fw.Close()
		}
		if err != nil {
			return nil, 0, err
		}
		body, method = packed.Bytes(), zip.Deflate
	}
	info := aesInfo{version: 2, strength: aesStrength}
	salt := make([]byte, info.saltLen())
	if _, err := rand.Read(salt); err != nil {
		return nil, 0, err
	}
	encKey, authKey, check, err := aesKeys(password, salt, info.keyLen())
	if err != nil {
		return nil, 0, err
	}
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, 0, err
	}
	sealed = make([]byte, 0, len(salt)+aesVerifyLen+len(body)+aesMACLen)
	sealed = append(sealed, salt...)
	sealed = append(sealed, check...)
	start := len(sealed)
	sealed = append(sealed, body...)
	newCTRLE(block).XORKeyStream(sealed[start:], body)
	mac := hmac.New(sha1.New, authKey)
	mac.Write(sealed[start:])
	sealed = append(sealed, mac.Sum(nil)[:aesMACLen]...)
	return sealed, method, nil
}

// aesOpen decrypts and unpacks an AES entry's data.
func aesOpen(f *zip.File, info aesInfo, password string) ([]byte, error) {
	if password == "" {
		return nil, errNeedPassword
	}
	rd, err := f.OpenRaw()
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(rd)
	if err != nil {
		return nil, err
	}
	if len(raw) < info.overhead() {
		return nil, errAltered
	}
	salt := raw[:info.saltLen()]
	encKey, authKey, check, err := aesKeys(password, salt, info.keyLen())
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare(check, raw[len(salt):len(salt)+aesVerifyLen]) != 1 {
		return nil, errWrongPassword
	}
	body := raw[len(salt)+aesVerifyLen : len(raw)-aesMACLen]
	mac := hmac.New(sha1.New, authKey)
	mac.Write(body)
	if !hmac.Equal(mac.Sum(nil)[:aesMACLen], raw[len(raw)-aesMACLen:]) {
		return nil, errAltered
	}
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(body))
	newCTRLE(block).XORKeyStream(plain, body)
	switch info.method {
	case zip.Store:
		return plain, nil
	case zip.Deflate:
		return io.ReadAll(flate.NewReader(bytes.NewReader(plain)))
	}
	return nil, fmt.Errorf("compression method %d is not supported", info.method)
}

// zipEntry is an entry's contents: unpacked, and decrypted when it is
// encrypted, with the password given.
func zipEntry(f *zip.File, password string) ([]byte, error) {
	if info, ok := aesOf(f); ok {
		data, err := aesOpen(f, info, password)
		if err != nil {
			return nil, fmt.Errorf("'%s': %w", f.Name, err)
		}
		return data, nil
	}
	if f.Flags&zipFlagCrypt != 0 {
		return nil, fmt.Errorf("'%s' is encrypted the old way (ZipCrypto), which tlua does not read; AES is what it writes", f.Name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
