//go:build cgo && ((darwin && (amd64 || arm64)) || (linux && (amd64 || arm || arm64)) || (openbsd && (amd64 || arm64)) || (windows && amd64))

package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pwiecz/go-fltk"

	in "tlua/internal/gui/fltkinput"
)

// The pages put each link at the start of a paragraph, so where to click
// is known: a paragraph's line is 23 high with 10 after it, below 12 of
// padding.
func TestMarkdownView(t *testing.T) {
	dir := t.TempDir()
	pages := map[string]string{
		"one.md": "[Next](two.md)\n\n[Web](https://example.com/x)\n\n[App](app:thing)\n\n## The end\n\nlast\n",
		"two.md": "[Home](one.md#the-end)\n\n# Two\n\nsecond page\n",
	}
	for name, text := range pages {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var opened []string
	saved := openCommand
	openCommand = func(target string) *exec.Cmd {
		opened = append(opened, target)
		return exec.Command("true")
	}
	defer func() { openCommand = saved }()

	onScreen(t, `
form = gui.Form{width = 340, height = 240}
gui.clipboard = function(text) copied = text end -- not the real one
navigated, hovered, linked = 0, nil, {}
view = form:MarkdownView{left = 10, top = 10, width = 320, height = 220,
  file = "`+filepath.ToSlash(dir)+`/one.md",
  onNavigate = function() navigated = navigated + 1 end,
  onHover = function(self, url) hovered = url end,
  onLink = function(self, url)
    linked[#linked + 1] = url
    if url:match("^app:") then return true end
  end}
form:show()`, func(s *scene) {
		line := func(n int) (int, int) { return s.at("view", 12+12, 12+n*33+11) }
		s.expect(`#view:headings() == 1 and view:headings()[1].anchor == "the-end"`)

		// Over a link the pointer is a hand, and the program hears of it.
		x, y := line(0)
		in.Send(in.Event{Type: fltk.MOVE, X: x, Y: y})
		in.Send(in.Event{Type: fltk.MOVE, X: x + 1, Y: y})
		s.expect(`hovered == "two.md" and view.pointer == "hand"`)
		lx, ly := line(3)
		in.Send(in.Event{Type: fltk.MOVE, X: lx + 200, Y: ly})
		s.expect(`hovered == nil and view.pointer == "default"`)

		// A link to a page shows it, and back comes back.
		in.Click(x, y)
		s.expect(`view.file:match("two%.md$") and navigated == 1 and linked[1] == "two.md"`)
		s.expect(`view:canGoBack() and not view:canGoForward()`)
		s.expect(`view:headings()[1].text == "Two"`)
		in.Click(line(0))
		s.expect(`view.file:match("one%.md$") and navigated == 2`)
		s.focus("view")
		in.Key(fltk.LEFT, "", fltk.ALT)
		s.expect(`view.file:match("two%.md$") and view:canGoForward()`)
		s.lua(`view:back()`)
		s.expect(`view.file:match("one%.md$") and not view:canGoBack()`)

		// A web link goes to the browser; an app: link to the program.
		in.Click(line(1))
		s.pump()
		if len(opened) != 1 || opened[0] != "https://example.com/x" {
			t.Errorf("the web link opened %v", opened)
		}
		in.Click(line(2))
		s.expect(`linked[#linked] == "app:thing" and view.file:match("one%.md$")`)
		if len(opened) != 1 {
			t.Errorf("the app link was opened too: %v", opened)
		}

		// Dragging selects, and the selection is copied as text.
		sx, sy := s.at("view", 12, 12+3*33+11)
		in.Drag(sx, sy, sx+200, sy, 4)
		s.expect(`view:selectedText() == "The end"`)
		in.Key('c', "c", fltk.CTRL)
		s.expect(`copied == "The end"`)

		// search finds and selects, from the top again at the end.
		s.expect(`view:search("LAST") and view:selectedText() == "last"`)
		s.expect(`view:search("next") and view:selectedText() == "Next"`)
		s.expect(`not view:search("nowhere")`)

		// Text given directly is shown; a page that is not there says so.
		s.lua(`view.text = "# Hi\n\nthere [x](#hi)"`)
		s.expect(`view:headings()[1].text == "Hi" and view:scrollTo("hi") and not view:scrollTo("nope")`)
		s.expect(`select(1, view:open("nope.md")) == false and view.text:find("Not found")`)
	})
}

func TestMarkdownViewIsBuiltIn(t *testing.T) {
	onScreen(t, `
local k = gui.kinds().MarkdownView
builtin, events = k.builtin, table.concat(k.events, " ")
fileType, sizeDefault = k.props.file.type, k.props.textSize.default
ok, err = pcall(gui.define, {name = "MarkdownView", build = function() end})
form = gui.Form{}
layout = gui.dump(form:MarkdownView{name = "help", text = "*hi*", width = 100})
`, func(s *scene) {
		s.expect(`builtin and events:find("onLink") and events:find("onNavigate") and events:find("onHover")`)
		s.expect(`fileType == "file" and sizeDefault == 15`)
		s.expect(`not ok and err:find("already defined")`)
		s.expect(`layout.kind == "MarkdownView" and layout.text == "*hi*" and layout.width == 100`)
	})
}

// A list of contents links to the headings, and a wiki link finds its
// page, here by the dashes a GitHub wiki puts for spaces.
func TestMarkdownViewContentsAndWikiLinks(t *testing.T) {
	dir := t.TempDir()
	pages := map[string]string{
		"home.md":        "[[toc]]\n\n# A\n\n[[Second Page]]\n\n## B\n\nend\n",
		"Second-Page.md": "# Second\n",
	}
	for name, text := range pages {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	onScreen(t, `
form = gui.Form{width = 340, height = 400}
linked = {}
view = form:MarkdownView{left = 10, top = 10, width = 320, height = 380,
  file = "`+filepath.ToSlash(dir)+`/home.md",
  onLink = function(self, url) linked[#linked + 1] = url end}
form:show()`, func(s *scene) {
		// The list: A, then B under it, 24 further in; then the heading A
		// (16 before it, 42 high, 10 after), then the wiki link.
		in.Click(s.at("view", 12+24+4, 12+23+11))
		s.expect(`linked[1] == "#b" and view.file:match("home%.md$")`)
		in.Click(s.at("view", 12+4, 12+(46+12)+16+52+11))
		s.expect(`linked[2] == "Second Page.md" and view.file:match("Second%-Page%.md$")`)
		s.expect(`view:headings()[1].text == "Second"`)
	})
}
