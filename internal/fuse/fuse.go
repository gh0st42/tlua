// Package fuse builds standalone executables: it attaches a Lua program to a
// copy of the interpreter.
package fuse

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"tlua/internal/payload"
)

const usage = `usage: tlua fuse [-o output] [-play] [--base interpreter] <main.lua | directory | archive.zip | bundle.ztl>

Attaches a Lua program to a copy of the interpreter, producing a standalone
executable that runs the program instead of reading options.

  main.lua      embed a single Lua file
  directory     zip the directory (it must contain main.lua) and embed it
  archive.zip   embed an existing archive (it must contain main.lua); a
                bundle (.ztl, .app) made by "tlua bundle" works the same

Options:
  -o output     where to write the executable (default: named after the source)
  -play         the program is written for the console: the executable opens a
                window and runs it the way "tlua play" does. A program that
                calls boot() itself needs no flag.
  --base path   interpreter to build on (default: this binary; use a
                cross-compiled tlua to build for another platform)
`

// Command implements `tlua fuse`, returning the process exit status.
func Command(args []string) int {
	fs := flag.NewFlagSet("fuse", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	out := fs.String("o", "", "output executable")
	play := fs.Bool("play", false, "the program wants a window")
	base := fs.String("base", "", "interpreter to build on")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}
	src := fs.Arg(0)

	if *base == "" {
		exe, err := os.Executable()
		if err != nil {
			return fatal("cannot locate the tlua binary: %v", err)
		}
		*base = exe
	}
	prefix, err := payload.InterpreterPrefix(*base)
	if err != nil {
		return fatal("%v", err)
	}

	kind, data, err := buildPayload(src)
	if err != nil {
		return fatal("%v", err)
	}
	if *play {
		kind = kind.AsGame()
	} else if main := mainSource(kind, data); looksLikeAGame(main) && !asksForAWindow(main) {
		// Fused without -play, a console program fails at the first drawing
		// call with "attempt to call a non-function object", which says
		// nothing about what is actually wrong. Better to say it here.
		//
		// A program that calls boot() has said it for itself and needs no
		// telling: that is what the call is for.
		fmt.Fprintf(os.Stderr,
			"note: %s defines _draw or _update; fuse it with -play, or have it call boot(), "+
				"if it should open a window\n", src)
	}

	if *out == "" {
		*out = defaultOutputName(src, *base)
	}
	if err := writeFused(*out, prefix, kind, data); err != nil {
		return fatal("%v", err)
	}

	fmt.Fprintf(os.Stderr, "wrote %s (%s, %d bytes of program)\n", *out, kindName(kind), len(data))
	return 0
}

func kindName(k payload.Kind) string {
	what := "single file"
	if k.Shape() == payload.Zip {
		what = "archive"
	}
	if k.Game() {
		what += ", opens a window"
	}
	return what
}

// buildPayload turns a source path into the bytes to attach.
func buildPayload(src string) (payload.Kind, []byte, error) {
	st, err := os.Stat(src)
	if err != nil {
		return 0, nil, err
	}

	switch {
	case st.IsDir():
		data, err := zipDir(src, "", "")
		return payload.Zip, data, err

	case payload.IsBundleName(src):
		data, err := os.ReadFile(src)
		if err != nil {
			return 0, nil, err
		}
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return 0, nil, fmt.Errorf("%s is not a zip archive: %w", src, err)
		}
		if _, err := payload.NewArchive(zr); err != nil {
			return 0, nil, err
		}
		// A bundle made with -play is fused as a game without saying so again.
		if strings.TrimSpace(zr.Comment) == payload.PlayMarker {
			return payload.ZipGame, data, nil
		}
		return payload.Zip, data, nil

	default:
		data, err := os.ReadFile(src)
		return payload.Lua, data, err
	}
}

// zipDir packs a directory, which must hold main.lua at its top level.
// comment is the zip's comment, which is how a bundle says -play; skip is a
// file left out, the bundle being written when it is written inside dir.
func zipDir(dir, comment, skip string) ([]byte, error) {
	if _, err := os.Stat(filepath.Join(dir, payload.EntryName)); err != nil {
		return nil, fmt.Errorf("%s has no %s", dir, payload.EntryName)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// Skip dot files: .git, .DS_Store and friends have no business in an app.
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if skip != "" {
			if abs, err := filepath.Abs(p); err == nil && abs == skip {
				return nil
			}
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		hdr.Method = zip.Deflate
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
	if err != nil {
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

func writeFused(out string, prefix []byte, kind payload.Kind, program []byte) error {
	trailer := payload.Trailer(kind, len(program))

	tmp := out + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	for _, chunk := range [][]byte{prefix, program, trailer} {
		if _, err := f.Write(chunk); err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	// Rename last so a failed fuse never leaves a half-written executable.
	if err := os.Rename(tmp, out); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Chmod(out, 0o755)
}

// defaultOutputName names the executable after the program, keeping the base
// interpreter's extension so Windows builds stay .exe.
func defaultOutputName(src, base string) string {
	name := filepath.Base(strings.TrimSuffix(filepath.Clean(src), string(filepath.Separator)))
	name = strings.TrimSuffix(name, filepath.Ext(name))
	if name == "" || name == "." {
		name = "app"
	}
	if ext := filepath.Ext(base); strings.EqualFold(ext, ".exe") {
		name += ext
	}
	return name
}

func fatal(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, "tlua fuse: "+format+"\n", args...)
	return 1
}

// looksLikeAGame and asksForAWindow are only ever used to offer a hint, so a
// wrong guess costs a line of output and nothing else.
func looksLikeAGame(src []byte) bool { return payload.LooksLikeAGame(src) }

func asksForAWindow(src []byte) bool { return payload.AsksForAWindow(src) }

// mainSource reports the program's main chunk, for looking at before it is
// attached. It gives up quietly: this is only used for the hint above.
func mainSource(kind payload.Kind, data []byte) []byte {
	if kind.Shape() != payload.Zip {
		return data
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}
	archive, err := payload.NewArchive(zr)
	if err != nil {
		return nil
	}
	src, err := archive.Read(payload.EntryName)
	if err != nil {
		return nil
	}
	return src
}
