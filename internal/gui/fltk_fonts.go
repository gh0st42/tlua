//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import (
	"sort"
	"strings"
	"sync"

	"github.com/pwiecz/go-fltk"
)

// Fonts are "sans", "serif" and "mono", FLTK's own, or any font installed
// on the system, by the name it goes by ("Menlo", "Perfect DOS VGA 437
// Win"), case aside. FLTK is asked for the system's fonts the first time a
// name that is not one of the three is wanted.

var systemFonts struct {
	once     sync.Once
	families map[string]*[4]fltk.Font // by lower-case name: regular, bold, italic, bold italic
	names    []string                 // as the system names them, sorted
}

var builtinFonts = map[string]fltk.Font{"sans": fltk.HELVETICA, "serif": fltk.TIMES, "mono": fltk.COURIER}

// styleSuffixes are the endings that name a family's styles, longest first.
var styleSuffixes = []struct {
	suffix string
	style  int
}{
	{" bold italic", 3}, {" bold oblique", 3}, {" bold", 1}, {" italic", 2},
	{" oblique", 2}, {" regular", 0}, {" plain", 0}, {" roman", 0},
}

// familyOf is a font's family name and the style its name gives.
func familyOf(name string) (string, int) {
	l := strings.ToLower(strings.TrimSpace(name))
	if i := strings.IndexByte(l, ':'); i > 0 {
		l = l[:i] // "Academy Engraved LET Plain:1.0"
	}
	for _, s := range styleSuffixes {
		if strings.HasSuffix(l, s.suffix) && len(l) > len(s.suffix) {
			return strings.TrimSpace(l[:len(l)-len(s.suffix)]), s.style
		}
	}
	return l, 0
}

func loadSystemFonts() {
	systemFonts.once.Do(func() {
		fams := map[string]*[4]fltk.Font{}
		shown := map[string]string{}
		n := fltk.SetFonts()
		for i := 16; i < n; i++ { // the first 16 are FLTK's own
			name, attr := fltk.GetFontName(fltk.Font(i))
			fam, style := familyOf(name)
			if fam == "" {
				continue
			}
			if attr >= 1 && attr <= 3 {
				style = int(attr)
			}
			f := fams[fam]
			if f == nil {
				f = &[4]fltk.Font{-1, -1, -1, -1}
				fams[fam] = f
			}
			if f[style] < 0 {
				f[style] = fltk.Font(i)
			}
			if style == 0 {
				if _, ok := shown[fam]; !ok {
					shown[fam] = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(name, " Regular"), " Plain"))
				}
			}
		}
		for fam, f := range fams {
			if f[0] < 0 { // only styled faces: the first one stands for the family
				for _, g := range f {
					if g >= 0 {
						f[0] = g
						break
					}
				}
			}
			if _, ok := shown[fam]; !ok {
				name, _ := fltk.GetFontName(f[0])
				shown[fam] = name
			}
		}
		names := make([]string, 0, len(shown))
		for _, name := range shown {
			names = append(names, name)
		}
		sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
		systemFonts.families, systemFonts.names = fams, names
	})
}

// fontFace is the FLTK font of a name in a style; false for a name no
// font has. A font with no bold or italic face of its own is drawn plain.
func fontFace(name string, bold, italic bool) (fltk.Font, bool) {
	style := 0
	if bold {
		style++
	}
	if italic {
		style += 2
	}
	if f, ok := builtinFonts[name]; ok {
		// FLTK keeps each family's bold, italic and bold italic one after
		// another.
		return f + fltk.Font(style), true
	}
	loadSystemFonts()
	fam, _ := familyOf(name)
	f := systemFonts.families[fam]
	if f == nil {
		return 0, false
	}
	if f[style] >= 0 {
		return f[style], true
	}
	if style == 3 { // bold italic it has not: bold, or else italic
		for _, s := range []int{1, 2} {
			if f[s] >= 0 {
				return f[s], true
			}
		}
	}
	return f[0], true
}

// fontKnown says whether a font of that name is there.
func fontKnown(name string) bool {
	_, ok := fontFace(name, false, false)
	return ok
}

// installedFonts is every font's name: the three of FLTK's own, then the
// system's.
func installedFonts() []string {
	loadSystemFonts()
	return append([]string{"sans", "serif", "mono"}, systemFonts.names...)
}
