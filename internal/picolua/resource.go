package picolua

import (
	"fmt"
	"path/filepath"
	"strings"
)

// A kind of resource: what it is called, where games keep it, and what it is
// usually named.
//
// The point of this is that a program should be able to say what it wants
// rather than where it is: sfx("jump") finds sfx/jump.wav or
// assets/sfx/jump.wav without being told which, and sfx("assets/sfx/jump.wav")
// still means exactly that. The layout a game grows into is its own business,
// and both of the usual ones are here.
type kind struct {
	what string   // for the error when nothing is found
	dirs []string // where to look, in order; "" is the program's own folder
	exts []string // what to try adding when a name has no extension
}

var (
	kindImage = kind{
		what: "image",
		dirs: []string{"", "gfx", "assets/gfx", "assets"},
		exts: []string{".png"},
	}
	kindSound = kind{
		what: "sound",
		dirs: []string{"", "sfx", "assets/sfx", "assets"},
		exts: []string{".wav", ".ogg"},
	}
	kindMusic = kind{
		what: "music",
		dirs: []string{"", "music", "assets/music", "bgm", "assets/bgm", "sfx", "assets/sfx", "assets"},
		exts: []string{".ogg", ".wav"},
	}
	kindMap = kind{
		what: "map",
		dirs: []string{"", "maps", "assets/maps", "assets"},
		exts: []string{".tmj", ".json"},
	}
	kindTileset = kind{
		what: "tileset",
		dirs: []string{"", "gfx", "assets/gfx", "maps", "assets/maps", "assets"},
		exts: []string{".tsj", ".json"},
	}
	kindData = kind{
		what: "file",
		dirs: []string{"", "data", "assets/data", "maps", "assets/maps", "assets"},
		exts: []string{".txt"},
	}
)

// candidates reports the paths to try for a name, in order.
//
// A name with an extension on it is taken as a filename and only looked for in
// each of the places; a name without one is a name, and every extension the
// kind knows is tried in each place. An absolute path is meant literally.
func candidates(k kind, name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if filepath.IsAbs(name) {
		return []string{name}
	}

	names := []string{name}
	if filepath.Ext(name) == "" {
		names = names[:0]
		for _, ext := range k.exts {
			names = append(names, name+ext)
		}
	}

	out := make([]string, 0, len(k.dirs)*len(names))
	for _, dir := range k.dirs {
		for _, n := range names {
			if dir == "" {
				out = append(out, n)
				continue
			}
			out = append(out, filepath.ToSlash(filepath.Join(dir, n)))
		}
	}
	return out
}

// find reads a resource, trying each of the places it could be.
//
// The answer is remembered, because a program calls sfx("jump") every time
// something jumps and looking through six places for it each time would be
// work done over and over for the same answer.
func (r *Runtime) find(k kind, name string) (path string, data []byte, err error) {
	if found, ok := r.found[name]; ok {
		data, err := r.read(found)
		return found, data, err
	}

	tried := candidates(k, name)
	for _, candidate := range tried {
		data, err := r.read(candidate)
		if err != nil {
			continue
		}
		if r.found == nil {
			r.found = map[string]string{}
		}
		r.found[name] = candidate
		return candidate, data, nil
	}

	if len(tried) == 0 {
		return "", nil, fmt.Errorf("no %s named", k.what)
	}
	return "", nil, fmt.Errorf("no %s called %q: tried %s",
		k.what, name, strings.Join(tried, ", "))
}
