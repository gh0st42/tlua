package doc

import (
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"

	"tlua/docs"
)

func TestTopics(t *testing.T) {
	texts, _, err := pages()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index", "sys", "term", "tui", "inspect", "dkjson", "argparse",
		"socket", "socket.http", "lua-language", "lua-stdlib", "gui", "design", "pico"} {
		if texts[name] == "" {
			t.Errorf("no page %s", name)
		}
	}
	for topic, want := range map[string][2]string{
		"":                    {"Modules\n=======", "Pages: index, "},
		"tui":                 {"tui — Turbo Vision", "tui.messagebox"},
		"lua":                 {"Lua language basics", "Not in Lua 5.1"},
		"sys.copy":            {"sys.copy(src, dst)\n\nCopies", ""},
		"tui.Application:run": {"tui.Application\n", "app:run()"},
		"socket.http.request": {"http.request(url_or_reqt, body)", ""},
		"socket.tp.connect":   {"tp.connect(", ""},
	} {
		text, err := show(texts, topic, 80, false)
		if err != nil {
			t.Errorf("%q: %v", topic, err)
			continue
		}
		if !strings.HasPrefix(text, want[0]) || !strings.Contains(text, want[1]) {
			t.Errorf("%q: got\n%.300s", topic, text)
		}
	}
	// A part of a page ends where the next heading of its level begins.
	if text, _ := show(texts, "sys.copy", 80, false); strings.Contains(text, "sys.move") {
		t.Errorf("sys.copy runs on into sys.move:\n%s", text)
	}
	if _, err := show(texts, "connect", 80, false); err == nil || !strings.Contains(err.Error(), "tlua doc socket.tp.connect") {
		t.Errorf("connect, on two pages: %v", err)
	}
	if _, err := show(texts, "no such thing", 80, false); err == nil || !strings.Contains(err.Error(), "the pages are index,") {
		t.Errorf("nonsense: %v", err)
	}
}

var escapes = regexp.MustCompile("\x1b\\[[0-9;]*m")

// Every page is drawn, in colour and without, and its prose fits the width.
func TestEveryPageFits(t *testing.T) {
	texts, _, _ := pages()
	for name := range texts {
		for _, color := range []bool{false, true} {
			text, err := show(texts, name, 72, color)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if color != strings.Contains(text, "\x1b[") {
				t.Errorf("%s: colour %v, but escapes %v", name, color, !color)
			}
			for _, line := range strings.Split(escapes.ReplaceAllString(text, ""), "\n") {
				if n := len([]rune(line)); n > 72 && !strings.HasPrefix(line, "    ") {
					t.Errorf("%s: %d columns: %q", name, n, line)
				}
			}
		}
	}
}

// The links from one page to another lead to a page that is built in too.
func TestLinksBetweenPages(t *testing.T) {
	links := regexp.MustCompile(`\]\(([^)#\s]+\.md)(#[^)]*)?\)`)
	fs.WalkDir(docs.Files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, _ := docs.Files.ReadFile(p)
		for _, m := range links.FindAllStringSubmatch(string(data), -1) {
			if strings.Contains(m[1], "://") {
				continue
			}
			target := path.Join(path.Dir(p), m[1])
			if _, err := fs.Stat(docs.Files, target); err != nil {
				t.Errorf("%s links to %s, which is not built in", p, m[1])
			}
		}
		return nil
	})
}

func TestCommand(t *testing.T) {
	var out, errs strings.Builder
	if code := command([]string{"sys.cwd"}, &out, &errs); code != 0 || !strings.HasPrefix(out.String(), "sys.cwd()") {
		t.Errorf("status %d, %q, %q", code, out.String(), errs.String())
	}
	out.Reset()
	if code := command([]string{"nothing-like-it"}, &out, &errs); code != 1 || out.Len() != 0 {
		t.Errorf("status %d for an unknown topic", code)
	}
	if code := command([]string{"a", "b"}, &out, &errs); code != 2 {
		t.Errorf("status %d for two topics", code)
	}
	errs.Reset()
	if code := command([]string{"-h"}, &out, &errs); code != 0 || !strings.Contains(errs.String(), "usage: tlua doc") {
		t.Errorf("status %d for -h: %q", code, errs.String())
	}
}
