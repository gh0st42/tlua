package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tlua/internal/lsp"
)

// buffer is one open file, or an unsaved new file when path is empty.
type buffer struct {
	path  string // absolute; empty for a new file that has never been saved
	name  string // what the tab bar shows
	area  *codeArea
	dirty bool
	// crlf records that the file was read with DOS line endings, so that saving
	// it does not quietly rewrite every line.
	crlf bool
	// diagnostics is what the language server last said about this buffer.
	diagnostics []lsp.Diagnostic
}

// newBuffer builds an editing widget wired to this editor's bookkeeping.
func (e *Editor) newBuffer(path, name, text string) *buffer {
	b := &buffer{path: path, name: name}

	area := newCodeArea()
	area.SetText(text, false)
	// One clipboard for the whole editor, so cut and paste crosses buffers.
	area.SetClipboard(
		func(s string) { e.clipboard = s },
		func() string { return e.clipboard },
	)
	area.SetChangedFunc(func() {
		area.invalidate()
		e.lspGeneration++
		// The server is told once the typing pauses, and reports back.
		e.scheduleSync()
		if !b.dirty {
			b.dirty = true
			e.refreshTabs()
		}
		e.refreshStatus()
	})
	area.SetMovedFunc(func() {
		e.lspGeneration++
		e.refreshStatus()
		// The text and the cursor are already up to date here, which is what
		// the parameter hint needs to follow them.
		e.refreshSignature()
	})

	b.area = area
	return b
}

// lspPath is the path a language server is told about. A buffer that has never
// been saved has none, so it borrows its name in the working directory: the
// server owns the text either way, and the file need not exist.
func (b *buffer) lspPath() string {
	if b.path != "" {
		return b.path
	}
	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, b.name)
	}
	return b.name
}

// title is the buffer's name with the markers the bars show: '*' when
// modified, '»' when it is the file F5 runs.
func (e *Editor) title(b *buffer) string {
	name := b.name
	if b.dirty {
		name += "*"
	}
	if errors, warnings := problemCount(b.diagnostics); errors > 0 {
		name += "!"
	} else if warnings > 0 {
		name += "?"
	}
	if e.primary == b {
		name = "»" + name
	}
	return name
}

// openFile loads path into a buffer, switching to it if it is already open.
func (e *Editor) openFile(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for i, b := range e.buffers {
		if b.path == abs {
			e.selectBuffer(i)
			return nil
		}
	}

	st, err := os.Stat(abs)
	switch {
	case err == nil && st.IsDir():
		return fmt.Errorf("%s is a directory", path)
	case err != nil && !os.IsNotExist(err):
		return err
	}

	var text string
	crlf := false
	if err == nil {
		data, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		text = string(data)
		crlf = strings.Contains(text, "\r\n")
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}

	b := e.newBuffer(abs, displayName(abs), text)
	b.crlf = crlf
	if e.lsp != nil {
		_ = e.lsp.Sync(abs, text)
	}
	e.addBuffer(b)
	if err != nil {
		e.setStatus(fmt.Sprintf("New file %s", b.name))
	}
	return nil
}

// newFile adds an empty, never-saved buffer.
func (e *Editor) newFile() {
	e.untitled++
	e.addBuffer(e.newBuffer("", fmt.Sprintf("untitled%d.lua", e.untitled), ""))
}

func (e *Editor) addBuffer(b *buffer) {
	e.buffers = append(e.buffers, b)
	e.editors.AddPage(pageName(len(e.buffers)-1), b.area, true, false)
	if e.primary == nil && b.path != "" {
		e.primary = b
	}
	e.selectBuffer(len(e.buffers) - 1)
}

// save writes a buffer back to disk. A buffer with no path needs a name first,
// which is the caller's job.
//
// When a language server is formatting, the buffer is formatted first, so what
// lands on disk is what the screen shows.
func (e *Editor) save(b *buffer) error {
	if b.path == "" {
		return fmt.Errorf("%s has never been saved", b.name)
	}
	if e.formatOnSave && e.canFormat() {
		if err := e.formatBuffer(b); err != nil {
			// A server that cannot be reached must not stop a save.
			e.lspNote = "Not formatted: " + err.Error()
		}
	}
	text := b.area.GetText()
	if !strings.HasSuffix(text, "\n") && text != "" {
		text += "\n" // a Lua file, like any text file, ends in a newline
	}
	onDisk := text
	if b.crlf {
		onDisk = strings.ReplaceAll(text, "\n", "\r\n")
	}
	if err := os.WriteFile(b.path, []byte(onDisk), 0o644); err != nil {
		return err
	}
	if e.lsp != nil {
		_ = e.lsp.DidSave(b.path, text)
	}
	b.dirty = false
	e.refreshTabs()
	e.refreshStatus()
	return nil
}

// saveAs points a buffer at a new path and writes it there.
func (e *Editor) saveAs(b *buffer, path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	oldPath, oldName := b.path, b.name
	b.path, b.name = abs, displayName(abs)
	if err := e.save(b); err != nil {
		b.path, b.name = oldPath, oldName
		return err
	}
	if e.primary == nil {
		e.primary = b
	}
	e.refreshTabs()
	return nil
}

// closeBuffer drops a buffer; the editor always keeps at least one.
func (e *Editor) closeBuffer(b *buffer) {
	idx := -1
	for i, other := range e.buffers {
		if other == b {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}

	// Pages are named by position, so rebuild them all after the removal.
	for i := range e.buffers {
		e.editors.RemovePage(pageName(i))
	}
	e.buffers = append(e.buffers[:idx], e.buffers[idx+1:]...)
	if e.primary == b {
		e.primary = nil
	}
	for i, other := range e.buffers {
		e.editors.AddPage(pageName(i), other.area, true, false)
		if e.primary == nil && other.path != "" {
			e.primary = other
		}
	}

	if e.lsp != nil {
		// Including buffers that were never saved: the server was told about
		// them under the same name.
		_ = e.lsp.DidClose(b.lspPath())
	}
	if len(e.buffers) == 0 {
		e.newFile()
		return
	}
	if idx >= len(e.buffers) {
		idx = len(e.buffers) - 1
	}
	e.selectBuffer(idx)
}

func (e *Editor) selectBuffer(i int) {
	if i < 0 || i >= len(e.buffers) {
		return
	}
	e.hideSignature() // it belonged to the buffer being left
	e.completing = false
	e.current = i
	e.editors.SwitchToPage(pageName(i))
	e.app.SetFocus(e.buffers[i].area)
	e.refreshTabs()
	e.refreshStatus()
}

func (e *Editor) nextBuffer() {
	if len(e.buffers) > 1 {
		e.selectBuffer((e.current + 1) % len(e.buffers))
	}
}

func (e *Editor) prevBuffer() {
	if len(e.buffers) > 1 {
		e.selectBuffer((e.current - 1 + len(e.buffers)) % len(e.buffers))
	}
}

// buf is the buffer being edited.
func (e *Editor) buf() *buffer {
	if e.current < 0 || e.current >= len(e.buffers) {
		return nil
	}
	return e.buffers[e.current]
}

// bufferFor finds the open buffer for a path, which is how a run error jumps
// to the right window.
func (e *Editor) bufferFor(path string) (int, *buffer) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return -1, nil
	}
	for i, b := range e.buffers {
		if b.path == abs {
			return i, b
		}
	}
	return -1, nil
}

// gotoLine puts the cursor at the start of a 1-based line, and the line where it
// can be seen.
func (e *Editor) gotoLine(b *buffer, line int) {
	if line < 1 {
		line = 1
	}
	offset := offsetAt(b.area.GetText(), line-1, 0)
	e.showAndSelect(b, offset, offset)
}

// displayDir names a directory in a way that can be recognised: relative to the
// working directory when it is inside it, under ~ when it is in the home
// directory, and in full otherwise. Unlike a file, the last element of a folder's
// path is rarely enough to tell which folder it is.
func displayDir(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, abs); err == nil && !strings.HasPrefix(rel, "..") {
			if rel == "." {
				return "."
			}
			return rel
		}
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(abs, home+string(filepath.Separator)) {
		return "~" + strings.TrimPrefix(abs, home)
	}
	return abs
}

// displayName keeps paths short: a file under the working directory shows as a
// relative path, anything else keeps its base name.
func displayName(abs string) string {
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, abs); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return filepath.Base(abs)
}

func pageName(i int) string { return fmt.Sprintf("buffer-%d", i) }
