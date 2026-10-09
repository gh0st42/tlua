package fuse

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tlua/internal/payload"
)

const bundleUsage = `usage: tlua bundle [-o output] [-play] <main.lua | directory>

Packs a Lua program into a bundle: one zip file that any tlua runs, on any
platform, with "tlua app.ztl". It is what "tlua fuse" attaches to a binary,
shipped without the binary.

  main.lua      bundle a single Lua file
  directory     bundle the directory (it must contain main.lua), leaving
                out dot files

Options:
  -o output     where to write the bundle (default: named after the source,
                with .ztl). tlua runs a .ztl, .zip or .app file as a bundle.
  -play         the program is written for the console: the bundle opens a
                window and runs it the way "tlua play" does. A program that
                calls boot() itself needs no flag.
`

// BundleCommand implements `tlua bundle`, returning the process exit status.
func BundleCommand(args []string) int {
	fs := flag.NewFlagSet("bundle", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, bundleUsage) }
	out := fs.String("o", "", "output bundle")
	play := fs.Bool("play", false, "the program wants a window")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}
	src := fs.Arg(0)

	if *out == "" {
		*out = bundleName(src) + ".ztl"
	} else if !payload.IsBundleName(*out) {
		fmt.Fprintf(os.Stderr, "note: tlua runs a bundle by its name; call %s .ztl, .zip or .app\n", *out)
	}

	data, err := Bundle(src, *out, *play)
	if err != nil {
		return bundleFatal("%v", err)
	}
	window := *play
	if main := mainSource(payload.Zip, data); !*play && looksLikeAGame(main) && !asksForAWindow(main) {
		window = true
		fmt.Fprintf(os.Stderr,
			"note: %s defines _draw or _update, so the bundle opens a window; "+
				"bundle it with -play, or have it call boot(), to say so\n", src)
	}
	if err := writeFile(*out, data); err != nil {
		return bundleFatal("%v", err)
	}
	what := "runs as a script"
	if window {
		what = "opens a window"
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%s, %d bytes)\n", *out, what, len(data))
	return 0
}

// Bundle packs src, a directory or a single Lua file, into the zip that is a
// bundle, marked as a game when play is set. out is where it will be written,
// so that a bundle written into the folder it is made of is not packed into
// the next one.
func Bundle(src, out string, play bool) ([]byte, error) {
	comment := ""
	if play {
		comment = payload.PlayMarker
	}
	st, err := os.Stat(src)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		skip, _ := filepath.Abs(out)
		return zipDir(src, comment, skip)
	}
	if !strings.EqualFold(filepath.Ext(src), ".lua") {
		return nil, fmt.Errorf("%s is neither a folder nor a .lua file", src)
	}

	// A single file is the main.lua of a bundle of one.
	text, err := os.ReadFile(src)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	hdr := &zip.FileHeader{Name: payload.EntryName, Method: zip.Deflate, Modified: st.ModTime()}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(text); err != nil {
		return nil, err
	}
	if comment != "" {
		if err := zw.SetComment(comment); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// bundleName names a bundle after its program: the folder, for a folder or
// the main.lua in one, and the file otherwise.
func bundleName(src string) string {
	abs, err := filepath.Abs(src)
	if err != nil {
		abs = src
	}
	if filepath.Base(abs) == payload.EntryName {
		abs = filepath.Dir(abs)
	}
	name := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "app"
	}
	return name
}

// writeFile writes the bundle through a temporary file, so a failure never
// leaves half of one.
func writeFile(out string, data []byte) error {
	tmp := out + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, out); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func bundleFatal(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, "tlua bundle: "+format+"\n", args...)
	return 1
}
