package design

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"

	"tlua/internal/lsp"
)

func TestPositionsAreLinesAndUTF16Columns(t *testing.T) {
	text := "ab\nçd𝄞x\n"
	for cursor, want := range map[int]lsp.Position{
		0: {Line: 0, Character: 0}, 2: {Line: 0, Character: 2}, 3: {Line: 1, Character: 0},
		5: {Line: 1, Character: 1}, 10: {Line: 1, Character: 4},
	} {
		if got := positionOf(text, cursor); got != want {
			t.Errorf("positionOf(%d) = %+v, want %+v", cursor, got, want)
		}
	}
}

func TestCompletionsReplaceTheWordTyped(t *testing.T) {
	text := "gui.Fo"
	items := completions(text, len(text), []lsp.CompletionItem{
		{Label: "Form", SortText: "2"},
		{Label: "Frame", SortText: "1", TextEdit: &lsp.TextEdit{
			Range:   lsp.Range{Start: lsp.Position{Character: 4}, End: lsp.Position{Character: 6}},
			NewText: "Frame",
		}},
	})
	if len(items) != 2 || items[0].label != "Frame" {
		t.Fatalf("items %+v: want them in the server's order", items)
	}
	if items[0].from != 5 || items[0].to != 6 || items[0].text != "Frame" {
		t.Errorf("an item's own edit: %+v", items[0])
	}
	if items[1].from != 5 || items[1].to != 6 || items[1].text != "Form" {
		t.Errorf("the word being typed: %+v", items[1])
	}
}

// TestTheServerKnowsTheGuiModule asks a real lua-language-server, where
// there is one, what follows "gui." in a project's code: tlua's own
// declarations are given to it, so it knows.
func TestTheServerKnowsTheGuiModule(t *testing.T) {
	if _, err := exec.LookPath("lua-language-server"); err != nil {
		t.Skip("no lua-language-server on PATH")
	}
	t.Setenv(lsp.EnvServer, "")
	r, _ := newLua(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.lua"), []byte("local gui = require \"gui\"\n"), 0o644)
	r.L.SetGlobal("dir", lua.LString(dir))
	run(t, r.L, `
server = require "design.server"
s = assert(server.start(dir))
text = 'local gui = require "gui"\ngui.'
path = dir .. "/main.lua"
`)
	poll := func(call string) lua.LValue {
		t.Helper()
		run(t, r.L, "req = "+call)
		deadline := time.Now().Add(60 * time.Second)
		for time.Now().Before(deadline) {
			run(t, r.L, "done, value, why = req:result()")
			if lua.LVAsBool(r.L.GetGlobal("done")) {
				if why := r.L.GetGlobal("why"); why != lua.LNil {
					t.Fatalf("%s: %s", call, why)
				}
				return r.L.GetGlobal("value")
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("%s: no answer", call)
		return nil
	}
	// The first answers can come before the server has read the library.
	var labels []string
	for try := 0; try < 40 && !contains(labels, "Form"); try++ {
		labels = nil
		if items, ok := poll(`s:complete(path, text, #text)`).(*lua.LTable); ok {
			items.ForEach(func(_, v lua.LValue) {
				labels = append(labels, lua.LVAsString(v.(*lua.LTable).RawGetString("label")))
			})
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !contains(labels, "Form") {
		t.Fatalf("after gui. the server offers %v, without Form", labels)
	}
	help := lua.LVAsString(poll(`s:hover(path, text, #'local gu')`))
	if !strings.Contains(help, "gui") {
		t.Errorf("hover over gui says %q", help)
	}
	run(t, r.L, `assert(s:state() == "ready"); s:close()`)
}

// contains says whether a label in list is want, or want(...).
func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want || strings.HasPrefix(s, want+"(") {
			return true
		}
	}
	return false
}
