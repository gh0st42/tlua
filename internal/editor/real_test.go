package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"tlua/internal/lsp"
)

// TestRealServerCompletesAfterADot drives the editor with whatever language
// server is installed, typing "io." the way a user does. It skips when there is
// no server to ask.
func TestRealServerCompletesAfterADot(t *testing.T) {
	command, args, ok := lsp.Find()
	if !ok {
		t.Skip("no language server on PATH")
	}
	// start() defaults to no server, so name the real one explicitly.
	t.Setenv(lsp.EnvServer, strings.Join(append([]string{command}, args...), " "))

	dir := t.TempDir()
	path := filepath.Join(dir, "main.lua")
	if err := os.WriteFile(path, []byte("local f = io\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e, screen := start(t, path)
	waitFor(t, e, "the language server", func() bool { return e.lsp != nil && e.lsp.CanComplete() })
	onEditor(t, e, func() { t.Logf("server: %s, triggers %v", e.lsp.Name(), e.lsp.TriggerCharacters()) })

	onEditor(t, e, func() {
		b := e.buffers[0]
		offset := offsetAt(b.area.GetText(), 0, 12)
		b.area.Select(offset, offset)
	})

	// A real server needs a moment to read the standard library, so type the
	// dot, and if nothing comes back, ask again by hand as a user would.
	typeText(screen, ".")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		opened := false
		onEditor(t, e, func() { opened = e.modals == 1 })
		if opened {
			break
		}
		press(screen, tcell.KeyCtrlSpace, 0, tcell.ModNone)
		time.Sleep(300 * time.Millisecond)
	}

	onEditor(t, e, func() {
		if e.modals != 1 {
			t.Fatalf("no completion list appeared; status is %q", e.status)
		}
		list := completionList(t, e)
		t.Logf("%d completions after io.", list.GetItemCount())
		labels := []string{}
		for i := 0; i < list.GetItemCount() && i < 6; i++ {
			label, _ := list.GetItemText(i)
			labels = append(labels, label)
		}
		t.Logf("first few: %s", strings.Join(labels, ", "))
		if list.GetItemCount() == 0 {
			t.Error("the list is empty")
		}
	})

	// Picking the first one inserts it after the dot.
	press(screen, tcell.KeyEnter, 0, tcell.ModNone)
	waitFor(t, e, "the insertion", func() bool {
		text := e.buffers[0].area.GetText()
		return e.modals == 0 && text != "local f = io.\n" && strings.HasPrefix(text, "local f = io.")
	})
	onEditor(t, e, func() { t.Logf("buffer now: %q", e.buffers[0].area.GetText()) })
}
