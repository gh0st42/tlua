//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import (
	"testing"

	"github.com/pwiecz/go-fltk"

	in "tlua/internal/gui/fltkinput"
)

// A MarkdownEdit takes the keyboard, edits its document as Markdown, and
// hands links and right clicks to the program. Its text column starts 16
// in, and its first line is 23 high below 16 of padding.
func TestMarkdownEdit(t *testing.T) {
	onScreen(t, `
form = gui.Form{width = 420, height = 260}
changes, linked, menu = 0, nil, nil
edit = form:MarkdownEdit{left = 10, top = 10, width = 400, height = 240,
  onChange = function() changes = changes + 1 end,
  onLink = function(self, url) linked = url end,
  onMenu = function() menu = true end}
edit:load("[[Other Note]] is linked\n")
form:show()`, func(s *scene) {
		s.expect(`edit.pointer == "text" and edit:markdown() == "[[Other Note]] is linked\n"`)

		// Typing goes in at the caret, and Markdown shortcuts work.
		s.focus("edit")
		s.lua(`edit.ed:moveToDocument(1) edit:moved()`)
		in.Key(fltk.ENTER_KEY, "", 0)
		in.Type("# Title")
		s.expect(`edit:markdown() == "[[Other Note]] is linked\n\n# Title\n" and changes > 0`)
		in.Key(fltk.BACKSPACE, "", 0)
		s.expect(`edit:markdown():match("# Titl\n$")`)

		// A Cmd- or Ctrl-click on a link is the program's to follow.
		x, y := s.at("edit", 16+30, 16+11)
		in.ClickWith(x, y, fltk.CTRL)
		s.expect(`linked == "Other Note.md"`)

		in.RightClick(x, y)
		s.expect(`menu == true`)
	})
}
