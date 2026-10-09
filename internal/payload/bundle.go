package payload

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A bundle is a program shipped as a zip file of its own, for tlua to run:
// `tlua app.ztl`. It is the archive `tlua fuse` would attach to a binary,
// left unattached, so that one file runs on every platform tlua does.
//
// Bundles are told apart by name, not by content, so that `tlua data.zip`
// is never a surprise: .ztl (zipped tlua) is the name `tlua bundle` gives
// one, and .zip and .app are accepted too.
var BundleExts = []string{".ztl", ".zip", ".app"}

// PlayMarker is the zip comment of a bundle made with `tlua bundle -play`:
// the program is written for the console and wants a window around it. It is
// the bundle's version of the kind a fused binary keeps in its trailer.
const PlayMarker = "tlua:play"

// IsBundleName reports whether a file is named as a bundle.
func IsBundleName(name string) bool {
	ext := filepath.Ext(name)
	for _, e := range BundleExts {
		if strings.EqualFold(ext, e) {
			return true
		}
	}
	return false
}

// IsBundle reports whether name is a bundle to run: a file, not a folder (a
// macOS .app is a folder, and a real application), named as one.
func IsBundle(name string) bool {
	if !IsBundleName(name) {
		return false
	}
	st, err := os.Stat(name)
	return err == nil && st.Mode().IsRegular()
}

// OpenBundle opens a bundle as the program it holds. It is a Zip payload, or
// a ZipGame one when it was made with -play or its main.lua is plainly
// written for the console and does not say boot() for itself.
func OpenBundle(name string) (*Payload, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	zr, err := zip.NewReader(f, st.Size())
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%s is not a zip archive: %w", name, err)
	}
	archive, err := NewArchive(zr)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	p := &Payload{Kind: Zip, Archive: archive, file: f}
	if strings.TrimSpace(zr.Comment) == PlayMarker {
		p.Kind = ZipGame
	} else if src, err := archive.Read(EntryName); err == nil && LooksLikeAGame(src) && !AsksForAWindow(src) {
		// There is no command line to say -play on when the bundle is
		// opened, and run as a script, a console program only fails at its
		// first drawing call.
		p.Kind = ZipGame
	}
	return p, nil
}

// gameCallbacks is what a program written for the console defines, and
// nothing else much does: the two functions the frame loop calls.
var gameCallbacks = regexp.MustCompile(`(?:^|[^\w])(?:function\s+)?_(?:draw|update)\s*[=(]`)

// LooksLikeAGame reports whether a program appears to be written for the
// console.
func LooksLikeAGame(src []byte) bool {
	return src != nil && gameCallbacks.Match(src)
}

// bootCall is how a program says for itself that it wants the console.
var bootCall = regexp.MustCompile(`(?:^|[^\w.:])boot\s*[({"']`)

// AsksForAWindow reports whether a program calls boot().
func AsksForAWindow(src []byte) bool {
	return src != nil && bootCall.Match(src)
}
