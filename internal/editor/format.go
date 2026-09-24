package editor

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"tlua/internal/lsp"
)

// How long the editor is prepared to wait for a language server: long enough
// for a formatter to think, short enough that a wedged server cannot hold a
// save hostage.
const (
	lspStartTimeout  = 15 * time.Second
	lspFormatTimeout = 3 * time.Second
)

// startLanguageServer looks for a language server and, if one is on PATH,
// starts it in the background. Nothing waits for it: the editor is usable
// while the server settles, and formatting begins working once it answers.
func (e *Editor) startLanguageServer() {
	command, args, ok := lsp.Find()
	if !ok {
		return
	}

	root, err := filepath.Abs(".")
	if err != nil {
		root = "."
	}
	if b := e.buf(); b != nil && b.path != "" {
		root = filepath.Dir(b.path)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), lspStartTimeout)
		defer cancel()
		client, err := lsp.Start(ctx, command, args, root)
		e.app.QueueUpdateDraw(func() {
			if err != nil {
				e.lspNote = fmt.Sprintf("%s did not start: %v", filepath.Base(command), err)
				return
			}
			if e.leaving {
				client.Close() // the editor left while the server was starting
				return
			}
			e.lsp = client
			e.watchDiagnostics(client)
			// Tell it about what is already open, so it starts reading the
			// workspace now rather than when the first question is asked: a
			// server that has just started answers "nothing" until it has.
			for _, b := range e.buffers {
				_ = client.Sync(b.lspPath(), b.area.GetText())
			}
			if client.CanFormat() {
				e.lspNote = client.Name() + ": formatting on save"
			} else {
				e.lspNote = client.Name() + ": no formatting offered"
			}
			// The server arrives whenever it arrives, so say so only while the
			// status line has nothing of the user's own on it.
			if e.status == "" || strings.HasPrefix(e.status, "F5 runs") {
				e.setStatus(e.lspNote)
			}
		})
	}()
}

// stopLanguageServer shuts the server down on the way out.
func (e *Editor) stopLanguageServer() {
	if e.lsp != nil {
		e.lsp.Close()
		e.lsp = nil
	}
}

// canFormat reports whether there is a server able to format.
func (e *Editor) canFormat() bool {
	return e.lsp != nil && e.lsp.CanFormat()
}

// formatBuffer replaces a buffer's text with the language server's formatting
// of it. The whole text goes back as one edit, so one undo takes the formatting
// away again, and the cursor keeps its line.
func (e *Editor) formatBuffer(b *buffer) error {
	if !e.canFormat() {
		return fmt.Errorf("no language server is formatting")
	}
	path := b.path
	if path == "" {
		return fmt.Errorf("%s has no file name yet", b.name)
	}

	text := b.area.GetText()
	ctx, cancel := context.WithTimeout(context.Background(), lspFormatTimeout)
	defer cancel()

	formatted, err := e.lsp.Format(ctx, path, text, lsp.DefaultFormatOptions())
	if err != nil {
		return err
	}
	if formatted == text {
		return nil
	}

	row, column, _, _ := b.area.GetCursor()
	b.area.Replace(0, b.area.GetTextLength(), formatted)
	e.restoreCursor(b, row, column)
	return nil
}

// restoreCursor puts the cursor back on the line it was on, as near the same
// column as that line now allows.
func (e *Editor) restoreCursor(b *buffer, row, column int) {
	offset := offsetAt(b.area.GetText(), row, column)
	b.area.Select(offset, offset)
}

// offsetAt returns the byte offset of a display column on a line, counting
// columns the way the text area draws them: a tab is a whole tab stop, and a
// wide rune is two. A column past the end of the line stops at its end.
func offsetAt(text string, row, column int) int {
	offset, line := 0, 0
	for line < row {
		nl := strings.IndexByte(text[offset:], '\n')
		if nl < 0 {
			return len(text)
		}
		offset += nl + 1
		line++
	}

	display := 0
	for i := offset; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == '\n' || display >= column {
			return i
		}
		display += cellWidth(r)
		i += size
	}
	return len(text)
}

// formatCurrent is the Format command: F12, or the Edit menu.
func (e *Editor) formatCurrent() {
	b := e.buf()
	if b == nil {
		return
	}
	switch {
	case e.lsp == nil:
		e.setStatus("No language server; put one on PATH, or set " + lsp.EnvServer)
	case !e.lsp.CanFormat():
		e.setStatus(e.lsp.Name() + " does not offer formatting")
	default:
		if err := e.formatBuffer(b); err != nil {
			e.setStatus("Format: " + err.Error())
			return
		}
		e.setStatus("Formatted " + b.name + " with " + e.lsp.Name())
	}
}

// toggleFormatOnSave turns the formatting pass on and off, for when a server's
// idea of tidy is not the one you want.
func (e *Editor) toggleFormatOnSave() {
	e.formatOnSave = !e.formatOnSave
	switch {
	case !e.formatOnSave:
		e.setStatus("Format on save is off")
	case e.canFormat():
		e.setStatus("Format on save is on, using " + e.lsp.Name())
	default:
		e.setStatus("Format on save is on, but no language server is formatting")
	}
	e.menus = e.buildMenus() // the menu shows the state
}

// formatOnSaveLabel is what the Edit menu says about the setting.
func (e *Editor) formatOnSaveLabel() string {
	if e.formatOnSave {
		return "Format on save [on]"
	}
	return "Format on save [off]"
}

// languageServerDialog says what the editor found, or what to do about finding
// nothing. A language server is optional, so this explains rather than warns.
func (e *Editor) languageServerDialog() {
	lines := []string{}
	switch {
	case e.lsp == nil && e.lspNote != "":
		lines = append(lines, e.lspNote)
	case e.lsp == nil:
		lines = append(lines,
			"No language server was found on PATH.",
			"",
			"The editor looks for lua-language-server,",
			"emmylua_ls and lua-lsp, in that order.",
			"Set "+lsp.EnvServer+" to name another one,",
			"or "+lsp.EnvServer+"=off to do without.")
	default:
		state := "on"
		if !e.formatOnSave {
			state = "off"
		}
		lines = append(lines, "Server:  "+e.lsp.Name())
		if e.lsp.CanFormat() {
			lines = append(lines, "Formats: yes", "On save: "+state, "", "F12 formats the buffer now.")
		} else {
			lines = append(lines, "Formats: no", "", "This server offers no formatting,", "so saving leaves the text alone.")
		}
		if e.lspNote != "" {
			lines = append(lines, "", e.lspNote)
		}
	}
	e.message("Language server", joinLines(lines))
}

func joinLines(lines []string) string {
	out := ""
	for i, line := range lines {
		if i > 0 {
			out += "\n"
		}
		out += line
	}
	return out
}
