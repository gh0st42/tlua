package payload

import (
	"archive/zip"
	"fmt"
	"io"
	"path"
	"strings"
)

// Archive is the read-only file system inside a fused zip.
type Archive struct {
	files map[string]*zip.File
	names []string
	root  string // top-level folder to strip, when the zip has exactly one
}

// NewArchive indexes a zip and checks that it has an entry point.
func NewArchive(zr *zip.Reader) (*Archive, error) {
	a := &Archive{files: make(map[string]*zip.File, len(zr.File))}
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		name := path.Clean(f.Name)
		a.files[name] = f
		a.names = append(a.names, name)
	}

	if _, ok := a.files[EntryName]; ok {
		return a, nil
	}
	// `zip -r app.zip mygame` puts everything under one folder; accept that
	// too rather than making people re-zip from inside the directory.
	roots := map[string]bool{}
	for name := range a.files {
		if i := strings.Index(name, "/"); i > 0 {
			roots[name[:i]] = true
		}
	}
	if len(roots) == 1 {
		for r := range roots {
			if _, ok := a.files[r+"/"+EntryName]; ok {
				a.root = r + "/"
				return a, nil
			}
		}
	}
	return nil, fmt.Errorf("fused archive has no %s at its root", EntryName)
}

// Has reports whether the archive holds name.
func (a *Archive) Has(name string) bool {
	_, ok := a.open(name)
	return ok
}

// Read returns the contents of one file.
func (a *Archive) Read(name string) ([]byte, error) {
	f, ok := a.open(name)
	if !ok {
		return nil, fmt.Errorf("no file '%s' in archive", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// List returns the archive's file names, with any single root folder stripped.
func (a *Archive) List() []string {
	out := make([]string, 0, len(a.names))
	for _, n := range a.names {
		if a.root != "" {
			if !strings.HasPrefix(n, a.root) {
				continue
			}
			n = strings.TrimPrefix(n, a.root)
		}
		out = append(out, n)
	}
	return out
}

func (a *Archive) open(name string) (*zip.File, bool) {
	f, ok := a.files[a.root+path.Clean(name)]
	return f, ok
}
