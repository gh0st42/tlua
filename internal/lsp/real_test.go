package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRealServerFormats exercises the client against whatever language server
// is installed on the machine running the tests, and skips when there is none.
// The fake server in testdata proves the protocol handling; this proves it
// against a server that was not written to match it.
func TestRealServerFormats(t *testing.T) {
	command, args, ok := Find()
	if !ok {
		t.Skip("no language server on PATH")
	}
	t.Logf("using %s %v", command, args)

	dir := t.TempDir()
	path := filepath.Join(dir, "main.lua")
	messy := "local   t  =  {a=1,b=2}\nfor i=1,10   do\nprint( i ,t.a )\nend\n"
	if err := os.WriteFile(path, []byte(messy), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := Start(ctx, command, args, dir)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer client.Close()

	t.Logf("server name: %q, formats: %v", client.Name(), client.CanFormat())
	if !client.CanFormat() {
		t.Skip("this server does not offer formatting")
	}

	formatted, err := client.Format(ctx, path, messy, DefaultFormatOptions())
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	t.Logf("before:\n%s\nafter:\n%s", messy, formatted)
	if formatted == messy {
		t.Error("the server changed nothing")
	}
}

// TestRealServerCompletesAndHovers asks a real server about a standard library
// function, and skips when there is no server to ask.
func TestRealServerCompletesAndHovers(t *testing.T) {
	command, args, ok := Find()
	if !ok {
		t.Skip("no language server on PATH")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "main.lua")
	text := "local s = str\nprint(s)\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := Start(ctx, command, args, dir)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer client.Close()
	t.Logf("%s completes: %v, hovers: %v", client.Name(), client.CanComplete(), client.CanHover())

	if client.CanComplete() {
		// A real server reads the workspace before it can answer, and says null
		// until it has, so give it a few tries.
		var items []CompletionItem
		for attempt := 0; attempt < 10; attempt++ {
			var err error
			items, err = client.Complete(ctx, path, text, Position{Line: 0, Character: 13})
			if err != nil {
				t.Fatalf("complete: %v", err)
			}
			if len(items) > 0 {
				break
			}
			time.Sleep(300 * time.Millisecond)
		}
		t.Logf("%d completions for \"str\"", len(items))
		for i, item := range items {
			if i == 5 {
				break
			}
			t.Logf("  %-20s %-10s %s", item.Label, item.Kind, oneLineForLog(item.Help()))
		}
		if len(items) == 0 {
			t.Error("a real server offered nothing for \"str\"")
		}
	}

	if client.CanHover() {
		// Over "print" on the second line.
		help, err := client.Hover(ctx, path, text, Position{Line: 1, Character: 2})
		if err != nil {
			t.Fatalf("hover: %v", err)
		}
		t.Logf("hover over print:\n%s", help)
		if help == "" {
			t.Error("a real server said nothing about print")
		}
	}
}

func oneLineForLog(text string) string {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return text[:i] + "..."
	}
	return text
}
